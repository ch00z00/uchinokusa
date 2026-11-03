package postgres_test

import (
	"bytes"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/db"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/seedfile"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// seedSpecies は、リポジトリの初期データ（db/seeds/species.yaml）を投入する。
func seedSpecies(t *testing.T, pool *pgxpool.Pool) []domain.Species {
	t.Helper()
	list, err := seedfile.ParseSpecies(bytes.NewReader(db.SpeciesSeed))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.NewSpeciesRepository(pool).UpsertAll(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestSpeciesRepository_UpsertAll(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewSpeciesRepository(pool)
	list, err := seedfile.ParseSpecies(bytes.NewReader(db.SpeciesSeed))
	if err != nil {
		t.Fatal(err)
	}

	first, err := repo.UpsertAll(ctx, list)
	if err != nil {
		t.Fatal(err)
	}
	if first.Inserted != len(list) || first.Updated != 0 {
		t.Errorf("first run = %+v, want %d inserted", first, len(list))
	}

	// 植物から参照されている種類も、2回目の投入で消えたり ID が変わったりしない
	monstera := searchOne(t, repo, "モンステラ")
	householdID := createHousehold(t, pool)
	if _, err := pool.Exec(ctx, `INSERT INTO plants (household_id, name, species_id) VALUES ($1, 'リビングのモンステラ', $2)`, householdID, monstera.ID); err != nil {
		t.Fatal(err)
	}

	// 値を変えて投入し直すと、同じ slug の行を上書きする（何度実行しても同じ結果になる）
	list[0].Aliases = append(list[0].Aliases, "テスト用の別名")
	second, err := repo.UpsertAll(ctx, list)
	if err != nil {
		t.Fatal(err)
	}
	if second.Inserted != 0 || second.Updated != len(list) {
		t.Errorf("second run = %+v, want %d updated", second, len(list))
	}
	again := searchOne(t, repo, "テスト用の別名")
	if again.ID != monstera.ID {
		t.Errorf("ID changed from %s to %s", monstera.ID, again.ID)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM plants WHERE species_id = $1`, monstera.ID).Scan(&n); err != nil || n != 1 {
		t.Errorf("plant lost its species: n = %d, err = %v", n, err)
	}
}

func searchOne(t *testing.T, repo *postgres.SpeciesRepository, q string) domain.Species {
	t.Helper()
	got, err := repo.Search(t.Context(), domain.NormalizeSearchText(q), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatalf("search %q: no results", q)
	}
	return got[0]
}

// 投入した値と出典が、そのまま読み出せる。
func TestSpeciesRepository_RoundTrip(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t)
	seedSpecies(t, pool)
	repo := postgres.NewSpeciesRepository(pool)

	g := searchOne(t, repo, "ガジュマル")
	if g.Slug != "ficus-microcarpa" || g.ScientificName != "Ficus microcarpa" || g.LightLevel != domain.LevelHigh {
		t.Errorf("species = %+v", g)
	}
	if g.MinTempC == nil || *g.MinTempC != 5 || g.OutdoorOK == nil || !*g.OutdoorOK || g.WaterTendency != domain.WaterModerate {
		t.Errorf("properties = %+v", g)
	}
	if g.Care.Watering == nil || g.Care.Watering.Condition == "" || len(g.Care.RepottingSuitableMonths) != 5 {
		t.Errorf("care = %+v", g.Care)
	}
	if len(g.Sources) != 1 || g.Sources[0].Quote == "" || !g.Sources[0].AccessedOn.IsValid() {
		t.Errorf("sources = %+v", g.Sources)
	}
	if err := g.Validate(); err != nil {
		t.Errorf("read back species is invalid: %v", err)
	}

	// 名前だけの種類は、性質が空のまま
	u := searchOne(t, repo, "ウンベラータ")
	if u.LightLevel != "" || u.MinTempC != nil || u.OutdoorOK != nil || u.Care.Watering != nil || len(u.Sources) != 0 {
		t.Errorf("names-only species = %+v", u)
	}
}

func TestSpeciesRepository_Search(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t)
	seedSpecies(t, pool)
	repo := postgres.NewSpeciesRepository(pool)

	tests := []struct {
		name  string
		q     string
		first string   // 先頭に来るべき和名
		want  []string // 結果に含まれるべき和名
	}{
		{name: "和名", q: "モンステラ", first: "モンステラ"},
		{name: "ひらがなで入力しても見つかる", q: "もんすてら", first: "モンステラ"},
		{name: "半角カナでも見つかる", q: "ﾊﾟｷﾗ", first: "パキラ"},
		{name: "学名（大文字・小文字を区別しない）", q: "FICUS", want: []string{"ガジュマル", "インドゴムノキ", "ベンジャミン", "カシワバゴムノキ", "フィカス・ウンベラータ"}},
		{name: "別名", q: "幸福の木", first: "ドラセナ・フラグランス"},
		{name: "別名（ひらがなを含む）", q: "ゴムの木", first: "インドゴムノキ"},
		{name: "中黒なしで入力しても見つかる", q: "フィカスウンベラータ", first: "フィカス・ウンベラータ"},
		{name: "部分一致", q: "ゴム", want: []string{"インドゴムノキ", "カシワバゴムノキ", "ベンジャミン"}},
		// 和名で一致したもの（文字列の前のほう）を、別名で一致したものより先に並べる
		{name: "和名の一致を先に並べる", q: "ガジュマル", first: "ガジュマル", want: []string{"ベンジャミン"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Search(t.Context(), domain.NormalizeSearchText(tt.q), 20)
			if err != nil {
				t.Fatal(err)
			}
			names := map[string]bool{}
			for _, s := range got {
				names[s.NameJa] = true
			}
			if tt.first != "" && (len(got) == 0 || got[0].NameJa != tt.first) {
				t.Errorf("first = %v, want %s", namesOf(got), tt.first)
			}
			for _, w := range tt.want {
				if !names[w] {
					t.Errorf("results %v do not include %s", namesOf(got), w)
				}
			}
		})
	}

	t.Run("LIKE の特別な文字は、文字そのものとして探す", func(t *testing.T) {
		got, err := repo.Search(t.Context(), "%", 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("%% matched %v", namesOf(got))
		}
	})

	t.Run("空の検索語では、件数の上限まで和名の順に返す", func(t *testing.T) {
		got, err := repo.Search(t.Context(), "", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 5 {
			t.Errorf("results = %d, want 5", len(got))
		}
	})
}

func namesOf(list []domain.Species) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.NameJa)
	}
	return out
}
