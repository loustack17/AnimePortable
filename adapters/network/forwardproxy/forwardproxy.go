// SPDX-License-Identifier: MPL-2.0

// Package forwardproxy holds the one forward-proxy setting every secure HTTP client of the
// app consults, so a change in settings reaches new connections without a restart. The
// composition root creates the Source and hands its Address method to each client.
package forwardproxy

import (
	"strings"
	"sync"

	"animeportable/core"
)

const wininetDefaultPort = "80"

// SystemProxy is the Windows (WinINet) proxy setting of the current user.
type SystemProxy struct {
	// Enabled is ProxyEnable; Server is the raw ProxyServer string.
	Enabled bool
	Server  string
	// AutoConfig is true when a PAC script (AutoConfigURL) is set, which the app cannot run.
	AutoConfig bool
}

// SystemReader reads the Windows system proxy.
type SystemReader func() (SystemProxy, error)

// Resolution is the outcome of resolving the current mode for one new connection.
type Resolution struct {
	// Address is the proxy as host:port, or "" for a direct connection.
	Address string
	// SystemUnusable is true in system mode when Windows names a proxy the app cannot use
	// (SOCKS only, PAC script, unreadable setting): the connection is direct, and callers
	// can say so.
	SystemUnusable bool
}

// Source resolves the current proxy mode to a host:port, or "" for a direct connection.
type Source struct {
	mu     sync.RWMutex
	mode   core.ProxyMode
	manual string
	system SystemReader
}

// New returns a Source that reads the operating system proxy in system mode.
func New() *Source { return NewWithReader(readSystem) }

// NewWithReader returns a Source with a replaceable system proxy reader, for tests.
func NewWithReader(system SystemReader) *Source { return &Source{system: system} }

// Preview returns a separate Source with the given settings that reads the system proxy the
// same way, so a setting can be tried without changing the one in use.
func (source *Source) Preview(mode core.ProxyMode, manual string) *Source {
	preview := NewWithReader(source.system)
	preview.Set(mode, manual)
	return preview
}

// Set replaces the mode and the manual address. The Unspecified mode means no proxy.
func (source *Source) Set(mode core.ProxyMode, manual string) {
	source.mu.Lock()
	source.mode, source.manual = mode, manual
	source.mu.Unlock()
}

// Address is the proxy to use for a new connection: see Resolve.
func (source *Source) Address() string { return source.Resolve().Address }

// Resolve picks the proxy for a new connection. A manual address that is empty or invalid
// means direct. In system mode, a Windows proxy that is off means direct; one that is on but
// unusable (SOCKS only, PAC script, unreadable) also means direct and sets SystemUnusable.
// When both a PAC script and a static proxy are set, the static proxy is used: the app does not
// run PAC scripts, so the script is only reported (SystemUnusable) when there is no static proxy.
func (source *Source) Resolve() Resolution {
	source.mu.RLock()
	mode, manual, system := source.mode, source.manual, source.system
	source.mu.RUnlock()
	switch mode {
	case core.ProxyModeManual:
		if manual != "" && core.ValidProxyAddress(manual) {
			return Resolution{Address: manual}
		}
	case core.ProxyModeSystem:
		if system == nil {
			return Resolution{}
		}
		setting, err := system()
		if err != nil {
			return Resolution{SystemUnusable: true}
		}
		if address := systemAddress(setting); address != "" {
			return Resolution{Address: address}
		}
		return Resolution{SystemUnusable: setting.AutoConfig || setting.Enabled && strings.TrimSpace(setting.Server) != ""}
	}
	return Resolution{}
}

// systemAddress picks the proxy out of a WinINet ProxyServer value: either one entry or a
// list "http=entry;https=entry;...", of which https wins over http. An entry is host:port,
// optionally with an http:// prefix; a host without a port means port 80, as in WinINet.
func systemAddress(setting SystemProxy) string {
	server := strings.TrimSpace(setting.Server)
	if !setting.Enabled || server == "" {
		return ""
	}
	entry := server
	if strings.Contains(server, "=") {
		entry = ""
		for _, scheme := range []string{"https", "http"} {
			if value, ok := listEntry(server, scheme); ok {
				entry = value
				break
			}
		}
	}
	return entryAddress(entry)
}

func entryAddress(entry string) string {
	entry = strings.TrimSpace(entry)
	if len(entry) >= len("http://") && strings.EqualFold(entry[:len("http://")], "http://") {
		entry = entry[len("http://"):]
	}
	if entry == "" {
		return ""
	}
	if !hasPort(entry) {
		entry += ":" + wininetDefaultPort
	}
	if !core.ValidProxyAddress(entry) {
		return ""
	}
	return entry
}

// hasPort reports whether entry ends in ":port". A bracketed IPv6 literal without a port
// ends in "]".
func hasPort(entry string) bool {
	if strings.HasSuffix(entry, "]") {
		return false
	}
	return strings.Count(entry, ":") == 1 || strings.HasPrefix(entry, "[") && strings.Contains(entry, "]:")
}

func listEntry(server, scheme string) (string, bool) {
	for _, entry := range strings.Split(server, ";") {
		name, value, found := strings.Cut(strings.TrimSpace(entry), "=")
		if found && strings.EqualFold(strings.TrimSpace(name), scheme) {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}
