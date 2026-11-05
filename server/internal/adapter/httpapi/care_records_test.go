package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testRecord = domain.CareRecord{
	ID:               uuid.MustParse("01920000-0000-7000-8000-0000000000f1"),
	HouseholdID:      testHousehold.ID,
	PlantID:          testPlant.ID,
	TaskID:           testTask.ID,
	TaskKind:         domain.CareWatering,
	Outcome:          domain.OutcomeCompleted,
	PerformedOn:      civil.Date{Year: 2026, Month: 11, Day: 4},
	PerformedAt:      time.Date(2026, 11, 4, 9, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
	ActorUserID:      uuid.MustParse("01920000-0000-7000-8000-0000000000a1"),
	ActorDisplayName: "さくら",
	CycleSeq:         3,
	CreatedAt:        time.Date(2026, 11, 4, 9, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// fakeCareRecordService は、app.CareRecordService の偽物である。受け取った入力を記録する。
// 送られた cycleSeq で振る舞いを変える：3 は成功、2 は完了済み、それ以外は予定の回が合わない。ほかのタスクは権限なし。
type fakeCareRecordService struct {
	inputs []domain.CompletionInput
	skips  []domain.SkipInput
}

// Skip は、cycleSeq が 3 なら成功、それ以外は予定の回が合わない。
func (f *fakeCareRecordService) Skip(_ context.Context, _, taskID uuid.UUID, in domain.SkipInput) (domain.CareRecord, domain.CareTask, error) {
	f.skips = append(f.skips, in)
	if taskID != testTask.ID {
		return domain.CareRecord{}, domain.CareTask{}, domain.ErrForbidden
	}
	if *in.CycleSeq != 3 {
		return domain.CareRecord{}, domain.CareTask{}, &domain.ConflictError{Code: domain.ConflictStaleCycle}
	}
	rec := testRecord
	rec.Outcome, rec.SkipReason, rec.SkipReasonText, rec.RecheckOn = domain.OutcomeSkipped, domain.SkipOther, "旅行中", civil.Date{Year: 2026, Month: 11, Day: 5}
	deferred := testTask
	deferred.NextDueOn = rec.RecheckOn
	return rec, deferred, nil
}

func (f *fakeCareRecordService) Complete(_ context.Context, _, taskID uuid.UUID, in domain.CompletionInput) (domain.CareRecord, domain.CareTask, error) {
	f.inputs = append(f.inputs, in)
	if taskID != testTask.ID {
		return domain.CareRecord{}, domain.CareTask{}, domain.ErrForbidden
	}
	advanced := testTask
	advanced.CycleSeq = 4
	switch *in.CycleSeq {
	case 3:
		return testRecord, advanced, nil
	case 2:
		return domain.CareRecord{}, domain.CareTask{}, &domain.AlreadyCompletedError{Existing: testRecord, Task: advanced}
	default:
		return domain.CareRecord{}, domain.CareTask{}, &domain.ConflictError{Code: domain.ConflictStaleCycle}
	}
}

func newRecordRouter(t *testing.T, records *fakeCareRecordService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Records: records})
}

func TestCompleteCareTask(t *testing.T) {
	t.Parallel()
	path := "/v1/tasks/" + testTask.ID.String() + "/completions"

	t.Run("作った記録と更新後のタスクを 201 で返す", func(t *testing.T) {
		t.Parallel()
		records := &fakeCareRecordService{}
		rec := serve(t, newRecordRouter(t, records), http.MethodPost, path, "Bearer "+validToken, `{"cycleSeq": 3, "performedOn": "2026-11-04", "allowExtra": true}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		const wantRecord = `{"actorDisplayName":"さくら","actorUserId":"01920000-0000-7000-8000-0000000000a1","createdAt":"2026-11-04T00:30:00Z",` +
			`"cycleSeq":3,"id":"01920000-0000-7000-8000-0000000000f1","isExtra":false,"kind":"watering","outcome":"completed",` +
			`"performedAt":"2026-11-04T00:30:00Z","performedOn":"2026-11-04","plantId":"01920000-0000-7000-8000-0000000000d1","taskId":"01920000-0000-7000-8000-0000000000e1"}`
		got := decode[struct {
			Record json.RawMessage
			Task   struct{ CycleSeq int }
		}](t, rec)
		if string(got.Record) != wantRecord || got.Task.CycleSeq != 4 {
			t.Errorf("record =\n%s\nwant\n%s\ntask = %+v", got.Record, wantRecord, got.Task)
		}
		in := records.inputs[0]
		if in.CycleSeq == nil || *in.CycleSeq != 3 || in.PerformedOn == nil || *in.PerformedOn != "2026-11-04" || !in.AllowExtra {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("実施日と追加の記録の指定を省くと、nil と false を渡す", func(t *testing.T) {
		t.Parallel()
		records := &fakeCareRecordService{}
		rec := serve(t, newRecordRouter(t, records), http.MethodPost, path, "Bearer "+validToken, `{"cycleSeq": 3}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if in := records.inputs[0]; in.PerformedOn != nil || in.AllowExtra {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("完了済みの回は 409 で、誰がいつ完了したかと最新のタスクを返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newRecordRouter(t, &fakeCareRecordService{}), http.MethodPost, path, "Bearer "+validToken, `{"cycleSeq": 2}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Code     string
			Existing json.RawMessage
			Task     struct{ CycleSeq int }
		}](t, rec)
		const wantExisting = `{"actorDisplayName":"さくら","performedAt":"2026-11-04T00:30:00Z","performedOn":"2026-11-04","recordId":"01920000-0000-7000-8000-0000000000f1"}`
		if got.Code != "already_completed" || string(got.Existing) != wantExisting || got.Task.CycleSeq != 4 {
			t.Errorf("body = %s", rec.Body.String())
		}
	})

	t.Run("予定の回が合わなければ 409 stale_cycle", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newRecordRouter(t, &fakeCareRecordService{}), http.MethodPost, path, "Bearer "+validToken, `{"cycleSeq": 9}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := decode[errorBody](t, rec); got.Code != "stale_cycle" {
			t.Errorf("code = %q", got.Code)
		}
	})

	t.Run("権限がなければ 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newRecordRouter(t, &fakeCareRecordService{}), http.MethodPost, "/v1/tasks/"+uuid.Must(uuid.NewV7()).String()+"/completions", "Bearer "+validToken, `{"cycleSeq": 3}`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}

func TestSkipCareTask(t *testing.T) {
	t.Parallel()
	path := "/v1/tasks/" + testTask.ID.String() + "/skips"

	t.Run("作った記録（理由・確認日つき）と更新後のタスクを 201 で返す", func(t *testing.T) {
		t.Parallel()
		records := &fakeCareRecordService{}
		rec := serve(t, newRecordRouter(t, records), http.MethodPost, path, "Bearer "+validToken, `{"cycleSeq": 3, "reason": "other", "reasonText": "旅行中", "recheckOn": "2026-11-05"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Record struct {
				Outcome, SkipReason, SkipReasonText, RecheckOn string
			}
			Task struct{ NextDueOn string }
		}](t, rec)
		if got.Record.Outcome != "skipped" || got.Record.SkipReason != "other" || got.Record.SkipReasonText != "旅行中" ||
			got.Record.RecheckOn != "2026-11-05" || got.Task.NextDueOn != "2026-11-05" {
			t.Errorf("body = %s", rec.Body.String())
		}
		in := records.skips[0]
		if in.CycleSeq == nil || *in.CycleSeq != 3 || in.Reason == nil || *in.Reason != "other" ||
			in.ReasonText == nil || *in.ReasonText != "旅行中" || in.RecheckOn == nil || *in.RecheckOn != "2026-11-05" {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("完了の記録には、スキップの項目がない", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newRecordRouter(t, &fakeCareRecordService{}), http.MethodPost, "/v1/tasks/"+testTask.ID.String()+"/completions", "Bearer "+validToken, `{"cycleSeq": 3}`)
		if body := rec.Body.String(); strings.Contains(body, "skipReason") || strings.Contains(body, "recheckOn") {
			t.Errorf("body = %s", body)
		}
	})

	t.Run("予定の回が合わなければ 409 stale_cycle", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newRecordRouter(t, &fakeCareRecordService{}), http.MethodPost, path, "Bearer "+validToken, `{"cycleSeq": 2}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := decode[errorBody](t, rec); got.Code != "stale_cycle" {
			t.Errorf("code = %q", got.Code)
		}
	})

	t.Run("権限がなければ 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newRecordRouter(t, &fakeCareRecordService{}), http.MethodPost, "/v1/tasks/"+uuid.Must(uuid.NewV7()).String()+"/skips", "Bearer "+validToken, `{"cycleSeq": 3}`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}
