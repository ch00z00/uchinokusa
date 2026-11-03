package httpapi

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// fakeClock は、テストで進める時計である。
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestUserRateLimiter(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{t: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)}
	l := newUserRateLimiter(rate.Every(6*time.Second), 3, clock.now)
	alice, bob := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	// まとめて使えるのは3回まで
	for i := range 3 {
		if ok, _ := l.allow(alice); !ok {
			t.Fatalf("request %d was rejected", i+1)
		}
	}
	ok, retryAfter := l.allow(alice)
	if ok {
		t.Fatal("4th request was allowed")
	}
	if retryAfter != 6*time.Second {
		t.Errorf("retryAfter = %v, want 6s", retryAfter)
	}

	// 利用者ごとに数える
	if ok, _ := l.allow(bob); !ok {
		t.Error("bob was limited by alice's requests")
	}

	// 6秒たつと1回分だけ回復する
	clock.advance(6 * time.Second)
	if ok, _ := l.allow(alice); !ok {
		t.Error("request after 6s was rejected")
	}
	if ok, _ := l.allow(alice); ok {
		t.Error("2 requests were allowed after only 6s")
	}
}

// しばらく使われていない利用者の分は片付け、メモリが増え続けないようにする。
func TestUserRateLimiter_SweepsIdleUsers(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{t: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)}
	l := newUserRateLimiter(rate.Every(6*time.Second), 3, clock.now)

	for range 100 {
		l.allow(uuid.Must(uuid.NewV7()))
	}
	clock.advance(limiterIdleTTL + time.Second)
	active := uuid.Must(uuid.NewV7())
	l.allow(active)

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.users) != 1 {
		t.Errorf("users = %d, want 1 (only the active user)", len(l.users))
	}
}
