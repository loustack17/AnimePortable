package securehttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Kind string

const (
	KindConfig       Kind = "config"
	KindURL          Kind = "url"
	KindOrigin       Kind = "origin"
	KindAddress      Kind = "address"
	KindRedirect     Kind = "redirect"
	KindNetwork      Kind = "network"
	KindTimeout      Kind = "timeout"
	KindCanceled     Kind = "canceled"
	KindResponse     Kind = "response"
	KindBodyTooLarge Kind = "body_too_large"
	KindStatus       Kind = "status"
)

const (
	InvalidConfig   = KindConfig
	InvalidURL      = KindURL
	InvalidOrigin   = KindOrigin
	InvalidAddress  = KindAddress
	Redirect        = KindRedirect
	Network         = KindNetwork
	Timeout         = KindTimeout
	Canceled        = KindCanceled
	InvalidResponse = KindResponse
	BodyTooLarge    = KindBodyTooLarge
	Status          = KindStatus
)

type Error struct {
	Kind  Kind
	cause error
}

func (e *Error) Error() string {
	switch e.Kind {
	case KindConfig:
		return "securehttp: invalid configuration"
	case KindURL:
		return "securehttp: invalid request URL"
	case KindOrigin:
		return "securehttp: origin is not allowed"
	case KindAddress:
		return "securehttp: destination address is not allowed"
	case KindRedirect:
		return "securehttp: redirect rejected"
	case KindTimeout:
		return "securehttp: request timed out"
	case KindCanceled:
		return "securehttp: request canceled"
	case KindResponse:
		return "securehttp: invalid response"
	case KindBodyTooLarge:
		return "securehttp: response body exceeds limit"
	case KindStatus:
		return "securehttp: unsuccessful response status"
	default:
		return "securehttp: network request failed"
	}
}

func (e *Error) Unwrap() error { return e.cause }

type Config struct {
	AllowedOrigins         []string
	ConnectTimeout         time.Duration
	TLSHandshakeTimeout    time.Duration
	ResponseHeaderTimeout  time.Duration
	IdleConnTimeout        time.Duration
	OverallTimeout         time.Duration
	MaxRedirects           int
	MaxResponseBytes       int64
	MaxResponseHeaderBytes int64
	ExtraSensitiveHeaders  []string
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (r *Response) RequireSuccess() error {
	if r == nil || r.StatusCode < http.StatusOK || r.StatusCode >= http.StatusMultipleChoices {
		return &Error{Kind: KindStatus}
	}
	return nil
}

type Client struct {
	httpClient            *http.Client
	origins               map[string]struct{}
	maxBody               int64
	maxRedirects          int
	extraSensitiveHeaders []string
	resolver              func(context.Context, string) ([]netip.Addr, error)
	dialer                func(context.Context, string, string) (net.Conn, error)
}

const (
	defaultConnectTimeout               = 5 * time.Second
	defaultTLSHandshakeTimeout          = 5 * time.Second
	defaultResponseHeaderTimeout        = 10 * time.Second
	defaultIdleConnTimeout              = 30 * time.Second
	defaultOverallTimeout               = 30 * time.Second
	defaultMaxRedirects                 = 5
	defaultMaxResponseBytes       int64 = 8 << 20
	defaultMaxResponseHeaderBytes int64 = 32 << 10
)

func New(config Config) (*Client, error) {
	return newClient(config, nil, nil)
}

func newClient(config Config, resolver func(context.Context, string) ([]netip.Addr, error), dialer func(context.Context, string, string) (net.Conn, error)) (*Client, error) {
	if err := normalizeConfig(&config); err != nil {
		return nil, err
	}
	origins := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		canonical, err := canonicalOriginString(origin)
		if err != nil {
			return nil, &Error{Kind: KindConfig}
		}
		origins[canonical] = struct{}{}
	}
	if resolver == nil {
		resolver = func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}
	}
	if dialer == nil {
		d := &net.Dialer{Timeout: config.ConnectTimeout}
		dialer = d.DialContext
	}
	c := &Client{
		origins:               origins,
		maxBody:               config.MaxResponseBytes,
		maxRedirects:          config.MaxRedirects,
		extraSensitiveHeaders: config.ExtraSensitiveHeaders,
		resolver:              resolver,
		dialer:                dialer,
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            c.dialContext,
		DialTLSContext:         nil,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    config.TLSHandshakeTimeout,
		ResponseHeaderTimeout:  config.ResponseHeaderTimeout,
		IdleConnTimeout:        config.IdleConnTimeout,
		MaxResponseHeaderBytes: config.MaxResponseHeaderBytes,
	}
	c.httpClient = &http.Client{
		Transport:     transport,
		Timeout:       config.OverallTimeout,
		CheckRedirect: c.checkRedirect,
	}
	return c, nil
}

