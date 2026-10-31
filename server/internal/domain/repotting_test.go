package domain_test

import (
	"slices"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestRepottingPeriodOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		date       string
		hemisphere domain.Hemisphere
		want       domain.RepottingPeriod
	}{
		// 北半球：適期 4〜6月・9月、真夏 7〜8月、真冬 12〜2月、それ以外（3月・10〜11月）はどちらでもない
		{date: "2026-01-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingMidwinter},
		{date: "2026-02-28", hemisphere: domain.HemisphereNorth, want: domain.RepottingMidwinter},
		{date: "2028-02-29", hemisphere: domain.HemisphereNorth, want: domain.RepottingMidwinter},
		{date: "2026-03-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingNeutral},
		{date: "2026-03-31", hemisphere: domain.HemisphereNorth, want: domain.RepottingNeutral},
		{date: "2026-04-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingSuitable},
		{date: "2026-06-30", hemisphere: domain.HemisphereNorth, want: domain.RepottingSuitable},
		{date: "2026-07-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingMidsummer},
		{date: "2026-08-31", hemisphere: domain.HemisphereNorth, want: domain.RepottingMidsummer},
		{date: "2026-09-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingSuitable},
		{date: "2026-09-30", hemisphere: domain.HemisphereNorth, want: domain.RepottingSuitable},
		{date: "2026-10-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingNeutral},
		{date: "2026-11-30", hemisphere: domain.HemisphereNorth, want: domain.RepottingNeutral},
		{date: "2026-12-01", hemisphere: domain.HemisphereNorth, want: domain.RepottingMidwinter},
		// 南半球：6か月ずれる
		{date: "2026-01-15", hemisphere: domain.HemisphereSouth, want: domain.RepottingMidsummer},
		{date: "2026-03-01", hemisphere: domain.HemisphereSouth, want: domain.RepottingSuitable},
		{date: "2026-07-15", hemisphere: domain.HemisphereSouth, want: domain.RepottingMidwinter},
		{date: "2026-10-01", hemisphere: domain.HemisphereSouth, want: domain.RepottingSuitable},
	}
	for _, tt := range tests {
		t.Run(tt.date+"/"+string(tt.hemisphere), func(t *testing.T) {
			t.Parallel()
			if got := domain.RepottingPeriodOf(date(t, tt.date), tt.hemisphere); got != tt.want {
				t.Errorf("RepottingPeriodOf(%s, %s) = %s, want %s", tt.date, tt.hemisphere, got, tt.want)
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
	}
	for p, w := range want {
		if got := p.ShouldAvoid(); got != w {
			t.Errorf("%s.ShouldAvoid() = %v, want %v", p, got, w)
		}
	}
}

func TestProposeRepotting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		purchasedOn      string
		today            string // 空なら購入日と同じ（登録した日に購入した）
		hemisphere       domain.Hemisphere
		want             string
		afterAcclimation bool
		avoided          []domain.RepottingPeriod
	}{
		{name: "慣らし期間のあとが適期なら、その日", purchasedOn: "2026-04-01", want: "2026-04-15", afterAcclimation: true},
		{name: "慣らし期間のあとが 6/30（適期の最終日）", purchasedOn: "2026-06-16", want: "2026-06-30", afterAcclimation: true},
		{name: "慣らし期間のあとが 7/1（真夏の初日）なら、真夏を飛ばして 9/1", purchasedOn: "2026-06-17", want: "2026-09-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidsummer}},
		{name: "慣らし期間のあとが 8/31（真夏の最終日）なら 9/1", purchasedOn: "2026-08-17", want: "2026-09-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidsummer}},
		{name: "慣らし期間のあとが 9/1 なら、その日", purchasedOn: "2026-08-18", want: "2026-09-01", afterAcclimation: true},
		{name: "慣らし期間のあとが 9/30 なら、その日", purchasedOn: "2026-09-16", want: "2026-09-30", afterAcclimation: true},
		{name: "慣らし期間のあとが 10/1 なら、真冬を飛ばして翌年の 4/1", purchasedOn: "2026-09-17", want: "2027-04-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "慣らし期間のあとが 11/30 なら、翌年の 4/1", purchasedOn: "2026-11-16", want: "2027-04-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "慣らし期間のあとが 2/28（真冬の最終日）なら 4/1", purchasedOn: "2026-02-14", want: "2026-04-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "慣らし期間のあとが 3/1 なら、避ける時期は飛ばさずに 4/1", purchasedOn: "2026-02-15", want: "2026-04-01", afterAcclimation: true},
		{name: "うるう年：慣らし期間のあとが 2/29 なら 4/1", purchasedOn: "2028-02-15", want: "2028-04-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
		{name: "うるう年：慣らし期間のあとが 3/1 なら 4/1", purchasedOn: "2028-02-16", want: "2028-04-01", afterAcclimation: true},
		{name: "慣らし期間が終わっていれば、今日から探す", purchasedOn: "2026-01-10", today: "2026-05-20", want: "2026-05-20"},
		{name: "慣らし期間がちょうど今日で終わる", purchasedOn: "2026-05-06", today: "2026-05-20", want: "2026-05-20", afterAcclimation: true},
		{name: "慣らし期間の途中なら、終わる日から探す", purchasedOn: "2026-05-10", today: "2026-05-20", want: "2026-05-24", afterAcclimation: true},
		{name: "以前に買った植物でも、今日が真夏なら 9/1", purchasedOn: "2025-04-01", today: "2026-07-20", want: "2026-09-01", avoided: []domain.RepottingPeriod{domain.RepottingMidsummer}},
		{name: "南半球：10月は適期", purchasedOn: "2026-10-01", hemisphere: domain.HemisphereSouth, want: "2026-10-15", afterAcclimation: true},
		{name: "南半球：6月は真冬なので 10/1", purchasedOn: "2026-06-01", hemisphere: domain.HemisphereSouth, want: "2026-10-01", afterAcclimation: true, avoided: []domain.RepottingPeriod{domain.RepottingMidwinter}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			today := tt.today
			if today == "" {
				today = tt.purchasedOn
			}
			h := tt.hemisphere
			if h == "" {
				h = domain.HemisphereNorth
			}

			got := domain.ProposeRepotting(date(t, tt.purchasedOn), date(t, today), h)
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

// どの日に買っても、提案日は「起点以降で、最初の適期の日」になる。
func TestProposeRepotting_FirstSuitableDay(t *testing.T) {
	t.Parallel()

	start, end := date(t, "2027-01-01"), date(t, "2029-01-01") // うるう年（2028年）を含む
	for _, h := range []domain.Hemisphere{domain.HemisphereNorth, domain.HemisphereSouth} {
		for purchased := start; purchased.Before(end); purchased = purchased.AddDays(1) {
			got := domain.ProposeRepotting(purchased, purchased, h)
			earliest := purchased.AddDays(domain.RepottingAcclimationDays)

			if got.Earliest != earliest {
				t.Fatalf("%s, %s: Earliest = %s, want %s", h, purchased, got.Earliest, earliest)
			}
			if p := domain.RepottingPeriodOf(got.Date, h); p != domain.RepottingSuitable {
				t.Fatalf("%s, %s: proposed %s is %s, want suitable", h, purchased, got.Date, p)
			}
			for d := earliest; d.Before(got.Date); d = d.AddDays(1) {
				if domain.RepottingPeriodOf(d, h) == domain.RepottingSuitable {
					t.Fatalf("%s, %s: proposed %s, but %s is already suitable", h, purchased, got.Date, d)
				}
			}
		}
	}
}
