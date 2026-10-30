package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// DeviceRepository は、user_devices テーブルへのアクセスを提供する。
type DeviceRepository struct {
	q *sqlcgen.Queries
}

func NewDeviceRepository(pool *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{q: sqlcgen.New(pool)}
}

// Upsert は、FCM トークンが同じ端末があれば上書きし、なければ作る。別のユーザーの端末なら、userID のユーザーに付け替える。
// 1つの SQL（ON CONFLICT）で行うため、同じトークンの登録が同時に届いても、端末は1つにまとまる。
func (r *DeviceRepository) Upsert(ctx context.Context, userID uuid.UUID, reg domain.DeviceRegistration) (domain.Device, error) {
	row, err := r.q.UpsertUserDevice(ctx, sqlcgen.UpsertUserDeviceParams{
		UserID:   userID,
		FCMToken: reg.FCMToken,
		Platform: string(reg.Platform),
		TimeZone: reg.TimeZone,
	})
	if err != nil {
		return domain.Device{}, fmt.Errorf("upsert device: %w", err)
	}
	return domain.Device{
		ID:         row.ID,
		UserID:     row.UserID,
		FCMToken:   row.FCMToken,
		Platform:   domain.Platform(row.Platform),
		TimeZone:   row.TimeZone,
		LastSeenAt: row.LastSeenAt,
	}, nil
}

// Delete は、userID のユーザーの端末を消す。該当する端末がなくてもエラーにしない。
func (r *DeviceRepository) Delete(ctx context.Context, userID, deviceID uuid.UUID) error {
	if err := r.q.DeleteUserDevice(ctx, sqlcgen.DeleteUserDeviceParams{ID: deviceID, UserID: userID}); err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	return nil
}
