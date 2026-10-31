package domain

import (
	"time"

	"cloud.google.com/go/civil"
)

// RepottingAcclimationDays は、購入してから植え替えまでに置く慣らし期間の日数である（要件17-2）。
// 新しい環境に慣れる前の植え替えは株を傷めやすいため。将来は設定テーブルへ移す。
const RepottingAcclimationDays = 14

// RepottingPeriod は、植え替えの時期としての分類である（要件17-3）。
type RepottingPeriod string

const (
	RepottingSuitable  RepottingPeriod = "suitable"  // 適期：4〜6月、9月
	RepottingNeutral   RepottingPeriod = "neutral"   // 適期ではないが、避けるほどでもない：3月、10〜11月
	RepottingMidsummer RepottingPeriod = "midsummer" // 避ける時期（真夏）：7〜8月
	RepottingMidwinter RepottingPeriod = "midwinter" // 避ける時期（真冬）：12〜2月
)

// ShouldAvoid は、植え替えを避ける時期かを返す。予定日がこの時期なら、やらない方がよいことの助言を出す（要件17-8）。
func (p RepottingPeriod) ShouldAvoid() bool {
	return p == RepottingMidsummer || p == RepottingMidwinter
}

// RepottingPeriodOf は、日付が植え替えの時期としてどれに当たるかを返す。南半球では6か月ずらす。
func RepottingPeriodOf(d civil.Date, h Hemisphere) RepottingPeriod {
	switch northernMonth(d, h) {
	case time.April, time.May, time.June, time.September:
		return RepottingSuitable
	case time.July, time.August:
		return RepottingMidsummer
	case time.December, time.January, time.February:
		return RepottingMidwinter
	}
	return RepottingNeutral
}

// RepottingProposal は、植え替えの提案日と、その日を選んだ理由の材料である。
// 画面の文言（「慣らし期間のあと、真夏を避けた最初の適期です」など）は、これをもとに i18n で組み立てる（要件17-7）。
type RepottingProposal struct {
	Date             civil.Date        // 提案日
	Earliest         civil.Date        // 探し始めた日（購入日 + 慣らし期間と、今日の遅いほう）
	AfterAcclimation bool              // 慣らし期間の終わりを待った（今日はまだ慣らし期間の中、または最終日）
	Avoided          []RepottingPeriod // Earliest から提案日までに飛ばした、避ける時期（順に、重複なし）
}

// ProposeRepotting は、植え替えの提案日を求める（要件17-2・17-3）。
// 購入日 + 慣らし期間と今日の遅いほうから探し始め、その日が適期ならその日、
// そうでなければ、次に適期に入る月の1日を提案日にする。
func ProposeRepotting(purchasedOn, today civil.Date, h Hemisphere) RepottingProposal {
	acclimated := purchasedOn.AddDays(RepottingAcclimationDays)
	p := RepottingProposal{Earliest: today, AfterAcclimation: !acclimated.Before(today)}
	if p.AfterAcclimation {
		p.Earliest = acclimated
	}

	d := p.Earliest
	// 適期は毎年あるため、12か月以内に必ず見つかる
	for range 12 {
		period := RepottingPeriodOf(d, h)
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
