package cropidentity

import (
	"context"

	"github.com/Will413028/hawkr/ai-service/internal/observability"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

// WithTransientRetry 讓品項解析在失敗時當場重試一次。
//
// 為什麼「有錯就重試」是安全的判準：Resolve 只有一條錯誤路徑，就是 taxonomyCandidates 的
// 分類樹查詢（見 resolver_v2.go）。「查無此品項」「需要消歧」都是**成功回傳的
// 結果**，不是錯誤。所以能走到這裡的一律是基礎設施層的抖動，重試有意義；不會發生
// 「對一個確定的答案重試一次再拿到同樣答案」那種純粹浪費買家時間的情況。
//
// ⛔ 只重試一次，不做退避迴圈。買家正在等這一輪的回覆，多一次往返已經是延遲預算
// 的上限；真的連兩次都失敗就交給跨輪重試（crop_gate.go 的 needsResolution），
// 那一層不佔用買家的等待時間。
//
// ⛔ 呼叫端的 context 已經結束就不重試：那不是依賴抖動，是這次請求沒有時間了，
// 重試只會讓逾時更晚被發現。
func WithTransientRetry(inner Resolver) Resolver {
	if inner == nil {
		return nil
	}
	return &retryingResolver{inner: inner}
}

type retryingResolver struct{ inner Resolver }

func (r *retryingResolver) Resolve(ctx context.Context, itemText string) (DisambiguationResult, error) {
	got, err := r.inner.Resolve(ctx, itemText)
	if err == nil {
		return got, nil
	}
	if ctx.Err() != nil {
		return DisambiguationResult{}, err
	}

	pkglogger.Warn(ctx, "procurement: crop resolution failed, retrying once", zap.Error(err))
	observability.RecordProcurementCropResolverV2(ctx, "retry")

	got, retryErr := r.inner.Resolve(ctx, itemText)
	if retryErr != nil {
		return DisambiguationResult{}, retryErr
	}
	return got, nil
}
