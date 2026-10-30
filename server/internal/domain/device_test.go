package domain_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNewDeviceRegistration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fcmToken   string
		platform   string
		timeZone   string
		wantFields map[string]string // ValidationError の fields。nil なら成功
	}{
		{name: "iOS の端末", fcmToken: "token-1", platform: "ios", timeZone: "Asia/Tokyo"},
		{name: "Android の端末", fcmToken: "token-1", platform: "android", timeZone: "America/New_York"},
		{name: "UTC も使える", fcmToken: "token-1", platform: "ios", timeZone: "UTC"},
		{name: "トークンが空", fcmToken: "", platform: "ios", timeZone: "Asia/Tokyo", wantFields: map[string]string{"fcmToken": "required"}},
		{name: "トークンが長すぎる", fcmToken: strings.Repeat("a", 4097), platform: "ios", timeZone: "Asia/Tokyo", wantFields: map[string]string{"fcmToken": "too_long"}},
		{name: "知らない OS", fcmToken: "token-1", platform: "windows", timeZone: "Asia/Tokyo", wantFields: map[string]string{"platform": "invalid"}},
		{name: "OS の大文字・小文字は区別する", fcmToken: "token-1", platform: "iOS", timeZone: "Asia/Tokyo", wantFields: map[string]string{"platform": "invalid"}},
		{name: "タイムゾーンが空", fcmToken: "token-1", platform: "ios", timeZone: "", wantFields: map[string]string{"timeZone": "required"}},
		{name: "存在しないタイムゾーン", fcmToken: "token-1", platform: "ios", timeZone: "Asia/Nowhere", wantFields: map[string]string{"timeZone": "invalid"}},
		// time.LoadLocation は "Local" をサーバーのタイムゾーンとして受け付けるが、端末のタイムゾーンとしては意味がない
		{name: "Local は使えない", fcmToken: "token-1", platform: "ios", timeZone: "Local", wantFields: map[string]string{"timeZone": "invalid"}},
		{
			name: "複数の誤りは、まとめて返す", fcmToken: "", platform: "", timeZone: "x",
			wantFields: map[string]string{"fcmToken": "required", "platform": "invalid", "timeZone": "invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewDeviceRegistration(tt.fcmToken, tt.platform, tt.timeZone)
			if tt.wantFields == nil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				want := domain.DeviceRegistration{FCMToken: tt.fcmToken, Platform: domain.Platform(tt.platform), TimeZone: tt.timeZone}
				if got != want {
					t.Errorf("got %+v, want %+v", got, want)
				}
				return
			}
			var verr *domain.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if !maps.Equal(verr.Fields, tt.wantFields) {
				t.Errorf("fields = %v, want %v", verr.Fields, tt.wantFields)
			}
		})
	}
}
