package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestTodayDate(t *testing.T) {
	t.Parallel()
	fallback := civil.Date{Year: 2026, Month: 11, Day: 4}

	for _, tt := range []struct {
		name string
		in   *string
		want civil.Date
	}{
		{"アプリが送った端末の日付を使う", ptr("2026-11-05"), civil.Date{Year: 2026, Month: 11, Day: 5}},
		{"省くと、世帯のタイムゾーンでの今日にする", nil, fallback},
		{"空も省いたのと同じ", ptr(" "), fallback},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.TodayDate(tt.in, fallback)
			if err != nil || got != tt.want {
				t.Errorf("got %v, %v; want %v", got, err, tt.want)
			}
		})
	}

	t.Run("形式の誤りは 422", func(t *testing.T) {
		t.Parallel()
		_, err := domain.TodayDate(ptr("11/5"), fallback)
		var v *domain.ValidationError
		if !errors.As(err, &v) || !reflect.DeepEqual(v.Fields, map[string]string{"date": "invalid"}) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestNewTodayItem(t *testing.T) {
	t.Parallel()
	today := civil.Date{Year: 2026, Month: 11, Day: 4}
	for _, tt := range []struct {
		name string
		due  civil.Date
		want int
	}{
		{"今日が予定日なら 0 日", today, 0},
		{"予定日を過ぎた日数", civil.Date{Year: 2026, Month: 10, Day: 30}, 5},
		// 直近の次の予定（要件9-6）は、まだ来ていないので 0 日
		{"予定日が先なら 0 日", civil.Date{Year: 2026, Month: 11, Day: 8}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item := domain.NewTodayItem(domain.TodayTask{Task: domain.CareTask{NextDueOn: tt.due}}, today)
			if item.OverdueDays != tt.want {
				t.Errorf("OverdueDays = %d, want %d", item.OverdueDays, tt.want)
			}
		})
	}
}
