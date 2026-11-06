package advisor_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// stubAdvisor は、決まった提案を返すプロバイダである。
type stubAdvisor struct {
	id       string
	triggers []advisor.Trigger
}

func (a stubAdvisor) ID() string                  { return a.id }
func (a stubAdvisor) Triggers() []advisor.Trigger { return a.triggers }
func (a stubAdvisor) Advise(context.Context, advisor.Context) ([]advisor.Proposal, error) {
	return nil, nil
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	t.Run("きっかけごとに、登録した順にプロバイダを返す", func(t *testing.T) {
		t.Parallel()
		a := stubAdvisor{id: "a.v1", triggers: []advisor.Trigger{advisor.TriggerPlantCreated, advisor.TriggerTaskDueChanged}}
		b := stubAdvisor{id: "b.v1", triggers: []advisor.Trigger{advisor.TriggerDaily}}
		c := stubAdvisor{id: "c.v1", triggers: []advisor.Trigger{advisor.TriggerPlantCreated}}
		r, err := advisor.NewRegistry(a, b, c)
		if err != nil {
			t.Fatal(err)
		}
		ids := func(list []advisor.Advisor) []string {
			out := []string{}
			for _, x := range list {
				out = append(out, x.ID())
			}
			return out
		}
		if got := ids(r.For(advisor.TriggerPlantCreated)); !slices.Equal(got, []string{"a.v1", "c.v1"}) {
			t.Errorf("plant_created = %v", got)
		}
		if got := ids(r.For(advisor.TriggerDaily)); !slices.Equal(got, []string{"b.v1"}) {
			t.Errorf("daily = %v", got)
		}
	})

	t.Run("同じ ID のプロバイダは登録できない", func(t *testing.T) {
		t.Parallel()
		a := stubAdvisor{id: "a.v1"}
		if _, err := advisor.NewRegistry(a, a); err == nil {
			t.Error("err = nil")
		}
	})

	t.Run("空の ID は登録できない", func(t *testing.T) {
		t.Parallel()
		if _, err := advisor.NewRegistry(stubAdvisor{}); err == nil {
			t.Error("err = nil")
		}
	})
}

func validProposal() advisor.Proposal {
	return advisor.Proposal{
		Kind:         domain.SuggestionDo,
		Action:       domain.SuggestionCreateTask,
		TaskKind:     domain.CareRepotting,
		TaskSchedule: domain.Schedule{Base: domain.Interval{Value: 12, Unit: domain.IntervalMonth}},
		ProposedDate: civil.Date{Year: 2027, Month: 5, Day: 1},
		MessageKey:   "repotting.initial",
		ReasonKey:    "repotting.initial.reason",
		ReasonParams: map[string]any{"species": "モンステラ"},
		ValidFrom:    civil.Date{Year: 2026, Month: 11, Day: 6},
		DedupeKey:    "repotting:initial:p1",
	}
}

func TestProposal_Validate(t *testing.T) {
	t.Parallel()
	if err := validProposal().Validate(); err != nil {
		t.Errorf("valid: err = %v", err)
	}

	for _, tt := range []struct {
		name   string
		modify func(*advisor.Proposal)
		want   string
	}{
		{"知らない種類", func(p *advisor.Proposal) { p.Kind = "maybe" }, "kind"},
		{"知らない行動", func(p *advisor.Proposal) { p.Action = "call" }, "action"},
		{"本文のキーがない", func(p *advisor.Proposal) { p.MessageKey = "" }, "message"},
		{"理由のキーがない", func(p *advisor.Proposal) { p.ReasonKey = "" }, "reason"},
		{"重複防止のキーがない", func(p *advisor.Proposal) { p.DedupeKey = "" }, "dedupe"},
		{"有効期間の始まりがない", func(p *advisor.Proposal) { p.ValidFrom = civil.Date{} }, "valid"},
		{"有効期間が逆", func(p *advisor.Proposal) { p.ValidUntil = civil.Date{Year: 2026, Month: 11, Day: 5} }, "valid"},
		{"タスクを作る提案に、作業の種類がない", func(p *advisor.Proposal) { p.TaskKind = "" }, "task"},
		{"タスクを作る提案に、周期がない", func(p *advisor.Proposal) { p.TaskSchedule = domain.Schedule{} }, "task"},
		{"条件つきの提案に、条件がない", func(p *advisor.Proposal) { p.Kind, p.Action = domain.SuggestionConditional, domain.SuggestionInfo }, "condition"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := validProposal()
			tt.modify(&p)
			if err := p.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestProposal_Suggestion(t *testing.T) {
	t.Parallel()
	household, plant, task := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	p := validProposal()
	p.TargetTaskID = task

	got, err := p.Suggestion("repotting.v1", household, plant)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Suggestion{
		HouseholdID: household, PlantID: plant, TaskID: task, ProviderID: "repotting.v1",
		Kind: p.Kind, Action: p.Action, TaskKind: p.TaskKind, TaskSchedule: p.TaskSchedule, ProposedDate: p.ProposedDate,
		MessageKey: p.MessageKey, MessageParams: json.RawMessage(`{}`), ReasonKey: p.ReasonKey, ReasonParams: json.RawMessage(`{"species":"モンステラ"}`),
		ValidFrom: p.ValidFrom, DedupeKey: p.DedupeKey, Status: domain.SuggestionPending,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
