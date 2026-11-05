package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeNotes は、app.NoteRepository の偽物である。メモをメモリに持ち、呼ばれた操作を記録する。
type fakeNotes struct {
	byID    map[uuid.UUID]domain.Note
	queries []domain.NoteQuery
	more    bool
	updated []string
	deleted []uuid.UUID
}

func (f *fakeNotes) Create(_ context.Context, householdID, plantID, authorID uuid.UUID, body string) (domain.Note, error) {
	n := domain.Note{ID: uuid.Must(uuid.NewV7()), HouseholdID: householdID, PlantID: plantID, AuthorUserID: uuid.NullUUID{UUID: authorID, Valid: true}, Body: body}
	f.byID[n.ID] = n
	return n, nil
}

func (f *fakeNotes) HouseholdOf(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	n, ok := f.byID[id]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return n.HouseholdID, nil
}

func (f *fakeNotes) Get(_ context.Context, householdID, id uuid.UUID) (domain.Note, error) {
	n, ok := f.byID[id]
	if !ok || n.HouseholdID != householdID {
		return domain.Note{}, domain.ErrNotFound
	}
	return n, nil
}

func (f *fakeNotes) Update(ctx context.Context, householdID, id uuid.UUID, body string) (domain.Note, error) {
	n, err := f.Get(ctx, householdID, id)
	if err != nil {
		return domain.Note{}, err
	}
	n.Body = body
	f.byID[id] = n
	f.updated = append(f.updated, body)
	return n, nil
}

func (f *fakeNotes) Delete(ctx context.Context, householdID, id uuid.UUID) error {
	if _, err := f.Get(ctx, householdID, id); err != nil {
		return err
	}
	delete(f.byID, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeNotes) ListByPlant(_ context.Context, householdID, plantID uuid.UUID, q domain.NoteQuery) ([]domain.Note, bool, error) {
	f.queries = append(f.queries, q)
	out := []domain.Note{}
	for _, n := range f.byID {
		if n.HouseholdID == householdID && n.PlantID == plantID {
			out = append(out, n)
		}
	}
	return out, f.more, nil
}

// fakePlantLocator は、app.PlantLocator の偽物である。
type fakePlantLocator map[uuid.UUID]uuid.UUID

func (f fakePlantLocator) HouseholdOf(_ context.Context, plantID uuid.UUID) (uuid.UUID, error) {
	h, ok := f[plantID]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return h, nil
}

// noteFixture は、householdFixture の世帯に植物を1つと、編集者が書いたメモを1つ用意する。
type noteFixture struct {
	householdFixture
	plant, editorNote uuid.UUID
	notes             *fakeNotes
	svc               *app.NoteService
}

func newNoteFixture() noteFixture {
	hf := newHouseholdFixture()
	f := noteFixture{householdFixture: hf, plant: uuid.Must(uuid.NewV7()), editorNote: uuid.Must(uuid.NewV7()), notes: &fakeNotes{byID: map[uuid.UUID]domain.Note{}}}
	f.notes.byID[f.editorNote] = domain.Note{
		ID: f.editorNote, HouseholdID: hf.household, PlantID: f.plant,
		AuthorUserID: uuid.NullUUID{UUID: hf.editor, Valid: true}, Body: "新芽が出た", CreatedAt: time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC),
	}
	f.svc = app.NewNoteService(f.notes, fakePlantLocator{f.plant: hf.household}, hf.authz)
	return f
}

