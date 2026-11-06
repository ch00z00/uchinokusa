package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SuggestionRepository は、提案の保存先と、プロバイダに渡す情報の読み出し先である（実装は adapter/postgres）。
type SuggestionRepository interface {
	// Subject は、植物と、その世帯・植物マスタ・置き場所・タスク・最近の記録を返す。植物がなければ domain.ErrNotFound を返す。
	Subject(ctx context.Context, plantID uuid.UUID) (advisor.Subject, error)
	// Save は、提案を保存する。世帯に同じ重複防止のキーの提案があれば（見送ったものも含む）、保存せず created を false にする。
	// ただし、撤回した（expired の）提案なら、未回答に戻して created を true にする。
	Save(ctx context.Context, s domain.Suggestion) (saved domain.Suggestion, created bool, err error)
	// Retract は、プロバイダの、タスクへの未回答の提案のうち、キーが keep にないものを撤回する（状態を expired にする）。
	Retract(ctx context.Context, householdID uuid.UUID, providerID string, taskID uuid.UUID, keep []string) error
	// HouseholdOf は、提案が属する世帯の ID を返す。提案がなければ domain.ErrNotFound を返す。
	HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error)
	// ListActive は、on の日に有効な（未回答で、有効期間の中の）提案を、作った順に返す。
	ListActive(ctx context.Context, householdID uuid.UUID, on civil.Date) ([]domain.Suggestion, error)
	// Accept は、提案の行をロックして採用する（domain.PlanAcceptance）。タスクを作る提案なら、同じトランザクションでタスクを作る。
	// タスクを作らない提案では、ゼロ値のタスクを返す。
	Accept(ctx context.Context, householdID, id, actorID uuid.UUID, in domain.AcceptInput, today civil.Date) (domain.Suggestion, domain.CareTask, error)
	// Dismiss は、提案の行をロックして見送る（domain.CheckDismiss）。
	Dismiss(ctx context.Context, householdID, id, actorID uuid.UUID) (domain.Suggestion, error)
}

// AdviceEvaluator は、出来事をきっかけに提案を評価する（実装は SuggestionService）。
// 植物・タスクのサービスが、変更のあとに呼ぶ。エラーを返さないので、元の操作の結果は変わらない。
type AdviceEvaluator interface {
	Evaluate(ctx context.Context, ev AdviceEvent) []domain.Suggestion
}

// AdviceEvent は、提案を作るきっかけの出来事である。
type AdviceEvent struct {
	Trigger advisor.Trigger
	PlantID uuid.UUID
	TaskID  uuid.UUID // task_due_changed の対象のタスク
}

// SuggestionService は、助言プロバイダを呼んで提案を保存し、提案の一覧・採用・見送りを扱う（設計の判断3、要件19）。
// 表示・採用・見送りの流れはプロバイダの種類によらないので、新しいプロバイダは Registry に登録するだけで加えられる（要件19-5）。
type SuggestionService struct {
	repo     SuggestionRepository
	registry *advisor.Registry
	authz    *Authorizer
	now      func() time.Time
	logger   *slog.Logger
}

func NewSuggestionService(repo SuggestionRepository, registry *advisor.Registry, authz *Authorizer, now func() time.Time, logger *slog.Logger) *SuggestionService {
	return &SuggestionService{repo: repo, registry: registry, authz: authz, now: now, logger: logger}
}

// Evaluate は、きっかけのプロバイダを呼び、新しく保存した提案を返す。
// 植物の登録などの元の操作を失敗させないため、エラーは返さず記録だけする（設計の「プロバイダの障害」）。
// プロバイダのエラー・panic・形の誤った提案は、そのプロバイダ（その提案）だけを飛ばし、ほかのプロバイダは続ける。
// アーカイブした植物には、プロバイダを呼ばない。
//
// タスクへの評価（task_due_changed）では、プロバイダが今回返さなかった、そのタスクへの前の未回答の提案を撤回する。
// 予定日を変えたあとに、前の予定日についての助言が残らないようにするため。失敗したプロバイダの提案は撤回しない。
func (s *SuggestionService) Evaluate(ctx context.Context, ev AdviceEvent) []domain.Suggestion {
	subject, err := s.repo.Subject(ctx, ev.PlantID)
	if err != nil {
		s.logger.ErrorContext(ctx, "load advice subject", "trigger", ev.Trigger, "plant_id", ev.PlantID, "error", err)
		return nil
	}
	if subject.Plant.ArchivedAt != nil {
		return nil
	}
	in := advisor.Context{Trigger: ev.Trigger, Today: domain.TodayIn(s.now(), subject.Household.TimeZone), TaskID: ev.TaskID, Subject: subject}

	var created []domain.Suggestion
	for _, a := range s.registry.For(ev.Trigger) {
		proposals, err := advise(ctx, a, in)
		if err != nil {
			s.logger.ErrorContext(ctx, "advisor failed", "advisor", a.ID(), "trigger", ev.Trigger, "plant_id", ev.PlantID, "error", err)
			continue
		}
		keep := make([]string, 0, len(proposals))
		for _, p := range proposals {
			// 保存に失敗した提案も、プロバイダの今の考えには入っているので、撤回しない
			keep = append(keep, p.DedupeKey)
			saved, ok, err := s.save(ctx, a.ID(), subject, p)
			if err != nil {
				s.logger.ErrorContext(ctx, "save suggestion", "advisor", a.ID(), "dedupe_key", p.DedupeKey, "plant_id", ev.PlantID, "error", err)
				continue
			}
			if ok {
				created = append(created, saved)
			}
		}
		if ev.TaskID != uuid.Nil {
			if err := s.repo.Retract(ctx, subject.Household.ID, a.ID(), ev.TaskID, keep); err != nil {
				s.logger.ErrorContext(ctx, "retract suggestions", "advisor", a.ID(), "task_id", ev.TaskID, "error", err)
			}
		}
	}
	return created
}

