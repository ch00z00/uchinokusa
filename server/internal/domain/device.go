package domain

import (
	"time"

	"github.com/google/uuid"
)

// Platform は、端末の OS の種類である。
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Device は、プッシュ通知の送信先となる端末である。1つの FCM トークンは、1人のユーザーにだけ属する。
type Device struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	FCMToken   string
	Platform   Platform
	TimeZone   string // 端末の IANA タイムゾーン。リマインダーの日付と時刻の判定に使う（要件15-7）
	LastSeenAt time.Time
}

// DeviceRegistration は、検証済みの端末の登録内容である。
type DeviceRegistration struct {
	FCMToken string
	Platform Platform
	TimeZone string
}

// MaxFCMTokenLength は、FCM トークンの最大の長さ（バイト数）である。
// 実際のトークンは数百バイトだが、形式は公開されていないため、余裕をもたせて不正に大きな入力だけを防ぐ。
const MaxFCMTokenLength = 4096

// NewDeviceRegistration は、端末の登録内容を検証する。誤りがあれば、すべての項目の理由をまとめて返す。
func NewDeviceRegistration(fcmToken, platform, timeZone string) (DeviceRegistration, error) {
	fields := map[string]string{}

	switch {
	case fcmToken == "":
		fields["fcmToken"] = "required"
	case len(fcmToken) > MaxFCMTokenLength:
		fields["fcmToken"] = "too_long"
	}

	p := Platform(platform)
	if p != PlatformIOS && p != PlatformAndroid {
		fields["platform"] = "invalid"
	}

	switch {
	case timeZone == "":
		fields["timeZone"] = "required"
	case !isIANATimeZone(timeZone):
		fields["timeZone"] = "invalid"
	}

	if len(fields) > 0 {
		return DeviceRegistration{}, &ValidationError{Fields: fields}
	}
	return DeviceRegistration{FCMToken: fcmToken, Platform: p, TimeZone: timeZone}, nil
}

// isIANATimeZone は、名前が IANA のタイムゾーンとして読み込めるかを返す。
// time.LoadLocation は "Local"（サーバーのタイムゾーン）も受け付けるため、除く。
func isIANATimeZone(name string) bool {
	if name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}
