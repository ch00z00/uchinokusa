package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// User は、アプリのユーザーである。Firebase Auth のユーザーと1対1で対応する。
type User struct {
	ID           uuid.UUID
	FirebaseUID  string
	DisplayName  string
	AuthProvider string
	CreatedAt    time.Time
}

// Identity は、認証プロバイダ（Firebase Auth）が確認したユーザーの情報である。
type Identity struct {
	UID      string // Firebase の uid
	Name     string // プロバイダから受け取った名前。受け取れない場合は空（Apple は ID トークンに名前を含めない）
	Provider string // サインインの方法（apple.com、google.com など）
}

// MaxDisplayNameLength は、表示名の最大の文字数である。通知の文面やメンバーの一覧に収まる長さにする。
const MaxDisplayNameLength = 30

// NormalizeDisplayName は、表示名の前後の空白を除き、長さを検証する。
func NormalizeDisplayName(s string) (string, error) {
	name := strings.TrimSpace(s) // 全角の空白（U+3000）も除かれる
	switch {
	case name == "":
		return "", &ValidationError{Fields: map[string]string{"displayName": "required"}}
	case utf8.RuneCountInString(name) > MaxDisplayNameLength:
		return "", &ValidationError{Fields: map[string]string{"displayName": "too_long"}}
	}
	return name, nil
}
