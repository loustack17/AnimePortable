//go:build resourcepass && windows

// SPDX-License-Identifier: MPL-2.0

// Package resourcepass measures whether repeating an operation makes goroutines or memory keep
// growing (Loop 38b, docs/loops/LOOP38.md). It is built only with the resourcepass tag, so it
// never reaches the app or the normal test run. Windows only, like the app.
package resourcepass

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

const (
	// Warmup cycles fill pools, caches and lazily started goroutines before the baseline.
	Warmup = 20
	// Cycles is the default; a scenario or RESOURCEPASS_CYCLES may ask for more, never fewer.
	Cycles = 200
	every  = 20

	goroutineSlack   = 2
	maxHeapGrowth    = 1 << 20
	maxPrivateGrowth = 8 << 20
	settleTimeout    = 5 * time.Second
)

// Sample is one measurement after a number of cycles.
type Sample struct {
	Cycle      int
	Goroutines int
	HeapInuse  uint64
	Private    uint64
}

// Run measures the goroutines in use before setup, warms up, then repeats cycle at least
// minimum times (Cycles when 0; a multiple of 20), sampling every 20, and finally runs teardown. It fails the test when goroutines do not come
// back to their count before setup, or the samples after warm-up grow beyond the Loop 38 limits.
// Samples go to RESOURCEPASS_OUT/<name>.csv when that folder is set.
func Run(t *testing.T, name string, minimum int, setup func(), cycle func(i int), teardown func()) []Sample {
	t.Helper()
	count := cycles(minimum)
	before := settledGoroutines(0)
	if setup != nil {
		setup()
	}
	for i := range Warmup {
		cycle(i)
	}
	baseline := settledGoroutines(0)
	samples := []Sample{measure(0, baseline)}
	for i := 1; i <= count; i++ {
		cycle(Warmup + i)
		if i%every == 0 {
			samples = append(samples, measure(i, baseline))
		}
	}
	if teardown != nil {
		teardown()
	}
	after := settledGoroutines(before)
	write(t, name, samples, before, after)
	for _, problem := range Problems(samples, baseline, before, after) {
		t.Error(name + ": " + problem)
	}
	return samples
}

// Problems lists every Loop 38 limit the samples break.
func Problems(samples []Sample, baseline, before, after int) []string {
	var problems []string
	last := samples[len(samples)-1]
	if last.Goroutines > baseline+goroutineSlack {
		problems = append(problems, fmt.Sprintf("goroutines grew from %d to %d", baseline, last.Goroutines))
	}
	if after > before+goroutineSlack {
		problems = append(problems, fmt.Sprintf("goroutines after teardown %d, before setup %d", after, before))
	}
	heap := func(sample Sample) uint64 { return sample.HeapInuse }
	private := func(sample Sample) uint64 { return sample.Private }
	if growth := growth(samples, heap); growth > maxHeapGrowth {
		problems = append(problems, fmt.Sprintf("heap in use grew %.0f bytes between the first and last five samples", growth))
	}
	rising := true
	for i := 1; i < len(samples); i++ {
		rising = rising && samples[i].HeapInuse > samples[i-1].HeapInuse
	}
	if rising {
		problems = append(problems, "heap in use rose at every sample")
	}
	// Private bytes swing by megabytes between samples (C heaps), so single samples are not
	// compared; the first and last five are, as for the heap.
	if growth := growth(samples, private); growth > maxPrivateGrowth {
		problems = append(problems, fmt.Sprintf("private bytes grew %.0f bytes between the first and last five samples", growth))
	}
	for _, sample := range samples {
		if sample.Private == 0 {
			problems = append(problems, "private bytes could not be read")
			break
		}
	}
	return problems
}

func cycles(minimum int) int {
	count := max(minimum-minimum%every, Cycles)
	if value, err := strconv.Atoi(os.Getenv("RESOURCEPASS_CYCLES")); err == nil && value > count && value%every == 0 {
		return value
	}
	return count
}

// growth is the mean of the last five samples minus the mean of the first five.
func growth(samples []Sample, value func(Sample) uint64) float64 {
	mean := func(window []Sample) float64 {
		var total float64
		for _, sample := range window {
			total += float64(value(sample))
		}
		return total / float64(len(window))
	}
	return mean(samples[len(samples)-5:]) - mean(samples[:5])
}

// measure collects garbage first so the heap figure is what is still referenced. That is a
// leak check only; the idle memory target is measured on the real app without forced collection.
func measure(cycle, baseline int) Sample {
	goroutines := settledGoroutines(baseline)
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return Sample{Cycle: cycle, Goroutines: goroutines, HeapInuse: stats.HeapInuse, Private: privateBytes()}
}

// settledGoroutines waits up to settleTimeout for goroutines that are still finishing a cycle
// to exit (target 0: no wait), so a slow exit is not read as growth, then returns the lowest
// count seen over 200 ms. Every sample is taken the same way, so samples compare.
func settledGoroutines(target int) int {
	deadline := time.Now().Add(settleTimeout)
	count := runtime.NumGoroutine()
	for target > 0 && count > target+goroutineSlack && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		count = runtime.NumGoroutine()
	}
	for end := time.Now().Add(200 * time.Millisecond); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		count = min(count, runtime.NumGoroutine())
	}
	return count
}

func write(t *testing.T, name string, samples []Sample, before, after int) {
	t.Helper()
	for _, sample := range samples {
		t.Logf("%s cycle=%d goroutines=%d heap_inuse=%d private=%d", name, sample.Cycle, sample.Goroutines, sample.HeapInuse, sample.Private)
	}
	t.Logf("%s goroutines before setup=%d after teardown=%d", name, before, after)
	directory := os.Getenv("RESOURCEPASS_OUT")
	if directory == "" {
		return
	}
	file, err := os.Create(filepath.Join(directory, name+".csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	out := csv.NewWriter(file)
	_ = out.Write([]string{"cycle", "goroutines", "heap_inuse", "private_bytes"})
	for _, sample := range samples {
		_ = out.Write([]string{strconv.Itoa(sample.Cycle), strconv.Itoa(sample.Goroutines), strconv.FormatUint(sample.HeapInuse, 10), strconv.FormatUint(sample.Private, 10)})
	}
	_ = out.Write([]string{"before_setup", strconv.Itoa(before), "", ""})
	_ = out.Write([]string{"after_teardown", strconv.Itoa(after), "", ""})
	out.Flush()
	if err := out.Error(); err != nil {
		t.Fatal(err)
	}
}
