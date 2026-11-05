package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestNewNoteBody(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		in   *string
		want string
	}{
		{"本文をそのまま使う", ptr("葉の裏にハダニ。シャワーで流した"), "葉の裏にハダニ。シャワーで流した"},
		{"前後の空白を除く", ptr("  新芽が出た\n"), "新芽が出た"},
		{"途中の改行は残す", ptr("1行目\n2行目"), "1行目\n2行目"},
		{"上限ちょうどの文字数", ptr(strings.Repeat("葉", domain.MaxNoteBodyLength)), strings.Repeat("葉", domain.MaxNoteBodyLength)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewNoteBody(tt.in)
			if err != nil || got != tt.want {
				t.Errorf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		name string
		in   *string
		want string
	}{
		{"省くと required", nil, "required"},
		{"空白だけも required", ptr(" \n "), "required"},
		// 上限は、バイト数ではなく文字数で数える（データベースの char_length と同じ）
		{"上限を超えると too_long", ptr(strings.Repeat("葉", domain.MaxNoteBodyLength+1)), "too_long"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewNoteBody(tt.in)
			var v *domain.ValidationError
			if !errors.As(err, &v) || !reflect.DeepEqual(v.Fields, map[string]string{"body": tt.want}) {
				t.Errorf("err = %v; want body=%s", err, tt.want)
			}
		})
	}
}

func TestNewNoteQuery(t *testing.T) {
	t.Parallel()

	t.Run("省くと先頭から既定の件数", func(t *testing.T) {
		t.Parallel()
		q, err := domain.NewNoteQuery(domain.NoteQueryInput{})
		if err != nil || q.After != nil || q.Limit != domain.DefaultNotePageSize {
			t.Errorf("got %+v, %v", q, err)
		}
	})

	t.Run("前のページの最後のメモから続ける", func(t *testing.T) {
		t.Parallel()
		last := domain.Note{ID: uuid.New(), CreatedAt: time.Date(2026, 11, 5, 9, 30, 0, 123456000, time.UTC)}
		cursor := domain.NoteCursorOf(last).Encode()
		q, err := domain.NewNoteQuery(domain.NoteQueryInput{Cursor: &cursor, Limit: ptr(10)})
		if err != nil || q.After == nil || q.Limit != 10 {
			t.Fatalf("got %+v, %v", q, err)
		}
		if q.After.ID != last.ID || !q.After.CreatedAt.Equal(last.CreatedAt) {
			t.Errorf("after = %+v; want %+v", q.After, last)
		}
	})

	t.Run("読めない位置と範囲の外の件数は 422", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewNoteQuery(domain.NoteQueryInput{Cursor: ptr("xyz"), Limit: ptr(domain.MaxNotePageSize + 1)})
		var v *domain.ValidationError
		if !errors.As(err, &v) || !reflect.DeepEqual(v.Fields, map[string]string{"cursor": "invalid", "limit": "invalid"}) {
			t.Errorf("err = %v", err)
		}
	})
}
