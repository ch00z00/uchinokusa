package domain_test

import (
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

var taskToday = civil.Date{Year: 2026, Month: 11, Day: 4}

func TestNewCareTask(t *testing.T) {
	t.Parallel()

	t.Run("初回の予定日を省くと、今日にする", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCareTask(domain.CareTaskInput{
			Kind:          ptr("watering"),
			Interval:      &domain.IntervalInput{Value: 7, Unit: "day"},
			ConditionText: ptr(" 土の表面が乾いていたら "),
			Note:          ptr("底から流れるまでたっぷり"),
		}, taskToday)
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != domain.CareWatering || got.CustomName != "" || got.Schedule.Base != (domain.Interval{Value: 7, Unit: domain.IntervalDay}) ||
			len(got.Schedule.Seasonal) != 0 || got.ConditionText != "土の表面が乾いていたら" || got.Note != "底から流れるまでたっぷり" || got.NextDueOn != taskToday {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("初回の予定日・季節ごとの周期を入れる", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCareTask(domain.CareTaskInput{
			Kind:     ptr("watering"),
			Interval: &domain.IntervalInput{Value: 1, Unit: "week"},
			SeasonalIntervals: map[string]domain.IntervalInput{
				"winter": {Value: 2, Unit: "week"},
				"summer": {Value: 4, Unit: "day"},
			},
			NextDueOn: ptr("2026-11-06"),
		}, taskToday)
		if err != nil {
			t.Fatal(err)
		}
		want := domain.SeasonalIntervals{
			domain.SeasonWinter: {Value: 2, Unit: domain.IntervalWeek},
			domain.SeasonSummer: {Value: 4, Unit: domain.IntervalDay},
		}
		if len(got.Schedule.Seasonal) != 2 || got.Schedule.Seasonal[domain.SeasonWinter] != want[domain.SeasonWinter] || got.Schedule.Seasonal[domain.SeasonSummer] != want[domain.SeasonSummer] {
			t.Errorf("seasonal = %+v, want %+v", got.Schedule.Seasonal, want)
		}
		if got.NextDueOn != (civil.Date{Year: 2026, Month: 11, Day: 6}) {
			t.Errorf("next due = %v", got.NextDueOn)
		}
	})

	t.Run("その他は作業名が必須で、ほかの種類では作業名を使わない", func(t *testing.T) {
		t.Parallel()
		other, err := domain.NewCareTask(domain.CareTaskInput{Kind: ptr("other"), CustomName: ptr(" 葉水 "), Interval: &domain.IntervalInput{Value: 1, Unit: "day"}}, taskToday)
		if err != nil || other.CustomName != "葉水" {
			t.Errorf("other = %+v, %v", other, err)
		}
		watering, err := domain.NewCareTask(domain.CareTaskInput{Kind: ptr("watering"), CustomName: ptr("葉水"), Interval: &domain.IntervalInput{Value: 1, Unit: "day"}}, taskToday)
		if err != nil || watering.CustomName != "" {
			t.Errorf("watering = %+v, %v", watering, err)
		}
	})

	day := func(v int) *domain.IntervalInput { return &domain.IntervalInput{Value: v, Unit: "day"} }
	tests := []struct {
		name       string
		in         domain.CareTaskInput
		wantFields map[string]string
	}{
		{name: "種類と周期がない", in: domain.CareTaskInput{}, wantFields: map[string]string{"kind": "required", "interval": "required"}},
		{name: "知らない種類", in: domain.CareTaskInput{Kind: ptr("misting"), Interval: day(1)}, wantFields: map[string]string{"kind": "invalid"}},
		{name: "その他で作業名がない", in: domain.CareTaskInput{Kind: ptr("other"), CustomName: ptr(" "), Interval: day(1)}, wantFields: map[string]string{"customName": "required"}},
		{
			name:       "作業名が長すぎる",
			in:         domain.CareTaskInput{Kind: ptr("other"), CustomName: ptr(strings.Repeat("葉", domain.MaxCustomNameLength+1)), Interval: day(1)},
			wantFields: map[string]string{"customName": "too_long"},
		},
		{name: "周期が0", in: domain.CareTaskInput{Kind: ptr("watering"), Interval: day(0)}, wantFields: map[string]string{"interval": "invalid"}},
		{name: "周期の単位が違う", in: domain.CareTaskInput{Kind: ptr("watering"), Interval: &domain.IntervalInput{Value: 1, Unit: "year"}}, wantFields: map[string]string{"interval": "invalid"}},
		{name: "日の周期が長すぎる", in: domain.CareTaskInput{Kind: ptr("watering"), Interval: day(366)}, wantFields: map[string]string{"interval": "invalid"}},
		{name: "週の周期が長すぎる", in: domain.CareTaskInput{Kind: ptr("watering"), Interval: &domain.IntervalInput{Value: 53, Unit: "week"}}, wantFields: map[string]string{"interval": "invalid"}},
		{name: "月の周期が長すぎる", in: domain.CareTaskInput{Kind: ptr("repotting"), Interval: &domain.IntervalInput{Value: 25, Unit: "month"}}, wantFields: map[string]string{"interval": "invalid"}},
		{
			name:       "季節ごとの周期の誤りは、季節ごとに返す",
			in:         domain.CareTaskInput{Kind: ptr("watering"), Interval: day(7), SeasonalIntervals: map[string]domain.IntervalInput{"winter": {Value: 0, Unit: "week"}}},
			wantFields: map[string]string{"seasonalIntervals.winter": "invalid"},
		},
		{
			name:       "知らない季節",
			in:         domain.CareTaskInput{Kind: ptr("watering"), Interval: day(7), SeasonalIntervals: map[string]domain.IntervalInput{"rainy": {Value: 3, Unit: "day"}}},
			wantFields: map[string]string{"seasonalIntervals.rainy": "invalid"},
		},
		{
			name:       "実行条件が長すぎる",
			in:         domain.CareTaskInput{Kind: ptr("watering"), Interval: day(7), ConditionText: ptr(strings.Repeat("土", domain.MaxConditionTextLength+1))},
			wantFields: map[string]string{"conditionText": "too_long"},
		},
		{
			name:       "メモが長すぎる",
			in:         domain.CareTaskInput{Kind: ptr("watering"), Interval: day(7), Note: ptr(strings.Repeat("水", domain.MaxTaskNoteLength+1))},
			wantFields: map[string]string{"note": "too_long"},
		},
		{name: "初回の予定日の形式が違う", in: domain.CareTaskInput{Kind: ptr("watering"), Interval: day(7), NextDueOn: ptr("11/6")}, wantFields: map[string]string{"nextDueOn": "invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewCareTask(tt.in, taskToday)
			assertFields(t, err, tt.wantFields)
		})
	}

	t.Run("周期の上限ちょうどは通る", func(t *testing.T) {
		t.Parallel()
		for _, i := range []domain.IntervalInput{{Value: 365, Unit: "day"}, {Value: 52, Unit: "week"}, {Value: 24, Unit: "month"}} {
			if _, err := domain.NewCareTask(domain.CareTaskInput{Kind: ptr("repotting"), Interval: &i}, taskToday); err != nil {
				t.Errorf("%+v: err = %v", i, err)
			}
		}
	})
}

func TestNewCareTaskPatch(t *testing.T) {
	t.Parallel()

	t.Run("送らなかった項目は変えない（nil のまま）", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCareTaskPatch(domain.CareTaskInput{}, domain.CareWatering)
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.CareTaskPatch{}) {
			t.Errorf("got %+v, want empty", got)
		}
	})

	t.Run("周期・実行条件・予定日を変え、空文字で実行条件とメモを消す", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCareTaskPatch(domain.CareTaskInput{
			Interval:      &domain.IntervalInput{Value: 10, Unit: "day"},
			ConditionText: ptr(""),
			Note:          ptr(""),
			NextDueOn:     ptr("2026-11-20"),
		}, domain.CareWatering)
		if err != nil {
			t.Fatal(err)
		}
		if got.Interval == nil || *got.Interval != (domain.Interval{Value: 10, Unit: domain.IntervalDay}) ||
			got.ConditionText == nil || *got.ConditionText != "" || got.Note == nil || *got.Note != "" ||
			got.NextDueOn == nil || *got.NextDueOn != (civil.Date{Year: 2026, Month: 11, Day: 20}) || got.Seasonal != nil {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("空の季節ごとの周期は、季節ごとの周期をなくすことを表す", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewCareTaskPatch(domain.CareTaskInput{SeasonalIntervals: map[string]domain.IntervalInput{}}, domain.CareWatering)
		if err != nil {
			t.Fatal(err)
		}
		if got.Seasonal == nil || len(*got.Seasonal) != 0 {
			t.Errorf("seasonal = %+v, want empty (not nil)", got.Seasonal)
		}
	})

	t.Run("作業名は、その他のタスクだけで変えられる", func(t *testing.T) {
		t.Parallel()
		other, err := domain.NewCareTaskPatch(domain.CareTaskInput{CustomName: ptr("葉水")}, domain.CareOther)
		if err != nil || other.CustomName == nil || *other.CustomName != "葉水" {
			t.Errorf("other = %+v, %v", other, err)
		}
		watering, err := domain.NewCareTaskPatch(domain.CareTaskInput{CustomName: ptr("葉水")}, domain.CareWatering)
		if err != nil || watering.CustomName != nil {
			t.Errorf("watering = %+v, %v", watering, err)
		}
	})

	t.Run("誤りは登録と同じ規則で確かめ、予定日は空にできない", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewCareTaskPatch(domain.CareTaskInput{
			CustomName: ptr(""), Interval: &domain.IntervalInput{Value: 0, Unit: "day"}, NextDueOn: ptr(""),
		}, domain.CareOther)
		assertFields(t, err, map[string]string{"customName": "required", "interval": "invalid", "nextDueOn": "required"})
	})
}

