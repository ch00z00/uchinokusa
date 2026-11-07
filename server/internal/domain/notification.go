package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// NoticeKind は、通知のアウトボックスの種類である。通知の data の type にも使い、アプリはこれを見て開く画面を決める。
type NoticeKind string

const (
	NoticeCompletion   NoticeKind = "completion"    // 植物の詳細（plantId があるとき）、なければ今日やること一覧
	NoticeMemberJoined NoticeKind = "member_joined" // 世帯のメンバー一覧
	NoticeReminder     NoticeKind = "reminder"      // 今日やること一覧（リマインダー。アウトボックスには入らない）
)

// MaxNoticeAttempts は、FCM に送れなかった通知を送り直す回数の上限である。超えたら failed にして諦める。
const MaxNoticeAttempts = 3

// NoticeGroup は、1件の通知にまとめる、アウトボックスの行のまとまり（世帯・種類・操作した人）である。
type NoticeGroup struct {
	HouseholdID uuid.UUID
	Kind        NoticeKind
	ActorUserID uuid.UUID // 退会して消えたユーザーなら uuid.Nil
}

// NoticeBatch は、まとめた通知を送るのに必要な情報である。
type NoticeBatch struct {
	NoticeGroup
	HouseholdName string
	ActorName     string
	Items         []CompletionItem // 完了通知のときだけ。完了した順
	Recipients    []uuid.UUID
}

// DispatchOutcome は、1つのまとまりを送った結果である。
type DispatchOutcome string

const (
	DispatchSent    DispatchOutcome = "sent"    // 送った（宛先がいなかった場合を含む）
	DispatchFailed  DispatchOutcome = "failed"  // FCM に送れなかった。次の実行で送り直す
	DispatchSkipped DispatchOutcome = "skipped" // 送るものがなかった（取り消し・削除された記録、ほかで処理中）
)

// CompletionItem は、完了通知にまとめる1件の完了である。
type CompletionItem struct {
	PlantID    uuid.UUID
	PlantName  string
	Kind       CareKind
	CustomName string // 種類が「その他」のときの作業名
}

// careLabel は、内訳に使う作業の名前である。「その他」は作業名を使う。
func careLabel(kind CareKind, customName string) string {
	switch kind {
	case CareWatering:
		return "水やり"
	case CareFertilizing:
		return "肥料"
	case CareRepotting:
		return "植え替え"
	case CarePruning:
		return "剪定"
	}
	return customName
}

// breakdown は、作業の名前ごとの件数を、出てきた順に「水やり2件、肥料1件」の形にする。
func breakdown(labels []string) string {
	var order []string
	counts := map[string]int{}
	for _, l := range labels {
		if counts[l] == 0 {
			order = append(order, l)
		}
		counts[l]++
	}
	parts := make([]string, 0, len(order))
	for _, l := range order {
		parts = append(parts, fmt.Sprintf("%s%d件", l, counts[l]))
	}
	return strings.Join(parts, "、")
}

// sentence は、1件だけのときの文の述語（「〇〇さんが」に続く部分）である。
func (i CompletionItem) sentence() string {
	switch i.Kind {
	case CareWatering:
		return i.PlantName + "に水やりしました"
	case CareFertilizing:
		return i.PlantName + "に肥料をあげました"
	case CareRepotting:
		return i.PlantName + "を植え替えました"
	case CarePruning:
		return i.PlantName + "を剪定しました"
	}
	return i.PlantName + "の" + i.CustomName + "をしました"
}

// CompletionNotice は、1人のメンバーの完了をまとめた通知を作る（要件16-1・16-4・16-5）。items は1件以上で、完了した順に渡す。
// 複数の世帯に所属するメンバーがどの世帯の通知かわかるよう、タイトルを世帯名にする。
// すべて同じ植物の完了なら、タップでその植物を開けるよう plantId を入れる。
func CompletionNotice(householdID uuid.UUID, householdName, actorName string, items []CompletionItem) PushMessage {
	data := map[string]string{"type": string(NoticeCompletion), "householdId": householdID.String()}
	if samePlant(items) {
		data["plantId"] = items[0].PlantID.String()
	}
	if len(items) == 1 {
		return PushMessage{Title: householdName, Body: actorName + "さんが" + items[0].sentence(), Data: data}
	}

	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, careLabel(it.Kind, it.CustomName))
	}
	body := fmt.Sprintf("%sさんが%d件のお手入れをしました（%s）", actorName, len(items), breakdown(labels))
	return PushMessage{Title: householdName, Body: body, Data: data}
}

func samePlant(items []CompletionItem) bool {
	for _, it := range items[1:] {
		if it.PlantID != items[0].PlantID {
			return false
		}
	}
	return true
}

// MemberJoinedNotice は、新しいメンバーの参加をオーナーに知らせる通知を作る（要件3-8）。
func MemberJoinedNotice(householdID uuid.UUID, householdName, memberName string) PushMessage {
	return PushMessage{
		Title: householdName,
		Body:  memberName + "さんが世帯に参加しました",
		Data:  map[string]string{"type": string(NoticeMemberJoined), "householdId": householdID.String()},
	}
}
