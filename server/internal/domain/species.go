package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// Species は、植物マスタの1種類である。全ユーザー共通のデータで、世帯には属さない（要件6-6）。
// 性質の値は、出典で確かめられたものだけを入れる。空（ゼロ値・nil）は「わからない」を表す（docs/decisions/0009）。
type Species struct {
	ID             uuid.UUID
	Slug           string // 初期データの各行を指す固定の名前（例：monstera-deliciosa）
	NameJa         string
	ScientificName string
	Aliases        []string

	LightLevel    Level         // 好む光の量
	MinTempC      *float64      // 冬越しの最低温度の目安
	HumidityLevel Level         // 好む湿度
	AirflowNeed   Level         // 風通しの必要性
	WaterTendency WaterTendency // 水やりの量の傾向
	OutdoorOK     *bool         // 春〜秋に屋外に出せるか

	Care    SpeciesCare
	Sources []SpeciesSource
}

// Level は、光・湿度・風通しの程度である。
type Level string

const (
	LevelLow    Level = "low"
	LevelMedium Level = "medium"
	LevelHigh   Level = "high"
)

// WaterTendency は、水やりの量の傾向である。
type WaterTendency string

const (
	WaterLess     WaterTendency = "less"     // 乾かし気味
	WaterModerate WaterTendency = "moderate" // 鉢土の表面が乾いたら
	WaterMore     WaterTendency = "more"     // 乾かさない
)

// SpeciesCare は、種類ごとのお手入れの目安である。タスクの作成画面の初期値に使う（要件6-4）。
type SpeciesCare struct {
	Watering *WateringAdvice
	// RepottingSuitableMonths は、植え替えの適期の月（北半球、1〜12）である。空なら既定値を使う（要件17-3）。
	RepottingSuitableMonths []int
}

// WateringAdvice は、水やりの実行条件の目安である。
type WateringAdvice struct {
	Condition string // 例：鉢土の表面が乾いてきたら
	Winter    string // 冬の管理（空ならほかの季節と同じ）
}

// SpeciesSource は、性質の値の出典である。Fields は、この出典にもとづく項目の名前である（要件17-10）。
type SpeciesSource struct {
	URL        string
	Title      string
	AccessedOn civil.Date
	Fields     []string
	Quote      string // 値の根拠になった文（見直しのため）
}