func TestNoteService_Create(t *testing.T) {
	t.Parallel()

	t.Run("編集者は、自分を作成者としてメモを作れる", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		got, err := f.svc.Create(t.Context(), f.editor, f.plant, ptr(" 葉水をした "))
		if err != nil {
			t.Fatal(err)
		}
		if got.HouseholdID != f.household || got.PlantID != f.plant || got.AuthorUserID.UUID != f.editor || got.Body != "葉水をした" {
			t.Errorf("note = %+v", got)
		}
	})

	t.Run("空のメモは required", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		_, err := f.svc.Create(t.Context(), f.editor, f.plant, ptr("  "))
		assertFields(t, err, map[string]string{"body": "required"})
	})

	t.Run("閲覧者は作れない", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		if _, err := f.svc.Create(t.Context(), f.viewer, f.plant, ptr("メモ")); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("メンバーでない人には、入力の誤りより先に 404 を返す", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		if _, err := f.svc.Create(t.Context(), f.stranger, f.plant, nil); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("植物がなければ 404", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		if _, err := f.svc.Create(t.Context(), f.editor, uuid.Must(uuid.NewV7()), ptr("メモ")); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestNoteService_List(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者も見られ、続きがあれば最後のメモの位置を返す", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		f.notes.more = true
		limit := 1
		page, err := f.svc.List(t.Context(), f.viewer, f.plant, domain.NoteQueryInput{Limit: &limit})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Notes) != 1 || page.Notes[0].ID != f.editorNote {
			t.Fatalf("notes = %+v", page.Notes)
		}
		if want := domain.NoteCursorOf(page.Notes[0]).Encode(); page.NextCursor != want {
			t.Errorf("nextCursor = %q, want %q", page.NextCursor, want)
		}
		if len(f.notes.queries) != 1 || f.notes.queries[0].Limit != 1 {
			t.Errorf("queries = %+v", f.notes.queries)
		}
	})

	t.Run("続きがなければ位置を返さない", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		page, err := f.svc.List(t.Context(), f.viewer, f.plant, domain.NoteQueryInput{})
		if err != nil || page.NextCursor != "" {
			t.Errorf("page = %+v, %v", page, err)
		}
	})

	t.Run("メンバーでなければ 404", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		if _, err := f.svc.List(t.Context(), f.stranger, f.plant, domain.NoteQueryInput{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestNoteService_Update(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		user    func(noteFixture) uuid.UUID
		wantErr error
	}{
		{"作成者本人は編集できる", func(f noteFixture) uuid.UUID { return f.editor }, nil},
		{"オーナーは、ほかの人のメモも編集できる", func(f noteFixture) uuid.UUID { return f.owner }, nil},
		{"閲覧者は編集できない", func(f noteFixture) uuid.UUID { return f.viewer }, domain.ErrForbidden},
		{"メンバーでなければ 404", func(f noteFixture) uuid.UUID { return f.stranger }, domain.ErrNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newNoteFixture()
			got, err := f.svc.Update(t.Context(), tt.user(f), f.editorNote, ptr(" 新芽が2枚出た "))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.Body != "新芽が2枚出た" {
				t.Errorf("note = %+v", got)
			}
		})
	}

	t.Run("ほかの編集者のメモは、編集者には編集できない", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		other := uuid.Must(uuid.NewV7())
		n := f.notes.byID[f.editorNote]
		n.AuthorUserID = uuid.NullUUID{UUID: other, Valid: true}
		f.notes.byID[f.editorNote] = n
		if _, err := f.svc.Update(t.Context(), f.editor, f.editorNote, ptr("x")); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
		if len(f.notes.updated) != 0 {
			t.Errorf("updated = %v", f.notes.updated)
		}
	})

	t.Run("権限のない人には、入力の誤りより先に 403 を返す", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		if _, err := f.svc.Update(t.Context(), f.viewer, f.editorNote, nil); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("上限を超える本文は too_long", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		_, err := f.svc.Update(t.Context(), f.editor, f.editorNote, ptr(strings.Repeat("葉", domain.MaxNoteBodyLength+1)))
		assertFields(t, err, map[string]string{"body": "too_long"})
	})

	t.Run("メモがなければ 404", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture()
		if _, err := f.svc.Update(t.Context(), f.owner, uuid.Must(uuid.NewV7()), ptr("x")); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestNoteService_Delete(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		user    func(noteFixture) uuid.UUID
		wantErr error
	}{
		{"作成者本人は消せる", func(f noteFixture) uuid.UUID { return f.editor }, nil},
		{"オーナーは、ほかの人のメモも消せる", func(f noteFixture) uuid.UUID { return f.owner }, nil},
		{"閲覧者は消せない", func(f noteFixture) uuid.UUID { return f.viewer }, domain.ErrForbidden},
		{"メンバーでなければ 404", func(f noteFixture) uuid.UUID { return f.stranger }, domain.ErrNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newNoteFixture()
			err := f.svc.Delete(t.Context(), tt.user(f), f.editorNote)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if deleted := len(f.notes.deleted) == 1; deleted != (tt.wantErr == nil) {
				t.Errorf("deleted = %v", f.notes.deleted)
			}
		})
	}
}
