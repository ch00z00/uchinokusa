package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// ポリシー表のすべての組み合わせ（ロール × 操作）を確かめる（要件4-1・4-2）。
// 期待値はこのテストで独立に書き、実装の表を写さない。
func TestRole_Allows(t *testing.T) {
	t.Parallel()

	const (
		o = "owner"
		e = "editor"
		v = "viewer"
	)
	// 操作ごとに、許可するロール
	want := map[domain.Action][]string{
		domain.ActionHouseholdManage: {o},
		domain.ActionInviteManage:    {o},
		domain.ActionMemberManage:    {o},
		domain.ActionPlantWrite:      {o, e},
		domain.ActionTaskWrite:       {o, e},
		domain.ActionRecordWrite:     {o, e},
		domain.ActionNoteWrite:       {o, e},
		domain.ActionRead:            {o, e, v},
		domain.ActionSettingsSelf:    {o, e, v},
	}

	// 実装に、テストで期待値を書いていない操作があれば気づけるようにする
	for _, a := range domain.Actions() {
		if _, ok := want[a]; !ok {
			t.Errorf("action %q has no expectation in this test", a)
		}
	}

	roles := []domain.Role{domain.RoleOwner, domain.RoleEditor, domain.RoleViewer}
	for action, allowed := range want {
		for _, role := range roles {
			wantAllowed := false
			for _, r := range allowed {
				wantAllowed = wantAllowed || r == string(role)
			}
			if got := role.Allows(action); got != wantAllowed {
				t.Errorf("%s.Allows(%s) = %v, want %v", role, action, got, wantAllowed)
			}
		}
	}
}

// 表にないロール・操作は、許可しない（明示的に許可したものだけを通す）。
func TestRole_Allows_Unknown(t *testing.T) {
	t.Parallel()

	if domain.Role("admin").Allows(domain.ActionRead) {
		t.Error("unknown role was allowed to read")
	}
	if domain.Role("").Allows(domain.ActionRead) {
		t.Error("empty role was allowed to read")
	}
	if domain.RoleOwner.Allows(domain.Action("plant.destroy_everything")) {
		t.Error("owner was allowed an unknown action")
	}
}

func TestParseRole(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"owner", "editor", "viewer"} {
		if r, err := domain.ParseRole(s); err != nil || string(r) != s {
			t.Errorf("ParseRole(%q) = %q, %v", s, r, err)
		}
	}
	for _, s := range []string{"", "admin", "Owner"} {
		if _, err := domain.ParseRole(s); err == nil {
			t.Errorf("ParseRole(%q): want error", s)
		}
	}
}

// メモの編集・削除は、作成者本人かオーナーに限る（要件13-3）。
func TestMembership_CanModifyNote(t *testing.T) {
	t.Parallel()

	me := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	byMe := uuid.NullUUID{UUID: me, Valid: true}
	byOther := uuid.NullUUID{UUID: other, Valid: true}
	deletedAuthor := uuid.NullUUID{} // 作成者が退会した

	tests := []struct {
		name    string
		role    domain.Role
		author  uuid.NullUUID
		wantErr error
	}{
		{name: "編集者は、自分のメモを編集できる", role: domain.RoleEditor, author: byMe},
		{name: "編集者は、ほかの人のメモを編集できない", role: domain.RoleEditor, author: byOther, wantErr: domain.ErrForbidden},
		{name: "編集者は、退会した人のメモを編集できない", role: domain.RoleEditor, author: deletedAuthor, wantErr: domain.ErrForbidden},
		{name: "オーナーは、自分のメモを編集できる", role: domain.RoleOwner, author: byMe},
		{name: "オーナーは、ほかの人のメモを編集できる", role: domain.RoleOwner, author: byOther},
		{name: "オーナーは、退会した人のメモを編集できる", role: domain.RoleOwner, author: deletedAuthor},
		// 閲覧者はメモを書けない（note.write がない）。以前に編集者として書いたメモでも、今は編集できない
		{name: "閲覧者は、自分のメモでも編集できない", role: domain.RoleViewer, author: byMe, wantErr: domain.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := domain.Membership{HouseholdID: uuid.Must(uuid.NewV7()), UserID: me, Role: tt.role}
			err := m.CanModifyNote(tt.author)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
