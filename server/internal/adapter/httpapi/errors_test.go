package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

func TestWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantFields map[string]string
	}{
		{
			name:       "リクエストの形式の誤りは 400",
			err:        &badRequestError{err: errors.New("invalid JSON")},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:       "入力の誤りは 422 で、入力欄ごとの理由を返す",
			err:        &domain.ValidationError{Fields: map[string]string{"name": "required"}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantFields: map[string]string{"name": "required"},
		},
		{
			name:       "未認証は 401",
			err:        domain.ErrUnauthenticated,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthenticated",
		},
		{
			name:       "権限なしは 403",
			err:        domain.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "見つからないは 404",
			err:        domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "ラップされていても種類を判定できる",
			err:        fmt.Errorf("get plant: %w", domain.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "衝突は 409",
			err:        &domain.ConflictError{Code: "conflict"},
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "API 定義にない衝突のコードは、汎用の conflict にする",
			err:        &domain.ConflictError{Code: "not_in_openapi"},
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "それ以外のエラーは 500",
			err:        errors.New("connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
			rec := httptest.NewRecorder()
			writeError(rec, httptest.NewRequest(http.MethodGet, "/v1/plants", nil), logger, tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			var body struct {
				Code    string            `json:"code"`
				Message string            `json:"message"`
				Fields  map[string]string `json:"fields"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tt.wantCode)
			}
			if body.Message == "" {
				t.Error("message is empty")
			}
			if len(body.Fields) != len(tt.wantFields) {
				t.Errorf("fields = %v, want %v", body.Fields, tt.wantFields)
			}
			for k, v := range tt.wantFields {
				if body.Fields[k] != v {
					t.Errorf("fields[%q] = %q, want %q", k, body.Fields[k], v)
				}
			}
		})
	}
}

// 500 のときは、原因をログに残し、レスポンスには内部の情報を出さない。
func TestWriteError_InternalDoesNotLeak(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, slog.LevelInfo))
	req := httptest.NewRequest(http.MethodGet, "/v1/plants", nil)
	req = req.WithContext(logging.WithRequestID(req.Context(), "req-internal"))
	rec := httptest.NewRecorder()

	writeError(rec, req, logger, errors.New("dial tcp 10.0.0.5:5432: connection refused"))

	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Errorf("response leaks the cause: %s", rec.Body.String())
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("want exactly one JSON log line, got %q: %v", buf.String(), err)
	}
	if line["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", line["level"])
	}
	if line["request_id"] != "req-internal" {
		t.Errorf("request_id = %v, want req-internal", line["request_id"])
	}
	if got, _ := line["error"].(string); !strings.Contains(got, "connection refused") {
		t.Errorf("error = %q, want the cause", got)
	}
}
