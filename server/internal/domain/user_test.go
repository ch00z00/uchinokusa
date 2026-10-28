package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNormalizeDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         string
		want       string
		wantReason string // ValidationError の fields["displayName"]。空なら成功
	}{
		{name: "そのまま使う", in: "ゆうすけ", want: "ゆうすけ"},
		{name: "前後の空白（全角を含む）を除く", in: " 　ゆうすけ\t", want: "ゆうすけ"},
		{name: "30文字ちょうどは使える", in: strings.Repeat("あ", 30), want: strings.Repeat("あ", 30)},
		{name: "空は使えない", in: "", wantReason: "required"},
		{name: "空白だけは使えない", in: " 　 ", wantReason: "required"},
		{name: "31文字は長すぎる（バイト数ではなく文字数で数える）", in: strings.Repeat("あ", 31), wantReason: "too_long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NormalizeDisplayName(tt.in)
			if tt.wantReason == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				if got != tt.want {
					t.Errorf("got %q, want %q", got, tt.want)
				}
				return
			}
			var verr *domain.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if verr.Fields["displayName"] != tt.wantReason {
				t.Errorf("fields = %v, want displayName=%s", verr.Fields, tt.wantReason)
			}
		})
	}
}
