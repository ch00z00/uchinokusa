package domain_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func ptr(s string) *string { return &s }

func TestNewHouseholdInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inName     string
		prefecture *string
		city       *string
		want       domain.NewHousehold
		wantFields map[string]string // nil なら成功
	}{
		{name: "名前だけ", inName: "自宅", want: domain.NewHousehold{Name: "自宅"}},
		{name: "地域つき（前後の空白を除く）", inName: " 実家 ", prefecture: ptr(" 長野県"), city: ptr("松本市 "), want: domain.NewHousehold{Name: "実家", Region: domain.Region{Prefecture: "長野県", City: "松本市"}}},
		{name: "地域の空文字は未設定", inName: "自宅", prefecture: ptr(""), city: ptr("  "), want: domain.NewHousehold{Name: "自宅"}},
		{name: "名前は30文字まで", inName: strings.Repeat("あ", 30), want: domain.NewHousehold{Name: strings.Repeat("あ", 30)}},
		{name: "名前が空", inName: " 　", wantFields: map[string]string{"name": "required"}},
		{name: "名前が長すぎる", inName: strings.Repeat("あ", 31), wantFields: map[string]string{"name": "too_long"}},
		{
			name: "複数の誤りは、まとめて返す", inName: "", prefecture: ptr(strings.Repeat("あ", 51)), city: ptr(strings.Repeat("い", 51)),
			wantFields: map[string]string{"name": "required", "regionPrefecture": "too_long", "regionCity": "too_long"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewHouseholdInput(tt.inName, tt.prefecture, tt.city)
			assertFields(t, err, tt.wantFields)
			if tt.wantFields == nil && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewHouseholdPatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inName     *string
		prefecture *string
		city       *string
		want       domain.HouseholdPatch
		wantFields map[string]string
	}{
		{name: "何も送らなければ、何も変えない", want: domain.HouseholdPatch{}},
		{name: "名前だけ変える", inName: ptr(" 自宅2 "), want: domain.HouseholdPatch{Name: ptr("自宅2")}},
		{name: "地域を変える", prefecture: ptr("東京都"), city: ptr("世田谷区"), want: domain.HouseholdPatch{Prefecture: ptr("東京都"), City: ptr("世田谷区")}},
		// 空文字（空白だけを含む）は「未設定に戻す」。項目がない（nil）の「変えない」とは区別する
		{name: "地域の空文字は、未設定に戻す", prefecture: ptr(" "), want: domain.HouseholdPatch{Prefecture: ptr("")}},
		{name: "名前を空にはできない", inName: ptr(""), wantFields: map[string]string{"name": "required"}},
		{name: "地域が長すぎる", city: ptr(strings.Repeat("あ", 51)), wantFields: map[string]string{"regionCity": "too_long"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewHouseholdPatch(tt.inName, tt.prefecture, tt.city)
			assertFields(t, err, tt.wantFields)
			if tt.wantFields != nil {
				return
			}
			if !equalPtr(got.Name, tt.want.Name) || !equalPtr(got.Prefecture, tt.want.Prefecture) || !equalPtr(got.City, tt.want.City) {
				t.Errorf("got %s, want %s", formatPatch(got), formatPatch(tt.want))
			}
		})
	}
}

// assertFields は、wantFields が nil なら成功を、そうでなければ ValidationError の fields を確かめる。
func assertFields(t *testing.T, err error, wantFields map[string]string) {
	t.Helper()
	if wantFields == nil {
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		return
	}
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if !maps.Equal(verr.Fields, wantFields) {
		t.Errorf("fields = %v, want %v", verr.Fields, wantFields)
	}
}

func equalPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func formatPatch(p domain.HouseholdPatch) string {
	f := func(s *string) string {
		if s == nil {
			return "<nil>"
		}
		return "\"" + *s + "\""
	}
	return "{Name:" + f(p.Name) + " Prefecture:" + f(p.Prefecture) + " City:" + f(p.City) + "}"
}
