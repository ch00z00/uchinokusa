package domain_test

import (
	"strings"
	"testing"

	"cloud.google.com/go/civil"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNormalizeSearchText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{in: "モンステラ", want: "モンステラ"},
		{in: "もんすてら", want: "モンステラ"},                  // ひらがなをカタカナにそろえる
		{in: "ﾓﾝｽﾃﾗ", want: "モンステラ"},                  // 半角カナを全角にそろえる（NFKC）
		{in: "Ｍｏｎｓｔｅｒａ", want: "monstera"},            // 全角英字を半角・小文字に
		{in: "Ficus elastica", want: "ficuselastica"}, // 空白を除く
		{in: "フィカス・ウンベラータ", want: "フィカスウンベラータ"},       // 中黒を除く
		{in: "ゴムの木", want: "ゴムノ木"},                    // 漢字はそのまま
		{in: "  ", want: ""},
	}
	for _, tt := range tests {
		if got := domain.NormalizeSearchText(tt.in); got != tt.want {
			t.Errorf("NormalizeSearchText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func validSpecies() domain.Species {
	minTemp := 5.0
	outdoor := true
	return domain.Species{
		Slug:           "ficus-microcarpa",
		NameJa:         "ガジュマル",
		ScientificName: "Ficus microcarpa",
		Aliases:        []string{"フィカス・ミクロカルパ"},
		LightLevel:     domain.LevelHigh,
		MinTempC:       &minTemp,
		OutdoorOK:      &outdoor,
		WaterTendency:  domain.WaterModerate,
		Care: domain.SpeciesCare{
			Watering:                &domain.WateringAdvice{Condition: "鉢土の表面が乾いてから"},
			RepottingSuitableMonths: []int{5, 6, 7, 8, 9},
		},
		Sources: []domain.SpeciesSource{{
			URL:        "https://www.shuminoengei.jp/m-pc/a-page_p_detail/target_plant_code-1045/target_tab-2",
			Title:      "ガジュマルの育て方・栽培方法｜みんなの趣味の園芸（NHK出版）",
			AccessedOn: civil.Date{Year: 2026, Month: 11, Day: 3},
			Fields:     []string{"light_level", "min_temp_c", "outdoor_ok", "water_tendency", "watering", "repotting"},
		}},
	}
}

func TestSpecies_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(*domain.Species)
		wantErr string // 空なら成功。エラーの文に含まれるべき語
	}{
		{name: "正しいデータ", modify: func(*domain.Species) {}},
		{name: "名前だけで、性質も出典もない", modify: func(s *domain.Species) {
			*s = domain.Species{Slug: "ficus-umbellata", NameJa: "フィカス・ウンベラータ", ScientificName: "Ficus umbellata"}
		}},
		{name: "slug の形式が違う", modify: func(s *domain.Species) { s.Slug = "Ficus Microcarpa" }, wantErr: "slug"},
		{name: "和名がない", modify: func(s *domain.Species) { s.NameJa = " " }, wantErr: "name_ja"},
		{name: "光の量が知らない値", modify: func(s *domain.Species) { s.LightLevel = "bright" }, wantErr: "light_level"},
		{name: "水やりの傾向が知らない値", modify: func(s *domain.Species) { s.WaterTendency = "often" }, wantErr: "water_tendency"},
		{name: "最低温度がありえない値", modify: func(s *domain.Species) { v := 45.0; s.MinTempC = &v }, wantErr: "min_temp_c"},
		{name: "植え替えの月が範囲外", modify: func(s *domain.Species) { s.Care.RepottingSuitableMonths = []int{5, 13} }, wantErr: "repotting"},
		// 出典の決まり：値を入れた項目は、どれかの出典の fields に挙げる（docs/decisions/0009）
		{name: "出典のない値がある", modify: func(s *domain.Species) { s.HumidityLevel = domain.LevelHigh }, wantErr: "humidity_level has no source"},
		{name: "出典に挙げた項目に値がない", modify: func(s *domain.Species) { s.MinTempC = nil }, wantErr: "min_temp_c is listed in a source but not set"},
		{name: "出典の URL が https でない", modify: func(s *domain.Species) { s.Sources[0].URL = "http://example.com" }, wantErr: "url"},
		{name: "出典の題名がない", modify: func(s *domain.Species) { s.Sources[0].Title = "" }, wantErr: "title"},
		{name: "出典の確認日がない", modify: func(s *domain.Species) { s.Sources[0].AccessedOn = civil.Date{} }, wantErr: "accessed_on"},
		{name: "出典の fields に知らない項目", modify: func(s *domain.Species) { s.Sources[0].Fields = append(s.Sources[0].Fields, "color") }, wantErr: "color"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := validSpecies()
			tt.modify(&s)
			err := s.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want an error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSpeciesList(t *testing.T) {
	t.Parallel()

	a := validSpecies()
	b := validSpecies()
	b.NameJa = "別の名前"
	if err := domain.ValidateSpeciesList([]domain.Species{a, b}); err == nil || !strings.Contains(err.Error(), "duplicate slug") {
		t.Errorf("duplicate slug: err = %v", err)
	}
	b = validSpecies()
	b.Slug = "another-slug"
	if err := domain.ValidateSpeciesList([]domain.Species{a, b}); err == nil || !strings.Contains(err.Error(), "duplicate name_ja") {
		t.Errorf("duplicate name: err = %v", err)
	}
}

func TestSpecies_SearchText(t *testing.T) {
	t.Parallel()
	s := validSpecies()
	got := s.SearchText()
	for _, want := range []string{"ガジュマル", "ficusmicrocarpa", "フィカスミクロカルパ"} {
		if !strings.Contains(got, want) {
			t.Errorf("SearchText() = %q, missing %q", got, want)
		}
	}
	// 検索語は空白を含まない（NormalizeSearchText で除く）ので、名前どうしを空白で区切れば、名前をまたいで一致しない
	if strings.Contains(got, "ルficus") {
		t.Errorf("SearchText() = %q joins names without a separator", got)
	}
}

// 種類ごとの植え替えの適期は、既定値の避ける時期より優先する。
func TestSpecies_RepottingRule(t *testing.T) {
	t.Parallel()

	july := civil.Date{Year: 2026, Month: 7, Day: 15}
	august := civil.Date{Year: 2026, Month: 8, Day: 15}
	january := civil.Date{Year: 2026, Month: 1, Day: 15}

	tests := []struct {
		name     string
		suitable []int
		want     map[civil.Date]domain.RepottingPeriod
	}{
		{
			name:     "情報がなければ既定値",
			suitable: nil,
			want:     map[civil.Date]domain.RepottingPeriod{july: domain.RepottingMidsummer, january: domain.RepottingMidwinter},
		},
		{
			name:     "適期が 5〜9月なら、7〜8月は適期になり、冬は避ける時期のまま",
			suitable: []int{5, 6, 7, 8, 9},
			want:     map[civil.Date]domain.RepottingPeriod{july: domain.RepottingSuitable, august: domain.RepottingSuitable, january: domain.RepottingMidwinter},
		},
		{
			name:     "適期が 5〜7月なら、8月は避ける時期のまま",
			suitable: []int{5, 6, 7},
			want:     map[civil.Date]domain.RepottingPeriod{july: domain.RepottingSuitable, august: domain.RepottingMidsummer},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := domain.Species{Care: domain.SpeciesCare{RepottingSuitableMonths: tt.suitable}}
			rule := s.RepottingRule()
			for d, want := range tt.want {
				if got := rule.PeriodOf(d, domain.HemisphereNorth); got != want {
					t.Errorf("%s: %s, want %s", d, got, want)
				}
			}
		})
	}
}
