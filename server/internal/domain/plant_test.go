package domain_test

import (
	"strings"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

const (
	testSpeciesID  = "01920000-0000-7000-8000-0000000000a1"
	testLocationID = "01920000-0000-7000-8000-0000000000c1"
)

func TestNewPlantProfile(t *testing.T) {
	t.Parallel()

	t.Run("名前だけなら、種類・置き場所・購入日は未設定", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewPlantProfile(domain.PlantInput{Name: ptr(" モンステラ ")})
		if err != nil {
			t.Fatal(err)
		}
		if want := (domain.PlantProfile{Name: "モンステラ"}); got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("マスタの種類・置き場所・購入日を入れる", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewPlantProfile(domain.PlantInput{
			Name: ptr("リビングのモンステラ"), SpeciesID: ptr(testSpeciesID), LocationID: ptr(testLocationID), AcquiredOn: ptr("2026-05-03"),
		})
		if err != nil {
			t.Fatal(err)
		}
		want := domain.PlantProfile{
			Name:       "リビングのモンステラ",
			Species:    domain.PlantSpecies{ID: uuid.MustParse(testSpeciesID)},
			LocationID: uuid.MustParse(testLocationID),
			AcquiredOn: civil.Date{Year: 2026, Month: 5, Day: 3},
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("マスタにない種類は自由入力にできる", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewPlantProfile(domain.PlantInput{Name: ptr("名前のわからない多肉"), SpeciesFreeText: ptr(" 多肉植物 ")})
		if err != nil {
			t.Fatal(err)
		}
		if want := (domain.PlantSpecies{FreeText: "多肉植物"}); got.Species != want {
			t.Errorf("species = %+v, want %+v", got.Species, want)
		}
	})

	t.Run("空文字は未設定として扱う", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewPlantProfile(domain.PlantInput{
			Name: ptr("ポトス"), SpeciesID: ptr(""), SpeciesFreeText: ptr(" "), LocationID: ptr(""), AcquiredOn: ptr(""),
		})
		if err != nil {
			t.Fatal(err)
		}
		if want := (domain.PlantProfile{Name: "ポトス"}); got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	tests := []struct {
		name       string
		in         domain.PlantInput
		wantFields map[string]string
	}{
		{name: "名前がない", in: domain.PlantInput{}, wantFields: map[string]string{"name": "required"}},
		{name: "名前が空白だけ", in: domain.PlantInput{Name: ptr("  ")}, wantFields: map[string]string{"name": "required"}},
		{
			name:       "名前が長すぎる",
			in:         domain.PlantInput{Name: ptr(strings.Repeat("草", domain.MaxPlantNameLength+1))},
			wantFields: map[string]string{"name": "too_long"},
		},
		{
			name:       "自由入力の種類が長すぎる",
			in:         domain.PlantInput{Name: ptr("ポトス"), SpeciesFreeText: ptr(strings.Repeat("草", domain.MaxSpeciesFreeTextLength+1))},
			wantFields: map[string]string{"speciesFreeText": "too_long"},
		},
		{
			name:       "種類はマスタと自由入力の片方だけ",
			in:         domain.PlantInput{Name: ptr("ポトス"), SpeciesID: ptr(testSpeciesID), SpeciesFreeText: ptr("ポトス")},
			wantFields: map[string]string{"speciesFreeText": "exclusive"},
		},
		{name: "種類の ID が UUID でない", in: domain.PlantInput{Name: ptr("ポトス"), SpeciesID: ptr("pothos")}, wantFields: map[string]string{"speciesId": "invalid"}},
		{name: "置き場所の ID が UUID でない", in: domain.PlantInput{Name: ptr("ポトス"), LocationID: ptr("1")}, wantFields: map[string]string{"locationId": "invalid"}},
		{name: "購入日の形式が違う", in: domain.PlantInput{Name: ptr("ポトス"), AcquiredOn: ptr("2026/05/03")}, wantFields: map[string]string{"acquiredOn": "invalid"}},
		{name: "購入日が存在しない日", in: domain.PlantInput{Name: ptr("ポトス"), AcquiredOn: ptr("2026-02-30")}, wantFields: map[string]string{"acquiredOn": "invalid"}},
		{
			name:       "誤りはまとめて返す",
			in:         domain.PlantInput{SpeciesID: ptr("x"), LocationID: ptr("x"), AcquiredOn: ptr("x")},
			wantFields: map[string]string{"name": "required", "speciesId": "invalid", "locationId": "invalid", "acquiredOn": "invalid"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewPlantProfile(tt.in)
			assertFields(t, err, tt.wantFields)
		})
	}

	t.Run("名前の上限ちょうどは通る（文字数で数える）", func(t *testing.T) {
		t.Parallel()
		if _, err := domain.NewPlantProfile(domain.PlantInput{Name: ptr(strings.Repeat("草", domain.MaxPlantNameLength))}); err != nil {
			t.Errorf("err = %v", err)
		}
	})
}

func TestNewPlantPatch(t *testing.T) {
	t.Parallel()

	t.Run("送らなかった項目は変えない（nil のまま）", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewPlantPatch(domain.PlantInput{})
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.PlantPatch{}) {
			t.Errorf("got %+v, want empty", got)
		}
	})

	t.Run("空文字は未設定に戻すことを表す", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewPlantPatch(domain.PlantInput{LocationID: ptr(""), AcquiredOn: ptr("")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != nil || got.Species != nil || got.LocationID == nil || *got.LocationID != uuid.Nil || got.AcquiredOn == nil || !got.AcquiredOn.IsZero() {
			t.Errorf("got %+v", got)
		}
	})

	// 種類はマスタと自由入力のどちらか1つなので、片方を送ると、種類をまるごと置き換える
	speciesTests := []struct {
		name string
		in   domain.PlantInput
		want domain.PlantSpecies
	}{
		{name: "マスタの種類にすると、自由入力は消える", in: domain.PlantInput{SpeciesID: ptr(testSpeciesID)}, want: domain.PlantSpecies{ID: uuid.MustParse(testSpeciesID)}},
		{name: "自由入力にすると、マスタの紐づけは消える", in: domain.PlantInput{SpeciesFreeText: ptr("多肉植物")}, want: domain.PlantSpecies{FreeText: "多肉植物"}},
		{name: "空文字で、種類を未設定に戻す", in: domain.PlantInput{SpeciesID: ptr("")}, want: domain.PlantSpecies{}},
	}
	for _, tt := range speciesTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewPlantPatch(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Species == nil || *got.Species != tt.want {
				t.Errorf("species = %+v, want %+v", got.Species, tt.want)
			}
		})
	}

	t.Run("名前は空にできず、ほかの項目も登録と同じ規則で確かめる", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewPlantPatch(domain.PlantInput{
			Name: ptr(""), SpeciesID: ptr(testSpeciesID), SpeciesFreeText: ptr("ポトス"), LocationID: ptr("x"), AcquiredOn: ptr("x"),
		})
		assertFields(t, err, map[string]string{"name": "required", "speciesFreeText": "exclusive", "locationId": "invalid", "acquiredOn": "invalid"})
	})
}
