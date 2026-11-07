// Package httpapi は、HTTP のルーティング、ハンドラー、ミドルウェアを提供する。
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
)

// basePath は、API 定義（api/openapi.yaml）の servers の URL である。
const basePath = "/v1"

// Deps は、ルーターが使う依存である。
type Deps struct {
	Logger      *slog.Logger
	Verifier    TokenVerifier
	Accounts    AccountService
	Households  HouseholdService
	Members     MembershipService
	Invitations InvitationService
	Species     SpeciesService
	Locations   LocationService
	Plants      PlantService
	Tasks       CareTaskService
	Records     CareRecordService
	Today       TodayService
	Notes       NoteService
	Calendar    CalendarService
	Suggestions SuggestionService
	// Realtime は、WebSocket でのリアルタイム同期である（nil なら /v1/ws を登録しない）。
	Realtime RealtimeServer
}

// RealtimeServer は、WebSocket の接続を受け付ける（実装は adapter/ws の Hub）。
type RealtimeServer interface {
	Serve(w http.ResponseWriter, r *http.Request, userID uuid.UUID)
}

// NewRouter は、API のすべてのエンドポイントを登録したハンドラーを返す。
func NewRouter(d Deps) http.Handler {
	logger := d.Logger
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	// API 定義から生成したルーティングに、エラーの変換（writeError）をつないで登録する。
	// リクエストを読めなかったときは 400、ハンドラーがエラーを返したときはその種類に応じたステータスにする
	onRequestError := func(w http.ResponseWriter, r *http.Request, err error) {
		writeError(w, r, logger, &badRequestError{err: err})
	}
	// 招待の確認と参加は、招待コードの総当たりを防ぐため、利用者ごとに回数を制限する（要件20-3）。
	// 10回まで続けて呼べ、6秒ごとに1回分回復する（1分に約10回）
	invitationLimit := withRateLimit(newUserRateLimiter(rate.Every(6*time.Second), 10, time.Now), "GetInvitation", "AcceptInvitation")
	strict := oapi.NewStrictHandlerWithOptions(&server{logger: logger, accounts: d.Accounts, households: d.Households, members: d.Members, invitations: d.Invitations, species: d.Species, locations: d.Locations, plants: d.Plants, tasks: d.Tasks, records: d.Records, today: d.Today, notes: d.Notes, calendar: d.Calendar, suggestions: d.Suggestions}, []oapi.StrictMiddlewareFunc{invitationLimit}, oapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: onRequestError,
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, r, logger, err)
		},
	})
	oapi.HandlerWithOptions(strict, oapi.StdHTTPServerOptions{
		BaseURL:          basePath,
		BaseRouter:       mux,
		ErrorHandlerFunc: onRequestError,
		// /v1 のすべてのエンドポイントで、ID トークンを検証する（/healthz は対象外）
		Middlewares: []oapi.MiddlewareFunc{withAuth(logger, d.Verifier, d.Accounts)},
	})

	// WebSocket は、API 定義（OpenAPI）では表せないので、ここで直接登録する。認証は、ほかの API と同じ Authorization ヘッダーで行う
	if d.Realtime != nil {
		mux.Handle("GET "+basePath+"/ws", withAuth(logger, d.Verifier, d.Accounts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d.Realtime.Serve(w, r, currentUser(r.Context()).ID)
		})))
	}

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
