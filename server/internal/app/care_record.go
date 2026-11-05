package app

import (
	"context"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CareRecordRepository は、お手入れの記録の保存先である（実装は adapter/postgres）。
type CareRecordRepository interface {
	// Complete は、タスクの行ロックを取って domain.PlanCompletion で変更を決め、記録の作成・タスクの更新・完了通知の予約を
	// 1つのトランザクションで行う。完了済みの回なら *domain.AlreadyCompletedError を返す。
	Complete(ctx context.Context, householdID, taskID, actorID uuid.UUID, c domain.Completion) (domain.CareRecord, domain.CareTask, error)
	// Skip は、タスクの行ロックを取って domain.CheckSkip で予定の回を確かめ、記録の作成と、次回予定を確認日にする更新を
	// 1つのトランザクションで行う。完了通知は予約しない（要件16-7）。
	Skip(ctx context.Context, householdID, taskID, actorID uuid.UUID, s domain.Skip) (domain.CareRecord, domain.CareTask, error)
	// HouseholdOf は、記録が属する世帯の ID を返す。なければ domain.ErrNotFound を返す。
	HouseholdOf(ctx context.Context, recordID uuid.UUID) (uuid.UUID, error)
	// Undo は、タスクの行ロックを取って domain.CheckUndo で取り消せるかを確かめ、記録する前の予定日・回に戻して記録を消す。
	// まだ送っていない完了通知は、取り消し済みにする（要件16-6）。
	Undo(ctx context.Context, householdID, recordID, actorID uuid.UUID, now time.Time) (domain.CareTask, error)
	// UpdatePerformedOn は、記録の実施日を変え、最新の完了であれば次回予定を計算し直す（domain.RescheduleAfterChange）。
	// タスクを削除していれば、ゼロ値のタスクを返す。
	UpdatePerformedOn(ctx context.Context, householdID, recordID uuid.UUID, p domain.RecordPatch) (domain.CareRecord, domain.CareTask, error)
	// Delete は、記録を消し、最新の完了であれば、残った最新の完了から次回予定を計算し直す。
	Delete(ctx context.Context, householdID, recordID uuid.UUID) (domain.CareTask, error)
	// ListByPlant は、植物の履歴を新しい順に q.Limit 件まで返す。more は、次のページがあるかを表す。
	ListByPlant(ctx context.Context, householdID, plantID uuid.UUID, q domain.RecordQuery) (records []domain.CareRecord, more bool, err error)
	// LastPerformed は、植物の、作業の種類ごとの最終実施日を返す。
	LastPerformed(ctx context.Context, householdID, plantID uuid.UUID) ([]domain.LastPerformed, error)
}

// MaxBatchCompletions は、まとめて完了できるタスクの数の上限である。
const MaxBatchCompletions = 50

// BatchCompletionItem は、まとめて完了の1件の入力である。
type BatchCompletionItem struct {
	TaskID uuid.UUID
	Input  domain.CompletionInput
}

// BatchCompletionResult は、まとめて完了の1件の結果である。Err が nil なら、Record と Task に完了の結果が入る。
type BatchCompletionResult struct {
	TaskID uuid.UUID
	Record domain.CareRecord
	Task   domain.CareTask
	Err    error
}

// TaskLocator は、タスク・植物の ID から、権限を確かめる世帯と、今日を決めるタイムゾーンを探す（実装は CareTaskRepository）。
type TaskLocator interface {
	HouseholdOf(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error)
	TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error)
	TaskPlantOf(ctx context.Context, plantID uuid.UUID) (TaskPlant, error)
}

// CareRecordService は、お手入れの記録（完了・スキップ・取り消し・まとめて完了・編集・削除・履歴）を扱う（要件10・11-2〜11-4）。
type CareRecordService struct {
	records CareRecordRepository
	tasks   TaskLocator
	authz   *Authorizer
	now     func() time.Time
}

// NewCareRecordService は、CareRecordService を作る。now は現在時刻を返す関数で、テストでは固定の時刻を渡す。
func NewCareRecordService(records CareRecordRepository, tasks TaskLocator, authz *Authorizer, now func() time.Time) *CareRecordService {
	return &CareRecordService{records: records, tasks: tasks, authz: authz, now: now}
}

// Complete は、タスクの完了を記録し、次回予定を計算して予定の回を進める。編集者以上ができる（要件10-1〜10-3・10-5）。
// 実施日を省くと、世帯のタイムゾーンでの今日にする。同じ回がすでに完了していれば *domain.AlreadyCompletedError を返す。
func (s *CareRecordService) Complete(ctx context.Context, userID, taskID uuid.UUID, in domain.CompletionInput) (domain.CareRecord, domain.CareTask, error) {
	householdID, today, err := s.authorize(ctx, userID, taskID)
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	c, err := domain.NewCompletion(in, today)
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	return s.records.Complete(ctx, householdID, taskID, userID, c)
}

// Skip は、タスクを「今回はスキップ」として記録し、次回予定を確認日にする。作業をしたことにはせず、予定の回も進めない。
// 編集者以上ができる（要件10-6〜10-8）。確認日を省くと、世帯のタイムゾーンでの翌日にする。
func (s *CareRecordService) Skip(ctx context.Context, userID, taskID uuid.UUID, in domain.SkipInput) (domain.CareRecord, domain.CareTask, error) {
	householdID, today, err := s.authorize(ctx, userID, taskID)
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	sk, err := domain.NewSkip(in, today)
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	return s.records.Skip(ctx, householdID, taskID, userID, sk)
}

