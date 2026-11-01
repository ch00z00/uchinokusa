package domain

import (
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// ConflictLastOwner は、唯一のオーナーが退出・ロールの変更をしようとしたときの衝突のコードである（要件2-8）。
const ConflictLastOwner = "last_owner"

// Member は、世帯のメンバーの一覧に表示する情報である。
type Member struct {
	UserID      uuid.UUID
	DisplayName string
	Role        Role
	JoinedAt    time.Time
}

// ParseAssignableRole は、ロールの変更で指定できるロール（編集者・閲覧者）を読む。
// オーナーは指定できない。オーナーの交代は移譲だけで行い、オーナーが常に1人になるようにする。
func ParseAssignableRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleEditor, RoleViewer:
		return r, nil
	}
	return "", &ValidationError{Fields: map[string]string{"role": "invalid"}}
}

// NotificationSettings は、世帯ごとの、メンバー本人の通知設定である（要件15-3・16-3）。
type NotificationSettings struct {
	ReminderEnabled         bool
	ReminderTime            civil.Time // 端末のタイムゾーンでの時刻（分まで）
	CompletionNotifyEnabled bool
}

const reminderTimeLayout = "15:04"

// NewNotificationSettings は、通知設定を検証する。時刻は `HH:MM`（2桁ずつ）とする。
func NewNotificationSettings(reminderEnabled bool, reminderTime string, completionNotifyEnabled bool) (NotificationSettings, error) {
	t, err := time.Parse(reminderTimeLayout, reminderTime)
	// time.Parse は "8:00" も受け付けるため、書式に戻して一致するかで、2桁の形式に限る
	if err != nil || t.Format(reminderTimeLayout) != reminderTime {
		return NotificationSettings{}, &ValidationError{Fields: map[string]string{"reminderTime": "invalid"}}
	}
	return NotificationSettings{
		ReminderEnabled:         reminderEnabled,
		ReminderTime:            civil.Time{Hour: t.Hour(), Minute: t.Minute()},
		CompletionNotifyEnabled: completionNotifyEnabled,
	}, nil
}

// ReminderTimeString は、リマインダーの時刻を `HH:MM` で返す。
func (s NotificationSettings) ReminderTimeString() string {
	return fmt.Sprintf("%02d:%02d", s.ReminderTime.Hour, s.ReminderTime.Minute)
}
