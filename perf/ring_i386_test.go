//go:build 386

package perf

import (
	"io"
	"testing"

	"github.com/cilium/ebpf/internal/unix"
)

func TestForwardReaderNativeCounterWrap(t *testing.T) {
	meta := &unix.PerfEventMmapPage{Data_head: 16, Data_tail: 1<<32 - 16}
	rr := newForwardReader(meta, make([]byte, 64))
	rr.loadHead()
	for i := 0; i < 2; i++ {
		buf := make([]byte, 16)
		n, err := rr.Read(buf)
		if n != 16 || (i == 1 && err != io.EOF) {
			t.Fatalf("wrapped read %d made no progress: n=%d err=%v", i, n, err)
		}
	}
	rr.writeTail()
	rr.loadHead()
	if n, err := rr.Read(make([]byte, 8)); n != 0 || err != io.EOF || rr.tail != 1<<32+16 {
		t.Fatalf("wrapped empty ring spins or replays: n=%d err=%v tail=%d", n, err, rr.tail)
	}
}

func TestReverseReaderNativeCounterWrap(t *testing.T) {
	meta := &unix.PerfEventMmapPage{Data_head: 16}
	rr := newReverseReader(meta, make([]byte, 64))
	meta.Data_head = 1<<32 - 16
	rr.loadHead()
	for i := 0; i < 2; i++ {
		n, err := rr.Read(make([]byte, 16))
		if n != 16 || (i == 1 && err != io.EOF) {
			t.Fatalf("reverse counter wrap lost data: n=%d err=%v", n, err)
		}
	}
}
