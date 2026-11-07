package firebase_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"google.golang.org/api/option"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/firebase"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeFCM は、FCM の送信 API（POST /projects/{id}/messages:send）の代わりである。
// トークンが "gone" で始まれば UNREGISTERED、"bad" で始まれば INVALID_ARGUMENT を返す。
type fakeFCM struct {
	mu       sync.Mutex
	messages []map[string]any
}

func (f *fakeFCM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/projects/demo-uchinokusa/messages:send" {
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
		return
	}
	var body struct {
		Message map[string]any `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.messages = append(f.messages, body.Message)
	f.mu.Unlock()

	token, _ := body.Message["token"].(string)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasPrefix(token, "gone"):
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":404,"message":"Requested entity was not found.","status":"NOT_FOUND",
			"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`)
	case strings.HasPrefix(token, "bad"):
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":400,"message":"Invalid argument","status":"INVALID_ARGUMENT",
			"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"}]}}`)
	default:
		fmt.Fprint(w, `{"name":"projects/demo-uchinokusa/messages/1"}`)
	}
}

func newMessenger(t *testing.T) (*firebase.Messenger, *fakeFCM) {
	t.Helper()
	fcm := &fakeFCM{}
	srv := httptest.NewServer(fcm)
	t.Cleanup(srv.Close)
	m, err := firebase.NewMessenger(t.Context(), "demo-uchinokusa", "", slog.New(slog.NewTextHandler(io.Discard, nil)),
		option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	return m, fcm
}

func TestMessenger_Send(t *testing.T) {
	t.Parallel()

	t.Run("トークンごとに送り、結果をトークンの順に返す", func(t *testing.T) {
		t.Parallel()
		m, fcm := newMessenger(t)
		msg := domain.PushMessage{Title: "自宅", Body: "さくらさんがモンステラに水やりしました", Data: map[string]string{"householdId": "h1", "plantId": "p1"}}
		got, err := m.Send(t.Context(), []string{"t1", "gone-1", "bad-1", "t2"}, msg)
		if err != nil {
			t.Fatal(err)
		}
		want := []domain.PushOutcome{domain.PushDelivered, domain.PushUnregistered, domain.PushFailed, domain.PushDelivered}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("outcomes = %v, want %v", got, want)
		}
		if len(fcm.messages) != 4 {
			t.Fatalf("messages = %d", len(fcm.messages))
		}
		for _, sent := range fcm.messages {
			if !reflect.DeepEqual(sent["notification"], map[string]any{"title": "自宅", "body": "さくらさんがモンステラに水やりしました"}) ||
				!reflect.DeepEqual(sent["data"], map[string]any{"householdId": "h1", "plantId": "p1"}) {
				t.Errorf("message = %v", sent)
			}
		}
	})

	t.Run("FCM の上限（500件）を超えるトークンは、分けて送る", func(t *testing.T) {
		t.Parallel()
		m, fcm := newMessenger(t)
		tokens := make([]string, 501)
		for i := range tokens {
			tokens[i] = fmt.Sprintf("t%d", i)
		}
		tokens[500] = "gone-last"
		got, err := m.Send(t.Context(), tokens, domain.PushMessage{Title: "t", Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 501 || len(fcm.messages) != 501 {
			t.Fatalf("outcomes = %d, messages = %d", len(got), len(fcm.messages))
		}
		if got[0] != domain.PushDelivered || got[500] != domain.PushUnregistered {
			t.Errorf("first = %v, last = %v", got[0], got[500])
		}
	})

	t.Run("トークンがなければ送らない", func(t *testing.T) {
		t.Parallel()
		m, fcm := newMessenger(t)
		got, err := m.Send(t.Context(), nil, domain.PushMessage{Title: "t", Body: "b"})
		if err != nil || len(got) != 0 || len(fcm.messages) != 0 {
			t.Errorf("outcomes = %v, err = %v, messages = %d", got, err, len(fcm.messages))
		}
	})
}
