package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNewCompletion(t *testing.T) {
	t.Parallel()

	t.Run("実施日を省くと、今日にする", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCompletion(domain.CompletionInput{CycleSeq: ptr(3)}, taskToday)
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.Completion{CycleSeq: 3, PerformedOn: taskToday}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("過去の実施日と、追加の記録の指定を受け付ける", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCompletion(domain.CompletionInput{CycleSeq: ptr(3), PerformedOn: ptr(" 2026-10-30 "), AllowExtra: true}, taskToday)
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.Completion{CycleSeq: 3, PerformedOn: civil.Date{Year: 2026, Month: 10, Day: 30}, AllowExtra: true}) {
			t.Errorf("got %+v", got)
		}
	})

	tests := []struct {
		name string
		in   domain.CompletionInput
		want map[string]string
	}{
		{"予定の回の番号は必須", domain.CompletionInput{}, map[string]string{"cycleSeq": "required"}},
		{"予定の回の番号は1以上", domain.CompletionInput{CycleSeq: ptr(0)}, map[string]string{"cycleSeq": "invalid"}},
		{"実施日の形式の誤り", domain.CompletionInput{CycleSeq: ptr(1), PerformedOn: ptr("11/4")}, map[string]string{"performedOn": "invalid"}},
		{"未来の実施日は受け付けない", domain.CompletionInput{CycleSeq: ptr(1), PerformedOn: ptr("2026-11-05")}, map[string]string{"performedOn": "invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewCompletion(tt.in, taskToday)
			var v *domain.ValidationError
			if !errors.As(err, &v) || !reflect.DeepEqual(v.Fields, tt.want) {
				t.Errorf("err = %v, want fields %v", err, tt.want)
			}
		})
	}
}

func TestPlanCompletion(t *testing.T) {
	t.Parallel()

	// 予定の回の番号が 3 の、週1回（冬は2週に1回）の水やり
	task := domain.CareTask{
		CareTaskProfile: domain.CareTaskProfile{
			Kind: domain.CareWatering,
			Schedule: domain.Schedule{
				Base:     domain.Interval{Value: 1, Unit: domain.IntervalWeek},
				Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: {Value: 2, Unit: domain.IntervalWeek}},
			},
		},
		NextDueOn:     civil.Date{Year: 2026, Month: 11, Day: 6},
		DueOverridden: true,
		CycleSeq:      3,
	}

	tests := []struct {
		name string
		c    domain.Completion
		want domain.CompletionPlan
	}{
		{
			"今の回の完了は、実施日に周期を足して次回予定にし、回の番号を進める",
			domain.Completion{CycleSeq: 3, PerformedOn: taskToday},
			domain.CompletionPlan{RecordCycleSeq: 3, NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 11}, NextCycleSeq: 4},
		},
		{
			"周期は実施日の季節で決める（12月は冬の周期）",
			domain.Completion{CycleSeq: 3, PerformedOn: civil.Date{Year: 2026, Month: 12, Day: 1}},
			domain.CompletionPlan{RecordCycleSeq: 3, NextDueOn: civil.Date{Year: 2026, Month: 12, Day: 15}, NextCycleSeq: 4},
		},
		{
			"今の回なら、追加の記録の指定があっても通常の完了にする",
			domain.Completion{CycleSeq: 3, PerformedOn: taskToday, AllowExtra: true},
			domain.CompletionPlan{RecordCycleSeq: 3, NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 11}, NextCycleSeq: 4},
		},
		{
			"完了済みの回への追加の記録は、タスクの予定を変えない",
			domain.Completion{CycleSeq: 2, PerformedOn: taskToday, AllowExtra: true},
			domain.CompletionPlan{Extra: true, RecordCycleSeq: 2, NextDueOn: task.NextDueOn, NextCycleSeq: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.PlanCompletion(task, tt.c, domain.HemisphereNorth)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}

	conflicts := []struct {
		name string
		c    domain.Completion
		want string
	}{
		{"完了済みの回は already_completed", domain.Completion{CycleSeq: 2, PerformedOn: taskToday}, domain.ConflictAlreadyCompleted},
		{"まだ来ていない回は stale_cycle", domain.Completion{CycleSeq: 4, PerformedOn: taskToday}, domain.ConflictStaleCycle},
		{"まだ来ていない回は、追加の記録の指定があっても stale_cycle", domain.Completion{CycleSeq: 4, PerformedOn: taskToday, AllowExtra: true}, domain.ConflictStaleCycle},
	}
	for _, tt := range conflicts {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.PlanCompletion(task, tt.c, domain.HemisphereNorth)
			var c *domain.ConflictError
			if !errors.As(err, &c) || c.Code != tt.want {
				t.Errorf("err = %v, want conflict %q", err, tt.want)
			}
		})
	}
}

func TestAlreadyCompletedError(t *testing.T) {
	t.Parallel()
	// 詳細を扱わない層（まとめて完了の結果など）でも、衝突のコードとして扱えるようにする
	var err error = &domain.AlreadyCompletedError{}
	var c *domain.ConflictError
	if !errors.As(err, &c) || c.Code != domain.ConflictAlreadyCompleted {
		t.Errorf("errors.As(ConflictError) = %v, %+v", err, c)
	}
}
