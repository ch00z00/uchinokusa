package httpapi

import (
	"context"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// PlantService は、植物の登録・一覧・編集・削除・アーカイブを行う（実装は app.PlantService）。
type PlantService interface {
	List(ctx context.Context, userID, householdID uuid.UUID, q app.PlantQuery) ([]domain.Plant, error)
	Create(ctx context.Context, userID, householdID uuid.UUID, in domain.PlantInput) (domain.Plant, error)
	Get(ctx context.Context, userID, plantID uuid.UUID) (domain.Plant, error)
	Update(ctx context.Context, userID, plantID uuid.UUID, in domain.PlantInput) (domain.Plant, error)
	Delete(ctx context.Context, userID, plantID uuid.UUID) error
	Archive(ctx context.Context, userID, plantID uuid.UUID) (domain.Plant, error)
	Unarchive(ctx context.Context, userID, plantID uuid.UUID) (domain.Plant, error)
}

func (s *server) ListPlants(ctx context.Context, req oapi.ListPlantsRequestObject) (oapi.ListPlantsResponseObject, error) {
	// 送られなかった条件は、ゼロ値（絞り込まない・アーカイブしていない植物）にする
	var q app.PlantQuery
	params := req.Params
	if params.LocationId != nil {
		q.LocationID = *params.LocationId
	}
	if params.Q != nil {
		q.Q = *params.Q
	}
	if params.Archived != nil {
		q.Archived = *params.Archived
	}
	list, err := s.plants.List(ctx, currentUser(ctx).ID, req.HouseholdId, q)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.Plant, 0, len(list))
	for _, p := range list {
		out = append(out, toPlant(p))
	}
	return oapi.ListPlants200JSONResponse{Plants: out}, nil
}

func (s *server) CreatePlant(ctx context.Context, req oapi.CreatePlantRequestObject) (oapi.CreatePlantResponseObject, error) {
	b := req.Body
	p, err := s.plants.Create(ctx, currentUser(ctx).ID, req.HouseholdId, domain.PlantInput{
		Name: &b.Name, SpeciesID: b.SpeciesId, SpeciesFreeText: b.SpeciesFreeText, LocationID: b.LocationId, AcquiredOn: b.AcquiredOn,
	})
	if err != nil {
		return nil, err
	}
	return oapi.CreatePlant201JSONResponse(toPlant(p)), nil
}

func (s *server) GetPlant(ctx context.Context, req oapi.GetPlantRequestObject) (oapi.GetPlantResponseObject, error) {
	p, err := s.plants.Get(ctx, currentUser(ctx).ID, req.PlantId)
	if err != nil {
		return nil, err
	}
	return oapi.GetPlant200JSONResponse(toPlant(p)), nil
}

func (s *server) UpdatePlant(ctx context.Context, req oapi.UpdatePlantRequestObject) (oapi.UpdatePlantResponseObject, error) {
	b := req.Body
	p, err := s.plants.Update(ctx, currentUser(ctx).ID, req.PlantId, domain.PlantInput{
		Name: b.Name, SpeciesID: b.SpeciesId, SpeciesFreeText: b.SpeciesFreeText, LocationID: b.LocationId, AcquiredOn: b.AcquiredOn,
	})
	if err != nil {
		return nil, err
	}
	return oapi.UpdatePlant200JSONResponse(toPlant(p)), nil
}

func (s *server) DeletePlant(ctx context.Context, req oapi.DeletePlantRequestObject) (oapi.DeletePlantResponseObject, error) {
	if err := s.plants.Delete(ctx, currentUser(ctx).ID, req.PlantId); err != nil {
		return nil, err
	}
	return oapi.DeletePlant204Response{}, nil
}

func (s *server) ArchivePlant(ctx context.Context, req oapi.ArchivePlantRequestObject) (oapi.ArchivePlantResponseObject, error) {
	p, err := s.plants.Archive(ctx, currentUser(ctx).ID, req.PlantId)
	if err != nil {
		return nil, err
	}
	return oapi.ArchivePlant200JSONResponse(toPlant(p)), nil
}

func (s *server) UnarchivePlant(ctx context.Context, req oapi.UnarchivePlantRequestObject) (oapi.UnarchivePlantResponseObject, error) {
	p, err := s.plants.Unarchive(ctx, currentUser(ctx).ID, req.PlantId)
	if err != nil {
		return nil, err
	}
	return oapi.UnarchivePlant200JSONResponse(toPlant(p)), nil
}

// toPlant は、未設定の項目を、null ではなく項目ごと省いた形にする。
func toPlant(p domain.Plant) oapi.Plant {
	out := oapi.Plant{
		Id:              p.ID,
		Name:            p.Name,
		SpeciesId:       omitNilUUID(p.Species.ID),
		SpeciesName:     omitEmptyAs[string](p.SpeciesName),
		SpeciesFreeText: omitEmptyAs[string](p.Species.FreeText),
		LocationId:      omitNilUUID(p.LocationID),
		CreatedAt:       p.CreatedAt.UTC(),
	}
	if !p.AcquiredOn.IsZero() {
		out.AcquiredOn = &openapi_types.Date{Time: p.AcquiredOn.In(time.UTC)}
	}
	if !p.NextDueOn.IsZero() {
		out.NextDueOn = &openapi_types.Date{Time: p.NextDueOn.In(time.UTC)}
	}
	if p.ArchivedAt != nil {
		at := p.ArchivedAt.UTC()
		out.ArchivedAt = &at
	}
	return out
}

func omitNilUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
