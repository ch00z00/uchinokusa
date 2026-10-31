package domain_test

import (
	"slices"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// tropicalRule は、夏も生育期として扱う種類のルールの例である（適期 5〜9月、避ける時期 12〜2月）。
var tropicalRule = mustRule([]int{5, 6, 7, 8, 9}, []int{12, 1, 2})

func mustRule(suitable, avoid []int) domain.RepottingRule {
	r, err := domain.NewRepottingRule(suitable, avoid)
	if err != nil {
		panic(err)
	}
	return r
}

func TestNewRepottingRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		suitable []int
		avoid    []int
		wantErr  bool
	}{
		{name: "適期と避ける月", suitable: []int{5, 6, 9}, avoid: []int{7, 8, 12, 1, 2}},
		{name: "避ける月はなくてもよい", suitable: []int{4, 5}},
		{name: "適期がない", avoid: []int{1}, wantErr: true},
		{name: "月が範囲外（0）", suitable: []int{0, 5}, wantErr: true},
		{name: "月が範囲外（13）", suitable: []int{5}, avoid: []int{13}, wantErr: true},
		{name: "適期と避ける月が重なる", suitable: []int{5, 6}, avoid: []int{6, 7}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewRepottingRule(tt.suitable, tt.avoid)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepottingRule_PeriodOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rule       domain.RepottingRule
		date       string
		hemisphere domain.Hemisphere
		want       domain.RepottingPeriod
	}{
		// 既定値（北半球）：適期 5〜6月・9月、真夏 7〜8月、真冬 12〜2月、それ以外（3〜4月・10〜11月）はどちらでもない
		{rule: domain.DefaultRepottingRule, date: "2026-01-01", want: domain.RepottingMidwinter},
		{rule: domain.DefaultRepottingRule, date: "2026-02-28", want: domain.RepottingMidwinter},
		{rule: domain.DefaultRepottingRule, date: "2028-02-29", want: domain.RepottingMidwinter},
		{rule: domain.DefaultRepottingRule, date: "2026-03-01", want: domain.RepottingNeutral},
		{rule: domain.DefaultRepottingRule, date: "2026-04-30", want: domain.RepottingNeutral},
		{rule: domain.DefaultRepottingRule, date: "2026-05-01", want: domain.RepottingSuitable},
		{rule: domain.DefaultRepottingRule, date: "2026-06-30", want: domain.RepottingSuitable},
		{rule: domain.DefaultRepottingRule, date: "2026-07-01", want: domain.RepottingMidsummer},
		{rule: domain.DefaultRepottingRule, date: "2026-08-31", want: domain.RepottingMidsummer},
		{rule: domain.DefaultRepottingRule, date: "2026-09-01", want: domain.RepottingSuitable},
		{rule: domain.DefaultRepottingRule, date: "2026-09-30", want: domain.RepottingSuitable},
		{rule: domain.DefaultRepottingRule, date: "2026-10-01", want: domain.RepottingNeutral},
		{rule: domain.DefaultRepottingRule, date: "2026-11-30", want: domain.RepottingNeutral},
		{rule: domain.DefaultRepottingRule, date: "2026-12-01", want: domain.RepottingMidwinter},
		// 既定値（南半球）：6か月ずれる
		{rule: domain.DefaultRepottingRule, date: "2026-01-15", hemisphere: domain.HemisphereSouth, want: domain.RepottingMidsummer},
		{rule: domain.DefaultRepottingRule, date: "2026-03-01", hemisphere: domain.HemisphereSouth, want: domain.RepottingSuitable},
		{rule: domain.DefaultRepottingRule, date: "2026-07-15", hemisphere: domain.HemisphereSouth, want: domain.RepottingMidwinter},
		{rule: domain.DefaultRepottingRule, date: "2026-10-31", hemisphere: domain.HemisphereSouth, want: domain.RepottingNeutral},
		{rule: domain.DefaultRepottingRule, date: "2026-11-01", hemisphere: domain.HemisphereSouth, want: domain.RepottingSuitable},
		// 種類ごとのルール
		{name: "夏も生育期の種類では、7月は適期", rule: tropicalRule, date: "2026-07-15", want: domain.RepottingSuitable},
		{name: "春・秋に避ける月は、汎用の「避ける」", rule: mustRule([]int{9, 10}, []int{3, 4}), date: "2026-04-10", want: domain.RepottingAvoid},
		{name: "6月を避ける月にした場合は、真夏として扱う", rule: mustRule([]int{9}, []int{6}), date: "2026-06-10", want: domain.RepottingMidsummer},
	}
	for _, tt := range tests {
		name := tt.name
		if name == "" {
			name = tt.date + "/" + string(tt.hemisphere)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := tt.hemisphere
			if h == "" {
				h = domain.HemisphereNorth
			}
			if got := tt.rule.PeriodOf(date(t, tt.date), h); got != tt.want {
				t.Errorf("PeriodOf(%s, %s) = %s, want %s", tt.date, h, got, tt.want)
			}
		})
	}
}

