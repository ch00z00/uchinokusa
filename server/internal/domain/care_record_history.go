package domain

import (
	"encoding/base64"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// RecordPatchInput は、記録の編集の入力（未検証）である。変えられるのは実施日だけ。
type RecordPatchInput struct {
	PerformedOn *string
}

// RecordPatch は、検証済みの記録の編集である。
type RecordPatch struct {
	PerformedOn civil.Date
}

// NewRecordPatch は、記録の編集を検証する。完了と同じく、未来の実施日は受け付けない。
func NewRecordPatch(in RecordPatchInput, today civil.Date) (RecordPatch, error) {
	s := strings.TrimSpace(deref(in.PerformedOn))
	if s == "" {
		return RecordPatch{}, &ValidationError{Fields: map[string]string{"performedOn": "required"}}
	}
	d, err := civil.ParseDate(s)
	if err != nil || !d.IsValid() || d.After(today) {
		return RecordPatch{}, &ValidationError{Fields: map[string]string{"performedOn": "invalid"}}
	}
	return RecordPatch{PerformedOn: d}, nil
}

// RecordChange は、記録の編集・削除の前後の、タスクの記録の状況である。
type RecordChange struct {
	Target        CareRecord  // 編集・削除する記録（変更前）
	ScheduleOwner CareRecord  // 変更前に、タスクの予定を最後に決めた記録（追加の記録を除き、記録した順で最後のもの）
	LatestBefore  CareRecord  // 変更前の、最新の完了（実施日の新しい順）
	LatestAfter   *CareRecord // 変更後の、最新の完了。完了が残らなければ nil
}

// RescheduleAfterChange は、記録の編集・削除のあとの次回予定を決める（要件12-4）。変えなければ false を返す。
// 計算し直すのは、変えた記録が変更の前か後で最新の完了であり、タスクの予定を最後に決めたのが完了の場合だけ。
//   - あとのスキップが予定を決めていれば、ユーザーが選んだ確認日を残す
//   - 完了が残らなければ、計算の起点がないので、予定を変えない
func RescheduleAfterChange(t CareTask, ch RecordChange, h Hemisphere) (civil.Date, bool) {
	if ch.Target.Outcome != OutcomeCompleted || ch.ScheduleOwner.Outcome != OutcomeCompleted || ch.LatestAfter == nil {
		return civil.Date{}, false
	}
	if ch.Target.ID != ch.LatestBefore.ID && ch.Target.ID != ch.LatestAfter.ID {
		return civil.Date{}, false
	}
	return t.Schedule.NextDueAfterCompletion(ch.LatestAfter.PerformedOn, h), true
}

// 履歴のページの件数。
const (
	DefaultRecordPageSize = 30
	MaxRecordPageSize     = 100
)

// RecordCursor は、履歴の並び（実施日・記録した日時・ID の新しい順）での位置である。次のページは、この位置より後から始める。
type RecordCursor struct {
	PerformedOn civil.Date
	CreatedAt   time.Time
	ID          uuid.UUID
}

// CursorOf は、記録の位置を返す。
func CursorOf(r CareRecord) RecordCursor {
	return RecordCursor{PerformedOn: r.PerformedOn, CreatedAt: r.CreatedAt, ID: r.ID}
}

// Encode は、位置をアプリに渡す文字列にする。アプリは中身を解釈せず、そのまま送り返す。
func (c RecordCursor) Encode() string {
	s := c.PerformedOn.String() + "|" + c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func decodeRecordCursor(s string) (RecordCursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return RecordCursor{}, false
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 3 {
		return RecordCursor{}, false
	}
	d, err1 := civil.ParseDate(parts[0])
	at, err2 := time.Parse(time.RFC3339Nano, parts[1])
	id, err3 := uuid.Parse(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return RecordCursor{}, false
	}
	return RecordCursor{PerformedOn: d, CreatedAt: at, ID: id}, true
}

// RecordQueryInput は、履歴の取得の条件（未検証）である。
type RecordQueryInput struct {
	Kind   *string
	Cursor *string
	Limit  *int
}

// RecordQuery は、検証済みの履歴の取得の条件である。
type RecordQuery struct {
	Kind  CareKind      // "" ならすべての種類（要件12-2）
	After *RecordCursor // nil なら先頭から
	Limit int
}

// NewRecordQuery は、履歴の取得の条件を検証する。件数を省くと DefaultRecordPageSize にする。
func NewRecordQuery(in RecordQueryInput) (RecordQuery, error) {
	fields := map[string]string{}
	q := RecordQuery{Limit: DefaultRecordPageSize}
	if k := deref(in.Kind); k != "" {
		q.Kind = CareKind(k)
		if !isCareKind(q.Kind) {
			fields["kind"] = "invalid"
		}
	}
	if s := deref(in.Cursor); s != "" {
		if c, ok := decodeRecordCursor(s); ok {
			q.After = &c
		} else {
			fields["cursor"] = "invalid"
		}
	}
	if in.Limit != nil {
		q.Limit = *in.Limit
		if q.Limit < 1 || q.Limit > MaxRecordPageSize {
			fields["limit"] = "invalid"
		}
	}
	if len(fields) > 0 {
		return RecordQuery{}, &ValidationError{Fields: fields}
	}
	return q, nil
}

func isCareKind(k CareKind) bool {
	for _, c := range careKinds {
		if k == c {
			return true
		}
	}
	return false
}

// LastPerformed は、作業の種類ごとの最終実施日である（要件12-3）。種類が other のときは、作業名ごとに分ける。
type LastPerformed struct {
	Kind        CareKind
	CustomName  string
	PerformedOn civil.Date
}

// RecordPage は、履歴の1ページである。
type RecordPage struct {
	Records       []CareRecord
	NextCursor    string // 次のページがなければ ""
	LastPerformed []LastPerformed
}
