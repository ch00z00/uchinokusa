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
}

// TaskLocator は、タスクの ID から、権限を確かめる世帯と、今日を決めるタイムゾーンを探す（実装は CareTaskRepository）。
type TaskLocator interface {
	HouseholdOf(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error)
	TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error)
}

// CareRecordService は、お手入れの記録（完了・スキップ）を扱う（要件10・11-2〜11-4）。
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
