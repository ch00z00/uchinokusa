package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// oneCompletionPerCycleIdx は、同じ予定の回への完了を1件に限る部分ユニーク制約の名前である。
const oneCompletionPerCycleIdx = "care_records_one_completion_per_cycle_idx"

// CareRecordRepository は、app.CareRecordRepository の PostgreSQL の実装である。
type CareRecordRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

// NewCareRecordRepository は、CareRecordRepository を作る。
func NewCareRecordRepository(pool *pgxpool.Pool) *CareRecordRepository {
	return &CareRecordRepository{pool: pool, q: sqlcgen.New(pool)}
}

// Complete は、タスクの完了を記録する。タスクの行ロック → 予定の回の照合（domain.PlanCompletion） → 記録の作成 →
// タスクの更新 → 完了通知の予約を、1つのトランザクションで行う（設計の判断2）。
// 同じタスクへの同時の完了は行ロックで順に処理されるので、後のほうは先の完了を見て *domain.AlreadyCompletedError になる。
func (r *CareRecordRepository) Complete(ctx context.Context, householdID, taskID, actorID uuid.UUID, c domain.Completion) (domain.CareRecord, domain.CareTask, error) {
	var (
		record domain.CareRecord
		task   domain.CareTask
	)
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		locked, current, err := lockCareTask(ctx, q, householdID, taskID)
		if err != nil {
			return err
		}

		plan, err := domain.PlanCompletion(current, c, domain.Hemisphere(locked.Hemisphere))
		var conflict *domain.ConflictError
		if errors.As(err, &conflict) && conflict.Code == domain.ConflictAlreadyCompleted {
			return alreadyCompleted(ctx, q, current, c.CycleSeq)
		}
		if err != nil {
			return err
		}

		params := sqlcgen.CreateCompletionRecordParams{
			HouseholdID:    householdID,
			PlantID:        current.PlantID,
			TaskID:         nullUUID(taskID),
			TaskKind:       string(current.Kind),
			TaskCustomName: nullIfEmpty(current.CustomName),
			PerformedOn:    pgDate(c.PerformedOn),
			CycleSeq:       int32(plan.RecordCycleSeq),
			IsExtra:        plan.Extra,
			ActorUserID:    actorID,
		}
		if !plan.Extra {
			// 取り消し（8.3）で戻す先。追加の記録はタスクを変えないので、戻す先を持たない
			params.PrevNextDueOn = locked.CareTask.NextDueOn
			params.PrevCycleSeq = pgtype.Int4{Int32: locked.CareTask.CycleSeq, Valid: true}
			params.PrevDueOverridden = &locked.CareTask.DueOverridden
		}
		row, err := q.CreateCompletionRecord(ctx, params)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == oneCompletionPerCycleIdx {
			// 行ロックがあるため通常は起きない。ロックを通らない書き込みがあっても、最後は制約が重複を防ぐ
			return &domain.ConflictError{Code: domain.ConflictAlreadyCompleted}
		}
		if err != nil {
			return fmt.Errorf("create completion record: %w", err)
		}
		record = toDomainCareRecord(row)
		ch.add(householdID, domain.ChangeRecord, record.ID, domain.ChangeCreated, current.PlantID)

		task = current
		if !plan.Extra {
			ch.add(householdID, domain.ChangeTask, taskID, domain.ChangeUpdated, current.PlantID)
			advanced, err := q.AdvanceCareTask(ctx, sqlcgen.AdvanceCareTaskParams{ID: taskID, NextDueOn: pgDate(plan.NextDueOn), CycleSeq: int32(plan.NextCycleSeq)})
			if err != nil {
				return fmt.Errorf("advance care task: %w", err)
			}
			if task, err = toDomainCareTask(advanced); err != nil {
				return err
			}
		}

		if err := q.EnqueueCompletion(ctx, sqlcgen.EnqueueCompletionParams{HouseholdID: householdID, ActorUserID: nullUUID(actorID), RecordID: nullUUID(record.ID)}); err != nil {
			return fmt.Errorf("enqueue completion: %w", err)
		}
		return nil
	})
	return record, task, err
}

