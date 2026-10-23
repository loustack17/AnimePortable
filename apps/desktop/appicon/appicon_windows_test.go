//go:build windows && cgo

// SPDX-License-Identifier: MPL-2.0

package appicon

import "testing"

func TestEveryEmbeddedSizeLoads(t *testing.T) {
	loadOnce.Do(load)
	if len(images) != 3 {
		t.Fatalf("loaded %d icon sizes, want 3", len(images))
	}
	for index, want := range []int{16, 32, 48} {
		if images[index].W() != want || images[index].H() != want {
			t.Errorf("size %d is %dx%d", want, images[index].W(), images[index].H())
		}
	}
}
