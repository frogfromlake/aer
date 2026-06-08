package auth

import (
	"sync"
	"time"
)

// LoginThrottle is an in-memory, exponential-backoff brute-force throttle for
// the login endpoint (ADR-040 / security review M-3). It is a THROTTLE, not a
// hard lockout: each failed attempt arms a growing delay that auto-resets after
// an idle window, and a success clears it immediately — so an attacker cannot
// lock a victim out, but cannot grind passwords either. Keyed by both account
// (email) and client IP; the strictest currently-armed key wins.
//
// In-memory is correct at the single-instance POC scale; a multi-instance
// deployment would move this to a shared store (reassess then). The map is
// swept by Sweep() on the same cadence as session cleanup.
type LoginThrottle struct {
	mu       sync.Mutex
	states   map[string]*throttleState
	maxFree  int           // failures allowed before backoff arms
	base     time.Duration // first backoff step
	maxDelay time.Duration // backoff cap
	window   time.Duration // idle duration after which a key auto-resets
	now      func() time.Time
}

type throttleState struct {
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
}

// NewLoginThrottle builds a throttle. Sensible defaults: maxFree=5, base=1s,
// maxDelay=5m, window=15m.
func NewLoginThrottle(maxFree int, base, maxDelay, window time.Duration) *LoginThrottle {
	return &LoginThrottle{
		states:   make(map[string]*throttleState),
		maxFree:  maxFree,
		base:     base,
		maxDelay: maxDelay,
		window:   window,
		now:      time.Now,
	}
}

// Blocked reports whether any key is currently in a backoff window, and the
// longest remaining retry-after across the keys.
func (t *LoginThrottle) Blocked(keys ...string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	var maxRetry time.Duration
	blocked := false
	for _, k := range keys {
		st := t.states[k]
		if st == nil {
			continue
		}
		if now.Sub(st.lastFailure) > t.window {
			delete(t.states, k) // idle → auto-reset
			continue
		}
		if now.Before(st.blockedUntil) {
			blocked = true
			if r := st.blockedUntil.Sub(now); r > maxRetry {
				maxRetry = r
			}
		}
	}
	return blocked, maxRetry
}

// Fail records a failed attempt for every key and arms / extends backoff.
func (t *LoginThrottle) Fail(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	for _, k := range keys {
		st := t.states[k]
		if st == nil || now.Sub(st.lastFailure) > t.window {
			st = &throttleState{}
			t.states[k] = st
		}
		st.failures++
		st.lastFailure = now
		if st.failures > t.maxFree {
			shift := st.failures - t.maxFree
			if shift > 20 {
				shift = 20 // guard against overflow
			}
			d := t.base << uint(shift)
			if d <= 0 || d > t.maxDelay {
				d = t.maxDelay
			}
			st.blockedUntil = now.Add(d)
		}
	}
}

// Succeed clears every key (a successful login resets the throttle).
func (t *LoginThrottle) Succeed(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range keys {
		delete(t.states, k)
	}
}

// Sweep drops idle keys. Cheap; call periodically.
func (t *LoginThrottle) Sweep() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	for k, st := range t.states {
		if now.Sub(st.lastFailure) > t.window {
			delete(t.states, k)
		}
	}
}
