// Package pgtest は、結合テスト用の PostgreSQL を testcontainers で用意する。
//
// コンテナはテストのバイナリ（パッケージ）ごとに1つだけ起動する。マイグレーション済みのデータベースを
// テンプレートにして、テストごとに新しいデータベースを複製するため、テストを並行に実行しても互いに影響しない。
// コンテナは、テストの終了後に testcontainers（Ryuk）が削除する。
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
)

const (
	image = "postgres:16-alpine" // compose.yaml と本番（Cloud SQL）のメジャーバージョンにそろえる
	// templateDB は、マイグレーションを適用しておくテンプレートのデータベースである。
	templateDB = "uchinokusa_template"
)

var (
	once     sync.Once
	adminURL string // コンテナの postgres データベースへの接続先（データベースの作成に使う）
	setupErr error
)

// New は、マイグレーション済みの新しいデータベースに接続したプールを返す。
// `go test -short` のときは、テストを飛ばす。
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(t.Context(), createDatabase(t, templateDB))
	if err != nil {
		t.Fatalf("pgtest: connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// NewEmptyURL は、テーブルのない新しいデータベースの接続先を返す。マイグレーション自体のテストに使う。
func NewEmptyURL(t *testing.T) string {
	t.Helper()
	return createDatabase(t, "template0")
}

func createDatabase(t *testing.T, template string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("pgtest: skipping integration test in short mode")
	}

	once.Do(func() { setupErr = setup(context.Background()) })
	if setupErr != nil {
		t.Fatalf("pgtest: setup: %v", setupErr)
	}

	name := "test_" + randomHex()
	if err := execAdmin(t.Context(), fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, template)); err != nil {
		t.Fatalf("pgtest: create database: %v", err)
	}
	return withDatabase(adminURL, name)
}

// setup は、コンテナを起動し、テンプレートのデータベースにマイグレーションを適用する。
func setup(ctx context.Context) error {
	ctr, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	adminURL, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("connection string: %w", err)
	}

	if err := execAdmin(ctx, "CREATE DATABASE "+templateDB); err != nil {
		return err
	}
	// Migrate は終了時に接続を閉じる。テンプレートに接続が残っていると、複製に失敗する
	if err := postgres.Migrate(ctx, withDatabase(adminURL, templateDB)); err != nil {
		return fmt.Errorf("migrate template: %w", err)
	}
	return nil
}

func execAdmin(ctx context.Context, sql string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, sql)
	return err
}

// withDatabase は、接続先の URL のデータベース名を差し替える。
func withDatabase(rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err) // testcontainers が返す URL なので、壊れていることはない
	}
	u.Path = "/" + name
	return u.String()
}

func randomHex() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
