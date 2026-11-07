package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// outboxFixture は、世帯（自宅）に、さくら（編集者）・母（オーナー）・祖母（閲覧者）・父（閲覧者、完了通知を切っている）がいる状態である。
type outboxFixture struct {
	recordFixture
	outbox            *postgres.OutboxRepository
	mom, grandma, dad uuid.UUID
	fertilizing       domain.CareTask // ポトスの肥料のタスク
	completion        domain.NoticeGroup
}

func newOutboxFixture(t *testing.T) outboxFixture {
	t.Helper()
	f := outboxFixture{recordFixture: newRecordFixture(t)}
	f.outbox = postgres.NewOutboxRepository(f.pool)
	f.mom, f.grandma, f.dad = uuid.MustParse(f.user()), uuid.MustParse(f.user()), uuid.MustParse(f.user())
	f.exec(`UPDATE users SET display_name = '母' WHERE id = $1`, f.mom)
	home := f.home.String()
	f.membership(home, f.sakura.String(), "editor")
	f.membership(home, f.mom.String(), "owner")
	f.membership(home, f.grandma.String(), "viewer")
	f.membership(home, f.dad.String(), "viewer")
	f.exec(`UPDATE memberships SET completion_notify_enabled = false WHERE user_id = $1`, f.dad)
	d := wateringDraft
	d.Kind = domain.CareFertilizing
	f.fertilizing = f.createTask(f.pothosPlant, d)
	f.completion = domain.NoticeGroup{HouseholdID: f.home, Kind: domain.NoticeCompletion, ActorUserID: f.sakura}
	return f
}

// completeTask は、さくらがタスクを完了する（アウトボックスに完了通知を積む）。
func (f outboxFixture) completeTask(task domain.CareTask) domain.CareRecord {
	f.t.Helper()
	rec, _, err := f.records.Complete(f.ctx, f.home, task.ID, f.sakura, domain.Completion{CycleSeq: task.CycleSeq, PerformedOn: performedOn})
	if err != nil {
		f.t.Fatal(err)
	}
	return rec
}

// age は、アウトボックスのすべての行を d だけ古くする（まとめる時間を過ぎたことにする）。
func (f outboxFixture) age(d time.Duration) {
	f.t.Helper()
	f.exec(`UPDATE notification_outbox SET created_at = created_at - make_interval(secs => $1)`, d.Seconds())
}

func (f outboxFixture) statuses() map[string]int {
	f.t.Helper()
	rows, err := f.pool.Query(f.ctx, `SELECT status, count(*) FROM notification_outbox GROUP BY status`)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			f.t.Fatal(err)
		}
		out[s] = n
	}
	return out
}

