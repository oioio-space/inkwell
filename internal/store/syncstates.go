package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
)

// RemoveSyncStates removes obsolete peer sync caches, leaving document chunks
// intact. Call before attaching a webview; ephemeral peers never write them again.
func (kv *KV) RemoveSyncStates() (int, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	docs, err := fs.ReadDir(kv.r.FS(), repoDir)
	if err != nil {
		return 0, fmt.Errorf("store: list sync caches: %w", err)
	}
	count := 0
	for _, doc := range docs {
		if !doc.IsDir() {
			continue
		}
		dir := path.Join(repoDir, doc.Name(), "sync-state")
		info, err := kv.r.Lstat(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return count, fmt.Errorf("store: inspect sync cache: %w", err)
		}
		if !info.IsDir() {
			continue
		}
		if err := kv.r.RemoveAll(dir); err != nil {
			return count, fmt.Errorf("store: remove sync cache: %w", err)
		}
		count++
	}
	return count, nil
}
