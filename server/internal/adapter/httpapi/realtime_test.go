package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
)

// fakeRealtime は、WebSocket の接続を受け付けた相手を記録する。
type fakeRealtime struct {
	users []uuid.UUID
}

func (f *fakeRealtime) Serve(w http.ResponseWriter, _ *http.Request, userID uuid.UUID) {
	f.users = append(f.users, userID)
	w.WriteHeader(http.StatusSwitchingProtocols)
}

func TestRealtime(t *testing.T) {
	t.Parallel()
	newRouter := func(rt *fakeRealtime) http.Handler {
		return httpapi.NewRouter(httpapi.Deps{
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			Verifier: &fakeVerifier{},
			Accounts: &fakeAccounts{},
			Realtime: rt,
		})
	}

	t.Run("ID トークンで認証したユーザーの接続を受け付ける", func(t *testing.T) {
		t.Parallel()
		rt := &fakeRealtime{}
		req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()
		newRouter(rt).ServeHTTP(rec, req)
		if rec.Code != http.StatusSwitchingProtocols || len(rt.users) != 1 || rt.users[0] != testUser.ID {
			t.Errorf("status = %d, users = %v", rec.Code, rt.users)
		}
	})

	t.Run("トークンがなければ 401 にし、接続しない", func(t *testing.T) {
		t.Parallel()
		rt := &fakeRealtime{}
		rec := httptest.NewRecorder()
		newRouter(rt).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/ws", nil))
		if rec.Code != http.StatusUnauthorized || len(rt.users) != 0 {
			t.Errorf("status = %d, users = %v", rec.Code, rt.users)
		}
	})
}
