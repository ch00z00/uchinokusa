// Package gcp は、Google Cloud のサービス（Cloud Scheduler の OIDC トークンなど）との連携を提供する。
package gcp

import (
	"context"
	"fmt"
	"slices"

	"google.golang.org/api/idtoken"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// JobVerifier は、Cloud Scheduler がジョブのエンドポイントを呼ぶときに付ける OIDC トークン（Google の ID トークン）を検証する。
type JobVerifier struct {
	validator      *idtoken.Validator
	audience       string
	serviceAccount string
}

// NewJobVerifier は、audience（Cloud Scheduler に設定する宛先）と、呼び出しを許すサービスアカウントのメールアドレスで、JobVerifier を作る。
func NewJobVerifier(ctx context.Context, audience, serviceAccount string, opts ...idtoken.ClientOption) (*JobVerifier, error) {
	v, err := idtoken.NewValidator(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &JobVerifier{validator: v, audience: audience, serviceAccount: serviceAccount}, nil
}

// googleIssuers は、Google の ID トークンの発行者（iss）である。
var googleIssuers = []string{"https://accounts.google.com", "accounts.google.com"}

// VerifyJob は、トークンが Google の発行した、この宛先あての、許したサービスアカウントのものかを確かめる。
// トークンとして正しくなければ domain.ErrUnauthenticated（401）、ほかのアカウントのものなら domain.ErrForbidden（403）を返す。
func (v *JobVerifier) VerifyJob(ctx context.Context, token string) error {
	// 署名（Google の公開鍵）・宛先（aud）・期限（exp）を確かめる
	p, err := v.validator.Validate(ctx, token, v.audience)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUnauthenticated, err)
	}
	// Validate は発行者を確かめないので、ここで確かめる
	if !slices.Contains(googleIssuers, p.Issuer) {
		return fmt.Errorf("%w: unexpected issuer %q", domain.ErrUnauthenticated, p.Issuer)
	}
	// Google のどのアカウントでも正しいトークンは作れるので、呼び出したアカウントを確かめる
	email, _ := p.Claims["email"].(string)
	verified, _ := p.Claims["email_verified"].(bool)
	if !verified || email != v.serviceAccount {
		return fmt.Errorf("%w: job called by %q", domain.ErrForbidden, email)
	}
	return nil
}
