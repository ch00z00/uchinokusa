package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// 完了の衝突のコード（設計の判断2）。
const (
	// ConflictAlreadyCompleted は、送られた予定の回が、すでに完了していることを表す（要件11-2・11-4）。
	ConflictAlreadyCompleted = "already_completed"
	// ConflictStaleCycle は、送られた予定の回が、タスクの今の回と合わないことを表す（アプリの表示が古い）。
	ConflictStaleCycle = "stale_cycle"
)

// CareOutcome は、お手入れの記録の結果である。
type CareOutcome string

const (
	OutcomeCompleted CareOutcome = "completed"
	OutcomeSkipped   CareOutcome = "skipped"
)

// CareRecord は、お手入れの記録（完了・スキップ）である。
// 作業の種類と作業した人の名前は、記録の時点の値を持つ。タスクの削除やメンバーの退出のあとも表示できるようにするため。
type CareRecord struct {
	ID               uuid.UUID
	HouseholdID      uuid.UUID
	PlantID          uuid.UUID
	TaskID           uuid.UUID // タスクがなくなっていれば uuid.Nil
	TaskKind         CareKind
	TaskCustomName   string
	Outcome          CareOutcome
	PerformedOn      civil.Date // 実施日
	PerformedAt      time.Time  // 記録した日時
	ActorUserID      uuid.UUID  // アカウントを削除していれば uuid.Nil
	ActorDisplayName string
	CycleSeq         int        // 記録した予定の回の番号
	IsExtra          bool       // 完了済みの回への、追加の記録
	SkipReason       SkipReason // スキップの理由。理由なし・完了なら ""
	SkipReasonText   string     // スキップの理由が other のときの自由入力
	RecheckOn        civil.Date // スキップの確認日。完了ならゼロ値
	CreatedAt        time.Time
}

// SkipReason は、スキップの理由である（要件10-7）。助言の改善に使えるよう、自由入力と分けて決まった値で持つ（要件19-7）。
type SkipReason string

const (
	SkipSoilWet    SkipReason = "soil_wet"    // 土がまだ湿っている
	SkipBadWeather SkipReason = "bad_weather" // 天気が悪い
	SkipPlantWeak  SkipReason = "plant_weak"  // 株が弱っている
	SkipOther      SkipReason = "other"       // その他（自由入力）
)

// MaxSkipReasonTextLength は、スキップの理由の自由入力の上限（文字数）である。
const MaxSkipReasonTextLength = 200

// CompletionInput は、完了の記録の入力（未検証）である。
type CompletionInput struct {
	CycleSeq    *int    // アプリが見ていた予定の回の番号
	PerformedOn *string // 省くと今日
	AllowExtra  bool    // 完了済みの回でも、追加の記録として保存する（要件11-3）
}

// Completion は、検証済みの完了の記録の入力である。
type Completion struct {
	CycleSeq    int
	PerformedOn civil.Date
	AllowExtra  bool
}

// NewCompletion は、完了の記録の入力を検証する。実施日を省くと today にする。
// 過去の実施日は受け付ける（要件10-3）が、未来の実施日は受け付けない。
func NewCompletion(in CompletionInput, today civil.Date) (Completion, error) {
	fields := map[string]string{}
	c := Completion{PerformedOn: today, AllowExtra: in.AllowExtra}
	switch {
	case in.CycleSeq == nil:
		fields["cycleSeq"] = "required"
	case *in.CycleSeq < 1:
		fields["cycleSeq"] = "invalid"
	default:
		c.CycleSeq = *in.CycleSeq
	}
	if s := strings.TrimSpace(deref(in.PerformedOn)); s != "" {
		d, err := civil.ParseDate(s)
		if err != nil || !d.IsValid() || d.After(today) {
			fields["performedOn"] = "invalid"
		}
		c.PerformedOn = d
	}
	if len(fields) > 0 {
		return Completion{}, &ValidationError{Fields: fields}
	}
	return c, nil
}

// CompletionPlan は、完了を記録したときの、記録とタスクの変更の内容である。
type CompletionPlan struct {
	Extra          bool       // 追加の記録。タスクの予定は変えない
	RecordCycleSeq int        // 記録に残す予定の回の番号
	NextDueOn      civil.Date // タスクの新しい次回予定
	NextCycleSeq   int        // タスクの新しい予定の回の番号
}

