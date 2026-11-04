package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeLocations は、app.LocationRepository の偽物である。置き場所をメモリに持ち、呼ばれた操作を記録する。
type fakeLocations struct {
	byID    map[uuid.UUID]domain.Location
	patched []domain.LocationPatch
	deleted []uuid.UUID
}

func (f *fakeLocations) List(_ context.Context, householdID uuid.UUID) ([]domain.Location, error) {
	out := []domain.Location{}
	for _, l := range f.byID {
		if l.HouseholdID == householdID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeLocations) Create(_ context.Context, householdID uuid.UUID, p domain.LocationProfile) (domain.Location, error) {
	l := domain.Location{ID: uuid.Must(uuid.NewV7()), HouseholdID: householdID, LocationProfile: p}
	f.byID[l.ID] = l
	return l, nil
}

func (f *fakeLocations) HouseholdOf(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	l, ok := f.byID[id]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return l.HouseholdID, nil
}

func (f *fakeLocations) Update(_ context.Context, householdID, id uuid.UUID, p domain.LocationPatch) (domain.Location, error) {
	l, ok := f.byID[id]
	if !ok || l.HouseholdID != householdID {
		return domain.Location{}, domain.ErrNotFound
	}
	f.patched = append(f.patched, p)
	return l, nil
}

func (f *fakeLocations) Delete(_ context.Context, householdID, id uuid.UUID) error {
	l, ok := f.byID[id]
	if !ok || l.HouseholdID != householdID {
		return domain.ErrNotFound
	}
	delete(f.byID, id)
	f.deleted = append(f.deleted, id)
	return nil
}

// locationFixture は、householdFixture の世帯とメンバーに、置き場所を1つ加えた状態を用意する。
type locationFixture struct {
	householdFixture
	location  uuid.UUID
	locations *fakeLocations
	svc       *app.LocationService
}

func newLocationFixture() locationFixture {
	hf := newHouseholdFixture()
	f := locationFixture{householdFixture: hf, location: uuid.Must(uuid.NewV7()), locations: &fakeLocations{byID: map[uuid.UUID]domain.Location{}}}
	f.locations.byID[f.location] = domain.Location{ID: f.location, HouseholdID: hf.household, LocationProfile: domain.LocationProfile{Name: "リビング", IsIndoor: true}}
	f.svc = app.NewLocationService(f.locations, hf.authz)
	return f
}

func TestLocationService_List(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者も一覧を見られる", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		got, err := f.svc.List(t.Context(), f.viewer, f.household)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "リビング" {
			t.Errorf("locations = %+v", got)
		}
	})

	t.Run("メンバーでなければ 404 にあたるエラー", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if _, err := f.svc.List(t.Context(), f.stranger, f.household); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestLocationService_Create(t *testing.T) {
	t.Parallel()

	t.Run("編集者は作成できる", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		got, err := f.svc.Create(t.Context(), f.editor, f.household, app.LocationInput{Name: ptr(" 南の窓 "), WindowDirection: ptr("s")})
		if err != nil {
			t.Fatal(err)
		}
		if got.HouseholdID != f.household || got.Name != "南の窓" || !got.IsIndoor || got.WindowDirection != domain.DirectionSouth {
			t.Errorf("location = %+v", got)
		}
	})

	t.Run("名前を送らなければ required", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		_, err := f.svc.Create(t.Context(), f.editor, f.household, app.LocationInput{})
		assertFields(t, err, map[string]string{"name": "required"})
	})

	t.Run("閲覧者は作成できない", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if _, err := f.svc.Create(t.Context(), f.viewer, f.household, app.LocationInput{Name: ptr("玄関")}); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("メンバーでない人には、入力の誤りより先に 404 を返す", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if _, err := f.svc.Create(t.Context(), f.stranger, f.household, app.LocationInput{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestLocationService_Update(t *testing.T) {
	t.Parallel()

	t.Run("編集者は変更できる", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if _, err := f.svc.Update(t.Context(), f.editor, f.location, app.LocationInput{Shading: ptr("sheer")}); err != nil {
			t.Fatal(err)
		}
		if len(f.locations.patched) != 1 || f.locations.patched[0].Shading == nil || *f.locations.patched[0].Shading != domain.ShadingSheer {
			t.Errorf("patched = %+v", f.locations.patched)
		}
	})

	tests := []struct {
		name    string
		user    func(locationFixture) uuid.UUID
		id      func(locationFixture) uuid.UUID
		in      app.LocationInput
		wantErr error
	}{
		{name: "閲覧者は変更できない", user: func(f locationFixture) uuid.UUID { return f.viewer }, id: func(f locationFixture) uuid.UUID { return f.location }, wantErr: domain.ErrForbidden},
		// 置き場所の ID を知っていても、世帯のメンバーでなければ存在を明かさない
		{name: "ほかの世帯の置き場所は 404", user: func(f locationFixture) uuid.UUID { return f.stranger }, id: func(f locationFixture) uuid.UUID { return f.location }, wantErr: domain.ErrNotFound},
		{name: "存在しない置き場所は 404", user: func(f locationFixture) uuid.UUID { return f.owner }, id: func(locationFixture) uuid.UUID { return uuid.Must(uuid.NewV7()) }, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newLocationFixture()
			if _, err := f.svc.Update(t.Context(), tt.user(f), tt.id(f), tt.in); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.locations.patched) != 0 {
				t.Errorf("patched = %+v, want none", f.locations.patched)
			}
		})
	}

	t.Run("入力の誤りは 422 にあたるエラー", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		_, err := f.svc.Update(t.Context(), f.owner, f.location, app.LocationInput{Name: ptr(""), WindowDistance: ptr("1m")})
		assertFields(t, err, map[string]string{"name": "required", "windowDistance": "invalid"})
	})
}

func TestLocationService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("編集者は削除できる", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if err := f.svc.Delete(t.Context(), f.editor, f.location); err != nil {
			t.Fatal(err)
		}
		if len(f.locations.deleted) != 1 {
			t.Errorf("deleted = %v", f.locations.deleted)
		}
	})

	t.Run("閲覧者は削除できない", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if err := f.svc.Delete(t.Context(), f.viewer, f.location); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("ほかの世帯の置き場所は 404", func(t *testing.T) {
		t.Parallel()
		f := newLocationFixture()
		if err := f.svc.Delete(t.Context(), f.stranger, f.location); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
