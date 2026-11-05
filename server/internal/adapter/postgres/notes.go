package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// NoteRepository は、notes テーブルへのアクセスを提供する。
type NoteRepository struct {
	q *sqlcgen.Queries
}

func NewNoteRepository(pool *pgxpool.Pool) *NoteRepository {
	return &NoteRepository{q: sqlcgen.New(pool)}
}

// Create は、世帯の植物にメモを作る。作成者の名前は、作成の時点の値を写す。
func (r *NoteRepository) Create(ctx context.Context, householdID, plantID, authorID uuid.UUID, body string) (domain.Note, error) {
	row, err := r.q.CreateNote(ctx, sqlcgen.CreateNoteParams{HouseholdID: householdID, PlantID: plantID, AuthorUserID: authorID, Body: body})
	if err != nil {
		return domain.Note{}, fmt.Errorf("create note: %w", err)
	}
	return toDomainNote(row), nil
}

// HouseholdOf は、メモが属する世帯の ID を返す。メモがなければ domain.ErrNotFound を返す。
func (r *NoteRepository) HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	householdID, err := r.q.GetNoteHouseholdID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get note household: %w", err)
	}
	return householdID, nil
}

// Get は、世帯のメモを返す。世帯のメモでなければ domain.ErrNotFound を返す。
func (r *NoteRepository) Get(ctx context.Context, householdID, id uuid.UUID) (domain.Note, error) {
	row, err := r.q.GetNote(ctx, sqlcgen.GetNoteParams{ID: id, HouseholdID: householdID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, fmt.Errorf("get note: %w", err)
	}
	return toDomainNote(row), nil
}

// Update は、メモの本文を変える。本文が変わったときだけ、編集した日時を付ける。世帯のメモでなければ domain.ErrNotFound を返す。
func (r *NoteRepository) Update(ctx context.Context, householdID, id uuid.UUID, body string) (domain.Note, error) {
	row, err := r.q.UpdateNoteBody(ctx, sqlcgen.UpdateNoteBodyParams{ID: id, HouseholdID: householdID, Body: body})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Note{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Note{}, fmt.Errorf("update note: %w", err)
	}
	return toDomainNote(row), nil
}

// Delete は、メモを消す。世帯のメモでなければ domain.ErrNotFound を返す。
func (r *NoteRepository) Delete(ctx context.Context, householdID, id uuid.UUID) error {
	n, err := r.q.DeleteNote(ctx, sqlcgen.DeleteNoteParams{ID: id, HouseholdID: householdID})
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListByPlant は、植物のメモを新しい順に q.Limit 件まで返す。続きがあれば more を true にする。
// 続きを確かめるため、1件多く読んでから切り詰める。
func (r *NoteRepository) ListByPlant(ctx context.Context, householdID, plantID uuid.UUID, q domain.NoteQuery) ([]domain.Note, bool, error) {
	params := sqlcgen.ListPlantNotesParams{HouseholdID: householdID, PlantID: plantID, RowLimit: int32(q.Limit + 1)}
	if q.After != nil {
		params.AfterCreatedAt = &q.After.CreatedAt
		params.AfterID = nullUUID(q.After.ID)
	}
	rows, err := r.q.ListPlantNotes(ctx, params)
	if err != nil {
		return nil, false, fmt.Errorf("list plant notes: %w", err)
	}
	more := len(rows) > q.Limit
	if more {
		rows = rows[:q.Limit]
	}
	out := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainNote(row))
	}
	return out, more, nil
}

func toDomainNote(row sqlcgen.Note) domain.Note {
	n := domain.Note{
		ID:                row.ID,
		HouseholdID:       row.HouseholdID,
		PlantID:           row.PlantID,
		AuthorUserID:      row.AuthorUserID,
		AuthorDisplayName: row.AuthorDisplayName,
		Body:              row.Body,
		CreatedAt:         row.CreatedAt,
	}
	if row.EditedAt != nil {
		n.EditedAt = *row.EditedAt
	}
	return n
}
