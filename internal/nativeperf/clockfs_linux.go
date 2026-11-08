package nativeperf

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// ClockAssets exposes a monotonic clock only in the dedicated perf_native asset
// set. Each unique request is a synchronous timestamp boundary in the renderer.
// The elapsed interval includes transport overhead; none is subtracted.
func ClockAssets(base fs.FS) fs.FS {
	return &clockAssets{FS: base, origin: time.Now()}
}

type clockAssets struct {
	fs.FS
	origin time.Time
}

func (a *clockAssets) Open(name string) (fs.File, error) {
	if !strings.HasPrefix(name, "__native_perf_clock/") {
		return a.FS.Open(name)
	}
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	var resolution unix.Timespec
	if err := unix.ClockGetres(unix.CLOCK_MONOTONIC, &resolution); err != nil {
		return nil, err
	}
	data, err := json.Marshal(struct {
		MS           float64 `json:"ms"`
		ResolutionMS float64 `json:"resolutionMs"`
	}{float64(time.Since(a.origin).Nanoseconds()) / 1e6, float64(resolution.Nano()) / 1e6})
	if err != nil {
		return nil, err
	}
	return &clockFile{Reader: bytes.NewReader(data), name: path.Base(name)}, nil
}

type clockFile struct {
	*bytes.Reader
	name string
}

func (f *clockFile) Close() error               { return nil }
func (f *clockFile) Stat() (fs.FileInfo, error) { return clockInfo{f.name, f.Size()}, nil }

type clockInfo struct {
	name string
	size int64
}

func (f clockInfo) Name() string       { return f.name }
func (f clockInfo) Size() int64        { return f.size }
func (f clockInfo) Mode() fs.FileMode  { return 0o400 }
func (f clockInfo) ModTime() time.Time { return time.Time{} }
func (f clockInfo) IsDir() bool        { return false }
func (f clockInfo) Sys() any           { return nil }
