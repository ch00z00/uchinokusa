// uchinokusa は、API サーバー・マイグレーション・初期データの投入を、サブコマンドで実行する。
//
//	uchinokusa serve    API サーバーを起動する
//	uchinokusa migrate  データベースのマイグレーションを実行する
//	uchinokusa seed     植物マスタの初期データを投入する
package main

import (
	"bytes"
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
	// タイムゾーンのデータをバイナリに埋め込む。実行環境（コンテナ）に tzdata がなくても、
	// 端末のタイムゾーンの検証とリマインダーの判定ができるようにする
	_ "time/tzdata"

	"github.com/ch00z00/uchinokusa/server/db"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/firebase"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/seedfile"
	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/config"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

const usage = `usage: uchinokusa <command>

commands:
  serve    start the API server
  migrate  run database migrations
  seed     load the plant species master data`

var errUsage = errors.New(usage)

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
	"seed":    seed,
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

// seed は、植物マスタの初期データ（db/seeds/species.yaml）を検証してから投入する。
// slug をキーに追加・上書きするので、何度実行しても同じ結果になる。データに誤りがあれば、何も投入しない。
func seed(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	list, err := seedfile.ParseSpecies(bytes.NewReader(db.SpeciesSeed))
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	res, err := postgres.NewSpeciesRepository(pool).UpsertAll(ctx, list)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	logger.Info("species seeded", slog.Int("inserted", res.Inserted), slog.Int("updated", res.Updated))
	return nil
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
	if cfg.InviteBaseURL == nil {
		return errors.New("INVITE_BASE_URL is required to serve")
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	verifier, err := firebase.NewVerifier(ctx, cfg.FirebaseProjectID, cfg.FirebaseCredentialsFile)
	if err != nil {
		return err
	}

	authz := app.NewAuthorizer(postgres.NewMembershipRepository(pool))
	taskRepo := postgres.NewCareTaskRepository(pool)
	plantRepo := postgres.NewPlantRepository(pool)
	// 助言プロバイダ。新しいプロバイダは、ここに登録する（docs/decisions/0027）
	advisors, err := advisor.NewRegistry(advisor.Repotting{})
	if err != nil {
		return err
	}
	suggestions := app.NewSuggestionService(postgres.NewSuggestionRepository(pool), advisors, authz, time.Now, logger)
	router := httpapi.NewRouter(httpapi.Deps{
		Logger:      logger,
		Verifier:    verifier,
		Accounts:    app.NewAccountService(postgres.NewUserRepository(pool), postgres.NewDeviceRepository(pool)),
		Households:  app.NewHouseholdService(postgres.NewHouseholdRepository(pool), authz),
		Members:     app.NewMembershipService(postgres.NewMemberRepository(pool), authz),
		Invitations: app.NewInvitationService(postgres.NewInvitationRepository(pool), authz, cfg.InviteBaseURL),
		Species:     app.NewSpeciesService(postgres.NewSpeciesRepository(pool)),
		Locations:   app.NewLocationService(postgres.NewLocationRepository(pool), authz),
		Plants:      app.NewPlantService(plantRepo, authz, suggestions),
		Tasks:       app.NewCareTaskService(taskRepo, authz, time.Now, suggestions),
		Records:     app.NewCareRecordService(postgres.NewCareRecordRepository(pool), taskRepo, authz, time.Now),
		Today:       app.NewTodayService(postgres.NewTodayRepository(pool), authz, time.Now),
		Notes:       app.NewNoteService(postgres.NewNoteRepository(pool), plantRepo, authz),
		Calendar:    app.NewCalendarService(postgres.NewCalendarRepository(pool), authz, time.Now),
		Suggestions: suggestions,
	})
	srv := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // ヘッダーを少しずつ送る攻撃（Slowloris）への対策
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// 変更イベントを受け取る（どのインスタンスで起きた変更も届く）。WebSocket への配信はタスク 11.2 でつなぐ
	listenerCtx, stopListener := context.WithCancel(ctx)
	defer stopListener()
	go func() {
		_ = postgres.NewChangeListener(pool, logger).Run(listenerCtx, func(e domain.ChangeEvent) {
			logger.Debug("household changed", slog.String("household_id", e.HouseholdID.String()), slog.Int64("version", e.Version),
				slog.String("entity", string(e.Entity)), slog.String("op", string(e.Op)))
		})
	}()

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
