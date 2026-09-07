package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bp0lr/dmut/defines"
)

// This regression uses real temporary files. It is opt-in locally and enabled
// explicitly in CI, outside -race, to avoid conflating instrumentation with the
// memory characteristic being tested. There is no corresponding application cap.
func TestOfflineMemoryBound(t *testing.T) {
	if os.Getenv("DMUT_TEST_LARGE_FILES") != "1" {
		t.Skip("set DMUT_TEST_LARGE_FILES=1 to run large-file memory regressions")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, workload := range []string{"dictionary", "output"} {
		t.Run(workload, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestOfflineMemoryHelper$", "-test.v")
			cmd.Env = append(os.Environ(), "DMUT_MEMORY_WORKLOAD="+workload, "GOGC=50", "GOMEMLIMIT=off")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("memory regression failed: %v\n%s", err, out)
			}
			t.Logf("%s", out)
		})
	}
}

func TestOfflineMemoryHelper(t *testing.T) {
	workload := os.Getenv("DMUT_MEMORY_WORKLOAD")
	if workload == "" {
		t.Skip("subprocess helper")
	}
	dir := t.TempDir()
	words, output := filepath.Join(dir, "words.txt"), filepath.Join(dir, "generated.txt")
	f, err := os.Create(words)
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriterSize(f, 64<<10)
	flags := defines.PermutationList{}
	domainCount := int64(32)
	switch workload {
	case "dictionary":
		// Five million entries would occupy a growing []string even when no
		// dictionary-driven rule is enabled. The snapshot must stay on disk.
		_, err = io.Copy(w, repeatLines("stage\n", 5_000_000))
		flags.AddToDomain, flags.AddSeparator = true, true
		domainCount = 1
	case "output":
		for i := 0; i < 50_000 && err == nil; i++ {
			_, err = fmt.Fprintf(w, "w%06d\n", i)
		}
	default:
		t.Fatalf("unknown workload %q", workload)
	}
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	a := newApplication(config{Dictionary: words, SaveTo: output, SaveOnly: true, Permutations: flags}, io.Discard, io.Discard)
	peak, err := measureHeap(func() error {
		return a.run(context.Background(), repeatLines("test.example.com\n", domainCount))
	})
	runtime.KeepAlive(a)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if workload == "output" && info.Size() < 192<<20 {
		t.Fatalf("output fixture was too small: %d bytes", info.Size())
	}
	// Generous allowance for the runtime and transient record allocations; the
	// old collecting path retained substantially more than this in either case.
	if peak > 96<<20 {
		t.Fatalf("heap grew with input/output: %.1f MiB", float64(peak)/(1<<20))
	}
	assertOfflineTempsRemoved(t, dir)
	t.Logf("workload=%s output_bytes=%d peak_heap_mib=%.1f", workload, info.Size(), float64(peak)/(1<<20))
}

type repeatingLines struct {
	block     []byte
	offset    int
	remaining int64
}

func repeatLines(line string, count int64) *repeatingLines {
	return &repeatingLines{block: bytes.Repeat([]byte(line), 4096), remaining: int64(len(line)) * count}
}

func (r *repeatingLines) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	filled := 0
	for filled < int(n) {
		copied := copy(p[filled:int(n)], r.block[r.offset:])
		filled += copied
		r.offset = (r.offset + copied) % len(r.block)
	}
	r.remaining -= n
	return int(n), nil
}

func measureHeap(run func() error) (uint64, error) {
	runtime.GC()
	var peak atomic.Uint64
	sample := func() {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		for previous := peak.Load(); stats.HeapAlloc > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, stats.HeapAlloc) {
				break
			}
		}
	}
	sample()
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sample()
			case <-stop:
				return
			}
		}
	}()
	err := run()
	sample()
	close(stop)
	<-done
	return peak.Load(), err
}
