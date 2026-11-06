package domain

import (
	"strings"
	"time"

	"cloud.google.com/go/civil"
)

// MaxCalendarMonthsAhead は、カレンダーで見られる、今月から先の月数である。
// 見込みの予定は、今の予定日から1回ずつ展開するので、遠い先の月ほど計算が長くなる。その上限を決めるため。
const MaxCalendarMonthsAhead = 24

// CalendarMonth は、カレンダーの月である。
type CalendarMonth struct {
	Year  int
	Month time.Month
}

// ParseCalendarMonth は、カレンダーの月（YYYY-MM）を検証する。省くと today の月にする。
// 過去の月はいくらでも戻れる（記録を見るため）。先の月は MaxCalendarMonthsAhead までにする。
func ParseCalendarMonth(s *string, today civil.Date) (CalendarMonth, error) {
	m := CalendarMonth{Year: today.Year, Month: today.Month}
	if v := strings.TrimSpace(deref(s)); v != "" {
		t, err := time.Parse("2006-01", v)
		if err != nil {
			return CalendarMonth{}, &ValidationError{Fields: map[string]string{"month": "invalid"}}
		}
		m = CalendarMonth{Year: t.Year(), Month: t.Month()}
	}
	if m.index()-(CalendarMonth{Year: today.Year, Month: today.Month}).index() > MaxCalendarMonthsAhead {
		return CalendarMonth{}, &ValidationError{Fields: map[string]string{"month": "invalid"}}
	}
	return m, nil
}

func (m CalendarMonth) index() int { return m.Year*12 + int(m.Month) - 1 }

// First は、月の最初の日を返す。
func (m CalendarMonth) First() civil.Date { return civil.Date{Year: m.Year, Month: m.Month, Day: 1} }

// Last は、月の最後の日を返す。
func (m CalendarMonth) Last() civil.Date {
	return civil.Date{Year: m.Year, Month: m.Month, Day: daysIn(m.Year, m.Month)}
}

func (m CalendarMonth) String() string { return m.First().String()[:7] }

// CalendarPlan は、カレンダーに出す予定の1件である。
type CalendarPlan struct {
	Date civil.Date
	Task CareTask
	// Projected は、周期から展開した見込みであることを表す。false はタスクの今の予定日（完了の対象になる回）。
	Projected bool
}

// ExpandPlans は、タスクの予定を、月の範囲で展開する（要件14-1・14-5）。一時停止中のタスクは予定を出さない。
//   - 1件目は、タスクの今の予定日（期限切れでも、その日に出す）
//   - 2件目からは、前の予定日に作業したものとして、その日の季節の周期を足す（完了と同じ計算）
//   - 期限切れなら、早くても今日作業するので、2件目は今日から数える
func ExpandPlans(t CareTask, today civil.Date, m CalendarMonth, h Hemisphere) []CalendarPlan {
	if t.Status != TaskActive || t.NextDueOn.IsZero() {
		return nil
	}
	first, last := m.First(), m.Last()
	var out []CalendarPlan
	add := func(d civil.Date, projected bool) {
		if !d.Before(first) && !d.After(last) {
			out = append(out, CalendarPlan{Date: d, Task: t, Projected: projected})
		}
	}
	add(t.NextDueOn, false)
	d := t.NextDueOn
	if d.Before(today) {
		d = today
	}
	// NextDueAfterCompletion は必ず先の日を返すので、この繰り返しは終わる
	for d = t.Schedule.NextDueAfterCompletion(d, h); !d.After(last); d = t.Schedule.NextDueAfterCompletion(d, h) {
		add(d, true)
	}
	return out
}

// CalendarNote は、カレンダーに出すメモと、それを書いた日（世帯のタイムゾーン）である。
type CalendarNote struct {
	Date civil.Date
	Note Note
}

// PlantCalendar は、植物の1か月のカレンダーである（要件14）。
type PlantCalendar struct {
	Month   CalendarMonth
	Records []CareRecord // 実施日がその月の記録（完了・スキップ）
	Plans   []CalendarPlan
	Notes   []CalendarNote
}
