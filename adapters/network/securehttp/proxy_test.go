// SPDX-License-Identifier: MPL-2.0

package securehttp

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// connectProxy is a forward proxy on a loopback TCP port that tunnels CONNECT requests to a
// fixed upstream and records what the client asked for.
type connectProxy struct {
	listener net.Listener
	upstream string
	status   int

	mu          sync.Mutex
	targets     []string
	connections int
}

func newConnectProxy(t *testing.T, upstream string, status int) *connectProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &connectProxy{listener: listener, upstream: upstream, status: status}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			proxy.mu.Lock()
			proxy.connections++
			proxy.mu.Unlock()
			go proxy.serve(connection)
		}
	}()
	return proxy
}

func (p *connectProxy) address() string { return p.listener.Addr().String() }

func (p *connectProxy) recorded() ([]string, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...), p.connections
}

func (p *connectProxy) serve(client net.Conn) {
	defer client.Close()
	reader := bufio.NewReader(client)
	request, err := http.ReadRequest(reader)
	if err != nil || request.Method != http.MethodConnect {
		return
	}
	p.mu.Lock()
	p.targets = append(p.targets, request.Host)
	p.mu.Unlock()
	if p.status != http.StatusOK {
		_, _ = io.WriteString(client, "HTTP/1.1 "+strconv.Itoa(p.status)+" "+http.StatusText(p.status)+"\r\n\r\n")
		return
	}
	upstream, err := net.Dial("tcp", p.upstream)
	if err != nil {
		return
	}
	defer upstream.Close()
	_, _ = io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n")
	go func() { _, _ = io.Copy(upstream, reader); upstream.Close() }()
	_, _ = io.Copy(client, upstream)
}

func forbiddenResolver(t *testing.T) func(context.Context, string) ([]netip.Addr, error) {
	return func(_ context.Context, host string) ([]netip.Addr, error) {
		t.Errorf("local DNS used for %q while a proxy is configured", host)
		return nil, io.ErrUnexpectedEOF
	}
}

func proxiedClient(t *testing.T, origin string, roots *x509.CertPool, proxy func() string) *Client {
	t.Helper()
	c, err := newClient(Config{AllowedOrigins: []string{origin}, Proxy: proxy}, forbiddenResolver(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if roots != nil {
		c.httpClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	}
	return c
}

func TestProxySendsConnectWithHostnameAndKeepsTLSVerification(t *testing.T) {
	server, roots := trustedTLSServer(t, "example.test")
	defer server.Close()
	proxy := newConnectProxy(t, server.Listener.Addr().String(), http.StatusOK)
	c := proxiedClient(t, "https://example.test", roots, func() string { return proxy.address() })
	response, err := c.Do(mustRequest(t, "https://example.test/page"))
	if err != nil || response.StatusCode != http.StatusOK || string(response.Body) != "ok" {
		t.Fatal(response, err)
	}
	if targets, _ := proxy.recorded(); len(targets) != 1 || targets[0] != "example.test:443" {
		t.Fatalf("CONNECT targets = %v", targets)
	}
}

func TestProxyKeepsOriginAllowlistAndCertificateChecks(t *testing.T) {
	server, roots := trustedTLSServer(t, "example.test")
	defer server.Close()
	proxy := newConnectProxy(t, server.Listener.Addr().String(), http.StatusOK)
	c := proxiedClient(t, "https://example.test", roots, func() string { return proxy.address() })
	if _, err := c.Do(mustRequest(t, "https://other.test/")); !isKind(err, KindOrigin) {
		t.Fatal(err)
	}
	if targets, _ := proxy.recorded(); len(targets) != 0 {
		t.Fatalf("a request outside the allowlist reached the proxy: %v", targets)
	}
	// The tunnel is made, but the certificate does not chain to a trusted root.
	untrusted := proxiedClient(t, "https://example.test", nil, func() string { return proxy.address() })
	if _, err := untrusted.Do(mustRequest(t, "https://example.test/")); !isKind(err, KindNetwork) {
		t.Fatal(err)
	}
	wrongName := proxiedClient(t, "https://wrong.test", roots, func() string { return proxy.address() })
	if _, err := wrongName.Do(mustRequest(t, "https://wrong.test/")); !isKind(err, KindNetwork) {
		t.Fatal(err)
	}
}

func TestProxyExemptionCoversOnlyTheProxyHop(t *testing.T) {
	proxy := newConnectProxy(t, "127.0.0.1:1", http.StatusOK)
	c := proxiedClient(t, "https://127.0.0.1", nil, func() string { return proxy.address() })
	// A loopback proxy is allowed, but a loopback destination is still rejected, and nothing is sent.
	if _, err := c.Do(mustRequest(t, "https://127.0.0.1/")); !isKind(err, KindAddress) {
		t.Fatal(err)
	}
	if _, connections := proxy.recorded(); connections != 0 {
		t.Fatal("a blocked destination was sent to the proxy")
	}
	if err := c.validateDestination(context.Background(), "10.0.0.1"); !isKind(err, KindAddress) {
		t.Fatal(err)
	}
	if err := c.validateDestination(context.Background(), "redirect.example"); err != nil {
		t.Fatal(err)
	}
	// Without a proxy, a destination name is still resolved and judged locally.
	direct := newTestClient(t, []string{"https://example.com"}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}, nil)
	if err := direct.validateDestination(context.Background(), "example.com"); !isKind(err, KindAddress) {
		t.Fatal(err)
	}
}

func TestProxyFailuresDoNotMentionTheProxyAddress(t *testing.T) {
	server, roots := trustedTLSServer(t, "example.test")
	defer server.Close()
	refusing := newConnectProxy(t, server.Listener.Addr().String(), http.StatusForbidden)
	down := newConnectProxy(t, server.Listener.Addr().String(), http.StatusOK)
	downAddress := down.address()
	down.listener.Close()
	for name, test := range map[string]struct {
		address string
		kind    Kind
	}{
		"refused":     {refusing.address(), KindProxyRefused},
		"unreachable": {downAddress, KindProxy},
	} {
		t.Run(name, func(t *testing.T) {
			c := proxiedClient(t, "https://example.test", roots, func() string { return test.address })
			_, err := c.Do(mustRequest(t, "https://example.test/"))
			if !isKind(err, test.kind) {
				t.Fatal(err)
			}
			_, port, _ := net.SplitHostPort(test.address)
			for _, text := range []string{err.Error(), unwrapChain(err)} {
				if strings.Contains(text, test.address) || strings.Contains(text, port) {
					t.Fatalf("error mentions the proxy: %q", text)
				}
			}
		})
	}
}

func unwrapChain(err error) string {
	var parts []string
	for ; err != nil; err = unwrapOnce(err) {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, " | ")
}

func unwrapOnce(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return nil
}

func TestProxyChangeAppliesToNewRequests(t *testing.T) {
	server, roots := trustedTLSServer(t, "example.test")
	defer server.Close()
	first := newConnectProxy(t, server.Listener.Addr().String(), http.StatusOK)
	second := newConnectProxy(t, server.Listener.Addr().String(), http.StatusOK)
	var mu sync.Mutex
	current := first.address()
	c := proxiedClient(t, "https://example.test", roots, func() string { mu.Lock(); defer mu.Unlock(); return current })
	get := func() {
		t.Helper()
		if response, err := c.Do(mustRequest(t, "https://example.test/")); err != nil || response.StatusCode != http.StatusOK {
			t.Fatal(response, err)
		}
	}
	get()
	get() // reuses the first tunnel
	mu.Lock()
	current = second.address()
	mu.Unlock()
	get()
	if _, connections := first.recorded(); connections != 1 {
		t.Fatalf("first proxy connections = %d, want 1", connections)
	}
	if targets, connections := second.recorded(); connections != 1 || len(targets) != 1 {
		t.Fatalf("second proxy = %v, %d connections", targets, connections)
	}
	// Back to direct: the name is resolved locally again and the proxy is not used.
	mu.Lock()
	current = ""
	mu.Unlock()
	dialed := ""
	c.resolver = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}
	c.dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = address
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	get()
	if dialed != "1.1.1.1:443" {
		t.Fatalf("direct dial = %q", dialed)
	}
	if _, connections := second.recorded(); connections != 1 {
		t.Fatal("the proxy was used after it was switched off")
	}
}

