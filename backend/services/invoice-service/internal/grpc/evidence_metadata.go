package grpc

import (
	"context"

	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"go.uber.org/zap"
)


// EvidenceMetadataResolver 是佐證中介資料的出向面（實作在 client/media）。
type EvidenceMetadataResolver interface {
	BatchGetEvidenceMetadata(ctx context.Context, objectKeys []string, taxProfileID int64) (map[string]media.AssetMetadata, error)
}

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