// advise は、プロバイダを呼ぶ。panic もエラーにして、元の操作を止めない。
func advise(ctx context.Context, a advisor.Advisor, in advisor.Context) (proposals []advisor.Proposal, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return a.Advise(ctx, in)
}

func (s *SuggestionService) save(ctx context.Context, providerID string, subject advisor.Subject, p advisor.Proposal) (domain.Suggestion, bool, error) {
	if err := p.Validate(); err != nil {
		return domain.Suggestion{}, false, fmt.Errorf("invalid proposal: %w", err)
	}
	sg, err := p.Suggestion(providerID, subject.Household.ID, subject.Plant.ID)
	if err != nil {
		return domain.Suggestion{}, false, err
	}
	return s.repo.Save(ctx, sg)
}

// List は、世帯の有効な提案を返す。閲覧者も見られる。日付を省くと、世帯のタイムゾーンでの今日にする（今日やること一覧と同じ）。
func (s *SuggestionService) List(ctx context.Context, userID, householdID uuid.UUID, date *string) ([]domain.Suggestion, error) {
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionRead); err != nil {
		return nil, err
	}
	today, err := s.today(ctx, householdID)
	if err != nil {
		return nil, err
	}
	on, err := domain.TodayDate(date, today)
	if err != nil {
		return nil, err
	}
	return s.repo.ListActive(ctx, householdID, on)
}

// Accept は、提案を採用する。タスクを作る提案なら、提案日（日付を送れば、その日）を予定日にしたタスクを作る（要件17-4・17-5）。
// 編集者以上ができる（タスクを作るため）。対応した人と日時を記録する（要件19-6）。
func (s *SuggestionService) Accept(ctx context.Context, userID, suggestionID uuid.UUID, in domain.AcceptInput) (domain.Suggestion, domain.CareTask, error) {
	householdID, err := s.authorize(ctx, userID, suggestionID)
	if err != nil {
		return domain.Suggestion{}, domain.CareTask{}, err
	}
	today, err := s.today(ctx, householdID)
	if err != nil {
		return domain.Suggestion{}, domain.CareTask{}, err
	}
	sg, task, err := s.repo.Accept(ctx, householdID, suggestionID, userID, in, today)
	if err != nil {
		return domain.Suggestion{}, domain.CareTask{}, err
	}
	if task.ID != uuid.Nil {
		// 日付を変えて採用した日が、避ける時期のこともあるため（要件17-8）
		s.Evaluate(ctx, AdviceEvent{Trigger: advisor.TriggerTaskDueChanged, PlantID: sg.PlantID, TaskID: task.ID})
	}
	return sg, task, nil
}

// Dismiss は、提案を見送る。同じ重複防止のキーの提案は、このあと作らない（要件17-6）。編集者以上ができる。
func (s *SuggestionService) Dismiss(ctx context.Context, userID, suggestionID uuid.UUID) (domain.Suggestion, error) {
	householdID, err := s.authorize(ctx, userID, suggestionID)
	if err != nil {
		return domain.Suggestion{}, err
	}
	return s.repo.Dismiss(ctx, householdID, suggestionID, userID)
}

// authorize は、提案の世帯で、タスクの権限を確かめる。入力の検証より先に行う。
func (s *SuggestionService) authorize(ctx context.Context, userID, suggestionID uuid.UUID) (uuid.UUID, error) {
	householdID, err := s.repo.HouseholdOf(ctx, suggestionID)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.authz.Require(ctx, userID, householdID, domain.ActionTaskWrite); err != nil {
		return uuid.Nil, err
	}
	return householdID, nil
}

func (s *SuggestionService) today(ctx context.Context, householdID uuid.UUID) (civil.Date, error) {
	timeZone, err := s.repo.TimeZoneOf(ctx, householdID)
	if err != nil {
		return civil.Date{}, err
	}
	return domain.TodayIn(s.now(), timeZone), nil
}
