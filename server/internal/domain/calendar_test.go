package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func day(m time.Month, d int) civil.Date { return civil.Date{Year: 2026, Month: m, Day: d} }

func TestParseCalendarMonth(t *testing.T) {
	t.Parallel()
	today := day(time.November, 6)

	for _, tt := range []struct {
		name string
		in   *string
		want domain.CalendarMonth
	}{
		{"指定した月", ptr("2026-12"), domain.CalendarMonth{Year: 2026, Month: time.December}},
		{"省くと今月", nil, domain.CalendarMonth{Year: 2026, Month: time.November}},
		{"過去の月はいくらでも戻れる", ptr("2019-01"), domain.CalendarMonth{Year: 2019, Month: time.January}},
		{"2年先の月まで", ptr("2028-11"), domain.CalendarMonth{Year: 2028, Month: time.November}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseCalendarMonth(tt.in, today)
			if err != nil || got != tt.want {
				t.Errorf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}

	for _, in := range []string{"2026-13", "2026/11", "2026-11-01", "abc", "2028-12"} {
		t.Run("受け付けない："+in, func(t *testing.T) {
			t.Parallel()
			_, err := domain.ParseCalendarMonth(&in, today)
			var v *domain.ValidationError
			if !errors.As(err, &v) || !reflect.DeepEqual(v.Fields, map[string]string{"month": "invalid"}) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestCalendarMonth_Range(t *testing.T) {
	t.Parallel()
	m := domain.CalendarMonth{Year: 2028, Month: time.February}
	if m.First() != (civil.Date{Year: 2028, Month: 2, Day: 1}) || m.Last() != (civil.Date{Year: 2028, Month: 2, Day: 29}) || m.String() != "2028-02" {
		t.Errorf("first = %v, last = %v, string = %v", m.First(), m.Last(), m.String())
	}
}

func TestExpandPlans(t *testing.T) {
	t.Parallel()
	weekly := domain.Schedule{Base: domain.Interval{Value: 1, Unit: domain.IntervalWeek}}
	task := func(next civil.Date, s domain.Schedule) domain.CareTask {
		return domain.CareTask{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareWatering, Schedule: s}, NextDueOn: next, Status: domain.TaskActive}
	}
	nov := domain.CalendarMonth{Year: 2026, Month: time.November}
	today := day(time.November, 4)

	for _, tt := range []struct {
		name  string
		task  domain.CareTask
		month domain.CalendarMonth
		want  []domain.CalendarPlan
	}{
		{
			// 今の予定日は実際の予定で、そのあとは「予定日に作業した」として周期で展開した見込み
			"今の予定日から、周期で月の終わりまで展開する", task(today, weekly), nov,
			[]domain.CalendarPlan{{Date: day(time.November, 4)}, {Date: day(time.November, 11), Projected: true}, {Date: day(time.November, 18), Projected: true}, {Date: day(time.November, 25), Projected: true}},
		},
		{
			// 期限切れのタスクは、早くても今日作業するので、見込みは今日から数える
			"期限切れなら、今の予定日を残し、見込みは今日から数える", task(day(time.October, 30), weekly), nov,
			[]domain.CalendarPlan{{Date: day(time.November, 11), Projected: true}, {Date: day(time.November, 18), Projected: true}, {Date: day(time.November, 25), Projected: true}},
		},
		{
			"期限切れの予定日は、その月に表示する", task(day(time.October, 30), weekly), domain.CalendarMonth{Year: 2026, Month: time.October},
			[]domain.CalendarPlan{{Date: day(time.October, 30)}},
		},
		{
			// 11/25（秋）から1週間で 12/2。12/2 からは冬の周期（2週間）になる
			"季節の周期で展開する", task(day(time.November, 25), domain.Schedule{
				Base:     weekly.Base,
				Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: {Value: 2, Unit: domain.IntervalWeek}},
			}), domain.CalendarMonth{Year: 2026, Month: time.December},
			[]domain.CalendarPlan{{Date: day(time.December, 2), Projected: true}, {Date: day(time.December, 16), Projected: true}, {Date: day(time.December, 30), Projected: true}},
		},
		{
			"先の月まで展開する", task(today, domain.Schedule{Base: domain.Interval{Value: 1, Unit: domain.IntervalMonth}}), domain.CalendarMonth{Year: 2027, Month: time.March},
			[]domain.CalendarPlan{{Date: civil.Date{Year: 2027, Month: 3, Day: 4}, Projected: true}},
		},
		{"過ぎた月に見込みは出さない", task(today, weekly), domain.CalendarMonth{Year: 2026, Month: time.September}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.ExpandPlans(tt.task, today, tt.month, domain.HemisphereNorth)
			for i := range got {
				got[i].Task = domain.CareTask{} // 比べるのは日付と見込みかどうか
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}

	t.Run("項目にタスクを持つ", func(t *testing.T) {
		t.Parallel()
		tk := task(today, weekly)
		got := domain.ExpandPlans(tk, today, nov, domain.HemisphereNorth)
		if len(got) == 0 || got[0].Task.Kind != domain.CareWatering {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("一時停止中のタスクは予定を出さない", func(t *testing.T) {
		t.Parallel()
		tk := task(today, weekly)
		tk.Status = domain.TaskPaused
		if got := domain.ExpandPlans(tk, today, nov, domain.HemisphereNorth); len(got) != 0 {
			t.Errorf("got %+v", got)
		}
	})
}
