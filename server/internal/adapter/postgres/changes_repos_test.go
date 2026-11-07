package postgres_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// change は、変更イベントのうち、書き込みごとに確かめる部分である（バージョンと操作した人は TestChanges_PublishedOnCommit で確かめる）。
type change struct {
	household uuid.UUID
	entity    domain.ChangeEntity
	id        uuid.UUID
	op        domain.ChangeOp
	plant     uuid.UUID
}

func changesOf(events []domain.ChangeEvent) []change {
	out := make([]change, 0, len(events))
	for _, e := range events {
		out = append(out, change{e.HouseholdID, e.Entity, e.EntityID, e.Op, e.PlantID})
	}
	return out
}

// assertChanges は、op を実行したときに届いた変更イベントが want と同じ順で同じことを確かめる。
// バージョン番号が1つずつ進むことも確かめる。
func assertChanges(t *testing.T, w changeWatcher, op func() (want []change)) {
	t.Helper()
	want := op()
	events := w.collect()
	if got := changesOf(events); !slices.Equal(got, want) {
		t.Errorf("changes = %+v\nwant      %+v", got, want)
	}
	for i := 1; i < len(events); i++ {
		if events[i].HouseholdID == events[i-1].HouseholdID && events[i].Version != events[i-1].Version+1 {
			t.Errorf("versions = %d, %d", events[i-1].Version, events[i].Version)
		}
	}
}