func TestClientWithoutProxyKeepsTransportProxyUnset(t *testing.T) {
	c := newTestClient(t, []string{"https://example.com"}, nil, nil)
	if c.httpClient.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("transport proxy set without a configured source")
	}
}

func TestDialProxyTriesEachResolvedAddress(t *testing.T) {
	c, err := newClient(Config{AllowedOrigins: []string{"https://example.test"}, Proxy: func() string { return "proxy.test:8080" }}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var dialed []string
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	c.dialer = func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		if len(dialed) == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return client, nil
	}
	connection, err := c.dialProxy(context.Background(), "tcp", "proxy.test", "8080")
	if err != nil || connection != client {
		t.Fatal(connection, err)
	}
	if len(dialed) != 2 || dialed[0] != "10.0.0.1:8080" || dialed[1] != "10.0.0.2:8080" {
		t.Fatalf("dialed %v", dialed)
	}
	c.dialer = func(context.Context, string, string) (net.Conn, error) { return nil, io.ErrUnexpectedEOF }
	if _, err := c.dialProxy(context.Background(), "tcp", "proxy.test", "8080"); !isKind(err, KindProxy) {
		t.Fatalf("all addresses failing = %v", err)
	}
}

func TestOnlyTheProxyTheHookLastReturnedIsExemptFromTheAddressPolicy(t *testing.T) {
	current := "127.0.0.1:7890"
	c, err := newClient(Config{AllowedOrigins: []string{"https://example.test"}, Proxy: func() string { return current }}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}, func(context.Context, string, string) (net.Conn, error) { return nil, io.ErrUnexpectedEOF })
	if err != nil {
		t.Fatal(err)
	}
	// Before the hook has chosen a proxy, nothing is exempt.
	if _, err := c.dialContext(context.Background(), "tcp", "127.0.0.1:7890"); !isKind(err, KindAddress) {
		t.Fatalf("before any request: %v", err)
	}
	if _, err := c.proxyFor(mustRequest(t, "https://example.test/")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.dialContext(context.Background(), "tcp", "127.0.0.1:7890"); !isKind(err, KindProxy) {
		t.Fatalf("the chosen proxy must be dialled: %v", err)
	}
	// The source changes, but the hook has not chosen the new proxy yet: its address stays blocked.
	current = "127.0.0.1:9999"
	if _, err := c.dialContext(context.Background(), "tcp", "127.0.0.1:9999"); !isKind(err, KindAddress) {
		t.Fatalf("an address the hook did not choose: %v", err)
	}
}
