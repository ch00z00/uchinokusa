package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeSpecies は、app.SpeciesRepository の偽物である。受け取った検索語と件数を記録する。
type fakeSpecies struct {
	query string
	limit int
}

func (f *fakeSpecies) Search(_ context.Context, normalizedQuery string, limit int) ([]domain.Species, error) {
	f.query, f.limit = normalizedQuery, limit
	return []domain.Species{}, nil
}

func TestSpeciesService_Search(t *testing.T) {
	t.Parallel()

	t.Run("検索語を正規化して、20件まで探す", func(t *testing.T) {
		t.Parallel()
		repo := &fakeSpecies{}
		if _, err := app.NewSpeciesService(repo).Search(t.Context(), " もんすてら "); err != nil {
			t.Fatal(err)
		}
		if repo.query != "モンステラ" || repo.limit != 20 {
			t.Errorf("query = %q, limit = %d", repo.query, repo.limit)
		}
	})

	t.Run("長すぎる検索語は、探さずに検証のエラー", func(t *testing.T) {
		t.Parallel()
		repo := &fakeSpecies{}
		_, err := app.NewSpeciesService(repo).Search(t.Context(), strings.Repeat("あ", 101))
		assertFields(t, err, map[string]string{"q": "too_long"})
		if repo.limit != 0 {
			t.Error("repository was called")
		}
	})
}
