package auth

import (
	"testing"
	"time"
)

func TestLoginThrottle_BackoffAndReset(t *testing.T) {
	now := time.Now()
	tr := NewLoginThrottle(3, time.Second, time.Minute, 10*time.Minute)
	tr.now = func() time.Time { return now }

	// First 3 failures are free (no block).
	for i := 0; i < 3; i++ {
		tr.Fail("email:a")
		if blocked, _ := tr.Blocked("email:a"); blocked {
			t.Fatalf("attempt %d should not be blocked yet", i+1)
		}
	}
	// 4th failure arms backoff.
	tr.Fail("email:a")
	blocked, retry := tr.Blocked("email:a")
	if !blocked || retry <= 0 {
		t.Fatalf("expected block with positive retry, got blocked=%v retry=%v", blocked, retry)
	}

	// Success clears it immediately.
	tr.Succeed("email:a")
	if blocked, _ := tr.Blocked("email:a"); blocked {
		t.Fatal("success must clear the throttle")
	}
}

func TestLoginThrottle_AutoResetAfterWindow(t *testing.T) {
	now := time.Now()
	tr := NewLoginThrottle(1, time.Second, time.Minute, 5*time.Minute)
	tr.now = func() time.Time { return now }

	tr.Fail("ip:1.2.3.4")
	tr.Fail("ip:1.2.3.4") // armed
	if blocked, _ := tr.Blocked("ip:1.2.3.4"); !blocked {
		t.Fatal("expected blocked after exceeding free attempts")
	}
	// Advance past the idle window → auto-reset.
	now = now.Add(6 * time.Minute)
	if blocked, _ := tr.Blocked("ip:1.2.3.4"); blocked {
		t.Fatal("expected auto-reset after the idle window")
	}
}

func TestLoginThrottle_MultiKeyStrictestWins(t *testing.T) {
	now := time.Now()
	tr := NewLoginThrottle(2, time.Second, time.Minute, 10*time.Minute)
	tr.now = func() time.Time { return now }

	// Only the email key is hammered; a fresh IP key is clean. Blocked() over
	// both must still report blocked (strictest wins).
	for i := 0; i < 4; i++ {
		tr.Fail("email:victim")
	}
	if blocked, _ := tr.Blocked("email:victim", "ip:9.9.9.9"); !blocked {
		t.Fatal("expected blocked when any key is armed")
	}
}
