//go:build linux

package perf

import (
	"os"
	"testing"
	"unsafe"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/unix"
	linux "golang.org/x/sys/unix"
)

func TestDetachedPerfEnabledTime(t *testing.T) {
	var fds [2]int
	if err := linux.Pipe2(fds[:], linux.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	defer linux.Close(fds[0])
	defer linux.Close(fds[1])
	ring := &perfEventRing{fd: fds[0]}
	for i, enabled := range []uint64{10, 20, 20} {
		var value [16]byte
		internal.NativeEndian.PutUint64(value[8:], enabled)
		if n, err := linux.Write(fds[1], value[:]); err != nil || n != len(value) {
			t.Fatalf("write: %d %v", n, err)
		}
		detached, err := ring.detached()
		if err != nil || detached != (i == 2) {
			t.Fatalf("enabled time %d: detached=%v err=%v", enabled, detached, err)
		}
	}
}

func TestReplacementDrainsRetiredBufferBeforeClosing(t *testing.T) {
	makeRing := func(payload string) *perfEventRing {
		t.Helper()
		memory, err := linux.Mmap(-1, 0, 2*os.Getpagesize(), linux.PROT_READ|linux.PROT_WRITE, linux.MAP_PRIVATE|linux.MAP_ANON)
		if err != nil {
			t.Fatal(err)
		}
		meta := (*unix.PerfEventMmapPage)(unsafe.Pointer(&memory[0]))
		data := memory[os.Getpagesize():]
		internal.NativeEndian.PutUint32(data[:4], unix.PERF_RECORD_SAMPLE)
		internal.NativeEndian.PutUint16(data[6:8], uint16(12+len(payload)))
		internal.NativeEndian.PutUint32(data[8:12], uint32(len(payload)))
		copy(data[12:], payload)
		meta.Data_head = uint64(12 + len(payload))
		ring := &perfEventRing{fd: -1, cpu: 3, mmap: memory, ringReader: newForwardReader(meta, data)}
		ring.loadHead()
		return ring
	}
	old, current := makeRing("before-offline"), makeRing("after-online")
	defer old.Close()
	defer current.Close()
	old.retired = true
	reader := &Reader{rings: []*perfEventRing{current}, epollRings: []*perfEventRing{current, old}, retiredRings: map[*perfEventRing]bool{old: true}, eventHeader: make([]byte, perfEventHeaderSize)}
	var rec Record
	for _, want := range []string{"before-offline", "after-online"} {
		if err := reader.ReadInto(&rec); err != nil || string(rec.RawSample) != want || rec.CPU != 3 {
			t.Fatalf("unread sample lost on CPU replacement: %+v %v", rec, err)
		}
	}
	if old.mmap != nil || len(reader.retiredRings) != 0 || current.mmap == nil {
		t.Fatal("retired buffer leaked or current buffer closed")
	}
}
