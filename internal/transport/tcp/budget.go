package tcp

import "sync"

// budget is a byte semaphore: acquire blocks until enough bytes are free.
type budget struct {
	cond   *sync.Cond
	mu     sync.Mutex
	avail  int
	size   int
	closed bool
}

func newBudget(size int) *budget {
	b := &budget{avail: size, size: size}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// acquire takes n bytes (at most the budget size), waiting for them to be
// released. It reports false if the budget was closed meanwhile.
func (b *budget) acquire(n int) bool {
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

// release gives n bytes back.
func (b *budget) release(n int) {
	if n == 0 {
		return
	}
	b.mu.Lock()
	b.avail += n
	b.mu.Unlock()
	b.cond.Broadcast()
}

// close wakes every waiter; later acquires fail.
func (b *budget) close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	b.cond.Broadcast()
}
