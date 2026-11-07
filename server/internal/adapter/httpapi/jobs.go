package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

// JobRunner は、名前で定期ジョブを実行する（実装は job.Runner）。
type JobRunner interface {
	Run(ctx context.Context, name string) (job.Report, error)
}

// JobTokenVerifier は、ジョブの呼び出し元のトークンを確かめる（実装は adapter/gcp の JobVerifier）。
type JobTokenVerifier interface {
	VerifyJob(ctx context.Context, token string) error
}

// jobResponse は、ジョブの実行の結果である（Cloud Scheduler の実行のログに残る）。
type jobResponse struct {
	Job        string `json:"job"`
	Processed  int    `json:"processed"`
	Failed     int    `json:"failed"`
	Skipped    bool   `json:"skipped"`
	DurationMs int64  `json:"durationMs"`
}

// handleJob は、POST /internal/jobs/{name} で、名前のジョブを実行する。
// 失敗したら 500 を返し、Cloud Scheduler の再試行に任せる（ジョブは二重に実行されても結果が変わらないように作る）。
func handleJob(logger *slog.Logger, runner JobRunner) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rep, err := runner.Run(r.Context(), r.PathValue("name"))
		if err != nil {
			writeError(w, r, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, jobResponse{Job: rep.Job, Processed: rep.Processed, Failed: rep.Failed, Skipped: rep.Skipped, DurationMs: rep.Duration.Milliseconds()})
	})
}

// withJobAuth は、Authorization ヘッダーのトークンが Cloud Scheduler のサービスアカウントのものかを確かめる。
// アプリのユーザーの ID トークン（Firebase）では呼べない。
func withJobAuth(logger *slog.Logger, verifier JobTokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeError(w, r, logger, domain.ErrUnauthenticated)
				return
			}
			if err := verifier.VerifyJob(r.Context(), token); err != nil {
				if errors.Is(err, domain.ErrUnauthenticated) {
					w.Header().Set("WWW-Authenticate", "Bearer")
				}
				writeError(w, r, logger, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
