package postgres_test

import (
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// todayFixture は、home に次のタスクを持つ。今日は 2026-11-04 とする。
//   - モンステラ（リビング）：水やり 11/4、肥料 11/10
//   - ポトス（置き場所なし）：水やり 10/30
//   - 対象外：一時停止・削除したタスク、アーカイブした植物のタスク、ほかの世帯のタスク（どれも 11/1）
type todayFixture struct {
	taskFixture
	today                                 *postgres.TodayRepository
	monsteraWatering, fertilizing, pothos domain.CareTask
}

var todayDate = civil.Date{Year: 2026, Month: 11, Day: 4}

func newTodayFixture(t *testing.T) todayFixture {
	t.Helper()
	f := todayFixture{taskFixture: newTaskFixture(t)}
	f.today = postgres.NewTodayRepository(f.pool)
	f.exec(`UPDATE plants SET location_id = $2 WHERE id = $1`, f.monsteraPlant, f.living)

	f.monsteraWatering = f.createTask(f.monsteraPlant, wateringDraft)
	fertilizing := wateringDraft
	fertilizing.Kind, fertilizing.NextDueOn = domain.CareFertilizing, civil.Date{Year: 2026, Month: 11, Day: 10}
	f.fertilizing = f.createTask(f.monsteraPlant, fertilizing)
	pothos := wateringDraft
	pothos.NextDueOn = civil.Date{Year: 2026, Month: 10, Day: 30}
	f.pothos = f.createTask(f.pothosPlant, pothos)

	f.addTask(f.monsteraPlant, "2026-11-01", "paused", false)
	f.addTask(f.monsteraPlant, "2026-11-01", "active", true)
	archived := f.create("アーカイブした植物", domain.PlantProfile{})
	f.exec(`UPDATE plants SET archived_at = now() WHERE id = $1`, archived.ID)
	f.addTask(archived.ID, "2026-11-01", "active", false)
	otherPlant := uuid.MustParse(f.plant(f.other.String(), nil))
	f.addTask(otherPlant, "2026-11-01", "active", false)
	return f
}

func TestTodayRepository_Due(t *testing.T) {
	t.Parallel()
	f := newTodayFixture(t)

	got, err := f.today.Due(f.ctx, f.home, todayDate)
	if err != nil {
		t.Fatal(err)
	}
	// 期限切れの古いものから
	if len(got) != 2 || got[0].Task.ID != f.pothos.ID || got[1].Task.ID != f.monsteraWatering.ID {
		t.Fatalf("due = %+v", got)
	}
	if got[1].PlantName != "モンステラ" || got[1].LocationID != f.living || got[1].LocationName == "" || got[1].Task.CycleSeq != 1 {
		t.Errorf("monstera = %+v", got[1])
	}
	if got[0].PlantName != "ポトス" || got[0].LocationID != uuid.Nil || got[0].LocationName != "" {
		t.Errorf("pothos = %+v", got[0])
	}
}

func TestTodayRepository_Next(t *testing.T) {
	t.Parallel()
	f := newTodayFixture(t)

	next, ok, err := f.today.Next(f.ctx, f.home, todayDate)
	if err != nil || !ok || next.Task.ID != f.fertilizing.ID {
		t.Errorf("next = %+v, %v, %v; want fertilizing", next, ok, err)
	}
	if _, ok, err := f.today.Next(f.ctx, f.home, civil.Date{Year: 2026, Month: 11, Day: 10}); err != nil || ok {
		t.Errorf("after the last task: ok = %v, %v; want none", ok, err)
	}
}

func TestTodayRepository_TaskSuggestions(t *testing.T) {
	t.Parallel()
	f := newTodayFixture(t)
	add := func(task uuid.UUID, kind, status, from string, until *string, key string) {
		f.exec(`INSERT INTO suggestions (household_id, plant_id, task_id, provider_id, kind, action, condition_text, message_key, message_params, reason_key, valid_from, valid_until, status, dedupe_key)
			SELECT household_id, plant_id, id, 'weather.v1', $2, 'skip_today', '雨なら', 'm', '{"mm": 10}', 'r', $3, $4, $5, $6 FROM care_tasks WHERE id = $1`,
			task, kind, from, until, status, key)
	}
	add(f.monsteraWatering.ID, "avoid", "pending", "2026-11-04", ptrTo("2026-11-04"), "a")
	add(f.pothos.ID, "conditional", "pending", "2026-11-01", nil, "b")
	add(f.monsteraWatering.ID, "do", "pending", "2026-11-01", nil, "c")                    // 実施をすすめる提案は載せない
	add(f.monsteraWatering.ID, "avoid", "dismissed", "2026-11-01", nil, "d")               // 見送った
	add(f.monsteraWatering.ID, "avoid", "pending", "2026-11-01", ptrTo("2026-11-03"), "e") // 期限切れ
	add(f.monsteraWatering.ID, "avoid", "pending", "2026-11-05", nil, "f")                 // まだ有効でない
	add(f.fertilizing.ID, "avoid", "pending", "2026-11-01", nil, "g")                      // 一覧に載らないタスク

	got, err := f.today.TaskSuggestions(f.ctx, f.home, []uuid.UUID{f.monsteraWatering.ID, f.pothos.ID}, todayDate)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("suggestions = %+v", got)
	}
	s := got[0]
	if s.TaskID != f.monsteraWatering.ID || s.Kind != domain.SuggestionAvoid || s.Action != domain.SuggestionSkipToday || s.ConditionText != "雨なら" ||
		string(s.MessageParams) != `{"mm": 10}` || s.ValidUntil != todayDate {
		t.Errorf("suggestion = %+v", s)
	}
	if got[1].TaskID != f.pothos.ID || got[1].ValidUntil != (civil.Date{}) {
		t.Errorf("suggestion = %+v", got[1])
	}
}

func ptrTo(s string) *string { return &s }
