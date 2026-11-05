package domain

import (
	"strings"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// TodayDate は、今日やること一覧の「今日」を決める。アプリが送った端末の日付を使い、省けば fallback（世帯のタイムゾーンでの今日）にする。
func TodayDate(s *string, fallback civil.Date) (civil.Date, error) {
	v := strings.TrimSpace(deref(s))
	if v == "" {
		return fallback, nil
	}
	d, err := civil.ParseDate(v)
	if err != nil || !d.IsValid() {
		return civil.Date{}, &ValidationError{Fields: map[string]string{"date": "invalid"}}
	}
	return d, nil
}

// TodayTask は、今日やること一覧に載せるタスクと、表示に使う植物・置き場所である。
type TodayTask struct {
	Task         CareTask
	PlantName    string
	LocationID   uuid.UUID // 置き場所が未設定なら uuid.Nil
	LocationName string
}

// TodayItem は、今日やること一覧の1件である（要件9-2〜9-4）。
type TodayItem struct {
	TodayTask
	OverdueDays int          // 予定日を過ぎた日数。今日が予定日なら 0
	Suggestions []Suggestion // タスクへの有効な「やらない方がよい」「条件つき」の提案
}

// NewTodayItem は、today の時点の項目を作る。予定日がまだ先なら、過ぎた日数は 0 にする。
func NewTodayItem(t TodayTask, today civil.Date) TodayItem {
	return TodayItem{TodayTask: t, OverdueDays: max(today.DaysSince(t.Task.NextDueOn), 0)}
}

// TodayList は、今日やること一覧である。
type TodayList struct {
	Date  civil.Date
	Items []TodayItem
	Next  *TodayItem // Items が空のときの、直近の次の予定（要件9-6）。予定がなければ nil
}
