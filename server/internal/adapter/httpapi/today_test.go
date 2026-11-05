package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

// fakeTodayService は、app.TodayService の偽物である。testHousehold なら testTask の1件を、それ以外は次の予定だけを返す。
type fakeTodayService struct {
	dates []*string
}

func (f *fakeTodayService) List(_ context.Context, _, householdID uuid.UUID, date *string) (domain.TodayList, error) {
	f.dates = append(f.dates, date)
	day := civil.Date{Year: 2026, Month: 11, Day: 4}
	item := domain.TodayItem{
		TodayTask:   domain.TodayTask{Task: testTask, PlantName: "モンステラ", LocationID: uuid.MustParse("01920000-0000-7000-8000-0000000000c1"), LocationName: "リビング"},
		OverdueDays: 2,
		Suggestions: []domain.Suggestion{{
			ID: uuid.MustParse("01920000-0000-7000-8000-0000000000b1"), ProviderID: "weather.v1", Kind: domain.SuggestionAvoid, Action: domain.SuggestionSkipToday,
			MessageKey: "weather.rain", MessageParams: json.RawMessage(`{"mm":10}`), ReasonKey: "weather.rain.reason", ValidFrom: day, ValidUntil: day,
		}},
	}
	if householdID != testHousehold.ID {
		next := item
		next.LocationID, next.LocationName, next.OverdueDays, next.Suggestions = uuid.Nil, "", 0, nil
		return domain.TodayList{Date: day, Items: []domain.TodayItem{}, Next: &next}, nil
	}
	return domain.TodayList{Date: day, Items: []domain.TodayItem{item}}, nil
}

func newTodayRouter(t *testing.T, today *fakeTodayService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Today: today})
}

func TestGetToday(t *testing.T) {
	t.Parallel()

	t.Run("項目に、タスク・植物と置き場所・期限切れの日数・提案を入れる", func(t *testing.T) {
		t.Parallel()
		today := &fakeTodayService{}
		rec := serve(t, newTodayRouter(t, today), http.MethodGet, "/v1/households/"+testHousehold.ID.String()+"/today?date=2026-11-04", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Date  string
			Items []struct {
				Task        struct{ Id uuid.UUID }
				Plant       json.RawMessage
				OverdueDays int
				Suggestions []json.RawMessage
			}
			Next *json.RawMessage
		}](t, rec)
		if got.Date != "2026-11-04" || len(got.Items) != 1 || got.Next != nil {
			t.Fatalf("body = %s", rec.Body.String())
		}
		item := got.Items[0]
		const wantPlant = `{"id":"01920000-0000-7000-8000-0000000000d1","locationId":"01920000-0000-7000-8000-0000000000c1","locationName":"リビング","name":"モンステラ"}`
		const wantSuggestion = `{"action":"skip_today","id":"01920000-0000-7000-8000-0000000000b1","kind":"avoid","messageKey":"weather.rain","messageParams":{"mm":10},` +
			`"providerId":"weather.v1","reasonKey":"weather.rain.reason","reasonParams":{},"validFrom":"2026-11-04","validUntil":"2026-11-04"}`
		if item.Task.Id != testTask.ID || string(item.Plant) != wantPlant || item.OverdueDays != 2 || len(item.Suggestions) != 1 || string(item.Suggestions[0]) != wantSuggestion {
			t.Errorf("item = %s\n%s", item.Plant, item.Suggestions)
		}
		if d := today.dates[0]; d == nil || *d != "2026-11-04" {
			t.Errorf("date = %v", d)
		}
	})

	t.Run("今日やることがなければ、空の items と次の予定を返す", func(t *testing.T) {
		t.Parallel()
		today := &fakeTodayService{}
		rec := serve(t, newTodayRouter(t, today), http.MethodGet, "/v1/households/"+uuid.Must(uuid.NewV7()).String()+"/today", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Items []json.RawMessage
			Next  *struct {
				Plant       map[string]any
				Suggestions []json.RawMessage
			}
		}](t, rec)
		if got.Items == nil || len(got.Items) != 0 || got.Next == nil || got.Next.Plant["locationId"] != nil || got.Next.Suggestions == nil {
			t.Errorf("body = %s", rec.Body.String())
		}
		if today.dates[0] != nil {
			t.Errorf("date = %v, want nil", *today.dates[0])
		}
	})
}
