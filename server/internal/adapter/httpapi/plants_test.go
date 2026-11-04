package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testPlant = domain.Plant{
	ID:          uuid.MustParse("01920000-0000-7000-8000-0000000000d1"),
	HouseholdID: testHousehold.ID,
	PlantProfile: domain.PlantProfile{
		Name:       "リビングのモンステラ",
		Species:    domain.PlantSpecies{ID: uuid.MustParse("01920000-0000-7000-8000-0000000000a1")},
		LocationID: testLocation.ID,
		AcquiredOn: civil.Date{Year: 2026, Month: 5, Day: 3},
	},
	SpeciesName: "モンステラ",
	NextDueOn:   civil.Date{Year: 2026, Month: 11, Day: 8},
	CreatedAt:   time.Date(2026, 11, 4, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// fakePlantService は、app.PlantService の偽物である。
// testPlant は testHousehold の植物で、ほかの世帯・植物は見つからないものとして振る舞う。受け取った入力を記録する。
type fakePlantService struct {
	queries  []app.PlantQuery
	inputs   []domain.PlantInput
	deleted  []uuid.UUID
	archived []bool
}

func (f *fakePlantService) List(_ context.Context, _, householdID uuid.UUID, q app.PlantQuery) ([]domain.Plant, error) {
	f.queries = append(f.queries, q)
	if householdID != testHousehold.ID {
		return nil, domain.ErrNotFound
	}
	return []domain.Plant{testPlant}, nil
}

func (f *fakePlantService) Create(_ context.Context, _, householdID uuid.UUID, in domain.PlantInput) (domain.Plant, error) {
	f.inputs = append(f.inputs, in)
	if householdID != testHousehold.ID {
		return domain.Plant{}, domain.ErrNotFound
	}
	p, err := domain.NewPlantProfile(in)
	if err != nil {
		return domain.Plant{}, err
	}
	return domain.Plant{ID: testPlant.ID, HouseholdID: householdID, PlantProfile: p, CreatedAt: testPlant.CreatedAt}, nil
}

func (f *fakePlantService) Get(_ context.Context, _, plantID uuid.UUID) (domain.Plant, error) {
	if plantID != testPlant.ID {
		return domain.Plant{}, domain.ErrNotFound
	}
	return testPlant, nil
}

func (f *fakePlantService) Update(_ context.Context, _, plantID uuid.UUID, in domain.PlantInput) (domain.Plant, error) {
	f.inputs = append(f.inputs, in)
	if plantID != testPlant.ID {
		return domain.Plant{}, domain.ErrNotFound
	}
	return testPlant, nil
}

func (f *fakePlantService) Delete(_ context.Context, _, plantID uuid.UUID) error {
	if plantID != testPlant.ID {
		return domain.ErrForbidden
	}
	f.deleted = append(f.deleted, plantID)
	return nil
}

func (f *fakePlantService) Archive(_ context.Context, _, plantID uuid.UUID) (domain.Plant, error) {
	return f.setArchived(plantID, true)
}

func (f *fakePlantService) Unarchive(_ context.Context, _, plantID uuid.UUID) (domain.Plant, error) {
	return f.setArchived(plantID, false)
}

func (f *fakePlantService) setArchived(plantID uuid.UUID, archived bool) (domain.Plant, error) {
	if plantID != testPlant.ID {
		return domain.Plant{}, domain.ErrForbidden
	}
	f.archived = append(f.archived, archived)
	p := testPlant
	if archived {
		at := time.Date(2026, 11, 5, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
		p.ArchivedAt = &at
	}
	return p, nil
}

func newPlantRouter(t *testing.T, plants *fakePlantService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Plants: plants})
}

// testPlantJSON は、testPlant のレスポンスである。
const testPlantJSON = `{"acquiredOn":"2026-05-03","createdAt":"2026-11-04T00:00:00Z","id":"01920000-0000-7000-8000-0000000000d1",` +
	`"locationId":"01920000-0000-7000-8000-0000000000c1","name":"リビングのモンステラ","nextDueOn":"2026-11-08",` +
	`"speciesId":"01920000-0000-7000-8000-0000000000a1","speciesName":"モンステラ"}`

func TestListPlants(t *testing.T) {
	t.Parallel()

	t.Run("次のお手入れ予定日つきで返す", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodGet, "/v1/households/"+testHousehold.ID.String()+"/plants", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got, want := rec.Body.String(), `{"plants":[`+testPlantJSON+`]}`+"\n"; got != want {
			t.Errorf("body =\n%s\nwant\n%s", got, want)
		}
		// 条件を送らなければ、アーカイブしていない植物のすべて
		if len(plants.queries) != 1 || plants.queries[0] != (app.PlantQuery{}) {
			t.Errorf("queries = %+v", plants.queries)
		}
	})

	t.Run("置き場所・名前・アーカイブの条件を渡す", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodGet,
			"/v1/households/"+testHousehold.ID.String()+"/plants?locationId="+testLocation.ID.String()+"&q=%E3%82%82%E3%82%93&archived=true", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if want := (app.PlantQuery{LocationID: testLocation.ID, Q: "もん", Archived: true}); len(plants.queries) != 1 || plants.queries[0] != want {
			t.Errorf("queries = %+v, want %+v", plants.queries, want)
		}
	})

	t.Run("置き場所の ID が UUID でなければ 400", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodGet, "/v1/households/"+testHousehold.ID.String()+"/plants?locationId=x", "Bearer "+validToken, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("所属していない世帯は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodGet, "/v1/households/"+uuid.Must(uuid.NewV7()).String()+"/plants", "Bearer "+validToken, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestCreatePlant(t *testing.T) {
	t.Parallel()

	t.Run("登録した植物を 201 で返し、未設定の項目は省く", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodPost, "/v1/households/"+testHousehold.ID.String()+"/plants", "Bearer "+validToken,
			`{"name": "多肉", "speciesFreeText": "多肉植物"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		const want = `{"createdAt":"2026-11-04T00:00:00Z","id":"01920000-0000-7000-8000-0000000000d1","name":"多肉","speciesFreeText":"多肉植物"}` + "\n"
		if got := rec.Body.String(); got != want {
			t.Errorf("body =\n%s\nwant\n%s", got, want)
		}
		// 送った項目は値を、送らなかった項目は nil をサービスに渡す
		in := plants.inputs[0]
		if in.Name == nil || *in.Name != "多肉" || in.SpeciesFreeText == nil || in.SpeciesID != nil || in.LocationID != nil || in.AcquiredOn != nil {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("入力の誤りは 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodPost, "/v1/households/"+testHousehold.ID.String()+"/plants", "Bearer "+validToken,
			`{"name": "", "acquiredOn": "2026-02-30"}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rec.Code)
		}
		if got := decode[errorBody](t, rec); got.Fields["name"] != "required" || got.Fields["acquiredOn"] != "invalid" {
			t.Errorf("fields = %v", got.Fields)
		}
	})
}

