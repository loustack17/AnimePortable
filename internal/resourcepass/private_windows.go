//go:build resourcepass && windows

// SPDX-License-Identifier: MPL-2.0

package resourcepass

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// processMemoryCountersEx is PROCESS_MEMORY_COUNTERS_EX.
type processMemoryCountersEx struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
	privateUsage               uintptr
}

// privateBytes is the process's private commit (Task Manager's "commit size"), which also
// counts memory held by C code such as libmpv.
func privateBytes() uint64 {
	var counters processMemoryCountersEx
	counters.cb = uint32(unsafe.Sizeof(counters))
	ok, _, _ := getProcessMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&counters)), uintptr(counters.cb))
	if ok == 0 {
		return 0
	}
	return uint64(counters.privateUsage)
}
