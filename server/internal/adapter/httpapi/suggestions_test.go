package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
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

var testSuggestion = domain.Suggestion{
	ID:           uuid.MustParse("01920000-0000-7000-8000-0000000000b2"),
	PlantID:      testPlant.ID,
	ProviderID:   "repotting.v1",
	Kind:         domain.SuggestionDo,
	Action:       domain.SuggestionCreateTask,
	TaskKind:     domain.CareRepotting,
	ProposedDate: civil.Date{Year: 2027, Month: 5, Day: 1},
	MessageKey:   "repotting.initial",
	ReasonKey:    "repotting.initial.reason",
	ReasonParams: json.RawMessage(`{"species":"モンステラ"}`),
	ValidFrom:    civil.Date{Year: 2026, Month: 11, Day: 6},
	Status:       domain.SuggestionPending,
}

// fakeSuggestionService は、app.SuggestionService の偽物である。受け取った日付を記録する。
type fakeSuggestionService struct {
	dates    []*string
	accepted []domain.AcceptInput
}

func (f *fakeSuggestionService) List(_ context.Context, _, _ uuid.UUID, date *string) ([]domain.Suggestion, error) {
	f.dates = append(f.dates, date)
	return []domain.Suggestion{testSuggestion}, nil
}

func (f *fakeSuggestionService) Accept(_ context.Context, _, id uuid.UUID, in domain.AcceptInput) (domain.Suggestion, domain.CareTask, error) {
	f.accepted = append(f.accepted, in)
	if id != testSuggestion.ID {
		return domain.Suggestion{}, domain.CareTask{}, &domain.ConflictError{Code: domain.ConflictAlreadyResponded}
	}
	s := testSuggestion
	s.Status, s.RespondedAt = domain.SuggestionAccepted, time.Date(2026, 11, 6, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	return s, testTask, nil
}

func (f *fakeSuggestionService) Dismiss(_ context.Context, _, id uuid.UUID) (domain.Suggestion, error) {
	if id != testSuggestion.ID {
		return domain.Suggestion{}, &domain.ConflictError{Code: domain.ConflictSuggestionExpired}
	}
	s := testSuggestion
	s.Status = domain.SuggestionDismissed
	return s, nil
}

func newSuggestionRouter(t *testing.T, svc *fakeSuggestionService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Suggestions: svc})
}

const wantSuggestionJSON = `{"action":"create_task","id":"01920000-0000-7000-8000-0000000000b2","kind":"do","messageKey":"repotting.initial","messageParams":{},` +
	`"plantId":"01920000-0000-7000-8000-0000000000d1","proposedDate":"2027-05-01","providerId":"repotting.v1","reasonKey":"repotting.initial.reason",` +
	`"reasonParams":{"species":"モンステラ"},"status":"pending","taskKind":"repotting","validFrom":"2026-11-06"}`

func TestListSuggestions(t *testing.T) {
	t.Parallel()
	svc := &fakeSuggestionService{}
	rec := serve(t, newSuggestionRouter(t, svc), http.MethodGet, "/v1/households/"+testHousehold.ID.String()+"/suggestions?date=2026-11-06", "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	if got := canonicalJSON(t, rec.Body.Bytes()); got != `{"suggestions":[`+wantSuggestionJSON+`]}` {
		t.Errorf("body = %s", got)
	}
	if d := svc.dates[0]; d == nil || *d != "2026-11-06" {
		t.Errorf("date = %v", d)
	}
}

func TestAcceptSuggestion(t *testing.T) {
	t.Parallel()

	t.Run("採用した提案と、作ったタスクを返す", func(t *testing.T) {
		t.Parallel()
		svc := &fakeSuggestionService{}
		rec := serve(t, newSuggestionRouter(t, svc), http.MethodPost, "/v1/suggestions/"+testSuggestion.ID.String()+"/accept", "Bearer "+validToken, `{"date":"2027-05-10"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Suggestion struct{ Status, RespondedAt string }
			Task       *struct{ Id uuid.UUID }
		}](t, rec)
		if got.Suggestion.Status != "accepted" || got.Suggestion.RespondedAt != "2026-11-06T00:00:00Z" || got.Task == nil || got.Task.Id != testTask.ID {
			t.Errorf("body = %s", rec.Body.String())
		}
		if d := svc.accepted[0].Date; d == nil || *d != "2027-05-10" {
			t.Errorf("date = %v", d)
		}
	})

	t.Run("本文を省いてもよい", func(t *testing.T) {
		t.Parallel()
		svc := &fakeSuggestionService{}
		rec := serve(t, newSuggestionRouter(t, svc), http.MethodPost, "/v1/suggestions/"+testSuggestion.ID.String()+"/accept", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK || svc.accepted[0].Date != nil {
			t.Errorf("status = %d (%s), input = %+v", rec.Code, rec.Body.String(), svc.accepted)
		}
	})

	t.Run("答えた提案は 409 already_responded", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newSuggestionRouter(t, &fakeSuggestionService{}), http.MethodPost, "/v1/suggestions/"+uuid.Must(uuid.NewV7()).String()+"/accept", "Bearer "+validToken, "")
		if got := decode[struct{ Code string }](t, rec); rec.Code != http.StatusConflict || got.Code != "already_responded" {
			t.Errorf("status = %d (%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestDismissSuggestion(t *testing.T) {
	t.Parallel()

	t.Run("見送った提案を返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newSuggestionRouter(t, &fakeSuggestionService{}), http.MethodPost, "/v1/suggestions/"+testSuggestion.ID.String()+"/dismiss", "Bearer "+validToken, "")
		if got := decode[struct{ Status string }](t, rec); rec.Code != http.StatusOK || got.Status != "dismissed" {
			t.Errorf("status = %d (%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("有効期間を過ぎた提案は 409 suggestion_expired", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newSuggestionRouter(t, &fakeSuggestionService{}), http.MethodPost, "/v1/suggestions/"+uuid.Must(uuid.NewV7()).String()+"/dismiss", "Bearer "+validToken, "")
		if got := decode[struct{ Code string }](t, rec); rec.Code != http.StatusConflict || got.Code != "suggestion_expired" {
			t.Errorf("status = %d (%s)", rec.Code, rec.Body.String())
		}
	})
}
