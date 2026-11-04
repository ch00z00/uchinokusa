package app

import (
	"context"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// TaskPlant は、タスクを作る植物について、タスクの作成に必要な情報である。
type TaskPlant struct {
	HouseholdID uuid.UUID
	TimeZone    string              // 世帯のタイムゾーン。初回の予定日の既定値（今日）を決める
	Care        *domain.SpeciesCare // 植物マスタのお手入れの目安。マスタと紐づかなければ nil
}

// CareTaskRepository は、お手入れタスクの保存先である（実装は adapter/postgres）。
type CareTaskRepository interface {
	// TaskPlantOf は、植物の世帯・タイムゾーン・植物マスタの目安を返す。植物がなければ domain.ErrNotFound を返す。
	TaskPlantOf(ctx context.Context, plantID uuid.UUID) (TaskPlant, error)
	// List は、植物の削除していないタスク（一時停止中を含む）を返す。
	List(ctx context.Context, householdID, plantID uuid.UUID) ([]domain.CareTask, error)
	Create(ctx context.Context, householdID, plantID uuid.UUID, d domain.CareTaskDraft) (domain.CareTask, error)
	// HouseholdOf は、タスクが属する世帯の ID を返す。タスクがない・削除済みなら domain.ErrNotFound を返す。
	HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// Get は、世帯の削除していないタスクを返す。なければ domain.ErrNotFound を返す。
	Get(ctx context.Context, householdID, id uuid.UUID) (domain.CareTask, error)
	// Update は、世帯のタスクを変更する。予定日を変えた場合は、手で変えた印（due_overridden）を立てる。
	Update(ctx context.Context, householdID, id uuid.UUID, p domain.CareTaskPatch) (domain.CareTask, error)
	// Delete は、世帯のタスクを削除する（論理削除。記録は履歴として残す）。
	Delete(ctx context.Context, householdID, id uuid.UUID) error
	// SetStatus は、世帯のタスクの状態を変える。nextDueOn が nil なら、次回予定は変えない。
	SetStatus(ctx context.Context, householdID, id uuid.UUID, status domain.TaskStatus, nextDueOn *civil.Date) (domain.CareTask, error)
	// TimeZoneOf は、世帯のタイムゾーンを返す。タスクの ID だけで呼ばれる操作で、世帯の今日を決めるために使う。
	TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error)
}

// CareTaskService は、お手入れタスクの作成・一覧・変更・削除・一時停止・再開を扱う（要件8）。
type CareTaskService struct {
	tasks CareTaskRepository
	authz *Authorizer
	now   func() time.Time
}

// NewCareTaskService は、CareTaskService を作る。now は現在時刻を返す関数で、テストでは固定の時刻を渡す。
func NewCareTaskService(tasks CareTaskRepository, authz *Authorizer, now func() time.Time) *CareTaskService {
	return &CareTaskService{tasks: tasks, authz: authz, now: now}
}

// List は、植物のタスクを返す。閲覧者も見られる。
func (s *CareTaskService) List(ctx context.Context, userID, plantID uuid.UUID) ([]domain.CareTask, error) {
	p, err := s.plantFor(ctx, userID, plantID, domain.ActionRead)
	if err != nil {
		return nil, err
	}
	return s.tasks.List(ctx, p.HouseholdID, plantID)
}

// Defaults は、タスクの作成画面の初期値を、植物マスタのお手入れの目安から返す（要件6-4）。閲覧者も見られる。
func (s *CareTaskService) Defaults(ctx context.Context, userID, plantID uuid.UUID) ([]domain.TaskDefault, error) {
	p, err := s.plantFor(ctx, userID, plantID, domain.ActionRead)
	if err != nil {
		return nil, err
	}
	return domain.TaskDefaultsOf(p.Care), nil
}

// Create は、植物にタスクを作る。編集者以上ができる（要件8-1〜8-6）。
// 初回の予定日を省くと、世帯のタイムゾーンでの今日にする。
func (s *CareTaskService) Create(ctx context.Context, userID, plantID uuid.UUID, in domain.CareTaskInput) (domain.CareTask, error) {
	p, err := s.plantFor(ctx, userID, plantID, domain.ActionTaskWrite)
	if err != nil {
		return domain.CareTask{}, err
	}
	d, err := domain.NewCareTask(in, domain.TodayIn(s.now(), p.TimeZone))
	if err != nil {
		return domain.CareTask{}, err
	}
	return s.tasks.Create(ctx, p.HouseholdID, plantID, d)
}

