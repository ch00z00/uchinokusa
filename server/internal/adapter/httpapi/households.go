package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// HouseholdService は、世帯の作成・一覧・変更・削除を行う（実装は app.HouseholdService）。
type HouseholdService interface {
	Create(ctx context.Context, userID uuid.UUID, name string, prefecture, city *string) (domain.HouseholdWithRole, error)
	List(ctx context.Context, userID uuid.UUID) ([]domain.HouseholdWithRole, error)
	Update(ctx context.Context, userID, householdID uuid.UUID, name, prefecture, city *string) (domain.HouseholdWithRole, error)
	Delete(ctx context.Context, userID, householdID uuid.UUID) error
}

func (s *server) ListHouseholds(ctx context.Context, _ oapi.ListHouseholdsRequestObject) (oapi.ListHouseholdsResponseObject, error) {
	hs, err := s.households.List(ctx, currentUser(ctx).ID)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.Household, 0, len(hs)) // 0件でも null ではなく [] にする
	for _, h := range hs {
		out = append(out, toHousehold(h))
	}
	return oapi.ListHouseholds200JSONResponse{Households: out}, nil
}

func (s *server) CreateHousehold(ctx context.Context, req oapi.CreateHouseholdRequestObject) (oapi.CreateHouseholdResponseObject, error) {
	h, err := s.households.Create(ctx, currentUser(ctx).ID, req.Body.Name, req.Body.RegionPrefecture, req.Body.RegionCity)
	if err != nil {
		return nil, err
	}
	return oapi.CreateHousehold201JSONResponse(toHousehold(h)), nil
}

func (s *server) UpdateHousehold(ctx context.Context, req oapi.UpdateHouseholdRequestObject) (oapi.UpdateHouseholdResponseObject, error) {
	h, err := s.households.Update(ctx, currentUser(ctx).ID, req.HouseholdId, req.Body.Name, req.Body.RegionPrefecture, req.Body.RegionCity)
	if err != nil {
		return nil, err
	}
	return oapi.UpdateHousehold200JSONResponse(toHousehold(h)), nil
}

func (s *server) DeleteHousehold(ctx context.Context, req oapi.DeleteHouseholdRequestObject) (oapi.DeleteHouseholdResponseObject, error) {
	if err := s.households.Delete(ctx, currentUser(ctx).ID, req.HouseholdId); err != nil {
		return nil, err
	}
	return oapi.DeleteHousehold204Response{}, nil
}

func toHousehold(h domain.HouseholdWithRole) oapi.Household {
	return oapi.Household{
		Id:               h.ID,
		Name:             h.Name,
		Role:             oapi.Role(h.Role),
		RegionPrefecture: omitEmpty(h.Region.Prefecture), // 未設定なら項目ごと省く
		RegionCity:       omitEmpty(h.Region.City),
		TimeZone:         h.TimeZone,
		Hemisphere:       oapi.Hemisphere(h.Hemisphere),
		CreatedAt:        h.CreatedAt.UTC(),
	}
}

func omitEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
