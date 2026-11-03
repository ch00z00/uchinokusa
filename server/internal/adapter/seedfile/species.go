// Package seedfile は、初期データのファイル（db/seeds/）を読み込み、ドメインの型にして検証する。
package seedfile

import (
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/civil"
	"go.yaml.in/yaml/v3"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// speciesYAML は、species.yaml の1種類の形である。項目名はファイルの見やすさを優先してスネークケースにする。
type speciesYAML struct {
	Slug                    string        `yaml:"slug"`
	NameJa                  string        `yaml:"name_ja"`
	ScientificName          string        `yaml:"scientific_name"`
	Aliases                 []string      `yaml:"aliases"`
	LightLevel              string        `yaml:"light_level"`
	MinTempC                *float64      `yaml:"min_temp_c"`
	HumidityLevel           string        `yaml:"humidity_level"`
	AirflowNeed             string        `yaml:"airflow_need"`
	WaterTendency           string        `yaml:"water_tendency"`
	OutdoorOK               *bool         `yaml:"outdoor_ok"`
	Watering                *wateringYAML `yaml:"watering"`
	RepottingSuitableMonths []int         `yaml:"repotting_suitable_months"`
	Sources                 []sourceYAML  `yaml:"sources"`
}

type wateringYAML struct {
	Condition string `yaml:"condition"`
	Winter    string `yaml:"winter"`
}

type sourceYAML struct {
	URL        string   `yaml:"url"`
	Title      string   `yaml:"title"`
	AccessedOn string   `yaml:"accessed_on"`
	Fields     []string `yaml:"fields"`
	Quote      string   `yaml:"quote"`
}

// ParseSpecies は、植物マスタの初期データを読み込み、検証する。1つでも誤りがあれば、すべての誤りをまとめて返す。
func ParseSpecies(r io.Reader) ([]domain.Species, error) {
	dec := yaml.NewDecoder(r)
	// 知らない項目（書き間違い）をエラーにする。無視すると、出典の決まりの検証をすり抜けてしまう
	dec.KnownFields(true)
	var rows []speciesYAML
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("parse species: %w", err)
	}

	list := make([]domain.Species, 0, len(rows))
	var errs []error
	for _, row := range rows {
		s, err := row.toDomain()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		list = append(list, s)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if err := domain.ValidateSpeciesList(list); err != nil {
		return nil, err
	}
	return list, nil
}

func (y speciesYAML) toDomain() (domain.Species, error) {
	s := domain.Species{
		Slug:           y.Slug,
		NameJa:         y.NameJa,
		ScientificName: y.ScientificName,
		Aliases:        y.Aliases,
		LightLevel:     domain.Level(y.LightLevel),
		MinTempC:       y.MinTempC,
		HumidityLevel:  domain.Level(y.HumidityLevel),
		AirflowNeed:    domain.Level(y.AirflowNeed),
		WaterTendency:  domain.WaterTendency(y.WaterTendency),
		OutdoorOK:      y.OutdoorOK,
		Care:           domain.SpeciesCare{RepottingSuitableMonths: y.RepottingSuitableMonths},
	}
	if w := y.Watering; w != nil {
		s.Care.Watering = &domain.WateringAdvice{Condition: w.Condition, Winter: w.Winter}
	}
	for i, src := range y.Sources {
		d, err := civil.ParseDate(src.AccessedOn)
		if err != nil {
			return domain.Species{}, fmt.Errorf("species %q: sources[%d].accessed_on %q must be YYYY-MM-DD", y.Slug, i, src.AccessedOn)
		}
		s.Sources = append(s.Sources, domain.SpeciesSource{URL: src.URL, Title: src.Title, AccessedOn: d, Fields: src.Fields, Quote: src.Quote})
	}
	return s, nil
}
