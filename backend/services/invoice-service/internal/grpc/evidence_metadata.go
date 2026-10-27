package grpc

import (
	"context"

	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 稅籍佐證的中介資料（#3031）
//
// 🔴 S05／A02 的佐證清單在此之前只有檔案路徑：file_asset_id／file_name／file_size／
// uploaded_at 四欄全服務零賦值。其中 **file_asset_id 是 Gateway 組下載路徑的唯一輸入**
// （`/api/v1/app/media/secure_download/{id}`）⇒ 症狀是「看得到檔名、點不下去」，
// 與 latest_job_id 缺失時的「看得到異常、點不進去」是同一個形狀。
//
// 🔴 **不在本服務存一份拷貝**：檔名／大小／上傳時間的 owner 是 media，
// 反正規化過來就是第二個真實來源，而它會在 media 那側改動時無聲漂移。
// ---------------------------------------------------------------------------

// EvidenceMetadataResolver 是佐證中介資料的出向面（實作在 client/media）。
type EvidenceMetadataResolver interface {
	BatchGetEvidenceMetadata(ctx context.Context, objectKeys []string, taxProfileID int64) (map[string]media.AssetMetadata, error)
}

// resolveEvidenceMetadata 批次取中介資料。
//
// 🔴 失敗降級成空 map、不中止查詢：稅籍資料本身完全健康，media 一次抖動不該把整個
// S02／A02 打不開——退回「只有檔名、沒有下載鍵」正是修這件事之前的畫面，不會更差。
//
// ⚠️ 降級要留痕，否則「下載鍵又不見了」在下一次發生時無從診斷。
func (s *InvoiceServer) resolveEvidenceMetadata(
	ctx context.Context, objectKeys []string, taxProfileID int64,
) map[string]media.AssetMetadata {
	keys := dedupeStrings(objectKeys)
	if s.evidenceMeta == nil || len(keys) == 0 || taxProfileID == 0 {
		return map[string]media.AssetMetadata{}
	}
	got, err := s.evidenceMeta.BatchGetEvidenceMetadata(ctx, keys, taxProfileID)
	if err != nil {
		if s.log != nil {
			s.log.Warn("invoice.query.evidence_metadata_unavailable",
				zap.Int("key_count", len(keys)),
				zap.String("degrade", "佐證清單退回只有檔案路徑，下載鍵組不出來"),
				zap.Error(err))
		}
		return map[string]media.AssetMetadata{}
	}
	return got
}
