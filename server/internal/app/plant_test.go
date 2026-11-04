package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakePlants は、app.PlantRepository の偽物である。植物をメモリに持ち、呼ばれた操作を記録する。
type fakePlants struct {
	byID     map[uuid.UUID]domain.Plant
	filters  []domain.PlantFilter
	created  []domain.PlantProfile
	patched  []domain.PlantPatch
	deleted  []uuid.UUID
	archived []bool
}

func (f *fakePlants) List(_ context.Context, householdID uuid.UUID, filter domain.PlantFilter) ([]domain.Plant, error) {
	f.filters = append(f.filters, filter)
	out := []domain.Plant{}
	for _, p := range f.byID {
		if p.HouseholdID == householdID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakePlants) Create(_ context.Context, householdID uuid.UUID, p domain.PlantProfile) (domain.Plant, error) {
	f.created = append(f.created, p)
	plant := domain.Plant{ID: uuid.Must(uuid.NewV7()), HouseholdID: householdID, PlantProfile: p}
	f.byID[plant.ID] = plant
	return plant, nil
}

func (f *fakePlants) HouseholdOf(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	p, ok := f.byID[id]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return p.HouseholdID, nil
}

func (f *fakePlants) find(householdID, id uuid.UUID) (domain.Plant, error) {
	p, ok := f.byID[id]
	if !ok || p.HouseholdID != householdID {
		return domain.Plant{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakePlants) Get(_ context.Context, householdID, id uuid.UUID) (domain.Plant, error) {
	return f.find(householdID, id)
}

func (f *fakePlants) Update(_ context.Context, householdID, id uuid.UUID, p domain.PlantPatch) (domain.Plant, error) {
	f.patched = append(f.patched, p)
	return f.find(householdID, id)
}

func (f *fakePlants) Delete(_ context.Context, householdID, id uuid.UUID) error {
	if _, err := f.find(householdID, id); err != nil {
		return err
	}
	delete(f.byID, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakePlants) SetArchived(_ context.Context, householdID, id uuid.UUID, archived bool) (domain.Plant, error) {
	p, err := f.find(householdID, id)
	if err != nil {
		return domain.Plant{}, err
	}
	f.archived = append(f.archived, archived)
	p.ArchivedAt = nil
	if archived {
		now := time.Now()
		p.ArchivedAt = &now
	}
	f.byID[id] = p
	return p, nil
}

// plantFixture は、householdFixture の世帯とメンバーに、植物を1つ加えた状態を用意する。
type plantFixture struct {
	householdFixture
	plant  uuid.UUID
	plants *fakePlants
	svc    *app.PlantService
}

func newPlantFixture() plantFixture {
	hf := newHouseholdFixture()
	f := plantFixture{householdFixture: hf, plant: uuid.Must(uuid.NewV7()), plants: &fakePlants{byID: map[uuid.UUID]domain.Plant{}}}
	f.plants.byID[f.plant] = domain.Plant{ID: f.plant, HouseholdID: hf.household, PlantProfile: domain.PlantProfile{Name: "モンステラ"}}
	f.svc = app.NewPlantService(f.plants, hf.authz)
	return f
}

func TestPlantService_List(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者も一覧を見られる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		got, err := f.svc.List(t.Context(), f.viewer, f.household, app.PlantQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "モンステラ" {
			t.Errorf("plants = %+v", got)
		}
	})

	t.Run("名前の検索語は、植物マスタの検索と同じく表記の揺れをそろえて渡す", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		location := uuid.Must(uuid.NewV7())
		if _, err := f.svc.List(t.Context(), f.viewer, f.household, app.PlantQuery{LocationID: location, Q: "もん すてら", Archived: true}); err != nil {
			t.Fatal(err)
		}
		want := domain.PlantFilter{LocationID: location, Query: "モンステラ", Archived: true}
		if len(f.plants.filters) != 1 || f.plants.filters[0] != want {
			t.Errorf("filters = %+v, want %+v", f.plants.filters, want)
		}
	})

	t.Run("検索語が長すぎれば 422 にあたるエラー", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		_, err := f.svc.List(t.Context(), f.viewer, f.household, app.PlantQuery{Q: strings.Repeat("あ", 101)})
		assertFields(t, err, map[string]string{"q": "too_long"})
	})

	t.Run("メンバーでなければ 404 にあたるエラー", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if _, err := f.svc.List(t.Context(), f.stranger, f.household, app.PlantQuery{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestPlantService_Create(t *testing.T) {
	t.Parallel()

	t.Run("編集者は登録できる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		got, err := f.svc.Create(t.Context(), f.editor, f.household, domain.PlantInput{Name: ptr(" ポトス "), SpeciesFreeText: ptr("ポトス")})
		if err != nil {
			t.Fatal(err)
		}
		if got.HouseholdID != f.household || got.Name != "ポトス" || got.Species.FreeText != "ポトス" {
			t.Errorf("plant = %+v", got)
		}
	})

	t.Run("入力の誤りは 422 にあたるエラーで、保存しない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		_, err := f.svc.Create(t.Context(), f.editor, f.household, domain.PlantInput{AcquiredOn: ptr("x")})
		assertFields(t, err, map[string]string{"name": "required", "acquiredOn": "invalid"})
		if len(f.plants.created) != 0 {
			t.Errorf("created = %+v", f.plants.created)
		}
	})

	t.Run("閲覧者は登録できない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if _, err := f.svc.Create(t.Context(), f.viewer, f.household, domain.PlantInput{Name: ptr("ポトス")}); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("メンバーでない人には、入力の誤りより先に 404 を返す", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if _, err := f.svc.Create(t.Context(), f.stranger, f.household, domain.PlantInput{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestPlantService_Get(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者も詳細を見られる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		got, err := f.svc.Get(t.Context(), f.viewer, f.plant)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != f.plant {
			t.Errorf("plant = %+v", got)
		}
	})

	t.Run("ほかの世帯の植物は 404", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if _, err := f.svc.Get(t.Context(), f.stranger, f.plant); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

// plantCase は、植物の ID だけで呼ぶ操作（変更・削除・アーカイブ）の、権限の確かめ方の共通のケースである。
type plantCase struct {
	name    string
	user    func(plantFixture) uuid.UUID
	id      func(plantFixture) uuid.UUID
	wantErr error
}

var plantWriteDenied = []plantCase{
	{name: "閲覧者はできない", user: func(f plantFixture) uuid.UUID { return f.viewer }, id: func(f plantFixture) uuid.UUID { return f.plant }, wantErr: domain.ErrForbidden},
	// 植物の ID を知っていても、世帯のメンバーでなければ存在を明かさない
	{name: "ほかの世帯の植物は 404", user: func(f plantFixture) uuid.UUID { return f.stranger }, id: func(f plantFixture) uuid.UUID { return f.plant }, wantErr: domain.ErrNotFound},
	{name: "存在しない植物は 404", user: func(f plantFixture) uuid.UUID { return f.owner }, id: func(plantFixture) uuid.UUID { return uuid.Must(uuid.NewV7()) }, wantErr: domain.ErrNotFound},
}

func TestPlantService_Update(t *testing.T) {
	t.Parallel()

	t.Run("編集者は変更できる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if _, err := f.svc.Update(t.Context(), f.editor, f.plant, domain.PlantInput{LocationID: ptr("")}); err != nil {
			t.Fatal(err)
		}
		if len(f.plants.patched) != 1 || f.plants.patched[0].LocationID == nil || *f.plants.patched[0].LocationID != uuid.Nil {
			t.Errorf("patched = %+v", f.plants.patched)
		}
	})

	for _, tt := range plantWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newPlantFixture()
			if _, err := f.svc.Update(t.Context(), tt.user(f), tt.id(f), domain.PlantInput{Name: ptr("x")}); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.plants.patched) != 0 {
				t.Errorf("patched = %+v, want none", f.plants.patched)
			}
		})
	}

	t.Run("入力の誤りは 422 にあたるエラー", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		_, err := f.svc.Update(t.Context(), f.owner, f.plant, domain.PlantInput{Name: ptr(""), SpeciesID: ptr("x")})
		assertFields(t, err, map[string]string{"name": "required", "speciesId": "invalid"})
	})
}