// Skip は、タスクのスキップを記録する。タスクの行ロック → 予定の回の確認（domain.CheckSkip） → 記録の作成 →
// 次回予定を確認日にする更新を、1つのトランザクションで行う。予定の回は進めず、完了通知も予約しない（要件16-7）。
func (r *CareRecordRepository) Skip(ctx context.Context, householdID, taskID, actorID uuid.UUID, s domain.Skip) (domain.CareRecord, domain.CareTask, error) {
	var (
		record domain.CareRecord
		task   domain.CareTask
	)
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		locked, current, err := lockCareTask(ctx, q, householdID, taskID)
		if err != nil {
			return err
		}
		if err := domain.CheckSkip(current, s); err != nil {
			return err
		}

		row, err := q.CreateSkipRecord(ctx, sqlcgen.CreateSkipRecordParams{
			HouseholdID:    householdID,
			PlantID:        current.PlantID,
			TaskID:         nullUUID(taskID),
			TaskKind:       string(current.Kind),
			TaskCustomName: nullIfEmpty(current.CustomName),
			PerformedOn:    pgDate(s.PerformedOn),
			SkipReasonCode: nullIfEmpty(string(s.Reason)),
			SkipReasonText: nullIfEmpty(s.ReasonText),
			RecheckOn:      pgDate(s.RecheckOn),
			CycleSeq:       int32(s.CycleSeq),
			// 取り消し（8.3）で戻す先
			PrevNextDueOn:     locked.CareTask.NextDueOn,
			PrevCycleSeq:      pgtype.Int4{Int32: locked.CareTask.CycleSeq, Valid: true},
			PrevDueOverridden: &locked.CareTask.DueOverridden,
			ActorUserID:       actorID,
		})
		if err != nil {
			return fmt.Errorf("create skip record: %w", err)
		}
		record = toDomainCareRecord(row)
		ch.add(householdID, domain.ChangeRecord, record.ID, domain.ChangeCreated, current.PlantID)
		ch.add(householdID, domain.ChangeTask, taskID, domain.ChangeUpdated, current.PlantID)

		deferred, err := q.DeferCareTask(ctx, sqlcgen.DeferCareTaskParams{ID: taskID, NextDueOn: pgDate(s.RecheckOn)})
		if err != nil {
			return fmt.Errorf("defer care task: %w", err)
		}
		task, err = toDomainCareTask(deferred)
		return err
	})
	return record, task, err
}

// HouseholdOf は、記録が属する世帯の ID を返す。記録がなければ domain.ErrNotFound を返す。
func (r *CareRecordRepository) HouseholdOf(ctx context.Context, recordID uuid.UUID) (uuid.UUID, error) {
	householdID, err := r.q.GetCareRecordHouseholdID(ctx, recordID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get care record household: %w", err)
	}
	return householdID, nil
}

