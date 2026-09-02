// SPDX-License-Identifier: MPL-2.0

package qsort

import "testing"

func TestSliceSortsUsingComparator(t *testing.T) {
	values := []int{4, 1, 3, 2}
	Slice(values, func(left, right int) int { return left - right })

	for index, value := range []int{1, 2, 3, 4} {
		if values[index] != value {
			t.Fatalf("values[%d] = %d, want %d", index, values[index], value)
		}
	}
}
