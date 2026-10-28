package media

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/media"
)

// AssetInfo 代表 media-service 轉正後的資產資訊。
type AssetInfo struct {
	AssetId   string
	ObjectKey string
}

const (
	// finalizeTimeout 是轉正呼叫的時間預算。
	finalizeTimeout = 10 * time.Second
	// linkTimeout 是「單次」建立引用的時間預算——與 finalize 分開計，避免 finalize 偏慢時
	// 把 link 擠進 deadline 而靜默失敗。
	linkTimeout = 5 * time.Second
	// linkMaxAttempts 是建立引用的嘗試次數（含首次）。
	linkMaxAttempts = 3
	// linkRetryBackoff 是重試間隔的基數，第 n 次重試等待 n × 此值。
	linkRetryBackoff = 200 * time.Millisecond
	// releaseTimeout 是釋放引用的時間預算。原本這支完全沒有 timeout、直接用呼叫端的
	// ctx——對同步呼叫端只是慢，但對 converge worker 是正確性問題：一次卡死的呼叫會讓
	// 租約在它還在跑的時候過期，於是出現「兩個持有者同時處理同一列」，其中一個帶著
	// 過期的 desired。
	releaseTimeout = 5 * time.Second
)

// FinalizeAndLink 將 staged token 轉換為正式資產，並建立 asset_reference（防止 GC 清除）。
// 實作放在 pkg 層，各服務的 media client 直接委派到此函式。
//
// link 失敗會回報給 caller，不吞掉。理由：所有呼叫端（ai-service 風格縮圖、
// content 模組 banner、review-service 評論圖、im-service 聊天圖）都在寫入自己的
// 表之前呼叫本函式，且收到 error 就中止，因此回 error 的結果是「沒有任何人引用這個
// objectKey」——正好落在 media GC 對孤兒資產的處理範圍（09 §8.4）。反之若在此吞掉錯誤，caller 會把 objectKey 寫進 DB，形成
// 「有人在用但沒有 asset_reference」，7 天後 GC 刪掉的就是正在顯示的圖，且事後無跡可循。
//
// LinkImages 以 ON CONFLICT DO NOTHING 實作、設計書明定冪等（09 §3.2.4），故重試安全。
func FinalizeAndLink(ctx context.Context, client pb.MediaServiceClient, stagedToken, refType, refID string) (AssetInfo, error) {
	finalizeCtx, cancel := context.WithTimeout(ctx, finalizeTimeout)
	defer cancel()

	resp, err := client.FinalizeImages(finalizeCtx, &pb.FinalizeImagesRequest{
		StagedTokens: []string{stagedToken},
		RefType:      refType,
	})
	if err != nil {
		return AssetInfo{}, fmt.Errorf("finalize images: %w", err)
	}
	if resp.GetCode() != 0 {
		return AssetInfo{}, fmt.Errorf("finalize images failed (code=%d): %s", resp.GetCode(), resp.GetMessage())
	}
	if len(resp.GetAssets()) == 0 {
		return AssetInfo{}, fmt.Errorf("finalize returned no assets")
	}

	a := resp.GetAssets()[0]
	info := AssetInfo{
		AssetId:   a.GetAssetId(),
		ObjectKey: a.GetObjectKey(),
	}

	if linkErr := LinkImages(ctx, client, refType, refID, []string{info.AssetId}); linkErr != nil {
		return AssetInfo{}, fmt.Errorf("link images (asset_id=%s, ref=%s/%s): %w",
			info.AssetId, refType, refID, linkErr)
	}

	return info, nil
}

