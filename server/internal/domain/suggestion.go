package domain

import (
	"encoding/json"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// SuggestionKind は、提案の種類である。
type SuggestionKind string

const (
	SuggestionDo          SuggestionKind = "do"          // 実施をすすめる
	SuggestionAvoid       SuggestionKind = "avoid"       // やらない方がよい
	SuggestionConditional SuggestionKind = "conditional" // 条件つき（「もし〜なら」）
)

// SuggestionAction は、提案に対してアプリが取れる行動である。
type SuggestionAction string

const (
	SuggestionCreateTask SuggestionAction = "create_task"
	SuggestionSkipToday  SuggestionAction = "skip_today"
	SuggestionInfo       SuggestionAction = "info"
)

// Suggestion は、助言プロバイダの提案である。本文と理由は i18n のキーとパラメーターで持ち、文言はアプリが引く。
// 提案を作る仕組みはタスク 10.x で加える。
type Suggestion struct {
	ID            uuid.UUID
	PlantID       uuid.UUID
	TaskID        uuid.UUID
	ProviderID    string
	Kind          SuggestionKind
	Action        SuggestionAction
	ConditionText string
	MessageKey    string
	MessageParams json.RawMessage
	ReasonKey     string
	ReasonParams  json.RawMessage
	ValidFrom     civil.Date
	ValidUntil    civil.Date // 期限がなければゼロ値
}
