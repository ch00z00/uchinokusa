// Package httpapi は、HTTP のルーティング、ハンドラー、ミドルウェアを提供する。
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// NewRouter は、API のすべてのエンドポイントを登録したハンドラーを返す。
func NewRouter(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	// 外側から順に実行される：リクエスト ID の付与 → アクセスログ → 各ハンドラー
	return withRequestID(withAccessLog(logger, mux))
}

// handleHealthz は、死活監視用のエンドポイントである。プロセスが応答できることだけを示す。
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // ヘッダーを送ったあとなので、失敗してもできることはない
}