// 出典の Fields に書ける項目の名前。
const (
	fieldLightLevel    = "light_level"
	fieldMinTempC      = "min_temp_c"
	fieldHumidityLevel = "humidity_level"
	fieldAirflowNeed   = "airflow_need"
	fieldWaterTendency = "water_tendency"
	fieldOutdoorOK     = "outdoor_ok"
	fieldWatering      = "watering"
	fieldRepotting     = "repotting"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// setFields は、値が入っている性質の項目の名前を返す。
func (s Species) setFields() map[string]bool {
	return map[string]bool{
		fieldLightLevel:    s.LightLevel != "",
		fieldMinTempC:      s.MinTempC != nil,
		fieldHumidityLevel: s.HumidityLevel != "",
		fieldAirflowNeed:   s.AirflowNeed != "",
		fieldWaterTendency: s.WaterTendency != "",
		fieldOutdoorOK:     s.OutdoorOK != nil,
		fieldWatering:      s.Care.Watering != nil,
		fieldRepotting:     len(s.Care.RepottingSuitableMonths) > 0,
	}
}

// Validate は、植物マスタの1種類を検証する。値を入れた項目には、出典がなければならない。
func (s Species) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if !slugPattern.MatchString(s.Slug) {
		add("slug %q must be lowercase words joined by hyphens", s.Slug)
	}
	if strings.TrimSpace(s.NameJa) == "" {
		add("name_ja is required")
	}
	for field, v := range map[string]Level{fieldLightLevel: s.LightLevel, fieldHumidityLevel: s.HumidityLevel, fieldAirflowNeed: s.AirflowNeed} {
		if v != "" && v != LevelLow && v != LevelMedium && v != LevelHigh {
			add("%s %q must be low, medium, or high", field, v)
		}
	}
	if w := s.WaterTendency; w != "" && w != WaterLess && w != WaterModerate && w != WaterMore {
		add("water_tendency %q must be less, moderate, or more", w)
	}
	if t := s.MinTempC; t != nil && (*t < -30 || *t > 30) {
		add("min_temp_c %v is out of range", *t)
	}
	if w := s.Care.Watering; w != nil && strings.TrimSpace(w.Condition) == "" {
		add("watering.condition is required")
	}
	if m := s.Care.RepottingSuitableMonths; len(m) > 0 {
		if _, err := NewRepottingRule(m, nil); err != nil {
			add("repotting: %w", err)
		}
	}

	// 出典の決まり：値を入れた項目はどれかの出典に挙げ、出典に挙げた項目には値を入れる
	set := s.setFields()
	sourced := map[string]bool{}
	for i, src := range s.Sources {
		if !strings.HasPrefix(src.URL, "https://") {
			add("sources[%d].url %q must start with https://", i, src.URL)
		}
		if strings.TrimSpace(src.Title) == "" {
			add("sources[%d].title is required", i)
		}
		if !src.AccessedOn.IsValid() {
			add("sources[%d].accessed_on is required", i)
		}
		for _, f := range src.Fields {
			if _, known := set[f]; !known {
				add("sources[%d].fields: unknown field %q", i, f)
				continue
			}
			sourced[f] = true
		}
	}
	for f, isSet := range set {
		switch {
		case isSet && !sourced[f]:
			add("%s has no source", f)
		case !isSet && sourced[f]:
			add("%s is listed in a source but not set", f)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("species %q: %w", s.Slug, err)
	}
	return nil
}

// ValidateSpeciesList は、初期データ全体を検証する。各種類の検証に加え、slug と和名の重複を確かめる。
func ValidateSpeciesList(list []Species) error {
	var errs []error
	slugs, names := map[string]bool{}, map[string]bool{}
	for _, s := range list {
		if err := s.Validate(); err != nil {
			errs = append(errs, err)
		}
		if slugs[s.Slug] {
			errs = append(errs, fmt.Errorf("duplicate slug %q", s.Slug))
		}
		if names[s.NameJa] {
			errs = append(errs, fmt.Errorf("duplicate name_ja %q", s.NameJa))
		}
		slugs[s.Slug], names[s.NameJa] = true, true
	}
	return errors.Join(errs...)
}

// SearchText は、和名・学名・別名を正規化し、空白で区切ってつないだ検索用の文字列を返す。
// 検索語は NormalizeSearchText で空白を除くので、名前をまたいで一致することはない。
func (s Species) SearchText() string {
	names := append([]string{s.NameJa, s.ScientificName}, s.Aliases...)
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n := NormalizeSearchText(n); n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, " ")
}

// NormalizeSearchText は、検索のために表記の揺れをそろえる。
// 全角・半角（NFKC）、大文字・小文字、ひらがな・カタカナをそろえ、空白と中黒（・）を除く。
func NormalizeSearchText(s string) string {
	s = strings.ToLower(norm.NFKC.String(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r), r == '・':
			continue
		case (r >= 'ぁ' && r <= 'ゖ') || r == 'ゝ' || r == 'ゞ':
			r += 'ァ' - 'ぁ' // ひらがなとカタカナは、Unicode で同じ並びになっている
		}
		b.WriteRune(r)
	}
	return b.String()
}

// RepottingRule は、この種類の植え替えのルールを返す（要件17-3）。
// 種類ごとの適期があれば、それを適期にし、既定値の避ける時期から適期の月を除いたものを避ける時期にする。
// 出典には「適期」は書かれていても「避ける時期」は書かれていないことが多いため、避ける時期は既定値を引き継ぐ。
func (s Species) RepottingRule() RepottingRule {
	suitable, err := monthsOf(s.Care.RepottingSuitableMonths)
	if err != nil || suitable == 0 {
		return DefaultRepottingRule
	}
	return RepottingRule{Suitable: suitable, Avoid: DefaultRepottingRule.Avoid &^ suitable}
}
