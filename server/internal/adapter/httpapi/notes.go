package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// NoteService は、植物ごとのメモを扱う（実装は app.NoteService）。
type NoteService interface {
	List(ctx context.Context, userID, plantID uuid.UUID, in domain.NoteQueryInput) (domain.NotePage, error)
	Create(ctx context.Context, userID, plantID uuid.UUID, body *string) (domain.Note, error)
	Update(ctx context.Context, userID, noteID uuid.UUID, body *string) (domain.Note, error)
	Delete(ctx context.Context, userID, noteID uuid.UUID) error
}

func (s *server) ListPlantNotes(ctx context.Context, req oapi.ListPlantNotesRequestObject) (oapi.ListPlantNotesResponseObject, error) {
	page, err := s.notes.List(ctx, currentUser(ctx).ID, req.PlantId, domain.NoteQueryInput{Cursor: req.Params.Cursor, Limit: req.Params.Limit})
	if err != nil {
		return nil, err
	}
	out := oapi.ListPlantNotes200JSONResponse{Notes: make([]oapi.Note, len(page.Notes)), NextCursor: omitEmptyAs[string](page.NextCursor)}
	for i, n := range page.Notes {
		out.Notes[i] = toNote(n)
	}
	return out, nil
}

func (s *server) CreateNote(ctx context.Context, req oapi.CreateNoteRequestObject) (oapi.CreateNoteResponseObject, error) {
	n, err := s.notes.Create(ctx, currentUser(ctx).ID, req.PlantId, &req.Body.Body)
	if err != nil {
		return nil, err
	}
	return oapi.CreateNote201JSONResponse(toNote(n)), nil
}

func (s *server) UpdateNote(ctx context.Context, req oapi.UpdateNoteRequestObject) (oapi.UpdateNoteResponseObject, error) {
	n, err := s.notes.Update(ctx, currentUser(ctx).ID, req.NoteId, &req.Body.Body)
	if err != nil {
		return nil, err
	}
	return oapi.UpdateNote200JSONResponse(toNote(n)), nil
}

func (s *server) DeleteNote(ctx context.Context, req oapi.DeleteNoteRequestObject) (oapi.DeleteNoteResponseObject, error) {
	if err := s.notes.Delete(ctx, currentUser(ctx).ID, req.NoteId); err != nil {
		return nil, err
	}
	return oapi.DeleteNote204Response{}, nil
}

// toNote は、退会した作成者の ID と、編集していないメモの編集日時を、項目ごと省いた形にする。
func toNote(n domain.Note) oapi.Note {
	out := oapi.Note{
		Id:                n.ID,
		PlantId:           n.PlantID,
		Body:              n.Body,
		AuthorDisplayName: n.AuthorDisplayName,
		CreatedAt:         n.CreatedAt.UTC(),
	}
	if n.AuthorUserID.Valid {
		out.AuthorUserId = &n.AuthorUserID.UUID
	}
	if !n.EditedAt.IsZero() {
		editedAt := n.EditedAt.UTC()
		out.EditedAt = &editedAt
	}
	return out
}
