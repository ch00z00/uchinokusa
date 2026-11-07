package postgres

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// ReminderRepository は、リマインダーの対象と、送信の記録（reminder_dispatches）を扱う（job.ReminderStore）。
type ReminderRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewReminderRepository(pool *pgxpool.Pool) *ReminderRepository {
	return &ReminderRepository{pool: pool, q: sqlcgen.New(pool)}
}

// Candidates は、リマインダーが有効で、端末があるメンバーを返す。時刻の判定はジョブが行う
// （端末のタイムゾーンごとに日付と時刻が違い、日付をまたぐ判定もあるため。家族の規模ではすべて読んでも軽い）。
func (r *ReminderRepository) Candidates(ctx context.Context) ([]domain.ReminderCandidate, error) {
	rows, err := r.q.ListReminderCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reminder candidates: %w", err)
	}
	out := make([]domain.ReminderCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ReminderCandidate{
			MembershipID: row.MembershipID, HouseholdID: row.HouseholdID, UserID: row.UserID,
			ReminderTime: fromPgTime(row.ReminderTime), TimeZone: row.TimeZone,
		})
	}
	return out, nil
}

// errReminderNotSent は、送れなかったので、送信の記録を取り消す（ロールバックする）ための印である。
var errReminderNotSent = errors.New("reminder not sent")

// Dispatch は、1つのトランザクションで、世帯の date 以前の未完了タスクを読み、その日の送信の記録を作り、send に渡す。
//   - 未完了タスクがなければ、送らず、記録も作らない（その日のうちにタスクが戻れば、次の実行で送る）
//   - 記録はユニーク制約で1日1回にする。二重に実行しても、2回目は記録を作れないので送らない
//   - send がエラーを返せば、ロールバックして記録を消す（次の実行で送り直す）
func (r *ReminderRepository) Dispatch(ctx context.Context, c domain.ReminderCandidate, date civil.Date, send func(context.Context, domain.ReminderBatch) error) (domain.DispatchOutcome, error) {
	outcome := domain.DispatchSkipped
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		tasks, err := q.ListReminderTasks(ctx, sqlcgen.ListReminderTasksParams{HouseholdID: c.HouseholdID, LocalDate: pgDate(date)})
		if err != nil {
			return fmt.Errorf("list reminder tasks: %w", err)
		}
		if len(tasks) == 0 {
			return nil
		}
		if _, err := q.CreateReminderDispatch(ctx, sqlcgen.CreateReminderDispatchParams{MembershipID: c.MembershipID, LocalDate: pgDate(date)}); errors.Is(err, pgx.ErrNoRows) {
			return nil // その日はもう送った、またはリマインダーを切った・世帯を抜けた
		} else if err != nil {
			return fmt.Errorf("create reminder dispatch: %w", err)
		}

		b := domain.ReminderBatch{ReminderCandidate: c, Date: date}
		if b.HouseholdName, err = q.GetNoticeHouseholdName(ctx, c.HouseholdID); err != nil {
			return fmt.Errorf("get household name: %w", err)
		}
		for _, t := range tasks {
			item := domain.ReminderItem{Kind: domain.CareKind(t.Kind)}
			if t.CustomName != nil {
				item.CustomName = *t.CustomName
			}
			b.Items = append(b.Items, item)
		}
		if err := send(ctx, b); err != nil {
			outcome = domain.DispatchFailed
			return errReminderNotSent
		}
		outcome = domain.DispatchSent
		return nil
	})
	if err != nil && !errors.Is(err, errReminderNotSent) {
		return "", err
	}
	return outcome, nil
}
