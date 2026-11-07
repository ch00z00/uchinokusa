package job_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

type fakeJob struct {
	name   string
	result job.Result
	err    error
	runs   int
}

func (j *fakeJob) Name() string { return j.name }

func (j *fakeJob) Run(context.Context) (job.Result, error) {
	j.runs++
	return j.result, j.err
}

// fakeLocker は、held にある名前のロックを、ほかで実行中として扱う。
type fakeLocker struct {
	held     map[string]bool
	err      error
	unlocked []string
}

func (l *fakeLocker) TryLock(_ context.Context, name string) (func(), bool, error) {
	if l.err != nil {
		return nil, false, l.err
	}
	if l.held[name] {
		return nil, false, nil
	}
	return func() { l.unlocked = append(l.unlocked, name) }, true, nil
}

// clock は、呼ばれるたびに 1.5 秒進む時計である（所要時間を確かめるため）。
func clock() func() time.Time {
	t := time.Date(2026, 11, 7, 8, 0, 0, 0, time.UTC)
	return func() time.Time {
		now := t
		t = t.Add(1500 * time.Millisecond)
		return now
	}
}

func newRunner(t *testing.T, lock *fakeLocker, jobs ...job.Job) (*job.Runner, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	r, err := job.NewRunner(lock, slog.New(slog.NewJSONHandler(&buf, nil)), clock(), jobs...)
	if err != nil {
		t.Fatal(err)
	}
	return r, &buf
}

// lastLog は、最後に出たログの1行を返す。
func lastLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var m map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &m); err != nil {
		t.Fatalf("log %q: %v", buf.String(), err)
	}
	return m
}

func TestNewRunner_RejectsBadNames(t *testing.T) {
	t.Parallel()
	for name, jobs := range map[string][]job.Job{
		"空の名前":  {&fakeJob{name: ""}},
		"名前の重複": {&fakeJob{name: "reminders"}, &fakeJob{name: "reminders"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := job.NewRunner(&fakeLocker{}, slog.Default(), time.Now, jobs...); err == nil {
				t.Error("err = nil")
			}
		})
	}
}

func TestRunner_Run(t *testing.T) {
	t.Parallel()

	t.Run("ジョブを実行し、処理件数と所要時間をログに出す", func(t *testing.T) {
		t.Parallel()
		j := &fakeJob{name: "reminders", result: job.Result{Processed: 3, Failed: 1}}
		lock := &fakeLocker{}
		r, buf := newRunner(t, lock, j, &fakeJob{name: "other"})
		rep, err := r.Run(t.Context(), "reminders")
		if err != nil {
			t.Fatal(err)
		}
		want := job.Report{Job: "reminders", Result: job.Result{Processed: 3, Failed: 1}, Duration: 1500 * time.Millisecond}
		if rep != want || j.runs != 1 {
			t.Errorf("report = %+v, runs = %d", rep, j.runs)
		}
		// 終わったらロックを外す
		if len(lock.unlocked) != 1 || lock.unlocked[0] != "reminders" {
			t.Errorf("unlocked = %v", lock.unlocked)
		}
		log := lastLog(t, buf)
		if log["msg"] != "job finished" || log["job"] != "reminders" || log["processed"] != float64(3) || log["failed"] != float64(1) ||
			log["skipped"] != false || log["duration_ms"] != float64(1500) || log["level"] != "INFO" {
			t.Errorf("log = %v", log)
		}
	})

	t.Run("ほかで実行中なら、実行せずに skipped にする", func(t *testing.T) {
		t.Parallel()
		j := &fakeJob{name: "reminders"}
		r, buf := newRunner(t, &fakeLocker{held: map[string]bool{"reminders": true}}, j)
		rep, err := r.Run(t.Context(), "reminders")
		if err != nil {
			t.Fatal(err)
		}
		if !rep.Skipped || j.runs != 0 {
			t.Errorf("report = %+v, runs = %d", rep, j.runs)
		}
		if log := lastLog(t, buf); log["skipped"] != true {
			t.Errorf("log = %v", log)
		}
	})

	t.Run("知らないジョブは domain.ErrNotFound", func(t *testing.T) {
		t.Parallel()
		r, _ := newRunner(t, &fakeLocker{}, &fakeJob{name: "reminders"})
		if _, err := r.Run(t.Context(), "unknown"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("ジョブが失敗したら、途中までの件数とあわせてエラーのログを出し、エラーを返す", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		j := &fakeJob{name: "reminders", result: job.Result{Processed: 2}, err: boom}
		lock := &fakeLocker{}
		r, buf := newRunner(t, lock, j)
		rep, err := r.Run(t.Context(), "reminders")
		if !errors.Is(err, boom) || rep.Processed != 2 {
			t.Errorf("report = %+v, err = %v", rep, err)
		}
		if len(lock.unlocked) != 1 {
			t.Errorf("unlocked = %v", lock.unlocked)
		}
		log := lastLog(t, buf)
		if log["msg"] != "job failed" || log["level"] != "ERROR" || log["processed"] != float64(2) || log["error"] != "boom" {
			t.Errorf("log = %v", log)
		}
	})

	t.Run("ロックを取れなければ、実行せずにエラーを返す", func(t *testing.T) {
		t.Parallel()
		j := &fakeJob{name: "reminders"}
		r, _ := newRunner(t, &fakeLocker{err: errors.New("db down")}, j)
		// 実行中だった（skipped）のではなく、確かめられなかったので、失敗として Cloud Scheduler にやり直させる
		rep, err := r.Run(t.Context(), "reminders")
		if err == nil || rep.Skipped || j.runs != 0 {
			t.Errorf("report = %+v, err = %v, runs = %d", rep, err, j.runs)
		}
	})
}
