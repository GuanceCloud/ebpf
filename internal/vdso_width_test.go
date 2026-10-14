// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package internal

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"unsafe"
)

type tinyAuxvReader struct{ io.Reader }

func (r tinyAuxvReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}
func TestVDSOAddressNativeWidth(t *testing.T) {
	width := int(unsafe.Sizeof(uintptr(0)))
	entry := func(tag, value uint64) []byte {
		b := make([]byte, 2*width)
		if width == 4 {
			NativeEndian.PutUint32(b, uint32(tag))
			NativeEndian.PutUint32(b[4:], uint32(value))
		} else {
			NativeEndian.PutUint64(b, tag)
			NativeEndian.PutUint64(b[8:], value)
		}
		return b
	}
	blob := append(entry(6, 4096), entry(33, 0xf1234000)...)
	got, err := vdsoMemoryAddress(tinyAuxvReader{bytes.NewReader(blob)})
	if err != nil || got != 0xf1234000 {
		t.Fatalf("native pointer: %#x %v", got, err)
	}
	if _, err := vdsoMemoryAddress(bytes.NewReader(entry(0, 0))); !errors.Is(err, errAuxvNoVDSO) {
		t.Fatalf("terminator: %v", err)
	}
	if _, err := vdsoMemoryAddress(bytes.NewReader(entry(33, 0))); err == nil {
		t.Fatal("zero vDSO address accepted")
	}
	if _, err := vdsoMemoryAddress(bytes.NewReader(blob[:len(blob)-1])); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated native entry: %v", err)
	}
}
