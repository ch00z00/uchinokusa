package domain

import "github.com/google/uuid"

// ChangeEntity は、変更されたデータの種類である（設計の「イベントの取り決め」）。
type ChangeEntity string

const (
	ChangePlant      ChangeEntity = "plant"
	ChangeLocation   ChangeEntity = "location"
	ChangeTask       ChangeEntity = "task"
	ChangeRecord     ChangeEntity = "record"
	ChangeNote       ChangeEntity = "note"
	ChangeMember     ChangeEntity = "member" // EntityID はユーザーの ID
	ChangeHousehold  ChangeEntity = "household"
	ChangeSuggestion ChangeEntity = "suggestion"
)

// ChangeOp は、変更の操作である。
type ChangeOp string

const (
	ChangeCreated ChangeOp = "created"
	ChangeUpdated ChangeOp = "updated"
	ChangeDeleted ChangeOp = "deleted"
)

// ChangeEvent は、世帯のデータの変更が確定したことを、ほかのメンバーの画面に知らせるイベントである（要件11-1）。
// 中身は「何が変わったか」だけで、アプリは受け取ったら、そのデータを取り直す。
type ChangeEvent struct {
	HouseholdID uuid.UUID
	// Version は、世帯のバージョン番号である。変更のたびに1つ進むので、アプリは飛び（取りこぼし）に気づける。
	Version     int64
	Entity      ChangeEntity
	EntityID    uuid.UUID
	Op          ChangeOp
	ActorUserID uuid.UUID // 変更した人。わからなければ uuid.Nil
	PlantID     uuid.UUID // 植物に属するデータ（タスク・記録・メモ・提案）の植物。なければ uuid.Nil
}
