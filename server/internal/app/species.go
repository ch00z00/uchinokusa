package app

import (
	"context"
	"unicode/utf8"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SpeciesRepository は、植物マスタの保存先である（実装は adapter/postgres）。
type SpeciesRepository interface {
	// Search は、正規化した検索語を和名・学名・別名に含む種類を、limit 件まで返す。
	Search(ctx context.Context, normalizedQuery string, limit int) ([]domain.Species, error)
}

const (
	// maxSpeciesQueryLength は、検索語の最大の文字数である。種類の名前より十分に長い。
	maxSpeciesQueryLength = 100
	// speciesSearchLimit は、検索結果の最大の件数である。候補の一覧に収まる数にする。
	speciesSearchLimit = 20
)

// SpeciesService は、植物マスタの検索を扱う（要件6-2）。植物マスタは全ユーザー共通なので、世帯の権限は確かめない。
type SpeciesService struct {
	species SpeciesRepository
}

func NewSpeciesService(species SpeciesRepository) *SpeciesService {
	return &SpeciesService{species: species}
}

// Search は、和名・学名・別名で植物マスタを探す。表記の揺れ（ひらがな・カタカナなど）は区別しない。
func (s *SpeciesService) Search(ctx context.Context, q string) ([]domain.Species, error) {
	if utf8.RuneCountInString(q) > maxSpeciesQueryLength {
		return nil, &domain.ValidationError{Fields: map[string]string{"q": "too_long"}}
	}
	return s.species.Search(ctx, domain.NormalizeSearchText(q), speciesSearchLimit)
}
