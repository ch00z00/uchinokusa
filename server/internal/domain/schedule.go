package domain

import (
	"time"

	"cloud.google.com/go/civil"
)

// IntervalUnit は、お手入れの周期の単位である。
type IntervalUnit string

const (
	IntervalDay   IntervalUnit = "day"
	IntervalWeek  IntervalUnit = "week"
	IntervalMonth IntervalUnit = "month"
)

// Interval は、お手入れの周期（例：2週に1回）である。Value は1以上とする（検証はタスクの作成時に行う）。
type Interval struct {
	Value int
	Unit  IntervalUnit
}

// AddTo は、日付に周期を足した日を返す。
// 月の場合、足した先の月にその日がなければ、月末に丸める（例：1/31 + 1か月 = 2/28、うるう年は 2/29）。
// 単位が不正な場合は、日付をそのまま返す。
func (i Interval) AddTo(d civil.Date) civil.Date {
	switch i.Unit {
	case IntervalDay:
		return d.AddDays(i.Value)
	case IntervalWeek:
		return d.AddDays(7 * i.Value)
	case IntervalMonth:
		return addMonthsClamped(d, i.Value)
	}
	return d
}

// addMonthsClamped は、n か月後の同じ日を返す。その日がなければ月末にする。
// civil.Date.AddMonths（time.AddDate）は、2/31 を 3/3 のように繰り上げるため使わない。
func addMonthsClamped(d civil.Date, n int) civil.Date {
	// 1日に揃えてから月を足せば、繰り上がりは起きない
	first := civil.Date{Year: d.Year, Month: d.Month, Day: 1}.AddMonths(n)
	first.Day = min(d.Day, daysIn(first.Year, first.Month))
	return first
}

// daysIn は、その月の日数を返す。翌月の0日（＝その月の末日）として求める。
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// Season は、季節である。
type Season string

const (
	SeasonSpring Season = "spring"
	SeasonSummer Season = "summer"
	SeasonAutumn Season = "autumn"
	SeasonWinter Season = "winter"
)

// Hemisphere は、世帯のある半球である。南半球では季節が6か月ずれる。
type Hemisphere string

const (
	HemisphereNorth Hemisphere = "north"
	HemisphereSouth Hemisphere = "south"
)

// SeasonOf は、日付の季節を返す。北半球では、春 3〜5月、夏 6〜8月、秋 9〜11月、冬 12〜2月とする。
func SeasonOf(d civil.Date, h Hemisphere) Season {
	m := d.Month
	if h == HemisphereSouth {
		m = (m+5)%12 + 1 // 6か月ずらす（1月 → 7月、7月 → 1月）
	}
	switch m {
	case time.March, time.April, time.May:
		return SeasonSpring
	case time.June, time.July, time.August:
		return SeasonSummer
	case time.September, time.October, time.November:
		return SeasonAutumn
	}
	return SeasonWinter
}

// SeasonalIntervals は、季節ごとの周期である。設定のない季節は、基本の周期を使う。
type SeasonalIntervals map[Season]Interval

// Schedule は、お手入れタスクの周期の設定である（要件8-6）。
type Schedule struct {
	Base     Interval
	Seasonal SeasonalIntervals
}

// EffectiveInterval は、その日に有効な周期を返す。その日の季節の周期があれば、それを優先する。
func (s Schedule) EffectiveInterval(on civil.Date, h Hemisphere) Interval {
	if i, ok := s.Seasonal[SeasonOf(on, h)]; ok {
		return i
	}
	return s.Base
}

// NextDueAfterCompletion は、完了したときの次回予定を返す。
// 実施日を起点に、実施日の季節で有効な周期を足す（要件10-2・10-3・10-5）。
// 次回予定は必ず実施日より後にする。周期が不正でも、実施日の翌日にする。
func (s Schedule) NextDueAfterCompletion(performedOn civil.Date, h Hemisphere) civil.Date {
	next := s.EffectiveInterval(performedOn, h).AddTo(performedOn)
	if !next.After(performedOn) {
		return performedOn.AddDays(1)
	}
	return next
}