// PlanCompletion は、ロックしたタスクに完了を記録したときの変更を決める（設計の判断2、次回予定の計算ルール）。
//   - 今の回の完了：次回予定を「実施日 + 実施日の季節の周期」にし、回の番号を1つ進める
//   - 完了済みの回（今の回より前）：AllowExtra なら追加の記録にし、そうでなければ already_completed
//   - まだ来ていない回：stale_cycle
func PlanCompletion(t CareTask, c Completion, h Hemisphere) (CompletionPlan, error) {
	switch {
	case c.CycleSeq == t.CycleSeq:
		return CompletionPlan{
			RecordCycleSeq: c.CycleSeq,
			NextDueOn:      t.Schedule.NextDueAfterCompletion(c.PerformedOn, h),
			NextCycleSeq:   t.CycleSeq + 1,
		}, nil
	case c.CycleSeq < t.CycleSeq && c.AllowExtra:
		return CompletionPlan{Extra: true, RecordCycleSeq: c.CycleSeq, NextDueOn: t.NextDueOn, NextCycleSeq: t.CycleSeq}, nil
	case c.CycleSeq < t.CycleSeq:
		return CompletionPlan{}, &ConflictError{Code: ConflictAlreadyCompleted}
	default:
		return CompletionPlan{}, &ConflictError{Code: ConflictStaleCycle}
	}
}

// AlreadyCompletedError は、送られた予定の回がすでに完了していたことを、先に完了した記録と最新のタスクとともに表す（要件11-2）。
type AlreadyCompletedError struct {
	Existing CareRecord
	Task     CareTask
}

func (e *AlreadyCompletedError) Error() string {
	return "conflict: " + ConflictAlreadyCompleted
}

// Unwrap は、詳細を扱わない層でも、衝突のコード（ConflictError）として扱えるようにする。
func (e *AlreadyCompletedError) Unwrap() error {
	return &ConflictError{Code: ConflictAlreadyCompleted}
}

// SkipInput は、スキップの記録の入力（未検証）である。
type SkipInput struct {
	CycleSeq   *int    // アプリが見ていた予定の回の番号
	Reason     *string // 任意
	ReasonText *string // 任意。Reason が other のときだけ
	RecheckOn  *string // 省くと翌日
}

// Skip は、検証済みのスキップの記録の入力である。
type Skip struct {
	CycleSeq    int
	Reason      SkipReason
	ReasonText  string
	PerformedOn civil.Date // スキップを判断した日（今日）
	RecheckOn   civil.Date // 次回の確認日。タスクの次回予定になる
}

// NewSkip は、スキップの記録の入力を検証する。確認日を省くと、today の翌日にする。
// 確認日は翌日以降に限る（要件10-8）。今日以前にすると、スキップしたタスクが今日やることに残ってしまうため。
func NewSkip(in SkipInput, today civil.Date) (Skip, error) {
	fields := map[string]string{}
	s := Skip{PerformedOn: today, RecheckOn: today.AddDays(1)}
	switch {
	case in.CycleSeq == nil:
		fields["cycleSeq"] = "required"
	case *in.CycleSeq < 1:
		fields["cycleSeq"] = "invalid"
	default:
		s.CycleSeq = *in.CycleSeq
	}
	if r := SkipReason(strings.TrimSpace(deref(in.Reason))); r != "" {
		switch r {
		case SkipSoilWet, SkipBadWeather, SkipPlantWeak, SkipOther:
			s.Reason = r
		default:
			fields["reason"] = "invalid"
		}
	}
	if text := strings.TrimSpace(deref(in.ReasonText)); text != "" {
		switch {
		case SkipReason(strings.TrimSpace(deref(in.Reason))) != SkipOther:
			fields["reasonText"] = "invalid"
		case utf8.RuneCountInString(text) > MaxSkipReasonTextLength:
			fields["reasonText"] = "too_long"
		}
		s.ReasonText = text
	}
	if r := strings.TrimSpace(deref(in.RecheckOn)); r != "" {
		d, err := civil.ParseDate(r)
		if err != nil || !d.IsValid() || !d.After(today) {
			fields["recheckOn"] = "invalid"
		}
		s.RecheckOn = d
	}
	if len(fields) > 0 {
		return Skip{}, &ValidationError{Fields: fields}
	}
	return s, nil
}

// CheckSkip は、ロックしたタスクの今の回へのスキップかを確かめる（設計の「次回予定の計算ルール」）。
// スキップは回を進めない（スキップした回に、あとから完了できる）ので、今の回以外へのスキップは stale_cycle にする。
func CheckSkip(t CareTask, s Skip) error {
	if s.CycleSeq != t.CycleSeq {
		return &ConflictError{Code: ConflictStaleCycle}
	}
	return nil
}
