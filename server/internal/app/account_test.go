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
	// アカウントの削除
	deleting    domain.User // 削除するユーザー（コミットの直前の処理に渡す）
	deleteErr   error
	deleteCalls int
	deleted     bool
}

func (f *fakeUsers) GetOrCreate(_ context.Context, id domain.Identity) (domain.User, error) {
	f.created = id
	return domain.User{ID: uuid.Must(uuid.NewV7()), DisplayName: id.Name}, f.err
}

func (f *fakeUsers) Delete(ctx context.Context, userID uuid.UUID, beforeCommit func(context.Context, domain.User) error) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	// 本物と同じく、コミットの直前の処理が失敗したら、消さない（ロールバック）
	if err := beforeCommit(ctx, f.deleting); err != nil {
		return err
	}
	f.deleted = true
	return nil
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
			svc := app.NewAccountService(users, &fakeDevices{}, nil, nil)

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
	svc := app.NewAccountService(&fakeUsers{err: domain.ErrNotFound}, &fakeDevices{}, nil, nil)

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
		u, err := app.NewAccountService(users, &fakeDevices{}, nil, nil).UpdateDisplayName(t.Context(), uuid.Must(uuid.NewV7()), " はなこ ")
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
		_, err := app.NewAccountService(users, &fakeDevices{}, nil, nil).UpdateDisplayName(t.Context(), uuid.Must(uuid.NewV7()), "  ")
		var verr *domain.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("err = %v, want *ValidationError", err)
		}
		if users.updatedName != "" {
			t.Errorf("repository was called with %q", users.updatedName)
		}
	})
}

// fakeDevices は、app.DeviceRepository の偽物である。受け取った値を記録する。
type fakeDevices struct {
	upserted []domain.DeviceRegistration
	deleted  []uuid.UUID
}

func (f *fakeDevices) Upsert(_ context.Context, userID uuid.UUID, reg domain.DeviceRegistration) (domain.Device, error) {
	f.upserted = append(f.upserted, reg)
	return domain.Device{ID: uuid.Must(uuid.NewV7()), UserID: userID, FCMToken: reg.FCMToken, Platform: reg.Platform, TimeZone: reg.TimeZone}, nil
}

func (f *fakeDevices) Delete(_ context.Context, _, deviceID uuid.UUID) error {
	f.deleted = append(f.deleted, deviceID)
	return nil
}

func TestAccountService_RegisterDevice(t *testing.T) {
	t.Parallel()

	t.Run("検証した内容を保存する", func(t *testing.T) {
		t.Parallel()
		devices := &fakeDevices{}
		userID := uuid.Must(uuid.NewV7())
		d, err := app.NewAccountService(&fakeUsers{}, devices, nil, nil).RegisterDevice(t.Context(), userID, "token-1", "ios", "Asia/Tokyo")
		if err != nil {
			t.Fatal(err)
		}
		want := domain.DeviceRegistration{FCMToken: "token-1", Platform: domain.PlatformIOS, TimeZone: "Asia/Tokyo"}
		if len(devices.upserted) != 1 || devices.upserted[0] != want {
			t.Errorf("upserted = %+v, want [%+v]", devices.upserted, want)
		}
		if d.UserID != userID {
			t.Errorf("device.UserID = %s, want %s", d.UserID, userID)
		}
	})

	t.Run("不正な内容は保存しない", func(t *testing.T) {
		t.Parallel()
		devices := &fakeDevices{}
		_, err := app.NewAccountService(&fakeUsers{}, devices, nil, nil).RegisterDevice(t.Context(), uuid.Must(uuid.NewV7()), "token-1", "windows", "Asia/Tokyo")
		var verr *domain.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("err = %v, want *ValidationError", err)
		}
		if len(devices.upserted) != 0 {
			t.Errorf("repository was called with %+v", devices.upserted)
		}
	})
}

func TestAccountService_UnregisterDevice(t *testing.T) {
	t.Parallel()
	devices := &fakeDevices{}
	deviceID := uuid.Must(uuid.NewV7())

	if err := app.NewAccountService(&fakeUsers{}, devices, nil, nil).UnregisterDevice(t.Context(), uuid.Must(uuid.NewV7()), deviceID); err != nil {
		t.Fatal(err)
	}
	if len(devices.deleted) != 1 || devices.deleted[0] != deviceID {
		t.Errorf("deleted = %v, want [%s]", devices.deleted, deviceID)
	}
}

// fakeIdentities は、Firebase のユーザーの削除を記録する。
type fakeIdentities struct {
	deleted []string
	err     error
}

func (f *fakeIdentities) DeleteUser(_ context.Context, uid string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, uid)
	return nil
}

// fakeApple は、Apple のトークンの失効を記録する。
type fakeApple struct {
	codes []string
	err   error
}

