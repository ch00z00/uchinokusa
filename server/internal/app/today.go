package app

import (
	"context"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// TodayRepository は、今日やること一覧のデータの読み出し先である（実装は adapter/postgres）。
type TodayRepository interface {
	TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error)
	// Due は、予定日が today 以前の、有効なタスク（アーカイブしていない植物のもの）を、予定日の古い順に返す。
	Due(ctx context.Context, householdID uuid.UUID, today civil.Date) ([]domain.TodayTask, error)
	// Next は、予定日が today より後の、直近の有効なタスクを返す。なければ ok が false になる。
	Next(ctx context.Context, householdID uuid.UUID, today civil.Date) (task domain.TodayTask, ok bool, err error)
	// TaskSuggestions は、タスクへの、today の時点で有効な「やらない方がよい」「条件つき」の提案を返す。
	TaskSuggestions(ctx context.Context, householdID uuid.UUID, taskIDs []uuid.UUID, today civil.Date) ([]domain.Suggestion, error)
}

// TodayService は、今日やること一覧を返す（要件9）。
type TodayService struct {
	repo  TodayRepository
	authz *Authorizer
	now   func() time.Time
}

// NewTodayService は、TodayService を作る。now は現在時刻を返す関数で、テストでは固定の時刻を渡す。
func NewTodayService(repo TodayRepository, authz *Authorizer, now func() time.Time) *TodayService {
	return &TodayService{repo: repo, authz: authz, now: now}
}

// List は、世帯の今日やること一覧を返す。閲覧者も見られる。
// 「今日」は、アプリが送った端末の日付（date）にする。省けば、世帯のタイムゾーンでの今日にする。
// 今日やることがなければ、直近の次の予定を添える（要件9-6）。
func (s *TodayService) List(ctx context.Context, userID, householdID uuid.UUID, date *string) (domain.TodayList, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRead); err != nil {
		return domain.TodayList{}, err
	}
	timeZone, err := s.repo.TimeZoneOf(ctx, householdID)
	if err != nil {
		return domain.TodayList{}, err
	}
	today, err := domain.TodayDate(date, domain.TodayIn(s.now(), timeZone))
	if err != nil {
		return domain.TodayList{}, err
	}

	due, err := s.repo.Due(ctx, householdID, today)
	if err != nil {
		return domain.TodayList{}, err
	}
	list := domain.TodayList{Date: today, Items: make([]domain.TodayItem, len(due))}
	if len(due) == 0 {
		next, ok, err := s.repo.Next(ctx, householdID, today)
		if err != nil {
			return domain.TodayList{}, err
		}
		if ok {
			item := domain.NewTodayItem(next, today)
			list.Next = &item
		}
		return list, nil
	}

	taskIDs := make([]uuid.UUID, len(due))
	for i, t := range due {
		taskIDs[i] = t.Task.ID
	}
	suggestions, err := s.repo.TaskSuggestions(ctx, householdID, taskIDs, today)
	if err != nil {
		return domain.TodayList{}, err
	}
	byTask := map[uuid.UUID][]domain.Suggestion{}
	for _, sg := range suggestions {
		byTask[sg.TaskID] = append(byTask[sg.TaskID], sg)
	}
	for i, t := range due {
		list.Items[i] = domain.NewTodayItem(t, today)
		list.Items[i].Suggestions = byTask[t.Task.ID]
	}
	return list, nil
}