// Update は、タスクの周期・実行条件・予定日などを変更する。編集者以上ができる（要件8-8）。
// 周期を変えても次回予定日は変えない（新しい周期は次の完了から使う）。
func (s *CareTaskService) Update(ctx context.Context, userID, taskID uuid.UUID, in domain.CareTaskInput) (domain.CareTask, error) {
	householdID, err := s.authorize(ctx, userID, taskID)
	if err != nil {
		return domain.CareTask{}, err
	}
	current, err := s.tasks.Get(ctx, householdID, taskID)
	if err != nil {
		return domain.CareTask{}, err
	}
	p, err := domain.NewCareTaskPatch(in, current.Kind)
	if err != nil {
		return domain.CareTask{}, err
	}
	return s.tasks.Update(ctx, householdID, taskID, p)
}

// Delete は、タスクを削除する。過去の記録は履歴として残す。編集者以上ができる（要件8-10）。
func (s *CareTaskService) Delete(ctx context.Context, userID, taskID uuid.UUID) error {
	householdID, err := s.authorize(ctx, userID, taskID)
	if err != nil {
		return err
	}
	return s.tasks.Delete(ctx, householdID, taskID)
}

// Pause は、タスクを一時停止し、予定・今日やること・リマインダーの対象から外す。編集者以上ができる（要件8-9）。
// 次回予定はそのまま残す。すでに一時停止中でも、エラーにせず今の状態を返す（家族の2人が同時に押す場合があるため）。
func (s *CareTaskService) Pause(ctx context.Context, userID, taskID uuid.UUID) (domain.CareTask, error) {
	householdID, err := s.authorize(ctx, userID, taskID)
	if err != nil {
		return domain.CareTask{}, err
	}
	return s.tasks.SetStatus(ctx, householdID, taskID, domain.TaskPaused, nil)
}

// Resume は、一時停止したタスクを再開する。編集者以上ができる（要件8-9）。
// 次回予定は、世帯のタイムゾーンでの今日以降にする（domain.ResumedNextDueOn）。
// 一時停止していないタスクは、そのまま返す。期限切れの予定日を今日に動かすと、期限切れの表示が消えてしまうため。
func (s *CareTaskService) Resume(ctx context.Context, userID, taskID uuid.UUID) (domain.CareTask, error) {
	householdID, err := s.authorize(ctx, userID, taskID)
	if err != nil {
		return domain.CareTask{}, err
	}
	current, err := s.tasks.Get(ctx, householdID, taskID)
	if err != nil {
		return domain.CareTask{}, err
	}
	if current.Status != domain.TaskPaused {
		return current, nil
	}
	timeZone, err := s.tasks.TimeZoneOf(ctx, householdID)
	if err != nil {
		return domain.CareTask{}, err
	}
	next := domain.ResumedNextDueOn(current.NextDueOn, domain.TodayIn(s.now(), timeZone))
	return s.tasks.SetStatus(ctx, householdID, taskID, domain.TaskActive, &next)
}

// plantFor は、植物の世帯を探し、ユーザーがその世帯で action を行えることを確かめる。
// 植物がない場合と、所属していない世帯の植物の場合は、どちらも domain.ErrNotFound にする（存在を明かさない）。
func (s *CareTaskService) plantFor(ctx context.Context, userID, plantID uuid.UUID, action domain.Action) (TaskPlant, error) {
	p, err := s.tasks.TaskPlantOf(ctx, plantID)
	if err != nil {
		return TaskPlant{}, err
	}
	if _, err := s.authz.Require(ctx, userID, p.HouseholdID, action); err != nil {
		return TaskPlant{}, err
	}
	return p, nil
}

// authorize は、タスクの ID から世帯を探し、ユーザーがその世帯でタスクを変更できることを確かめる。
func (s *CareTaskService) authorize(ctx context.Context, userID, taskID uuid.UUID) (uuid.UUID, error) {
	householdID, err := s.tasks.HouseholdOf(ctx, taskID)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionTaskWrite); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}
