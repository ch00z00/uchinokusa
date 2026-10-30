// Package app は、ユースケース（各 Service）、権限の判定、トランザクションの境界を提供する。
package app

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// UserRepository は、ユーザーの保存先である（実装は adapter/postgres）。
type UserRepository interface {
	// GetOrCreate は、Firebase の uid が一致するユーザーを返し、いなければ作る。削除済みなら domain.ErrNotFound を返す。
	GetOrCreate(ctx context.Context, id domain.Identity) (domain.User, error)
	// UpdateDisplayName は、表示名を変更する。ユーザーがいなければ domain.ErrNotFound を返す。
	UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (domain.User, error)
}

// DeviceRepository は、プッシュ通知の送信先となる端末の保存先である（実装は adapter/postgres）。
type DeviceRepository interface {
	// Upsert は、FCM トークンが同じ端末があれば上書きし、なければ作る。別のユーザーの端末なら、userID のユーザーに付け替える。
	Upsert(ctx context.Context, userID uuid.UUID, reg domain.DeviceRegistration) (domain.Device, error)
	// Delete は、userID のユーザーの端末を消す。該当する端末がなくてもエラーにしない。
	Delete(ctx context.Context, userID, deviceID uuid.UUID) error
}

// AccountService は、サインインしたユーザーの特定と、プロフィール・端末の変更を扱う（要件1）。
type AccountService struct {
	users   UserRepository
	devices DeviceRepository
}

func NewAccountService(users UserRepository, devices DeviceRepository) *AccountService {
	return &AccountService{users: users, devices: devices}
}

// SignIn は、認証済みの Identity に対応するユーザーを返す。初めてのユーザーなら作る（要件1-2）。
// 削除済みのユーザーは domain.ErrUnauthenticated にする。
func (s *AccountService) SignIn(ctx context.Context, id domain.Identity) (domain.User, error) {
	id.Name = initialDisplayName(id.Name)
	u, err := s.users.GetOrCreate(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, domain.ErrUnauthenticated
	}
	return u, err
}

// UpdateDisplayName は、表示名を検証して変更する。
func (s *AccountService) UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (domain.User, error) {
	name, err := domain.NormalizeDisplayName(name)
	if err != nil {
		return domain.User{}, err
	}
	return s.users.UpdateDisplayName(ctx, userID, name)
}

// RegisterDevice は、端末を検証して、プッシュ通知の送信先に登録・更新する。
// アプリの起動のたびに呼ばれ、端末のタイムゾーンを最新に保つ（要件15-7）。
func (s *AccountService) RegisterDevice(ctx context.Context, userID uuid.UUID, fcmToken, platform, timeZone string) (domain.Device, error) {
	reg, err := domain.NewDeviceRegistration(fcmToken, platform, timeZone)
	if err != nil {
		return domain.Device{}, err
	}
	return s.devices.Upsert(ctx, userID, reg)
}

// UnregisterDevice は、端末をプッシュ通知の送信先から外す（要件1-5）。
func (s *AccountService) UnregisterDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	return s.devices.Delete(ctx, userID, deviceID)
}

// initialDisplayName は、プロバイダから受け取った名前を表示名の初期値にする。
// 初期値はユーザーが確認・変更する（要件1-2）ため、エラーにはせず、長すぎる名前は切り詰める。
func initialDisplayName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) > domain.MaxDisplayNameLength {
		runes = runes[:domain.MaxDisplayNameLength]
	}
	return strings.TrimSpace(string(runes))
}