func (f *fakeApple) Revoke(_ context.Context, code string) error {
	f.codes = append(f.codes, code)
	return f.err
}

func TestAccountService_DeleteAccount(t *testing.T) {
	t.Parallel()
	google := domain.User{ID: uuid.Must(uuid.NewV7()), FirebaseUID: "uid-google", AuthProvider: "google.com"}
	apple := domain.User{ID: uuid.Must(uuid.NewV7()), FirebaseUID: "uid-apple", AuthProvider: "apple.com"}

	t.Run("Firebase のユーザーを消してから、コミットする", func(t *testing.T) {
		t.Parallel()
		users, ids, ap := &fakeUsers{deleting: google}, &fakeIdentities{}, &fakeApple{}
		if err := app.NewAccountService(users, &fakeDevices{}, ids, ap).DeleteAccount(t.Context(), google.ID, ""); err != nil {
			t.Fatal(err)
		}
		if !users.deleted || len(ids.deleted) != 1 || ids.deleted[0] != "uid-google" {
			t.Errorf("deleted = %v, firebase = %v", users.deleted, ids.deleted)
		}
		// Apple でサインインしていなければ、Apple には何もしない
		if len(ap.codes) != 0 {
			t.Errorf("apple = %v", ap.codes)
		}
	})

	t.Run("Apple でサインインしたユーザーは、認可コードで Apple のトークンを失効させる", func(t *testing.T) {
		t.Parallel()
		users, ids, ap := &fakeUsers{deleting: apple}, &fakeIdentities{}, &fakeApple{}
		if err := app.NewAccountService(users, &fakeDevices{}, ids, ap).DeleteAccount(t.Context(), apple.ID, " code-1 "); err != nil {
			t.Fatal(err)
		}
		if len(ap.codes) != 1 || ap.codes[0] != "code-1" || !users.deleted || len(ids.deleted) != 1 {
			t.Errorf("apple = %v, deleted = %v, firebase = %v", ap.codes, users.deleted, ids.deleted)
		}
	})

	t.Run("Apple のユーザーで認可コードがなければ、入力の誤りにして消さない", func(t *testing.T) {
		t.Parallel()
		users, ids := &fakeUsers{deleting: apple}, &fakeIdentities{}
		err := app.NewAccountService(users, &fakeDevices{}, ids, &fakeApple{}).DeleteAccount(t.Context(), apple.ID, "  ")
		var v *domain.ValidationError
		if !errors.As(err, &v) || v.Fields["appleAuthorizationCode"] != "required" {
			t.Errorf("err = %v", err)
		}
		if users.deleted || len(ids.deleted) != 0 {
			t.Errorf("deleted = %v, firebase = %v", users.deleted, ids.deleted)
		}
	})

	t.Run("Apple の失効が設定されていなければ、Apple のユーザーは消さない", func(t *testing.T) {
		t.Parallel()
		users := &fakeUsers{deleting: apple}
		if err := app.NewAccountService(users, &fakeDevices{}, &fakeIdentities{}, nil).DeleteAccount(t.Context(), apple.ID, "code-1"); err == nil || users.deleted {
			t.Errorf("err = %v, deleted = %v", err, users.deleted)
		}
	})

	t.Run("Apple の失効か Firebase の削除が失敗したら、消さない", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		for _, tt := range []struct {
			ids *fakeIdentities
			ap  *fakeApple
		}{
			{&fakeIdentities{}, &fakeApple{err: boom}},
			{&fakeIdentities{err: boom}, &fakeApple{}},
		} {
			users := &fakeUsers{deleting: apple}
			if err := app.NewAccountService(users, &fakeDevices{}, tt.ids, tt.ap).DeleteAccount(t.Context(), apple.ID, "code-1"); !errors.Is(err, boom) || users.deleted {
				t.Errorf("err = %v, deleted = %v", err, users.deleted)
			}
		}
		// Apple の失効が失敗したら、Firebase のユーザーも消さない
		ids := &fakeIdentities{}
		_ = app.NewAccountService(&fakeUsers{deleting: apple}, &fakeDevices{}, ids, &fakeApple{err: boom}).DeleteAccount(t.Context(), apple.ID, "code-1")
		if len(ids.deleted) != 0 {
			t.Errorf("firebase = %v", ids.deleted)
		}
	})

	t.Run("オーナー権限を移す必要があれば、そのまま返す", func(t *testing.T) {
		t.Parallel()
		transfer := &domain.OwnerTransferRequiredError{Households: []domain.Household{{Name: "自宅"}}}
		err := app.NewAccountService(&fakeUsers{deleteErr: transfer}, &fakeDevices{}, &fakeIdentities{}, &fakeApple{}).DeleteAccount(t.Context(), google.ID, "")
		if !errors.Is(err, transfer) {
			t.Errorf("err = %v", err)
		}
	})
}
