package frame

import (
	"context"
	"sync"
)

// Budget is a byte semaphore: [Budget.AcquireCtx] waits until enough bytes are
// free, or its context ends. Create it with [NewBudget].
type Budget struct {
	mu     sync.Mutex
	size   int
	avail  int
	closed bool
	// freed is closed when bytes are released or the budget closes, then forgotten; it exists only while
	// someone waits, so an uncontended Release allocates nothing.
	freed chan struct{}
}

// NewBudget returns a Budget of size bytes.
func NewBudget(size int) *Budget {
	return &Budget{size: size, avail: size}
}

// Acquire takes n bytes (at most the budget size), waiting for them to be
// released. It reports false if the budget was closed meanwhile.
func (b *Budget) Acquire(n int) bool {
	return b.AcquireCtx(context.Background(), n) == nil
}

// AcquireCtx takes n bytes (at most the budget size), waiting for them to be
// released. It returns ctx's error if ctx ends first, and
// [context.Canceled] if the budget was closed meanwhile; either way nothing is
// taken.
func (b *Budget) AcquireCtx(ctx context.Context, n int) error {
	for {
		b.mu.Lock()
		if err := ctx.Err(); err != nil {
			b.mu.Unlock()
			return err
		}
		switch {
		case b.closed:
			b.mu.Unlock()
			return context.Canceled
		case b.avail >= n:
			b.avail -= n
			b.mu.Unlock()
			return nil
		}
		if b.freed == nil {
			b.freed = make(chan struct{})
		}
		freed := b.freed
		b.mu.Unlock()
		select {
		case <-freed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// TryAcquire takes n bytes if they are free now and reports whether it did.
func (b *Budget) TryAcquire(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.avail < n {
		return false
	}
	b.avail -= n
	return true
}

// Release gives n bytes back.
func (b *Budget) Release(n int) {
	if n == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.avail += n
	b.wakeLocked()
}

// Held is how many bytes are taken.
func (b *Budget) Held() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size - b.avail
}

// Close wakes every waiter; later acquires fail.
func (b *Budget) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.wakeLocked()
}

func (b *Budget) wakeLocked() {
	if b.freed != nil {
		close(b.freed)
		b.freed = nil
	}
}
