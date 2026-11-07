package job

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SeasonStore は、季節の切り替えで、タスクの次回予定を計算し直す（実装は postgres.SeasonRepository）。
type SeasonStore interface {
	// Households は、すべての世帯（ID・タイムゾーン・半球）を返す。
	Households(ctx context.Context) ([]domain.Household, error)
	// Recalculate は、世帯の季節ごとの周期を持つタスクの次回予定を、seasonStart の季節の周期で計算し直し、変えたタスクの数を返す。
	Recalculate(ctx context.Context, householdID uuid.UUID, seasonStart civil.Date, h domain.Hemisphere) (int, error)
}

// Seasons は、季節の初日に、季節ごとの周期を持つタスクの次回予定を計算し直すジョブである（要件8-7）。
// 1時間ごとに実行し、世帯のタイムゾーンで今日が季節の初日の世帯だけを計算し直す。初日のうちなら何度実行しても同じ結果になる。
type Seasons struct {
	store SeasonStore
	now   func() time.Time
}

func NewSeasons(store SeasonStore, now func() time.Time) *Seasons {
	return &Seasons{store: store, now: now}
}

func (s *Seasons) Name() string { return "seasons" }

func (s *Seasons) Run(ctx context.Context) (Result, error) {
	var res Result
	households, err := s.store.Households(ctx)
	if err != nil {
		return res, fmt.Errorf("list households: %w", err)
	}
	now := s.now()
	for _, h := range households {
		today := domain.TodayIn(now, h.TimeZone)
		if !domain.IsSeasonStart(today) {
			continue
		}
		n, err := s.store.Recalculate(ctx, h.ID, today, h.Hemisphere)
		if err != nil {
			return res, fmt.Errorf("recalculate season of household %s: %w", h.ID, err)
		}
		res.Processed += n
	}
	return res, nil
}