func normalizeConfig(config *Config) error {
	if len(config.AllowedOrigins) == 0 {
		return &Error{Kind: KindConfig}
	}
	if config.ConnectTimeout < 0 || config.TLSHandshakeTimeout < 0 || config.ResponseHeaderTimeout < 0 || config.IdleConnTimeout < 0 || config.OverallTimeout < 0 || config.MaxRedirects < 0 || config.MaxResponseBytes < 0 || config.MaxResponseBytes == math.MaxInt64 || config.MaxResponseHeaderBytes < 0 {
		return &Error{Kind: KindConfig}
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = defaultConnectTimeout
	}
	if config.TLSHandshakeTimeout == 0 {
		config.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	}
	if config.ResponseHeaderTimeout == 0 {
		config.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = defaultIdleConnTimeout
	}
	if config.OverallTimeout == 0 {
		config.OverallTimeout = defaultOverallTimeout
	}
	if config.MaxRedirects == 0 {
		config.MaxRedirects = defaultMaxRedirects
	}
	if config.MaxResponseBytes == 0 {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	if config.MaxResponseHeaderBytes == 0 {
		config.MaxResponseHeaderBytes = defaultMaxResponseHeaderBytes
	}
	extra, ok := normalizeSensitiveHeaders(config.ExtraSensitiveHeaders)
	if !ok {
		return &Error{Kind: KindConfig}
	}
	config.ExtraSensitiveHeaders = extra
	return nil
}

func (c *Client) Do(req *http.Request) (*Response, error) {
	if c == nil || c.httpClient == nil || req == nil {
		return nil, &Error{Kind: KindURL}
	}
	if err := c.validateURL(req.URL); err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Host = ""
	response, err := c.httpClient.Do(clone)
	if err != nil {
		return nil, sanitizeError(err)
	}
	if response == nil || response.Body == nil {
		return nil, &Error{Kind: KindResponse}
	}
	if response.ContentLength > c.maxBody {
		if response.Body.Close() != nil {
			return nil, &Error{Kind: KindNetwork}
		}
		return nil, &Error{Kind: KindBodyTooLarge}
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, c.maxBody+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		return nil, sanitizeError(readErr)
	}
	if int64(len(body)) > c.maxBody {
		return nil, &Error{Kind: KindBodyTooLarge}
	}
	if closeErr != nil {
		return nil, &Error{Kind: KindNetwork}
	}
	return &Response{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: append([]byte(nil), body...)}, nil
}

func (c *Client) checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > c.maxRedirects {
		return &Error{Kind: KindRedirect}
	}
	if err := c.validateURL(request.URL); err != nil {
		return &Error{Kind: KindRedirect}
	}
	if _, err := c.resolveAndValidate(request.Context(), request.URL.Hostname()); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return &Error{Kind: KindRedirect}
	}
	if len(via) > 0 && canonicalURLOrigin(via[len(via)-1].URL) != canonicalURLOrigin(request.URL) {
		stripSensitiveHeaders(request.Header, c.extraSensitiveHeaders)
	}
	return nil
}

func (c *Client) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || !validPort(port) {
		return nil, &Error{Kind: KindAddress}
	}
	addresses, err := c.resolveAndValidate(ctx, host)
	if err != nil {
		return nil, err
	}
	return c.dialer(ctx, network, net.JoinHostPort(addresses[0].String(), port))
}

