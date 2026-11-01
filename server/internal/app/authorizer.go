package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// MembershipRepository は、世帯のメンバーシップの保存先である（実装は adapter/postgres）。
type MembershipRepository interface {
	// Get は、ユーザーの世帯でのメンバーシップを返す。メンバーでなければ domain.ErrNotFound を返す。
	Get(ctx context.Context, householdID, userID uuid.UUID) (domain.Membership, error)
}

// Authorizer は、世帯のデータへの操作を許可するかを判定する（要件4・20-2）。
// 世帯のデータを扱うすべてのユースケースは、最初にこれを通す。
type Authorizer struct {
	memberships MembershipRepository
}

func NewAuthorizer(memberships MembershipRepository) *Authorizer {
	return &Authorizer{memberships: memberships}
}

// Require は、ユーザーが世帯のメンバーで、ロールが操作を許可することを確かめ、メンバーシップを返す。
// メンバーでなければ domain.ErrNotFound（存在を明かさない。要件4-4）、
// ロールが足りなければ domain.ErrForbidden（要件4-3）を返す。
func (a *Authorizer) Require(ctx context.Context, userID, householdID uuid.UUID, action domain.Action) (domain.Membership, error) {
	m, err := a.memberships.Get(ctx, householdID, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	if !m.Role.Allows(action) {
		return domain.Membership{}, domain.ErrForbidden
	}
	return m, nil
}
