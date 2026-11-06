package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SuggestionService は、提案の一覧・採用・見送りを扱う（実装は app.SuggestionService）。
type SuggestionService interface {
	List(ctx context.Context, userID, householdID uuid.UUID, date *string) ([]domain.Suggestion, error)
	Accept(ctx context.Context, userID, suggestionID uuid.UUID, in domain.AcceptInput) (domain.Suggestion, domain.CareTask, error)
	Dismiss(ctx context.Context, userID, suggestionID uuid.UUID) (domain.Suggestion, error)
}

func (s *server) ListSuggestions(ctx context.Context, req oapi.ListSuggestionsRequestObject) (oapi.ListSuggestionsResponseObject, error) {
	list, err := s.suggestions.List(ctx, currentUser(ctx).ID, req.HouseholdId, req.Params.Date)
	if err != nil {
		return nil, err
	}
	out := oapi.ListSuggestions200JSONResponse{Suggestions: make([]oapi.Suggestion, len(list))}
	for i, sg := range list {
		out.Suggestions[i] = toSuggestion(sg)
	}
	return out, nil
}

func (s *server) AcceptSuggestion(ctx context.Context, req oapi.AcceptSuggestionRequestObject) (oapi.AcceptSuggestionResponseObject, error) {
	var in domain.AcceptInput
	if req.Body != nil { // 本文は省いてもよい
		in.Date = req.Body.Date
	}
	sg, task, err := s.suggestions.Accept(ctx, currentUser(ctx).ID, req.SuggestionId, in)
	if err != nil {
		return nil, err
	}
	return oapi.AcceptSuggestion200JSONResponse{Suggestion: toSuggestion(sg), Task: omitZeroTask(task)}, nil
}

func (s *server) DismissSuggestion(ctx context.Context, req oapi.DismissSuggestionRequestObject) (oapi.DismissSuggestionResponseObject, error) {
	sg, err := s.suggestions.Dismiss(ctx, currentUser(ctx).ID, req.SuggestionId)
	if err != nil {
		return nil, err
	}
	return oapi.DismissSuggestion200JSONResponse(toSuggestion(sg)), nil
}

// toSuggestion は、未設定の項目（対象・作るタスク・期限・対応した日時）を、項目ごと省いた形にする。
func toSuggestion(sg domain.Suggestion) oapi.Suggestion {
	out := oapi.Suggestion{
		Id:            sg.ID,
		PlantId:       omitNilUUID(sg.PlantID),
		TaskId:        omitNilUUID(sg.TaskID),
		ProviderId:    sg.ProviderID,
		Kind:          oapi.SuggestionKind(sg.Kind),
		Action:        oapi.SuggestionAction(sg.Action),
		TaskKind:      omitEmptyAs[oapi.CareKind](sg.TaskKind),
		ConditionText: omitEmptyAs[string](sg.ConditionText),
		MessageKey:    sg.MessageKey,
		MessageParams: toParams(sg.MessageParams),
		ReasonKey:     sg.ReasonKey,
		ReasonParams:  toParams(sg.ReasonParams),
		ValidFrom:     toDate(sg.ValidFrom),
		Status:        oapi.SuggestionStatus(sg.Status),
	}
	if !sg.ProposedDate.IsZero() {
		d := toDate(sg.ProposedDate)
		out.ProposedDate = &d
	}
	if !sg.ValidUntil.IsZero() {
		until := toDate(sg.ValidUntil)
		out.ValidUntil = &until
	}
	if !sg.RespondedAt.IsZero() {
		at := sg.RespondedAt.UTC()
		out.RespondedAt = &at
	}
	return out
}
