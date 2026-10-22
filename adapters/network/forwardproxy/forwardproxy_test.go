// SPDX-License-Identifier: MPL-2.0

package forwardproxy

import (
	"errors"
	"testing"

	"animeportable/core"
)

func TestSystemAddress(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		server  string
		want    string
	}{
		{"plain host and port", true, "proxy.example:8080", "proxy.example:8080"},
		{"plain IPv4", true, "127.0.0.1:7890", "127.0.0.1:7890"},
		{"surrounding spaces", true, "  proxy.example:8080 ", "proxy.example:8080"},
		{"disabled", false, "proxy.example:8080", ""},
		{"empty", true, "", ""},
		{"https entry wins", true, "http=a.example:80;https=b.example:443;ftp=c.example:21", "b.example:443"},
		{"https entry wins when listed first", true, "https=b.example:443;http=a.example:80", "b.example:443"},
		{"http entry when no https", true, "ftp=c.example:21;http=a.example:80", "a.example:80"},
		{"entry names are case-insensitive", true, "HTTPS=b.example:443", "b.example:443"},
		{"only unsupported entries", true, "socks=127.0.0.1:1080", ""},
		{"http scheme", true, "http://proxy.example:8080", "proxy.example:8080"},
		{"http scheme in a list", true, "http=http://a.example:8080;https=http://b.example:8443", "b.example:8443"},
		{"upper-case http scheme", true, "HTTP://proxy.example:8080", "proxy.example:8080"},
		{"https scheme", true, "https://proxy.example:8080", ""},
		{"https scheme in list value", true, "https=https://proxy.example:8080", ""},
		{"http scheme with a path", true, "http://proxy.example:8080/pac", ""},
		{"http scheme without a host", true, "http://", ""},
		{"host without port means 80", true, "proxy.example", "proxy.example:80"},
		{"IPv4 without port", true, "127.0.0.1", "127.0.0.1:80"},
		{"http scheme and no port", true, "http://proxy.example", "proxy.example:80"},
		{"list entry without port", true, "http=proxy.example", "proxy.example:80"},
		{"IPv6 with port", true, "[::1]:8080", "[::1]:8080"},
		{"IPv6 without port", true, "[::1]", "[::1]:80"},
		{"bare IPv6", true, "::1", ""},
		{"empty port", true, "proxy.example:", ""},
		{"port out of range", true, "proxy.example:70000", ""},
		{"https entry invalid does not fall back", true, "http=a.example:80;https=bad_host", ""},
		{"user information", true, "user@proxy.example:8080", ""},
		{"path", true, "proxy.example:8080/pac", ""},
		{"PAC style value", true, "<local>", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := systemAddress(SystemProxy{Enabled: test.enabled, Server: test.server}); got != test.want {
				t.Fatalf("systemAddress(%v, %q) = %q, want %q", test.enabled, test.server, got, test.want)
			}
		})
	}
}

func TestSourceModes(t *testing.T) {
	enabled, server, readErr := true, "system.example:3128", error(nil)
	source := NewWithReader(func() (SystemProxy, error) { return SystemProxy{Enabled: enabled, Server: server}, readErr })
	if source.Address() != "" {
		t.Fatal("a new source must be direct")
	}
	source.Set(core.ProxyModeNone, "manual.example:8080")
	if source.Address() != "" {
		t.Fatal("none must be direct")
	}
	source.Set(core.ProxyModeManual, "manual.example:8080")
	if got := source.Address(); got != "manual.example:8080" {
		t.Fatalf("manual = %q", got)
	}
	for _, invalid := range []string{"", "http://manual.example:8080", "user@manual.example:8080", "manual.example", "manual.example:8080/x"} {
		source.Set(core.ProxyModeManual, invalid)
		if got := source.Address(); got != "" {
			t.Fatalf("manual %q = %q, want direct", invalid, got)
		}
	}
	source.Set(core.ProxyModeSystem, "manual.example:8080")
	if got := source.Address(); got != "system.example:3128" {
		t.Fatalf("system = %q", got)
	}
	// The system setting is read per call, so a change in Windows applies to new requests.
	server = "other.example:8888"
	if got := source.Address(); got != "other.example:8888" {
		t.Fatalf("system after change = %q", got)
	}
	enabled = false
	if got := source.Address(); got != "" {
		t.Fatalf("system disabled = %q", got)
	}
	enabled, readErr = true, errors.New("registry unreadable")
	if got := source.Address(); got != "" {
		t.Fatalf("system unreadable = %q", got)
	}
	source.Set(core.ProxyModeUnspecified, "manual.example:8080")
	if got := source.Address(); got != "" {
		t.Fatalf("unspecified = %q", got)
	}
}

func TestSourceWithoutReaderIsDirectInSystemMode(t *testing.T) {
	source := NewWithReader(nil)
	source.Set(core.ProxyModeSystem, "")
	if source.Address() != "" {
		t.Fatal("system mode without a reader must be direct")
	}
}

func TestResolveSaysWhenTheSystemProxyIsUnusable(t *testing.T) {
	tests := []struct {
		name    string
		setting SystemProxy
		err     error
		want    Resolution
	}{
		{"usable", SystemProxy{Enabled: true, Server: "p.example:8080"}, nil, Resolution{Address: "p.example:8080"}},
		{"off", SystemProxy{Server: "p.example:8080"}, nil, Resolution{}},
		{"nothing set", SystemProxy{}, nil, Resolution{}},
		{"SOCKS only", SystemProxy{Enabled: true, Server: "socks=127.0.0.1:1080"}, nil, Resolution{SystemUnusable: true}},
		{"PAC script", SystemProxy{AutoConfig: true}, nil, Resolution{SystemUnusable: true}},
		{"PAC script and a usable server", SystemProxy{Enabled: true, Server: "p.example:8080", AutoConfig: true}, nil, Resolution{Address: "p.example:8080"}},
		{"unparseable server", SystemProxy{Enabled: true, Server: "<local>"}, nil, Resolution{SystemUnusable: true}},
		{"read error", SystemProxy{}, errors.New("registry"), Resolution{SystemUnusable: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := NewWithReader(func() (SystemProxy, error) { return test.setting, test.err })
			source.Set(core.ProxyModeSystem, "")
			if got := source.Resolve(); got != test.want {
				t.Fatalf("Resolve() = %+v, want %+v", got, test.want)
			}
		})
	}
	// Only system mode can be unusable.
	source := NewWithReader(func() (SystemProxy, error) { return SystemProxy{}, errors.New("registry") })
	source.Set(core.ProxyModeManual, "")
	if got := source.Resolve(); got != (Resolution{}) {
		t.Fatalf("manual = %+v", got)
	}
}

func TestPreviewReadsTheSameSystemProxyWithoutChangingTheSource(t *testing.T) {
	source := NewWithReader(func() (SystemProxy, error) { return SystemProxy{Enabled: true, Server: "p.example:8080"}, nil })
	source.Set(core.ProxyModeManual, "m.example:1")
	preview := source.Preview(core.ProxyModeSystem, "")
	if got := preview.Address(); got != "p.example:8080" {
		t.Fatalf("preview = %q", got)
	}
	if got := source.Address(); got != "m.example:1" {
		t.Fatalf("source changed to %q", got)
	}
}
