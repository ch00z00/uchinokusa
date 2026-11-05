package postgres_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// noteFixture は、メモのリポジトリの結合テストの共通の状態である。世帯（home）の植物と、メモを書くユーザーを用意する。
type noteFixture struct {
	plantFixture
	notes  *postgres.NoteRepository
	plant  uuid.UUID
	sakura uuid.UUID
}

func newNoteFixture(t *testing.T) noteFixture {
	t.Helper()
	f := noteFixture{plantFixture: newPlantFixture(t)}
	f.notes = postgres.NewNoteRepository(f.pool)
	f.plant = f.create("モンステラ", domain.PlantProfile{}).ID
	f.sakura = createUserNamed(t, f.pool, "sakura", "さくら")
	return f
}

func (f noteFixture) write(body string) domain.Note {
	f.t.Helper()
	n, err := f.notes.Create(f.ctx, f.home, f.plant, f.sakura, body)
	if err != nil {
		f.t.Fatal(err)
	}
	return n
}

func bodiesOf(notes []domain.Note) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.Body)
	}
	return out
}

func TestNoteRepository_Create(t *testing.T) {
	t.Parallel()
	f := newNoteFixture(t)

	n := f.write("新芽が出た")
	if n.HouseholdID != f.home || n.PlantID != f.plant || n.Body != "新芽が出た" ||
		n.AuthorUserID != (uuid.NullUUID{UUID: f.sakura, Valid: true}) || n.AuthorDisplayName != "さくら" ||
		!n.EditedAt.IsZero() || n.CreatedAt.IsZero() {
		t.Errorf("note = %+v", n)
	}

	// 作成者の名前は作成の時点の値を残し、アカウントを削除しても表示できる（要件13-2）
	f.exec(`UPDATE users SET display_name = 'さくらこ' WHERE id = $1`, f.sakura)
	f.exec(`DELETE FROM users WHERE id = $1`, f.sakura)
	got, err := f.notes.Get(f.ctx, f.home, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthorUserID.Valid || got.AuthorDisplayName != "さくら" {
		t.Errorf("after account deletion: %+v", got)
	}
}

func TestNoteRepository_ListByPlant(t *testing.T) {
	t.Parallel()
	f := newNoteFixture(t)
	a, b, c := f.write("a"), f.write("b"), f.write("c")
	// b と c は同じ作成日時にし、ID の新しい順（c が先）で並ぶことを確かめる
	f.exec(`UPDATE notes SET created_at = '2026-11-01T09:00:00Z' WHERE id = $1`, a.ID)
	f.exec(`UPDATE notes SET created_at = '2026-11-03T09:00:00Z' WHERE id = ANY($1)`, []uuid.UUID{b.ID, c.ID})
	// 編集しても、並びは作成日時のまま
	if _, err := f.notes.Update(f.ctx, f.home, a.ID, "a2"); err != nil {
		t.Fatal(err)
	}
	// ほかの植物・ほかの世帯のメモは返さない
	other := f.create("ポトス", domain.PlantProfile{}).ID
	if _, err := f.notes.Create(f.ctx, f.home, other, f.sakura, "ほかの植物"); err != nil {
		t.Fatal(err)
	}

	first, more, err := f.notes.ListByPlant(f.ctx, f.home, f.plant, domain.NoteQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"c", "b"}; !slices.Equal(bodiesOf(first), want) || !more {
		t.Errorf("first page = %v, more = %v; want %v, true", bodiesOf(first), more, want)
	}
	after := domain.NoteCursorOf(first[len(first)-1])
	second, more, err := f.notes.ListByPlant(f.ctx, f.home, f.plant, domain.NoteQuery{After: &after, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a2"}; !slices.Equal(bodiesOf(second), want) || more {
		t.Errorf("second page = %v, more = %v; want %v, false", bodiesOf(second), more, want)
	}

	none, _, err := f.notes.ListByPlant(f.ctx, f.other, f.plant, domain.NoteQuery{Limit: 10})
	if err != nil || len(none) != 0 {
		t.Errorf("other household: %v, %v", bodiesOf(none), err)
	}
}

func TestNoteRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("本文を変えると、編集した日時を付ける", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture(t)
		n := f.write("新芽が出た")
		got, err := f.notes.Update(f.ctx, f.home, n.ID, "新芽が2枚出た")
		if err != nil {
			t.Fatal(err)
		}
		if got.Body != "新芽が2枚出た" || got.EditedAt.IsZero() || !got.CreatedAt.Equal(n.CreatedAt) {
			t.Errorf("note = %+v", got)
		}
	})

	t.Run("同じ本文で保存しただけなら、編集済みにしない", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture(t)
		n := f.write("新芽が出た")
		got, err := f.notes.Update(f.ctx, f.home, n.ID, "新芽が出た")
		if err != nil {
			t.Fatal(err)
		}
		if !got.EditedAt.IsZero() {
			t.Errorf("editedAt = %v, want zero", got.EditedAt)
		}
	})

	t.Run("ほかの世帯のメモは見つからない", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture(t)
		n := f.write("新芽が出た")
		if _, err := f.notes.Update(f.ctx, f.other, n.ID, "x"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestNoteRepository_Delete(t *testing.T) {
	t.Parallel()
	f := newNoteFixture(t)
	n := f.write("新芽が出た")

	if err := f.notes.Delete(f.ctx, f.other, n.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other household: err = %v, want ErrNotFound", err)
	}
	if err := f.notes.Delete(f.ctx, f.home, n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.notes.Get(f.ctx, f.home, n.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}
	if err := f.notes.Delete(f.ctx, f.home, n.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("twice: err = %v, want ErrNotFound", err)
	}
}

func TestNoteRepository_HouseholdOf(t *testing.T) {
	t.Parallel()
	f := newNoteFixture(t)
	n := f.write("新芽が出た")

	if got, err := f.notes.HouseholdOf(f.ctx, n.ID); err != nil || got != f.home {
		t.Errorf("got %v, %v; want %v", got, err, f.home)
	}
	if _, err := f.notes.HouseholdOf(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
