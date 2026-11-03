package seedfile_test

import (
	"bytes"
	"strings"
	"testing"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/db"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/seedfile"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// リポジトリの初期データそのものが、決まり（出典など）を満たしていることを確かめる。
// データを編集して決まりを破ると、このテストが失敗する。
func TestParseSpecies_EmbeddedSeed(t *testing.T) {
	t.Parallel()

	list, err := seedfile.ParseSpecies(bytes.NewReader(db.SpeciesSeed))
	if err != nil {
		t.Fatalf("db/seeds/species.yaml is invalid:\n%v", err)
	}
	// 主要な室内観葉植物 30〜50種（要件6-5）
	if n := len(list); n < 30 || n > 50 {
		t.Errorf("species = %d, want 30 to 50", n)
	}
	withCare := 0
	for _, s := range list {
		if len(s.Sources) > 0 {
			withCare++
		}
	}
	if withCare < 10 {
		t.Errorf("species with sourced care = %d, want at least 10", withCare)
	}
}

func TestParseSpecies(t *testing.T) {
	t.Parallel()

	const doc = `
- slug: ficus-microcarpa
  name_ja: ガジュマル
  scientific_name: Ficus microcarpa
  aliases: [フィカス・ミクロカルパ]
  light_level: high
  min_temp_c: 5
  outdoor_ok: true
  watering:
    condition: 鉢土の表面が乾いてから
    winter: 乾かし気味に
  repotting_suitable_months: [5, 6, 7, 8, 9]
  sources:
    - url: https://example.com/gajumaru
      title: ガジュマルの育て方
      accessed_on: 2026-11-03
      fields: [light_level, min_temp_c, outdoor_ok, watering, repotting]
      quote: 冬は5℃以上
- slug: ficus-umbellata
  name_ja: フィカス・ウンベラータ
`
	list, err := seedfile.ParseSpecies(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("species = %d, want 2", len(list))
	}
	g := list[0]
	if g.Slug != "ficus-microcarpa" || g.NameJa != "ガジュマル" || g.ScientificName != "Ficus microcarpa" || len(g.Aliases) != 1 {
		t.Errorf("names = %+v", g)
	}
	if g.LightLevel != domain.LevelHigh || g.MinTempC == nil || *g.MinTempC != 5 || g.OutdoorOK == nil || !*g.OutdoorOK {
		t.Errorf("properties = %+v", g)
	}
	if g.Care.Watering == nil || g.Care.Watering.Winter != "乾かし気味に" || len(g.Care.RepottingSuitableMonths) != 5 {
		t.Errorf("care = %+v", g.Care)
	}
	if src := g.Sources[0]; src.AccessedOn != (civil.Date{Year: 2026, Month: 11, Day: 3}) || src.Quote != "冬は5℃以上" {
		t.Errorf("source = %+v", src)
	}
	if u := list[1]; u.LightLevel != "" || u.MinTempC != nil || u.Care.Watering != nil {
		t.Errorf("names-only species has properties: %+v", u)
	}
}

func TestParseSpecies_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		// 書き間違えた項目を黙って無視すると、出典の決まりの検証をすり抜けるため、エラーにする
		{name: "知らない項目", doc: "- slug: a\n  name_ja: あ\n  ligth_level: high\n", wantErr: "ligth_level"},
		{name: "出典のない値", doc: "- slug: a\n  name_ja: あ\n  light_level: high\n", wantErr: "light_level has no source"},
		{name: "日付の形式が違う", doc: "- slug: a\n  name_ja: あ\n  sources:\n    - url: https://example.com\n      title: t\n      accessed_on: 2026/11/03\n", wantErr: "accessed_on"},
		{name: "slug の重複", doc: "- slug: a\n  name_ja: あ\n- slug: a\n  name_ja: い\n", wantErr: "duplicate slug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := seedfile.ParseSpecies(strings.NewReader(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want an error containing %q", err, tt.wantErr)
			}
		})
	}
}
