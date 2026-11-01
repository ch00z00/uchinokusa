package domain

import (
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"
)

// Role は、世帯でのメンバーのロールである。ユーザーは世帯ごとに異なるロールを持てる（要件2-3）。
type Role string

const (
	RoleOwner  Role = "owner"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// ParseRole は、文字列をロールにする。知らない値はエラーにする。
func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleOwner, RoleEditor, RoleViewer:
		return r, nil
	}
	return "", fmt.Errorf("unknown role %q", s)
}

// Action は、権限の判定の単位となる操作である。
type Action string

const (
	ActionHouseholdManage Action = "household.manage" // 世帯名の変更、世帯の削除
	ActionInviteManage    Action = "invite.manage"    // 招待の作成・取り消し
	ActionMemberManage    Action = "member.manage"    // ロールの変更、メンバーの削除、オーナー権限の移譲
	ActionPlantWrite      Action = "plant.write"      // 植物・置き場所の作成・編集・削除
	ActionTaskWrite       Action = "task.write"       // お手入れタスクの作成・編集・削除・一時停止
	ActionRecordWrite     Action = "record.write"     // 完了・スキップ・取り消し・記録の編集
	ActionNoteWrite       Action = "note.write"       // メモの作成（編集・削除は CanModifyNote も確かめる）
	ActionRead            Action = "read"             // 世帯のデータの閲覧
	ActionSettingsSelf    Action = "settings.self"    // 自分の通知設定の変更
)

// policy は、操作ごとに許可するロールの表である（要件4-1・4-2）。
// ここにない操作は、どのロールにも許可しない。
var policy = map[Action][]Role{
	ActionHouseholdManage: {RoleOwner},
	ActionInviteManage:    {RoleOwner},
	ActionMemberManage:    {RoleOwner},
	ActionPlantWrite:      {RoleOwner, RoleEditor},
	ActionTaskWrite:       {RoleOwner, RoleEditor},
	ActionRecordWrite:     {RoleOwner, RoleEditor},
	ActionNoteWrite:       {RoleOwner, RoleEditor},
	ActionRead:            {RoleOwner, RoleEditor, RoleViewer},
	ActionSettingsSelf:    {RoleOwner, RoleEditor, RoleViewer},
}

// Actions は、ポリシー表にあるすべての操作を返す（テストで、表の漏れを確かめるため）。
func Actions() []Action {
	return slices.Collect(maps.Keys(policy))
}

// Allows は、ロールが操作を許可するかを返す。知らないロール・操作は許可しない。
func (r Role) Allows(a Action) bool {
	return slices.Contains(policy[a], r)
}

// Membership は、ユーザーの世帯への所属とロールである。
type Membership struct {
	HouseholdID uuid.UUID
	UserID      uuid.UUID
	Role        Role
}

// CanModifyNote は、メモを編集・削除できるかを確かめる（要件13-3）。
// note.write を持ち、かつ作成者本人かオーナーである必要がある。作成者が退会している（author が NULL）場合は、オーナーだけが扱える。
// できない場合は ErrForbidden を返す。
func (m Membership) CanModifyNote(author uuid.NullUUID) error {
	if !m.Role.Allows(ActionNoteWrite) {
		return ErrForbidden
	}
	if m.Role == RoleOwner || (author.Valid && author.UUID == m.UserID) {
		return nil
	}
	return ErrForbidden
}
