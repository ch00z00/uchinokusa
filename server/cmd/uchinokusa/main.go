// uchinokusa は、API サーバー・マイグレーション・初期データの投入を、サブコマンドで実行する。
//
//	uchinokusa serve    API サーバーを起動する
//	uchinokusa migrate  データベースのマイグレーションを実行する
//	uchinokusa seed     植物マスタの初期データを投入する
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/firebase"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/config"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

const usage = `usage: uchinokusa <command>

commands:
  serve    start the API server
  migrate  run database migrations
  seed     load the plant species master data`

var (
	errUsage          = errors.New(usage)
	errNotImplemented = errors.New("not implemented yet")
)

// shutdownTimeout は、停止の合図を受けてから、処理中のリクエストの完了を待つ時間である。
// Cloud Run は SIGTERM のあと10秒で強制終了するため、それより短くする。
const shutdownTimeout = 8 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type command func(ctx context.Context, cfg config.Config, logger *slog.Logger) error

var commands = map[string]command{
	"serve":   serve,
	"migrate": migrate,
	// seed は、タスク 6.1 で植物マスタの初期データとあわせて実装する
	"seed": notImplemented("seed"),
}

// run は、サブコマンドを振り分けて実行する。テストしやすいように、外部への依存を引数で受け取る。
func run(ctx context.Context, args []string, getenv func(string) string, logOut io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}
	cmd, ok := commands[args[0]]
	if !ok {
		return errUsage
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(logging.NewHandler(logOut, cfg.LogLevel))

	return cmd(ctx, cfg, logger)
}

func notImplemented(name string) command {
	return func(context.Context, config.Config, *slog.Logger) error {
		return fmt.Errorf("%s: %w", name, errNotImplemented)
	}
}

// migrate は、まだ適用していないマイグレーションをすべて適用する。
func migrate(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	logger.Info("migrations applied")
	return nil
}

// serve は、HTTP サーバーを起動し、ctx が終わったらグレースフルに停止する。
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	verifier, err := firebase.NewVerifier(ctx, cfg.FirebaseProjectID, cfg.FirebaseCredentialsFile)
	if err != nil {
		return err
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Logger:   logger,
		Verifier: verifier,
		Accounts: app.NewAccountService(postgres.NewUserRepository(pool)),
	})
	srv := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // ヘッダーを少しずつ送る攻撃（Slowloris）への対策
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", slog.Int("port", cfg.Port))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		// ポートが使用中など、起動に失敗した場合
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}
