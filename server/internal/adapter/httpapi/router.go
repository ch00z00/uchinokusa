// Package httpapi は、HTTP のルーティング、ハンドラー、ミドルウェアを提供する。
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
)

// basePath は、API 定義（api/openapi.yaml）の servers の URL である。
const basePath = "/v1"

// NewRouter は、API のすべてのエンドポイントを登録したハンドラーを返す。
func NewRouter(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	// API 定義から生成したルーティングに、エラーの変換（writeError）をつないで登録する。
	// リクエストを読めなかったときは 400、ハンドラーがエラーを返したときはその種類に応じたステータスにする
	onRequestError := func(w http.ResponseWriter, r *http.Request, err error) {
		writeError(w, r, logger, &badRequestError{err: err})
	}
	strict := oapi.NewStrictHandlerWithOptions(&server{}, nil, oapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: onRequestError,
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, r, logger, err)
		},
	})
	oapi.HandlerWithOptions(strict, oapi.StdHTTPServerOptions{
		BaseURL:          basePath,
		BaseRouter:       mux,
		ErrorHandlerFunc: onRequestError,
	})

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
