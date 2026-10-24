package config_test

import (
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/config"
)

// envFrom は、map を環境変数の代わりにする getenv 関数を返す。
func envFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"DATABASE_URL": "postgres://localhost:5432/uchinokusa",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want INFO", cfg.LogLevel)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("AllowedOrigins = %v, want empty", cfg.AllowedOrigins)
	}
}

func TestLoad_AllValues(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"PORT":                           "9000",
		"DATABASE_URL":                   "postgres://db:5432/uchinokusa",
		"FIREBASE_PROJECT_ID":            "demo-uchinokusa",
		"GOOGLE_APPLICATION_CREDENTIALS": "/secrets/service-account.json",
		"ALLOWED_ORIGINS":                " https://uchinokusa.app , ,https://dev.uchinokusa.app",
		"LOG_LEVEL":                      "debug",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := config.Config{
		Port:                    9000,
		DatabaseURL:             "postgres://db:5432/uchinokusa",
		FirebaseProjectID:       "demo-uchinokusa",
		FirebaseCredentialsFile: "/secrets/service-account.json",
		AllowedOrigins:          []string{"https://uchinokusa.app", "https://dev.uchinokusa.app"},
		LogLevel:                slog.LevelDebug,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantMsg []string // エラーメッセージに含まれるべき環境変数名
	}{
		{
			name:    "DATABASE_URL がない",
			env:     map[string]string{},
			wantMsg: []string{"DATABASE_URL"},
		},
		{
			name:    "PORT が数値でない",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "PORT": "abc"},
			wantMsg: []string{"PORT"},
		},
		{
			name:    "PORT が範囲外",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "PORT": "70000"},
			wantMsg: []string{"PORT"},
		},
		{
			name:    "LOG_LEVEL が不正",
			env:     map[string]string{"DATABASE_URL": "postgres://x", "LOG_LEVEL": "verbose"},
			wantMsg: []string{"LOG_LEVEL"},
		},
		{
			// 複数の誤りは、1回でまとめて知らせる
			name:    "複数の誤り",
			env:     map[string]string{"PORT": "abc", "LOG_LEVEL": "verbose"},
			wantMsg: []string{"DATABASE_URL", "PORT", "LOG_LEVEL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(envFrom(tt.env))
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			for _, key := range tt.wantMsg {
				if !strings.Contains(err.Error(), key) {
					t.Errorf("error %q does not mention %s", err, key)
				}
			}
		})
	}
}
