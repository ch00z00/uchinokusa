package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CalendarService は、植物ごとのカレンダーのデータを返す（実装は app.CalendarService）。
type CalendarService interface {
	Month(ctx context.Context, userID, plantID uuid.UUID, month *string) (domain.PlantCalendar, error)
}

func (s *server) GetPlantCalendar(ctx context.Context, req oapi.GetPlantCalendarRequestObject) (oapi.GetPlantCalendarResponseObject, error) {
	cal, err := s.calendar.Month(ctx, currentUser(ctx).ID, req.PlantId, req.Params.Month)
	if err != nil {
		return nil, err
	}
	out := oapi.GetPlantCalendar200JSONResponse{
		Month:   cal.Month.String(),
		Records: make([]oapi.CareRecord, len(cal.Records)),
		Plans:   make([]oapi.CalendarPlan, len(cal.Plans)),
		Notes:   make([]oapi.CalendarNote, len(cal.Notes)),
	}
	for i, r := range cal.Records {
		out.Records[i] = toCareRecord(r)
	}
	for i, p := range cal.Plans {
		out.Plans[i] = oapi.CalendarPlan{
			Date:       toDate(p.Date),
			Projected:  p.Projected,
			TaskId:     p.Task.ID,
			Kind:       oapi.CareKind(p.Task.Kind),
			CustomName: omitEmptyAs[string](p.Task.CustomName),
		}
	}
	for i, n := range cal.Notes {
		out.Notes[i] = oapi.CalendarNote{Date: toDate(n.Date), Note: toNote(n.Note)}
	}
	return out, nil
}
