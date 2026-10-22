// SPDX-License-Identifier: MPL-2.0

package core

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
)

const maxProxyAddressBytes = 255

// ValidProxyAddress reports whether address may be stored as the manual proxy. Empty is
// allowed (not set). Otherwise it must be host:port with a DNS name or IP literal (IPv6 in
// brackets) and a port of 1-65535: no scheme, credentials, path, query, spaces or control
// characters.
func ValidProxyAddress(address string) bool {
	if address == "" {
		return true
	}
	if len(address) > maxProxyAddressBytes {
		return false
	}
	for index := 0; index < len(address); index++ {
		if character := address[index]; character <= ' ' || character >= 0x7f {
			return false
		}
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if number, err := strconv.ParseUint(port, 10, 16); err != nil || number == 0 {
		return false
	}
	if address[0] == '[' {
		ip, err := netip.ParseAddr(host)
		return err == nil && ip.Is6() && ip.Zone() == ""
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Is4()
	}
	return validDNSName(host)
}

// validDNSName checks the letter-digit-hyphen syntax of RFC 1123. An all-numeric last label
// is refused so a malformed IPv4 literal such as 300.1.1.1 is not taken for a name.
func validDNSName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for index := 0; index < len(label); index++ {
			character := label[index]
			isAlphanumeric := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
			if !isAlphanumeric && character != '-' {
				return false
			}
		}
	}
	last := labels[len(labels)-1]
	return strings.Trim(last, "0123456789") != ""
}
