package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Field 是轉正的 token 在模組輸入裡的欄位（Go 的欄位名，清單的元素帶 [n]，見 apperr.FieldViolation）。Message 是 media 拒絕
// 這個 token 時給使用者看的文字；空字串沿用 media 的文字。
type Field struct {
	Name    string
	Message string
}

// ErrStagedTokenRejected 標記 Media.Finalize 對「這個 staged token」的拒絕（無效、過期、不屬於上傳者、已轉正的物件不符）：呼叫方
// 重新上傳就能解決。Media 的實作用 errors.Is 能辨認的方式包住它，而且 Finalize 只在這個情況回 InvalidInput 一類的使用者錯誤：
// 呼叫方自己傳錯的參數（缺上傳者、無效的 ref_type、空白的 token）由 CheckFinalizeArgs 在送出前擋下，是未分類的程式錯誤。
var ErrStagedTokenRejected = model.ErrStagedTokenRejected

const defaultRejectedMessage = "圖片無法使用，請重新上傳"

// Item 是一個要轉正的 staged token 與它在模組輸入裡的欄位。
type Item struct {
	Field Field
	Token string
}

// FinalizeField 把一個 staged token 轉正（不建立引用）。media 拒絕這個 token（ErrStagedTokenRejected）時，換成指向 f 的欄位錯誤
// （InvalidInputOn），media 原本的錯誤留在錯誤的文字（進 log）；其他錯誤（內部錯誤、不可用）原樣回傳。
func FinalizeField(ctx context.Context, m Media, uploader Uploader, f Field, token, refType string) (Asset, error) {
	assets, err := FinalizeFields(ctx, m, uploader, refType, []Item{{Field: f, Token: token}}, nil)
	if err != nil {
		return Asset{}, err
	}
	return assets[0], nil
}

// FinalizeList 轉正清單欄位 field 的全部 token，第 i 個元素的問題指到 "<field>[i]"。語意見 FinalizeFields；check 回非空字串
// 表示第 i 個資產不合格，那就是該元素的訊息（欄位由這裡加上索引，呼叫方不必自己組路徑）。
func FinalizeList(ctx context.Context, m Media, uploader Uploader, field string, tokens []string, refType string, check func(i int, a Asset) string) ([]Asset, error) {
	items := make([]Item, len(tokens))
	for i, token := range tokens {
		items[i] = Item{Field: Field{Name: apperr.IndexedField(field, i)}, Token: token}
	}
	return FinalizeFields(ctx, m, uploader, refType, items, check)
}

// FinalizeFields 轉正 items 的全部 token（不建立引用），並一次回報所有不合格的：media 拒絕的 token（ErrStagedTokenRejected）與
// check 判定不合格的資產（check 非 nil 時，每轉正一個就以它在 items 的位置 i 檢查，回非空字串就是該欄位的訊息），全部收進同一個
// InvalidInput，每個不合格的 item 一筆欄位錯誤，client 一次就能標出所有要重傳的。整體的 Message 在每筆訊息相同時就是那句，否則
// 是通用的一句。media 原本的錯誤留在錯誤的文字（進 log）。
//
// 其他錯誤（內部錯誤、不可用）不是使用者能改的：立刻停下原樣回傳，後面的 token 不再轉正。已轉正的資產不建立引用，所以只要請求
// 失敗（有不合格的 item，或停在其他錯誤），這一批已轉正的資產就沒有引用，由 media 的 GC 在建立滿 7 天後回收，不需要補償
// （與業務列寫入前轉正的設計相同）。成功時回傳的資產與 items 一一對應。
func FinalizeFields(ctx context.Context, m Media, uploader Uploader, refType string, items []Item, check func(i int, a Asset) string) ([]Asset, error) {
	out := make([]Asset, 0, len(items))
	var violations []apperr.FieldViolation
	var causes []error
	for i, it := range items {
		asset, err := m.Finalize(ctx, uploader, it.Token, refType)
		switch {
		case err == nil:
			if check != nil {
				if msg := check(i, asset); msg != "" {
					violations = append(violations, apperr.FieldViolation{Field: it.Field.Name, Message: msg})
					continue
				}
			}
			out = append(out, asset)
		case errors.Is(err, ErrStagedTokenRejected):
			message := it.Field.Message
			if message == "" {
				var ae *apperr.Error
				if errors.As(err, &ae) {
					message = ae.Message
				}
			}
			if message == "" {
				message = defaultRejectedMessage
			}
			violations = append(violations, apperr.FieldViolation{Field: it.Field.Name, Message: message})
			causes = append(causes, fmt.Errorf("轉正 %s：%w", it.Field.Name, err))
		default:
			return nil, err
		}
	}
	if len(violations) == 0 {
		return out, nil
	}
	message := violations[0].Message
	for _, v := range violations[1:] {
		if v.Message != message {
			message = defaultRejectedMessage
			break
		}
	}
	public := apperr.InvalidInputFields(violations, "%s", message)
	return nil, apperr.WithCause(public, errors.Join(causes...))
}
