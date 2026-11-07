package domain_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

var (
	noticeHome     = uuid.MustParse("01920000-0000-7000-8000-0000000000e1")
	noticeMonstera = uuid.MustParse("01920000-0000-7000-8000-0000000000e2")
	noticePothos   = uuid.MustParse("01920000-0000-7000-8000-0000000000e3")
)

func TestCompletionNotice(t *testing.T) {
	t.Parallel()
	monstera := func(kind domain.CareKind, custom string) domain.CompletionItem {
		return domain.CompletionItem{PlantID: noticeMonstera, PlantName: "モンステラ", Kind: kind, CustomName: custom}
	}
	pothos := func(kind domain.CareKind) domain.CompletionItem {
		return domain.CompletionItem{PlantID: noticePothos, PlantName: "ポトス", Kind: kind}
	}
	data := func(plant *uuid.UUID) map[string]string {
		m := map[string]string{"type": "completion", "householdId": noticeHome.String()}
		if plant != nil {
			m["plantId"] = plant.String()
		}
		return m
	}

	tests := []struct {
		name  string
		items []domain.CompletionItem
		body  string
		plant *uuid.UUID // タップで開く植物（すべて同じ植物のときだけ）
	}{
		{"水やり", []domain.CompletionItem{monstera(domain.CareWatering, "")}, "さくらさんがモンステラに水やりしました", &noticeMonstera},
		{"肥料", []domain.CompletionItem{monstera(domain.CareFertilizing, "")}, "さくらさんがモンステラに肥料をあげました", &noticeMonstera},
		{"植え替え", []domain.CompletionItem{monstera(domain.CareRepotting, "")}, "さくらさんがモンステラを植え替えました", &noticeMonstera},
		{"剪定", []domain.CompletionItem{monstera(domain.CarePruning, "")}, "さくらさんがモンステラを剪定しました", &noticeMonstera},
		{"その他は作業名を使う", []domain.CompletionItem{monstera(domain.CareOther, "葉水")}, "さくらさんがモンステラの葉水をしました", &noticeMonstera},
		{
			"複数は件数と内訳にまとめる（内訳は出てきた順）",
			[]domain.CompletionItem{monstera(domain.CareWatering, ""), pothos(domain.CareFertilizing), pothos(domain.CareWatering)},
			"さくらさんが3件のお手入れをしました（水やり2件、肥料1件）", nil,
		},
		{
			"その他は作業名ごとに数える",
			[]domain.CompletionItem{
				monstera(domain.CareOther, "葉水"),
				{PlantID: noticePothos, PlantName: "ポトス", Kind: domain.CareOther, CustomName: "支柱立て"},
				{PlantID: noticePothos, PlantName: "ポトス", Kind: domain.CareOther, CustomName: "葉水"},
			},
			"さくらさんが3件のお手入れをしました（葉水2件、支柱立て1件）", nil,
		},
		{
			"同じ植物の複数は、その植物を開く",
			[]domain.CompletionItem{monstera(domain.CareWatering, ""), monstera(domain.CarePruning, "")},
			"さくらさんが2件のお手入れをしました（水やり1件、剪定1件）", &noticeMonstera,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.CompletionNotice(noticeHome, "自宅", "さくら", tt.items)
			want := domain.PushMessage{Title: "自宅", Body: tt.body, Data: data(tt.plant)}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("notice = %+v\nwant     %+v", got, want)
			}
		})
	}
}

func TestMemberJoinedNotice(t *testing.T) {
	t.Parallel()
	got := domain.MemberJoinedNotice(noticeHome, "自宅", "ボブ")
	want := domain.PushMessage{Title: "自宅", Body: "ボブさんが世帯に参加しました", Data: map[string]string{"type": "member_joined", "householdId": noticeHome.String()}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("notice = %+v", got)
	}
}
