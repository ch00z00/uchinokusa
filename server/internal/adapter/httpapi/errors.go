package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

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
// ドメインのエラーから HTTP への変換は、すべてここで行う。
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var (
		badRequest *badRequestError
		validation *domain.ValidationError
		conflict   *domain.ConflictError
	)

	switch {
	case errors.As(err, &badRequest):
		writeJSON(w, http.StatusBadRequest, oapi.Error{Code: oapi.ErrorCodeBadRequest, Message: badRequest.Error()})
	case errors.As(err, &validation):
		writeJSON(w, http.StatusUnprocessableEntity, oapi.Error{
			Code:    oapi.ErrorCodeValidationFailed,
			Message: "validation failed",
			Fields:  &validation.Fields,
		})
	case errors.Is(err, domain.ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, oapi.Error{Code: oapi.ErrorCodeUnauthenticated, Message: "authentication required"})
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, oapi.Error{Code: oapi.ErrorCodeForbidden, Message: "permission denied"})
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, oapi.Error{Code: oapi.ErrorCodeNotFound, Message: "not found"})
	case errors.As(err, &conflict):
		code := oapi.ErrorCode(conflict.Code)
		if !code.Valid() {
			// API 定義にないコードは、アプリが分岐できないため、汎用のコードにする
			code = oapi.ErrorCodeConflict
		}
		writeJSON(w, http.StatusConflict, oapi.Error{Code: code, Message: conflict.Error()})
	default:
		// 原因はログにだけ残し、レスポンスには内部の情報（接続先など）を出さない
		logger.ErrorContext(r.Context(), "internal error", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, oapi.Error{Code: oapi.ErrorCodeInternal, Message: "internal server error"})
	}
}
