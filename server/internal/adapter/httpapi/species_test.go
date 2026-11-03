package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

// fakeSpeciesService は、app.SpeciesService の偽物である。受け取った検索語を記録する。
type fakeSpeciesService struct {
	queries []string
}

func (f *fakeSpeciesService) Search(_ context.Context, q string) ([]domain.Species, error) {
	f.queries = append(f.queries, q)
	minTemp, outdoor := 5.0, true
	return []domain.Species{
		{ID: uuid.MustParse("01920000-0000-7000-8000-0000000000e1"), NameJa: "ガジュマル", ScientificName: "Ficus microcarpa",
			LightLevel: domain.LevelHigh, MinTempC: &minTemp, WaterTendency: domain.WaterModerate, OutdoorOK: &outdoor},
		{ID: uuid.MustParse("01920000-0000-7000-8000-0000000000e2"), NameJa: "フィカス・ウンベラータ"},
	}, nil
}

func TestSearchSpecies(t *testing.T) {
	t.Parallel()
	species := &fakeSpeciesService{}
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	router := httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Species: species})

	rec := serve(t, router, http.MethodGet, "/v1/species?q="+url.QueryEscape("がじゅ"), "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	if len(species.queries) != 1 || species.queries[0] != "がじゅ" {
		t.Errorf("queries = %v", species.queries)
	}
	// 性質がわからない項目は、null ではなく項目ごと省く。別名がなくても [] にする
	const want = `{"species":[` +
		`{"aliases":[],"id":"01920000-0000-7000-8000-0000000000e1","lightLevel":"high","minTempC":5,"nameJa":"ガジュマル","outdoorOk":true,"scientificName":"Ficus microcarpa","waterTendency":"moderate"},` +
		`{"aliases":[],"id":"01920000-0000-7000-8000-0000000000e2","nameJa":"フィカス・ウンベラータ"}` +
		"]}\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}

	// 検索語がなくても呼べる
	if rec := serve(t, router, http.MethodGet, "/v1/species", "Bearer "+validToken, ""); rec.Code != http.StatusOK || species.queries[1] != "" {
		t.Errorf("without q: status = %d, queries = %v", rec.Code, species.queries)
	}
}