// Undo は、記録を取り消し、タスクを記録する前の予定日と予定の回に戻す（要件10-4）。
// 編集者以上で、記録した本人が、記録してから domain.UndoWindow のうちに、そのタスクの最新の記録に限ってできる。
func (s *CareRecordService) Undo(ctx context.Context, userID, recordID uuid.UUID) (domain.CareTask, error) {
	householdID, err := s.authorizeRecord(ctx, userID, recordID)
	if err != nil {
		return domain.CareTask{}, err
	}
	return s.records.Undo(ctx, householdID, recordID, userID, s.now())
}

// CompleteBatch は、世帯の複数のタスクを完了し、送った順にタスクごとの結果を返す（要件10-9）。
// 1件ずつ別のトランザクションで完了するので、重複や入力の誤りがあっても、ほかのタスクの完了は止めない。
// ほかの世帯のタスクは、その項目だけ domain.ErrNotFound になる。
func (s *CareRecordService) CompleteBatch(ctx context.Context, userID, householdID uuid.UUID, items []BatchCompletionItem) ([]BatchCompletionResult, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRecordWrite); err != nil {
		return nil, err
	}
	switch {
	case len(items) == 0:
		return nil, &domain.ValidationError{Fields: map[string]string{"items": "required"}}
	case len(items) > MaxBatchCompletions:
		return nil, &domain.ValidationError{Fields: map[string]string{"items": "too_many"}}
	}
	timeZone, err := s.tasks.TimeZoneOf(ctx, householdID)
	if err != nil {
		return nil, err
	}
	today := domain.TodayIn(s.now(), timeZone)

	results := make([]BatchCompletionResult, len(items))
	for i, item := range items {
		results[i].TaskID = item.TaskID
		c, err := domain.NewCompletion(item.Input, today)
		if err != nil {
			results[i].Err = err
			continue
		}
		results[i].Record, results[i].Task, results[i].Err = s.records.Complete(ctx, householdID, item.TaskID, userID, c)
	}
	return results, nil
}

// Update は、記録の実施日を変える。編集者以上ができ、記録した本人でなくてもよい（要件12-4）。
// 最新の完了であれば、次回予定を計算し直す。未来の実施日は、世帯のタイムゾーンで判定する。
func (s *CareRecordService) Update(ctx context.Context, userID, recordID uuid.UUID, in domain.RecordPatchInput) (domain.CareRecord, domain.CareTask, error) {
	householdID, err := s.authorizeRecord(ctx, userID, recordID)
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	timeZone, err := s.tasks.TimeZoneOf(ctx, householdID)
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	p, err := domain.NewRecordPatch(in, domain.TodayIn(s.now(), timeZone))
	if err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	return s.records.UpdatePerformedOn(ctx, householdID, recordID, p)
}

// Delete は、記録を消す。編集者以上ができる（要件12-4）。最新の完了であれば、次回予定を計算し直す。
func (s *CareRecordService) Delete(ctx context.Context, userID, recordID uuid.UUID) (domain.CareTask, error) {
	householdID, err := s.authorizeRecord(ctx, userID, recordID)
	if err != nil {
		return domain.CareTask{}, err
	}
	return s.records.Delete(ctx, householdID, recordID)
}

// History は、植物の履歴を新しい順に1ページ返し、作業の種類ごとの最終実施日を添える（要件12-1〜12-3）。閲覧者も見られる。
// 最終実施日は、種類の絞り込みにかかわらず、すべての種類について返す。
func (s *CareRecordService) History(ctx context.Context, userID, plantID uuid.UUID, in domain.RecordQueryInput) (domain.RecordPage, error) {
	p, err := s.tasks.TaskPlantOf(ctx, plantID)
	if err != nil {
		return domain.RecordPage{}, err
	}
	if _, err := s.authz.Require(ctx, userID, p.HouseholdID, domain.ActionRead); err != nil {
		return domain.RecordPage{}, err
	}
	q, err := domain.NewRecordQuery(in)
	if err != nil {
		return domain.RecordPage{}, err
	}
	records, more, err := s.records.ListByPlant(ctx, p.HouseholdID, plantID, q)
	if err != nil {
		return domain.RecordPage{}, err
	}
	last, err := s.records.LastPerformed(ctx, p.HouseholdID, plantID)
	if err != nil {
		return domain.RecordPage{}, err
	}
	page := domain.RecordPage{Records: records, LastPerformed: last}
	if more && len(records) > 0 {
		page.NextCursor = domain.CursorOf(records[len(records)-1]).Encode()
	}
	return page, nil
}

// authorizeRecord は、記録の世帯で、記録の権限を確かめる。入力の検証より先に行う。
func (s *CareRecordService) authorizeRecord(ctx context.Context, userID, recordID uuid.UUID) (uuid.UUID, error) {
	householdID, err := s.records.HouseholdOf(ctx, recordID)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRecordWrite); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}

// authorize は、タスクの世帯で記録の権限を確かめ、世帯のタイムゾーンでの今日を返す。入力の検証より先に行う。
func (s *CareRecordService) authorize(ctx context.Context, userID, taskID uuid.UUID) (uuid.UUID, civil.Date, error) {
	householdID, err := s.tasks.HouseholdOf(ctx, taskID)
	if err != nil {
		return uuid.Nil, civil.Date{}, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRecordWrite); err != nil {
		return uuid.Nil, civil.Date{}, err
	}
	timeZone, err := s.tasks.TimeZoneOf(ctx, householdID)
	if err != nil {
		return uuid.Nil, civil.Date{}, err
	}
	return householdID, domain.TodayIn(s.now(), timeZone), nil
}
