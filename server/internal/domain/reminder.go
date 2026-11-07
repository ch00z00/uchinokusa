package domain

import (
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// ReminderWindow は、リマインダーの時刻を過ぎてから送ってよい時間である。
// ジョブの実行が抜けても届くようにしつつ、時刻から大きく遅れて届かないようにする（昼に参加したメンバーに、朝のリマインダーが届くなど）。
const ReminderWindow = time.Hour

// ReminderDate は、now がリマインダーの時刻 at（loc での時刻）から ReminderWindow の間にあるかを判定し、
// あれば、そのリマインダーの日付（loc での日付）を返す。日付をまたぐ場合（23:30 の設定で 0:10）は、前の日の日付を返す。
func ReminderDate(now time.Time, loc *time.Location, at civil.Time) (civil.Date, bool) {
	today := civil.DateOf(now.In(loc))
	for _, d := range []civil.Date{today, today.AddDays(-1)} {
		// 夏時間が始まる日にない時刻（2:30 など）は、time.Date の決まりで、時計を進める前の時刻（1:30）になる
		start := time.Date(d.Year, d.Month, d.Day, at.Hour, at.Minute, 0, 0, loc)
		if !now.Before(start) && now.Before(start.Add(ReminderWindow)) {
			return d, true
		}
	}
	return civil.Date{}, false
}

// ReminderItem は、リマインダーにまとめる1件のタスクである。
type ReminderItem struct {
	Kind       CareKind
	CustomName string
}

// ReminderNotice は、世帯の今日以前の未完了タスクの件数と内訳を知らせる通知を作る（要件15-1・15-5）。items は1件以上。
// タップすると、その世帯の今日やること一覧を開く。
func ReminderNotice(householdID uuid.UUID, householdName string, items []ReminderItem) PushMessage {
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, careLabel(it.Kind, it.CustomName))
	}
	return PushMessage{
		Title: householdName,
		Body:  fmt.Sprintf("今日のお手入れが%d件あります（%s）", len(items), breakdown(labels)),
		Data:  map[string]string{"type": string(NoticeReminder), "householdId": householdID.String()},
	}
}

// ReminderCandidate は、リマインダーを送るかもしれないメンバー（リマインダーが有効で、端末があるメンバー）である。
type ReminderCandidate struct {
	MembershipID uuid.UUID
	HouseholdID  uuid.UUID
	UserID       uuid.UUID
	ReminderTime civil.Time
	// TimeZone は、メンバーが最後に使った端末のタイムゾーンである（要件15-7）
	TimeZone string
}

// ReminderBatch は、1人のメンバーに送るリマインダーの内容である。
type ReminderBatch struct {
	ReminderCandidate
	Date          civil.Date // リマインダーの日付（端末のタイムゾーンでの日付）
	HouseholdName string
	Items         []ReminderItem // 今日以前の未完了タスク（予定日の古い順）
}
