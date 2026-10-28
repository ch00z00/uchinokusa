package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeUsers は、app.UserRepository の偽物である。受け取った値を記録する。
type fakeUsers struct {
	created     domain.Identity
	updatedName string
	err         error
}

func (f *fakeUsers) GetOrCreate(_ context.Context, id domain.Identity) (domain.User, error) {
	f.created = id
	return domain.User{ID: uuid.Must(uuid.NewV7()), DisplayName: id.Name}, f.err
}

func (f *fakeUsers) UpdateDisplayName(_ context.Context, userID uuid.UUID, name string) (domain.User, error) {
	f.updatedName = name
	return domain.User{ID: userID, DisplayName: name}, f.err
}

func TestAccountService_SignIn_InitialDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{name: "プロバイダの名前をそのまま使う", providerName: "Yusuke", want: "Yusuke"},
		{name: "前後の空白を除く", providerName: "  Yusuke ", want: "Yusuke"},
		{name: "名前がなければ空（アプリで設定してもらう）", providerName: "", want: ""},
		{name: "長すぎる名前は、30文字に切り詰める", providerName: strings.Repeat("あ", 40), want: strings.Repeat("あ", 30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users := &fakeUsers{}
			svc := app.NewAccountService(users)

			if _, err := svc.SignIn(t.Context(), domain.Identity{UID: "u", Name: tt.providerName, Provider: "google.com"}); err != nil {
				t.Fatal(err)
			}
			if users.created.Name != tt.want {
				t.Errorf("initial name = %q, want %q", users.created.Name, tt.want)
			}
		})
	}
}

// 削除済みのユーザーは、認証エラーにする。
func TestAccountService_SignIn_DeletedUser(t *testing.T) {
	t.Parallel()
	svc := app.NewAccountService(&fakeUsers{err: domain.ErrNotFound})

	_, err := svc.SignIn(t.Context(), domain.Identity{UID: "u"})
	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestAccountService_UpdateDisplayName(t *testing.T) {
	t.Parallel()

	t.Run("前後の空白を除いて保存する", func(t *testing.T) {
		t.Parallel()
		users := &fakeUsers{}
		u, err := app.NewAccountService(users).UpdateDisplayName(t.Context(), uuid.Must(uuid.NewV7()), " はなこ ")
		if err != nil {
			t.Fatal(err)
		}
		if users.updatedName != "はなこ" || u.DisplayName != "はなこ" {
			t.Errorf("saved %q, returned %q, want はなこ", users.updatedName, u.DisplayName)
		}
	})

	t.Run("不正な名前は保存しない", func(t *testing.T) {
		t.Parallel()
		users := &fakeUsers{}
		_, err := app.NewAccountService(users).UpdateDisplayName(t.Context(), uuid.Must(uuid.NewV7()), "  ")
		var verr *domain.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("err = %v, want *ValidationError", err)
		}
		if users.updatedName != "" {
			t.Errorf("repository was called with %q", users.updatedName)
		}
	})
}
