package domain_test

import (
	"testing"
	"time"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestIsSeasonStart(t *testing.T) {
	t.Parallel()
	d := func(y int, m, day int) civil.Date { return civil.Date{Year: y, Month: time.Month(m), Day: day} }
	tests := []struct {
		date civil.Date
		want bool
	}{
		{d(2026, 3, 1), true},
		{d(2026, 6, 1), true},
		{d(2026, 9, 1), true},
		{d(2026, 12, 1), true},
		{d(2026, 12, 2), false},
		{d(2026, 1, 1), false},
		{d(2026, 11, 30), false},
	}
	for _, tt := range tests {
		if got := domain.IsSeasonStart(tt.date); got != tt.want {
			t.Errorf("IsSeasonStart(%v) = %v, want %v", tt.date, got, tt.want)
		}
	}
	// 南半球でも、季節の初日の日付は同じ（季節が6か月ずれるだけ）
	for _, m := range []int{3, 6, 9, 12} {
		start := d(2026, m, 1)
		before := start.AddDays(-1)
		for _, h := range []domain.Hemisphere{domain.HemisphereNorth, domain.HemisphereSouth} {
			if domain.SeasonOf(before, h) == domain.SeasonOf(start, h) {
				t.Errorf("%s: season does not change on %v", h, start)
			}
		}
	}
}

func TestSchedule_NextDueAtSeasonStart(t *testing.T) {
	t.Parallel()
	// 基本は週1回、冬は2週に1回、夏は3日に1回
	s := domain.Schedule{
		Base: domain.Interval{Value: 1, Unit: domain.IntervalWeek},
		Seasonal: domain.SeasonalIntervals{
			domain.SeasonWinter: {Value: 2, Unit: domain.IntervalWeek},
			domain.SeasonSummer: {Value: 3, Unit: domain.IntervalDay},
		},
	}
	d := func(y int, m, day int) civil.Date { return civil.Date{Year: y, Month: time.Month(m), Day: day} }
	tests := []struct {
		name        string
		last, start civil.Date
		h           domain.Hemisphere
		want        civil.Date
	}{
		{"最後の完了に、新しい季節の周期を足す", d(2026, 11, 25), d(2026, 12, 1), domain.HemisphereNorth, d(2026, 12, 9)},
		{"季節の周期がなければ、基本の周期", d(2026, 2, 26), d(2026, 3, 1), domain.HemisphereNorth, d(2026, 3, 5)},
		{"季節の初日より前なら、季節の初日にする", d(2026, 5, 20), d(2026, 6, 1), domain.HemisphereNorth, d(2026, 6, 1)},
		{"南半球は、6月が冬", d(2026, 5, 25), d(2026, 6, 1), domain.HemisphereSouth, d(2026, 6, 8)},
		// 初日に完了していれば、完了したときの計算（NextDueAfterCompletion）と同じになる
		{"初日の完了", d(2026, 12, 1), d(2026, 12, 1), domain.HemisphereNorth, d(2026, 12, 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := s.NextDueAtSeasonStart(tt.last, tt.start, tt.h); got != tt.want {
				t.Errorf("NextDueAtSeasonStart(%v, %v) = %v, want %v", tt.last, tt.start, got, tt.want)
			}
		})
	}
	if got, want := s.NextDueAtSeasonStart(d(2026, 12, 1), d(2026, 12, 1), domain.HemisphereNorth), s.NextDueAfterCompletion(d(2026, 12, 1), domain.HemisphereNorth); got != want {
		t.Errorf("same day: %v, after completion: %v", got, want)
	}
}
