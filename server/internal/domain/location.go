package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Location は、世帯の植物の置き場所である（要件7）。
type Location struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	LocationProfile
	// PlantCount は、この置き場所にある植物（アーカイブしたものを除く）の数である。
	// 削除の確認で「植物が未設定になる」ことを示すために使う（要件7-4）。
	PlantCount int
	CreatedAt  time.Time
}

// LocationProfile は、置き場所の名前と環境の情報である。環境の項目の空文字は、未設定（わからない）を表す（要件7-2）。
// 天気の助言（フェーズ2）でも、この値を使う。
type LocationProfile struct {
	Name            string
	IsIndoor        bool
	WindowDirection WindowDirection
	WindowDistance  WindowDistance
	Shading         Shading
}

// WindowDirection は、窓の向き（8方位）である。屋外の場合は、その場所が面している向きを表す。
type WindowDirection string

const (
	DirectionNorth     WindowDirection = "n"
	DirectionNortheast WindowDirection = "ne"
	DirectionEast      WindowDirection = "e"
	DirectionSoutheast WindowDirection = "se"
	DirectionSouth     WindowDirection = "s"
	DirectionSouthwest WindowDirection = "sw"
	DirectionWest      WindowDirection = "w"
	DirectionNorthwest WindowDirection = "nw"
)

// WindowDistance は、窓からの距離の目安である。メートルの境目は決めず、利用者の感覚で選んでもらう。
type WindowDistance string

const (
	WindowNear   WindowDistance = "near"   // 窓際
	WindowMiddle WindowDistance = "middle" // 窓から少し離れた所
	WindowFar    WindowDistance = "far"    // 部屋の奥
)

// Shading は、窓の遮光である。
type Shading string

const (
	ShadingNone  Shading = "none"  // 遮光なし（直射日光が当たる）
	ShadingSheer Shading = "sheer" // レースカーテンなどで光を和らげている
)

// MaxLocationNameLength は、置き場所の名前の最大の文字数である。一覧の絞り込みのチップに収まる長さにする。
const MaxLocationNameLength = 30

var (
	windowDirections = []WindowDirection{DirectionNorth, DirectionNortheast, DirectionEast, DirectionSoutheast, DirectionSouth, DirectionSouthwest, DirectionWest, DirectionNorthwest}
	windowDistances  = []WindowDistance{WindowNear, WindowMiddle, WindowFar}
	shadings         = []Shading{ShadingNone, ShadingSheer}
)

// NewLocationInput は、置き場所の作成の内容を検証する。
// 名前のほかは任意で、室内か屋外かを省くと室内（室内観葉植物のアプリのため）、環境の項目の nil と空文字は未設定にする。
func NewLocationInput(name string, isIndoor *bool, direction, distance, shading *string) (LocationProfile, error) {
	v := locationValidator{fields: map[string]string{}}
	p := LocationProfile{
		Name:            v.name(name),
		IsIndoor:        isIndoor == nil || *isIndoor,
		WindowDirection: enumOf(v, "windowDirection", deref(direction), windowDirections),
		WindowDistance:  enumOf(v, "windowDistance", deref(distance), windowDistances),
		Shading:         enumOf(v, "shading", deref(shading), shadings),
	}
	if err := v.err(); err != nil {
		return LocationProfile{}, err
	}
	return p, nil
}

// LocationPatch は、検証済みの、置き場所の変更の内容である。
// nil の項目は変えない。環境の項目の空文字は、未設定に戻すことを表す。
type LocationPatch struct {
	Name            *string
	IsIndoor        *bool
	WindowDirection *WindowDirection
	WindowDistance  *WindowDistance
	Shading         *Shading
}

// NewLocationPatch は、置き場所の変更の内容を検証する。
func NewLocationPatch(name *string, isIndoor *bool, direction, distance, shading *string) (LocationPatch, error) {
	v := locationValidator{fields: map[string]string{}}
	p := LocationPatch{IsIndoor: isIndoor}
	if name != nil {
		p.Name = ptrTo(v.name(*name))
	}
	if direction != nil {
		p.WindowDirection = ptrTo(enumOf(v, "windowDirection", *direction, windowDirections))
	}
	if distance != nil {
		p.WindowDistance = ptrTo(enumOf(v, "windowDistance", *distance, windowDistances))
	}
	if shading != nil {
		p.Shading = ptrTo(enumOf(v, "shading", *shading, shadings))
	}
	if err := v.err(); err != nil {
		return LocationPatch{}, err
	}
	return p, nil
}

// locationValidator は、置き場所の項目を検証し、誤りを項目ごとに集める。
type locationValidator struct {
	fields map[string]string
}

func (v locationValidator) name(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		v.fields["name"] = "required"
	case utf8.RuneCountInString(s) > MaxLocationNameLength:
		v.fields["name"] = "too_long"
	}
	return s
}

func (v locationValidator) err() error {
	if len(v.fields) > 0 {
		return &ValidationError{Fields: v.fields}
	}
	return nil
}

// enumOf は、s が allowed のどれか、または空文字（未設定）であることを確かめる。
// Go のメソッドは型パラメーターを持てないため、メソッドではなく関数にしている。
func enumOf[T ~string](v locationValidator, field, s string, allowed []T) T {
	if s == "" {
		return ""
	}
	for _, a := range allowed {
		if T(s) == a {
			return a
		}
	}
	v.fields[field] = "invalid"
	return ""
}
