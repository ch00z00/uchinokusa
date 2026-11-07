package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestDeleteMe(t *testing.T) {
	t.Parallel()
	call := func(accounts *fakeAccounts, body string) *httptest.ResponseRecorder {
		router := newMeRouter(t, accounts)
		req := httptest.NewRequest(http.MethodDelete, "/v1/me", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+validToken)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("本文がなくても削除する（Google でサインインしたユーザー）", func(t *testing.T) {
		t.Parallel()
		accounts := &fakeAccounts{}
		rec := call(accounts, "")
		if rec.Code != http.StatusNoContent || len(accounts.deleted) != 1 || accounts.deleted[0] != "" {
			t.Errorf("status = %d, deleted = %v, body = %s", rec.Code, accounts.deleted, rec.Body)
		}
	})

	t.Run("Apple の認可コードを渡す", func(t *testing.T) {
		t.Parallel()
		accounts := &fakeAccounts{}
		rec := call(accounts, `{"appleAuthorizationCode":"code-1"}`)
		if rec.Code != http.StatusNoContent || len(accounts.deleted) != 1 || accounts.deleted[0] != "code-1" {
			t.Errorf("status = %d, deleted = %v", rec.Code, accounts.deleted)
		}
	})

	t.Run("オーナー権限を移す必要があれば、409 と世帯の一覧を返す", func(t *testing.T) {
		t.Parallel()
		home := uuid.MustParse("01920000-0000-7000-8000-0000000000d1")
		rec := call(&fakeAccounts{deleteErr: &domain.OwnerTransferRequiredError{Households: []domain.Household{{ID: home, Name: "自宅"}}}}, "")
		want := `{"code":"owner_transfer_required","message":"conflict: owner_transfer_required (1 households)","households":[{"id":"` + home.String() + `","name":"自宅"}]}`
		if rec.Code != http.StatusConflict || canonicalJSON(t, rec.Body.Bytes()) != canonicalJSON(t, []byte(want)) {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
		}
	})

	t.Run("認可コードの誤りは 422", func(t *testing.T) {
		t.Parallel()
		rec := call(&fakeAccounts{deleteErr: &domain.ValidationError{Fields: map[string]string{"appleAuthorizationCode": "invalid"}}}, `{"appleAuthorizationCode":"old"}`)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"appleAuthorizationCode":"invalid"`) {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
		}
	})
}
