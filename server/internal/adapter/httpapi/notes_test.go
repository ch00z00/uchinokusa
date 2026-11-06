package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testNote = domain.Note{
	ID:                uuid.MustParse("01920000-0000-7000-8000-0000000000e1"),
	PlantID:           testPlant.ID,
	AuthorUserID:      uuid.NullUUID{UUID: uuid.MustParse("01920000-0000-7000-8000-0000000000a1"), Valid: true},
	AuthorDisplayName: "さくら",
	Body:              "新芽が出た",
	CreatedAt:         time.Date(2026, 11, 5, 9, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// canonicalJSON は、項目をキーの順に並べ直した JSON を返す（比べやすくするため）。
func canonicalJSON(t *testing.T, b []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// fakeNoteService は、app.NoteService の偽物である。受け取った本文と取得の条件を記録する。
type fakeNoteService struct {
	actors  []uuid.UUID // context に入っていた、操作した人（変更イベントに使う）
	bodies  []*string
	queries []domain.NoteQueryInput
	deleted []uuid.UUID
}

func (f *fakeNoteService) List(_ context.Context, _, _ uuid.UUID, in domain.NoteQueryInput) (domain.NotePage, error) {
	f.queries = append(f.queries, in)
	// 2件目は、作成者が退会し、編集したメモ
	edited := testNote
	edited.ID, edited.AuthorUserID, edited.EditedAt = uuid.MustParse("01920000-0000-7000-8000-0000000000e2"), uuid.NullUUID{}, time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	return domain.NotePage{Notes: []domain.Note{testNote, edited}, NextCursor: "next"}, nil
}

func (f *fakeNoteService) Create(ctx context.Context, _, _ uuid.UUID, body *string) (domain.Note, error) {
	f.actors = append(f.actors, app.ActorFrom(ctx))
	f.bodies = append(f.bodies, body)
	return testNote, nil
}

func (f *fakeNoteService) Update(_ context.Context, _, _ uuid.UUID, body *string) (domain.Note, error) {
	f.bodies = append(f.bodies, body)
	if *body == "" {
		return domain.Note{}, &domain.ValidationError{Fields: map[string]string{"body": "required"}}
	}
	return testNote, nil
}

func (f *fakeNoteService) Delete(_ context.Context, _, noteID uuid.UUID) error {
	if noteID != testNote.ID {
		return domain.ErrForbidden
	}
	f.deleted = append(f.deleted, noteID)
	return nil
}

func newNoteRouter(t *testing.T, notes *fakeNoteService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Notes: notes})
}

const wantNoteJSON = `{"authorDisplayName":"さくら","authorUserId":"01920000-0000-7000-8000-0000000000a1","body":"新芽が出た",` +
	`"createdAt":"2026-11-05T00:30:00Z","id":"01920000-0000-7000-8000-0000000000e1","plantId":"01920000-0000-7000-8000-0000000000d1"}`

func TestListPlantNotes(t *testing.T) {
	t.Parallel()
	notes := &fakeNoteService{}
	rec := serve(t, newNoteRouter(t, notes), http.MethodGet, "/v1/plants/"+testPlant.ID.String()+"/notes?cursor=abc&limit=10", "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	// 退会した作成者の ID と、編集していないメモの editedAt は、項目ごと省く
	const want = `{"nextCursor":"next","notes":[` + wantNoteJSON + `,` +
		`{"authorDisplayName":"さくら","body":"新芽が出た","createdAt":"2026-11-05T00:30:00Z","editedAt":"2026-11-05T01:00:00Z",` +
		`"id":"01920000-0000-7000-8000-0000000000e2","plantId":"01920000-0000-7000-8000-0000000000d1"}]}`
	if got := canonicalJSON(t, rec.Body.Bytes()); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}
	if q := notes.queries[0]; q.Cursor == nil || *q.Cursor != "abc" || q.Limit == nil || *q.Limit != 10 {
		t.Errorf("query = %+v", q)
	}
}

func TestCreateNote(t *testing.T) {
	t.Parallel()
	notes := &fakeNoteService{}
	rec := serve(t, newNoteRouter(t, notes), http.MethodPost, "/v1/plants/"+testPlant.ID.String()+"/notes", "Bearer "+validToken, `{"body":"新芽が出た"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	if got := canonicalJSON(t, rec.Body.Bytes()); got != wantNoteJSON {
		t.Errorf("body = %s", got)
	}
	if b := notes.bodies[0]; b == nil || *b != "新芽が出た" {
		t.Errorf("body = %v", b)
	}
	// 認証した人を、操作した人として context に入れる（変更イベントの actorUserId）
	if notes.actors[0] != testUser.ID {
		t.Errorf("actor = %v, want %v", notes.actors[0], testUser.ID)
	}
}

func TestUpdateNote(t *testing.T) {
	t.Parallel()

	t.Run("変えたメモを返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newNoteRouter(t, &fakeNoteService{}), http.MethodPatch, "/v1/notes/"+testNote.ID.String(), "Bearer "+validToken, `{"body":"新芽が出た"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := canonicalJSON(t, rec.Body.Bytes()); got != wantNoteJSON {
			t.Errorf("body = %s", got)
		}
	})

	t.Run("入力の誤りは 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newNoteRouter(t, &fakeNoteService{}), http.MethodPatch, "/v1/notes/"+testNote.ID.String(), "Bearer "+validToken, `{"body":""}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d (%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestDeleteNote(t *testing.T) {
	t.Parallel()

	t.Run("消すと 204", func(t *testing.T) {
		t.Parallel()
		notes := &fakeNoteService{}
		rec := serve(t, newNoteRouter(t, notes), http.MethodDelete, "/v1/notes/"+testNote.ID.String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusNoContent || len(notes.deleted) != 1 {
			t.Errorf("status = %d, deleted = %v", rec.Code, notes.deleted)
		}
	})

	t.Run("ほかの人のメモは 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newNoteRouter(t, &fakeNoteService{}), http.MethodDelete, "/v1/notes/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d (%s)", rec.Code, rec.Body.String())
		}
	})
}
