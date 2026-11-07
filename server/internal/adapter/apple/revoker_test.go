package apple_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/apple"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

var now = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// fakeApple は、Apple の /auth/token と /auth/revoke の代わりである。認可コード "good-code" だけを受け付ける。
type fakeApple struct {
	t        *testing.T
	key      *ecdsa.PrivateKey
	mu       sync.Mutex
	revoked  []url.Values
	tokenErr int // 0 でなければ、/auth/token をこのステータスで失敗させる
}

func (f *fakeApple) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.Method != http.MethodPost {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if r.Form.Get("client_id") != "app.uchinokusa" || !f.validSecret(r.Form.Get("client_secret")) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
		return
	}
	switch r.URL.Path {
	case "/auth/token":
		if f.tokenErr != 0 {
			w.WriteHeader(f.tokenErr)
			return
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "good-code" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-1", "refresh_token": "rt-1", "token_type": "Bearer", "expires_in": 3600})
	case "/auth/revoke":
		f.mu.Lock()
		f.revoked = append(f.revoked, r.Form)
		f.mu.Unlock()
	default:
		http.NotFound(w, r)
	}
}

// validSecret は、クライアントシークレットが、鍵で ES256 署名した、決まりどおりの JWT かを確かめる。
func (f *fakeApple) validSecret(secret string) bool {
	token, err := jwt.Parse(secret, func(tok *jwt.Token) (any, error) {
		if tok.Method != jwt.SigningMethodES256 || tok.Header["kid"] != "KEY123" {
			return nil, errors.New("unexpected header")
		}
		return &f.key.PublicKey, nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		f.t.Logf("client secret: %v", err)
		return false
	}
	c := token.Claims.(jwt.MapClaims)
	iat, _ := c["iat"].(float64)
	exp, _ := c["exp"].(float64)
	return c["iss"] == "TEAM123" && c["sub"] == "app.uchinokusa" && c["aud"] == "https://appleid.apple.com" &&
		int64(iat) == now.Unix() && int64(exp) > now.Unix() && int64(exp) <= now.Add(6*30*24*time.Hour).Unix()
}

func newRevoker(t *testing.T) (*apple.Revoker, *fakeApple) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeApple{t: t, key: key}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	r := apple.NewRevoker(apple.Config{TeamID: "TEAM123", KeyID: "KEY123", ClientID: "app.uchinokusa", PrivateKey: key}, srv.Client(), func() time.Time { return now })
	r.BaseURL = srv.URL
	return r, fake
}

func TestRevoker_Revoke(t *testing.T) {
	t.Parallel()

	t.Run("認可コードでトークンを受け取り、リフレッシュトークンを失効させる", func(t *testing.T) {
		t.Parallel()
		r, fake := newRevoker(t)
		if err := r.Revoke(t.Context(), "good-code"); err != nil {
			t.Fatal(err)
		}
		if len(fake.revoked) != 1 || fake.revoked[0].Get("token") != "rt-1" || fake.revoked[0].Get("token_type_hint") != "refresh_token" {
			t.Errorf("revoked = %v", fake.revoked)
		}
	})

	t.Run("認可コードが無効なら、入力の誤りにする（アプリが Apple で再認証し直す）", func(t *testing.T) {
		t.Parallel()
		r, fake := newRevoker(t)
		err := r.Revoke(t.Context(), "expired-code")
		var v *domain.ValidationError
		if !errors.As(err, &v) || v.Fields["appleAuthorizationCode"] != "invalid" {
			t.Errorf("err = %v", err)
		}
		if len(fake.revoked) != 0 {
			t.Errorf("revoked = %v", fake.revoked)
		}
	})

	t.Run("Apple が失敗したら、エラーを返す", func(t *testing.T) {
		t.Parallel()
		r, fake := newRevoker(t)
		fake.tokenErr = http.StatusServiceUnavailable
		err := r.Revoke(t.Context(), "good-code")
		var v *domain.ValidationError
		if err == nil || errors.As(err, &v) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestParsePrivateKey(t *testing.T) {
	t.Parallel()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	// Apple の .p8 は、PKCS #8 の PEM
	got, err := apple.ParsePrivateKey(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if err != nil || got == nil || !got.Equal(key) {
		t.Errorf("key = %v, err = %v", got, err)
	}
	if _, err := apple.ParsePrivateKey([]byte("not a key")); err == nil {
		t.Error("err = nil for broken key")
	}
}
