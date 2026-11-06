package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeCalendar は、app.CalendarRepository の偽物である。受け取った範囲を記録する。
type fakeCalendar struct {
	plant         app.CalendarPlant
	records       []domain.CareRecord
	tasks         []domain.CareTask
	notes         []domain.Note
	recordRanges  [][2]civil.Date
	noteRanges    [][2]time.Time
	taskRequested bool
}

func (f *fakeCalendar) CalendarPlant(context.Context, uuid.UUID) (app.CalendarPlant, error) {
	return f.plant, nil
}

func (f *fakeCalendar) Records(_ context.Context, _, _ uuid.UUID, first, last civil.Date) ([]domain.CareRecord, error) {
	f.recordRanges = append(f.recordRanges, [2]civil.Date{first, last})
	return f.records, nil
}

func (f *fakeCalendar) ActiveTasks(context.Context, uuid.UUID, uuid.UUID) ([]domain.CareTask, error) {
	f.taskRequested = true
	return f.tasks, nil
}

func (f *fakeCalendar) Notes(_ context.Context, _, _ uuid.UUID, startsAt, endsAt time.Time) ([]domain.Note, error) {
	f.noteRanges = append(f.noteRanges, [2]time.Time{startsAt, endsAt})
	return f.notes, nil
}

// calendarNow は、日本時間の 2026-12-01 0:30（UTC では 11/30）である。世帯のタイムゾーンで「今月」を決めることを確かめるため。
var calendarNow = time.Date(2026, 11, 30, 15, 30, 0, 0, time.UTC)

func newCalendarFixture() (householdFixture, *fakeCalendar, *app.CalendarService) {
	hf := newHouseholdFixture()
	repo := &fakeCalendar{plant: app.CalendarPlant{HouseholdID: hf.household, TimeZone: "Asia/Tokyo", Hemisphere: domain.HemisphereNorth}}
	return hf, repo, app.NewCalendarService(repo, hf.authz, func() time.Time { return calendarNow })
}

func TestCalendarService_Month(t *testing.T) {
	t.Parallel()

	t.Run("月を省くと、世帯のタイムゾーンでの今月にし、その範囲で読む", func(t *testing.T) {
		t.Parallel()
		hf, repo, svc := newCalendarFixture()
		cal, err := svc.Month(t.Context(), hf.viewer, uuid.Must(uuid.NewV7()), nil)
		if err != nil {
			t.Fatal(err)
		}
		if cal.Month != (domain.CalendarMonth{Year: 2026, Month: time.December}) {
			t.Errorf("month = %v", cal.Month)
		}
		if want := [2]civil.Date{{Year: 2026, Month: 12, Day: 1}, {Year: 2026, Month: 12, Day: 31}}; repo.recordRanges[0] != want {
			t.Errorf("record range = %v", repo.recordRanges[0])
		}
		tokyo, _ := time.LoadLocation("Asia/Tokyo")
		if r := repo.noteRanges[0]; !r[0].Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, tokyo)) || !r[1].Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, tokyo)) {
			t.Errorf("note range = %v", r)
		}
	})

	t.Run("タスクごとに展開した予定を、日付の順に並べる", func(t *testing.T) {
		t.Parallel()
		hf, repo, svc := newCalendarFixture()
		weekly := domain.Schedule{Base: domain.Interval{Value: 1, Unit: domain.IntervalWeek}}
		monthly := domain.Schedule{Base: domain.Interval{Value: 1, Unit: domain.IntervalMonth}}
		repo.tasks = []domain.CareTask{
			{ID: uuid.Must(uuid.NewV7()), CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareFertilizing, Schedule: monthly}, NextDueOn: civil.Date{Year: 2026, Month: 12, Day: 10}, Status: domain.TaskActive},
			{ID: uuid.Must(uuid.NewV7()), CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareWatering, Schedule: weekly}, NextDueOn: civil.Date{Year: 2026, Month: 12, Day: 3}, Status: domain.TaskActive},
		}
		cal, err := svc.Month(t.Context(), hf.viewer, uuid.Must(uuid.NewV7()), ptr("2026-12"))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range cal.Plans {
			got = append(got, p.Date.String()+" "+string(p.Task.Kind))
		}
		// 同じ日は、ActiveTasks が返したタスクの順のまま
		want := []string{"2026-12-03 watering", "2026-12-10 fertilizing", "2026-12-10 watering", "2026-12-17 watering", "2026-12-24 watering", "2026-12-31 watering"}
		if !slices.Equal(got, want) {
			t.Errorf("plans = %v\nwant %v", got, want)
		}
	})

	t.Run("メモを書いた日は、世帯のタイムゾーンで決める", func(t *testing.T) {
		t.Parallel()
		hf, repo, svc := newCalendarFixture()
		repo.notes = []domain.Note{{ID: uuid.Must(uuid.NewV7()), CreatedAt: time.Date(2026, 11, 30, 15, 0, 0, 0, time.UTC)}} // 日本時間の 12/1 0:00
		cal, err := svc.Month(t.Context(), hf.viewer, uuid.Must(uuid.NewV7()), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(cal.Notes) != 1 || cal.Notes[0].Date != (civil.Date{Year: 2026, Month: 12, Day: 1}) {
			t.Errorf("notes = %+v", cal.Notes)
		}
	})

	t.Run("アーカイブした植物は、予定を出さない", func(t *testing.T) {
		t.Parallel()
		hf, repo, svc := newCalendarFixture()
		repo.plant.Archived = true
		repo.tasks = []domain.CareTask{{CareTaskProfile: domain.CareTaskProfile{Schedule: domain.Schedule{Base: domain.Interval{Value: 1, Unit: domain.IntervalDay}}}, NextDueOn: civil.Date{Year: 2026, Month: 12, Day: 1}, Status: domain.TaskActive}}
		cal, err := svc.Month(t.Context(), hf.viewer, uuid.Must(uuid.NewV7()), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(cal.Plans) != 0 || cal.Plans == nil || repo.taskRequested {
			t.Errorf("plans = %v, taskRequested = %v", cal.Plans, repo.taskRequested)
		}
	})

	t.Run("読めない月は 422", func(t *testing.T) {
		t.Parallel()
		hf, _, svc := newCalendarFixture()
		_, err := svc.Month(t.Context(), hf.viewer, uuid.Must(uuid.NewV7()), ptr("2026/12"))
		assertFields(t, err, map[string]string{"month": "invalid"})
	})

	t.Run("メンバーでない人には、入力の誤りより先に 404 を返す", func(t *testing.T) {
		t.Parallel()
		hf, _, svc := newCalendarFixture()
		if _, err := svc.Month(t.Context(), hf.stranger, uuid.Must(uuid.NewV7()), ptr("2026/12")); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
