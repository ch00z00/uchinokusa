package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

// fakeCalendarService は、app.CalendarService の偽物である。受け取った月を記録する。
type fakeCalendarService struct {
	months []*string
}

func (f *fakeCalendarService) Month(_ context.Context, _, _ uuid.UUID, month *string) (domain.PlantCalendar, error) {
	f.months = append(f.months, month)
	other := domain.CareTask{ID: uuid.MustParse("01920000-0000-7000-8000-0000000000f1"), CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareOther, CustomName: "葉水"}}
	return domain.PlantCalendar{
		Month:   domain.CalendarMonth{Year: 2026, Month: time.November},
		Records: []domain.CareRecord{testRecord},
		Plans: []domain.CalendarPlan{
			{Date: civil.Date{Year: 2026, Month: 11, Day: 11}, Task: testTask},
			{Date: civil.Date{Year: 2026, Month: 11, Day: 18}, Task: other, Projected: true},
		},
		Notes: []domain.CalendarNote{{Date: civil.Date{Year: 2026, Month: 11, Day: 5}, Note: testNote}},
	}, nil
}

func TestGetPlantCalendar(t *testing.T) {
	t.Parallel()
	cal := &fakeCalendarService{}
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	router := httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Calendar: cal})

	rec := serve(t, router, http.MethodGet, "/v1/plants/"+testPlant.ID.String()+"/calendar?month=2026-11", "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	got := decode[struct {
		Month   string
		Records []struct{ Id uuid.UUID }
		Plans   []map[string]any
		Notes   []struct {
			Date string
			Note struct{ Id uuid.UUID }
		}
	}](t, rec)
	if got.Month != "2026-11" || len(got.Records) != 1 || got.Records[0].Id != testRecord.ID {
		t.Errorf("body = %s", rec.Body.String())
	}
	// 作業名は other のときだけ、項目に入れる
	b, err := json.Marshal(got.Plans)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`[{"date":"2026-11-11","kind":"watering","projected":false,"taskId":"%s"},`+
		`{"customName":"葉水","date":"2026-11-18","kind":"other","projected":true,"taskId":"01920000-0000-7000-8000-0000000000f1"}]`, testTask.ID)
	if plans := canonicalJSON(t, b); plans != want {
		t.Errorf("plans = %s\nwant %s", plans, want)
	}
	if len(got.Notes) != 1 || got.Notes[0].Date != "2026-11-05" || got.Notes[0].Note.Id != testNote.ID {
		t.Errorf("notes = %+v", got.Notes)
	}
	if m := cal.months[0]; m == nil || *m != "2026-11" {
		t.Errorf("month = %v", m)
	}
}
