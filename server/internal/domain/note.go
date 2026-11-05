package domain

import (
	"encoding/base64"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Note は、植物ごとのテキストのメモである（要件13）。作成者の名前は、作成の時点の値を持つ（記録と同じく、退出のあとも表示するため）。
type Note struct {
	ID                uuid.UUID
	HouseholdID       uuid.UUID
	PlantID           uuid.UUID
	AuthorUserID      uuid.NullUUID // アカウントを削除していれば無効
	AuthorDisplayName string
	Body              string
	EditedAt          time.Time // 編集していなければゼロ値（要件13-3）
	CreatedAt         time.Time
}

// MaxNoteBodyLength は、メモの本文の上限（文字数）である（要件13-4）。データベースの CHECK 制約と同じ値にする。
const MaxNoteBodyLength = 2000

// NewNoteBody は、メモの本文を検証する。前後の空白は除き、空なら required、上限を超えれば too_long にする。
func NewNoteBody(in *string) (string, error) {
	body := strings.TrimSpace(deref(in))
	switch {
	case body == "":
		return "", &ValidationError{Fields: map[string]string{"body": "required"}}
	case utf8.RuneCountInString(body) > MaxNoteBodyLength:
		return "", &ValidationError{Fields: map[string]string{"body": "too_long"}}
	}
	return body, nil
}

// メモの一覧のページの件数。
const (
	DefaultNotePageSize = 30
	MaxNotePageSize     = 100
)

// NoteCursor は、メモの一覧の並び（作成日時・ID の新しい順）での位置である。
type NoteCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// NoteCursorOf は、メモの位置を返す。
func NoteCursorOf(n Note) NoteCursor {
	return NoteCursor{CreatedAt: n.CreatedAt, ID: n.ID}
}

// Encode は、位置をアプリに渡す文字列にする。アプリは中身を解釈せず、そのまま送り返す。
func (c NoteCursor) Encode() string {
	s := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func decodeNoteCursor(s string) (NoteCursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return NoteCursor{}, false
	}
	at, id, ok := strings.Cut(string(b), "|")
	if !ok {
		return NoteCursor{}, false
	}
	t, err1 := time.Parse(time.RFC3339Nano, at)
	u, err2 := uuid.Parse(id)
	if err1 != nil || err2 != nil {
		return NoteCursor{}, false
	}
	return NoteCursor{CreatedAt: t, ID: u}, true
}

// NoteQueryInput は、メモの一覧の取得の条件（未検証）である。
type NoteQueryInput struct {
	Cursor *string
	Limit  *int
}

// NoteQuery は、検証済みのメモの一覧の取得の条件である。
type NoteQuery struct {
	After *NoteCursor // nil なら先頭から
	Limit int
}

// NewNoteQuery は、メモの一覧の取得の条件を検証する。件数を省くと DefaultNotePageSize にする。
func NewNoteQuery(in NoteQueryInput) (NoteQuery, error) {
	fields := map[string]string{}
	q := NoteQuery{Limit: DefaultNotePageSize}
	if s := deref(in.Cursor); s != "" {
		if c, ok := decodeNoteCursor(s); ok {
			q.After = &c
		} else {
			fields["cursor"] = "invalid"
		}
	}
	if in.Limit != nil {
		q.Limit = *in.Limit
		if q.Limit < 1 || q.Limit > MaxNotePageSize {
			fields["limit"] = "invalid"
		}
	}
	if len(fields) > 0 {
		return NoteQuery{}, &ValidationError{Fields: fields}
	}
	return q, nil
}

// NotePage は、メモの一覧の1ページである。
type NotePage struct {
	Notes      []Note
	NextCursor string // 次のページがなければ ""
}
