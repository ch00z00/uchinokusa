package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

var validEnv = map[string]string{"DATABASE_URL": "postgres://localhost:5432/uchinokusa"}

func TestRun_Usage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "サブコマンドがない", args: nil},
		{name: "知らないサブコマンド", args: []string{"unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := run(context.Background(), tt.args, envFrom(validEnv), io.Discard)
			if !errors.Is(err, errUsage) {
				t.Errorf("run() error = %v, want errUsage", err)
			}
		})
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), []string{"serve"}, envFrom(map[string]string{}), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("run() error = %v, want config error about DATABASE_URL", err)
	}
}

func TestRun_NotImplementedYet(t *testing.T) {
	t.Parallel()

	// migrate と seed は、タスク 1.3 で実装する
	for _, cmd := range []string{"migrate", "seed"} {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()

			err := run(context.Background(), []string{cmd}, envFrom(validEnv), io.Discard)
			if !errors.Is(err, errNotImplemented) {
				t.Errorf("run(%s) error = %v, want errNotImplemented", cmd, err)
			}
		})
	}
}
