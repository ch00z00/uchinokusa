package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// Plant は、世帯で育てている植物である（要件5）。株数に上限はない（要件5-3）。
type Plant struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	PlantProfile
	// SpeciesName は、紐づく植物マスタの和名である。マスタと紐づかなければ空。
	SpeciesName string
	// NextDueOn は、有効なタスクのうち、いちばん近い次回予定日である。タスクがなければゼロ値（要件5-4）。
	NextDueOn civil.Date
	// ArchivedAt は、「枯れた・手放した」としてアーカイブした日時である。nil ならアーカイブしていない（要件5-8）。
	ArchivedAt *time.Time
	CreatedAt  time.Time
}

// PlantProfile は、利用者が登録・編集する植物の情報である（要件5-2）。
// ゼロ値（uuid.Nil、ゼロ値の日付）は未設定を表す。
type PlantProfile struct {
	Name       string
	Species    PlantSpecies
	LocationID uuid.UUID
	AcquiredOn civil.Date // 購入日・お迎え日。植え替えの提案の起点に使う（要件17）
}

// PlantSpecies は、植物の種類である。植物マスタの種類（ID）か、マスタにない種類の自由入力（FreeText）の、どちらか一方だけを持つ（要件6-3）。
// 両方が空なら、種類は未設定である。
type PlantSpecies struct {
	ID       uuid.UUID
	FreeText string
}

// PlantFilter は、植物の一覧の絞り込みの条件である（要件5-5）。
type PlantFilter struct {
	LocationID uuid.UUID // uuid.Nil なら絞り込まない
	Query      string    // NormalizeSearchText で正規化した名前の検索語。空なら絞り込まない
	Archived   bool      // true ならアーカイブした植物だけ、false ならアーカイブしていない植物だけを返す
}

const (
	// MaxPlantNameLength は、植物の名前の最大の文字数である。一覧や通知（「〇〇の水やり」）に収まる長さにする。
	MaxPlantNameLength = 40
	// MaxSpeciesFreeTextLength は、自由入力の種類の最大の文字数である。植物マスタの和名・学名より十分に長い。
	MaxSpeciesFreeTextLength = 50
)

// PlantInput は、植物の登録・変更で受け取った、検証前の内容である。nil は送られなかった項目を表す。
// ID と日付も文字列で受け取る。変更で空文字を「未設定に戻す」の意味に使うためである（docs/decisions/0011）。
type PlantInput struct {
	Name            *string
	SpeciesID       *string
	SpeciesFreeText *string
	LocationID      *string
	AcquiredOn      *string // YYYY-MM-DD
}

// NewPlantProfile は、植物の登録の内容を検証する。名前のほかは任意で、nil と空文字は未設定にする。
func NewPlantProfile(in PlantInput) (PlantProfile, error) {
	v := plantValidator{fields: map[string]string{}}
	p := PlantProfile{
		Name:       v.name(deref(in.Name)),
		Species:    v.species(in.SpeciesID, in.SpeciesFreeText),
		LocationID: v.id("locationId", deref(in.LocationID)),
		AcquiredOn: v.date("acquiredOn", deref(in.AcquiredOn)),
	}
	if err := v.err(); err != nil {
		return PlantProfile{}, err
	}
	return p, nil
}

// PlantPatch は、検証済みの、植物の変更の内容である。
// nil の項目は変えない。ゼロ値（uuid.Nil、ゼロ値の日付、空の PlantSpecies）は、未設定に戻すことを表す。
type PlantPatch struct {
	Name       *string
	Species    *PlantSpecies // 種類は、マスタか自由入力かをまとめて置き換える
	LocationID *uuid.UUID
	AcquiredOn *civil.Date
}

// NewPlantPatch は、植物の変更の内容を検証する。
// 種類は、SpeciesID と SpeciesFreeText のどちらかを送ると、まるごと置き換える（送らなかった方は消える）。
func NewPlantPatch(in PlantInput) (PlantPatch, error) {
	v := plantValidator{fields: map[string]string{}}
	var p PlantPatch
	if in.Name != nil {
		p.Name = ptrTo(v.name(*in.Name))
	}
	if in.SpeciesID != nil || in.SpeciesFreeText != nil {
		p.Species = ptrTo(v.species(in.SpeciesID, in.SpeciesFreeText))
	}
	if in.LocationID != nil {
		p.LocationID = ptrTo(v.id("locationId", *in.LocationID))
	}
	if in.AcquiredOn != nil {
		p.AcquiredOn = ptrTo(v.date("acquiredOn", *in.AcquiredOn))
	}
	if err := v.err(); err != nil {
		return PlantPatch{}, err
	}
	return p, nil
}

// plantValidator は、植物の項目を検証し、誤りを項目ごとに集める。
type plantValidator struct {
	fields map[string]string
}

func (v plantValidator) name(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		v.fields["name"] = "required"
	case utf8.RuneCountInString(s) > MaxPlantNameLength:
		v.fields["name"] = "too_long"
	}
	return s
}

func (v plantValidator) species(id, freeText *string) PlantSpecies {
	sp := PlantSpecies{ID: v.id("speciesId", deref(id)), FreeText: strings.TrimSpace(deref(freeText))}
	switch {
	case utf8.RuneCountInString(sp.FreeText) > MaxSpeciesFreeTextLength:
		v.fields["speciesFreeText"] = "too_long"
	case sp.FreeText != "" && strings.TrimSpace(deref(id)) != "":
		v.fields["speciesFreeText"] = "exclusive"
	}
	return sp
}

// id は、UUID の文字列を読む。空文字は未設定（uuid.Nil）にする。
func (v plantValidator) id(field, s string) uuid.UUID {
	s = strings.TrimSpace(s)
	if s == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		v.fields[field] = "invalid"
		return uuid.Nil
	}
	return id
}

// date は、YYYY-MM-DD の日付を読む。空文字は未設定（ゼロ値）にする。2026-02-30 のような存在しない日は誤りにする。
func (v plantValidator) date(field, s string) civil.Date {
	s = strings.TrimSpace(s)
	if s == "" {
		return civil.Date{}
	}
	d, err := civil.ParseDate(s)
	if err != nil || !d.IsValid() {
		v.fields[field] = "invalid"
		return civil.Date{}
	}
	return d
}

func (v plantValidator) err() error {
	if len(v.fields) > 0 {
		return &ValidationError{Fields: v.fields}
	}
	return nil
}
