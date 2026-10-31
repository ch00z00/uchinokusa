package domain_test

import (
	"testing"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func date(t *testing.T, s string) civil.Date {
	t.Helper()
	d, err := civil.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestInterval_AddTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		from     string
		interval domain.Interval
		want     string
	}{
		{name: "日", from: "2026-10-31", interval: domain.Interval{Value: 3, Unit: domain.IntervalDay}, want: "2026-11-03"},
		{name: "日：年をまたぐ", from: "2026-12-30", interval: domain.Interval{Value: 5, Unit: domain.IntervalDay}, want: "2027-01-04"},
		{name: "日：平年の 2/28 の翌日は 3/1", from: "2026-02-28", interval: domain.Interval{Value: 1, Unit: domain.IntervalDay}, want: "2026-03-01"},
		{name: "日：うるう年の 2/28 の翌日は 2/29", from: "2028-02-28", interval: domain.Interval{Value: 1, Unit: domain.IntervalDay}, want: "2028-02-29"},
		{name: "週", from: "2026-10-31", interval: domain.Interval{Value: 2, Unit: domain.IntervalWeek}, want: "2026-11-14"},
		{name: "週：うるう日をまたぐ", from: "2028-02-25", interval: domain.Interval{Value: 1, Unit: domain.IntervalWeek}, want: "2028-03-03"},
		{name: "月", from: "2026-01-15", interval: domain.Interval{Value: 1, Unit: domain.IntervalMonth}, want: "2026-02-15"},
		{name: "月：年をまたぐ", from: "2026-12-31", interval: domain.Interval{Value: 1, Unit: domain.IntervalMonth}, want: "2027-01-31"},
		// time.AddDate（civil.Date.AddMonths）では 3/3 に繰り上がるが、月末に丸める
		{name: "月：1/31 の1か月後は 2/28", from: "2026-01-31", interval: domain.Interval{Value: 1, Unit: domain.IntervalMonth}, want: "2026-02-28"},
		{name: "月：うるう年の 1/31 の1か月後は 2/29", from: "2028-01-31", interval: domain.Interval{Value: 1, Unit: domain.IntervalMonth}, want: "2028-02-29"},
		{name: "月：3/31 の1か月後は 4/30", from: "2026-03-31", interval: domain.Interval{Value: 1, Unit: domain.IntervalMonth}, want: "2026-04-30"},
		{name: "月：年をまたいで月末に丸める", from: "2026-10-31", interval: domain.Interval{Value: 4, Unit: domain.IntervalMonth}, want: "2027-02-28"},
		{name: "月：うるう日の12か月後は 2/28", from: "2028-02-29", interval: domain.Interval{Value: 12, Unit: domain.IntervalMonth}, want: "2029-02-28"},
		{name: "月：月末でない日は丸めない", from: "2026-01-28", interval: domain.Interval{Value: 1, Unit: domain.IntervalMonth}, want: "2026-02-28"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.interval.AddTo(date(t, tt.from)); got != date(t, tt.want) {
				t.Errorf("%v + %+v = %v, want %v", tt.from, tt.interval, got, tt.want)
			}
		})
	}
}

func TestSeasonOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		date       string
		hemisphere domain.Hemisphere
		want       domain.Season
	}{
		// 北半球：春 3〜5月、夏 6〜8月、秋 9〜11月、冬 12〜2月
		{date: "2026-01-01", hemisphere: domain.HemisphereNorth, want: domain.SeasonWinter},
		{date: "2026-02-28", hemisphere: domain.HemisphereNorth, want: domain.SeasonWinter},
		{date: "2028-02-29", hemisphere: domain.HemisphereNorth, want: domain.SeasonWinter},
		{date: "2026-03-01", hemisphere: domain.HemisphereNorth, want: domain.SeasonSpring},
		{date: "2026-05-31", hemisphere: domain.HemisphereNorth, want: domain.SeasonSpring},
		{date: "2026-06-01", hemisphere: domain.HemisphereNorth, want: domain.SeasonSummer},
		{date: "2026-08-31", hemisphere: domain.HemisphereNorth, want: domain.SeasonSummer},
		{date: "2026-09-01", hemisphere: domain.HemisphereNorth, want: domain.SeasonAutumn},
		{date: "2026-11-30", hemisphere: domain.HemisphereNorth, want: domain.SeasonAutumn},
		{date: "2026-12-01", hemisphere: domain.HemisphereNorth, want: domain.SeasonWinter},
		{date: "2026-12-31", hemisphere: domain.HemisphereNorth, want: domain.SeasonWinter},
		// 南半球：季節が6か月ずれる
		{date: "2026-02-28", hemisphere: domain.HemisphereSouth, want: domain.SeasonSummer},
		{date: "2026-03-01", hemisphere: domain.HemisphereSouth, want: domain.SeasonAutumn},
		{date: "2026-06-01", hemisphere: domain.HemisphereSouth, want: domain.SeasonWinter},
		{date: "2026-09-01", hemisphere: domain.HemisphereSouth, want: domain.SeasonSpring},
		{date: "2026-12-01", hemisphere: domain.HemisphereSouth, want: domain.SeasonSummer},
	}
	for _, tt := range tests {
		t.Run(tt.date+"/"+string(tt.hemisphere), func(t *testing.T) {
			t.Parallel()
			if got := domain.SeasonOf(date(t, tt.date), tt.hemisphere); got != tt.want {
				t.Errorf("SeasonOf(%s, %s) = %s, want %s", tt.date, tt.hemisphere, got, tt.want)
			}
		})
	}
}

