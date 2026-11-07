package postgres_test

import (
	"errors"
	"slices"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// plantFixture は、植物のリポジトリの結合テストの共通の状態である。
// 世帯（home）と、その置き場所・植物マスタの種類、ほかの世帯（other）とその置き場所を用意する。
type plantFixture struct {
	*fixture
	repo          *postgres.PlantRepository
	home, other   uuid.UUID
	living        uuid.UUID // home の置き場所
	otherLocation uuid.UUID // other の置き場所
	monstera      uuid.UUID // 植物マスタの種類
}

func newPlantFixture(t *testing.T) plantFixture {
	t.Helper()
	f := plantFixture{fixture: newFixture(t)}
	f.repo = postgres.NewPlantRepository(f.pool)
	f.home, f.other = uuid.MustParse(f.household()), uuid.MustParse(f.household())
	f.living = uuid.MustParse(f.location(f.home.String()))
	f.otherLocation = uuid.MustParse(f.location(f.other.String()))
	f.monstera = uuid.MustParse(f.id(`INSERT INTO species (slug, name_ja) VALUES ('monstera-deliciosa', 'モンステラ')`))
	return f
}

func (f plantFixture) create(name string, p domain.PlantProfile) domain.Plant {
	f.t.Helper()
	p.Name = name
	plant, err := f.repo.Create(f.ctx, f.home, p)
	if err != nil {
		f.t.Fatal(err)
	}
	return plant
}

// addTask は、植物にタスクを作る。status が paused なら一時停止、deleted なら削除済みにする。
func (f plantFixture) addTask(plant uuid.UUID, nextDueOn, status string, deleted bool) {
	f.t.Helper()
	f.exec(`INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, next_due_on, status, deleted_at)
		SELECT household_id, id, 'watering', 7, 'day', $2, $3, CASE WHEN $4 THEN now() END FROM plants WHERE id = $1`,
		plant, nextDueOn, status, deleted)
}

func namesOfPlants(list []domain.Plant) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.Name)
	}
	return out
}

