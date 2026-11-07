package job

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/app"
)

// PlantLister は、助言の対象の植物を返す（実装は postgres.PlantRepository）。
type PlantLister interface {
	// ActiveIDs は、すべての世帯の、アーカイブしていない植物の ID を返す。
	ActiveIDs(ctx context.Context) ([]uuid.UUID, error)
}

// DailyAdvice は、毎日の評価をきっかけ（advisor.TriggerDaily）とする助言プロバイダを、植物ごとに呼ぶジョブである（要件19-5）。
// MVP では対象のプロバイダがないので、何もせずに終わる。フェーズ2で、天気と連動するプロバイダを登録する。
type DailyAdvice struct {
	registry *advisor.Registry
	plants   PlantLister
	advice   app.AdviceEvaluator
}

func NewDailyAdvice(registry *advisor.Registry, plants PlantLister, advice app.AdviceEvaluator) *DailyAdvice {
	return &DailyAdvice{registry: registry, plants: plants, advice: advice}
}

func (d *DailyAdvice) Name() string { return "daily-advice" }

// Run は、植物ごとに、毎日のきっかけで評価する。提案の保存・重複の防止は SuggestionService が行うので、
// 同じ日に二重に実行しても、同じ提案は増えない（docs/decisions/0027）。処理件数は、評価した植物の数である。
func (d *DailyAdvice) Run(ctx context.Context) (Result, error) {
	var res Result
	if len(d.registry.For(advisor.TriggerDaily)) == 0 {
		return res, nil // 呼ぶプロバイダがなければ、植物を読むだけ無駄になる
	}
	ids, err := d.plants.ActiveIDs(ctx)
	if err != nil {
		return res, fmt.Errorf("list active plants: %w", err)
	}
	for _, id := range ids {
		// Evaluate はエラーを返さない（プロバイダの失敗はログに残し、ほかの植物の評価を続ける）
		d.advice.Evaluate(ctx, app.AdviceEvent{Trigger: advisor.TriggerDaily, PlantID: id})
		res.Processed++
	}
	return res, nil
}
