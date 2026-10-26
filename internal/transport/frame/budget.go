package frame

import "sync"

// Budget is a byte semaphore: [Budget.Acquire] blocks until enough bytes are
// free. Create it with [NewBudget].
type Budget struct {
	cond   *sync.Cond
	mu     sync.Mutex
	avail  int
	closed bool
}

// NewBudget returns a Budget of size bytes.
func NewBudget(size int) *Budget {
	b := &Budget{avail: size}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Acquire takes n bytes (at most the budget size), waiting for them to be
// released. It reports false if the budget was closed meanwhile.
func (b *Budget) Acquire(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for b.avail < n && !b.closed {
		b.cond.Wait()
	}
	if b.closed {
		return false
	}
	b.avail -= n
	return true
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
	b.avail += n
	b.mu.Unlock()
	b.cond.Broadcast()
}

// Close wakes every waiter; later acquires fail.
func (b *Budget) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	b.cond.Broadcast()
}