func TestRepottingPeriod_ShouldAvoid(t *testing.T) {
	t.Parallel()

	want := map[domain.RepottingPeriod]bool{
		domain.RepottingSuitable:  false,
		domain.RepottingNeutral:   false,
		domain.RepottingMidsummer: true,
		domain.RepottingMidwinter: true,
		domain.RepottingAvoid:     true,
	}
	for p, w := range want {
		if got := p.ShouldAvoid(); got != w {
			t.Errorf("%s.ShouldAvoid() = %v, want %v", p, got, w)
		}
	}
}

func TestRepottingRule_Propose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		rule             domain.RepottingRule // ゼロ値なら既定値
		purchasedOn      string
		today            string // 空なら購入日と同じ（登録した日に購入した）
		hemisphere       domain.Hemisphere
		want             string
		afterAcclimation bool
		avoided          []domain.RepottingPeriod
	}{
		{name: "慣らし期間のあとが適期なら、その日", purchasedOn: "2026-05-01", want: "2026-05-15", afterAcclimation: true},
		{name: "慣らし期間のあとが 4/30 なら 5/1（避ける時期は飛ばさない）", purchasedOn: "2026-04-16", want: "2026-05-01", afterAcclimation: true},
		{name: "慣らし期間のあとが 6/30（適期の最終日）", purchasedOn: "2026-06-16", want: "2026-06-30", afterAcclimation: true},
		{name: "慣らし期間のあとが 7/1（真夏の初日）なら、真夏を飛ばして 9/1", purchasedOn: "2026-06-17", want: "2026-09-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidsummer}},
		{name: "慣らし期間のあとが 8/31（真夏の最終日）なら 9/1", purchasedOn: "2026-08-17", want: "2026-09-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidsummer}},
		{name: "慣らし期間のあとが 9/1 なら、その日", purchasedOn: "2026-08-18", want: "2026-09-01", afterAcclimation: true},
		{name: "慣らし期間のあとが 9/30 なら、その日", purchasedOn: "2026-09-16", want: "2026-09-30", afterAcclimation: true},
		{name: "慣らし期間のあとが 10/1 なら、真冬を飛ばして翌年の 5/1", purchasedOn: "2026-09-17", want: "2027-05-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "慣らし期間のあとが 11/30 なら、翌年の 5/1", purchasedOn: "2026-11-16", want: "2027-05-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "慣らし期間のあとが 2/28（真冬の最終日）なら 5/1", purchasedOn: "2026-02-14", want: "2026-05-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "慣らし期間のあとが 3/1 なら、避ける時期は飛ばさずに 5/1", purchasedOn: "2026-02-15", want: "2026-05-01", afterAcclimation: true},
		{name: "うるう年：慣らし期間のあとが 2/29 なら 5/1", purchasedOn: "2028-02-15", want: "2028-05-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "うるう年：慣らし期間のあとが 3/1 なら 5/1", purchasedOn: "2028-02-16", want: "2028-05-01", afterAcclimation: true},
		{name: "慣らし期間が終わっていれば、今日から探す", purchasedOn: "2026-01-10", today: "2026-05-20", want: "2026-05-20"},
		{name: "慣らし期間がちょうど今日で終わる", purchasedOn: "2026-05-06", today: "2026-05-20", want: "2026-05-20", afterAcclimation: true},
		{name: "慣らし期間の途中なら、終わる日から探す", purchasedOn: "2026-05-10", today: "2026-05-20", want: "2026-05-24", afterAcclimation: true},
		{name: "以前に買った植物でも、今日が真夏なら 9/1", purchasedOn: "2025-04-01", today: "2026-07-20", want: "2026-09-01", avoided: []domain.RepottingPeriod{domain.RepottingMidsummer}},
		{name: "南半球：11月は適期", purchasedOn: "2026-11-01", hemisphere: domain.HemisphereSouth, want: "2026-11-15", afterAcclimation: true},
		{name: "南半球：6月は真冬なので 11/1", purchasedOn: "2026-06-01", hemisphere: domain.HemisphereSouth, want: "2026-11-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "種類ごとのルール：夏も生育期なら、7/1 をそのまま提案する", rule: tropicalRule, purchasedOn: "2026-06-17", want: "2026-07-01", afterAcclimation: true},
		{name: "種類ごとのルール：春に避ける月を飛ばす", rule: mustRule([]int{9, 10}, []int{3, 4}), purchasedOn: "2026-02-20", want: "2026-09-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingAvoid}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rule := tt.rule
			if rule == (domain.RepottingRule{}) {
				rule = domain.DefaultRepottingRule
			}
			today := tt.today
			if today == "" {
				today = tt.purchasedOn
			}
			h := tt.hemisphere
			if h == "" {
				h = domain.HemisphereNorth
			}

			got := rule.Propose(date(t, tt.purchasedOn), date(t, today), h)
			if got.Date != date(t, tt.want) {
				t.Errorf("Date = %s, want %s", got.Date, tt.want)
			}
			if got.AfterAcclimation != tt.afterAcclimation {
				t.Errorf("AfterAcclimation = %v, want %v", got.AfterAcclimation, tt.afterAcclimation)
			}
			if !slices.Equal(got.Avoided, tt.avoided) {
				t.Errorf("Avoided = %v, want %v", got.Avoided, tt.avoided)
			}
		})
	}
}

// 適期のない不正なルール（ゼロ値など）では、既定値で提案する。
func TestRepottingRule_Propose_InvalidRuleFallsBackToDefault(t *testing.T) {
	t.Parallel()
	purchased, today := date(t, "2026-06-17"), date(t, "2026-06-17")

	got := domain.RepottingRule{}.Propose(purchased, today, domain.HemisphereNorth)
	want := domain.DefaultRepottingRule.Propose(purchased, today, domain.HemisphereNorth)
	if got.Date != want.Date {
		t.Errorf("Date = %s, want %s (default rule)", got.Date, want.Date)
	}
}

// どの日に買っても、提案日は「起点以降で、最初の適期の日」になる。
func TestRepottingRule_Propose_FirstSuitableDay(t *testing.T) {
	t.Parallel()

	rules := []domain.RepottingRule{domain.DefaultRepottingRule, tropicalRule, mustRule([]int{10}, []int{3, 4, 7})}
	start, end := date(t, "2027-01-01"), date(t, "2029-01-01") // うるう年（2028年）を含む
	for _, rule := range rules {
		for _, h := range []domain.Hemisphere{domain.HemisphereNorth, domain.HemisphereSouth} {
			for purchased := start; purchased.Before(end); purchased = purchased.AddDays(1) {
				got := rule.Propose(purchased, purchased, h)
				earliest := purchased.AddDays(domain.RepottingAcclimationDays)

				if got.Earliest != earliest {
					t.Fatalf("%s, %s: Earliest = %s, want %s", h, purchased, got.Earliest, earliest)
				}
				if p := rule.PeriodOf(got.Date, h); p != domain.RepottingSuitable {
					t.Fatalf("%+v, %s, %s: proposed %s is %s, want suitable", rule, h, purchased, got.Date, p)
				}
				for d := earliest; d.Before(got.Date); d = d.AddDays(1) {
					if rule.PeriodOf(d, h) == domain.RepottingSuitable {
						t.Fatalf("%+v, %s, %s: proposed %s, but %s is already suitable", rule, h, purchased, got.Date, d)
					}
				}
			}
		}
	}
}
