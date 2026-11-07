package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
)

// JobLocker は、ジョブの名前ごとのロックを、PostgreSQL の advisory lock で提供する（job.Locker）。
// どの API インスタンスも同じデータベースを使うので、インスタンスをまたいで、同じジョブを同時に1つだけ動かせる。
type JobLocker struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewJobLocker(pool *pgxpool.Pool, logger *slog.Logger) *JobLocker {
	return &JobLocker{pool: pool, logger: logger}
}

// TryLock は、ジョブのロックを待たずに取る。ロックは接続が持つので、外すまでプールの接続を1本借りたままにする。
// プロセスが落ちても、接続が切れれば PostgreSQL がロックを外す。
func (l *JobLocker) TryLock(ctx context.Context, name string) (func(), bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire: %w", err)
	}
	key := "job:" + name
	q := sqlcgen.New(conn)
	locked, err := q.TryAdvisoryLock(ctx, key)
	if err != nil || !locked {
		conn.Release()
		if err != nil {
			return nil, false, fmt.Errorf("try advisory lock: %w", err)
		}
		return nil, false, nil
	}
	unlock := func() {
		// ジョブの ctx が終わっていても外せるように、ctx を切り離す
		if _, err := q.AdvisoryUnlock(context.WithoutCancel(ctx), key); err != nil {
			// 外せなかった接続をプールに戻すと、ロックを持ったまま使い回されるので、接続ごと閉じる
			l.logger.Error("advisory unlock", slog.String("job", name), slog.Any("error", err))
			_ = conn.Conn().Close(context.WithoutCancel(ctx))
		}
		conn.Release()
	}
	return unlock, true, nil
}
