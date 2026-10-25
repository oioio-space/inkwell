package blob_test

import (
	"bytes"
	"testing"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

const benchSize = 16 << 20

// BenchmarkPut measures storing a file: copy, hash and outboard tree.
func BenchmarkPut(b *testing.B) {
	data := testData(benchSize, 1)
	b.SetBytes(benchSize)
	b.ReportAllocs()
	for b.Loop() {
		s, _ := openStore(b, "", blob.Options{})
		if _, err := s.Put(bytes.NewReader(data), benchSize); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadSlice measures serving one request: extract the slice and check
// it before sending.
func BenchmarkReadSlice(b *testing.B) {
	s, _ := openStore(b, "", blob.Options{})
	h, err := s.PutBytes(testData(benchSize, 2))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(blob.MaxSlice)
	b.ReportAllocs()
	var i uint64
	for b.Loop() {
		if _, err := s.ReadSlice(h, (i%(benchSize/blob.MaxSlice))*blob.MaxSlice, blob.MaxSlice); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

// BenchmarkFetch measures a whole download between two services on the
// in-memory network: requests, slice checks, disk writes, final check.
func BenchmarkFetch(b *testing.B) {
	net := memnet.New(1, memnet.Faults{})
	serverStore, _ := openStore(b, "", blob.Options{})
	h, err := serverStore.PutBytes(testData(benchSize, 3))
	if err != nil {
		b.Fatal(err)
	}
	serve, fetch := net.JoinBulk("s"), net.JoinBulk("f")
	net.Link("s", "f")
	run := func(lane transport.Transport, st *blob.Store) *blob.Service {
		svc, err := blob.NewService(lane, st, blob.ServiceOptions{
			Referenced: func(blob.Hash) bool { return true },
			Allowed:    func(transport.PeerID) bool { return true },
		})
		if err != nil {
			b.Fatal(err)
		}
		go func() { _ = svc.Run(b.Context()) }()
		return svc
	}
	run(serve, serverStore)
	st, _ := openStore(b, "", blob.Options{})
	svc := run(fetch, st)
	b.SetBytes(benchSize)
	b.ReportAllocs()
	for b.Loop() {
		if err := svc.Fetch(b.Context(), blob.Ref{Hash: h, Size: benchSize}); err != nil {
			b.Fatal(err)
		}
		if err := st.Remove(h); err != nil {
			b.Fatal(err)
		}
	}
}
