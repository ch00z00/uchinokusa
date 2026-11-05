package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CareRecordService は、お手入れの記録（完了・スキップ・取り消し・まとめて完了・編集・削除・履歴）を行う（実装は app.CareRecordService）。
type CareRecordService interface {
	Complete(ctx context.Context, userID, taskID uuid.UUID, in domain.CompletionInput) (domain.CareRecord, domain.CareTask, error)
	Skip(ctx context.Context, userID, taskID uuid.UUID, in domain.SkipInput) (domain.CareRecord, domain.CareTask, error)
	Undo(ctx context.Context, userID, recordID uuid.UUID) (domain.CareTask, error)
	CompleteBatch(ctx context.Context, userID, householdID uuid.UUID, items []app.BatchCompletionItem) ([]app.BatchCompletionResult, error)
	Update(ctx context.Context, userID, recordID uuid.UUID, in domain.RecordPatchInput) (domain.CareRecord, domain.CareTask, error)
	Delete(ctx context.Context, userID, recordID uuid.UUID) (domain.CareTask, error)
	History(ctx context.Context, userID, plantID uuid.UUID, in domain.RecordQueryInput) (domain.RecordPage, error)
}

func (s *server) CompleteCareTask(ctx context.Context, req oapi.CompleteCareTaskRequestObject) (oapi.CompleteCareTaskResponseObject, error) {
	b := req.Body
	rec, task, err := s.records.Complete(ctx, currentUser(ctx).ID, req.TaskId, domain.CompletionInput{
		CycleSeq:    &b.CycleSeq,
		PerformedOn: b.PerformedOn,
		AllowExtra:  b.AllowExtra != nil && *b.AllowExtra,
	})
	// 完了済みの回は、誰がいつ完了したかと最新のタスクを返す。アプリは楽観的な反映を戻して表示する（要件11-2）
	var dup *domain.AlreadyCompletedError
	if errors.As(err, &dup) {
		t := toCareTask(dup.Task)
		existing := toCompletedBy(dup.Existing)
		return oapi.CompleteCareTask409JSONResponse{Code: oapi.ErrorCodeAlreadyCompleted, Message: dup.Error(), Existing: &existing, Task: &t}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.CompleteCareTask201JSONResponse{Record: toCareRecord(rec), Task: toCareTask(task)}, nil
}

func (s *server) CompleteCareTasks(ctx context.Context, req oapi.CompleteCareTasksRequestObject) (oapi.CompleteCareTasksResponseObject, error) {
	items := make([]app.BatchCompletionItem, len(req.Body.Items))
	for i, it := range req.Body.Items {
		items[i] = app.BatchCompletionItem{TaskID: it.TaskId, Input: domain.CompletionInput{
			CycleSeq:    &it.CycleSeq,
			PerformedOn: it.PerformedOn,
			AllowExtra:  it.AllowExtra != nil && *it.AllowExtra,
		}}
	}
	results, err := s.records.CompleteBatch(ctx, currentUser(ctx).ID, req.HouseholdId, items)
	if err != nil {
		return nil, err
	}

	out := make([]oapi.CompleteCareTasksItemResult, len(results))
	for i, r := range results {
		out[i] = s.toBatchResult(ctx, r)
	}
	return oapi.CompleteCareTasks200JSONResponse{Results: out}, nil
}

// toBatchResult は、まとめて完了の1件の結果を、単独の完了と同じ内容（重複なら誰がいつ、失敗ならエラーのコード）にする。
func (s *server) toBatchResult(ctx context.Context, r app.BatchCompletionResult) oapi.CompleteCareTasksItemResult {
	out := oapi.CompleteCareTasksItemResult{TaskId: r.TaskID}
	var dup *domain.AlreadyCompletedError
	switch {
	case r.Err == nil:
		rec, t := toCareRecord(r.Record), toCareTask(r.Task)
		out.Status, out.Record, out.Task = oapi.CompleteCareTasksItemResultStatusCompleted, &rec, &t
	case errors.As(r.Err, &dup):
		existing, t := toCompletedBy(dup.Existing), toCareTask(dup.Task)
		out.Status, out.Existing, out.Task = oapi.CompleteCareTasksItemResultStatusAlreadyCompleted, &existing, &t
	default:
		status, body := errorResponse(r.Err)
		if status == http.StatusInternalServerError {
			logInternal(ctx, s.logger, r.Err)
		}
		out.Status, out.Error = oapi.CompleteCareTasksItemResultStatusFailed, &body
	}
	return out
}

func (s *server) UndoCareRecord(ctx context.Context, req oapi.UndoCareRecordRequestObject) (oapi.UndoCareRecordResponseObject, error) {
	task, err := s.records.Undo(ctx, currentUser(ctx).ID, req.RecordId)
	if err != nil {
		return nil, err
	}
	return oapi.UndoCareRecord200JSONResponse(toCareTask(task)), nil
}

func (s *server) UpdateCareRecord(ctx context.Context, req oapi.UpdateCareRecordRequestObject) (oapi.UpdateCareRecordResponseObject, error) {
	rec, task, err := s.records.Update(ctx, currentUser(ctx).ID, req.RecordId, domain.RecordPatchInput{PerformedOn: &req.Body.PerformedOn})
	if err != nil {
		return nil, err
	}
	return oapi.UpdateCareRecord200JSONResponse{Record: toCareRecord(rec), Task: omitZeroTask(task)}, nil
}

func (s *server) DeleteCareRecord(ctx context.Context, req oapi.DeleteCareRecordRequestObject) (oapi.DeleteCareRecordResponseObject, error) {
	task, err := s.records.Delete(ctx, currentUser(ctx).ID, req.RecordId)
	if err != nil {
		return nil, err
	}
	return oapi.DeleteCareRecord200JSONResponse{Task: omitZeroTask(task)}, nil
}

func (s *server) ListPlantRecords(ctx context.Context, req oapi.ListPlantRecordsRequestObject) (oapi.ListPlantRecordsResponseObject, error) {
	p := req.Params
	page, err := s.records.History(ctx, currentUser(ctx).ID, req.PlantId, domain.RecordQueryInput{Kind: p.Kind, Cursor: p.Cursor, Limit: p.Limit})
	if err != nil {
		return nil, err
	}
	out := oapi.ListPlantRecords200JSONResponse{
		Records:       make([]oapi.CareRecord, len(page.Records)),
		NextCursor:    omitEmptyAs[string](page.NextCursor),
		LastPerformed: make([]oapi.LastPerformed, len(page.LastPerformed)),
	}
	for i, r := range page.Records {
		out.Records[i] = toCareRecord(r)
	}
	for i, l := range page.LastPerformed {
		out.LastPerformed[i] = oapi.LastPerformed{Kind: oapi.CareKind(l.Kind), CustomName: omitEmptyAs[string](l.CustomName), PerformedOn: toDate(l.PerformedOn)}
	}
	return out, nil
}

// omitZeroTask は、タスクがない（削除した）ときに、項目ごと省く。
func omitZeroTask(t domain.CareTask) *oapi.CareTask {
	if t.ID == uuid.Nil {
		return nil
	}
	c := toCareTask(t)
	return &c
}

func (s *server) SkipCareTask(ctx context.Context, req oapi.SkipCareTaskRequestObject) (oapi.SkipCareTaskResponseObject, error) {
	b := req.Body
	rec, task, err := s.records.Skip(ctx, currentUser(ctx).ID, req.TaskId, domain.SkipInput{
		CycleSeq:   &b.CycleSeq,
		Reason:     b.Reason,
		ReasonText: b.ReasonText,
		RecheckOn:  b.RecheckOn,
	})
	if err != nil {
		return nil, err
	}
	return oapi.SkipCareTask201JSONResponse{Record: toCareRecord(rec), Task: toCareTask(task)}, nil
}

func toCompletedBy(r domain.CareRecord) oapi.CompletedBy {
	return oapi.CompletedBy{
		RecordId:         r.ID,
		ActorDisplayName: r.ActorDisplayName,
		PerformedOn:      toDate(r.PerformedOn),
		PerformedAt:      r.PerformedAt.UTC(),
	}
}

// toCareRecord は、未設定の項目（削除したタスク、削除したアカウントなど）を、項目ごと省いた形にする。
func toCareRecord(r domain.CareRecord) oapi.CareRecord {
	return oapi.CareRecord{
		Id:               r.ID,
		PlantId:          r.PlantID,
		TaskId:           omitNilUUID(r.TaskID),
		Kind:             oapi.CareKind(r.TaskKind),
		CustomName:       omitEmptyAs[string](r.TaskCustomName),
		Outcome:          oapi.CareOutcome(r.Outcome),
		PerformedOn:      toDate(r.PerformedOn),
		PerformedAt:      r.PerformedAt.UTC(),
		ActorUserId:      omitNilUUID(r.ActorUserID),
		ActorDisplayName: r.ActorDisplayName,
		CycleSeq:         r.CycleSeq,
		IsExtra:          r.IsExtra,
		SkipReason:       omitEmptyAs[oapi.SkipReason](r.SkipReason),
		SkipReasonText:   omitEmptyAs[string](r.SkipReasonText),
		RecheckOn:        omitZeroDate(r.RecheckOn),
		CreatedAt:        r.CreatedAt.UTC(),
	}
}

func omitZeroDate(d civil.Date) *openapi_types.Date {
	if d.IsZero() {
		return nil
	}
	t := toDate(d)
	return &t
}

func toDate(d civil.Date) openapi_types.Date {
	return openapi_types.Date{Time: d.In(time.UTC)}
}
