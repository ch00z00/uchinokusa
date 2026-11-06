// Package advisor は、提案を作る助言プロバイダの共通の取り決めと、その登録の仕組みである（設計の判断3、要件19-1・19-5）。
// プロバイダは、植物とその周りの情報（Context）を受け取って提案（Proposal）を返すだけで、保存や採用・見送りは
// app.SuggestionService が共通で行う。新しいプロバイダ（フェーズ2の天気の助言など）は、Advisor を実装して登録すれば加えられる。
package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// Trigger は、プロバイダを呼ぶきっかけである。
type Trigger string

const (
	TriggerPlantCreated   Trigger = "plant_created"    // 植物の登録
	TriggerTaskDueChanged Trigger = "task_due_changed" // タスクの予定日の変更（Context.TaskID が対象のタスク）
	TriggerDaily          Trigger = "daily"            // 日次のジョブ（MVP では呼ぶプロバイダがない）
)

// Subject は、提案の対象の植物と、その周りの情報である（要件19-1）。
type Subject struct {
	Household domain.Household
	Plant     domain.Plant
	Species   *domain.Species  // 植物マスタと紐づかなければ nil
	Location  *domain.Location // 置き場所が未設定なら nil
	Tasks     []domain.CareTask
	Records   []domain.CareRecord // 最近の記録（新しい順）。スキップの理由を含む
}

// Context は、プロバイダに渡す情報である。
type Context struct {
	Trigger Trigger
	Today   civil.Date // 世帯のタイムゾーンでの今日
	TaskID  uuid.UUID  // task_due_changed の対象のタスク。ほかのきっかけでは uuid.Nil
	Subject
}

// Proposal は、プロバイダが返す提案である。保存すると domain.Suggestion になる。
type Proposal struct {
	Kind          domain.SuggestionKind
	Action        domain.SuggestionAction
	TaskKind      domain.CareKind // create_task で作るタスクの種類
	TaskSchedule  domain.Schedule // create_task で作るタスクの周期
	TargetTaskID  uuid.UUID       // avoid・conditional の対象のタスク。植物への提案なら uuid.Nil
	ProposedDate  civil.Date      // create_task で作るタスクの予定日
	ConditionText string          // conditional の「もし〜なら」
	MessageKey    string
	MessageParams map[string]any
	ReasonKey     string
	ReasonParams  map[string]any
	ValidFrom     civil.Date
	ValidUntil    civil.Date // 期限がなければゼロ値
	// DedupeKey は、同じ提案を見分けるキーである（例：repotting:initial:{plantID}）。世帯の中で同じキーの提案は二度作らない。
	DedupeKey string
}

// Validate は、提案が保存できる形かを確かめる。プロバイダの誤りで、表示も採用もできない提案を保存しないため。
func (p Proposal) Validate() error {
	var errs []error
	switch p.Kind {
	case domain.SuggestionDo, domain.SuggestionAvoid, domain.SuggestionConditional:
	default:
		errs = append(errs, fmt.Errorf("unknown kind %q", p.Kind))
	}
	switch p.Action {
	case domain.SuggestionCreateTask:
		if p.TaskKind == "" || p.TaskSchedule.Base.Value < 1 {
			errs = append(errs, errors.New("create_task needs task kind and schedule"))
		}
	case domain.SuggestionSkipToday, domain.SuggestionInfo:
	default:
		errs = append(errs, fmt.Errorf("unknown action %q", p.Action))
	}
	if p.Kind == domain.SuggestionConditional && p.ConditionText == "" {
		errs = append(errs, errors.New("conditional needs condition text"))
	}
	if p.MessageKey == "" {
		errs = append(errs, errors.New("message key is empty"))
	}
	if p.ReasonKey == "" {
		errs = append(errs, errors.New("reason key is empty"))
	}
	if p.DedupeKey == "" {
		errs = append(errs, errors.New("dedupe key is empty"))
	}
	if p.ValidFrom.IsZero() || (!p.ValidUntil.IsZero() && p.ValidUntil.Before(p.ValidFrom)) {
		errs = append(errs, errors.New("invalid valid period"))
	}
	return errors.Join(errs...)
}

// Suggestion は、提案を、保存する前の domain.Suggestion にする。
func (p Proposal) Suggestion(providerID string, householdID, plantID uuid.UUID) (domain.Suggestion, error) {
	message, err := paramsJSON(p.MessageParams)
	if err != nil {
		return domain.Suggestion{}, fmt.Errorf("message params: %w", err)
	}
	reason, err := paramsJSON(p.ReasonParams)
	if err != nil {
		return domain.Suggestion{}, fmt.Errorf("reason params: %w", err)
	}
	return domain.Suggestion{
		HouseholdID:   householdID,
		PlantID:       plantID,
		TaskID:        p.TargetTaskID,
		ProviderID:    providerID,
		Kind:          p.Kind,
		Action:        p.Action,
		TaskKind:      p.TaskKind,
		TaskSchedule:  p.TaskSchedule,
		ProposedDate:  p.ProposedDate,
		ConditionText: p.ConditionText,
		MessageKey:    p.MessageKey,
		MessageParams: message,
		ReasonKey:     p.ReasonKey,
		ReasonParams:  reason,
		ValidFrom:     p.ValidFrom,
		ValidUntil:    p.ValidUntil,
		DedupeKey:     p.DedupeKey,
		Status:        domain.SuggestionPending,
	}, nil
}

// paramsJSON は、i18n のパラメーターを JSON にする。パラメーターがなければ {} にする。
func paramsJSON(params map[string]any) (json.RawMessage, error) {
	if len(params) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.Marshal(params)
}

// Advisor は、助言プロバイダである。
type Advisor interface {
	// ID は、プロバイダの ID（例：repotting.v1）である。提案の提案元として保存する（要件19-6）。
	ID() string
	// Triggers は、このプロバイダを呼ぶきっかけである。
	Triggers() []Trigger
	// Advise は、提案を返す。提案がなければ空。エラーを返しても、元の操作（植物の登録など）は失敗しない。
	Advise(ctx context.Context, in Context) ([]Proposal, error)
}

// Registry は、登録したプロバイダの集まりである。
type Registry struct {
	advisors []Advisor
}

// NewRegistry は、プロバイダを登録する。ID が空・重複している場合はエラーにする（提案元を見分けられなくなるため）。
func NewRegistry(advisors ...Advisor) (*Registry, error) {
	seen := map[string]bool{}
	for _, a := range advisors {
		id := a.ID()
		if id == "" {
			return nil, errors.New("advisor id is empty")
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate advisor id %q", id)
		}
		seen[id] = true
	}
	return &Registry{advisors: advisors}, nil
}

// For は、きっかけで呼ぶプロバイダを、登録した順に返す。
func (r *Registry) For(t Trigger) []Advisor {
	var out []Advisor
	for _, a := range r.advisors {
		for _, at := range a.Triggers() {
			if at == t {
				out = append(out, a)
				break
			}
		}
	}
	return out
}
