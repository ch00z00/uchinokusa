package httpapi_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

func newTestRouter(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{
		Logger:   logger,
		Verifier: &fakeVerifier{},
		Accounts: &fakeAccounts{},
	}), &buf
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	router, _ := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestHealthz_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	router, _ := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

var generatedID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		incoming string // クライアントが送る X-Request-Id（空なら送らない）
		wantSame bool   // 受け取った ID をそのまま使うか
	}{
		{name: "ヘッダーがなければ生成する", incoming: ""},
		{name: "正しい形式ならそのまま使う", incoming: "abc-123_XYZ", wantSame: true},
		{name: "長すぎる場合は生成し直す", incoming: strings.Repeat("a", 65)},
		{name: "使えない文字を含む場合は生成し直す", incoming: "abc def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, _ := newTestRouter(t)
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			if tt.incoming != "" {
				req.Header.Set("X-Request-Id", tt.incoming)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			got := rec.Header().Get("X-Request-Id")
			if tt.wantSame {
				if got != tt.incoming {
					t.Errorf("X-Request-Id = %q, want %q", got, tt.incoming)
				}
				return
			}
			if !generatedID.MatchString(got) {
				t.Errorf("X-Request-Id = %q, want 32 hex chars", got)
			}
		})
	}
}

func TestAccessLog(t *testing.T) {
	t.Parallel()

	router, buf := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req-access-log")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("want exactly one JSON log line, got %q: %v", buf.String(), err)
	}

	want := map[string]any{
		"msg":        "http request",
		"request_id": "req-access-log",
		"method":     "GET",
		"path":       "/healthz",
		"status":     float64(200), // JSON の数値は float64 になる
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v", k, line[k], v)
		}
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Error("duration_ms is missing")
	}
}
