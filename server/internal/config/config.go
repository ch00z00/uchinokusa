// Package config は、環境変数からサーバーの設定を読み込む。
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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
	// InviteBaseURL は、招待リンクのドメイン（INVITE_BASE_URL、例：https://invite.example.com）。
	// serve でだけ使うため、Load では必須にせず、未設定なら nil にする。
	InviteBaseURL *url.URL
	// JobAudience は、Cloud Scheduler がジョブを呼ぶときの OIDC トークンの宛先（JOB_AUDIENCE、例：https://api.example.com）。
	// JobServiceAccount は、ジョブを呼んでよいサービスアカウントのメールアドレス（JOB_SERVICE_ACCOUNT）。
	// 両方そろったときだけ、ジョブのエンドポイントを登録する。
	JobAudience       string
	JobServiceAccount string
	// Apple は、Sign in with Apple のトークンの失効に使う鍵とアプリの情報である（APPLE_TEAM_ID・APPLE_KEY_ID・APPLE_CLIENT_ID・APPLE_PRIVATE_KEY）。
	// すべてそろったときだけ設定し、なければ nil（Apple でサインインしたユーザーのアカウントは削除できない）。
	Apple *Apple
}

// Apple は、Sign in with Apple の REST API の設定である。
type Apple struct {
	TeamID     string
	KeyID      string
	ClientID   string // アプリの Bundle ID
	PrivateKey string // .p8 ファイルの中身（PEM）。Cloud Run では Secret Manager から渡す
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
		JobAudience:             getenv("JOB_AUDIENCE"),
		JobServiceAccount:       getenv("JOB_SERVICE_ACCOUNT"),
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

	if v := getenv("INVITE_BASE_URL"); v != "" {
		u, err := parseBaseURL(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("INVITE_BASE_URL %w, got %q", err, v))
		} else {
			cfg.InviteBaseURL = u
		}
	}

	switch {
	case cfg.JobAudience != "" && cfg.JobServiceAccount == "":
		errs = append(errs, errors.New("JOB_SERVICE_ACCOUNT is required when JOB_AUDIENCE is set"))
	case cfg.JobAudience == "" && cfg.JobServiceAccount != "":
		errs = append(errs, errors.New("JOB_AUDIENCE is required when JOB_SERVICE_ACCOUNT is set"))
	}

	apple := map[string]string{
		"APPLE_TEAM_ID": getenv("APPLE_TEAM_ID"), "APPLE_KEY_ID": getenv("APPLE_KEY_ID"),
		"APPLE_CLIENT_ID": getenv("APPLE_CLIENT_ID"), "APPLE_PRIVATE_KEY": getenv("APPLE_PRIVATE_KEY"),
	}
	var missing []string
	for _, key := range []string{"APPLE_TEAM_ID", "APPLE_KEY_ID", "APPLE_CLIENT_ID", "APPLE_PRIVATE_KEY"} {
		if apple[key] == "" {
			missing = append(missing, key)
		}
	}
	switch len(missing) {
	case 0:
		cfg.Apple = &Apple{TeamID: apple["APPLE_TEAM_ID"], KeyID: apple["APPLE_KEY_ID"], ClientID: apple["APPLE_CLIENT_ID"], PrivateKey: apple["APPLE_PRIVATE_KEY"]}
	case len(apple):
		// すべて未設定なら、Apple を使わない
	default:
		errs = append(errs, fmt.Errorf("%s are required when any APPLE_* is set", strings.Join(missing, ", ")))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// parseBaseURL は、パスを足して使う URL（スキームとホストだけ、または末尾のパスまで）を読む。
func parseBaseURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return nil, errors.New("must be a valid URL")
	case u.Scheme != "https" && u.Scheme != "http":
		return nil, errors.New("must start with https:// or http://")
	case u.Host == "":
		return nil, errors.New("must have a host")
	case u.RawQuery != "" || u.Fragment != "":
		return nil, errors.New("must not have a query or fragment")
	}
	return u, nil
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
