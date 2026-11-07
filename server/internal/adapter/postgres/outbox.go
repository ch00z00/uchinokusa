package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// OutboxRepository は、通知のアウトボックス（notification_outbox）から、まとめて送る通知を取り出す（job.Outbox）。
type OutboxRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool, q: sqlcgen.New(pool)}
}

// ReadyGroups は、送ってよいまとまりを、待っている時間の長い順に limit 件まで返す。
func (r *OutboxRepository) ReadyGroups(ctx context.Context, quietBefore, oldestBefore time.Time, limit int) ([]domain.NoticeGroup, error) {
	rows, err := r.q.ReadyNoticeGroups(ctx, sqlcgen.ReadyNoticeGroupsParams{QuietBefore: quietBefore, OldestBefore: oldestBefore, RowLimit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("ready notice groups: %w", err)
	}
	out := make([]domain.NoticeGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.NoticeGroup{HouseholdID: row.HouseholdID, Kind: domain.NoticeKind(row.Kind), ActorUserID: row.ActorUserID.UUID})
	}
	return out, nil
}

// Dispatch は、1つのトランザクションで、まとまりの未送信の行をロックし、送る内容を send に渡し、結果を行に書く。
//   - 送信と「送信済み」への更新を同じトランザクションで行うので、二重に実行しても送り直さない
//   - 取り消し（CancelCompletionNotice）が行をロックしていれば、その行は飛ばす（取り消しが確定すれば送らない。やめれば次に送る）。
//     こちらが先にロックしていれば、取り消しは送り終わるのを待ち、送信済みの行は変えない
//   - 記録が消えた行（植物の削除など）と、参加したユーザーが退会した行は、送らずに取り消し済みにする
//
// FCM への送信の間も行をロックしたままにする。ロックするのは、このまとまりの数行だけである。
func (r *OutboxRepository) Dispatch(ctx context.Context, g domain.NoticeGroup, send func(context.Context, domain.NoticeBatch) error) (domain.DispatchOutcome, error) {
	outcome := domain.DispatchSkipped
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		actor := nullUUID(g.ActorUserID)
		rows, err := q.LockNoticeGroup(ctx, sqlcgen.LockNoticeGroupParams{HouseholdID: g.HouseholdID, Kind: string(g.Kind), ActorUserID: actor})
		if err != nil {
			return fmt.Errorf("lock notice group: %w", err)
		}
		if len(rows) == 0 {
			return nil // ほかで処理した、または取り消し中
		}
		ids := make([]uuid.UUID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}

		b := domain.NoticeBatch{NoticeGroup: g}
		var deliver []uuid.UUID // 送る行（ほかは取り消し済みにする）
		switch g.Kind {
		case domain.NoticeCompletion:
			items, err := q.ListNoticeCompletions(ctx, ids)
			if err != nil {
				return fmt.Errorf("list notice completions: %w", err)
			}
			for _, it := range items {
				deliver = append(deliver, it.OutboxID)
				item := domain.CompletionItem{PlantID: it.PlantID, PlantName: it.PlantName, Kind: domain.CareKind(it.TaskKind)}
				if it.TaskCustomName != nil {
					item.CustomName = *it.TaskCustomName
				}
				b.Items = append(b.Items, item)
				// 作業者名は、記録の時点の名前（退会しても残る。要件4-7）
				b.ActorName = it.ActorDisplayName
			}
			if len(deliver) > 0 {
				b.Recipients, err = q.ListCompletionRecipients(ctx, sqlcgen.ListCompletionRecipientsParams{HouseholdID: g.HouseholdID, ActorUserID: actor})
				if err != nil {
					return fmt.Errorf("list completion recipients: %w", err)
				}
			}
		case domain.NoticeMemberJoined:
			// 参加したユーザーが退会していれば（actor_user_id が NULL）送らない。行をロックしている間は、
			// ユーザーの削除（行の actor_user_id を NULL にする）が待つので、ここで名前が見つからないことはない
			if actor.Valid {
				name, err := q.GetNoticeUserName(ctx, g.ActorUserID)
				if err != nil {
					return fmt.Errorf("get notice user name: %w", err)
				}
				b.ActorName, deliver = name, ids
			}
			if len(deliver) > 0 {
				b.Recipients, err = q.ListHouseholdOwners(ctx, sqlcgen.ListHouseholdOwnersParams{HouseholdID: g.HouseholdID, ActorUserID: actor})
				if err != nil {
					return fmt.Errorf("list household owners: %w", err)
				}
			}
		}

		if canceled := without(ids, deliver); len(canceled) > 0 {
			if err := q.CancelNotices(ctx, canceled); err != nil {
				return fmt.Errorf("cancel notices: %w", err)
			}
		}
		if len(deliver) == 0 {
			return nil
		}
		if b.HouseholdName, err = q.GetNoticeHouseholdName(ctx, g.HouseholdID); err != nil {
			return fmt.Errorf("get notice household name: %w", err)
		}

		if err := send(ctx, b); err != nil {
			outcome = domain.DispatchFailed
			if err := q.FailNotices(ctx, sqlcgen.FailNoticesParams{Ids: deliver, MaxAttempts: domain.MaxNoticeAttempts}); err != nil {
				return fmt.Errorf("fail notices: %w", err)
			}
			return nil
		}
		outcome = domain.DispatchSent
		if err := q.MarkNoticesSent(ctx, deliver); err != nil {
			return fmt.Errorf("mark notices sent: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return outcome, nil
}

// without は、all のうち、except にないものを返す。
func without(all, except []uuid.UUID) []uuid.UUID {
	keep := map[uuid.UUID]bool{}
	for _, id := range except {
		keep[id] = true
	}
	var out []uuid.UUID
	for _, id := range all {
		if !keep[id] {
			out = append(out, id)
		}
	}
	return out
}
