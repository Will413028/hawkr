package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Confirmer 是 Confirm 的呼叫端依賴；*Module 與 Fake 都滿足。
type Confirmer interface {
	Confirm(ctx context.Context, uploader Uploader, token string, want Visibility) (ConfirmedAsset, error)
}

// ConfirmItem 是請求裡一個要確認的 staged token，Field 是它在輸入的欄位名（清單元素帶索引）。
type ConfirmItem struct{ Field, Token string }

// defaultRejectedMessage 是多個欄位不合格、訊息不同時整體的一句。
func defaultRejectedMessage(ctx context.Context) string {
	return text.MediaConfirmSomeUnusable(uitext.LocaleFrom(ctx)).Text
}

// ImageRequiredMessage 是必填的圖片位置沒有圖也沒有現有的圖時，欄位錯誤的訊息。
func ImageRequiredMessage(ctx context.Context) string {
	return text.MediaImageRequired(uitext.LocaleFrom(ctx)).Text
}

// ConfirmFields 在擁有者寫入的 transaction 內依序確認 items 的 staged token（ctx 必須帶 transaction），並一次回報所有不合格的：media 拒絕的
// token（不存在、不屬於這個上傳者、過期、處理中、被拒、可見性不是 want；Confirm 回 NotFound、FailedPrecondition 或 InvalidInput）
// 各是一筆欄位錯誤，client 一次就能標出所有要重傳的；media 原本的錯誤留在錯誤的文字（進 log）。want 是擁有者要的可見性（公開圖片或機敏檔），
// 由 media 在寫認領之前檢查，不符的 token 不會被認領。
// 其他錯誤（內部錯誤、不可用）不是使用者能改的：立刻停下原樣回傳。有不合格的 item 時呼叫方回傳這個錯誤，整個 transaction 回滾，
// 已確認的資產一併回到暫存。成功時回傳的資產與 items 一一對應。
func ConfirmFields(ctx context.Context, m Confirmer, uploader Uploader, items []ConfirmItem, want Visibility) ([]ConfirmedAsset, error) {
	out := make([]ConfirmedAsset, 0, len(items))
	var violations []apperr.FieldViolation
	var causes []error
	elsewhere := false
	for _, it := range items {
		asset, err := m.Confirm(ctx, uploader, it.Token, want)
		if err != nil {
			if kind := apperr.KindOf(err); kind == apperr.KindNotFound || kind == apperr.KindFailedPrecondition || kind == apperr.KindInvalidInput {
				message := defaultRejectedMessage(ctx)
				var ae *apperr.Error
				if errors.As(err, &ae) && ae.Message != "" {
					message = ae.Message
				}
				elsewhere = elsewhere || errors.Is(err, ErrUploadAlreadyConfirmed)
				violations = append(violations, apperr.FieldViolation{Field: it.Field, Message: message})
				causes = append(causes, fmt.Errorf("確認 %s：%w", it.Field, err))
				continue
			}
			return nil, err
		}
		out = append(out, asset)
	}
	if len(violations) == 0 {
		return out, nil
	}
	message := violations[0].Message
	for _, v := range violations[1:] {
		if v.Message != message {
			message = defaultRejectedMessage(ctx)
			break
		}
	}
	out2 := apperr.WithCause(apperr.InvalidInputFields(violations, "%s", message), errors.Join(causes...))
	if elsewhere {
		out2 = model.MarkConfirmedElsewhere(out2)
	}
	return nil, out2
}

// ConfirmField 確認單一欄位的 staged token：不合格時回指向 field 的欄位錯誤，語意見 ConfirmFields。
func ConfirmField(ctx context.Context, m Confirmer, uploader Uploader, field, token string, want Visibility) (ConfirmedAsset, error) {
	assets, err := ConfirmFields(ctx, m, uploader, []ConfirmItem{{Field: field, Token: token}}, want)
	if err != nil {
		return ConfirmedAsset{}, err
	}
	return assets[0], nil
}