// Undo は、記録を取り消す。タスクの行ロック → 取り消せるかの確認（domain.CheckUndo） → 記録する前の予定日・回・手で変えた印に戻す →
// まだ送っていない完了通知を取り消し済みにする → 記録を消す、を1つのトランザクションで行う（設計の「取り消しの条件」）。
// 追加の記録は、タスクを変えていないので、記録を消すだけにする。
func (r *CareRecordRepository) Undo(ctx context.Context, householdID, recordID, actorID uuid.UUID, now time.Time) (domain.CareTask, error) {
	var task domain.CareTask
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.GetCareRecord(ctx, sqlcgen.GetCareRecordParams{ID: recordID, HouseholdID: householdID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get care record: %w", err)
		}
		// タスクを削除していれば、戻す先がない
		if !row.TaskID.Valid {
			return domain.ErrNotFound
		}
		// 完了・スキップと同じ行ロックを取り、そのあとで最新の記録を確かめる（ロックの前に確かめると、同時の完了を見落とす）
		_, task, err = lockCareTask(ctx, q, householdID, row.TaskID.UUID)
		if err != nil {
			return err
		}
		latest, err := q.GetLatestTaskRecord(ctx, row.TaskID)
		if err != nil {
			return fmt.Errorf("get latest task record: %w", err)
		}
		if err := domain.CheckUndo(toDomainCareRecord(row), latest.ID, actorID, now); err != nil {
			return err
		}

		if row.PrevCycleSeq.Valid {
			restored, err := q.RestoreCareTask(ctx, sqlcgen.RestoreCareTaskParams{
				ID:            row.TaskID.UUID,
				NextDueOn:     row.PrevNextDueOn,
				CycleSeq:      row.PrevCycleSeq.Int32,
				DueOverridden: row.PrevDueOverridden != nil && *row.PrevDueOverridden,
			})
			if err != nil {
				return fmt.Errorf("restore care task: %w", err)
			}
			if task, err = toDomainCareTask(restored); err != nil {
				return err
			}
			ch.add(householdID, domain.ChangeTask, task.ID, domain.ChangeUpdated, row.PlantID)
		}
		if err := q.CancelCompletionNotice(ctx, nullUUID(recordID)); err != nil {
			return fmt.Errorf("cancel completion notice: %w", err)
		}
		if err := q.DeleteCareRecord(ctx, recordID); err != nil {
			return fmt.Errorf("delete care record: %w", err)
		}
		ch.add(householdID, domain.ChangeRecord, recordID, domain.ChangeDeleted, row.PlantID)
		return nil
	})
	return task, err
}

// lockCareTask は、完了・スキップの前に、世帯のタスクの行をロックする。同じタスクへの記録は、先のコミットを待って順に処理される。
func lockCareTask(ctx context.Context, q *sqlcgen.Queries, householdID, taskID uuid.UUID) (sqlcgen.LockCareTaskRow, domain.CareTask, error) {
	locked, err := q.LockCareTask(ctx, sqlcgen.LockCareTaskParams{ID: taskID, HouseholdID: householdID})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.LockCareTaskRow{}, domain.CareTask{}, domain.ErrNotFound
	}
	if err != nil {
		return sqlcgen.LockCareTaskRow{}, domain.CareTask{}, fmt.Errorf("lock care task: %w", err)
	}
	current, err := toDomainCareTask(locked.CareTask)
	return locked, current, err
}

// alreadyCompleted は、完了済みの回について、先に完了した記録と最新のタスクを持つエラーを作る。
// 回が今より前でも、完了の記録がなければ（取り消しなどで）、アプリの表示が古いものとして stale_cycle にする。
func alreadyCompleted(ctx context.Context, q *sqlcgen.Queries, current domain.CareTask, cycleSeq int) error {
	row, err := q.GetCycleCompletion(ctx, sqlcgen.GetCycleCompletionParams{TaskID: nullUUID(current.ID), CycleSeq: int32(cycleSeq)})
	if errors.Is(err, pgx.ErrNoRows) {
		return &domain.ConflictError{Code: domain.ConflictStaleCycle}
	}
	if err != nil {
		return fmt.Errorf("get cycle completion: %w", err)
	}
	return &domain.AlreadyCompletedError{Existing: toDomainCareRecord(row), Task: current}
}

func toDomainCareRecord(row sqlcgen.CareRecord) domain.CareRecord {
	return domain.CareRecord{
		ID:               row.ID,
		HouseholdID:      row.HouseholdID,
		PlantID:          row.PlantID,
		TaskID:           row.TaskID.UUID,
		TaskKind:         domain.CareKind(row.TaskKind),
		TaskCustomName:   emptyIfNull(row.TaskCustomName),
		Outcome:          domain.CareOutcome(row.Outcome),
		PerformedOn:      civilDate(row.PerformedOn),
		PerformedAt:      row.PerformedAt,
		ActorUserID:      row.ActorUserID.UUID,
		ActorDisplayName: row.ActorDisplayName,
		CycleSeq:         int(row.CycleSeq),
		IsExtra:          row.IsExtra,
		SkipReason:       domain.SkipReason(emptyIfNull(row.SkipReasonCode)),
		SkipReasonText:   emptyIfNull(row.SkipReasonText),
		RecheckOn:        civilDate(row.RecheckOn),
		CreatedAt:        row.CreatedAt,
	}
}
