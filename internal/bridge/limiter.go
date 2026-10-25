package bridge

import (
	"sync"
	"time"
)

// logInterval is the minimum time between two logs of the same reason.
const logInterval = time.Second

// limiter lets one event per reason through each [logInterval] and counts the
// ones it suppressed, so a hostile webview cannot flood the log. The zero
// value is ready to use.
type limiter struct {
	reasons map[string]*limit
	mu      sync.Mutex
}

type limit struct {
	last       time.Time
	suppressed int
}

// allow reports whether an event of the given reason may be logged now. When
// it may, suppressed is the number of events of that reason dropped since the
// previous allowed one.
func (l *limiter) allow(reason string) (suppressed int, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.reasons == nil {
		l.reasons = make(map[string]*limit)
	}
	s := l.reasons[reason]
	if s == nil {
		s = new(limit)
		l.reasons[reason] = s
	}
	now := time.Now()
	if !s.last.IsZero() && now.Sub(s.last) < logInterval {
		s.suppressed++
		return 0, false
	}
	suppressed, s.last, s.suppressed = s.suppressed, now, 0
	return suppressed, true
}
