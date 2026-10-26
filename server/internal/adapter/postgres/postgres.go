// Package postgres は、PostgreSQL への接続、マイグレーション、sqlc で生成したクエリを使うリポジトリを提供する。
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ch00z00/uchinokusa/server/db"
)

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
