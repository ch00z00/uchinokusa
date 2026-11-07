package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeDeviceTokens は、ユーザーごとのトークンを持つ、app.DeviceTokenRepository の偽物である。
type fakeDeviceTokens struct {
	tokens    map[uuid.UUID][]string
	listErr   error
	deleteErr error
	calls     int
	deleted   []string
}

func (f *fakeDeviceTokens) TokensOf(_ context.Context, userIDs []uuid.UUID) ([]string, error) {
	f.calls++
	var out []string
	for _, id := range userIDs {
		out = append(out, f.tokens[id]...)
	}
	return out, f.listErr
}

func (f *fakeDeviceTokens) DeleteTokens(_ context.Context, tokens []string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, tokens...)
	return nil
}

// fakeSender は、トークンごとに決めた結果を返す。
type fakeSender struct {
	outcomes map[string]domain.PushOutcome
	err      error
	calls    int
	sent     []string
	msg      domain.PushMessage
}

func (f *fakeSender) Send(_ context.Context, tokens []string, msg domain.PushMessage) ([]domain.PushOutcome, error) {
	f.calls++
	f.sent, f.msg = tokens, msg
	if f.err != nil {
		return nil, f.err
	}
	out := make([]domain.PushOutcome, len(tokens))
	for i, t := range tokens {
		out[i] = f.outcomes[t]
		if out[i] == "" {
			out[i] = domain.PushDelivered
		}
	}
	return out, nil
}

var (
	pushUserA = uuid.MustParse("01920000-0000-7000-8000-0000000000aa")
	pushUserB = uuid.MustParse("01920000-0000-7000-8000-0000000000bb")
	pushMsg   = domain.PushMessage{Title: "自宅", Body: "さくらさんがモンステラに水やりしました", Data: map[string]string{"householdId": "h1"}}
)

func TestPushService_Notify(t *testing.T) {
	t.Parallel()

	t.Run("ユーザーたちのすべての端末に送り、無効になったトークンを消す", func(t *testing.T) {
		t.Parallel()
		devices := &fakeDeviceTokens{tokens: map[uuid.UUID][]string{pushUserA: {"a-phone", "a-tablet"}, pushUserB: {"b-phone"}}}
		sender := &fakeSender{outcomes: map[string]domain.PushOutcome{"a-tablet": domain.PushUnregistered, "b-phone": domain.PushFailed}}
		s := app.NewPushService(devices, sender, slog.Default())

		rep, err := s.Notify(t.Context(), []uuid.UUID{pushUserA, pushUserB}, pushMsg)
		if err != nil {
			t.Fatal(err)
		}
		if rep != (domain.PushReport{Delivered: 1, Failed: 1, Removed: 1}) {
			t.Errorf("report = %+v", rep)
		}
		if !reflect.DeepEqual(sender.sent, []string{"a-phone", "a-tablet", "b-phone"}) || !reflect.DeepEqual(sender.msg, pushMsg) {
			t.Errorf("sent = %v, msg = %+v", sender.sent, sender.msg)
		}
		// 一時的な失敗のトークンは消さない
		if !reflect.DeepEqual(devices.deleted, []string{"a-tablet"}) {
			t.Errorf("deleted = %v", devices.deleted)
		}
	})

	t.Run("端末がなければ送らない", func(t *testing.T) {
		t.Parallel()
		sender := &fakeSender{}
		s := app.NewPushService(&fakeDeviceTokens{}, sender, slog.Default())
		rep, err := s.Notify(t.Context(), []uuid.UUID{pushUserA}, pushMsg)
		if err != nil || rep != (domain.PushReport{}) || sender.calls != 0 {
			t.Errorf("report = %+v, err = %v, calls = %d", rep, err, sender.calls)
		}
	})

	t.Run("宛先のユーザーがいなければ、端末も探さない", func(t *testing.T) {
		t.Parallel()
		devices := &fakeDeviceTokens{}
		s := app.NewPushService(devices, &fakeSender{}, slog.Default())
		if _, err := s.Notify(t.Context(), nil, pushMsg); err != nil || devices.calls != 0 {
			t.Errorf("err = %v, calls = %d", err, devices.calls)
		}
	})

	t.Run("送れなければエラーを返す", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("fcm is down")
		devices := &fakeDeviceTokens{tokens: map[uuid.UUID][]string{pushUserA: {"a-phone"}}}
		s := app.NewPushService(devices, &fakeSender{err: boom}, slog.Default())
		if _, err := s.Notify(t.Context(), []uuid.UUID{pushUserA}, pushMsg); !errors.Is(err, boom) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("トークンを探せなければエラーを返す", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("db down")
		s := app.NewPushService(&fakeDeviceTokens{listErr: boom}, &fakeSender{}, slog.Default())
		if _, err := s.Notify(t.Context(), []uuid.UUID{pushUserA}, pushMsg); !errors.Is(err, boom) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("トークンを消せなくても、送った結果は返す（次に送るときに消し直す）", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		devices := &fakeDeviceTokens{tokens: map[uuid.UUID][]string{pushUserA: {"a-phone", "a-old"}}, deleteErr: errors.New("db down")}
		sender := &fakeSender{outcomes: map[string]domain.PushOutcome{"a-old": domain.PushUnregistered}}
		s := app.NewPushService(devices, sender, slog.New(slog.NewTextHandler(&buf, nil)))
		rep, err := s.Notify(t.Context(), []uuid.UUID{pushUserA}, pushMsg)
		if err != nil || rep != (domain.PushReport{Delivered: 1}) {
			t.Errorf("report = %+v, err = %v", rep, err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("delete unregistered tokens")) {
			t.Errorf("log = %s", buf.String())
		}
	})
}