var (
	everyWeek     = domain.Interval{Value: 1, Unit: domain.IntervalWeek}
	everyTwoWeeks = domain.Interval{Value: 2, Unit: domain.IntervalWeek}
	everyThreeDay = domain.Interval{Value: 3, Unit: domain.IntervalDay}
)

func TestSchedule_EffectiveInterval(t *testing.T) {
	t.Parallel()

	// 水やり：基本は週1回、冬は2週に1回、夏は3日に1回
	watering := domain.Schedule{
		Base:     everyWeek,
		Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: everyTwoWeeks, domain.SeasonSummer: everyThreeDay},
	}

	tests := []struct {
		name       string
		schedule   domain.Schedule
		on         string
		hemisphere domain.Hemisphere
		want       domain.Interval
	}{
		{name: "季節ごとの周期がなければ、基本の周期", schedule: domain.Schedule{Base: everyWeek}, on: "2026-01-15", hemisphere: domain.HemisphereNorth, want: everyWeek},
		{name: "冬は冬の周期", schedule: watering, on: "2026-01-15", hemisphere: domain.HemisphereNorth, want: everyTwoWeeks},
		{name: "夏は夏の周期", schedule: watering, on: "2026-07-15", hemisphere: domain.HemisphereNorth, want: everyThreeDay},
		{name: "季節の周期がない季節（春）は、基本の周期", schedule: watering, on: "2026-04-15", hemisphere: domain.HemisphereNorth, want: everyWeek},
		{name: "南半球の7月は冬", schedule: watering, on: "2026-07-15", hemisphere: domain.HemisphereSouth, want: everyTwoWeeks},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.schedule.EffectiveInterval(date(t, tt.on), tt.hemisphere); got != tt.want {
				t.Errorf("EffectiveInterval(%s) = %+v, want %+v", tt.on, got, tt.want)
			}
		})
	}
}

func TestSchedule_NextDueAfterCompletion(t *testing.T) {
	t.Parallel()

	watering := domain.Schedule{
		Base:     everyWeek,
		Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: everyTwoWeeks},
	}

	tests := []struct {
		name        string
		performedOn string
		want        string
	}{
		{name: "実施日の季節の周期で決める（冬）", performedOn: "2026-01-10", want: "2026-01-24"},
		{name: "実施日の季節の周期で決める（春）", performedOn: "2026-04-10", want: "2026-04-17"},
		// 次回予定が春に入っても、実施日（2/28）の季節の周期を使う
		{name: "冬の最終日に実施したら、冬の周期", performedOn: "2026-02-28", want: "2026-03-14"},
		{name: "春の初日に実施したら、春（基本）の周期", performedOn: "2026-03-01", want: "2026-03-08"},
		{name: "うるう日に実施（冬）", performedOn: "2028-02-29", want: "2028-03-14"},
		{name: "秋の最終日に実施したら、秋（基本）の周期", performedOn: "2026-11-30", want: "2026-12-07"},
		{name: "冬の初日に実施したら、冬の周期", performedOn: "2026-12-01", want: "2026-12-15"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := watering.NextDueAfterCompletion(date(t, tt.performedOn), domain.HemisphereNorth)
			if got != date(t, tt.want) {
				t.Errorf("NextDueAfterCompletion(%s) = %v, want %v", tt.performedOn, got, tt.want)
			}
		})
	}
}

// 事後条件：次回予定は、どの日に実施しても、実施日より後になる。
func TestSchedule_NextDueAfterCompletion_AlwaysAfterPerformedOn(t *testing.T) {
	t.Parallel()

	schedules := []domain.Schedule{
		{Base: domain.Interval{Value: 1, Unit: domain.IntervalDay}},
		{Base: domain.Interval{Value: 1, Unit: domain.IntervalMonth}},
		{Base: everyWeek, Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: {Value: 1, Unit: domain.IntervalMonth}}},
		// 不正な周期（0 以下）でも、実施日の翌日以降にする（入力の検証はタスクの作成時に行う）
		{Base: domain.Interval{Value: 0, Unit: domain.IntervalDay}},
		{Base: domain.Interval{Value: -1, Unit: domain.IntervalMonth}},
		{Base: domain.Interval{Value: 1, Unit: "year"}},
	}
	start, end := date(t, "2027-01-01"), date(t, "2029-01-01") // うるう年（2028年）を含む
	for _, s := range schedules {
		for _, h := range []domain.Hemisphere{domain.HemisphereNorth, domain.HemisphereSouth} {
			for d := start; d.Before(end); d = d.AddDays(1) {
				if got := s.NextDueAfterCompletion(d, h); !got.After(d) {
					t.Fatalf("%+v, %s: NextDueAfterCompletion(%s) = %s, want after %s", s, h, d, got, d)
				}
			}
		}
	}
}
