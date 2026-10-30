package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// TokenVerifier は、ID トークンを検証する（実装は adapter/firebase）。
// トークンが無効・期限切れなら domain.ErrUnauthenticated を返す。
type TokenVerifier interface {
	Verify(ctx context.Context, idToken string) (domain.Identity, error)
}

// AccountService は、ユーザーの特定と、プロフィール・端末の変更を行う（実装は app.AccountService）。
type AccountService interface {
	SignIn(ctx context.Context, id domain.Identity) (domain.User, error)
	UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (domain.User, error)
	RegisterDevice(ctx context.Context, userID uuid.UUID, fcmToken, platform, timeZone string) (domain.Device, error)
	UnregisterDevice(ctx context.Context, userID, deviceID uuid.UUID) error
}

type userKey struct{}

// currentUser は、withAuth が context に入れた、リクエストしたユーザーを返す。
// withAuth を通ったハンドラーでだけ呼ぶ。
func currentUser(ctx context.Context) domain.User {
	u, ok := ctx.Value(userKey{}).(domain.User)
	if !ok {
		panic("httpapi: currentUser called without withAuth") // ルーティングの組み立ての誤りなので、すぐに気づけるようにする
	}
	return u
}

// withAuth は、Authorization ヘッダーの ID トークンを検証し、ユーザーを context に入れる（要件1-3）。
// 初めてのユーザーなら、ユーザーを作る。
func withAuth(logger *slog.Logger, verifier TokenVerifier, accounts AccountService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fail := func(err error) {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeError(w, r, logger, err)
			}

			token, ok := bearerToken(r)
			if !ok {
				fail(domain.ErrUnauthenticated)
				return
			}
			identity, err := verifier.Verify(r.Context(), token)
			if err != nil {
				fail(err)
				return
			}
			user, err := accounts.SignIn(r.Context(), identity)
			if err != nil {
				fail(err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
		})
	}
}

// bearerToken は、`Authorization: Bearer <トークン>` からトークンを取り出す。
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
