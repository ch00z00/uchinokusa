package domain_test

import (
	"testing"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestParseAssignableRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in         string
		want       domain.Role
		wantFields map[string]string
	}{
		{in: "editor", want: domain.RoleEditor},
		{in: "viewer", want: domain.RoleViewer},
		// オーナーの交代は移譲だけで行う（オーナーが2人になることを防ぐ）
		{in: "owner", wantFields: map[string]string{"role": "invalid"}},
		{in: "admin", wantFields: map[string]string{"role": "invalid"}},
		{in: "", wantFields: map[string]string{"role": "invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseAssignableRole(tt.in)
			assertFields(t, err, tt.wantFields)
			if tt.wantFields == nil && got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNewNotificationSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reminderTime string
		want         civil.Time
		wantFields   map[string]string
	}{
		{reminderTime: "08:00", want: civil.Time{Hour: 8}},
		{reminderTime: "00:00", want: civil.Time{}},
		{reminderTime: "23:59", want: civil.Time{Hour: 23, Minute: 59}},
		{reminderTime: "8:00", wantFields: map[string]string{"reminderTime": "invalid"}}, // 2桁にそろえる
		{reminderTime: "24:00", wantFields: map[string]string{"reminderTime": "invalid"}},
		{reminderTime: "12:60", wantFields: map[string]string{"reminderTime": "invalid"}},
		{reminderTime: "08:00:00", wantFields: map[string]string{"reminderTime": "invalid"}},
		{reminderTime: "", wantFields: map[string]string{"reminderTime": "invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.reminderTime, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewNotificationSettings(true, tt.reminderTime, false)
			assertFields(t, err, tt.wantFields)
			if tt.wantFields != nil {
				return
			}
			want := domain.NotificationSettings{ReminderEnabled: true, ReminderTime: tt.want, CompletionNotifyEnabled: false}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
			if s := got.ReminderTimeString(); s != tt.reminderTime {
				t.Errorf("ReminderTimeString() = %q, want %q", s, tt.reminderTime)
			}
		})
	}
}
