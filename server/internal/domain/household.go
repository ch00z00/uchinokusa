package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Household は、植物をまとめて管理する単位（自宅・実家など）である。
type Household struct {
	ID         uuid.UUID
	Name       string
	Region     Region
	TimeZone   string
	Hemisphere Hemisphere
	CreatedAt  time.Time
}

// Region は、世帯のおおよその地域である（要件7-5）。空文字は未設定を表す。
type Region struct {
	Prefecture string // 都道府県
	City       string // 市区町村
}

// HouseholdWithRole は、世帯と、それを見ているユーザーのロールである。
type HouseholdWithRole struct {
	Household
	Role Role
}

const (
	// MaxHouseholdNameLength は、世帯名の最大の文字数である。世帯の切り替えの一覧や通知の文面に収まる長さにする。
	MaxHouseholdNameLength = 30
	// MaxRegionLength は、都道府県・市区町村それぞれの最大の文字数である。
	MaxRegionLength = 50
)

// NewHousehold は、検証済みの、世帯の作成の内容である。
type NewHousehold struct {
	Name   string
	Region Region
}

// NewHouseholdInput は、世帯の作成の内容を検証する。地域は任意で、nil と空文字は未設定として扱う。
func NewHouseholdInput(name string, prefecture, city *string) (NewHousehold, error) {
	v := householdValidator{fields: map[string]string{}}
	h := NewHousehold{
		Name: v.name(name),
		Region: Region{
			Prefecture: v.region("regionPrefecture", deref(prefecture)),
			City:       v.region("regionCity", deref(city)),
		},
	}
	if err := v.err(); err != nil {
		return NewHousehold{}, err
	}
	return h, nil
}

// HouseholdPatch は、検証済みの、世帯の変更の内容である。
// nil の項目は変えない。地域の空文字は、未設定に戻すことを表す。
type HouseholdPatch struct {
	Name       *string
	Prefecture *string
	City       *string
}

// NewHouseholdPatch は、世帯の変更の内容を検証する。
func NewHouseholdPatch(name, prefecture, city *string) (HouseholdPatch, error) {
	v := householdValidator{fields: map[string]string{}}
	var p HouseholdPatch
	if name != nil {
		p.Name = ptrTo(v.name(*name))
	}
	if prefecture != nil {
		p.Prefecture = ptrTo(v.region("regionPrefecture", *prefecture))
	}
	if city != nil {
		p.City = ptrTo(v.region("regionCity", *city))
	}
	if err := v.err(); err != nil {
		return HouseholdPatch{}, err
	}
	return p, nil
}

// householdValidator は、世帯の項目を検証し、誤りを項目ごとに集める。
type householdValidator struct {
	fields map[string]string
}

func (v householdValidator) name(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		v.fields["name"] = "required"
	case utf8.RuneCountInString(s) > MaxHouseholdNameLength:
		v.fields["name"] = "too_long"
	}
	return s
}

func (v householdValidator) region(field, s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxRegionLength {
		v.fields[field] = "too_long"
	}
	return s
}

func (v householdValidator) err() error {
	if len(v.fields) > 0 {
		return &ValidationError{Fields: v.fields}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptrTo[T any](v T) *T { return &v }
