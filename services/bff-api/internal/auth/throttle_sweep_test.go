package auth

import (
	"testing"
	"time"
)

func TestLoginThrottle_SweepPrunesIdleKeys(t *testing.T) {
	now := time.Now()
	tr := NewLoginThrottle(3, time.Second, time.Minute, 5*time.Minute)
	tr.now = func() time.Time { return now }

	tr.Fail("email:idle")
	tr.Fail("email:fresh")

	// Advance the idle one past the window but keep the fresh one alive by
	// re-failing it after the jump.
	now = now.Add(6 * time.Minute)
	tr.Fail("email:fresh")

	tr.Sweep()

	tr.mu.Lock()
	_, idleStill := tr.states["email:idle"]
	_, freshStill := tr.states["email:fresh"]
	tr.mu.Unlock()

	if idleStill {
		t.Fatal("Sweep must prune the key idle past the window")
	}
	if !freshStill {
		t.Fatal("Sweep must keep a key that failed within the window")
	}
}

func TestLoginThrottle_SweepNoopWhenNothingIdle(t *testing.T) {
	now := time.Now()
	tr := NewLoginThrottle(3, time.Second, time.Minute, 5*time.Minute)
	tr.now = func() time.Time { return now }

	tr.Fail("email:a")
	tr.Fail("email:b")

	tr.Sweep() // nothing past the window yet

	tr.mu.Lock()
	n := len(tr.states)
	tr.mu.Unlock()
	if n != 2 {
		t.Fatalf("Sweep should retain in-window keys, have %d", n)
	}
}

func TestLoginThrottle_FailBackoffCappedAtMaxDelay(t *testing.T) {
	now := time.Now()
	// base is large and maxDelay small so a single armed step already exceeds
	// the cap, exercising the `d > maxDelay` clamp.
	tr := NewLoginThrottle(0, time.Hour, time.Minute, time.Hour)
	tr.now = func() time.Time { return now }

	tr.Fail("email:capme") // failures=1 > maxFree=0 → shift=1 → base<<1 = 2h > 1m cap
	_, retry := tr.Blocked("email:capme")
	if retry > time.Minute {
		t.Fatalf("backoff must be capped at maxDelay (1m), got %v", retry)
	}
	if retry <= 0 {
		t.Fatalf("expected an armed backoff, got %v", retry)
	}
}

func TestLoginThrottle_FailShiftGuardAgainstOverflow(t *testing.T) {
	now := time.Now()
	// maxFree=0, base=1ns; >21 failures drives shift past the 20-cap guard and
	// the resulting delay overflows to <=0, falling back to maxDelay.
	tr := NewLoginThrottle(0, time.Nanosecond, 30*time.Second, time.Hour)
	tr.now = func() time.Time { return now }

	for i := 0; i < 30; i++ {
		tr.Fail("email:overflow")
	}
	_, retry := tr.Blocked("email:overflow")
	if retry <= 0 || retry > 30*time.Second {
		t.Fatalf("expected delay clamped to maxDelay (30s), got %v", retry)
	}
}
