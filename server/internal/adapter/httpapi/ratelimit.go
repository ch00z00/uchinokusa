package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
)

// limiterIdleTTL は、この時間だけ使われていない利用者の制限の状態を片付けるまでの時間である。
// 片付けたあとに来た利用者は、満杯のバケットから数え直す。
const limiterIdleTTL = 10 * time.Minute

// userRateLimiter は、利用者ごとのトークンバケットで、リクエストの回数を制限する（要件20-3）。
// 状態はプロセスのメモリに持つため、Cloud Run のインスタンスが増えると、合計の上限もその数だけ増える。
type userRateLimiter struct {
	limit rate.Limit // 1秒あたりに回復する回数
	burst int        // まとめて使える回数（バケットの大きさ）
	now   func() time.Time

	mu        sync.Mutex
	users     map[uuid.UUID]*userLimiter
	lastSweep time.Time
}

type userLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newUserRateLimiter(limit rate.Limit, burst int, now func() time.Time) *userRateLimiter {
	return &userRateLimiter{limit: limit, burst: burst, now: now, users: map[uuid.UUID]*userLimiter{}, lastSweep: now()}
}

// allow は、利用者のリクエストを1回分使えるかを返す。使えない場合は、次に使えるまでの時間を返す。
func (l *userRateLimiter) allow(userID uuid.UUID) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)
	u, ok := l.users[userID]
	if !ok {
		u = &userLimiter{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.users[userID] = u
	}
	u.lastSeen = now

	// 予約してみて、待たずに使えるならそのまま使う。待つ必要があれば、予約を取り消して待ち時間だけを返す
	r := u.limiter.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// sweep は、limiterIdleTTL より長く使われていない利用者の状態を消す。毎回は行わず、1分に1回にする。
func (l *userRateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for id, u := range l.users {
		if now.Sub(u.lastSeen) > limiterIdleTTL {
			delete(l.users, id)
		}
	}
}

// rateLimitedError は、レート制限を超えたことを表す。writeError が 429 と Retry-After に変換する。
type rateLimitedError struct {
	retryAfter time.Duration
}

func (e *rateLimitedError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v", e.retryAfter)
}

// withRateLimit は、指定した操作（operationID）にだけ、利用者ごとのレート制限をかける。
// withAuth のあとに動くので、currentUser で利用者を特定できる。
func withRateLimit(l *userRateLimiter, operationIDs ...string) oapi.StrictMiddlewareFunc {
	limited := map[string]bool{}
	for _, id := range operationIDs {
		limited[id] = true
	}
	return func(next oapi.StrictHandlerFunc, operationID string) oapi.StrictHandlerFunc {
		if !limited[operationID] {
			return next
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			if ok, retryAfter := l.allow(currentUser(ctx).ID); !ok {
				return nil, &rateLimitedError{retryAfter: retryAfter}
			}
			return next(ctx, w, r, req)
		}
	}
}