func TestGetPlant(t *testing.T) {
	t.Parallel()

	t.Run("植物を返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodGet, "/v1/plants/"+testPlant.ID.String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); got != testPlantJSON+"\n" {
			t.Errorf("body =\n%s\nwant\n%s", got, testPlantJSON)
		}
	})

	t.Run("見つからない植物は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodGet, "/v1/plants/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestUpdatePlant(t *testing.T) {
	t.Parallel()

	t.Run("空文字は、未設定に戻す指示としてそのまま渡す", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodPatch, "/v1/plants/"+testPlant.ID.String(), "Bearer "+validToken, `{"locationId": "", "acquiredOn": ""}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		in := plants.inputs[0]
		if in.LocationID == nil || *in.LocationID != "" || in.AcquiredOn == nil || *in.AcquiredOn != "" || in.Name != nil || in.SpeciesID != nil || in.SpeciesFreeText != nil {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("見つからない植物は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodPatch, "/v1/plants/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, `{"name": "x"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestDeletePlant(t *testing.T) {
	t.Parallel()

	t.Run("削除したら 204", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodDelete, "/v1/plants/"+testPlant.ID.String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if len(plants.deleted) != 1 {
			t.Errorf("deleted = %v", plants.deleted)
		}
	})

	t.Run("権限がなければ 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodDelete, "/v1/plants/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}

func TestArchiveAndUnarchivePlant(t *testing.T) {
	t.Parallel()

	t.Run("アーカイブしたら、アーカイブした日時を UTC で返す", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodPost, "/v1/plants/"+testPlant.ID.String()+"/archive", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := decode[struct{ ArchivedAt string }](t, rec); got.ArchivedAt != "2026-11-05T00:00:00Z" {
			t.Errorf("archivedAt = %q", got.ArchivedAt)
		}
		if len(plants.archived) != 1 || !plants.archived[0] {
			t.Errorf("archived calls = %v", plants.archived)
		}
	})

	t.Run("元に戻したら、アーカイブの日時は項目ごとない", func(t *testing.T) {
		t.Parallel()
		plants := &fakePlantService{}
		rec := serve(t, newPlantRouter(t, plants), http.MethodPost, "/v1/plants/"+testPlant.ID.String()+"/unarchive", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); got != testPlantJSON+"\n" {
			t.Errorf("body =\n%s\nwant\n%s", got, testPlantJSON)
		}
		if len(plants.archived) != 1 || plants.archived[0] {
			t.Errorf("archived calls = %v", plants.archived)
		}
	})

	t.Run("権限がなければ 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newPlantRouter(t, &fakePlantService{}), http.MethodPost, "/v1/plants/"+uuid.Must(uuid.NewV7()).String()+"/archive", "Bearer "+validToken, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}
