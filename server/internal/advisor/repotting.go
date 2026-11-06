package advisor

import (
	"context"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// RepottingInterval は、植え替えの提案を採用したときに作る、植え替えタスクの周期である。
// 一般向けの解説の「およそ2年に1回が目安」「1～2年を目安に」にもとづく（出典は docs/decisions/0028）。
// 根詰まりのサインがあれば、周期にかかわらず植え替える（予定日はユーザーが変えられる）。
var RepottingInterval = domain.Interval{Value: 24, Unit: domain.IntervalMonth}

// Repotting は、植え替えの助言プロバイダである（要件17、設計の RepottingAdvisor）。
//   - 植物の登録：購入日があり、植え替えタスクがなければ、植え替えタスクの提案を作る（要件17-1・17-2・17-7）
//   - タスクの予定日の変更：植え替えタスクの予定日が避ける時期なら、時期ごとの理由つきで警告する（要件17-8）
//
// 時期のルールは、植物マスタに種類ごとの適期があればそれを、なければ既定値を使う（domain.Species.RepottingRule）。
type Repotting struct{}

func (Repotting) ID() string { return "repotting.v1" }

func (Repotting) Triggers() []Trigger {
	return []Trigger{TriggerPlantCreated, TriggerTaskDueChanged}
}

func (r Repotting) Advise(_ context.Context, in Context) ([]Proposal, error) {
	switch in.Trigger {
	case TriggerPlantCreated:
		return r.initial(in), nil
	case TriggerTaskDueChanged:
		return r.avoid(in), nil
	}
	return nil, nil
}

// initial は、植物の登録で、植え替えタスクの提案を作る。
func (Repotting) initial(in Context) []Proposal {
	if in.Plant.AcquiredOn.IsZero() {
		return nil
	}
	for _, t := range in.Tasks {
		if t.Kind == domain.CareRepotting {
			return nil
		}
	}
	rule, species := ruleOf(in.Species)
	p := rule.Propose(in.Plant.AcquiredOn, in.Today, in.Household.Hemisphere)

	// 理由の材料（要件17-7）。アプリが「慣らし期間のあと、真冬を避けた最初の適期です」「ポトスの適期です」のように組み立てる
	avoided := make([]string, 0, len(p.Avoided))
	for _, period := range p.Avoided {
		avoided = append(avoided, string(period))
	}
	reason := map[string]any{"afterAcclimation": p.AfterAcclimation, "acclimationDays": domain.RepottingAcclimationDays, "avoided": avoided}
	if species != "" {
		reason["species"] = species
	}
	return []Proposal{{
		Kind:          domain.SuggestionDo,
		Action:        domain.SuggestionCreateTask,
		TaskKind:      domain.CareRepotting,
		TaskSchedule:  domain.Schedule{Base: RepottingInterval},
		ProposedDate:  p.Date,
		MessageKey:    "repotting.initial",
		MessageParams: map[string]any{"plantName": in.Plant.Name, "date": p.Date.String()},
		ReasonKey:     "repotting.initial.reason",
		ReasonParams:  reason,
		ValidFrom:     in.Today,
		// 期限は付けない。提案日を過ぎてから採用する場合は、日付を変えて採用できる
		DedupeKey: "repotting:initial:" + in.Plant.ID.String(),
	}}
}

// avoid は、植え替えタスクの予定日が避ける時期なら、やらない方がよいことの助言を作る（設定自体は妨げない）。
// キーに予定日を含めるので、予定日を変えるたびに、その日についての助言になる。
// 前の予定日についての助言は、返さなくなるので、SuggestionService が撤回する。
func (Repotting) avoid(in Context) []Proposal {
	var task *domain.CareTask
	for i := range in.Tasks {
		if in.Tasks[i].ID == in.TaskID {
			task = &in.Tasks[i]
		}
	}
	if task == nil || task.Kind != domain.CareRepotting || task.Status != domain.TaskActive || task.NextDueOn.Before(in.Today) {
		return nil
	}
	rule, _ := ruleOf(in.Species)
	period := rule.PeriodOf(task.NextDueOn, in.Household.Hemisphere)
	if !period.ShouldAvoid() {
		return nil
	}
	due := task.NextDueOn.String()
	return []Proposal{{
		Kind:          domain.SuggestionAvoid,
		Action:        domain.SuggestionInfo,
		TargetTaskID:  task.ID,
		MessageKey:    "repotting.avoid",
		MessageParams: map[string]any{"plantName": in.Plant.Name, "date": due},
		// 時期ごとの理由（真夏：暑さで生長が鈍り回復しにくい、真冬：寒さで根が傷みやすい）は、アプリが i18n で引く
		ReasonKey:  "repotting.avoid." + string(period) + ".reason",
		ValidFrom:  in.Today,
		ValidUntil: task.NextDueOn,
		DedupeKey:  "repotting:avoid:" + task.ID.String() + ":" + due,
	}}
}

// ruleOf は、植え替えのルールと、種類ごとのルールを使った場合はその種類の名前を返す。
func ruleOf(s *domain.Species) (domain.RepottingRule, string) {
	if s == nil || len(s.Care.RepottingSuitableMonths) == 0 {
		return domain.DefaultRepottingRule, ""
	}
	return s.RepottingRule(), s.NameJa
}
