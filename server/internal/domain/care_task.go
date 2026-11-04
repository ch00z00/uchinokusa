package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
)

// CareKind は、お手入れの作業の種類である（要件8-1）。
type CareKind string

const (
	CareWatering    CareKind = "watering"    // 水やり
	CareFertilizing CareKind = "fertilizing" // 肥料
	CareRepotting   CareKind = "repotting"   // 植え替え
	CarePruning     CareKind = "pruning"     // 剪定
	CareOther       CareKind = "other"       // その他（作業名を自由入力する。要件8-3）
)

// TaskStatus は、お手入れタスクの状態である。
type TaskStatus string

const (
	TaskActive TaskStatus = "active"
	TaskPaused TaskStatus = "paused" // 一時停止中。予定・リマインダーの対象から外す（要件8-9）
)

// CareTask は、植物ごとのお手入れタスクである（要件8）。
type CareTask struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	PlantID     uuid.UUID
	CareTaskProfile
	NextDueOn civil.Date // 次回予定日。一時停止中はゼロ値のことがある
	// DueOverridden は、予定日を手で変えたことを表す。季節の切り替えで計算し直す対象から外す（設計の「次回予定の計算ルール」）。
	DueOverridden bool
	// CycleSeq は、現在の予定の回の番号である。完了するたびに1つ進め、同じ回の重複した完了を防ぐ（設計の判断2）。
	CycleSeq  int
	Status    TaskStatus
	CreatedAt time.Time
}

// CareTaskProfile は、利用者が設定するタスクの内容である。空文字は未設定を表す。
type CareTaskProfile struct {
	Kind          CareKind
	CustomName    string // 種類が「その他」のときの作業名。ほかの種類では空
	Schedule      Schedule
	ConditionText string // 実行条件（例：土の表面が乾いていたら。要件8-5）
	Note          string // メモ（例：底から流れるまでたっぷり。要件8-4）
}

const (
	// MaxCustomNameLength は、作業名の最大の文字数である。通知（「〇〇の葉水」）に収まる長さにする。
	MaxCustomNameLength = 20
	// MaxConditionTextLength は、実行条件の最大の文字数である。今日やること一覧の1行に収まる長さにする。
	MaxConditionTextLength = 100
	// MaxTaskNoteLength は、タスクのメモの最大の文字数である。スキップの理由（care_records）と同じにする。
	MaxTaskNoteLength = 200
)

// maxIntervalValue は、周期の単位ごとの最大の値である。植え替え（1〜2年ごと）まで表せるようにする。
var maxIntervalValue = map[IntervalUnit]int{IntervalDay: 365, IntervalWeek: 52, IntervalMonth: 24}

var (
	careKinds = []CareKind{CareWatering, CareFertilizing, CareRepotting, CarePruning, CareOther}
	seasons   = []Season{SeasonSpring, SeasonSummer, SeasonAutumn, SeasonWinter}
)

// IntervalInput は、検証前の周期である。
type IntervalInput struct {
	Value int
	Unit  string
}

// CareTaskInput は、タスクの作成・変更で受け取った、検証前の内容である。nil は送られなかった項目を表す。
type CareTaskInput struct {
	Kind       *string // 作成のときだけ使う（種類はあとから変えない。記録が種類を持つため）
	CustomName *string
	Interval   *IntervalInput
	// SeasonalIntervals は、季節（spring・summer・autumn・winter）ごとの周期である。
	// nil は送られなかったことを、空の map は季節ごとの周期をなくすことを表す。
	SeasonalIntervals map[string]IntervalInput
	ConditionText     *string
	Note              *string
	NextDueOn         *string // YYYY-MM-DD。作成では初回の予定日、変更では手で変えた予定日
}

// CareTaskDraft は、作成の内容を検証済みの、まだ保存していないタスクである。
type CareTaskDraft struct {
	CareTaskProfile
	NextDueOn civil.Date
}

// NewCareTask は、タスクの作成の内容を検証する。初回の予定日を省くと、今日にする。
// 登録したその日に植物の状態を見て、完了かスキップを選べるようにするためである。
func NewCareTask(in CareTaskInput, today civil.Date) (CareTaskDraft, error) {
	v := taskValidator{fields: map[string]string{}}
	t := CareTaskDraft{NextDueOn: today}
	t.Kind = v.kind(in.Kind)
	if t.Kind == CareOther {
		t.CustomName = v.customName(deref(in.CustomName))
	}
	if in.Interval == nil {
		v.fields["interval"] = "required"
	} else {
		t.Schedule.Base = v.interval("interval", *in.Interval)
	}
	t.Schedule.Seasonal = v.seasonal(in.SeasonalIntervals)
	t.ConditionText = v.text("conditionText", deref(in.ConditionText), MaxConditionTextLength)
	t.Note = v.text("note", deref(in.Note), MaxTaskNoteLength)
	if s := strings.TrimSpace(deref(in.NextDueOn)); s != "" {
		t.NextDueOn = v.date(s)
	}
	if err := v.err(); err != nil {
		return CareTaskDraft{}, err
	}
	return t, nil
}

