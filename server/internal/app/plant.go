package app

import (
	"context"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// PlantRepository は、植物の保存先である（実装は adapter/postgres）。
type PlantRepository interface {
	// List は、世帯の植物のうち、条件に合うものを返す。
	List(ctx context.Context, householdID uuid.UUID, filter domain.PlantFilter) ([]domain.Plant, error)
	// Create は、世帯に植物を登録する。種類・置き場所が存在しない（置き場所はほかの世帯のものも含む）場合は domain.ValidationError を返す。
	Create(ctx context.Context, householdID uuid.UUID, p domain.PlantProfile) (domain.Plant, error)
	// HouseholdOf は、植物が属する世帯の ID を返す。植物がなければ domain.ErrNotFound を返す。
	HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// Get は、世帯の植物を返す。世帯の植物でなければ domain.ErrNotFound を返す。
	Get(ctx context.Context, householdID, id uuid.UUID) (domain.Plant, error)
	// Update は、世帯の植物を変更する。種類・置き場所の誤りは、Create と同じく domain.ValidationError を返す。
	Update(ctx context.Context, householdID, id uuid.UUID, p domain.PlantPatch) (domain.Plant, error)
	// Delete は、世帯の植物を、タスク・記録・メモとともに削除する。
	Delete(ctx context.Context, householdID, id uuid.UUID) error
	// SetArchived は、植物をアーカイブする（archived が false なら元に戻す）。
	SetArchived(ctx context.Context, householdID, id uuid.UUID, archived bool) (domain.Plant, error)
}

// maxPlantQueryLength は、植物の名前の検索語の最大の文字数である。植物の名前より十分に長い。
const maxPlantQueryLength = 100

// PlantQuery は、植物の一覧の条件である。
type PlantQuery struct {
	LocationID uuid.UUID // uuid.Nil なら絞り込まない
	Q          string    // 名前の検索語（正規化する前）
	Archived   bool
}

// PlantService は、植物の登録・一覧・編集・削除・アーカイブを扱う（要件5）。
type PlantService struct {
	plants PlantRepository
	authz  *Authorizer
	advice AdviceEvaluator
}

func NewPlantService(plants PlantRepository, authz *Authorizer, advice AdviceEvaluator) *PlantService {
	return &PlantService{plants: plants, authz: authz, advice: advice}
}

// List は、世帯の植物を返す。閲覧者も見られる（要件5-4・5-5）。
// 名前の検索は、植物マスタの検索と同じく、ひらがな・カタカナなどの表記の揺れを区別しない。
func (s *PlantService) List(ctx context.Context, userID, householdID uuid.UUID, q PlantQuery) ([]domain.Plant, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRead); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(q.Q) > maxPlantQueryLength {
		return nil, &domain.ValidationError{Fields: map[string]string{"q": "too_long"}}
	}
	return s.plants.List(ctx, householdID, domain.PlantFilter{
		LocationID: q.LocationID,
		Query:      domain.NormalizeSearchText(q.Q),
		Archived:   q.Archived,
	})
}

// Create は、植物を登録する。編集者以上ができ、株数に上限はない（要件5-1〜5-3）。
// 登録したあとに、提案を評価する（購入日があれば、植え替えの提案ができる。要件17-1）。
// 作った提案は返さない。アプリは、提案の一覧を取り直して表示する。
func (s *PlantService) Create(ctx context.Context, userID, householdID uuid.UUID, in domain.PlantInput) (domain.Plant, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionPlantWrite); err != nil {
		return domain.Plant{}, err
	}
	p, err := domain.NewPlantProfile(in)
	if err != nil {
		return domain.Plant{}, err
	}
	plant, err := s.plants.Create(ctx, householdID, p)
	if err != nil {
		return domain.Plant{}, err
	}
	s.advice.Evaluate(ctx, AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: plant.ID})
	return plant, nil
}

// Get は、植物を返す。閲覧者も見られ、アーカイブした植物も見られる（記録とメモを閲覧するため。要件5-8）。
func (s *PlantService) Get(ctx context.Context, userID, plantID uuid.UUID) (domain.Plant, error) {
	householdID, err := s.authorize(ctx, userID, plantID, domain.ActionRead)
	if err != nil {
		return domain.Plant{}, err
	}
	return s.plants.Get(ctx, householdID, plantID)
}

// Update は、植物の情報を変更する。編集者以上ができる（要件5-6）。
func (s *PlantService) Update(ctx context.Context, userID, plantID uuid.UUID, in domain.PlantInput) (domain.Plant, error) {
	householdID, err := s.authorize(ctx, userID, plantID, domain.ActionPlantWrite)
	if err != nil {
		return domain.Plant{}, err
	}
	p, err := domain.NewPlantPatch(in)
	if err != nil {
		return domain.Plant{}, err
	}
	return s.plants.Update(ctx, householdID, plantID, p)
}

// Delete は、植物を、タスク・記録・メモとともに削除する。編集者以上ができる（要件5-7）。
// 関連データも消えることの確認は、アプリが行ってから呼ぶ。
func (s *PlantService) Delete(ctx context.Context, userID, plantID uuid.UUID) error {
	householdID, err := s.authorize(ctx, userID, plantID, domain.ActionPlantWrite)
	if err != nil {
		return err
	}
	return s.plants.Delete(ctx, householdID, plantID)
}

// Archive は、植物を「枯れた・手放した」としてアーカイブする。編集者以上ができる（要件5-8）。
// アーカイブした植物は、通常の一覧とリマインダーの対象から外れ、記録とメモは残る。
func (s *PlantService) Archive(ctx context.Context, userID, plantID uuid.UUID) (domain.Plant, error) {
	return s.setArchived(ctx, userID, plantID, true)
}

// Unarchive は、アーカイブを元に戻す。誤ってアーカイブしたときのためにある。編集者以上ができる。
func (s *PlantService) Unarchive(ctx context.Context, userID, plantID uuid.UUID) (domain.Plant, error) {
	return s.setArchived(ctx, userID, plantID, false)
}

func (s *PlantService) setArchived(ctx context.Context, userID, plantID uuid.UUID, archived bool) (domain.Plant, error) {
	householdID, err := s.authorize(ctx, userID, plantID, domain.ActionPlantWrite)
	if err != nil {
		return domain.Plant{}, err
	}
	return s.plants.SetArchived(ctx, householdID, plantID, archived)
}

// authorize は、植物の ID から世帯を探し、ユーザーがその世帯で action を行えることを確かめる。
// 植物がない場合と、所属していない世帯の植物の場合は、どちらも domain.ErrNotFound にする（存在を明かさない）。
func (s *PlantService) authorize(ctx context.Context, userID, plantID uuid.UUID, action domain.Action) (uuid.UUID, error) {
	householdID, err := s.plants.HouseholdOf(ctx, plantID)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, action); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}
