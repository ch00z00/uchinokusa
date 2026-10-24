package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

const requestIDHeader = "X-Request-Id"

// validRequestID は、クライアントから受け取るリクエスト ID として認める形式である。
// ログに任意の文字列を書き込まれないように、文字の種類と長さを制限する。
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// withRequestID は、リクエストごとに ID を決めて context とレスポンスヘッダーに入れる。
// クライアントが正しい形式の X-Request-Id を送ってきた場合は、それを使う（アプリ側のログと突き合わせるため）。
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), id)))
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read は失敗しない（Go 1.24 以降）
	return hex.EncodeToString(b)
}

// withAccessLog は、リクエストごとに、メソッド・パス・ステータス・処理時間を1行のログに残す。
func withAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		logger.InfoContext(r.Context(), "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sw.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}

// statusWriter は、ハンドラーが返したステータスコードを記録する。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap により、http.ResponseController が元の ResponseWriter の機能（Flush など）を使えるようにする。
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
