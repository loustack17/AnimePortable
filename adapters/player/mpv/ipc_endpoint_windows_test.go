//go:build windows

// SPDX-License-Identifier: MPL-2.0

package mpv

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCurrentUserSecurityDescriptor(t *testing.T) {
	descriptor, err := currentUserSecurityDescriptor()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sddl := descriptor.String()
	if !strings.Contains(sddl, user.User.Sid.String()) || strings.Contains(sddl, ";;;WD)") || strings.Contains(sddl, ";;;AU)") {
		t.Fatalf("pipe DACL is not current-user-only: %s", sddl)
	}
}
