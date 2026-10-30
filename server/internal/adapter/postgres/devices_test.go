package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// createUser は、テスト用のユーザーを作り、その ID を返す。
func createUser(t *testing.T, pool *pgxpool.Pool, uid string) uuid.UUID {
	t.Helper()
	u, err := postgres.NewUserRepository(pool).GetOrCreate(t.Context(), domain.Identity{UID: uid, Provider: "google.com"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// countDevices は、FCM トークンが一致する端末の数を返す。
func countDevices(t *testing.T, pool *pgxpool.Pool, fcmToken string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM user_devices WHERE fcm_token = $1`, fcmToken).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDeviceRepository_Upsert(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewDeviceRepository(pool)
	userID := createUser(t, pool, "uid-1")

	created, err := repo.Upsert(ctx, userID, domain.DeviceRegistration{FCMToken: "token-1", Platform: domain.PlatformIOS, TimeZone: "Asia/Tokyo"})
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if created.ID.Version() != 7 || created.UserID != userID || created.Platform != domain.PlatformIOS || created.TimeZone != "Asia/Tokyo" {
		t.Errorf("created = %+v", created)
	}

	// 同じトークンでの2回目（アプリの再起動・旅行先でのタイムゾーンの変更）は、同じ端末を更新する
	updated, err := repo.Upsert(ctx, userID, domain.DeviceRegistration{FCMToken: "token-1", Platform: domain.PlatformIOS, TimeZone: "Europe/London"})
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if updated.ID != created.ID || updated.TimeZone != "Europe/London" {
		t.Errorf("updated = %+v, want the same device with the new time zone", updated)
	}
	if !updated.LastSeenAt.After(created.LastSeenAt) {
		t.Errorf("LastSeenAt = %v, want after %v", updated.LastSeenAt, created.LastSeenAt)
	}
	if n := countDevices(t, pool, "token-1"); n != 1 {
		t.Errorf("devices = %d, want 1", n)
	}
}

// 1台の端末でユーザーを切り替えた場合、トークンは新しいユーザーに付け替える（前のユーザーへの通知が届かないようにする）。
func TestDeviceRepository_Upsert_ReassignsToNewUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewDeviceRepository(pool)
	alice := createUser(t, pool, "uid-alice")
	bob := createUser(t, pool, "uid-bob")

	if _, err := repo.Upsert(ctx, alice, domain.DeviceRegistration{FCMToken: "shared-token", Platform: domain.PlatformAndroid, TimeZone: "Asia/Tokyo"}); err != nil {
		t.Fatal(err)
	}
	d, err := repo.Upsert(ctx, bob, domain.DeviceRegistration{FCMToken: "shared-token", Platform: domain.PlatformAndroid, TimeZone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if d.UserID != bob {
		t.Errorf("UserID = %s, want bob (%s)", d.UserID, bob)
	}
	if n := countDevices(t, pool, "shared-token"); n != 1 {
		t.Errorf("devices = %d, want 1", n)
	}

	// 付け替えたあとは、前のユーザーのサインアウトで、新しいユーザーの端末が消えない
	if err := repo.Delete(ctx, alice, d.ID); err != nil {
		t.Fatal(err)
	}
	if n := countDevices(t, pool, "shared-token"); n != 1 {
		t.Errorf("devices after alice's delete = %d, want 1", n)
	}
}

func TestDeviceRepository_Delete(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewDeviceRepository(pool)
	userID := createUser(t, pool, "uid-1")

	d, err := repo.Upsert(ctx, userID, domain.DeviceRegistration{FCMToken: "token-1", Platform: domain.PlatformIOS, TimeZone: "Asia/Tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, userID, d.ID); err != nil {
		t.Fatal(err)
	}
	if n := countDevices(t, pool, "token-1"); n != 0 {
		t.Errorf("devices = %d, want 0", n)
	}

	// すでに消えている端末を消しても、エラーにしない（サインアウトを止めない）
	if err := repo.Delete(ctx, userID, d.ID); err != nil {
		t.Errorf("second Delete: %v", err)
	}
}
