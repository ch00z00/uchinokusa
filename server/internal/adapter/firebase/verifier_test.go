package firebase_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/firebase"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/firebase/fbtest"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func newVerifier(t *testing.T) *firebase.Verifier {
	t.Helper()
	fbtest.Start(t)
	v, err := firebase.NewVerifier(t.Context(), fbtest.ProjectID, "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

func TestVerifier_ValidToken(t *testing.T) {
	t.Parallel()
	v := newVerifier(t)
	token, uid := fbtest.SignUp(t, "valid@example.com", "ゆうすけ")

	got, err := v.Verify(t.Context(), token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UID != uid {
		t.Errorf("UID = %q, want %q", got.UID, uid)
	}
	if got.Name != "ゆうすけ" {
		t.Errorf("Name = %q, want ゆうすけ", got.Name)
	}
	if got.Provider != "password" { // Emulator ではメールアドレスとパスワードで作る
		t.Errorf("Provider = %q, want password", got.Provider)
	}
}

// トークンが壊れている・期限切れ・別のプロジェクトのものは、認証エラーにする（要件1-3）。
func TestVerifier_InvalidTokens(t *testing.T) {
	t.Parallel()
	v := newVerifier(t)
	_, uid := fbtest.SignUp(t, "invalid@example.com", "")
	now := time.Now()

	tests := []struct {
		name   string
		token  string
		reason string // エラーの文言に含まれるはずの語。想定どおりの理由で拒否されたことを確かめる
	}{
		{name: "JWT の形式ではない", token: "not-a-jwt", reason: "segments"},
		{name: "期限切れ", token: unsignedToken(uid, fbtest.ProjectID, now.Add(-2*time.Hour), now.Add(-time.Hour)), reason: "expired"},
		{name: "別のプロジェクト向け", token: unsignedToken(uid, "demo-other", now, now.Add(time.Hour)), reason: "audience"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := v.Verify(t.Context(), tt.token)
			if !errors.Is(err, domain.ErrUnauthenticated) {
				t.Fatalf("err = %v, want ErrUnauthenticated", err)
			}
			if !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("err = %v, want the reason %q", err, tt.reason)
			}
		})
	}
}

// 期限と対象が正しければ、自作のトークンも通る。
// これにより、上のテストのトークンが「形式の誤り」ではなく「期限・対象の違い」で拒否されていることを保証する。
func TestVerifier_CraftedTokenWithValidClaims(t *testing.T) {
	t.Parallel()
	v := newVerifier(t)
	_, uid := fbtest.SignUp(t, "crafted@example.com", "")
	now := time.Now()

	// 発行時刻はユーザーを作ったあとにする（作る前に発行されたトークンは、失効したものとして扱われる）
	got, err := v.Verify(t.Context(), unsignedToken(uid, fbtest.ProjectID, now, now.Add(time.Hour)))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UID != uid {
		t.Errorf("UID = %q, want %q", got.UID, uid)
	}
}

// unsignedToken は、Emulator が発行するのと同じ、署名のない ID トークンを作る。
// Admin SDK は Emulator に接続しているとき署名を確かめないが、期限・発行者・対象のプロジェクトと、
// uid のユーザーが Emulator に存在することは確かめる。
func unsignedToken(uid, projectID string, issuedAt, expiresAt time.Time) string {
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := enc(map[string]string{"alg": "none", "typ": "JWT"})
	claims := enc(map[string]any{
		"iss":       "https://securetoken.google.com/" + projectID,
		"aud":       projectID,
		"sub":       uid,
		"user_id":   uid,
		"iat":       issuedAt.Unix(),
		"auth_time": issuedAt.Unix(),
		"exp":       expiresAt.Unix(),
		"firebase":  map[string]any{"sign_in_provider": "password", "identities": map[string]any{}},
	})
	return strings.Join([]string{header, claims, ""}, ".")
}
