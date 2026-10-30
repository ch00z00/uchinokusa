package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

const validToken = "valid-token"

// fakeVerifier は、validToken だけを正しいトークンとして扱う。
type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (domain.Identity, error) {
	if token != validToken {
		return domain.Identity{}, domain.ErrUnauthenticated
	}
	return domain.Identity{UID: "uid-1", Name: "ゆうすけ", Provider: "google.com"}, nil
}

var testUser = domain.User{
	ID:          uuid.MustParse("01920000-0000-7000-8000-000000000001"),
	FirebaseUID: "uid-1",
	DisplayName: "ゆうすけ",
	CreatedAt:   time.Date(2026, 10, 29, 9, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// fakeAccounts は、app.AccountService の偽物である。
type fakeAccounts struct {
	signInErr    error
	signedIn     []domain.Identity
	unregistered []uuid.UUID
}

func (f *fakeAccounts) SignIn(_ context.Context, id domain.Identity) (domain.User, error) {
	f.signedIn = append(f.signedIn, id)
	return testUser, f.signInErr
}

func (f *fakeAccounts) UpdateDisplayName(_ context.Context, userID uuid.UUID, name string) (domain.User, error) {
	if userID != testUser.ID {
		return domain.User{}, errors.New("unexpected user")
	}
	n, err := domain.NormalizeDisplayName(name)
	if err != nil {
		return domain.User{}, err
	}
	u := testUser
	u.DisplayName = n
	return u, nil
}

func (f *fakeAccounts) RegisterDevice(_ context.Context, userID uuid.UUID, fcmToken, platform, timeZone string) (domain.Device, error) {
	reg, err := domain.NewDeviceRegistration(fcmToken, platform, timeZone)
	if err != nil {
		return domain.Device{}, err
	}
	return domain.Device{
		ID:         testDeviceID,
		UserID:     userID,
		FCMToken:   reg.FCMToken,
		Platform:   reg.Platform,
		TimeZone:   reg.TimeZone,
		LastSeenAt: time.Date(2026, 10, 30, 8, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
	}, nil
}

func (f *fakeAccounts) UnregisterDevice(_ context.Context, userID, deviceID uuid.UUID) error {
	if userID != testUser.ID {
		return errors.New("unexpected user")
	}
	f.unregistered = append(f.unregistered, deviceID)
	return nil
}

func newMeRouter(t *testing.T, accounts *fakeAccounts) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: accounts})
}

func serve(t *testing.T, h http.Handler, method, path, authorization, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
	}
	return v
}

type errorBody struct {
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields"`
}

// トークンがない・形式が違う・無効な場合は、401 unauthenticated で拒否する（要件1-3）。
func TestAuth_RejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		authorization string
	}{
		{name: "ヘッダーがない", authorization: ""},
		{name: "Bearer ではない", authorization: "Basic dXNlcjpwYXNz"},
		{name: "トークンが空", authorization: "Bearer "},
		{name: "無効なトークン（期限切れを含む）", authorization: "Bearer expired-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accounts := &fakeAccounts{}
			rec := serve(t, newMeRouter(t, accounts), http.MethodGet, "/v1/me", tt.authorization, "")

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if got := decode[errorBody](t, rec).Code; got != "unauthenticated" {
				t.Errorf("code = %q, want unauthenticated", got)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", got)
			}
			if len(accounts.signedIn) != 0 {
				t.Error("SignIn was called for an invalid token")
			}
		})
	}
}

// 削除済みのユーザーなど、サインインできない場合も 401 にする。
func TestAuth_SignInFails(t *testing.T) {
	t.Parallel()
	rec := serve(t, newMeRouter(t, &fakeAccounts{signInErr: domain.ErrUnauthenticated}), http.MethodGet, "/v1/me", "Bearer "+validToken, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// 死活監視は認証なしで呼べる。
func TestAuth_HealthzIsPublic(t *testing.T) {
	t.Parallel()
	rec := serve(t, newMeRouter(t, &fakeAccounts{}), http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

type meBody struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdAt"`
}

func TestGetMe(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	rec := serve(t, newMeRouter(t, accounts), http.MethodGet, "/v1/me", "Bearer "+validToken, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	got := decode[meBody](t, rec)
	want := meBody{
		ID:          testUser.ID.String(),
		DisplayName: "ゆうすけ",
		CreatedAt:   "2026-10-29T00:30:00Z", // 日時は UTC で返す
	}
	if got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
	// 検証したトークンの Identity で、ユーザーを特定する
	if len(accounts.signedIn) != 1 || accounts.signedIn[0].UID != "uid-1" {
		t.Errorf("signedIn = %+v, want uid-1", accounts.signedIn)
	}
}

func TestUpdateMe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantName   string // 200 のとき
		wantCode   string // エラーのとき
		wantField  string // 422 のときの fields["displayName"]
	}{
		{name: "表示名を変更する", body: `{"displayName": " はなこ "}`, wantStatus: 200, wantName: "はなこ"},
		{name: "知らない項目は無視する（互換性のルール）", body: `{"displayName": "はなこ", "newField": 1}`, wantStatus: 200, wantName: "はなこ"},
		{name: "空の表示名は 422", body: `{"displayName": ""}`, wantStatus: 422, wantCode: "validation_failed", wantField: "required"},
		{name: "JSON として読めない場合は 400", body: `{"displayName":`, wantStatus: 400, wantCode: "bad_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := serve(t, newMeRouter(t, &fakeAccounts{}), http.MethodPut, "/v1/me", "Bearer "+validToken, tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				if got := decode[meBody](t, rec).DisplayName; got != tt.wantName {
					t.Errorf("displayName = %q, want %q", got, tt.wantName)
				}
				return
			}
			got := decode[errorBody](t, rec)
			if got.Code != tt.wantCode || got.Fields["displayName"] != tt.wantField {
				t.Errorf("error = %+v, want code=%s field=%s", got, tt.wantCode, tt.wantField)
			}
		})
	}
}
