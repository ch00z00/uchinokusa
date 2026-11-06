package app

import (
	"context"
	"slices"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CalendarPlant は、カレンダーの権限・今日・季節を決めるための、植物の世帯の情報である。
type CalendarPlant struct {
	HouseholdID uuid.UUID
	TimeZone    string
	Hemisphere  domain.Hemisphere
	Archived    bool
}

// CalendarRepository は、カレンダーのデータの読み出し先である（実装は adapter/postgres）。
type CalendarRepository interface {
	// CalendarPlant は、植物の世帯の情報を返す。植物がなければ domain.ErrNotFound を返す。
	CalendarPlant(ctx context.Context, plantID uuid.UUID) (CalendarPlant, error)
	// Records は、実施日が first〜last の記録を、実施日・記録した順に返す。
	Records(ctx context.Context, householdID, plantID uuid.UUID, first, last civil.Date) ([]domain.CareRecord, error)
	// ActiveTasks は、植物の有効なタスク（一時停止・削除を除く）を返す。
	ActiveTasks(ctx context.Context, householdID, plantID uuid.UUID) ([]domain.CareTask, error)
	// Notes は、作成日時が [startsAt, endsAt) のメモを、古い順に返す。
	Notes(ctx context.Context, householdID, plantID uuid.UUID, startsAt, endsAt time.Time) ([]domain.Note, error)
}

// CalendarService は、植物ごとのカレンダーのデータを返す（要件14）。
type CalendarService struct {
	repo  CalendarRepository
	authz *Authorizer
	now   func() time.Time
}

func NewCalendarService(repo CalendarRepository, authz *Authorizer, now func() time.Time) *CalendarService {
	return &CalendarService{repo: repo, authz: authz, now: now}
}

// Month は、植物の1か月分の記録・予定・メモを返す。閲覧者も見られる。month を省くと、世帯のタイムゾーンでの今月にする。
// 予定は、有効なタスクごとに、今の予定日と周期から月の範囲で展開する（domain.ExpandPlans）。
// アーカイブした植物は、記録とメモだけを返す（リマインダーと同じく、予定の対象から外すため）。
func (s *CalendarService) Month(ctx context.Context, userID, plantID uuid.UUID, month *string) (domain.PlantCalendar, error) {
	p, err := s.repo.CalendarPlant(ctx, plantID)
	if err != nil {
		return domain.PlantCalendar{}, err
	}
	if _, err := s.authz.Require(ctx, userID, p.HouseholdID, domain.ActionRead); err != nil {
		return domain.PlantCalendar{}, err
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		loc = time.UTC // domain.TodayIn と同じく、読めないタイムゾーンは UTC にする
	}
	today := civil.DateOf(s.now().In(loc))
	m, err := domain.ParseCalendarMonth(month, today)
	if err != nil {
		return domain.PlantCalendar{}, err
	}

	cal := domain.PlantCalendar{Month: m, Plans: []domain.CalendarPlan{}, Notes: []domain.CalendarNote{}}
	if cal.Records, err = s.repo.Records(ctx, p.HouseholdID, plantID, m.First(), m.Last()); err != nil {
		return domain.PlantCalendar{}, err
	}
	if !p.Archived {
		tasks, err := s.repo.ActiveTasks(ctx, p.HouseholdID, plantID)
		if err != nil {
			return domain.PlantCalendar{}, err
		}
		for _, t := range tasks {
			cal.Plans = append(cal.Plans, domain.ExpandPlans(t, today, m, p.Hemisphere)...)
		}
		// 日付の順にする。同じ日は、タスクの順（ActiveTasks の順）のまま
		slices.SortStableFunc(cal.Plans, func(a, b domain.CalendarPlan) int { return a.Date.Compare(b.Date) })
	}

	// メモを書いた日は、世帯のタイムゾーンで決める。月の境目も同じタイムゾーンの 0 時にする
	startsAt := m.First().In(loc)
	notes, err := s.repo.Notes(ctx, p.HouseholdID, plantID, startsAt, startsAt.AddDate(0, 1, 0))
	if err != nil {
		return domain.PlantCalendar{}, err
	}
	for _, n := range notes {
		cal.Notes = append(cal.Notes, domain.CalendarNote{Date: civil.DateOf(n.CreatedAt.In(loc)), Note: n})
	}
	return cal, nil
}
