package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// badRequestError は、リクエストを読めなかったこと（JSON の誤り、パラメーターの型の誤りなど）を表す。
// 生成したコードがリクエストを解釈するときのエラーを包む。
type badRequestError struct {
	err error
}

func (e *badRequestError) Error() string { return e.err.Error() }
func (e *badRequestError) Unwrap() error { return e.err }

// writeError は、エラーを HTTP のステータスと API 定義の Error の形に変換して返す。
// ドメインのエラーから HTTP への変換は、すべてここ（と errorResponse）で行う。
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var rateLimited *rateLimitedError
	if errors.As(err, &rateLimited) {
		// 秒に切り上げる（0秒と伝えると、すぐに送り直されてしまう）
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rateLimited.retryAfter.Seconds()))))
	}
	status, body := errorResponse(err)
	if status == http.StatusInternalServerError {
		logInternal(r.Context(), logger, err)
	}
	writeJSON(w, status, body)
}

// errorResponse は、エラーを HTTP のステータスと Error の形にする。まとめて完了の項目ごとのエラーにも使う。
func errorResponse(err error) (int, oapi.Error) {
	var (
		badRequest  *badRequestError
		validation  *domain.ValidationError
		conflict    *domain.ConflictError
		rateLimited *rateLimitedError
	)

	switch {
	case errors.As(err, &badRequest):
		return http.StatusBadRequest, oapi.Error{Code: oapi.ErrorCodeBadRequest, Message: badRequest.Error()}
	case errors.As(err, &validation):
		return http.StatusUnprocessableEntity, oapi.Error{
			Code:    oapi.ErrorCodeValidationFailed,
			Message: "validation failed",
			Fields:  &validation.Fields,
		}
	case errors.Is(err, domain.ErrUnauthenticated):
		return http.StatusUnauthorized, oapi.Error{Code: oapi.ErrorCodeUnauthenticated, Message: "authentication required"}
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, oapi.Error{Code: oapi.ErrorCodeForbidden, Message: "permission denied"}
	case errors.Is(err, domain.ErrInvitationInvalid):
		return http.StatusGone, oapi.Error{Code: oapi.ErrorCodeInvitationInvalid, Message: "invitation is expired, revoked, or unknown"}
	case errors.As(err, &rateLimited):
		return http.StatusTooManyRequests, oapi.Error{Code: oapi.ErrorCodeRateLimited, Message: "too many requests"}
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, oapi.Error{Code: oapi.ErrorCodeNotFound, Message: "not found"}
	case errors.As(err, &conflict):
		code := oapi.ErrorCode(conflict.Code)
		if !code.Valid() {
			// API 定義にないコードは、アプリが分岐できないため、汎用のコードにする
			code = oapi.ErrorCodeConflict
		}
		return http.StatusConflict, oapi.Error{Code: code, Message: conflict.Error()}
	default:
		// 原因はログにだけ残し（呼び出し側で logInternal）、レスポンスには内部の情報（接続先など）を出さない
		return http.StatusInternalServerError, oapi.Error{Code: oapi.ErrorCodeInternal, Message: "internal server error"}
	}
}

func logInternal(ctx context.Context, logger *slog.Logger, err error) {
	logger.ErrorContext(ctx, "internal error", slog.String("error", err.Error()))
}
