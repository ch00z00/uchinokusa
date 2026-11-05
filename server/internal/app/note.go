package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// NoteRepository は、メモの保存先である（実装は adapter/postgres）。
type NoteRepository interface {
	// Create は、世帯の植物にメモを作る。作成者の名前は、作成の時点の値を写す。
	Create(ctx context.Context, householdID, plantID, authorID uuid.UUID, body string) (domain.Note, error)
	// HouseholdOf は、メモが属する世帯の ID を返す。メモがなければ domain.ErrNotFound を返す。
	HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// Get は、世帯のメモを返す。世帯のメモでなければ domain.ErrNotFound を返す。
	Get(ctx context.Context, householdID, id uuid.UUID) (domain.Note, error)
	// Update は、メモの本文を変える。本文が変わったときだけ、編集した日時を付ける。
	Update(ctx context.Context, householdID, id uuid.UUID, body string) (domain.Note, error)
	Delete(ctx context.Context, householdID, id uuid.UUID) error
	// ListByPlant は、植物のメモを新しい順に q.Limit 件まで返す。続きがあれば more を true にする。
	ListByPlant(ctx context.Context, householdID, plantID uuid.UUID, q domain.NoteQuery) (notes []domain.Note, more bool, err error)
}

// PlantLocator は、植物の ID から、権限を確かめる世帯を探す（実装は PlantRepository）。
type PlantLocator interface {
	HouseholdOf(ctx context.Context, plantID uuid.UUID) (uuid.UUID, error)
}

// NoteService は、植物ごとのメモの一覧・作成・編集・削除を扱う（要件13）。
type NoteService struct {
	notes  NoteRepository
	plants PlantLocator
	authz  *Authorizer
}

func NewNoteService(notes NoteRepository, plants PlantLocator, authz *Authorizer) *NoteService {
	return &NoteService{notes: notes, plants: plants, authz: authz}
}

// List は、植物のメモを新しい順に1ページ返す（要件13-2）。閲覧者も見られる。
func (s *NoteService) List(ctx context.Context, userID, plantID uuid.UUID, in domain.NoteQueryInput) (domain.NotePage, error) {
	householdID, err := s.authorizePlant(ctx, userID, plantID, domain.ActionRead)
	if err != nil {
		return domain.NotePage{}, err
	}
	q, err := domain.NewNoteQuery(in)
	if err != nil {
		return domain.NotePage{}, err
	}
	notes, more, err := s.notes.ListByPlant(ctx, householdID, plantID, q)
	if err != nil {
		return domain.NotePage{}, err
	}
	page := domain.NotePage{Notes: notes}
	if more && len(notes) > 0 {
		page.NextCursor = domain.NoteCursorOf(notes[len(notes)-1]).Encode()
	}
	return page, nil
}

// Create は、植物にメモを作り、リクエストした人を作成者にする。編集者以上ができる（要件13-1・13-4）。
func (s *NoteService) Create(ctx context.Context, userID, plantID uuid.UUID, body *string) (domain.Note, error) {
	householdID, err := s.authorizePlant(ctx, userID, plantID, domain.ActionNoteWrite)
	if err != nil {
		return domain.Note{}, err
	}
	b, err := domain.NewNoteBody(body)
	if err != nil {
		return domain.Note{}, err
	}
	return s.notes.Create(ctx, householdID, plantID, userID, b)
}

// Update は、メモの本文を変える。作成者本人かオーナーができる（要件13-3・13-4）。
func (s *NoteService) Update(ctx context.Context, userID, noteID uuid.UUID, body *string) (domain.Note, error) {
	householdID, err := s.authorizeNote(ctx, userID, noteID)
	if err != nil {
		return domain.Note{}, err
	}
	b, err := domain.NewNoteBody(body)
	if err != nil {
		return domain.Note{}, err
	}
	return s.notes.Update(ctx, householdID, noteID, b)
}

// Delete は、メモを消す。作成者本人かオーナーができる（要件13-3）。
func (s *NoteService) Delete(ctx context.Context, userID, noteID uuid.UUID) error {
	householdID, err := s.authorizeNote(ctx, userID, noteID)
	if err != nil {
		return err
	}
	return s.notes.Delete(ctx, householdID, noteID)
}

// authorizePlant は、植物の世帯で、操作の権限を確かめる。入力の検証より先に行う。
func (s *NoteService) authorizePlant(ctx context.Context, userID, plantID uuid.UUID, action domain.Action) (uuid.UUID, error) {
	householdID, err := s.plants.HouseholdOf(ctx, plantID)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, action); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}

// authorizeNote は、メモの世帯で note.write を確かめ、さらに作成者本人かオーナーであることを確かめる（domain.Membership.CanModifyNote）。
// 入力の検証より先に行う。作成者は変わらないので、確かめたあとに変える間の食い違いは起きない。
func (s *NoteService) authorizeNote(ctx context.Context, userID, noteID uuid.UUID) (uuid.UUID, error) {
	householdID, err := s.notes.HouseholdOf(ctx, noteID)
	if err != nil {
		return uuid.Nil, err
	}
	m, err := s.authz.Require(ctx, userID, householdID, domain.ActionNoteWrite)
	if err != nil {
		return uuid.Nil, err
	}
	n, err := s.notes.Get(ctx, householdID, noteID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := m.CanModifyNote(n.AuthorUserID); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}
