package domain

import (
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
)

// RepottingAcclimationDays は、購入してから植え替えまでに置く慣らし期間の日数である（要件17-2）。
// 新しい環境に慣れる前の植え替えは株を傷めやすいため。将来は設定テーブルへ移す。
const RepottingAcclimationDays = 14

// RepottingPeriod は、植え替えの時期としての分類である（要件17-3）。
type RepottingPeriod string

const (
	RepottingSuitable  RepottingPeriod = "suitable"  // 適期
	RepottingNeutral   RepottingPeriod = "neutral"   // 適期ではないが、避けるほどでもない
	RepottingMidsummer RepottingPeriod = "midsummer" // 避ける時期（夏）：暑さで生長が鈍り、回復しにくい
	RepottingMidwinter RepottingPeriod = "midwinter" // 避ける時期（冬）：寒さで根が傷みやすい
	RepottingAvoid     RepottingPeriod = "avoid"     // 避ける時期（春・秋）：種類ごとのルールでだけ現れる
)

// ShouldAvoid は、植え替えを避ける時期かを返す。予定日がこの時期なら、やらない方がよいことの助言を出す（要件17-8）。
func (p RepottingPeriod) ShouldAvoid() bool {
	return p == RepottingMidsummer || p == RepottingMidwinter || p == RepottingAvoid
}

// Months は、月の集合である（ビット i が i 月を表す）。値の型なので、コピーしても共有されない。
type Months uint16

func (m Months) Has(month time.Month) bool { return m&(1<<month) != 0 }

// RepottingRule は、植え替えの適期と避ける月である。月は北半球のもので、南半球では6か月ずらして判定する。
// 植物マスタに種類ごとのルールがあればそれを使い、なければ DefaultRepottingRule を使う（要件17-3）。
type RepottingRule struct {
	Suitable Months
	Avoid    Months
}

// DefaultRepottingRule は、種類ごとのルールがない場合の既定値である（出典は docs/decisions/0009）。
// 適期 5〜6月・9月、避ける時期 7〜8月（真夏）・12〜2月（真冬）。3〜4月・10〜11月はどちらでもない。
var DefaultRepottingRule = mustRepottingRule([]int{5, 6, 9}, []int{7, 8, 12, 1, 2})

// NewRepottingRule は、月の番号（1〜12）からルールを作る。植物マスタのデータを読み込むときに使う。
// 適期が1つもない、月が範囲外、適期と避ける月が重なる場合はエラーにする。
func NewRepottingRule(suitable, avoid []int) (RepottingRule, error) {
	s, err := monthsOf(suitable)
	if err != nil {
		return RepottingRule{}, fmt.Errorf("suitable: %w", err)
	}
	a, err := monthsOf(avoid)
	if err != nil {
		return RepottingRule{}, fmt.Errorf("avoid: %w", err)
	}
	switch {
	case s == 0:
		return RepottingRule{}, errors.New("no suitable months")
	case s&a != 0:
		return RepottingRule{}, errors.New("suitable and avoid months overlap")
	}
	return RepottingRule{Suitable: s, Avoid: a}, nil
}

func mustRepottingRule(suitable, avoid []int) RepottingRule {
	r, err := NewRepottingRule(suitable, avoid)
	if err != nil {
		panic(err)
	}
	return r
}

func monthsOf(months []int) (Months, error) {
	var m Months
	for _, n := range months {
		if n < 1 || n > 12 {
			return 0, fmt.Errorf("month %d out of range", n)
		}
		m |= 1 << n
	}
	return m, nil
}

// PeriodOf は、日付が植え替えの時期としてどれに当たるかを返す。
// 避ける月は、北半球での季節によって、真夏・真冬・それ以外に分ける（理由の文言を変えるため）。
func (r RepottingRule) PeriodOf(d civil.Date, h Hemisphere) RepottingPeriod {
	m := northernMonth(d, h)
	switch {
	case r.Suitable.Has(m):
		return RepottingSuitable
	case !r.Avoid.Has(m):
		return RepottingNeutral
	}
	switch SeasonOf(civil.Date{Year: d.Year, Month: m, Day: 1}, HemisphereNorth) {
	case SeasonSummer:
		return RepottingMidsummer
	case SeasonWinter:
		return RepottingMidwinter
	}
	return RepottingAvoid
}

// RepottingProposal は、植え替えの提案日と、その日を選んだ理由の材料である。
// 画面の文言（「慣らし期間のあと、真夏を避けた最初の適期です」など）は、これをもとに i18n で組み立てる（要件17-7）。
type RepottingProposal struct {
	Date             civil.Date        // 提案日
	Earliest         civil.Date        // 探し始めた日（購入日 + 慣らし期間と、今日の遅いほう）
	AfterAcclimation bool              // 慣らし期間の終わりを待った（今日はまだ慣らし期間の中、または最終日）
	Avoided          []RepottingPeriod // Earliest から提案日までに飛ばした、避ける時期（順に、重複なし）
}

// Propose は、植え替えの提案日を求める（要件17-2・17-3）。
// 購入日 + 慣らし期間と今日の遅いほうから探し始め、その日が適期ならその日、
// そうでなければ、次に適期に入る月の1日を提案日にする。
// 適期のない不正なルール（ゼロ値など）では、既定値を使う。
func (r RepottingRule) Propose(purchasedOn, today civil.Date, h Hemisphere) RepottingProposal {
	if r.Suitable == 0 {
		r = DefaultRepottingRule
	}

	acclimated := purchasedOn.AddDays(RepottingAcclimationDays)
	p := RepottingProposal{Earliest: today, AfterAcclimation: !acclimated.Before(today)}
	if p.AfterAcclimation {
		p.Earliest = acclimated
	}

	d := p.Earliest
	// 適期は毎年あるため、12か月以内に必ず見つかる
	for range 12 {
		period := r.PeriodOf(d, h)
		if period == RepottingSuitable {
			break
		}
		if period.ShouldAvoid() && (len(p.Avoided) == 0 || p.Avoided[len(p.Avoided)-1] != period) {
			p.Avoided = append(p.Avoided, period)
		}
		d = civil.Date{Year: d.Year, Month: d.Month, Day: 1}.AddMonths(1) // 次の月の1日
	}
	p.Date = d
	return p
}
