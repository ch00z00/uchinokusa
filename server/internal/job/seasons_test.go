package job_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

type recalculation struct {
	start civil.Date
	h     domain.Hemisphere
}

type fakeSeasonStore struct {
	households []domain.Household
	changed    int
	listErr    error
	recalcErr  error
	calls      map[uuid.UUID]recalculation
}

func (f *fakeSeasonStore) Households(context.Context) ([]domain.Household, error) {
	return f.households, f.listErr
}

func (f *fakeSeasonStore) Recalculate(_ context.Context, householdID uuid.UUID, seasonStart civil.Date, h domain.Hemisphere) (int, error) {
	if f.calls == nil {
		f.calls = map[uuid.UUID]recalculation{}
	}
	f.calls[householdID] = recalculation{seasonStart, h}
	return f.changed, f.recalcErr
}

func TestSeasons_Run(t *testing.T) {
	t.Parallel()
	// 2026-12-01 0:30（東京）＝ 11-30 15:30（UTC）＝ 11-30 7:30（ロサンゼルス）
	now := time.Date(2026, 11, 30, 15, 30, 0, 0, time.UTC)
	tokyo := domain.Household{ID: uuid.New(), TimeZone: "Asia/Tokyo", Hemisphere: domain.HemisphereNorth}
	la := domain.Household{ID: uuid.New(), TimeZone: "America/Los_Angeles", Hemisphere: domain.HemisphereNorth}
	sydney := domain.Household{ID: uuid.New(), TimeZone: "Australia/Sydney", Hemisphere: domain.HemisphereSouth} // 南半球では 12/1 は夏の初日
	run := func(store *fakeSeasonStore) (job.Result, error) {
		return job.NewSeasons(store, func() time.Time { return now }).Run(t.Context())
	}

	t.Run("世帯のタイムゾーンで今日が季節の初日の世帯だけ、計算し直す", func(t *testing.T) {
		t.Parallel()
		store := &fakeSeasonStore{households: []domain.Household{tokyo, la, sydney}, changed: 2}
		res, err := run(store)
		if err != nil {
			t.Fatal(err)
		}
		dec1 := civil.Date{Year: 2026, Month: 12, Day: 1}
		want := map[uuid.UUID]recalculation{tokyo.ID: {dec1, domain.HemisphereNorth}, sydney.ID: {dec1, domain.HemisphereSouth}}
		if !reflect.DeepEqual(store.calls, want) {
			t.Errorf("calls = %v", store.calls)
		}
		// 計算し直したタスクの数
		if res != (job.Result{Processed: 4}) {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("読み書きできなければエラーを返す", func(t *testing.T) {
		t.Parallel()
		for _, store := range []*fakeSeasonStore{
			{listErr: errors.New("db down")},
			{households: []domain.Household{tokyo}, recalcErr: errors.New("db down")},
		} {
			if _, err := run(store); err == nil {
				t.Error("err = nil")
			}
		}
	})
}
