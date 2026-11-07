// Package job は、Cloud Scheduler から呼ばれる定期ジョブ（リマインダー・完了通知など）と、その実行の仕組みを提供する。
package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// Result は、1回の実行で処理した件数である。
type Result struct {
	Processed int // 処理した件数（送った通知の数など）
	Failed    int // 失敗した件数（次回の実行でやり直すものを含む）
}

// Job は、定期ジョブである。二重に実行されても結果が変わらないように作る（ユニーク制約・状態の更新・行ロック）。
type Job interface {
	Name() string
	Run(ctx context.Context) (Result, error)
}

// Locker は、ジョブの名前ごとのロックである（実装は postgres.JobLocker）。どの API インスタンスでも、同じジョブは同時に1つだけ動かす。
type Locker interface {
	// TryLock は、ロックを取れたら unlock と true を返す。ほかで実行中なら、待たずに false を返す。
	TryLock(ctx context.Context, name string) (unlock func(), ok bool, err error)
}

// Report は、ジョブの実行の結果である。
type Report struct {
	Job string
	Result
	Skipped  bool // ほかで実行中だったので、実行しなかった
	Duration time.Duration
}

// Runner は、名前でジョブを探して実行し、処理件数と所要時間をログに出す。
type Runner struct {
	lock   Locker
	logger *slog.Logger
	now    func() time.Time
	jobs   map[string]Job
}

// NewRunner は、jobs を名前で引けるようにする。名前が空・重複していればエラーにする（エンドポイントの名前になるため）。
func NewRunner(lock Locker, logger *slog.Logger, now func() time.Time, jobs ...Job) (*Runner, error) {
	r := &Runner{lock: lock, logger: logger, now: now, jobs: map[string]Job{}}
	for _, j := range jobs {
		name := j.Name()
		if name == "" {
			return nil, errors.New("job: empty name")
		}
		if _, ok := r.jobs[name]; ok {
			return nil, fmt.Errorf("job: duplicate name %q", name)
		}
		r.jobs[name] = j
	}
	return r, nil
}

// Run は、name のジョブを実行する。知らない名前なら domain.ErrNotFound を返す。
// ほかのインスタンスで同じジョブが実行中なら、実行せずに Skipped を返す（Cloud Scheduler の再送や、前の実行が長引いた場合）。
func (r *Runner) Run(ctx context.Context, name string) (Report, error) {
	j, ok := r.jobs[name]
	if !ok {
		return Report{}, domain.ErrNotFound
	}
	start := r.now()
	rep := Report{Job: name}

	unlock, ok, err := r.lock.TryLock(ctx, name)
	if err != nil {
		return rep, fmt.Errorf("lock job %s: %w", name, err)
	}
	if ok {
		defer unlock()
		rep.Result, err = j.Run(ctx)
	} else {
		rep.Skipped = true
	}
	rep.Duration = r.now().Sub(start)

	attrs := []any{
		slog.String("job", name), slog.Int("processed", rep.Processed), slog.Int("failed", rep.Failed),
		slog.Bool("skipped", rep.Skipped), slog.Int64("duration_ms", rep.Duration.Milliseconds()),
	}
	if err != nil {
		r.logger.ErrorContext(ctx, "job failed", append(attrs, slog.String("error", err.Error()))...)
		return rep, fmt.Errorf("job %s: %w", name, err)
	}
	r.logger.InfoContext(ctx, "job finished", attrs...)
	return rep, nil
}
