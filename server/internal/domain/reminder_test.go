package domain_test

import (
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestReminderDate(t *testing.T) {
	t.Parallel()
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) civil.Time { return civil.Time{Hour: h, Minute: m} }
	day := func(m time.Month, d int) civil.Date { return civil.Date{Year: 2026, Month: m, Day: d} }

	tests := []struct {
		name   string
		now    time.Time
		loc    *time.Location
		at     civil.Time
		want   civil.Date
		wantOK bool
	}{
		{"時刻ちょうどから送る", time.Date(2026, 11, 8, 8, 0, 0, 0, tokyo), tokyo, at(8, 0), day(11, 8), true},
		{"時刻の前は送らない", time.Date(2026, 11, 8, 7, 59, 59, 0, tokyo), tokyo, at(8, 0), civil.Date{}, false},
		{"1時間の間は送る", time.Date(2026, 11, 8, 8, 59, 59, 0, tokyo), tokyo, at(8, 0), day(11, 8), true},
		{"1時間を過ぎたら送らない", time.Date(2026, 11, 8, 9, 0, 0, 0, tokyo), tokyo, at(8, 0), civil.Date{}, false},
		// 端末のタイムゾーンで判定する（要件15-7）。東京の 8:05 は、ニューヨークでは前の日の 18:05
		{"端末のタイムゾーンの時刻で判定する", time.Date(2026, 11, 8, 8, 5, 0, 0, tokyo), newYork, at(8, 0), civil.Date{}, false},
		{"端末のタイムゾーンの日付を返す", time.Date(2026, 11, 8, 22, 5, 0, 0, tokyo), newYork, at(8, 0), day(11, 8), true},
		// サーバーの時刻（UTC）の日付ではなく、端末のタイムゾーンの日付にする。UTC の 11/8 23:05 は、東京では 11/9 8:05
		{"UTC と日付が違っても、端末の日付", time.Date(2026, 11, 8, 23, 5, 0, 0, time.UTC), tokyo, at(8, 0), day(11, 9), true},
		{"日付をまたいだら、前の日のリマインダー", time.Date(2026, 11, 9, 0, 10, 0, 0, tokyo), tokyo, at(23, 30), day(11, 8), true},
		{"日付をまたぐ設定でも、時刻の前の日のうちは送らない", time.Date(2026, 11, 8, 23, 29, 0, 0, tokyo), tokyo, at(23, 30), civil.Date{}, false},
		// 夏時間が始まる日の 2:30 はない。time.Date の決まりで、時計を進める前の 1:30（EST）から1時間とする
		{"夏時間で、ない時刻", time.Date(2026, 3, 8, 1, 40, 0, 0, newYork), newYork, at(2, 30), day(3, 8), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := domain.ReminderDate(tt.now, tt.loc, tt.at)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ReminderDate = %v, %v, want %v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestReminderNotice(t *testing.T) {
	t.Parallel()
	data := map[string]string{"type": "reminder", "householdId": noticeHome.String()}
	tests := []struct {
		name  string
		items []domain.ReminderItem
		body  string
	}{
		{"1件", []domain.ReminderItem{{Kind: domain.CareWatering}}, "今日のお手入れが1件あります（水やり1件）"},
		{
			"件数と内訳（内訳は出てきた順、その他は作業名）",
			[]domain.ReminderItem{{Kind: domain.CareWatering}, {Kind: domain.CareOther, CustomName: "葉水"}, {Kind: domain.CareWatering}, {Kind: domain.CareFertilizing}},
			"今日のお手入れが4件あります（水やり2件、葉水1件、肥料1件）",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.ReminderNotice(noticeHome, "自宅", tt.items)
			want := domain.PushMessage{Title: "自宅", Body: tt.body, Data: data}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("notice = %+v\nwant     %+v", got, want)
			}
		})
	}
}
