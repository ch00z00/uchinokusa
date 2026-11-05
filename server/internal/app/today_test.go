package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeToday は、app.TodayRepository の偽物である。due を今日やることに、next を直近の次の予定として返す。
type fakeToday struct {
	due         []domain.TodayTask
	next        *domain.TodayTask
	suggestions []domain.Suggestion
	dates       []civil.Date
	taskIDs     []uuid.UUID
}

func (f *fakeToday) TimeZoneOf(context.Context, uuid.UUID) (string, error) { return "Asia/Tokyo", nil }

func (f *fakeToday) Due(_ context.Context, _ uuid.UUID, today civil.Date) ([]domain.TodayTask, error) {
	f.dates = append(f.dates, today)
	return f.due, nil
}

func (f *fakeToday) Next(context.Context, uuid.UUID, civil.Date) (domain.TodayTask, bool, error) {
	if f.next == nil {
		return domain.TodayTask{}, false, nil
	}
	return *f.next, true, nil
}

func (f *fakeToday) TaskSuggestions(_ context.Context, _ uuid.UUID, taskIDs []uuid.UUID, _ civil.Date) ([]domain.Suggestion, error) {
	f.taskIDs = taskIDs
	return f.suggestions, nil
}

func newTodayService(f householdFixture, repo *fakeToday) *app.TodayService {
	return app.NewTodayService(repo, f.authz, func() time.Time { return taskNow })
}

func TestTodayService_List(t *testing.T) {
	t.Parallel()
	watering, fertilizing := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	due := []domain.TodayTask{
		{Task: domain.CareTask{ID: watering, NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 1}}},
		{Task: domain.CareTask{ID: fertilizing, NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 5}}},
	}

	t.Run("閲覧者も見られ、送った日付で、期限切れの日数と提案を添える", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		avoid := domain.Suggestion{ID: uuid.Must(uuid.NewV7()), TaskID: fertilizing, Kind: domain.SuggestionAvoid}
		repo := &fakeToday{due: due, suggestions: []domain.Suggestion{avoid}}
		got, err := newTodayService(f, repo).List(t.Context(), f.viewer, f.household, ptr("2026-11-05"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Date != (civil.Date{Year: 2026, Month: 11, Day: 5}) || len(got.Items) != 2 || got.Next != nil {
			t.Fatalf("got %+v", got)
		}
		if got.Items[0].OverdueDays != 4 || len(got.Items[0].Suggestions) != 0 {
			t.Errorf("items[0] = %+v", got.Items[0])
		}
		if got.Items[1].OverdueDays != 0 || len(got.Items[1].Suggestions) != 1 || got.Items[1].Suggestions[0].ID != avoid.ID {
			t.Errorf("items[1] = %+v", got.Items[1])
		}
		if len(repo.taskIDs) != 2 || repo.taskIDs[0] != watering || repo.taskIDs[1] != fertilizing {
			t.Errorf("taskIDs = %v", repo.taskIDs)
		}
	})

	t.Run("日付を省くと、世帯のタイムゾーンでの今日にする", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		repo := &fakeToday{due: due}
		if _, err := newTodayService(f, repo).List(t.Context(), f.viewer, f.household, nil); err != nil {
			t.Fatal(err)
		}
		if repo.dates[0] != (civil.Date{Year: 2026, Month: 11, Day: 4}) {
			t.Errorf("date = %v, want 2026-11-04 (Asia/Tokyo)", repo.dates[0])
		}
	})

	t.Run("今日やることがなければ、直近の次の予定を返す", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		next := domain.TodayTask{Task: domain.CareTask{ID: watering, NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 8}}}
		repo := &fakeToday{next: &next}
		got, err := newTodayService(f, repo).List(t.Context(), f.viewer, f.household, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Items) != 0 || got.Items == nil || got.Next == nil || got.Next.Task.ID != watering || got.Next.OverdueDays != 0 {
			t.Errorf("got %+v", got)
		}
		// 提案は今日やることにだけ添える
		if repo.taskIDs != nil {
			t.Errorf("taskIDs = %v, want not queried", repo.taskIDs)
		}
	})

	t.Run("日付の形式の誤りは 422", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		_, err := newTodayService(f, &fakeToday{}).List(t.Context(), f.viewer, f.household, ptr("11/5"))
		assertFields(t, err, map[string]string{"date": "invalid"})
	})

	t.Run("ほかの世帯は 404", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		if _, err := newTodayService(f, &fakeToday{}).List(t.Context(), f.stranger, f.household, nil); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
