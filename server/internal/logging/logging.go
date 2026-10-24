// Package logging は、JSON 形式の構造化ログと、context に入れたリクエスト ID の受け渡しを提供する。
package logging

import (
	"context"
	"io"
	"log/slog"
)

type requestIDKey struct{}

// WithRequestID は、リクエスト ID を入れた context を返す。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID は、context に入っているリクエスト ID を返す。入っていなければ空文字を返す。
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// NewHandler は、JSON 形式で w に書き出すハンドラーを返す。
// *Context 系のメソッド（InfoContext など）で記録すると、context のリクエスト ID を request_id として付ける。
func NewHandler(w io.Writer, level slog.Level) slog.Handler {
	return contextHandler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})}
}

// contextHandler は、context の値をログの項目に加えるハンドラーである。
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs と WithGroup は、派生したロガーでも contextHandler の働きを保つために包み直す。

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