// TestChanges_EveryWrite は、世帯のデータを変える書き込みが、どれも変更イベントを出すことを確かめる（要件11-1）。
// 書き込みを足したら、ここにも足す。
func TestChanges_EveryWrite(t *testing.T) {
	t.Parallel()
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("植物", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		w := watchChanges(t, f.pool)
		var id uuid.UUID
		assertChanges(t, w, func() []change {
			p, err := f.repo.Create(f.ctx, f.home, domain.PlantProfile{Name: "パキラ"})
			must(t, err)
			id = p.ID
			_, err = f.repo.Update(f.ctx, f.home, id, domain.PlantPatch{Name: ptrTo("大きなパキラ")})
			must(t, err)
			_, err = f.repo.SetArchived(f.ctx, f.home, id, true)
			must(t, err)
			must(t, f.repo.Delete(f.ctx, f.home, id))
			return []change{
				{f.home, domain.ChangePlant, id, domain.ChangeCreated, id},
				{f.home, domain.ChangePlant, id, domain.ChangeUpdated, id},
				{f.home, domain.ChangePlant, id, domain.ChangeUpdated, id},
				{f.home, domain.ChangePlant, id, domain.ChangeDeleted, id},
			}
		})
	})

	t.Run("置き場所の削除", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture(t)
		w := watchChanges(t, f.pool)
		assertChanges(t, w, func() []change {
			must(t, postgres.NewLocationRepository(f.pool).Delete(f.ctx, f.home, f.living))
			return []change{{f.home, domain.ChangeLocation, f.living, domain.ChangeDeleted, uuid.Nil}}
		})
	})

	t.Run("タスク", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		w := watchChanges(t, f.pool)
		assertChanges(t, w, func() []change {
			task, err := f.repo.Create(f.ctx, f.home, f.monsteraPlant, wateringDraft)
			must(t, err)
			due := nov(20)
			_, err = f.repo.Update(f.ctx, f.home, task.ID, domain.CareTaskPatch{NextDueOn: &due})
			must(t, err)
			_, err = f.repo.SetStatus(f.ctx, f.home, task.ID, domain.TaskPaused, nil)
			must(t, err)
			must(t, f.repo.Delete(f.ctx, f.home, task.ID))
			return []change{
				{f.home, domain.ChangeTask, task.ID, domain.ChangeCreated, f.monsteraPlant},
				{f.home, domain.ChangeTask, task.ID, domain.ChangeUpdated, f.monsteraPlant},
				{f.home, domain.ChangeTask, task.ID, domain.ChangeUpdated, f.monsteraPlant},
				{f.home, domain.ChangeTask, task.ID, domain.ChangeDeleted, f.monsteraPlant},
			}
		})
	})

	t.Run("記録", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		w := watchChanges(t, f.pool)
		p, task := f.monsteraPlant, f.task.ID
		assertChanges(t, w, func() []change {
			done, _, err := f.records.Complete(f.ctx, f.home, task, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: nov(4)})
			must(t, err)
			// 追加の記録は、タスクを変えない
			extra, _, err := f.records.Complete(f.ctx, f.home, task, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: nov(4), AllowExtra: true})
			must(t, err)
			skipped, _, err := f.records.Skip(f.ctx, f.home, task, f.sakura, domain.Skip{CycleSeq: 2, PerformedOn: nov(5), RecheckOn: nov(6)})
			must(t, err)
			_, err = f.records.Undo(f.ctx, f.home, skipped.ID, f.sakura, skipped.CreatedAt)
			must(t, err)
			// 最新の完了は追加の記録（11/4）のままなので、done の実施日を変えても、次回予定は変えない（0023）
			_, _, err = f.records.UpdatePerformedOn(f.ctx, f.home, done.ID, domain.RecordPatch{PerformedOn: nov(3)})
			must(t, err)
			_, err = f.records.Delete(f.ctx, f.home, extra.ID)
			must(t, err)
			return []change{
				{f.home, domain.ChangeRecord, done.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeTask, task, domain.ChangeUpdated, p},
				{f.home, domain.ChangeRecord, extra.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeRecord, skipped.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeTask, task, domain.ChangeUpdated, p},
				{f.home, domain.ChangeTask, task, domain.ChangeUpdated, p},
				{f.home, domain.ChangeRecord, skipped.ID, domain.ChangeDeleted, p},
				{f.home, domain.ChangeRecord, done.ID, domain.ChangeUpdated, p},
				// 追加の記録（11/4）は、実施日を 11/3 に変えたあとの最新の完了なので、消すと次回予定を計算し直す（0023）
				{f.home, domain.ChangeRecord, extra.ID, domain.ChangeDeleted, p},
				{f.home, domain.ChangeTask, task, domain.ChangeUpdated, p},
			}
		})
	})

	t.Run("季節の切り替え", func(t *testing.T) {
		t.Parallel()
		f := newSeasonFixture(t)
		w := watchChanges(t, f.pool)
		assertChanges(t, w, func() []change {
			_, err := f.seasons.Recalculate(f.ctx, f.home, dec1, domain.HemisphereNorth)
			must(t, err)
			return []change{{f.home, domain.ChangeTask, f.task.ID, domain.ChangeUpdated, f.monsteraPlant}}
		})
	})

	t.Run("完了済みの回への重複は、何も変えないのでイベントも出さない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		must(t, f.complete(1))
		w := watchChanges(t, f.pool)
		assertChanges(t, w, func() []change {
			if err := f.complete(1); err == nil {
				t.Fatal("err = nil")
			}
			return []change{}
		})
	})

	t.Run("メモ", func(t *testing.T) {
		t.Parallel()
		f := newNoteFixture(t)
		w := watchChanges(t, f.pool)
		assertChanges(t, w, func() []change {
			n := f.write("新芽が出た")
			_, err := f.notes.Update(f.ctx, f.home, n.ID, "新芽が2枚出た")
			must(t, err)
			must(t, f.notes.Delete(f.ctx, f.home, n.ID))
			return []change{
				{f.home, domain.ChangeNote, n.ID, domain.ChangeCreated, f.plant},
				{f.home, domain.ChangeNote, n.ID, domain.ChangeUpdated, f.plant},
				{f.home, domain.ChangeNote, n.ID, domain.ChangeDeleted, f.plant},
			}
		})
	})

	t.Run("提案", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		w := watchChanges(t, f.pool)
		p := f.monsteraPlant
		assertChanges(t, w, func() []change {
			accepted := f.save(f.repotting("initial"))
			// 同じキーの提案は保存しないので、イベントも出さない
			_, created, err := f.suggestions.Save(f.ctx, f.repotting("initial"))
			must(t, err)
			if created {
				t.Error("created = true")
			}
			_, task, err := f.suggestions.Accept(f.ctx, f.home, accepted.ID, f.sakura, domain.AcceptInput{}, nov(6))
			must(t, err)
			warn := f.repotting("warn")
			warn.Kind, warn.Action, warn.TaskID = domain.SuggestionAvoid, domain.SuggestionInfo, task.ID
			warning := f.save(warn)
			must(t, f.suggestions.Retract(f.ctx, f.home, "repotting.v1", task.ID, nil))
			dismissed := f.save(f.repotting("dismissed"))
			_, err = f.suggestions.Dismiss(f.ctx, f.home, dismissed.ID, f.sakura)
			must(t, err)
			return []change{
				{f.home, domain.ChangeSuggestion, accepted.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeTask, task.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeSuggestion, accepted.ID, domain.ChangeUpdated, p},
				{f.home, domain.ChangeSuggestion, warning.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeSuggestion, warning.ID, domain.ChangeUpdated, p},
				{f.home, domain.ChangeSuggestion, dismissed.ID, domain.ChangeCreated, p},
				{f.home, domain.ChangeSuggestion, dismissed.ID, domain.ChangeUpdated, p},
			}
		})
	})

	t.Run("世帯", func(t *testing.T) {
		t.Parallel()
		f := newMemberFixture(t)
		w := watchChanges(t, f.pool)
		households := postgres.NewHouseholdRepository(f.pool)
		assertChanges(t, w, func() []change {
			h, err := households.CreateWithOwner(t.Context(), f.dave, domain.NewHousehold{Name: "実家"})
			must(t, err)
			_, err = households.Update(t.Context(), h.ID, domain.HouseholdPatch{Name: ptrTo("祖母の家")})
			must(t, err)
			must(t, households.Delete(t.Context(), h.ID))
			return []change{
				{h.ID, domain.ChangeHousehold, h.ID, domain.ChangeCreated, uuid.Nil},
				{h.ID, domain.ChangeHousehold, h.ID, domain.ChangeUpdated, uuid.Nil},
				{h.ID, domain.ChangeHousehold, h.ID, domain.ChangeDeleted, uuid.Nil},
			}
		})
	})

	t.Run("メンバー", func(t *testing.T) {
		t.Parallel()
		f := newJoinFixture(t)
		w := watchChanges(t, f.pool)
		h := f.household
		members := postgres.NewMemberRepository(f.pool)
		assertChanges(t, w, func() []change {
			_, err := f.repo.Accept(t.Context(), domain.InvitationKey{Code: f.secret.Code}, f.dave)
			must(t, err)
			_, err = members.UpdateRole(t.Context(), h, f.carol, domain.RoleEditor)
			must(t, err)
			must(t, members.TransferOwnership(t.Context(), h, f.alice, f.bob))
			must(t, members.Remove(t.Context(), h, f.carol))
			_, err = postgres.NewUserRepository(f.pool).UpdateDisplayName(t.Context(), f.dave, "デイヴ")
			must(t, err)
			return []change{
				{h, domain.ChangeMember, f.dave, domain.ChangeCreated, uuid.Nil},
				{h, domain.ChangeMember, f.carol, domain.ChangeUpdated, uuid.Nil},
				{h, domain.ChangeMember, f.alice, domain.ChangeUpdated, uuid.Nil},
				{h, domain.ChangeMember, f.bob, domain.ChangeUpdated, uuid.Nil},
				{h, domain.ChangeMember, f.carol, domain.ChangeDeleted, uuid.Nil},
				{h, domain.ChangeMember, f.dave, domain.ChangeUpdated, uuid.Nil},
			}
		})
	})
}

// 通知の中身は、アプリに送るメッセージの項目の名前にそろえる（11.2 で、そのまま転送できるように）。
func TestChanges_PayloadShape(t *testing.T) {
	t.Parallel()
	f := newNoteFixture(t)
	conn, err := f.pool.Acquire(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(f.ctx, "LISTEN household_changes"); err != nil {
		t.Fatal(err)
	}
	n := f.write("新芽が出た")
	got, err := conn.Conn().WaitForNotification(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	// fixture が先に植物を作っている（バージョン 1）ので、メモはバージョン 2
	want := map[string]any{"householdId": f.home.String(), "version": float64(2), "entity": "note", "entityId": n.ID.String(), "op": "created", "plantId": f.plant.String()}
	if len(payload) != len(want) {
		t.Errorf("payload = %v", payload)
	}
	for k, v := range want {
		if payload[k] != v {
			t.Errorf("%s = %v, want %v", k, payload[k], v)
		}
	}
}