// CareTaskPatch は、検証済みの、タスクの変更の内容である。nil の項目は変えない。
// 周期を変えても次回予定日は変えず、新しい周期は次に完了したときから使う。予定日も変えたい場合は NextDueOn を送る。
type CareTaskPatch struct {
	CustomName    *string
	Interval      *Interval
	Seasonal      *SeasonalIntervals // 空の map は、季節ごとの周期をなくすことを表す
	ConditionText *string            // 空文字は、実行条件をなくすことを表す
	Note          *string
	NextDueOn     *civil.Date // 手で変えた予定日。季節の切り替えで計算し直す対象から外す
}

// NewCareTaskPatch は、タスクの変更の内容を検証する。kind は、変更するタスクの種類である。
// 作業名は、種類が「その他」のタスクだけで変えられる（ほかの種類では使わないので無視する）。
func NewCareTaskPatch(in CareTaskInput, kind CareKind) (CareTaskPatch, error) {
	v := taskValidator{fields: map[string]string{}}
	var p CareTaskPatch
	if in.CustomName != nil && kind == CareOther {
		p.CustomName = ptrTo(v.customName(*in.CustomName))
	}
	if in.Interval != nil {
		p.Interval = ptrTo(v.interval("interval", *in.Interval))
	}
	if in.SeasonalIntervals != nil {
		p.Seasonal = ptrTo(v.seasonal(in.SeasonalIntervals))
	}
	if in.ConditionText != nil {
		p.ConditionText = ptrTo(v.text("conditionText", *in.ConditionText, MaxConditionTextLength))
	}
	if in.Note != nil {
		p.Note = ptrTo(v.text("note", *in.Note, MaxTaskNoteLength))
	}
	if in.NextDueOn != nil {
		if s := strings.TrimSpace(*in.NextDueOn); s == "" {
			v.fields["nextDueOn"] = "required" // 有効なタスクは、必ず次回予定日を持つ
		} else {
			p.NextDueOn = ptrTo(v.date(s))
		}
	}
	if err := v.err(); err != nil {
		return CareTaskPatch{}, err
	}
	return p, nil
}

// TaskDefault は、タスクの作成画面に示す初期値である（要件6-4）。
type TaskDefault struct {
	Kind          CareKind
	ConditionText string
}

// TaskDefaultsOf は、植物マスタのお手入れの目安から、タスクの初期値を作る。care が nil（マスタと紐づかない）なら空。
// マスタにあるのは、出典で確かめた水やりの実行条件だけで、推奨周期はまだない（docs/decisions/0009）。
func TaskDefaultsOf(care *SpeciesCare) []TaskDefault {
	out := []TaskDefault{}
	if care != nil && care.Watering != nil && care.Watering.Condition != "" {
		out = append(out, TaskDefault{Kind: CareWatering, ConditionText: care.Watering.Condition})
	}
	return out
}

// TodayIn は、タイムゾーン（IANA の名前）での今日の日付を返す。読めない名前なら UTC の今日にする。
func TodayIn(now time.Time, timeZone string) civil.Date {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		loc = time.UTC
	}
	return civil.DateOf(now.In(loc))
}

// taskValidator は、タスクの項目を検証し、誤りを項目ごとに集める。
type taskValidator struct {
	fields map[string]string
}

func (v taskValidator) kind(s *string) CareKind {
	if s == nil || *s == "" {
		v.fields["kind"] = "required"
		return ""
	}
	for _, k := range careKinds {
		if CareKind(*s) == k {
			return k
		}
	}
	v.fields["kind"] = "invalid"
	return ""
}

func (v taskValidator) customName(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		v.fields["customName"] = "required"
	case utf8.RuneCountInString(s) > MaxCustomNameLength:
		v.fields["customName"] = "too_long"
	}
	return s
}

func (v taskValidator) interval(field string, in IntervalInput) Interval {
	unit := IntervalUnit(in.Unit)
	maxValue, ok := maxIntervalValue[unit]
	if !ok || in.Value < 1 || in.Value > maxValue {
		v.fields[field] = "invalid"
		return Interval{}
	}
	return Interval{Value: in.Value, Unit: unit}
}

func (v taskValidator) seasonal(in map[string]IntervalInput) SeasonalIntervals {
	out := SeasonalIntervals{}
	for name, i := range in {
		field := "seasonalIntervals." + name
		season, ok := seasonOf(name)
		if !ok {
			v.fields[field] = "invalid"
			continue
		}
		out[season] = v.interval(field, i)
	}
	return out
}

func seasonOf(name string) (Season, bool) {
	for _, s := range seasons {
		if Season(name) == s {
			return s, true
		}
	}
	return "", false
}

func (v taskValidator) text(field, s string, maxLength int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxLength {
		v.fields[field] = "too_long"
	}
	return s
}

func (v taskValidator) date(s string) civil.Date {
	d, err := civil.ParseDate(s)
	if err != nil || !d.IsValid() {
		v.fields["nextDueOn"] = "invalid"
		return civil.Date{}
	}
	return d
}

func (v taskValidator) err() error {
	if len(v.fields) > 0 {
		return &ValidationError{Fields: v.fields}
	}
	return nil
}
