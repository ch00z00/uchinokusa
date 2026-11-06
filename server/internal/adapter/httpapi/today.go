package httpapi

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// TodayService は、今日やること一覧を返す（実装は app.TodayService）。
type TodayService interface {
	List(ctx context.Context, userID, householdID uuid.UUID, date *string) (domain.TodayList, error)
}

func (s *server) GetToday(ctx context.Context, req oapi.GetTodayRequestObject) (oapi.GetTodayResponseObject, error) {
	list, err := s.today.List(ctx, currentUser(ctx).ID, req.HouseholdId, req.Params.Date)
	if err != nil {
		return nil, err
	}
	out := oapi.GetToday200JSONResponse{Date: toDate(list.Date), Items: make([]oapi.TodayItem, len(list.Items))}
	for i, item := range list.Items {
		out.Items[i] = toTodayItem(item)
	}
	if list.Next != nil {
		next := toTodayItem(*list.Next)
		out.Next = &next
	}
	return out, nil
}

func toTodayItem(item domain.TodayItem) oapi.TodayItem {
	out := oapi.TodayItem{
		Task: toCareTask(item.Task),
		Plant: oapi.TodayPlant{
			Id:           item.Task.PlantID,
			Name:         item.PlantName,
			LocationId:   omitNilUUID(item.LocationID),
			LocationName: omitEmptyAs[string](item.LocationName),
		},
		OverdueDays: item.OverdueDays,
		Suggestions: make([]oapi.Suggestion, len(item.Suggestions)),
	}
	for i, sg := range item.Suggestions {
		out.Suggestions[i] = toSuggestion(sg)
	}
	return out
}

// toParams は、i18n のパラメーター（DB の jsonb）を、レスポンスのオブジェクトにする。空や読めない値は {} にする。
func toParams(raw json.RawMessage) map[string]any {
	params := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &params) // DB の jsonb はオブジェクトとして保存しているので、通常は失敗しない
	}
	return params
}
