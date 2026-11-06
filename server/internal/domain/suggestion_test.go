package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// repottingSuggestion は、植え替えのタスクを作る提案（5/1 の提案日、5/31 まで有効）である。
func repottingSuggestion() domain.Suggestion {
	return domain.Suggestion{
		Kind:         domain.SuggestionDo,
		Action:       domain.SuggestionCreateTask,
		TaskKind:     domain.CareRepotting,
		TaskSchedule: domain.Schedule{Base: domain.Interval{Value: 12, Unit: domain.IntervalMonth}},
		ProposedDate: day(time.May, 1),
		ValidFrom:    day(time.April, 20),
		ValidUntil:   day(time.May, 31),
		Status:       domain.SuggestionPending,
	}
}

func TestSuggestion_ActiveOn(t *testing.T) {
	t.Parallel()
	s := repottingSuggestion()
	for _, tt := range []struct {
		name string
		s    func() domain.Suggestion
		on   civil.Date
		want bool
	}{
		{"有効期間の最初の日", repottingSuggestion, day(time.April, 20), true},
		{"有効期間の最後の日", repottingSuggestion, day(time.May, 31), true},
		{"有効期間の前", repottingSuggestion, day(time.April, 19), false},
		{"有効期間のあと", repottingSuggestion, day(time.June, 1), false},
		{"期限のない提案", func() domain.Suggestion { s := s; s.ValidUntil = civil.Date{}; return s }, day(time.December, 31), true},
		{"見送った提案", func() domain.Suggestion { s := s; s.Status = domain.SuggestionDismissed; return s }, day(time.May, 1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.s().ActiveOn(tt.on); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanAcceptance(t *testing.T) {
	t.Parallel()
	today := day(time.April, 25)

	t.Run("タスクを作る提案は、提案日を予定日にしたタスクを作る", func(t *testing.T) {
		t.Parallel()
		got, err := domain.PlanAcceptance(repottingSuggestion(), domain.AcceptInput{}, today)
		if err != nil {
			t.Fatal(err)
		}
		want := &domain.CareTaskDraft{
			CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareRepotting, Schedule: domain.Schedule{Base: domain.Interval{Value: 12, Unit: domain.IntervalMonth}}},
			NextDueOn:       day(time.May, 1),
		}
		if !reflect.DeepEqual(got.Task, want) {
			t.Errorf("task = %+v, want %+v", got.Task, want)
		}
	})

	t.Run("日付を変えて採用すると、変えた日を予定日にする", func(t *testing.T) {
		t.Parallel()
		got, err := domain.PlanAcceptance(repottingSuggestion(), domain.AcceptInput{Date: ptr("2026-06-10")}, today)
		if err != nil || got.Task == nil || got.Task.NextDueOn != day(time.June, 10) {
			t.Errorf("got %+v, %v", got, err)
		}
	})

	t.Run("タスクを作らない提案は、対応を記録するだけ", func(t *testing.T) {
		t.Parallel()
		s := repottingSuggestion()
		s.Kind, s.Action = domain.SuggestionAvoid, domain.SuggestionInfo
		got, err := domain.PlanAcceptance(s, domain.AcceptInput{}, today)
		if err != nil || got.Task != nil {
			t.Errorf("got %+v, %v", got, err)
		}
	})

	for _, tt := range []struct {
		name string
		s    func() domain.Suggestion
		in   domain.AcceptInput
		want map[string]string
	}{
		{"読めない日付", repottingSuggestion, domain.AcceptInput{Date: ptr("6/10")}, map[string]string{"date": "invalid"}},
		{"タスクを作らない提案に、日付は送れない", func() domain.Suggestion {
			s := repottingSuggestion()
			s.Action = domain.SuggestionInfo
			return s
		}, domain.AcceptInput{Date: ptr("2026-06-10")}, map[string]string{"date": "invalid"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.PlanAcceptance(tt.s(), tt.in, today)
			var v *domain.ValidationError
			if !errors.As(err, &v) || !reflect.DeepEqual(v.Fields, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		name   string
		status domain.SuggestionStatus
		today  civil.Date
		code   string
	}{
		{"採用した提案は、もう一度は答えられない", domain.SuggestionAccepted, today, domain.ConflictAlreadyResponded},
		{"見送った提案は、採用できない", domain.SuggestionDismissed, today, domain.ConflictAlreadyResponded},
		{"有効期間を過ぎた提案は、採用できない", domain.SuggestionPending, day(time.June, 1), domain.ConflictSuggestionExpired},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := repottingSuggestion()
			s.Status = tt.status
			_, err := domain.PlanAcceptance(s, domain.AcceptInput{}, tt.today)
			var c *domain.ConflictError
			if !errors.As(err, &c) || c.Code != tt.code {
				t.Errorf("err = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestCheckDismiss(t *testing.T) {
	t.Parallel()
	if err := domain.CheckDismiss(repottingSuggestion()); err != nil {
		t.Errorf("pending: err = %v", err)
	}
	for _, st := range []domain.SuggestionStatus{domain.SuggestionAccepted, domain.SuggestionDismissed} {
		s := repottingSuggestion()
		s.Status = st
		var c *domain.ConflictError
		if err := domain.CheckDismiss(s); !errors.As(err, &c) || c.Code != domain.ConflictAlreadyResponded {
			t.Errorf("%s: err = %v", st, err)
		}
	}
}
