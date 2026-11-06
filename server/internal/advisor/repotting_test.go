package advisor_test

import (
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func date(y, m, d int) civil.Date { return civil.Date{Year: y, Month: time.Month(m), Day: d} }

var plantID = uuid.MustParse("01920000-0000-7000-8000-0000000000d1")

// plantCreated は、11/6 に、購入日 acquired の植物を登録したときの情報である（北半球、植物マスタなし）。
func plantCreated(acquired civil.Date) advisor.Context {
	return advisor.Context{
		Trigger: advisor.TriggerPlantCreated,
		Today:   date(2026, 11, 6),
		Subject: advisor.Subject{
			Household: domain.Household{Hemisphere: domain.HemisphereNorth},
			Plant:     domain.Plant{ID: plantID, PlantProfile: domain.PlantProfile{Name: "リビングのモンステラ", AcquiredOn: acquired}},
		},
	}
}

func adviseOne(t *testing.T, in advisor.Context) []advisor.Proposal {
	t.Helper()
	got, err := advisor.Repotting{}.Advise(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if err := p.Validate(); err != nil {
			t.Errorf("invalid proposal %+v: %v", p, err)
		}
	}
	return got
}

func TestRepotting_ID(t *testing.T) {
	t.Parallel()
	a := advisor.Repotting{}
	if a.ID() != "repotting.v1" || !reflect.DeepEqual(a.Triggers(), []advisor.Trigger{advisor.TriggerPlantCreated, advisor.TriggerTaskDueChanged}) {
		t.Errorf("id = %s, triggers = %v", a.ID(), a.Triggers())
	}
}

func TestRepotting_PlantCreated(t *testing.T) {
	t.Parallel()

	t.Run("購入日つきの植物に、慣らし期間と避ける時期のあとの適期で、植え替えタスクを提案する", func(t *testing.T) {
		t.Parallel()
		// 11/1 に購入：慣らし期間は 11/15 まで。11月はどちらでもなく、12〜2月は真冬、3〜4月もどちらでもないので、5/1
		got := adviseOne(t, plantCreated(date(2026, 11, 1)))
		if len(got) != 1 {
			t.Fatalf("proposals = %+v", got)
		}
		want := advisor.Proposal{
			Kind: domain.SuggestionDo, Action: domain.SuggestionCreateTask, TaskKind: domain.CareRepotting,
			TaskSchedule:  domain.Schedule{Base: advisor.RepottingInterval},
			ProposedDate:  date(2027, 5, 1),
			MessageKey:    "repotting.initial",
			MessageParams: map[string]any{"plantName": "リビングのモンステラ", "date": "2027-05-01"},
			ReasonKey:     "repotting.initial.reason",
			ReasonParams:  map[string]any{"afterAcclimation": true, "acclimationDays": domain.RepottingAcclimationDays, "avoided": []string{"midwinter"}},
			ValidFrom:     date(2026, 11, 6),
			DedupeKey:     "repotting:initial:" + plantID.String(),
		}
		if !reflect.DeepEqual(got[0], want) {
			t.Errorf("got  %+v\nwant %+v", got[0], want)
		}
	})

	t.Run("植物マスタに種類ごとの適期があれば、それを使い、理由に種類の名前を入れる", func(t *testing.T) {
		t.Parallel()
		in := plantCreated(date(2026, 3, 1))
		in.Today = date(2026, 3, 10)
		in.Species = &domain.Species{NameJa: "ポトス", Care: domain.SpeciesCare{RepottingSuitableMonths: []int{3, 4, 5}}}
		got := adviseOne(t, in)
		// 3/15（慣らし期間の終わり）が適期の中
		if len(got) != 1 || got[0].ProposedDate != date(2026, 3, 15) || got[0].ReasonParams["species"] != "ポトス" {
			t.Errorf("proposals = %+v", got)
		}
	})

	t.Run("植物マスタに適期がなければ、種類の名前は入れない", func(t *testing.T) {
		t.Parallel()
		in := plantCreated(date(2026, 11, 1))
		in.Species = &domain.Species{NameJa: "パキラ"}
		got := adviseOne(t, in)
		if _, ok := got[0].ReasonParams["species"]; ok {
			t.Errorf("reason params = %v", got[0].ReasonParams)
		}
	})

	t.Run("南半球では、季節を6か月ずらす", func(t *testing.T) {
		t.Parallel()
		in := plantCreated(date(2026, 11, 1))
		in.Household.Hemisphere = domain.HemisphereSouth
		// 南半球の11月は北半球の5月にあたり、適期。慣らし期間の終わりの 11/15 を提案する
		if got := adviseOne(t, in); got[0].ProposedDate != date(2026, 11, 15) {
			t.Errorf("date = %v", got[0].ProposedDate)
		}
	})

	t.Run("購入日がなければ、提案しない", func(t *testing.T) {
		t.Parallel()
		if got := adviseOne(t, plantCreated(civil.Date{})); len(got) != 0 {
			t.Errorf("proposals = %+v", got)
		}
	})

	t.Run("植え替えタスクがすでにあれば、提案しない", func(t *testing.T) {
		t.Parallel()
		in := plantCreated(date(2026, 11, 1))
		in.Tasks = []domain.CareTask{{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareWatering}}, {CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareRepotting}}}
		if got := adviseOne(t, in); len(got) != 0 {
			t.Errorf("proposals = %+v", got)
		}
	})
}

