// SPDX-License-Identifier: MPL-2.0

package forwardproxy

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// readSystem reads the WinINet proxy of the current user. It never writes. A missing
// ProxyEnable or ProxyServer value means the setting is off, not an error.
func readSystem() (SystemProxy, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return SystemProxy{}, err
	}
	defer key.Close()
	var setting SystemProxy
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return SystemProxy{}, err
	}
	setting.Enabled = err == nil && enabled != 0
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return SystemProxy{}, err
	}
	setting.Server = server
	script, _, err := key.GetStringValue("AutoConfigURL")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return SystemProxy{}, err
	}
	setting.AutoConfig = script != ""
	return setting, nil
}