func TestPlantRepository_CreateAndGet(t *testing.T) {
	t.Parallel()

	t.Run("マスタの種類・置き場所・購入日つきで登録し、マスタの和名とともに読み出す", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		want := domain.PlantProfile{
			Name:       "リビングのモンステラ",
			Species:    domain.PlantSpecies{ID: f.monstera},
			LocationID: f.living,
			AcquiredOn: civil.Date{Year: 2026, Month: 5, Day: 3},
		}
		created, err := f.repo.Create(f.ctx, f.home, want)
		if err != nil {
			t.Fatal(err)
		}
		if created.ID == uuid.Nil || created.HouseholdID != f.home || created.PlantProfile != want ||
			created.SpeciesName != "モンステラ" || created.ArchivedAt != nil || created.CreatedAt.IsZero() {
			t.Errorf("created = %+v", created)
		}

		got, err := f.repo.Get(f.ctx, f.home, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.PlantProfile != want || got.SpeciesName != "モンステラ" || !got.NextDueOn.IsZero() {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("名前だけ・自由入力の種類でも登録できる", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		got := f.create("多肉", domain.PlantProfile{Species: domain.PlantSpecies{FreeText: "多肉植物"}})
		if want := (domain.PlantProfile{Name: "多肉", Species: domain.PlantSpecies{FreeText: "多肉植物"}}); got.PlantProfile != want || got.SpeciesName != "" {
			t.Errorf("got = %+v", got)
		}
	})

	// 種類・置き場所の誤りは、外部キーの違反を入力の誤りに変換する
	invalid := []struct {
		name      string
		profile   func(plantFixture) domain.PlantProfile
		wantField string
	}{
		{name: "存在しない種類", profile: func(plantFixture) domain.PlantProfile {
			return domain.PlantProfile{Species: domain.PlantSpecies{ID: uuid.Must(uuid.NewV7())}}
		}, wantField: "speciesId"},
		{name: "存在しない置き場所", profile: func(plantFixture) domain.PlantProfile {
			return domain.PlantProfile{LocationID: uuid.Must(uuid.NewV7())}
		}, wantField: "locationId"},
		{name: "ほかの世帯の置き場所", profile: func(f plantFixture) domain.PlantProfile {
			return domain.PlantProfile{LocationID: f.otherLocation}
		}, wantField: "locationId"},
	}
	for _, tt := range invalid {
		t.Run(tt.name+"は入力の誤り", func(t *testing.T) {
			t.Parallel()
			f := newPlantFixture(t)
			p := tt.profile(f)
			p.Name = "ポトス"
			_, err := f.repo.Create(f.ctx, f.home, p)
			var verr *domain.ValidationError
			if !errors.As(err, &verr) || verr.Fields[tt.wantField] != "invalid" || len(verr.Fields) != 1 {
				t.Errorf("err = %v, want %s invalid", err, tt.wantField)
			}
		})
	}

	t.Run("ほかの世帯の植物は Get できない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})
		if _, err := f.repo.Get(f.ctx, f.other, plant.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestPlantRepository_HouseholdOf(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	plant := f.create("ポトス", domain.PlantProfile{})

	if got, err := f.repo.HouseholdOf(f.ctx, plant.ID); err != nil || got != f.home {
		t.Errorf("HouseholdOf = %v, %v, want %v", got, err, f.home)
	}
	if _, err := f.repo.HouseholdOf(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestPlantRepository_List(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	monstera := f.create("モンステラ", domain.PlantProfile{LocationID: f.living})
	pothos := f.create("ポトス 100%", domain.PlantProfile{})
	archived := f.create("枯れたモンステラ", domain.PlantProfile{LocationID: f.living})
	if _, err := f.repo.SetArchived(f.ctx, f.home, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Create(f.ctx, f.other, domain.PlantProfile{Name: "ほかの世帯のモンステラ"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		filter domain.PlantFilter
		want   []string
	}{
		{name: "条件がなければ、アーカイブしていない植物を登録の古い順に返す", filter: domain.PlantFilter{}, want: []string{"モンステラ", "ポトス 100%"}},
		{name: "アーカイブした植物だけ", filter: domain.PlantFilter{Archived: true}, want: []string{"枯れたモンステラ"}},
		{name: "置き場所で絞り込む", filter: domain.PlantFilter{LocationID: f.living}, want: []string{"モンステラ"}},
		// 正規化した名前（name_search）と、正規化した検索語で比べる
		{name: "名前の部分一致で探す（表記の揺れは呼び出し側で正規化する）", filter: domain.PlantFilter{Query: domain.NormalizeSearchText("すてら")}, want: []string{"モンステラ"}},
		{name: "% は文字そのものとして探す", filter: domain.PlantFilter{Query: "%"}, want: []string{"ポトス 100%"}},
		{name: "一致しなければ空", filter: domain.PlantFilter{Query: "サボテン"}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.repo.List(f.ctx, f.home, tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if names := namesOfPlants(got); len(names) != len(tt.want) || (len(names) > 0 && names[0] != tt.want[0]) || (len(names) > 1 && names[1] != tt.want[1]) {
				t.Errorf("names = %q, want %q", names, tt.want)
			}
		})
	}

	t.Run("次のお手入れ予定日は、有効なタスクのいちばん近い予定日", func(t *testing.T) {
		f.addTask(monstera.ID, "2026-11-10", "active", false)
		f.addTask(monstera.ID, "2026-11-08", "active", false)
		f.addTask(monstera.ID, "2026-11-01", "paused", false) // 一時停止したタスクは数えない
		f.addTask(monstera.ID, "2026-11-02", "active", true)  // 削除したタスクも数えない
		got, err := f.repo.List(f.ctx, f.home, domain.PlantFilter{})
		if err != nil {
			t.Fatal(err)
		}
		byID := map[uuid.UUID]domain.Plant{}
		for _, p := range got {
			byID[p.ID] = p
		}
		if want := (civil.Date{Year: 2026, Month: 11, Day: 8}); byID[monstera.ID].NextDueOn != want {
			t.Errorf("monstera next due = %v, want %v", byID[monstera.ID].NextDueOn, want)
		}
		if !byID[pothos.ID].NextDueOn.IsZero() {
			t.Errorf("pothos next due = %v, want zero (no tasks)", byID[pothos.ID].NextDueOn)
		}
	})
}

func TestPlantRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("送った項目だけを変え、名前を変えると検索にも反映される", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{Species: domain.PlantSpecies{FreeText: "ポトス"}, LocationID: f.living, AcquiredOn: civil.Date{Year: 2026, Month: 5, Day: 3}})

		got, err := f.repo.Update(f.ctx, f.home, plant.ID, domain.PlantPatch{Name: ptrOf("ゴールデンポトス")})
		if err != nil {
			t.Fatal(err)
		}
		want := plant.PlantProfile
		want.Name = "ゴールデンポトス"
		if got.PlantProfile != want {
			t.Errorf("got %+v, want %+v", got.PlantProfile, want)
		}
		found, err := f.repo.List(f.ctx, f.home, domain.PlantFilter{Query: domain.NormalizeSearchText("ごーるでん")})
		if err != nil || len(found) != 1 {
			t.Errorf("search after rename = %v, %v", namesOfPlants(found), err)
		}
	})

	t.Run("種類をマスタに替え、置き場所と購入日を未設定に戻す", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{Species: domain.PlantSpecies{FreeText: "ポトス"}, LocationID: f.living, AcquiredOn: civil.Date{Year: 2026, Month: 5, Day: 3}})

		got, err := f.repo.Update(f.ctx, f.home, plant.ID, domain.PlantPatch{
			Species: &domain.PlantSpecies{ID: f.monstera}, LocationID: ptrOf(uuid.Nil), AcquiredOn: ptrOf(civil.Date{}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if want := (domain.PlantProfile{Name: "ポトス", Species: domain.PlantSpecies{ID: f.monstera}}); got.PlantProfile != want || got.SpeciesName != "モンステラ" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("ほかの世帯の置き場所には変えられない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})
		_, err := f.repo.Update(f.ctx, f.home, plant.ID, domain.PlantPatch{LocationID: &f.otherLocation})
		var verr *domain.ValidationError
		if !errors.As(err, &verr) || verr.Fields["locationId"] != "invalid" {
			t.Errorf("err = %v, want locationId invalid", err)
		}
	})

	t.Run("ほかの世帯の植物は変更できない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})
		if _, err := f.repo.Update(f.ctx, f.other, plant.ID, domain.PlantPatch{Name: ptrOf("x")}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		if got, _ := f.repo.Get(f.ctx, f.home, plant.ID); got.Name != "ポトス" {
			t.Errorf("name = %q, want unchanged", got.Name)
		}
	})
}

func TestPlantRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("タスク・記録・メモもいっしょに消える", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})
		task := f.task(f.home.String(), plant.ID.String())
		f.record(task, "completed", 1, false)
		f.note(f.home.String(), plant.ID.String())

		if err := f.repo.Delete(f.ctx, f.home, plant.ID); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"plants", "care_tasks", "care_records", "notes"} {
			if n := f.count(`SELECT count(*) FROM ` + table); n != 0 {
				t.Errorf("%s: %d rows left", table, n)
			}
		}
	})

	t.Run("ほかの世帯の植物は削除できない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})
		if err := f.repo.Delete(f.ctx, f.other, plant.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		if n := f.count(`SELECT count(*) FROM plants`); n != 1 {
			t.Errorf("plants = %d, want 1", n)
		}
	})
}

func TestPlantRepository_SetArchived(t *testing.T) {
	t.Parallel()

	t.Run("アーカイブし直しても、最初の日時のまま。元に戻すと消える", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})

		first, err := f.repo.SetArchived(f.ctx, f.home, plant.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if first.ArchivedAt == nil {
			t.Fatalf("archived = %+v", first)
		}
		again, err := f.repo.SetArchived(f.ctx, f.home, plant.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if again.ArchivedAt == nil || !again.ArchivedAt.Equal(*first.ArchivedAt) {
			t.Errorf("archived again at %v, want %v", again.ArchivedAt, first.ArchivedAt)
		}
		restored, err := f.repo.SetArchived(f.ctx, f.home, plant.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		if restored.ArchivedAt != nil {
			t.Errorf("restored = %+v", restored)
		}
	})

	t.Run("ほかの世帯の植物はアーカイブできない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		plant := f.create("ポトス", domain.PlantProfile{})
		if _, err := f.repo.SetArchived(f.ctx, f.other, plant.ID, true); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		// エラーだけでなく、行が変わっていないことも確かめる（読み直しの Get が 404 を返すだけの場合を見逃さない）
		if got, _ := f.repo.Get(f.ctx, f.home, plant.ID); got.ArchivedAt != nil {
			t.Errorf("archived_at = %v, want unchanged (nil)", got.ArchivedAt)
		}
	})
}

func ptrOf[T any](v T) *T { return &v }

func TestPlantRepository_ActiveIDs(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	pachira := f.create("パキラ", domain.PlantProfile{})
	archived := f.create("枯れたシダ", domain.PlantProfile{})
	if _, err := f.repo.SetArchived(f.ctx, f.home, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	other, err := f.repo.Create(f.ctx, f.other, domain.PlantProfile{Name: "ポトス"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.ActiveIDs(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// すべての世帯の、アーカイブしていない植物
	want := sortedIDs(pachira.ID, other.ID)
	if !slices.Equal(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}
