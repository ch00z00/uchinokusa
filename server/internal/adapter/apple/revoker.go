// Package apple は、Sign in with Apple の REST API（トークンの失効）を提供する。
package apple

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// Config は、Apple Developer で発行した、Sign in with Apple のための鍵とアプリの情報である。
type Config struct {
	TeamID     string            // チームの ID
	KeyID      string            // 鍵の ID
	ClientID   string            // アプリの Bundle ID（ネイティブアプリのサインインでは、これがクライアントの ID になる）
	PrivateKey *ecdsa.PrivateKey // 鍵（.p8）
}

// Revoker は、アカウントの削除で、Sign in with Apple のトークンを失効させる（App Store の審査の要件）。
type Revoker struct {
	cfg Config
	// BaseURL は、Apple の API の URL である（テストで差し替える）。
	BaseURL string
	client  *http.Client
	now     func() time.Time
}

func NewRevoker(cfg Config, client *http.Client, now func() time.Time) *Revoker {
	return &Revoker{cfg: cfg, BaseURL: "https://appleid.apple.com", client: client, now: now}
}

// ParsePrivateKey は、.p8 ファイルの中身（PKCS #8 の PEM）を鍵にする。
func ParsePrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	key, err := jwt.ParseECPrivateKeyFromPEM(pemData)
	if err != nil {
		return nil, fmt.Errorf("apple: parse private key: %w", err)
	}
	return key, nil
}

// Revoke は、アプリが削除の直前に Apple で受け取った認可コードで、トークンを受け取り、失効させる。
// 認可コードが無効（期限切れ・使用済み）なら、appleAuthorizationCode の入力の誤り（422）を返す。
// 認可コードの有効期限は5分で、1回しか使えないので、アプリは削除のたびに Apple で再認証する。
func (r *Revoker) Revoke(ctx context.Context, authorizationCode string) error {
	secret, err := r.clientSecret()
	if err != nil {
		return err
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	err = r.post(ctx, "/auth/token", url.Values{
		"client_id": {r.cfg.ClientID}, "client_secret": {secret},
		"code": {authorizationCode}, "grant_type": {"authorization_code"},
	}, &token)
	var ae *apiError
	if errors.As(err, &ae) && ae.Code == "invalid_grant" {
		return &domain.ValidationError{Fields: map[string]string{"appleAuthorizationCode": "invalid"}}
	}
	if err != nil {
		return err
	}

	// リフレッシュトークンを失効させると、アプリとの連携そのものが切れる（アクセストークンだけでは切れない）
	revoke, hint := token.RefreshToken, "refresh_token"
	if revoke == "" {
		revoke, hint = token.AccessToken, "access_token"
	}
	return r.post(ctx, "/auth/revoke", url.Values{
		"client_id": {r.cfg.ClientID}, "client_secret": {secret},
		"token": {revoke}, "token_type_hint": {hint},
	}, nil)
}

// clientSecret は、Apple の API に渡すクライアントシークレット（鍵で ES256 署名した JWT）を作る。有効期限は5分にする。
func (r *Revoker) clientSecret() (string, error) {
	now := r.now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": r.cfg.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
		"aud": "https://appleid.apple.com",
		"sub": r.cfg.ClientID,
	})
	token.Header["kid"] = r.cfg.KeyID
	s, err := token.SignedString(r.cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("apple: sign client secret: %w", err)
	}
	return s, nil
}

// apiError は、Apple の API が返したエラー（{"error": "invalid_grant"} など）である。
type apiError struct {
	Status int
	Code   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("apple: status %d: %s", e.Status, e.Code)
}

func (r *Revoker) post(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("apple: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("apple: %s: %w", path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("apple: read %s: %w", path, err)
	}
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e) // 本文がなくても、ステータスでエラーにする
		return &apiError{Status: res.StatusCode, Code: e.Error}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("apple: decode %s: %w", path, err)
	}
	return nil
}
