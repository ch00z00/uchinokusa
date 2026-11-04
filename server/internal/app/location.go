package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// LocationRepository は、置き場所の保存先である（実装は adapter/postgres）。
type LocationRepository interface {
	// List は、世帯のすべての置き場所を、植物の数とともに返す。
	List(ctx context.Context, householdID uuid.UUID) ([]domain.Location, error)
	Create(ctx context.Context, householdID uuid.UUID, p domain.LocationProfile) (domain.Location, error)
	// HouseholdOf は、置き場所が属する世帯の ID を返す。置き場所がなければ domain.ErrNotFound を返す。
	HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// Update は、世帯の置き場所を変更する。世帯の置き場所でなければ domain.ErrNotFound を返す。
	Update(ctx context.Context, householdID, id uuid.UUID, p domain.LocationPatch) (domain.Location, error)
	// Delete は、世帯の置き場所を削除し、紐づく植物を「置き場所未設定」にする。世帯の置き場所でなければ domain.ErrNotFound を返す。
	Delete(ctx context.Context, householdID, id uuid.UUID) error
}

// LocationInput は、置き場所の作成・変更で受け取った、検証前の内容である。nil は送られなかった項目を表す。
type LocationInput struct {
	Name            *string
	IsIndoor        *bool
	WindowDirection *string
	WindowDistance  *string
	Shading         *string
}

// LocationService は、置き場所の一覧・作成・変更・削除を扱う（要件7）。
type LocationService struct {
	locations LocationRepository
	authz     *Authorizer
}

func NewLocationService(locations LocationRepository, authz *Authorizer) *LocationService {
	return &LocationService{locations: locations, authz: authz}
}

// List は、世帯の置き場所を返す。閲覧者も見られる。
func (s *LocationService) List(ctx context.Context, userID, householdID uuid.UUID) ([]domain.Location, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRead); err != nil {
		return nil, err
	}
	return s.locations.List(ctx, householdID)
}

// Create は、置き場所を作る。編集者以上ができる（要件7-1・7-2）。
func (s *LocationService) Create(ctx context.Context, userID, householdID uuid.UUID, in LocationInput) (domain.Location, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionPlantWrite); err != nil {
		return domain.Location{}, err
	}
	var name string
	if in.Name != nil {
		name = *in.Name
	}
	p, err := domain.NewLocationInput(name, in.IsIndoor, in.WindowDirection, in.WindowDistance, in.Shading)
	if err != nil {
		return domain.Location{}, err
	}
	return s.locations.Create(ctx, householdID, p)
}

// Update は、置き場所を変更する。編集者以上ができる（要件7-3）。
// 植物は置き場所を ID で参照しているので、変更はそのまま紐づくすべての植物に反映される。
func (s *LocationService) Update(ctx context.Context, userID, locationID uuid.UUID, in LocationInput) (domain.Location, error) {
	householdID, err := s.authorize(ctx, userID, locationID)
	if err != nil {
		return domain.Location{}, err
	}
	p, err := domain.NewLocationPatch(in.Name, in.IsIndoor, in.WindowDirection, in.WindowDistance, in.Shading)
	if err != nil {
		return domain.Location{}, err
	}
	return s.locations.Update(ctx, householdID, locationID, p)
}

// Delete は、置き場所を削除し、紐づく植物を「置き場所未設定」にする。編集者以上ができる（要件7-4）。
// 紐づく植物の数の確認は、アプリが一覧の plantCount で行ってから呼ぶ。
func (s *LocationService) Delete(ctx context.Context, userID, locationID uuid.UUID) error {
	householdID, err := s.authorize(ctx, userID, locationID)
	if err != nil {
		return err
	}
	return s.locations.Delete(ctx, householdID, locationID)
}

// authorize は、置き場所の ID から世帯を探し、ユーザーがその世帯で置き場所を変更できることを確かめる。
// 置き場所がない場合と、所属していない世帯の置き場所の場合は、どちらも domain.ErrNotFound にする（存在を明かさない）。
func (s *LocationService) authorize(ctx context.Context, userID, locationID uuid.UUID) (uuid.UUID, error) {
	householdID, err := s.locations.HouseholdOf(ctx, locationID)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionPlantWrite); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}
