// Package postgres は、PostgreSQL への接続、マイグレーション、sqlc で生成したクエリを使うリポジトリを提供する。
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ch00z00/uchinokusa/server/db"
)

// NewPool は、接続プールを作り、データベースにつながることを確かめる。
// 起動時に接続できなければ、すぐにエラーにする（Cloud Run がインスタンスを作り直す）。
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// 接続先の URL にはパスワードが含まれるため、エラーには URL を出さない
		return nil, errors.New("parse DATABASE_URL: invalid format")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// OpenSQL は、pgx をドライバーにした database/sql の接続を返す。
// goose は database/sql を前提にしているため、マイグレーションではこちらを使う。
func OpenSQL(databaseURL string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		// 接続先の URL にはパスワードが含まれるため、エラーには URL を出さない
		return nil, errors.New("parse DATABASE_URL: invalid format")
	}
	return stdlib.OpenDB(*cfg), nil
}

// Migrate は、まだ適用していないマイグレーションをすべて適用する。適用済みなら何もしない。
func Migrate(ctx context.Context, databaseURL string) error {
	sqlDB, err := OpenSQL(databaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
