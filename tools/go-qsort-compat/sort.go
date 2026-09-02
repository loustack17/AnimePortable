// SPDX-License-Identifier: MPL-2.0

package qsort

import "sort"

func Slice[T any](values []T, compare func(T, T) int) {
	sort.Slice(values, func(i, j int) bool {
		return compare(values[i], values[j]) < 0
	})
}
