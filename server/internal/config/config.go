// Package config は、環境変数からサーバーの設定を読み込む。
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Config は、サーバーの実行に必要な設定である。
type Config struct {
	// Port は、HTTP サーバーが待ち受けるポート（PORT、初期値 8080）。Cloud Run が値を渡す。
	Port int
	// DatabaseURL は、PostgreSQL の接続先（DATABASE_URL、必須）。
	DatabaseURL string
	// FirebaseProjectID は、ID トークンの検証に使う Firebase のプロジェクト ID（FIREBASE_PROJECT_ID）。
	FirebaseProjectID string
	// FirebaseCredentialsFile は、サービスアカウントの鍵ファイルのパス（GOOGLE_APPLICATION_CREDENTIALS）。
	// Cloud Run では空にして、実行サービスアカウントの権限を使う。
	FirebaseCredentialsFile string
	// AllowedOrigins は、WebSocket などで許可するオリジン（ALLOWED_ORIGINS、カンマ区切り）。
	AllowedOrigins []string
	// LogLevel は、出力するログの最小レベル（LOG_LEVEL：debug / info / warn / error、初期値 info）。
	LogLevel slog.Level
}

// Load は、getenv（通常は os.Getenv）から設定を読み込む。
// 誤りが複数ある場合は、まとめて1つのエラーとして返す。
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:                    8080,
		DatabaseURL:             getenv("DATABASE_URL"),
		FirebaseProjectID:       getenv("FIREBASE_PROJECT_ID"),
		FirebaseCredentialsFile: getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		AllowedOrigins:          splitList(getenv("ALLOWED_ORIGINS")),
		LogLevel:                slog.LevelInfo,
	}

	var errs []error

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	if v := getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Errorf("PORT must be an integer between 1 and 65535, got %q", v))
		} else {
			cfg.Port = port
		}
	}

	if v := getenv("LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			errs = append(errs, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", v))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// splitList は、カンマ区切りの値を分割し、前後の空白と空の要素を取り除く。
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
