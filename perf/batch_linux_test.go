//go:build linux

// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package perf_test

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/perf"
	"golang.org/x/sys/unix"
)

func perfBatchProgram(tb testing.TB, mp *ebpf.Map, events int) *ebpf.Program {
	tb.Helper()
	ins := asm.Instructions{asm.Mov.Reg(asm.R6, asm.R1), asm.StoreImm(asm.RFP, -8, 1, asm.DWord)}
	for i := 0; i < events; i++ {
		ins = append(ins, asm.Mov.Reg(asm.R1, asm.R6), asm.LoadMapPtr(asm.R2, mp.FD()), asm.LoadImm(asm.R3, int64(0xffffffff), asm.DWord), asm.Mov.Reg(asm.R4, asm.RFP), asm.Add.Imm(asm.R4, -8), asm.Mov.Imm(asm.R5, 8), asm.FnPerfEventOutput.Call())
	}
	ins = append(ins, asm.Mov.Imm(asm.R0, 0), asm.Return())
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{Name: "batch_fixture", Type: ebpf.SchedCLS, License: "GPL", Instructions: ins})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := prog.Close(); err != nil {
			tb.Error(err)
		}
	})
	return prog
}
func TestReadBatchNativeDrainAndSparseTail(t *testing.T) {
	mp, err := ebpf.NewMap(&ebpf.MapSpec{Type: ebpf.PerfEventArray, KeySize: 4, ValueSize: 4})
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		t.Skip("isolated perf map requires BPF privilege")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer mp.Close() //nolint:errcheck
	reader, err := perf.NewReaderWithOptions(mp, 64<<10, perf.ReaderOptions{CPURefreshInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close() //nolint:errcheck
	prog := perfBatchProgram(t, mp, 100)
	for i := 0; i < 10; i++ {
		if _, _, err = prog.Test(make([]byte, 64)); err != nil {
			t.Fatal(err)
		}
	}
	reader.SetDeadline(time.Now().Add(2 * time.Second))
	var records [16]perf.Record
	count := 0
	for count < 1000 {
		n, err := reader.ReadBatch(records[:])
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			if records[i].LostSamples != 0 || len(records[i].RawSample) < 8 || binary.LittleEndian.Uint64(records[i].RawSample) != 1 {
				t.Fatal("batch corrupted or lost sample")
			}
		}
		count += n
	}
	if count != 1000 {
		t.Fatal("extra batch records")
	}
	sparse := perfBatchProgram(t, mp, 1)
	if _, _, err = sparse.Test(make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	n, err := reader.ReadBatch(records[:])
	if err != nil || n != 1 {
		t.Fatal("sparse final record waited for a full batch", n, err)
	}
	closed := make(chan error, 1)
	go func() { _, err := reader.ReadBatch(records[:]); closed <- err }()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if !errors.Is(err, perf.ErrClosed) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not interrupt blocked batch")
	}
}
