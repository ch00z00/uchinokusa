package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

const schedulerToken = "scheduler-token"

// fakeJobVerifier は、schedulerToken を Cloud Scheduler のトークン、"other-account" をほかのアカウントのトークンとして扱う。
type fakeJobVerifier struct{}

func (fakeJobVerifier) VerifyJob(_ context.Context, token string) error {
	switch token {
	case schedulerToken:
		return nil
	case "other-account":
		return domain.ErrForbidden
	}
	return domain.ErrUnauthenticated
}

type fakeJobRunner struct {
	ran    []string
	report job.Report
	err    error
}

func (f *fakeJobRunner) Run(_ context.Context, name string) (job.Report, error) {
	f.ran = append(f.ran, name)
	if name != "reminders" {
		return job.Report{}, domain.ErrNotFound
	}
	return f.report, f.err
}

func TestJobs(t *testing.T) {
	t.Parallel()
	newRouter := func(runner *fakeJobRunner) http.Handler {
		return httpapi.NewRouter(httpapi.Deps{
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			Verifier: &fakeVerifier{},
			Accounts: &fakeAccounts{},
			Jobs:     runner,
			JobAuth:  fakeJobVerifier{},
		})
	}
	call := func(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("Cloud Scheduler のトークンなら実行し、処理件数と所要時間を返す", func(t *testing.T) {
		t.Parallel()
		runner := &fakeJobRunner{report: job.Report{Job: "reminders", Result: job.Result{Processed: 3, Failed: 1}, Duration: 1500 * time.Millisecond}}
		rec := call(newRouter(runner), http.MethodPost, "/internal/jobs/reminders", schedulerToken)
		if rec.Code != http.StatusOK || len(runner.ran) != 1 || runner.ran[0] != "reminders" {
			t.Fatalf("status = %d, ran = %v, body = %s", rec.Code, runner.ran, rec.Body)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"job": "reminders", "processed": float64(3), "failed": float64(1), "skipped": false, "durationMs": float64(1500)}
		if len(body) != len(want) {
			t.Errorf("body = %v", body)
		}
		for k, v := range want {
			if body[k] != v {
				t.Errorf("%s = %v, want %v", k, body[k], v)
			}
		}
	})

	for _, tt := range []struct {
		name   string
		token  string
		status int
	}{
		{"トークンがなければ 401", "", http.StatusUnauthorized},
		{"正しくないトークンは 401", "forged", http.StatusUnauthorized},
		// アプリのユーザーの ID トークンでも呼べない
		{"アプリのユーザーの ID トークンは 401", validToken, http.StatusUnauthorized},
		{"ほかのアカウントのトークンは 403", "other-account", http.StatusForbidden},
	} {
		t.Run(tt.name+"にし、実行しない", func(t *testing.T) {
			t.Parallel()
			runner := &fakeJobRunner{}
			rec := call(newRouter(runner), http.MethodPost, "/internal/jobs/reminders", tt.token)
			if rec.Code != tt.status || len(runner.ran) != 0 {
				t.Errorf("status = %d, ran = %v", rec.Code, runner.ran)
			}
		})
	}

	t.Run("知らないジョブは 404", func(t *testing.T) {
		t.Parallel()
		rec := call(newRouter(&fakeJobRunner{}), http.MethodPost, "/internal/jobs/unknown", schedulerToken)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d", rec.Code)
		}
	})

	t.Run("ジョブが失敗したら 500（Cloud Scheduler がやり直す）", func(t *testing.T) {
		t.Parallel()
		rec := call(newRouter(&fakeJobRunner{err: errors.New("boom")}), http.MethodPost, "/internal/jobs/reminders", schedulerToken)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d", rec.Code)
		}
	})

	t.Run("POST 以外は受け付けない", func(t *testing.T) {
		t.Parallel()
		runner := &fakeJobRunner{}
		rec := call(newRouter(runner), http.MethodGet, "/internal/jobs/reminders", schedulerToken)
		if rec.Code != http.StatusMethodNotAllowed || len(runner.ran) != 0 {
			t.Errorf("status = %d, ran = %v", rec.Code, runner.ran)
		}
	})

	t.Run("ジョブの設定がなければ、エンドポイントを登録しない", func(t *testing.T) {
		t.Parallel()
		router := httpapi.NewRouter(httpapi.Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Verifier: &fakeVerifier{}, Accounts: &fakeAccounts{}})
		if rec := call(router, http.MethodPost, "/internal/jobs/reminders", schedulerToken); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d", rec.Code)
		}
	})

	t.Run("呼び出し元を確かめられなければ、ジョブがあっても登録しない", func(t *testing.T) {
		t.Parallel()
		runner := &fakeJobRunner{}
		router := httpapi.NewRouter(httpapi.Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Verifier: &fakeVerifier{}, Accounts: &fakeAccounts{}, Jobs: runner})
		if rec := call(router, http.MethodPost, "/internal/jobs/reminders", ""); rec.Code != http.StatusNotFound || len(runner.ran) != 0 {
			t.Errorf("status = %d, ran = %v", rec.Code, runner.ran)
		}
	})
}
