package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// deletionFixture は、memberFixture（自宅：アリスがオーナー、ボブが編集者、キャロルが閲覧者）に、
// ボブだけの世帯（実家）と、自宅でのボブ・アリスの記録・メモ・未送信の完了通知・ボブの端末を足した状態である。
type deletionFixture struct {
	memberFixture
	users       *postgres.UserRepository
	parents     uuid.UUID // ボブだけの世帯（ボブがオーナー）
	bobRecord   domain.CareRecord
	aliceRecord domain.CareRecord
	bobNote     domain.Note
}

func newDeletionFixture(t *testing.T) deletionFixture {
	t.Helper()
	f := deletionFixture{memberFixture: newMemberFixture(t)}
	f.users = postgres.NewUserRepository(f.pool)
	ctx := t.Context()
	parents, err := postgres.NewHouseholdRepository(f.pool).CreateWithOwner(ctx, f.bob, domain.NewHousehold{Name: "実家"})
	if err != nil {
		t.Fatal(err)
	}
	f.parents = parents.ID
	if _, err := postgres.NewPlantRepository(f.pool).Create(ctx, f.parents, domain.PlantProfile{Name: "サボテン"}); err != nil {
		t.Fatal(err)
	}

	plant, err := postgres.NewPlantRepository(f.pool).Create(ctx, f.household, domain.PlantProfile{Name: "モンステラ"})
	if err != nil {
		t.Fatal(err)
	}
	tasks, records := postgres.NewCareTaskRepository(f.pool), postgres.NewCareRecordRepository(f.pool)
	complete := func(kind domain.CareKind, actor uuid.UUID) domain.CareRecord {
		d := wateringDraft
		d.Kind = kind
		task, err := tasks.Create(ctx, f.household, plant.ID, d)
		if err != nil {
			t.Fatal(err)
		}
		rec, _, err := records.Complete(ctx, f.household, task.ID, actor, domain.Completion{CycleSeq: 1, PerformedOn: civil.Date{Year: 2026, Month: 11, Day: 4}})
		if err != nil {
			t.Fatal(err)
		}
		return rec
	}
	f.bobRecord = complete(domain.CareWatering, f.bob)
	f.aliceRecord = complete(domain.CareFertilizing, f.alice)
	if f.bobNote, err = postgres.NewNoteRepository(f.pool).Create(ctx, f.household, plant.ID, f.bob, "新芽が出た"); err != nil {
		t.Fatal(err)
	}
	reg, err := domain.NewDeviceRegistration("bob-phone", "ios", "Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.NewDeviceRepository(f.pool).Upsert(ctx, f.bob, reg); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f deletionFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// deleteUser は、ユーザーを削除し、削除の直前（コミットの前）に渡されたユーザーを返す（呼ばれなければ nil）。
func (f deletionFixture) deleteUser(t *testing.T, userID uuid.UUID, beforeErr error) (*domain.User, error) {
	t.Helper()
	var got *domain.User
	err := f.users.Delete(t.Context(), userID, func(_ context.Context, u domain.User) error {
		got = &u
		return beforeErr
	})
	return got, err
}

func TestUserRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("ほかにメンバーのいる世帯の唯一のオーナーなら、削除せずに、オーナー権限を移すべき世帯を返す（要件1-7）", func(t *testing.T) {
		t.Parallel()
		f := newDeletionFixture(t)
		// アリスだけの世帯は、移す必要がないので含めない
		if _, err := postgres.NewHouseholdRepository(f.pool).CreateWithOwner(t.Context(), f.alice, domain.NewHousehold{Name: "別荘"}); err != nil {
			t.Fatal(err)
		}
		got, err := f.deleteUser(t, f.alice, nil)
		var transfer *domain.OwnerTransferRequiredError
		if !errors.As(err, &transfer) {
			t.Fatalf("err = %v", err)
		}
		if want := []domain.Household{{ID: f.household, Name: "自宅"}}; !reflect.DeepEqual(transfer.Households, want) {
			t.Errorf("households = %+v", transfer.Households)
		}
		// 外部の削除（Apple・Firebase）も呼ばない
		if got != nil {
			t.Error("before-commit hook was called")
		}
		if n := f.count(t, `SELECT count(*) FROM users WHERE id = $1`, f.alice); n != 1 {
			t.Errorf("users = %d", n)
		}
	})

	t.Run("自分だけの世帯は関連データごと消し、ほかの世帯からは外し、記録とメモの作成者を匿名化する（要件1-6）", func(t *testing.T) {
		t.Parallel()
		f := newDeletionFixture(t)
		w := watchChanges(t, f.pool)
		var got *domain.User
		assertChanges(t, w, func() []change {
			var err error
			got, err = f.deleteUser(t, f.bob, nil)
			must := func(err error) {
				if err != nil {
					t.Fatal(err)
				}
			}
			must(err)
			return []change{
				{f.parents, domain.ChangeHousehold, f.parents, domain.ChangeDeleted, uuid.Nil},
				{f.household, domain.ChangeMember, f.bob, domain.ChangeDeleted, uuid.Nil},
			}
		})
		if got == nil || got.ID != f.bob || got.FirebaseUID != "uid-bob" {
			t.Errorf("hook user = %+v", got)
		}

		checks := []struct {
			name string
			sql  string
			args []any
			want int
		}{
			{"ボブだけの世帯", `SELECT count(*) FROM households WHERE id = $1`, []any{f.parents}, 0},
			{"ボブだけの世帯の植物", `SELECT count(*) FROM plants WHERE household_id = $1`, []any{f.parents}, 0},
			{"自宅は残る", `SELECT count(*) FROM households WHERE id = $1`, []any{f.household}, 1},
			{"ボブのメンバーシップ", `SELECT count(*) FROM memberships WHERE user_id = $1`, []any{f.bob}, 0},
			{"ほかのメンバーは残る", `SELECT count(*) FROM memberships WHERE household_id = $1`, []any{f.household}, 2},
			{"ボブの記録は残し、匿名化する", `SELECT count(*) FROM care_records WHERE id = $1 AND actor_user_id IS NULL AND actor_display_name = '退会したユーザー'`, []any{f.bobRecord.ID}, 1},
			{"アリスの記録は変えない", `SELECT count(*) FROM care_records WHERE id = $1 AND actor_user_id = $2 AND actor_display_name = 'アリス'`, []any{f.aliceRecord.ID, f.alice}, 1},
			{"ボブのメモは残し、匿名化する", `SELECT count(*) FROM notes WHERE id = $1 AND author_user_id IS NULL AND author_display_name = '退会したユーザー'`, []any{f.bobNote.ID}, 1},
			{"ボブの未送信の完了通知は送らない", `SELECT count(*) FROM notification_outbox WHERE record_id = $1 AND status = 'canceled'`, []any{f.bobRecord.ID}, 1},
			{"アリスの完了通知は送る", `SELECT count(*) FROM notification_outbox WHERE record_id = $1 AND status = 'pending'`, []any{f.aliceRecord.ID}, 1},
			{"ボブの端末", `SELECT count(*) FROM user_devices WHERE fcm_token = 'bob-phone'`, nil, 0},
			// ユーザーの行は、削除済みの印として残す（表示名は消す）。古い ID トークンで、ユーザーを作り直させないため
			{"ボブのユーザー（削除済みの印）", `SELECT count(*) FROM users WHERE id = $1 AND deleted_at IS NOT NULL AND display_name = ''`, []any{f.bob}, 1},
		}
		for _, c := range checks {
			if n := f.count(t, c.sql, c.args...); n != c.want {
				t.Errorf("%s: count = %d, want %d", c.name, n, c.want)
			}
		}

		// ほかの端末に残っている ID トークン（最大1時間有効）で呼ばれても、ユーザーを作り直さない
		if _, err := f.users.GetOrCreate(t.Context(), domain.Identity{UID: "uid-bob", Name: "ボブ", Provider: "google.com"}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("sign in after deletion: err = %v", err)
		}
		// 削除済みのユーザーは、もう一度は消せない
		if _, err := f.deleteUser(t, f.bob, nil); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("delete again: err = %v", err)
		}
	})

	t.Run("削除の直前の処理（Apple・Firebase）が失敗したら、何も消さない", func(t *testing.T) {
		t.Parallel()
		f := newDeletionFixture(t)
		boom := errors.New("apple is down")
		if _, err := f.deleteUser(t, f.bob, boom); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
		if n := f.count(t, `SELECT count(*) FROM users WHERE id = $1`, f.bob); n != 1 {
			t.Errorf("users = %d", n)
		}
		if n := f.count(t, `SELECT count(*) FROM households WHERE id = $1`, f.parents); n != 1 {
			t.Errorf("households = %d", n)
		}
		if n := f.count(t, `SELECT count(*) FROM care_records WHERE actor_display_name = 'ボブ'`); n != 1 {
			t.Errorf("bob's records = %d", n)
		}
	})

	t.Run("ユーザーがいなければ domain.ErrNotFound", func(t *testing.T) {
		t.Parallel()
		f := newDeletionFixture(t)
		if _, err := f.deleteUser(t, uuid.New(), nil); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("確かめている途中に参加したメンバーも、数に入れる", func(t *testing.T) {
		t.Parallel()
		f := newDeletionFixture(t)
		// 実家への参加のトランザクションが、メンバーシップを作ったところ（まだコミットしていない）
		tx, err := f.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) }()
		if _, err := tx.Exec(t.Context(), `INSERT INTO memberships (household_id, user_id, role) VALUES ($1, $2, 'viewer')`, f.parents, f.dave); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := f.deleteUser(t, f.bob, nil)
			done <- err
		}()
		// 削除は、参加が確定するまで待つ（世帯の行のロック）
		select {
		case err := <-done:
			t.Fatalf("deleted while a member was joining: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		if err := tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
		// 実家にはデイブがいるので、ボブが唯一のオーナーの、ほかにメンバーのいる世帯になる
		var transfer *domain.OwnerTransferRequiredError
		if err := <-done; !errors.As(err, &transfer) || len(transfer.Households) != 1 || transfer.Households[0].ID != f.parents {
			t.Errorf("err = %v", err)
		}
	})
}