// LinkImages 建立 asset_reference，失敗時重試。LinkImages 冪等（09 §3.2.4），
// 重複呼叫不會產生重複列，故重試安全。回傳最後一次的錯誤。
//
// ⚠️ 呼叫端必須把本函式的錯誤當**致命**處理，且在寫入業務列**之前**呼叫。
// 順序契約（媒體生命週期不變式，見 08 號設計書 §4.4）：
//
//	① LinkImages（致命）→ ② 業務列寫入（致命）→ ③ ReleaseReferences(keepAssetIDs=[新資產])
//
// 任一步失敗都只會殘留「有引用、沒人用」的資產（可由 GC 依 7 天孤兒規則回收），
// 絕不會出現「業務列指著、卻沒有引用」的資產——那種資產會被 GC 連同 R2 物件實體刪除，
// 事後無跡可循（2026-08 dev 上 16 個賣家 logo 即因此消失）。
//
// 這個 ①②③ 順序契約對**六個媒體擁有者一體適用**（貼文、ai-service 縮圖、content 模組
// banner、review-service 評論圖、im-service 聊天圖，見 56 號設計書 §7.7.3）。
//
// 貼文曾經是唯一的例外——③ 一度整個從寫入路徑拿掉，改由一支 level-triggered reconciler
// 每輪重放全部已納管列來收斂。理由是「貼文一旦被刪除，呼叫端就讀不到那個業務列，
// best-effort 重試永遠短路」，但那條理由只對**靠重讀業務列來重試**的舊 divergence worker
// 成立：一支同步的 ③ 手上已經有 refID 與 desired，不需要回頭讀那一列；而軟刪列本來就
// **刻意保留引用不釋放**（見 cms_post 的 COMMENT ON TABLE），刪除路徑根本不需要 ③。
// 例外因此撤回，reconciler 連同它的獨立 Deployment 一併移除，貼文回到與其餘五個擁有者
// 相同的形狀。
func LinkImages(ctx context.Context, client pb.MediaServiceClient, refType, refID string, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}

	var lastErr error

	for attempt := 1; attempt <= linkMaxAttempts; attempt++ {
		linkCtx, cancel := context.WithTimeout(ctx, linkTimeout)
		resp, err := client.LinkImages(linkCtx, &pb.LinkImagesRequest{
			RefType:  refType,
			RefId:    refID,
			AssetIds: assetIDs,
		})
		cancel()

		switch {
		case err != nil:
			lastErr = err
		case resp.GetCode() != 0:
			lastErr = fmt.Errorf("code=%d: %s", resp.GetCode(), resp.GetMessage())
		default:
			return nil
		}

		if attempt == linkMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (context done after %d attempts)", lastErr, attempt)
		case <-time.After(time.Duration(attempt) * linkRetryBackoff):
		}
	}

	return lastErr
}

// ReleaseReferences 移除 (refType, refIDs) 的 asset_reference，keepAssetIDs 名單內的資產除外。
//
// 兩種語意：
//   - keepAssetIDs 為 nil/空：釋放全部（實體或帳號刪除）。
//   - keepAssetIDs 非空：換圖收斂，media-service 在**單一交易**內刪舊留新，
//     不存在「新資產一度沒有引用」的窗口。
//
// 本呼叫是 best-effort：它只負責回收舊資產，失敗的後果是舊資產永遠不被 GC 回收（洩漏），
// 而洩漏可以事後清、實體刪除不能。因此它必須排在業務列寫入**之後**——順序契約見 LinkImages。
//
// 稽核「誰會帶 POST 進來」時，答案是兩個呼叫端：
//
//   - **寫入路徑**：`PostUseCase.UpdatePost` 在 commit 之後、且這次更新真的動到
//     `image_asset_id` 的宣告時送出（換圖 keep=[新資產]，清圖 keep=nil）。這是常態路徑。
//   - **`seller.deleted` handler**：整店退場時分批送出 keepAssetIDs 為 nil 的釋放全部
//     （`services/social-service/internal/worker/seller_deleted.go`）。軟刪列的引用刻意
//     保留不釋放（見 cms_post 的 COMMENT ON TABLE），所以「賣家整店退場、圖片不用留」
//     這條語意只能由這個 handler 自己負責。
//
// 不再有任何背景收斂元件：in-table divergence worker 於 2026-09-05 刪除，取代它的
// level-triggered reconciler 於 2026-09-06 隨 ③ 補回寫入路徑一併移除。③ 失敗的殘留
// 是洩漏（舊資產不被回收），走維運稽核處理，見 scripts/audit_media_reference_precheck.py。
//
// 上面那段 best-effort／時序描述對六個媒體擁有者一體適用。
func ReleaseReferences(ctx context.Context, client pb.MediaServiceClient, refType string, refIDs []string, keepAssetIDs []string) error {
	if len(refIDs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, releaseTimeout)
	defer cancel()
	resp, err := client.ReleaseReferences(ctx, &pb.ReleaseReferencesRequest{
		RefType:      refType,
		RefIds:       refIDs,
		KeepAssetIds: keepAssetIDs,
	})
	if err != nil {
		return fmt.Errorf("release references: %w", err)
	}
	if resp.GetCode() != 0 {
		return fmt.Errorf("release references failed (code=%d): %s", resp.GetCode(), resp.GetMessage())
	}
	return nil
}
