package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testLocation = domain.Location{
	ID:          uuid.MustParse("01920000-0000-7000-8000-0000000000c1"),
	HouseholdID: testHousehold.ID,
	LocationProfile: domain.LocationProfile{
		Name: "リビングの南窓", IsIndoor: true, WindowDirection: domain.DirectionSouth, Shading: domain.ShadingSheer,
	},
	PlantCount: 3,
	CreatedAt:  time.Date(2026, 11, 4, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// fakeLocationService は、app.LocationService の偽物である。
// testLocation は testHousehold の置き場所で、ほかの世帯・置き場所は見つからないものとして振る舞う。受け取った入力を記録する。
type fakeLocationService struct {
	inputs  []app.LocationInput
	deleted []uuid.UUID
}

func (f *fakeLocationService) List(_ context.Context, _, householdID uuid.UUID) ([]domain.Location, error) {
	if householdID != testHousehold.ID {
		return nil, domain.ErrNotFound
	}
	return []domain.Location{testLocation}, nil
}

func (f *fakeLocationService) Create(_ context.Context, _, householdID uuid.UUID, in app.LocationInput) (domain.Location, error) {
	f.inputs = append(f.inputs, in)
	if householdID != testHousehold.ID {
		return domain.Location{}, domain.ErrNotFound
	}
	if in.Name == nil || *in.Name == "" {
		return domain.Location{}, &domain.ValidationError{Fields: map[string]string{"name": "required"}}
	}
	l := testLocation
	l.LocationProfile, l.PlantCount = domain.LocationProfile{Name: *in.Name, IsIndoor: true}, 0
	return l, nil
}

func (f *fakeLocationService) Update(_ context.Context, _, locationID uuid.UUID, in app.LocationInput) (domain.Location, error) {
	f.inputs = append(f.inputs, in)
	if locationID != testLocation.ID {
		return domain.Location{}, domain.ErrNotFound
	}
	return testLocation, nil
}

func (f *fakeLocationService) Delete(_ context.Context, _, locationID uuid.UUID) error {
	if locationID != testLocation.ID {
		return domain.ErrForbidden
	}
	f.deleted = append(f.deleted, locationID)
	return nil
}

func newLocationRouter(t *testing.T, locations *fakeLocationService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Locations: locations})
}

func TestListLocations(t *testing.T) {
	t.Parallel()

	t.Run("植物の数つきで返し、未設定の環境の項目は省く", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newLocationRouter(t, &fakeLocationService{}), http.MethodGet, "/v1/households/"+testHousehold.ID.String()+"/locations", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		const want = `{"locations":[{"createdAt":"2026-11-04T00:00:00Z","id":"01920000-0000-7000-8000-0000000000c1","isIndoor":true,` +
			`"name":"リビングの南窓","plantCount":3,"shading":"sheer","windowDirection":"s"}]}` + "\n"
		if got := rec.Body.String(); got != want {
			t.Errorf("body =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("所属していない世帯は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newLocationRouter(t, &fakeLocationService{}), http.MethodGet, "/v1/households/"+uuid.Must(uuid.NewV7()).String()+"/locations", "Bearer "+validToken, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestCreateLocation(t *testing.T) {
	t.Parallel()

	t.Run("作成した置き場所を 201 で返す", func(t *testing.T) {
		t.Parallel()
		locations := &fakeLocationService{}
		rec := serve(t, newLocationRouter(t, locations), http.MethodPost, "/v1/households/"+testHousehold.ID.String()+"/locations", "Bearer "+validToken,
			`{"name": "寝室", "isIndoor": true, "windowDirection": "e"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Name       string
			PlantCount int
		}](t, rec)
		if got.Name != "寝室" || got.PlantCount != 0 {
			t.Errorf("body = %+v", got)
		}
		// 送った項目は値を、送らなかった項目は nil をサービスに渡す
		in := locations.inputs[0]
		if in.IsIndoor == nil || !*in.IsIndoor || in.WindowDirection == nil || *in.WindowDirection != "e" || in.WindowDistance != nil || in.Shading != nil {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("入力の誤りは 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newLocationRouter(t, &fakeLocationService{}), http.MethodPost, "/v1/households/"+testHousehold.ID.String()+"/locations", "Bearer "+validToken, `{"name": ""}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rec.Code)
		}
		if got := decode[errorBody](t, rec); got.Fields["name"] != "required" {
			t.Errorf("fields = %v", got.Fields)
		}
	})
}

func TestUpdateLocation(t *testing.T) {
	t.Parallel()

	t.Run("空文字は、未設定に戻す指示としてそのまま渡す", func(t *testing.T) {
		t.Parallel()
		locations := &fakeLocationService{}
		rec := serve(t, newLocationRouter(t, locations), http.MethodPatch, "/v1/locations/"+testLocation.ID.String(), "Bearer "+validToken, `{"shading": ""}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		in := locations.inputs[0]
		if in.Shading == nil || *in.Shading != "" || in.Name != nil || in.IsIndoor != nil {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("見つからない置き場所は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newLocationRouter(t, &fakeLocationService{}), http.MethodPatch, "/v1/locations/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, `{"name": "x"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestDeleteLocation(t *testing.T) {
	t.Parallel()

	t.Run("削除したら 204", func(t *testing.T) {
		t.Parallel()
		locations := &fakeLocationService{}
		rec := serve(t, newLocationRouter(t, locations), http.MethodDelete, "/v1/locations/"+testLocation.ID.String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if len(locations.deleted) != 1 {
			t.Errorf("deleted = %v", locations.deleted)
		}
	})

	t.Run("権限がなければ 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newLocationRouter(t, &fakeLocationService{}), http.MethodDelete, "/v1/locations/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}
