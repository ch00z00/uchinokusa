package gcp_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"

	"google.golang.org/api/option"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/gcp"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

const (
	audience  = "https://api.uchinokusa.app"
	scheduler = "scheduler@uchinokusa.iam.gserviceaccount.com"
)

// googleKey は、Google の公開鍵の代わりに使う鍵である。
var googleKey, otherKey = mustKey(), mustKey()

func mustKey() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
}

// certsTransport は、Google の公開鍵の取得（https://www.googleapis.com/oauth2/v3/certs）に、googleKey を返す。
type certsTransport struct{}

func (certsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		return nil, errors.New("unexpected request " + r.URL.String())
	}
	b64 := base64.RawURLEncoding.EncodeToString
	body, _ := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kid": "k1", "kty": "RSA", "alg": "RS256", "use": "sig",
		"n": b64(googleKey.N.Bytes()), "e": b64(big.NewInt(int64(googleKey.E)).Bytes()),
	}}})
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
}

// sign は、claims を RS256 で署名した JWT を作る。
func sign(key *rsa.PrivateKey, claims map[string]any) string {
	b64 := base64.RawURLEncoding.EncodeToString
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	content := b64(header) + "." + b64(payload)
	sum := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		panic(err)
	}
	return content + "." + b64(sig)
}

// schedulerClaims は、Cloud Scheduler のサービスアカウントの、正しい ID トークンの中身である。
func schedulerClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": "https://accounts.google.com", "aud": audience, "sub": "1234567890",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"email": scheduler, "email_verified": true,
	}
}

func TestJobVerifier(t *testing.T) {
	t.Parallel()
	v, err := gcp.NewJobVerifier(t.Context(), audience, scheduler, option.WithHTTPClient(&http.Client{Transport: certsTransport{}}))
	if err != nil {
		t.Fatal(err)
	}
	with := func(k string, val any) string {
		c := schedulerClaims()
		if val == nil {
			delete(c, k)
		} else {
			c[k] = val
		}
		return sign(googleKey, c)
	}

	tests := []struct {
		name  string
		token string
		want  error
	}{
		{"Cloud Scheduler のサービスアカウントの正しいトークン", sign(googleKey, schedulerClaims()), nil},
		{"iss が accounts.google.com でもよい", with("iss", "accounts.google.com"), nil},
		{"宛先が違う", with("aud", "https://other.example.com"), domain.ErrUnauthenticated},
		{"期限切れ", with("exp", time.Now().Add(-time.Minute).Unix()), domain.ErrUnauthenticated},
		// idtoken.Validate は iss を確かめないので、自分で確かめる
		{"Google 以外が発行した", with("iss", "https://evil.example.com"), domain.ErrUnauthenticated},
		{"Google の鍵で署名されていない", sign(otherKey, schedulerClaims()), domain.ErrUnauthenticated},
		{"壊れたトークン", "not-a-jwt", domain.ErrUnauthenticated},
		{"ほかのサービスアカウント", with("email", "attacker@other.iam.gserviceaccount.com"), domain.ErrForbidden},
		{"メールアドレスが確かめられていない", with("email_verified", false), domain.ErrForbidden},
		{"メールアドレスがない", with("email", nil), domain.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := v.VerifyJob(t.Context(), tt.token)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