// dispatch は、まとまりを送り、send に渡された内容（呼ばれなければ nil）を返す。sendErr を send の結果にする。
func (f outboxFixture) dispatch(g domain.NoticeGroup, sendErr error) (domain.DispatchOutcome, *domain.NoticeBatch) {
	f.t.Helper()
	var got *domain.NoticeBatch
	out, err := f.outbox.Dispatch(f.ctx, g, func(_ context.Context, b domain.NoticeBatch) error {
		got = &b
		return sendErr
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return out, got
}

func TestOutboxRepository_ReadyGroups(t *testing.T) {
	t.Parallel()
	f := newOutboxFixture(t)
	now := time.Now()
	add := func(kind string, actor uuid.UUID, status string, ago time.Duration) {
		f.exec(`INSERT INTO notification_outbox (household_id, kind, actor_user_id, status, created_at) VALUES ($1, $2, $3, $4, $5)`,
			f.home, kind, actor, status, now.Add(-ago))
	}
	// さくら：最後の完了から60秒を過ぎた
	add("completion", f.sakura, "pending", 90*time.Second)
	add("completion", f.sakura, "pending", 70*time.Second)
	// 母：続けて完了している（最後の完了から30秒）
	add("completion", f.mom, "pending", 90*time.Second)
	add("completion", f.mom, "pending", 30*time.Second)
	// 祖母：続けて完了しているが、最初の完了から10分を過ぎた
	add("completion", f.grandma, "pending", 11*time.Minute)
	add("completion", f.grandma, "pending", 10*time.Second)
	// 父：送った・取り消した行だけ
	add("completion", f.dad, "sent", time.Hour)
	add("completion", f.dad, "canceled", time.Hour)
	// 参加の通知は、完了とは別のまとまり
	add("member_joined", f.sakura, "pending", 2*time.Minute)

	got, err := f.outbox.ReadyGroups(f.ctx, now.Add(-time.Minute), now.Add(-10*time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	// 待っている時間の長い順
	want := []domain.NoticeGroup{
		{HouseholdID: f.home, Kind: domain.NoticeCompletion, ActorUserID: f.grandma},
		{HouseholdID: f.home, Kind: domain.NoticeMemberJoined, ActorUserID: f.sakura},
		{HouseholdID: f.home, Kind: domain.NoticeCompletion, ActorUserID: f.sakura},
	}
	if !slices.Equal(got, want) {
		t.Errorf("groups = %+v\nwant     %+v", got, want)
	}

	limited, err := f.outbox.ReadyGroups(f.ctx, now.Add(-time.Minute), now.Add(-10*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(limited, want[:1]) {
		t.Errorf("limited = %+v", limited)
	}
}

func TestOutboxRepository_Dispatch(t *testing.T) {
	t.Parallel()

	t.Run("完了をまとめて送り、送信済みにする", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		f.completeTask(f.task)
		f.completeTask(f.fertilizing)
		f.age(2 * time.Minute)

		out, b := f.dispatch(f.completion, nil)
		if out != domain.DispatchSent || b == nil {
			t.Fatalf("outcome = %v, batch = %v", out, b)
		}
		want := domain.NoticeBatch{
			NoticeGroup: f.completion, HouseholdName: "自宅", ActorName: "さくら",
			Items: []domain.CompletionItem{
				{PlantID: f.monsteraPlant, PlantName: "モンステラ", Kind: domain.CareWatering},
				{PlantID: f.pothosPlant, PlantName: "ポトス", Kind: domain.CareFertilizing},
			},
			// 作業した本人と、完了通知を切っている人には送らない（要件16-2・16-3）。閲覧者にも送る
			Recipients: sortedIDs(f.mom, f.grandma),
		}
		if !reflect.DeepEqual(*b, want) {
			t.Errorf("batch = %+v\nwant    %+v", *b, want)
		}
		if st := f.statuses(); st["sent"] != 2 || len(st) != 1 {
			t.Errorf("statuses = %v", st)
		}
		if n := f.count(`SELECT count(*) FROM notification_outbox WHERE sent_at IS NULL`); n != 0 {
			t.Errorf("rows without sent_at = %d", n)
		}

		// 二重に実行しても、送り直さない
		if out, b := f.dispatch(f.completion, nil); out != domain.DispatchSkipped || b != nil {
			t.Errorf("again: outcome = %v, batch = %v", out, b)
		}
	})

	t.Run("取り消した記録と、植物の削除で消えた記録は送らない（要件16-6）", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		undone := f.completeTask(f.task)
		if _, err := f.records.Undo(f.ctx, f.home, undone.ID, f.sakura, undone.CreatedAt); err != nil {
			t.Fatal(err)
		}
		deleted := f.completeTask(f.fertilizing)
		// 植物を削除すると、記録も消え、アウトボックスの行は record_id が NULL で残る
		_ = deleted
		if err := f.plantFixture.repo.Delete(f.ctx, f.home, f.pothosPlant); err != nil {
			t.Fatal(err)
		}
		f.age(2 * time.Minute)

		if out, b := f.dispatch(f.completion, nil); out != domain.DispatchSkipped || b != nil {
			t.Errorf("outcome = %v, batch = %v", out, b)
		}
		// 消えた記録の行も、取り消し済みにする（何度も読み直さないように）
		if st := f.statuses(); st["canceled"] != 2 || len(st) != 1 {
			t.Errorf("statuses = %v", st)
		}
	})

	t.Run("一部の記録が消えたら、残りだけを送る", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		f.completeTask(f.task)
		deleted := f.completeTask(f.fertilizing)
		// 植物を削除すると、記録も消え、アウトボックスの行は record_id が NULL で残る
		_ = deleted
		if err := f.plantFixture.repo.Delete(f.ctx, f.home, f.pothosPlant); err != nil {
			t.Fatal(err)
		}
		f.age(2 * time.Minute)
		out, b := f.dispatch(f.completion, nil)
		if out != domain.DispatchSent || b == nil || len(b.Items) != 1 || b.Items[0].PlantID != f.monsteraPlant {
			t.Fatalf("outcome = %v, batch = %+v", out, b)
		}
		if st := f.statuses(); st["sent"] != 1 || st["canceled"] != 1 {
			t.Errorf("statuses = %v", st)
		}
	})

	t.Run("ほかの人の完了は、別の通知にする", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		f.completeTask(f.task)
		if _, _, err := f.records.Complete(f.ctx, f.home, f.fertilizing.ID, f.mom, domain.Completion{CycleSeq: 1, PerformedOn: performedOn}); err != nil {
			t.Fatal(err)
		}
		f.age(2 * time.Minute)
		out, b := f.dispatch(f.completion, nil)
		if out != domain.DispatchSent || b == nil || len(b.Items) != 1 || b.Items[0].PlantID != f.monsteraPlant {
			t.Fatalf("outcome = %v, batch = %+v", out, b)
		}
		// 母の完了は、まだ送っていない
		if n := f.count(`SELECT count(*) FROM notification_outbox WHERE actor_user_id = $1 AND status = 'pending'`, f.mom); n != 1 {
			t.Errorf("mom's pending = %d", n)
		}
	})

	t.Run("その他の作業は、作業名を渡す", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		d := wateringDraft
		d.Kind, d.CustomName = domain.CareOther, "葉水"
		f.completeTask(f.createTask(f.monsteraPlant, d))
		f.age(2 * time.Minute)
		_, b := f.dispatch(f.completion, nil)
		want := []domain.CompletionItem{{PlantID: f.monsteraPlant, PlantName: "モンステラ", Kind: domain.CareOther, CustomName: "葉水"}}
		if b == nil || !reflect.DeepEqual(b.Items, want) {
			t.Errorf("batch = %+v", b)
		}
	})

	t.Run("送れなければ送り直し、3回で諦める", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		f.completeTask(f.task)
		f.age(2 * time.Minute)
		boom := errors.New("fcm is down")
		for i, wantStatus := range []string{"pending", "pending", "failed"} {
			out, b := f.dispatch(f.completion, boom)
			if out != domain.DispatchFailed || b == nil {
				t.Fatalf("attempt %d: outcome = %v", i+1, out)
			}
			var status string
			var attempts int
			f.queryRow(`SELECT status, attempts FROM notification_outbox`).scan(&status, &attempts)
			if status != wantStatus || attempts != i+1 {
				t.Errorf("attempt %d: status = %s, attempts = %d", i+1, status, attempts)
			}
		}
		if out, _ := f.dispatch(f.completion, nil); out != domain.DispatchSkipped {
			t.Errorf("after failed: outcome = %v", out)
		}
	})

	t.Run("参加の通知は、オーナーに送る（要件3-8）", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		bob := uuid.MustParse(f.user())
		f.exec(`UPDATE users SET display_name = 'ボブ' WHERE id = $1`, bob)
		f.membership(f.home.String(), bob.String(), "viewer")
		f.exec(`INSERT INTO notification_outbox (household_id, kind, actor_user_id, payload) VALUES ($1, 'member_joined', $2, '{"role":"viewer"}')`, f.home, bob)
		f.age(2 * time.Minute)

		g := domain.NoticeGroup{HouseholdID: f.home, Kind: domain.NoticeMemberJoined, ActorUserID: bob}
		out, b := f.dispatch(g, nil)
		want := domain.NoticeBatch{NoticeGroup: g, HouseholdName: "自宅", ActorName: "ボブ", Recipients: []uuid.UUID{f.mom}}
		if out != domain.DispatchSent || b == nil || !reflect.DeepEqual(*b, want) {
			t.Errorf("outcome = %v, batch = %+v", out, b)
		}
	})

	t.Run("参加したユーザーが退会していれば、参加の通知は送らない", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		f.exec(`INSERT INTO notification_outbox (household_id, kind, actor_user_id) VALUES ($1, 'member_joined', NULL)`, f.home)
		f.age(2 * time.Minute)
		g := domain.NoticeGroup{HouseholdID: f.home, Kind: domain.NoticeMemberJoined}
		if out, b := f.dispatch(g, nil); out != domain.DispatchSkipped || b != nil {
			t.Errorf("outcome = %v, batch = %v", out, b)
		}
		if st := f.statuses(); st["canceled"] != 1 {
			t.Errorf("statuses = %v", st)
		}
	})

	t.Run("取り消しの途中の行は飛ばし、取り消しをやめたら次に送る", func(t *testing.T) {
		t.Parallel()
		f := newOutboxFixture(t)
		first := f.completeTask(f.task)
		f.completeTask(f.fertilizing)
		f.age(2 * time.Minute)

		// 取り消しのトランザクションが、1件目の行をロックしている（まだコミットしていない）
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		// テストが途中で失敗しても、接続を返す（返さないと、後片付けでプールを閉じるのが終わらない）
		defer func() { _ = tx.Rollback(context.WithoutCancel(f.ctx)) }()
		if _, err := tx.Exec(f.ctx, `UPDATE notification_outbox SET status = 'canceled' WHERE record_id = $1`, first.ID); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
		defer cancel()
		var got domain.NoticeBatch
		out, err := f.outbox.Dispatch(ctx, f.completion, func(_ context.Context, b domain.NoticeBatch) error {
			got = b
			return nil
		})
		if err != nil || out != domain.DispatchSent || len(got.Items) != 1 || got.Items[0].PlantID != f.pothosPlant {
			t.Fatalf("outcome = %v, err = %v, items = %+v", out, err, got.Items)
		}

		// 取り消しをやめたので、1件目は次の実行で送る
		if err := tx.Rollback(f.ctx); err != nil {
			t.Fatal(err)
		}
		out, b := f.dispatch(f.completion, nil)
		if out != domain.DispatchSent || b == nil || len(b.Items) != 1 || b.Items[0].PlantID != f.monsteraPlant {
			t.Errorf("after rollback: outcome = %v, batch = %+v", out, b)
		}
	})
}

func sortedIDs(ids ...uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return ids
}
