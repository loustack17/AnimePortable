// SPDX-License-Identifier: MPL-2.0

package core

import (
	"strings"
	"testing"
)

func TestValidProxyAddress(t *testing.T) {
	valid := []string{
		"", "proxy.example:8080", "localhost:1", "a-b.example.com:65535", "127.0.0.1:3128",
		"[::1]:3128", "[2001:db8::1]:80", "Proxy.Example:00080", "xn--nxasmq6b.example:8080",
	}
	for _, address := range valid {
		if !ValidProxyAddress(address) {
			t.Errorf("ValidProxyAddress(%q) = false, want true", address)
		}
	}
	invalid := map[string]string{
		"no port":          "proxy.example",
		"empty port":       "proxy.example:",
		"port zero":        "proxy.example:0",
		"port too high":    "proxy.example:65536",
		"port sign":        "proxy.example:+80",
		"port text":        "proxy.example:http",
		"scheme":           "http://proxy.example:8080",
		"userinfo":         "user:pass@proxy.example:8080",
		"path":             "proxy.example:8080/x",
		"query":            "proxy.example:8080?x=1",
		"space":            "proxy example:8080",
		"leading space":    " proxy.example:8080",
		"control":          "proxy.example\n:8080",
		"non ascii":        "代理.example:8080",
		"bare ipv6":        "::1:8080",
		"ipv6 no brackets": "2001:db8::1:80",
		"brackets on name": "[proxy.example]:80",
		"brackets on ipv4": "[127.0.0.1]:80",
		"ipv6 zone":        "[fe80::1%eth0]:80",
		"empty host":       ":8080",
		"empty label":      "a..b:80",
		"leading hyphen":   "-a.example:80",
		"trailing hyphen":  "a-.example:80",
		"underscore":       "a_b.example:80",
		"bad ipv4":         "300.1.1.1:80",
		"numeric host":     "12345:80",
		"label too long":   strings.Repeat("a", 64) + ".example:80",
		"too long":         strings.Repeat("a.", 130) + "example:80",
	}
	for name, address := range invalid {
		if ValidProxyAddress(address) {
			t.Errorf("%s: ValidProxyAddress(%q) = true, want false", name, address)
		}
	}
}
