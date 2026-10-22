//go:build windows && cgo

// SPDX-License-Identifier: MPL-2.0

package fltkplayer

import (
	"testing"

	"animeportable/apps/desktop/uitext"
)

// The menu must hold its own copy: writing the next language's names into the table's array
// changed every later Traditional Chinese label.
func TestSetMenuItemsNeverWritesIntoTheTextTable(t *testing.T) {
	view := keyboardTestView(Callbacks{})
	defer view.freeLabelBuffers()
	// What NewWindow once did: the menu pointed straight into the table.
	view.menuItems = uitext.For("zh-TW").Sections[:]
	view.SetMenuItems(uitext.For("en").Sections[:])
	if got := uitext.For("zh-TW").Sections[0]; got != "首頁" {
		t.Fatalf("Traditional Chinese Home is now %q", got)
	}
	if view.menuItems[0] != "Home" {
		t.Fatalf("menu = %v, want the English names", view.menuItems)
	}
}
