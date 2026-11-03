package httpapi

import (
	"context"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SpeciesService は、植物マスタの検索を行う（実装は app.SpeciesService）。
type SpeciesService interface {
	Search(ctx context.Context, q string) ([]domain.Species, error)
}

func (s *server) SearchSpecies(ctx context.Context, req oapi.SearchSpeciesRequestObject) (oapi.SearchSpeciesResponseObject, error) {
	var q string
	if req.Params.Q != nil {
		q = *req.Params.Q
	}
	list, err := s.species.Search(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.Species, 0, len(list))
	for _, sp := range list {
		out = append(out, toSpecies(sp))
	}
	return oapi.SearchSpecies200JSONResponse{Species: out}, nil
}

// toSpecies は、わからない（空の）性質を、null ではなく項目ごと省いた形にする。
func toSpecies(s domain.Species) oapi.Species {
	aliases := s.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return oapi.Species{
		Id:             s.ID,
		NameJa:         s.NameJa,
		ScientificName: omitEmpty(s.ScientificName),
		Aliases:        aliases,
		LightLevel:     omitEmptyAs[oapi.Level](s.LightLevel),
		MinTempC:       s.MinTempC,
		HumidityLevel:  omitEmptyAs[oapi.Level](s.HumidityLevel),
		AirflowNeed:    omitEmptyAs[oapi.Level](s.AirflowNeed),
		WaterTendency:  omitEmptyAs[oapi.WaterTendency](s.WaterTendency),
		OutdoorOk:      s.OutdoorOK,
	}
}

// omitEmptyAs は、空文字なら nil、そうでなければ API の型に変換した値へのポインタを返す。
func omitEmptyAs[T ~string, S ~string](v S) *T {
	if v == "" {
		return nil
	}
	t := T(v)
	return &t
}
