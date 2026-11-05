package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// UpdatePerformedOn は、記録の実施日を変える。最新の完了であれば、次回予定を計算し直す（domain.RescheduleAfterChange）。
// タスクを削除していれば、記録だけを変え、ゼロ値のタスクを返す。
func (r *CareRecordRepository) UpdatePerformedOn(ctx context.Context, householdID, recordID uuid.UUID, p domain.RecordPatch) (domain.CareRecord, domain.CareTask, error) {
	var record domain.CareRecord
	task, err := r.changeRecord(ctx, householdID, recordID, func(q *sqlcgen.Queries) error {
		row, err := q.UpdateCareRecordPerformedOn(ctx, sqlcgen.UpdateCareRecordPerformedOnParams{ID: recordID, PerformedOn: pgDate(p.PerformedOn)})
		if err != nil {
			return fmt.Errorf("update care record: %w", err)
		}
		record = toDomainCareRecord(row)
		return nil
	})
	return record, task, err
}

// Delete は、記録を消す。最新の完了であれば、残った最新の完了から次回予定を計算し直す。
// 予定の回は戻さない。まだ送っていない完了通知は、取り消し済みにする（要件16-6）。
func (r *CareRecordRepository) Delete(ctx context.Context, householdID, recordID uuid.UUID) (domain.CareTask, error) {
	return r.changeRecord(ctx, householdID, recordID, func(q *sqlcgen.Queries) error {
		if err := q.CancelCompletionNotice(ctx, nullUUID(recordID)); err != nil {
			return fmt.Errorf("cancel completion notice: %w", err)
		}
		if err := q.DeleteCareRecord(ctx, recordID); err != nil {
			return fmt.Errorf("delete care record: %w", err)
		}
		return nil
	})
}

// changeRecord は、記録の編集・削除（apply）を、タスクの行ロックの中で行い、必要なら次回予定を計算し直す。
// 完了・スキップ・取り消しと同じロックを取るので、同時の記録と食い違わない。
func (r *CareRecordRepository) changeRecord(ctx context.Context, householdID, recordID uuid.UUID, apply func(*sqlcgen.Queries) error) (domain.CareTask, error) {
	var task domain.CareTask
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.GetCareRecord(ctx, sqlcgen.GetCareRecordParams{ID: recordID, HouseholdID: householdID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get care record: %w", err)
		}
		if !row.TaskID.Valid {
			return apply(q)
		}
		locked, current, err := lockCareTask(ctx, q, householdID, row.TaskID.UUID)
		if errors.Is(err, domain.ErrNotFound) {
			// タスクを削除していれば、予定はないので、記録だけを変える
			return apply(q)
		}
		if err != nil {
			return err
		}
		task = current

		owner, err := q.GetScheduleOwner(ctx, row.TaskID)
		if err != nil {
			return fmt.Errorf("get schedule owner: %w", err)
		}
		change := domain.RecordChange{Target: toDomainCareRecord(row), ScheduleOwner: toDomainCareRecord(owner)}
		if change.LatestBefore, err = latestCompletion(ctx, q, row.TaskID); err != nil {
			return err
		}
		if err := apply(q); err != nil {
			return err
		}
		after, err := latestCompletion(ctx, q, row.TaskID)
		if err != nil {
			return err
		}
		if after.ID != uuid.Nil {
			change.LatestAfter = &after
		}

		next, ok := domain.RescheduleAfterChange(current, change, domain.Hemisphere(locked.Hemisphere))
		if !ok {
			return nil
		}
		rescheduled, err := q.RescheduleCareTask(ctx, sqlcgen.RescheduleCareTaskParams{ID: current.ID, NextDueOn: pgDate(next)})
		if err != nil {
			return fmt.Errorf("reschedule care task: %w", err)
		}
		task, err = toDomainCareTask(rescheduled)
		return err
	})
	return task, err
}

// latestCompletion は、タスクの最新の完了を返す。完了がなければ、ゼロ値を返す。
func latestCompletion(ctx context.Context, q *sqlcgen.Queries, taskID uuid.NullUUID) (domain.CareRecord, error) {
	row, err := q.GetLatestCompletion(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CareRecord{}, nil
	}
	if err != nil {
		return domain.CareRecord{}, fmt.Errorf("get latest completion: %w", err)
	}
	return toDomainCareRecord(row), nil
}

// ListByPlant は、植物の履歴を新しい順に q.Limit 件まで返す。more は、次のページがあるかを表す（1件多く読んで確かめる）。
func (r *CareRecordRepository) ListByPlant(ctx context.Context, householdID, plantID uuid.UUID, q domain.RecordQuery) (records []domain.CareRecord, more bool, err error) {
	params := sqlcgen.ListPlantRecordsParams{
		HouseholdID: householdID,
		PlantID:     plantID,
		Kind:        nullIfEmpty(string(q.Kind)),
		RowLimit:    int32(q.Limit + 1),
	}
	if q.After != nil {
		params.AfterPerformedOn = pgDate(q.After.PerformedOn)
		params.AfterCreatedAt = &q.After.CreatedAt
		params.AfterID = nullUUID(q.After.ID)
	}
	rows, err := r.q.ListPlantRecords(ctx, params)
	if err != nil {
		return nil, false, fmt.Errorf("list plant records: %w", err)
	}
	if more = len(rows) > q.Limit; more {
		rows = rows[:q.Limit]
	}
	records = make([]domain.CareRecord, len(rows))
	for i, row := range rows {
		records[i] = toDomainCareRecord(row)
	}
	return records, more, nil
}

// LastPerformed は、植物の、作業の種類（other は作業名）ごとの最終実施日を返す（要件12-3）。
func (r *CareRecordRepository) LastPerformed(ctx context.Context, householdID, plantID uuid.UUID) ([]domain.LastPerformed, error) {
	rows, err := r.q.ListLastPerformed(ctx, sqlcgen.ListLastPerformedParams{HouseholdID: householdID, PlantID: plantID})
	if err != nil {
		return nil, fmt.Errorf("list last performed: %w", err)
	}
	out := make([]domain.LastPerformed, len(rows))
	for i, row := range rows {
		out[i] = domain.LastPerformed{Kind: domain.CareKind(row.TaskKind), CustomName: emptyIfNull(row.TaskCustomName), PerformedOn: civilDate(row.PerformedOn)}
	}
	return out, nil
}
