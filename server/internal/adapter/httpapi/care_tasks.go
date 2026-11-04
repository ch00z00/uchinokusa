package httpapi

import (
	"context"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CareTaskService は、お手入れタスクの作成・一覧・変更・削除・一時停止・再開を行う（実装は app.CareTaskService）。
type CareTaskService interface {
	List(ctx context.Context, userID, plantID uuid.UUID) ([]domain.CareTask, error)
	Defaults(ctx context.Context, userID, plantID uuid.UUID) ([]domain.TaskDefault, error)
	Create(ctx context.Context, userID, plantID uuid.UUID, in domain.CareTaskInput) (domain.CareTask, error)
	Update(ctx context.Context, userID, taskID uuid.UUID, in domain.CareTaskInput) (domain.CareTask, error)
	Delete(ctx context.Context, userID, taskID uuid.UUID) error
	Pause(ctx context.Context, userID, taskID uuid.UUID) (domain.CareTask, error)
	Resume(ctx context.Context, userID, taskID uuid.UUID) (domain.CareTask, error)
}

func (s *server) ListCareTasks(ctx context.Context, req oapi.ListCareTasksRequestObject) (oapi.ListCareTasksResponseObject, error) {
	list, err := s.tasks.List(ctx, currentUser(ctx).ID, req.PlantId)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.CareTask, 0, len(list))
	for _, t := range list {
		out = append(out, toCareTask(t))
	}
	return oapi.ListCareTasks200JSONResponse{Tasks: out}, nil
}

func (s *server) ListCareTaskDefaults(ctx context.Context, req oapi.ListCareTaskDefaultsRequestObject) (oapi.ListCareTaskDefaultsResponseObject, error) {
	list, err := s.tasks.Defaults(ctx, currentUser(ctx).ID, req.PlantId)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.CareTaskDefault, 0, len(list))
	for _, d := range list {
		out = append(out, oapi.CareTaskDefault{Kind: oapi.CareKind(d.Kind), ConditionText: omitEmptyAs[string](d.ConditionText)})
	}
	return oapi.ListCareTaskDefaults200JSONResponse{Defaults: out}, nil
}

func (s *server) CreateCareTask(ctx context.Context, req oapi.CreateCareTaskRequestObject) (oapi.CreateCareTaskResponseObject, error) {
	b := req.Body
	t, err := s.tasks.Create(ctx, currentUser(ctx).ID, req.PlantId, domain.CareTaskInput{
		Kind:              &b.Kind,
		CustomName:        b.CustomName,
		Interval:          toIntervalInput(&b.Interval),
		SeasonalIntervals: toSeasonalInput(b.SeasonalIntervals),
		ConditionText:     b.ConditionText,
		Note:              b.Note,
		NextDueOn:         b.NextDueOn,
	})
	if err != nil {
		return nil, err
	}
	return oapi.CreateCareTask201JSONResponse(toCareTask(t)), nil
}

func (s *server) UpdateCareTask(ctx context.Context, req oapi.UpdateCareTaskRequestObject) (oapi.UpdateCareTaskResponseObject, error) {
	b := req.Body
	t, err := s.tasks.Update(ctx, currentUser(ctx).ID, req.TaskId, domain.CareTaskInput{
		CustomName:        b.CustomName,
		Interval:          toIntervalInput(b.Interval),
		SeasonalIntervals: toSeasonalInput(b.SeasonalIntervals),
		ConditionText:     b.ConditionText,
		Note:              b.Note,
		NextDueOn:         b.NextDueOn,
	})
	if err != nil {
		return nil, err
	}
	return oapi.UpdateCareTask200JSONResponse(toCareTask(t)), nil
}

func (s *server) DeleteCareTask(ctx context.Context, req oapi.DeleteCareTaskRequestObject) (oapi.DeleteCareTaskResponseObject, error) {
	if err := s.tasks.Delete(ctx, currentUser(ctx).ID, req.TaskId); err != nil {
		return nil, err
	}
	return oapi.DeleteCareTask204Response{}, nil
}

func (s *server) PauseCareTask(ctx context.Context, req oapi.PauseCareTaskRequestObject) (oapi.PauseCareTaskResponseObject, error) {
	t, err := s.tasks.Pause(ctx, currentUser(ctx).ID, req.TaskId)
	if err != nil {
		return nil, err
	}
	return oapi.PauseCareTask200JSONResponse(toCareTask(t)), nil
}

func (s *server) ResumeCareTask(ctx context.Context, req oapi.ResumeCareTaskRequestObject) (oapi.ResumeCareTaskResponseObject, error) {
	t, err := s.tasks.Resume(ctx, currentUser(ctx).ID, req.TaskId)
	if err != nil {
		return nil, err
	}
	return oapi.ResumeCareTask200JSONResponse(toCareTask(t)), nil
}

func toIntervalInput(i *oapi.IntervalInput) *domain.IntervalInput {
	if i == nil {
		return nil
	}
	return &domain.IntervalInput{Value: i.Value, Unit: i.Unit}
}

// toSeasonalInput は、送られた季節だけを map にする。項目ごと送られなければ nil、{} なら空の map（季節ごとの周期をなくす）にする。
func toSeasonalInput(s *oapi.SeasonalIntervalsInput) map[string]domain.IntervalInput {
	if s == nil {
		return nil
	}
	out := map[string]domain.IntervalInput{}
	for season, i := range map[domain.Season]*oapi.IntervalInput{
		domain.SeasonSpring: s.Spring, domain.SeasonSummer: s.Summer, domain.SeasonAutumn: s.Autumn, domain.SeasonWinter: s.Winter,
	} {
		if i != nil {
			out[string(season)] = *toIntervalInput(i)
		}
	}
	return out
}

// toCareTask は、未設定の項目を、null ではなく項目ごと省いた形にする。季節ごとの周期は、なければ {} にする。
func toCareTask(t domain.CareTask) oapi.CareTask {
	out := oapi.CareTask{
		Id:            t.ID,
		PlantId:       t.PlantID,
		Kind:          oapi.CareKind(t.Kind),
		CustomName:    omitEmptyAs[string](t.CustomName),
		Interval:      toInterval(t.Schedule.Base),
		ConditionText: omitEmptyAs[string](t.ConditionText),
		Note:          omitEmptyAs[string](t.Note),
		CycleSeq:      t.CycleSeq,
		Status:        oapi.TaskStatus(t.Status),
		CreatedAt:     t.CreatedAt.UTC(),
	}
	for season, i := range t.Schedule.Seasonal {
		v := toInterval(i)
		switch season {
		case domain.SeasonSpring:
			out.SeasonalIntervals.Spring = &v
		case domain.SeasonSummer:
			out.SeasonalIntervals.Summer = &v
		case domain.SeasonAutumn:
			out.SeasonalIntervals.Autumn = &v
		case domain.SeasonWinter:
			out.SeasonalIntervals.Winter = &v
		}
	}
	if !t.NextDueOn.IsZero() {
		out.NextDueOn = &openapi_types.Date{Time: t.NextDueOn.In(time.UTC)}
	}
	return out
}

func toInterval(i domain.Interval) oapi.Interval {
	return oapi.Interval{Value: i.Value, Unit: oapi.IntervalUnit(i.Unit)}
}
