package frame_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// benchSizes are typical automerge sync frames (small) up to a large
// document snapshot.
var benchSizes = []int{256, 16 << 10, 64 << 10, 1 << 20}

func BenchmarkWrite(b *testing.B) {
	for _, size := range benchSizes {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			payload := make([]byte, size)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				if err := frame.Write(io.Discard, payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRead(b *testing.B) {
	for _, size := range benchSizes {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			var buf bytes.Buffer
			if err := frame.Write(&buf, make([]byte, size)); err != nil {
				b.Fatal(err)
			}
			wire := buf.Bytes()
			r := bytes.NewReader(wire)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				r.Reset(wire)
				if _, err := frame.Read(r, transport.MaxFrame); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
