package model

import (
	"context"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// CheckEvidenceTokens 檢查佐證的 staged token：最多 MaxEvidence 張，不可為空白。
func CheckEvidenceTokens(tokens []string) error {
	if len(tokens) > MaxEvidence {
		return apperr.InvalidInputOn([]string{"EvidenceTokens"}, "佐證最多 %d 張", MaxEvidence)
	}
	for i, token := range tokens {
		if strings.TrimSpace(token) == "" {
			return apperr.InvalidInputOn([]string{apperr.IndexedField("EvidenceTokens", i)}, "佐證不可為空白")
		}
	}
	return nil
}

// ConfirmEvidence 在寫入佐證的 transaction 內確認佐證的 staged token（ctx 必須帶 transaction）。佐證是治理的證據，只 Confirm、從不
// MarkDeleted（沒有刪除路徑，也沒有保存期限政策）。不合格的一次全部回報：media 拒絕的 token（無效、過期或不屬於這個上傳者）與不是機敏檔的
// 佐證（佐證必須是機敏檔，只經後台的擁有者端點讀、每次讀取都記錄）各指到那一張（EvidenceTokens[i]）。
func ConfirmEvidence(ctx context.Context, m Media, uploader media.Uploader, tokens []string) ([]Evidence, error) {
	items := make([]ConfirmItem, len(tokens))
	for i, token := range tokens {
		items[i] = ConfirmItem{Field: apperr.IndexedField("EvidenceTokens", i), Token: strings.TrimSpace(token)}
	}
	assets, err := ConfirmAll(ctx, m, uploader, items, media.VisibilitySecure)
	if err != nil {
		return nil, err
	}
	evidence := make([]Evidence, 0, len(assets))
	for _, a := range assets {
		evidence = append(evidence, Evidence{AssetID: a.ID, ObjectKey: a.ObjectKey})
	}
	return evidence, nil
}
