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
			svc := app.NewAccountService(users, &fakeDevices{})

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
	svc := app.NewAccountService(&fakeUsers{err: domain.ErrNotFound}, &fakeDevices{})

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
		u, err := app.NewAccountService(users, &fakeDevices{}).UpdateDisplayName(t.Context(), uuid.Must(uuid.NewV7()), " はなこ ")
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
		_, err := app.NewAccountService(users, &fakeDevices{}).UpdateDisplayName(t.Context(), uuid.Must(uuid.NewV7()), "  ")
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
		d, err := app.NewAccountService(&fakeUsers{}, devices).RegisterDevice(t.Context(), userID, "token-1", "ios", "Asia/Tokyo")
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
		_, err := app.NewAccountService(&fakeUsers{}, devices).RegisterDevice(t.Context(), uuid.Must(uuid.NewV7()), "token-1", "windows", "Asia/Tokyo")
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

	if err := app.NewAccountService(&fakeUsers{}, devices).UnregisterDevice(t.Context(), uuid.Must(uuid.NewV7()), deviceID); err != nil {
		t.Fatal(err)
	}
	if len(devices.deleted) != 1 || devices.deleted[0] != deviceID {
		t.Errorf("deleted = %v, want [%s]", devices.deleted, deviceID)
	}
}
