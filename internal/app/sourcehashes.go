package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/sidecar"
)

// SourceSnapshot owns immutable compilation inputs. Current rechecks the complete
// projected dependency set before identities can be offered for navigation.
type SourceSnapshot struct {
	Root    string
	Hashes  map[string]string
	Current func(context.Context) bool
	Cleanup func()
}

func snapshotProject(ctx context.Context, p *project.Project) (SourceSnapshot, error) {
	pool, err := sourceSnapshotPool()
	if err != nil {
		return SourceSnapshot{}, err
	}
	defer runtime.KeepAlive(pool.lease)
	base := pool.root
	root, err := os.MkdirTemp(base, "compile-")
	if err != nil {
		return SourceSnapshot{}, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	snapshot, err := p.SnapshotTo(ctx, root)
	if err != nil {
		cleanup()
		return SourceSnapshot{}, err
	}
	return SourceSnapshot{Root: root, Hashes: snapshot.TextHashes, Current: func(ctx context.Context) bool { return p.MatchesSnapshot(ctx, snapshot) }, Cleanup: cleanup}, nil
}

type snapshotPool struct {
	root  string
	lease *os.File
}

var sourceSnapshotPool = sync.OnceValues(func() (snapshotPool, error) {
	cache, err := sidecar.DefaultDir()
	if err != nil {
		return snapshotPool{}, err
	}
	root, lease, err := newSnapshotSession(filepath.Join(cache, "source-snapshots"))
	return snapshotPool{root: root, lease: lease}, err
})
