package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// LocationService は、置き場所の一覧・作成・変更・削除を行う（実装は app.LocationService）。
type LocationService interface {
	List(ctx context.Context, userID, householdID uuid.UUID) ([]domain.Location, error)
	Create(ctx context.Context, userID, householdID uuid.UUID, in app.LocationInput) (domain.Location, error)
	Update(ctx context.Context, userID, locationID uuid.UUID, in app.LocationInput) (domain.Location, error)
	Delete(ctx context.Context, userID, locationID uuid.UUID) error
}

func (s *server) ListLocations(ctx context.Context, req oapi.ListLocationsRequestObject) (oapi.ListLocationsResponseObject, error) {
	list, err := s.locations.List(ctx, currentUser(ctx).ID, req.HouseholdId)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.Location, 0, len(list))
	for _, l := range list {
		out = append(out, toLocation(l))
	}
	return oapi.ListLocations200JSONResponse{Locations: out}, nil
}

func (s *server) CreateLocation(ctx context.Context, req oapi.CreateLocationRequestObject) (oapi.CreateLocationResponseObject, error) {
	b := req.Body
	l, err := s.locations.Create(ctx, currentUser(ctx).ID, req.HouseholdId, app.LocationInput{
		Name: &b.Name, IsIndoor: b.IsIndoor, WindowDirection: b.WindowDirection, WindowDistance: b.WindowDistance, Shading: b.Shading,
	})
	if err != nil {
		return nil, err
	}
	return oapi.CreateLocation201JSONResponse(toLocation(l)), nil
}

func (s *server) UpdateLocation(ctx context.Context, req oapi.UpdateLocationRequestObject) (oapi.UpdateLocationResponseObject, error) {
	b := req.Body
	l, err := s.locations.Update(ctx, currentUser(ctx).ID, req.LocationId, app.LocationInput{
		Name: b.Name, IsIndoor: b.IsIndoor, WindowDirection: b.WindowDirection, WindowDistance: b.WindowDistance, Shading: b.Shading,
	})
	if err != nil {
		return nil, err
	}
	return oapi.UpdateLocation200JSONResponse(toLocation(l)), nil
}

func (s *server) DeleteLocation(ctx context.Context, req oapi.DeleteLocationRequestObject) (oapi.DeleteLocationResponseObject, error) {
	if err := s.locations.Delete(ctx, currentUser(ctx).ID, req.LocationId); err != nil {
		return nil, err
	}
	return oapi.DeleteLocation204Response{}, nil
}

// toLocation は、未設定の環境の項目を、null ではなく項目ごと省いた形にする。
func toLocation(l domain.Location) oapi.Location {
	return oapi.Location{
		Id:              l.ID,
		Name:            l.Name,
		IsIndoor:        l.IsIndoor,
		WindowDirection: omitEmptyAs[oapi.WindowDirection](l.WindowDirection),
		WindowDistance:  omitEmptyAs[oapi.WindowDistance](l.WindowDistance),
		Shading:         omitEmptyAs[oapi.Shading](l.Shading),
		PlantCount:      l.PlantCount,
		CreatedAt:       l.CreatedAt.UTC(),
	}
}
