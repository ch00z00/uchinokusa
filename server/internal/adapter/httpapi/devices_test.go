package httpapi_test

import (
	"maps"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

var testDeviceID = uuid.MustParse("01920000-0000-7000-8000-0000000000d1")

type deviceBody struct {
	ID         string `json:"id"`
	Platform   string `json:"platform"`
	TimeZone   string `json:"timeZone"`
	LastSeenAt string `json:"lastSeenAt"`
}

func TestRegisterDevice(t *testing.T) {
	t.Parallel()

	t.Run("登録した端末を返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMeRouter(t, &fakeAccounts{}), http.MethodPost, "/v1/me/devices", "Bearer "+validToken,
			`{"fcmToken": "token-1", "platform": "ios", "timeZone": "Asia/Tokyo"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		got := decode[deviceBody](t, rec)
		want := deviceBody{
			ID:         testDeviceID.String(),
			Platform:   "ios",
			TimeZone:   "Asia/Tokyo",
			LastSeenAt: "2026-10-29T23:00:00Z", // 日時は UTC で返す
		}
		if got != want {
			t.Errorf("body = %+v, want %+v", got, want)
		}
	})

	t.Run("不正な内容は 422 で、項目ごとの理由を返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMeRouter(t, &fakeAccounts{}), http.MethodPost, "/v1/me/devices", "Bearer "+validToken,
			`{"fcmToken": "", "platform": "windows", "timeZone": "Asia/Tokyo"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422 (%s)", rec.Code, rec.Body.String())
		}
		got := decode[errorBody](t, rec)
		wantFields := map[string]string{"fcmToken": "required", "platform": "invalid"}
		if got.Code != "validation_failed" || !maps.Equal(got.Fields, wantFields) {
			t.Errorf("error = %+v, want validation_failed %v", got, wantFields)
		}
	})

	t.Run("認証が必要", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMeRouter(t, &fakeAccounts{}), http.MethodPost, "/v1/me/devices", "",
			`{"fcmToken": "token-1", "platform": "ios", "timeZone": "Asia/Tokyo"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestUnregisterDevice(t *testing.T) {
	t.Parallel()

	t.Run("リクエストしたユーザーの端末として外す", func(t *testing.T) {
		t.Parallel()
		accounts := &fakeAccounts{}
		rec := serve(t, newMeRouter(t, accounts), http.MethodDelete, "/v1/me/devices/"+testDeviceID.String(), "Bearer "+validToken, "")

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204 (%s)", rec.Code, rec.Body.String())
		}
		if len(accounts.unregistered) != 1 || accounts.unregistered[0] != testDeviceID {
			t.Errorf("unregistered = %v, want [%s]", accounts.unregistered, testDeviceID)
		}
	})

	t.Run("ID が UUID でなければ 400", func(t *testing.T) {
		t.Parallel()
		accounts := &fakeAccounts{}
		rec := serve(t, newMeRouter(t, accounts), http.MethodDelete, "/v1/me/devices/not-a-uuid", "Bearer "+validToken, "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
		}
		if len(accounts.unregistered) != 0 {
			t.Error("UnregisterDevice was called for an invalid ID")
		}
	})
}
