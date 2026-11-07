package ws

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// アプリに送るメッセージ（設計の RealtimeMessage）。

// changedMessage は、世帯のデータが変わったことを知らせる。項目は、変更イベントの通知の中身（postgres の changePayload）とそろえる。
type changedMessage struct {
	Type        string     `json:"type"`
	HouseholdID uuid.UUID  `json:"householdId"`
	Version     int64      `json:"version"`
	Entity      string     `json:"entity"`
	EntityID    uuid.UUID  `json:"entityId"`
	Op          string     `json:"op"`
	ActorUserID *uuid.UUID `json:"actorUserId,omitempty"`
	PlantID     *uuid.UUID `json:"plantId,omitempty"`
}

func toChangedMessage(e domain.ChangeEvent) changedMessage {
	m := changedMessage{Type: "changed", HouseholdID: e.HouseholdID, Version: e.Version, Entity: string(e.Entity), EntityID: e.EntityID, Op: string(e.Op)}
	if e.ActorUserID != uuid.Nil {
		m.ActorUserID = &e.ActorUserID
	}
	if e.PlantID != uuid.Nil {
		m.PlantID = &e.PlantID
	}
	return m
}

// subscribedMessage は、購読できた世帯を知らせる。アプリは、これを受け取ってからデータを取り直す（取り直しと購読の間の変更を逃さないため）。
type subscribedMessage struct {
	Type         string      `json:"type"`
	HouseholdIDs []uuid.UUID `json:"householdIds"`
}

// householdMessage は、世帯だけを持つメッセージである（membership_revoked）。
type householdMessage struct {
	Type        string    `json:"type"`
	HouseholdID uuid.UUID `json:"householdId"`
}

type roleChangedMessage struct {
	Type        string      `json:"type"`
	HouseholdID uuid.UUID   `json:"householdId"`
	Role        domain.Role `json:"role"`
}

func marshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err) // 固定の型だけを渡すので、失敗しない
	}
	return data
}