func (c *Client) validateURL(value *url.URL) error {
	if value == nil || !strings.EqualFold(value.Scheme, "https") || value.Opaque != "" || value.User != nil || value.Fragment != "" || value.Host == "" {
		return &Error{Kind: KindURL}
	}
	origin, err := canonicalURLOriginChecked(value)
	if err != nil {
		return &Error{Kind: KindURL}
	}
	if _, ok := c.origins[origin]; !ok {
		return &Error{Kind: KindOrigin}
	}
	return nil
}

func (c *Client) resolveAndValidate(ctx context.Context, host string) ([]netip.Addr, error) {
	addresses, err := c.resolver(ctx, host)
	if err != nil {
		return nil, sanitizeError(err)
	}
	if len(addresses) == 0 {
		return nil, networkError(ctx)
	}
	normalized := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if blockedAddress(address) {
			return nil, &Error{Kind: KindAddress}
		}
		normalized = append(normalized, address)
	}
	return normalized, nil
}

func canonicalOriginString(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u == nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.User != nil {
		return "", errors.New("invalid origin")
	}
	return canonicalURLOriginChecked(u)
}

func canonicalURLOrigin(value *url.URL) string {
	origin, _ := canonicalURLOriginChecked(value)
	return origin
}

func canonicalURLOriginChecked(value *url.URL) (string, error) {
	if value == nil || !strings.EqualFold(value.Scheme, "https") || value.Host == "" || value.User != nil || value.Fragment != "" {
		return "", errors.New("invalid URL")
	}
	host := strings.TrimSuffix(strings.ToLower(value.Hostname()), ".")
	if host == "" || strings.HasSuffix(value.Host, ":") {
		return "", errors.New("invalid host")
	}
	if strings.Contains(host, ":") {
		address, err := netip.ParseAddr(host)
		if err != nil {
			return "", errors.New("invalid host")
		}
		host = "[" + address.String() + "]"
	} else if !validHostname(host) {
		return "", errors.New("invalid host")
	}
	port := value.Port()
	if port != "" && !validPort(port) {
		return "", errors.New("invalid port")
	}
	if port == "443" || port == "" {
		return "https://" + host, nil
	}
	return "https://" + host + ":" + port, nil
}

func validHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if r > 127 || !(r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

func validPort(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535
}

func blockedAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() || address.IsMulticast() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
		return true
	}
	if address.Is6() && !publicIPv6.Contains(address) {
		return true
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

var publicIPv6 = netip.MustParsePrefix("2000::/3")

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.169.254/32"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func sanitizeError(err error) error {
	if errors.Is(err, context.Canceled) {
		return &Error{Kind: KindCanceled, cause: context.Canceled}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: KindTimeout, cause: context.DeadlineExceeded}
	}
	var secure *Error
	if errors.As(err, &secure) {
		return &Error{Kind: secure.Kind, cause: secure.cause}
	}
	return &Error{Kind: KindNetwork}
}

func networkError(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return sanitizeError(ctx.Err())
	}
	return &Error{Kind: KindNetwork}
}

func RedactURL(value *url.URL) *url.URL {
	if value == nil {
		return nil
	}
	clone := *value
	clone.User = nil
	clone.Path = ""
	clone.RawPath = ""
	clone.RawQuery = ""
	clone.ForceQuery = false
	clone.Fragment = ""
	clone.RawFragment = ""
	return &clone
}

func RedactHeaders(headers http.Header, extraSensitiveHeaders ...string) http.Header {
	clone := headers.Clone()
	stripSensitiveHeaders(clone, normalizeHeaders(extraSensitiveHeaders))
	return clone
}

func normalizeHeaders(headers []string) []string {
	result, _ := normalizeSensitiveHeaders(headers)
	return result
}

func normalizeSensitiveHeaders(headers []string) ([]string, bool) {
	result := make([]string, 0, len(headers))
	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		if !validHeaderName(header) {
			return nil, false
		}
		header = http.CanonicalHeaderKey(header)
		if _, ok := seen[header]; !ok {
			seen[header] = struct{}{}
			result = append(result, header)
		}
	}
	return result, true
}

func validHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}

func stripSensitiveHeaders(headers http.Header, extra []string) {
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer", "Set-Cookie"} {
		headers.Del(header)
	}
	for _, header := range extra {
		headers.Del(header)
	}
}
