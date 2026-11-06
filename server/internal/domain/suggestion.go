package domain

import (
	"encoding/json"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// 提案への対応の衝突のコード。
const (
	// ConflictAlreadyResponded は、提案がすでに採用・見送りされていることを表す。
	ConflictAlreadyResponded = "already_responded"
	// ConflictSuggestionExpired は、提案の有効期間が過ぎていることを表す。
	ConflictSuggestionExpired = "suggestion_expired"
)

// SuggestionKind は、提案の種類である（要件19-2）。
type SuggestionKind string

const (
	SuggestionDo          SuggestionKind = "do"          // 実施をすすめる
	SuggestionAvoid       SuggestionKind = "avoid"       // やらない方がよい
	SuggestionConditional SuggestionKind = "conditional" // 条件つき（「もし〜なら」）
)

// SuggestionAction は、提案に対してアプリが取れる行動である。
type SuggestionAction string

const (
	SuggestionCreateTask SuggestionAction = "create_task" // 採用すると、タスクを作る
	SuggestionSkipToday  SuggestionAction = "skip_today"  // 今日はスキップをすすめる（スキップはアプリが記録の API で行う）
	SuggestionInfo       SuggestionAction = "info"        // 知らせるだけ
)

// SuggestionStatus は、提案へのユーザーの対応である（要件19-6）。
type SuggestionStatus string

const (
	SuggestionPending   SuggestionStatus = "pending"
	SuggestionAccepted  SuggestionStatus = "accepted"
	SuggestionDismissed SuggestionStatus = "dismissed"
)

// Suggestion は、助言プロバイダの提案である（要件19-3・19-6）。本文と理由は i18n のキーとパラメーターで持ち、文言はアプリが引く。
type Suggestion struct {
	ID            uuid.UUID
	HouseholdID   uuid.UUID
	PlantID       uuid.UUID // 植物への提案でなければ uuid.Nil
	TaskID        uuid.UUID // タスクへの提案でなければ uuid.Nil
	ProviderID    string    // 提案元（例：repotting.v1）
	Kind          SuggestionKind
	Action        SuggestionAction
	TaskKind      CareKind   // create_task で作るタスクの種類
	TaskSchedule  Schedule   // create_task で作るタスクの周期
	ProposedDate  civil.Date // create_task で作るタスクの予定日
	ConditionText string     // 条件つきの提案の「もし〜なら」
	MessageKey    string
	MessageParams json.RawMessage
	ReasonKey     string
	ReasonParams  json.RawMessage
	ValidFrom     civil.Date
	ValidUntil    civil.Date // 期限がなければゼロ値
	DedupeKey     string     // 世帯の中で同じキーの提案は1件だけ。見送ったあとも作り直さない（要件17-6）
	Status        SuggestionStatus
	RespondedBy   uuid.NullUUID // 対応したユーザー
	RespondedAt   time.Time     // 対応していなければゼロ値
	CreatedAt     time.Time
}

// ActiveOn は、提案が d の日に有効か（未回答で、有効期間の中か）を返す。
func (s Suggestion) ActiveOn(d civil.Date) bool {
	return s.Status == SuggestionPending && !d.Before(s.ValidFrom) && !s.expiredOn(d)
}

func (s Suggestion) expiredOn(d civil.Date) bool {
	return !s.ValidUntil.IsZero() && d.After(s.ValidUntil)
}

// AcceptInput は、提案の採用の入力（未検証）である。
type AcceptInput struct {
	Date *string // create_task の提案で、提案日を変えて採用するときの日付（要件17-5）
}

// Acceptance は、提案を採用したときの変更の内容である。
type Acceptance struct {
	Task *CareTaskDraft // 作るタスク。タスクを作らない提案では nil
}

// PlanAcceptance は、提案を採用できるかを確かめ、採用したときの変更を決める（要件17-4・17-5）。
//   - 未回答でなければ already_responded、有効期間を過ぎていれば suggestion_expired
//   - create_task は、提案日（日付を送れば、その日）を予定日にしたタスクを作る
//   - ほかの行動は、対応を記録するだけ。日付は送れない
func PlanAcceptance(s Suggestion, in AcceptInput, today civil.Date) (Acceptance, error) {
	if s.Status != SuggestionPending {
		return Acceptance{}, &ConflictError{Code: ConflictAlreadyResponded}
	}
	if s.expiredOn(today) {
		return Acceptance{}, &ConflictError{Code: ConflictSuggestionExpired}
	}
	date := s.ProposedDate
	if v := strings.TrimSpace(deref(in.Date)); v != "" {
		d, err := civil.ParseDate(v)
		if err != nil || !d.IsValid() || s.Action != SuggestionCreateTask {
			return Acceptance{}, &ValidationError{Fields: map[string]string{"date": "invalid"}}
		}
		date = d
	}
	if s.Action != SuggestionCreateTask {
		return Acceptance{}, nil
	}
	if date.IsZero() {
		date = today
	}
	return Acceptance{Task: &CareTaskDraft{
		CareTaskProfile: CareTaskProfile{Kind: s.TaskKind, Schedule: s.TaskSchedule},
		NextDueOn:       date,
	}}, nil
}

// CheckDismiss は、提案を見送れるかを確かめる。未回答でなければ already_responded にする。
// 有効期間を過ぎた提案も見送れる（アプリの表示から消すだけなので、害がないため）。
func CheckDismiss(s Suggestion) error {
	if s.Status != SuggestionPending {
		return &ConflictError{Code: ConflictAlreadyResponded}
	}
	return nil
}