// dueChanged は、植え替えタスクの予定日を due にしたときの情報である（11/6、北半球、植物マスタなし）。
func dueChanged(due civil.Date) (advisor.Context, uuid.UUID) {
	task := uuid.MustParse("01920000-0000-7000-8000-0000000000a9")
	in := plantCreated(date(2026, 10, 1))
	in.Trigger, in.TaskID = advisor.TriggerTaskDueChanged, task
	in.Tasks = []domain.CareTask{
		{ID: uuid.Must(uuid.NewV7()), CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareWatering}, NextDueOn: date(2027, 7, 1), Status: domain.TaskActive},
		{ID: task, CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareRepotting}, NextDueOn: due, Status: domain.TaskActive},
	}
	return in, task
}

func TestRepotting_TaskDueChanged(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		due    civil.Date
		reason string
	}{
		{"真夏の予定日には、暑さの理由で警告する", date(2027, 7, 20), "repotting.avoid.midsummer.reason"},
		{"真冬の予定日には、寒さの理由で警告する", date(2027, 1, 10), "repotting.avoid.midwinter.reason"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in, task := dueChanged(tt.due)
			got := adviseOne(t, in)
			if len(got) != 1 {
				t.Fatalf("proposals = %+v", got)
			}
			want := advisor.Proposal{
				Kind: domain.SuggestionAvoid, Action: domain.SuggestionInfo, TargetTaskID: task,
				MessageKey: "repotting.avoid", MessageParams: map[string]any{"plantName": "リビングのモンステラ", "date": tt.due.String()},
				ReasonKey: tt.reason, ValidFrom: date(2026, 11, 6), ValidUntil: tt.due,
				DedupeKey: "repotting:avoid:" + task.String() + ":" + tt.due.String(),
			}
			if !reflect.DeepEqual(got[0], want) {
				t.Errorf("got  %+v\nwant %+v", got[0], want)
			}
		})
	}

	t.Run("種類ごとの適期は、既定の避ける時期より優先する", func(t *testing.T) {
		t.Parallel()
		// 7月は既定では真夏だが、適期が 5〜9月の種類では適期になる
		in, _ := dueChanged(date(2027, 7, 20))
		in.Species = &domain.Species{Care: domain.SpeciesCare{RepottingSuitableMonths: []int{5, 6, 7, 8, 9}}}
		if got := adviseOne(t, in); len(got) != 0 {
			t.Errorf("proposals = %+v", got)
		}
	})

	for _, tt := range []struct {
		name   string
		modify func(*advisor.Context)
	}{
		{"適期の予定日には、警告しない", func(in *advisor.Context) { in.Tasks[1].NextDueOn = date(2027, 5, 10) }},
		{"どちらでもない月の予定日には、警告しない", func(in *advisor.Context) { in.Tasks[1].NextDueOn = date(2027, 4, 10) }},
		{"植え替え以外のタスクには、警告しない", func(in *advisor.Context) { in.TaskID = in.Tasks[0].ID }},
		{"過ぎた予定日には、警告しない", func(in *advisor.Context) { in.Tasks[1].NextDueOn = date(2026, 8, 1) }},
		{"一時停止中のタスクには、警告しない", func(in *advisor.Context) { in.Tasks[1].Status = domain.TaskPaused }},
		{"対象のタスクがなければ、警告しない", func(in *advisor.Context) { in.TaskID = uuid.Must(uuid.NewV7()) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in, _ := dueChanged(date(2027, 7, 20))
			tt.modify(&in)
			if got := adviseOne(t, in); len(got) != 0 {
				t.Errorf("proposals = %+v", got)
			}
		})
	}
}
