package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SpeciesRepository は、植物マスタ（species テーブル）へのアクセスを提供する。
type SpeciesRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewSpeciesRepository(pool *pgxpool.Pool) *SpeciesRepository {
	return &SpeciesRepository{pool: pool, q: sqlcgen.New(pool)}
}

// UpsertResult は、初期データの投入の結果である。
type UpsertResult struct {
	Inserted int
	Updated  int
}

// UpsertAll は、植物マスタを slug をキーに追加・上書きする。全体を1つのトランザクションで行う。
// 植物から参照されうるため、ファイルから消えた種類は削除しない。
func (r *SpeciesRepository) UpsertAll(ctx context.Context, list []domain.Species) (UpsertResult, error) {
	var res UpsertResult
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		for _, s := range list {
			params, err := toUpsertSpeciesParams(s)
			if err != nil {
				return err
			}
			inserted, err := q.UpsertSpecies(ctx, params)
			if err != nil {
				return fmt.Errorf("upsert species %q: %w", s.Slug, err)
			}
			if inserted {
				res.Inserted++
			} else {
				res.Updated++
			}
		}
		return nil
	})
	return res, err
}

// Search は、正規化した検索語（domain.NormalizeSearchText）を和名・学名・別名に含む種類を返す。
// 空の検索語なら、和名の順に limit 件まで返す。
func (r *SpeciesRepository) Search(ctx context.Context, normalizedQuery string, limit int) ([]domain.Species, error) {
	rows, err := r.q.SearchSpecies(ctx, sqlcgen.SearchSpeciesParams{
		Pattern:    "%" + escapeLike(normalizedQuery) + "%",
		Query:      normalizedQuery,
		MaxResults: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search species: %w", err)
	}
	out := make([]domain.Species, 0, len(rows))
	for _, row := range rows {
		s, err := toDomainSpecies(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// escapeLike は、LIKE で特別な意味を持つ文字（\ % _）を、文字そのものとして扱われるようにする。
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// recommendedCareJSON は、species.recommended_care（jsonb）の形である。
type recommendedCareJSON struct {
	Watering  *wateringJSON  `json:"watering,omitempty"`
	Repotting *repottingJSON `json:"repotting,omitempty"`
}

type wateringJSON struct {
	Condition string `json:"condition"`
	Winter    string `json:"winter,omitempty"`
}

type repottingJSON struct {
	SuitableMonths []int `json:"suitableMonths"`
}

func (c recommendedCareJSON) toDomain() domain.SpeciesCare {
	var care domain.SpeciesCare
	if w := c.Watering; w != nil {
		care.Watering = &domain.WateringAdvice{Condition: w.Condition, Winter: w.Winter}
	}
	if r := c.Repotting; r != nil {
		care.RepottingSuitableMonths = r.SuitableMonths
	}
	return care
}

// attributesJSON は、species.attributes（jsonb）の形である。MVP の画面で使わない項目もここに入れる（要件19-8）。
type attributesJSON struct {
	Sources []sourceJSON `json:"sources,omitempty"`
}

type sourceJSON struct {
	URL        string     `json:"url"`
	Title      string     `json:"title"`
	AccessedOn civil.Date `json:"accessedOn"`
	Fields     []string   `json:"fields"`
	Quote      string     `json:"quote,omitempty"`
}

func toUpsertSpeciesParams(s domain.Species) (sqlcgen.UpsertSpeciesParams, error) {
	var care recommendedCareJSON
	if w := s.Care.Watering; w != nil {
		care.Watering = &wateringJSON{Condition: w.Condition, Winter: w.Winter}
	}
	if m := s.Care.RepottingSuitableMonths; len(m) > 0 {
		care.Repotting = &repottingJSON{SuitableMonths: m}
	}
	careJSON, err := json.Marshal(care)
	if err != nil {
		return sqlcgen.UpsertSpeciesParams{}, fmt.Errorf("species %q: marshal care: %w", s.Slug, err)
	}

	var attrs attributesJSON
	for _, src := range s.Sources {
		attrs.Sources = append(attrs.Sources, sourceJSON{URL: src.URL, Title: src.Title, AccessedOn: src.AccessedOn, Fields: src.Fields, Quote: src.Quote})
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		return sqlcgen.UpsertSpeciesParams{}, fmt.Errorf("species %q: marshal attributes: %w", s.Slug, err)
	}

	aliases := s.Aliases
	if aliases == nil {
		aliases = []string{} // 列は NOT NULL
	}
	return sqlcgen.UpsertSpeciesParams{
		Slug:            s.Slug,
		NameJa:          s.NameJa,
		ScientificName:  nullIfEmpty(s.ScientificName),
		Aliases:         aliases,
		LightLevel:      nullIfEmpty(string(s.LightLevel)),
		MinTempC:        s.MinTempC,
		HumidityLevel:   nullIfEmpty(string(s.HumidityLevel)),
		AirflowNeed:     nullIfEmpty(string(s.AirflowNeed)),
		WaterTendency:   nullIfEmpty(string(s.WaterTendency)),
		OutdoorOk:       s.OutdoorOK,
		RecommendedCare: careJSON,
		Attributes:      attrsJSON,
		SearchText:      s.SearchText(),
	}, nil
}

func toDomainSpecies(row sqlcgen.Species) (domain.Species, error) {
	var care recommendedCareJSON
	if err := json.Unmarshal(row.RecommendedCare, &care); err != nil {
		return domain.Species{}, fmt.Errorf("species %q: unmarshal care: %w", row.Slug, err)
	}
	var attrs attributesJSON
	if err := json.Unmarshal(row.Attributes, &attrs); err != nil {
		return domain.Species{}, fmt.Errorf("species %q: unmarshal attributes: %w", row.Slug, err)
	}

	s := domain.Species{
		ID:             row.ID,
		Slug:           row.Slug,
		NameJa:         row.NameJa,
		ScientificName: emptyIfNull(row.ScientificName),
		Aliases:        row.Aliases,
		LightLevel:     domain.Level(emptyIfNull(row.LightLevel)),
		MinTempC:       row.MinTempC,
		HumidityLevel:  domain.Level(emptyIfNull(row.HumidityLevel)),
		AirflowNeed:    domain.Level(emptyIfNull(row.AirflowNeed)),
		WaterTendency:  domain.WaterTendency(emptyIfNull(row.WaterTendency)),
		OutdoorOK:      row.OutdoorOk,
	}
	s.Care = care.toDomain()
	for _, src := range attrs.Sources {
		s.Sources = append(s.Sources, domain.SpeciesSource{URL: src.URL, Title: src.Title, AccessedOn: src.AccessedOn, Fields: src.Fields, Quote: src.Quote})
	}
	return s, nil
}
