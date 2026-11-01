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
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testHousehold = domain.HouseholdWithRole{
	Household: domain.Household{
		ID:         uuid.MustParse("01920000-0000-7000-8000-0000000000a1"),
		Name:       "自宅",
		Region:     domain.Region{Prefecture: "東京都"},
		TimeZone:   "Asia/Tokyo",
		Hemisphere: domain.HemisphereNorth,
		CreatedAt:  time.Date(2026, 11, 1, 10, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
	},
	Role: domain.RoleOwner,
}

// fakeHouseholdService は、app.HouseholdService の偽物である。
// testHousehold のオーナーは testUser だけで、ほかの世帯は見つからないものとして振る舞う。
type fakeHouseholdService struct {
	list    []domain.HouseholdWithRole
	deleted []uuid.UUID
}

func (f *fakeHouseholdService) Create(_ context.Context, _ uuid.UUID, name string, prefecture, city *string) (domain.HouseholdWithRole, error) {
	in, err := domain.NewHouseholdInput(name, prefecture, city)
	if err != nil {
		return domain.HouseholdWithRole{}, err
	}
	h := testHousehold
	h.Name, h.Region = in.Name, in.Region
	return h, nil
}

func (f *fakeHouseholdService) List(_ context.Context, _ uuid.UUID) ([]domain.HouseholdWithRole, error) {
	return f.list, nil
}

func (f *fakeHouseholdService) Update(_ context.Context, userID, householdID uuid.UUID, name, _, _ *string) (domain.HouseholdWithRole, error) {
	if userID != testUser.ID || householdID != testHousehold.ID {
		return domain.HouseholdWithRole{}, domain.ErrNotFound
	}
	h := testHousehold
	if name != nil {
		h.Name = *name
	}
	return h, nil
}

func (f *fakeHouseholdService) Delete(_ context.Context, userID, householdID uuid.UUID) error {
	if userID != testUser.ID || householdID != testHousehold.ID {
		return domain.ErrNotFound
	}
	f.deleted = append(f.deleted, householdID)
	return nil
}

func newHouseholdRouter(t *testing.T, households *fakeHouseholdService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Households: households})
}

type householdBody struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Role             string  `json:"role"`
	RegionPrefecture *string `json:"regionPrefecture"`
	RegionCity       *string `json:"regionCity"`
	TimeZone         string  `json:"timeZone"`
	Hemisphere       string  `json:"hemisphere"`
	CreatedAt        string  `json:"createdAt"`
}

func TestCreateHousehold(t *testing.T) {
	t.Parallel()

	t.Run("作成した世帯を 201 で返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{}), http.MethodPost, "/v1/households", "Bearer "+validToken,
			`{"name": "実家", "regionPrefecture": "長野県"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
		}
		got := decode[householdBody](t, rec)
		if got.Name != "実家" || got.Role != "owner" || got.RegionPrefecture == nil || *got.RegionPrefecture != "長野県" {
			t.Errorf("body = %+v", got)
		}
		// 未設定の地域は、項目ごと省く（null や空文字にしない）
		if got.RegionCity != nil {
			t.Errorf("regionCity = %q, want omitted", *got.RegionCity)
		}
		if got.CreatedAt != "2026-11-01T01:00:00Z" {
			t.Errorf("createdAt = %s, want UTC", got.CreatedAt)
		}
	})

	t.Run("名前が空なら 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{}), http.MethodPost, "/v1/households", "Bearer "+validToken, `{"name": ""}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rec.Code)
		}
		if got := decode[errorBody](t, rec); got.Fields["name"] != "required" {
			t.Errorf("fields = %v", got.Fields)
		}
	})
}

func TestListHouseholds(t *testing.T) {
	t.Parallel()

	t.Run("所属する世帯をロールとともに返す", func(t *testing.T) {
		t.Parallel()
		parents := testHousehold
		parents.ID, parents.Name, parents.Role, parents.Region = uuid.Must(uuid.NewV7()), "実家", domain.RoleViewer, domain.Region{}
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{list: []domain.HouseholdWithRole{testHousehold, parents}}),
			http.MethodGet, "/v1/households", "Bearer "+validToken, "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := decode[struct{ Households []householdBody }](t, rec).Households
		if len(got) != 2 || got[0].Role != "owner" || got[1].Name != "実家" || got[1].Role != "viewer" {
			t.Errorf("households = %+v", got)
		}
	})

	t.Run("どこにも所属していなければ、空の配列（null ではない）", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{list: []domain.HouseholdWithRole{}}), http.MethodGet, "/v1/households", "Bearer "+validToken, "")
		if body := rec.Body.String(); body != "{\"households\":[]}\n" {
			t.Errorf("body = %q", body)
		}
	})
}

func TestUpdateHousehold(t *testing.T) {
	t.Parallel()

	t.Run("変更後の世帯を返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{}), http.MethodPatch, "/v1/households/"+testHousehold.ID.String(), "Bearer "+validToken, `{"name": "わが家"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if got := decode[householdBody](t, rec); got.Name != "わが家" {
			t.Errorf("name = %s", got.Name)
		}
	})

	t.Run("所属していない世帯は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{}), http.MethodPatch, "/v1/households/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, `{"name": "x"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestDeleteHousehold(t *testing.T) {
	t.Parallel()

	t.Run("削除したら 204", func(t *testing.T) {
		t.Parallel()
		households := &fakeHouseholdService{}
		rec := serve(t, newHouseholdRouter(t, households), http.MethodDelete, "/v1/households/"+testHousehold.ID.String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204 (%s)", rec.Code, rec.Body.String())
		}
		if len(households.deleted) != 1 {
			t.Errorf("deleted = %v", households.deleted)
		}
	})

	t.Run("ID が UUID でなければ 400", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newHouseholdRouter(t, &fakeHouseholdService{}), http.MethodDelete, "/v1/households/home", "Bearer "+validToken, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}
