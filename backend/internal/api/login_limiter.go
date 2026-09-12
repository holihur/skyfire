package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// loginLimiter slows down brute-force attempts against the password/TOTP
// login. State is kept in memory only, expires after a short window and is
// never persisted, so no client identifier (IP address, a personal datum) is
// retained beyond what is needed for the security measure itself.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attempt
	now      func() time.Time
}

type attempt struct {
	fails int
	last  time.Time
}

const (
	// loginMaxFails is the number of failures within loginWindow that triggers
	// a temporary block.
	loginMaxFails = 10
	// loginWindow bounds how long a failure is remembered.
	loginWindow = 15 * time.Minute
	// loginBlock is how long a blocked key must wait after the last failure.
	loginBlock = 15 * time.Minute
	// loginMaxTracked caps memory usage; the oldest entries are swept first.
	loginMaxTracked = 10000
)

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string]*attempt), now: time.Now}
}

// allowed reports whether an attempt may proceed; if not, it returns how long
// to wait.
func (l *loginLimiter) allowed(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[key]
	if !ok {
		return true, 0
	}
	now := l.now()
	if now.Sub(a.last) > loginWindow {
		delete(l.attempts, key)
		return true, 0
	}
	if a.fails >= loginMaxFails {
		if remaining := loginBlock - now.Sub(a.last); remaining > 0 {
			return false, remaining
		}
		delete(l.attempts, key)
	}
	return true, 0
}

// fail records a failed attempt for key.
func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.attempts) >= loginMaxTracked {
		l.sweepLocked()
	}
	a, ok := l.attempts[key]
	if !ok {
		a = &attempt{}
		l.attempts[key] = a
	}
	a.fails++
	a.last = l.now()
}

// reset clears the failure state after a successful login.
func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func (l *loginLimiter) sweepLocked() {
	now := l.now()
	for k, a := range l.attempts {
		if now.Sub(a.last) > loginWindow {
			delete(l.attempts, k)
		}
	}
}

// clientKey derives a rate-limit key from the request's peer address.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
