// Package firebase は、Firebase Admin SDK を使った ID トークンの検証と、FCM の送信を提供する。
package firebase

import (
	"context"
	"fmt"

	fb "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// Verifier は、Firebase Auth の ID トークンを検証する。
// 環境変数 FIREBASE_AUTH_EMULATOR_HOST があると、Emulator が発行したトークンを検証する。
type Verifier struct {
	client *auth.Client
}

// NewVerifier は、Verifier を作る。credentialsFile が空なら、実行環境の認証情報（Cloud Run のサービスアカウントなど）を使う。
func NewVerifier(ctx context.Context, projectID, credentialsFile string) (*Verifier, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	app, err := fb.NewApp(ctx, &fb.Config{ProjectID: projectID}, opts...)
	if err != nil {
		return nil, fmt.Errorf("firebase: new app: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: auth client: %w", err)
	}
	return &Verifier{client: client}, nil
}

// Verify は、ID トークンを検証し、ユーザーの情報を返す。
// トークンが壊れている・期限切れ・別のプロジェクトのものなら、domain.ErrUnauthenticated を返す。
func (v *Verifier) Verify(ctx context.Context, idToken string) (domain.Identity, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		if auth.IsCertificateFetchFailed(err) {
			// 公開鍵を取得できないのは、トークンではなくサーバー側の問題なので 500 にする
			return domain.Identity{}, fmt.Errorf("firebase: verify id token: %w", err)
		}
		return domain.Identity{}, fmt.Errorf("%w: %v", domain.ErrUnauthenticated, err)
	}

	name, _ := token.Claims["name"].(string)
	return domain.Identity{
		UID:      token.UID,
		Name:     name,
		Provider: token.Firebase.SignInProvider,
	}, nil
}
