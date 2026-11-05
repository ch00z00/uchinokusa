package httpapi

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CareRecordService は、お手入れの記録（完了）を行う（実装は app.CareRecordService）。
type CareRecordService interface {
	Complete(ctx context.Context, userID, taskID uuid.UUID, in domain.CompletionInput) (domain.CareRecord, domain.CareTask, error)
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
		return oapi.CompleteCareTask409JSONResponse{
			Code:    oapi.ErrorCodeAlreadyCompleted,
			Message: dup.Error(),
			Existing: &oapi.CompletedBy{
				RecordId:         dup.Existing.ID,
				ActorDisplayName: dup.Existing.ActorDisplayName,
				PerformedOn:      toDate(dup.Existing.PerformedOn),
				PerformedAt:      dup.Existing.PerformedAt.UTC(),
			},
			Task: &t,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return oapi.CompleteCareTask201JSONResponse{Record: toCareRecord(rec), Task: toCareTask(task)}, nil
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
		CreatedAt:        r.CreatedAt.UTC(),
	}
}

func toDate(d civil.Date) openapi_types.Date {
	return openapi_types.Date{Time: d.In(time.UTC)}
}