func TestPlantService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("編集者は削除できる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if err := f.svc.Delete(t.Context(), f.editor, f.plant); err != nil {
			t.Fatal(err)
		}
		if len(f.plants.deleted) != 1 {
			t.Errorf("deleted = %v", f.plants.deleted)
		}
	})

	for _, tt := range plantWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newPlantFixture()
			if err := f.svc.Delete(t.Context(), tt.user(f), tt.id(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.plants.deleted) != 0 {
				t.Errorf("deleted = %v, want none", f.plants.deleted)
			}
		})
	}
}

func TestPlantService_ArchiveAndUnarchive(t *testing.T) {
	t.Parallel()

	t.Run("編集者はアーカイブでき、元に戻せる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		archived, err := f.svc.Archive(t.Context(), f.editor, f.plant)
		if err != nil {
			t.Fatal(err)
		}
		if archived.ArchivedAt == nil {
			t.Errorf("archived = %+v", archived)
		}
		restored, err := f.svc.Unarchive(t.Context(), f.editor, f.plant)
		if err != nil {
			t.Fatal(err)
		}
		if restored.ArchivedAt != nil {
			t.Errorf("restored = %+v", restored)
		}
		if want := []bool{true, false}; len(f.plants.archived) != 2 || f.plants.archived[0] != want[0] || f.plants.archived[1] != want[1] {
			t.Errorf("archived calls = %v, want %v", f.plants.archived, want)
		}
	})

	for _, tt := range plantWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newPlantFixture()
			if _, err := f.svc.Archive(t.Context(), tt.user(f), tt.id(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("archive: err = %v, want %v", err, tt.wantErr)
			}
			if _, err := f.svc.Unarchive(t.Context(), tt.user(f), tt.id(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("unarchive: err = %v, want %v", err, tt.wantErr)
			}
			if len(f.plants.archived) != 0 {
				t.Errorf("archived calls = %v, want none", f.plants.archived)
			}
		})
	}
}
