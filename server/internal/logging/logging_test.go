package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

// decodeLines は、JSON のログを1行ずつ map に変換する。
func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var lines []map[string]any
	dec := json.NewDecoder(buf)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatalf("log is not JSON: %v", err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestHandler_AddsRequestIDFromContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, slog.LevelInfo))

	ctx := logging.WithRequestID(context.Background(), "req-123")
	logger.InfoContext(ctx, "hello", "k", "v")

	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if got := lines[0]["request_id"]; got != "req-123" {
		t.Errorf("request_id = %v, want req-123", got)
	}
	if got := lines[0]["msg"]; got != "hello" {
		t.Errorf("msg = %v, want hello", got)
	}
	if got := lines[0]["k"]; got != "v" {
		t.Errorf("k = %v, want v", got)
	}
}

func TestHandler_WithoutRequestID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, slog.LevelInfo))

	logger.InfoContext(context.Background(), "hello")

	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if _, ok := lines[0]["request_id"]; ok {
		t.Errorf("request_id should be absent, got %v", lines[0]["request_id"])
	}
}

func TestHandler_KeepsRequestIDWithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// With で派生させたロガーでも、リクエスト ID が付くこと
	logger := slog.New(logging.NewHandler(&buf, slog.LevelInfo)).
		With("component", "test")

	ctx := logging.WithRequestID(context.Background(), "req-456")
	logger.InfoContext(ctx, "hello", "k", "v")

	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if got := lines[0]["request_id"]; got != "req-456" {
		t.Errorf("request_id = %v, want req-456 at top level", got)
	}
	if got := lines[0]["component"]; got != "test" {
		t.Errorf("component = %v, want test", got)
	}
}

func TestHandler_RespectsLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(logging.NewHandler(&buf, slog.LevelInfo))

	logger.Debug("hidden")

	if buf.Len() != 0 {
		t.Errorf("debug log should be suppressed at INFO, got %q", buf.String())
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()

	if got := logging.RequestID(context.Background()); got != "" {
		t.Errorf("RequestID(empty ctx) = %q, want empty", got)
	}
	ctx := logging.WithRequestID(context.Background(), "req-789")
	if got := logging.RequestID(ctx); got != "req-789" {
		t.Errorf("RequestID() = %q, want req-789", got)
	}
}
