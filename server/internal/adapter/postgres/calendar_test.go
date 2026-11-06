package postgres_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestCalendarRepository_CalendarPlant(t *testing.T) {
	t.Parallel()
	f := newRecordFixture(t)
	repo := postgres.NewCalendarRepository(f.pool)
	f.exec(`UPDATE households SET time_zone = 'Australia/Sydney', hemisphere = 'south' WHERE id = $1`, f.home)

	got, err := repo.CalendarPlant(f.ctx, f.monsteraPlant)
	if err != nil {
		t.Fatal(err)
	}
	if got.HouseholdID != f.home || got.TimeZone != "Australia/Sydney" || got.Hemisphere != domain.HemisphereSouth || got.Archived {
		t.Errorf("plant = %+v", got)
	}
	f.exec(`UPDATE plants SET archived_at = now() WHERE id = $1`, f.monsteraPlant)
	if got, err := repo.CalendarPlant(f.ctx, f.monsteraPlant); err != nil || !got.Archived {
		t.Errorf("archived: %+v, %v", got, err)
	}
	if _, err := repo.CalendarPlant(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCalendarRepository_Records(t *testing.T) {
	t.Parallel()
	f := newRecordFixture(t)
	repo := postgres.NewCalendarRepository(f.pool)
	f.exec(`UPDATE care_tasks SET next_due_on = '2026-10-01' WHERE id = $1`, f.task.ID)
	// 月の境目の前後（10/31・11/1・11/30・12/1）に記録を作る
	for _, on := range []string{"2026-10-31", "2026-11-01", "2026-11-30", "2026-12-01"} {
		f.exec(`INSERT INTO care_records (household_id, plant_id, task_id, task_kind, outcome, performed_on, actor_display_name, cycle_seq, is_extra)
			VALUES ($1, $2, $3, 'watering', 'completed', $4, 'さくら', 1, true)`, f.home, f.monsteraPlant, f.task.ID, on)
	}
	// ほかの植物の記録は返さない
	other := f.createTask(f.pothosPlant, wateringDraft)
	f.exec(`INSERT INTO care_records (household_id, plant_id, task_id, task_kind, outcome, performed_on, actor_display_name, cycle_seq, is_extra)
		VALUES ($1, $2, $3, 'watering', 'completed', '2026-11-15', 'さくら', 1, true)`, f.home, f.pothosPlant, other.ID)

	nov := domain.CalendarMonth{Year: 2026, Month: time.November}
	got, err := repo.Records(f.ctx, f.home, f.monsteraPlant, nov.First(), nov.Last())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2026-11-01", "2026-11-30"}; !slices.Equal(dateStrings(got), want) {
		t.Errorf("records = %v, want %v", dateStrings(got), want)
	}
	if got, err := repo.Records(f.ctx, f.other, f.monsteraPlant, nov.First(), nov.Last()); err != nil || len(got) != 0 {
		t.Errorf("other household: %v, %v", dateStrings(got), err)
	}
}

func dateStrings(records []domain.CareRecord) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, r.PerformedOn.String())
	}
	return out
}

func TestCalendarRepository_ActiveTasks(t *testing.T) {
	t.Parallel()
	f := newRecordFixture(t) // f.task は有効な水やり
	repo := postgres.NewCalendarRepository(f.pool)
	paused := f.createTask(f.monsteraPlant, domain.CareTaskDraft{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareFertilizing, Schedule: wateringDraft.Schedule}, NextDueOn: wateringDraft.NextDueOn})
	f.exec(`UPDATE care_tasks SET status = 'paused' WHERE id = $1`, paused.ID)
	deleted := f.createTask(f.monsteraPlant, domain.CareTaskDraft{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CarePruning, Schedule: wateringDraft.Schedule}, NextDueOn: wateringDraft.NextDueOn})
	f.exec(`UPDATE care_tasks SET deleted_at = now() WHERE id = $1`, deleted.ID)
	f.createTask(f.pothosPlant, wateringDraft) // ほかの植物

	got, err := repo.ActiveTasks(f.ctx, f.home, f.monsteraPlant)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != f.task.ID || got[0].Schedule.Seasonal[domain.SeasonWinter].Value != 2 {
		t.Errorf("tasks = %+v", got)
	}
}

func TestCalendarRepository_Notes(t *testing.T) {
	t.Parallel()
	f := newNoteFixture(t)
	repo := postgres.NewCalendarRepository(f.pool)
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	// 日本時間の 11/1 0:00 は UTC の 10/31 15:00。境目の直前・ちょうど・月末の直前・翌月のちょうどにメモを作る
	for body, at := range map[string]string{
		"10月": "2026-10-31T14:59:59Z", "11月1日": "2026-10-31T15:00:00Z", "11月30日": "2026-11-30T14:59:59Z", "12月": "2026-11-30T15:00:00Z",
	} {
		n := f.write(body)
		f.exec(`UPDATE notes SET created_at = $2 WHERE id = $1`, n.ID, at)
	}

	got, err := repo.Notes(f.ctx, f.home, f.plant, time.Date(2026, 11, 1, 0, 0, 0, 0, tokyo), time.Date(2026, 12, 1, 0, 0, 0, 0, tokyo))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"11月1日", "11月30日"}; !slices.Equal(bodiesOf(got), want) {
		t.Errorf("notes = %v, want %v", bodiesOf(got), want)
	}
}
