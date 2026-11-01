package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// HouseholdRepository は、世帯の保存先である（実装は adapter/postgres）。
type HouseholdRepository interface {
	// CreateWithOwner は、世帯と、ownerID のユーザーのオーナーのメンバーシップを、まとめて作る。
	CreateWithOwner(ctx context.Context, ownerID uuid.UUID, h domain.NewHousehold) (domain.Household, error)
	// ListByUser は、ユーザーが所属するすべての世帯を、そのユーザーのロールとともに返す。
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.HouseholdWithRole, error)
	// Update は、世帯を変更する。世帯がなければ domain.ErrNotFound を返す。
	Update(ctx context.Context, id uuid.UUID, p domain.HouseholdPatch) (domain.Household, error)
	// Delete は、世帯と配下のデータをすべて削除する。
	Delete(ctx context.Context, id uuid.UUID) error
}

// HouseholdService は、世帯の作成・一覧・変更・削除を扱う（要件2）。
type HouseholdService struct {
	households HouseholdRepository
	authz      *Authorizer
}

func NewHouseholdService(households HouseholdRepository, authz *Authorizer) *HouseholdService {
	return &HouseholdService{households: households, authz: authz}
}

// Create は、世帯を作り、作ったユーザーをオーナーにする（要件2-1）。
func (s *HouseholdService) Create(ctx context.Context, userID uuid.UUID, name string, prefecture, city *string) (domain.HouseholdWithRole, error) {
	in, err := domain.NewHouseholdInput(name, prefecture, city)
	if err != nil {
		return domain.HouseholdWithRole{}, err
	}
	h, err := s.households.CreateWithOwner(ctx, userID, in)
	if err != nil {
		return domain.HouseholdWithRole{}, err
	}
	return domain.HouseholdWithRole{Household: h, Role: domain.RoleOwner}, nil
}

// List は、ユーザーが所属するすべての世帯を返す（要件2-3）。
func (s *HouseholdService) List(ctx context.Context, userID uuid.UUID) ([]domain.HouseholdWithRole, error) {
	return s.households.ListByUser(ctx, userID)
}

// Update は、世帯名・地域を変更する。オーナーだけができる（要件2-5・7-5）。
// 権限の判定を入力の検証より先に行う。メンバーでない人に検証のエラーを返すと、世帯の存在がわかってしまうため。
func (s *HouseholdService) Update(ctx context.Context, userID, householdID uuid.UUID, name, prefecture, city *string) (domain.HouseholdWithRole, error) {
	m, err := s.authz.Require(ctx, userID, householdID, domain.ActionHouseholdManage)
	if err != nil {
		return domain.HouseholdWithRole{}, err
	}
	patch, err := domain.NewHouseholdPatch(name, prefecture, city)
	if err != nil {
		return domain.HouseholdWithRole{}, err
	}
	h, err := s.households.Update(ctx, householdID, patch)
	if err != nil {
		return domain.HouseholdWithRole{}, err
	}
	return domain.HouseholdWithRole{Household: h, Role: m.Role}, nil
}

// Delete は、世帯と配下のデータをすべて削除する。オーナーだけができる（要件2-6）。
func (s *HouseholdService) Delete(ctx context.Context, userID, householdID uuid.UUID) error {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionHouseholdManage); err != nil {
		return err
	}
	return s.households.Delete(ctx, householdID)
}