func TestTaskDefaultsOf(t *testing.T) {
	t.Parallel()

	t.Run("マスタに水やりの実行条件があれば、水やりの初期値にする", func(t *testing.T) {
		t.Parallel()
		got := domain.TaskDefaultsOf(&domain.SpeciesCare{Watering: &domain.WateringAdvice{Condition: "鉢土の表面が乾いてきたら"}})
		want := []domain.TaskDefault{{Kind: domain.CareWatering, ConditionText: "鉢土の表面が乾いてきたら"}}
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("マスタと紐づかない、または目安がなければ空", func(t *testing.T) {
		t.Parallel()
		if got := domain.TaskDefaultsOf(nil); got == nil || len(got) != 0 {
			t.Errorf("nil care: %+v (want [] not nil)", got)
		}
		if got := domain.TaskDefaultsOf(&domain.SpeciesCare{RepottingSuitableMonths: []int{5, 6}}); len(got) != 0 {
			t.Errorf("no watering: %+v", got)
		}
	})
}

func TestTodayIn(t *testing.T) {
	t.Parallel()
	// UTC の 11/3 20:00 は、東京では 11/4 の朝 5:00
	now := time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)
	if got := domain.TodayIn(now, "Asia/Tokyo"); got != taskToday {
		t.Errorf("Tokyo: %v, want %v", got, taskToday)
	}
	if got := domain.TodayIn(now, "America/Los_Angeles"); got != (civil.Date{Year: 2026, Month: 11, Day: 3}) {
		t.Errorf("Los Angeles: %v", got)
	}
	// 読めないタイムゾーンは UTC として扱う（世帯の作成時に検証しているため、通常は起きない）
	if got := domain.TodayIn(now, "Nowhere/Unknown"); got != (civil.Date{Year: 2026, Month: 11, Day: 3}) {
		t.Errorf("unknown: %v", got)
	}
}
