package domain_test

import (
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNewLocationInput(t *testing.T) {
	t.Parallel()

	t.Run("名前だけなら、室内で、環境情報は未設定", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewLocationInput(" リビングの南窓 ", nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := domain.LocationProfile{Name: "リビングの南窓", IsIndoor: true}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("環境情報をすべて入れる", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewLocationInput("ベランダ", ptr(false), ptr("se"), ptr("near"), ptr("sheer"))
		if err != nil {
			t.Fatal(err)
		}
		want := domain.LocationProfile{
			Name: "ベランダ", IsIndoor: false,
			WindowDirection: domain.DirectionSoutheast, WindowDistance: domain.WindowNear, Shading: domain.ShadingSheer,
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("空文字は未設定として扱う", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewLocationInput("寝室", nil, ptr(""), ptr(""), ptr(""))
		if err != nil {
			t.Fatal(err)
		}
		if got.WindowDirection != "" || got.WindowDistance != "" || got.Shading != "" {
			t.Errorf("got %+v, want unset", got)
		}
	})

	tests := []struct {
		name       string
		locName    string
		direction  *string
		distance   *string
		shading    *string
		wantFields map[string]string
	}{
		{name: "名前が空", locName: "  ", wantFields: map[string]string{"name": "required"}},
		{name: "名前が長すぎる", locName: strings.Repeat("窓", domain.MaxLocationNameLength+1), wantFields: map[string]string{"name": "too_long"}},
		{name: "知らない向き", locName: "玄関", direction: ptr("south"), wantFields: map[string]string{"windowDirection": "invalid"}},
		{name: "知らない距離", locName: "玄関", distance: ptr("1m"), wantFields: map[string]string{"windowDistance": "invalid"}},
		{name: "知らない遮光", locName: "玄関", shading: ptr("curtain"), wantFields: map[string]string{"shading": "invalid"}},
		{
			name: "誤りはまとめて返す", locName: "", direction: ptr("N"), distance: ptr("x"), shading: ptr("x"),
			wantFields: map[string]string{"name": "required", "windowDirection": "invalid", "windowDistance": "invalid", "shading": "invalid"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewLocationInput(tt.locName, nil, tt.direction, tt.distance, tt.shading)
			assertFields(t, err, tt.wantFields)
		})
	}

	t.Run("名前の上限ちょうどは通る（文字数で数える）", func(t *testing.T) {
		t.Parallel()
		if _, err := domain.NewLocationInput(strings.Repeat("窓", domain.MaxLocationNameLength), nil, nil, nil, nil); err != nil {
			t.Errorf("err = %v", err)
		}
	})
}

func TestNewLocationPatch(t *testing.T) {
	t.Parallel()

	t.Run("送らなかった項目は変えない（nil のまま）", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewLocationPatch(nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.LocationPatch{}) {
			t.Errorf("got %+v, want empty", got)
		}
	})

	t.Run("空文字は未設定に戻すことを表す", func(t *testing.T) {
		t.Parallel()
		got, err := domain.NewLocationPatch(nil, nil, ptr(""), nil, ptr("none"))
		if err != nil {
			t.Fatal(err)
		}
		if got.WindowDirection == nil || *got.WindowDirection != "" || got.WindowDistance != nil || got.Shading == nil || *got.Shading != domain.ShadingNone {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("名前は空にできない", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewLocationPatch(ptr(""), ptr(true), ptr("x"), nil, nil)
		assertFields(t, err, map[string]string{"name": "required", "windowDirection": "invalid"})
	})
}
