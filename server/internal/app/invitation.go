package app

import (
	"context"
	"errors"
	"net/url"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// ErrInvitationCodeTaken は、招待コードがほかの招待と重なって保存できなかったことを表す。
var ErrInvitationCodeTaken = errors.New("invitation code already taken")

// InvitationRepository は、招待の保存先である（実装は adapter/postgres）。
type InvitationRepository interface {
	// Create は、招待を保存する。コードがほかの招待と重なれば ErrInvitationCodeTaken を返す。
	Create(ctx context.Context, householdID, createdBy uuid.UUID, role domain.Role, s domain.InvitationSecret) (domain.Invitation, error)
	// ListActive は、取り消されておらず期限内の招待を、新しい順に返す。
	ListActive(ctx context.Context, householdID uuid.UUID) ([]domain.Invitation, error)
	// Revoke は、招待を取り消す。世帯の招待でなければ domain.ErrNotFound を返す。取り消し済みでもエラーにしない。
	Revoke(ctx context.Context, householdID, invitationID uuid.UUID) error
}

// CreatedInvitation は、作成した招待と、招待リンクの URL である。
// URL にはトークンが入るが、トークンは保存しないので、URL を返せるのは作成時だけである。
type CreatedInvitation struct {
	domain.Invitation
	URL string
}

// maxCodeAttempts は、招待コードが重なったときに作り直す回数の上限である。
// 31^8（約8500億）通りのコードが重なることはまれなので、少ない回数で十分である。
const maxCodeAttempts = 3

// InvitationService は、招待の発行・一覧・取り消しを扱う（要件3-1・3-7）。
type InvitationService struct {
	invitations InvitationRepository
	authz       *Authorizer
	baseURL     *url.URL // 招待リンクのドメイン（例：https://invite.example.com）
}

func NewInvitationService(invitations InvitationRepository, authz *Authorizer, baseURL *url.URL) *InvitationService {
	return &InvitationService{invitations: invitations, authz: authz, baseURL: baseURL}
}

// Create は、編集者か閲覧者のロールを付与する招待を作る。オーナーだけができる。
func (s *InvitationService) Create(ctx context.Context, actorID, householdID uuid.UUID, role string) (CreatedInvitation, error) {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionInviteManage); err != nil {
		return CreatedInvitation{}, err
	}
	r, err := domain.ParseAssignableRole(role) // オーナーは招待で付与しない（docs/decisions/0012）
	if err != nil {
		return CreatedInvitation{}, err
	}

	for range maxCodeAttempts {
		secret, err := domain.NewInvitationSecret()
		if err != nil {
			return CreatedInvitation{}, err
		}
		inv, err := s.invitations.Create(ctx, householdID, actorID, r, secret)
		if errors.Is(err, ErrInvitationCodeTaken) {
			continue
		}
		if err != nil {
			return CreatedInvitation{}, err
		}
		return CreatedInvitation{Invitation: inv, URL: s.baseURL.JoinPath("i", secret.Token).String()}, nil
	}
	return CreatedInvitation{}, ErrInvitationCodeTaken
}

// List は、有効な招待の一覧を返す。オーナーだけができる。
func (s *InvitationService) List(ctx context.Context, actorID, householdID uuid.UUID) ([]domain.Invitation, error) {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionInviteManage); err != nil {
		return nil, err
	}
	return s.invitations.ListActive(ctx, householdID)
}

// Revoke は、招待を取り消し、ただちに使えなくする（要件3-7）。オーナーだけができる。
func (s *InvitationService) Revoke(ctx context.Context, actorID, householdID, invitationID uuid.UUID) error {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionInviteManage); err != nil {
		return err
	}
	return s.invitations.Revoke(ctx, householdID, invitationID)
}
