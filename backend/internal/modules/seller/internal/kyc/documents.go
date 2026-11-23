package kyc

import (
	"context"
	"slices"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	sellertext "github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 公司補充文件以機敏資產存在 media：新的文件在寫入業務列的 transaction 內 Confirm，被換掉的文件在同一個 transaction 內 MarkDeleted，
// 申請取消與個資清理把全部文件 MarkDeleted（終態）。

// uploaded 是已轉正的圖片，以它的 staged token 對應回請求。
type uploaded struct {
	field string
	token string
	asset media.ConfirmedAsset
}

// documentField 是公司文件在草稿輸入的欄位：帶它在清單的索引（單一元素的問題指到那一張，整個清單的問題如張數上限指 "CompanyDocuments"）。
func documentField(i int) string { return apperr.IndexedField("CompanyDocuments", i) }

func validateRef(ctx context.Context, ref model.KYCDocumentRef, field string) error {
	switch {
	case ref.AssetID != "" && ref.StagedToken != "":
		return apperr.InvalidInputOn([]string{field}, "%s", sellertext.KycDocumentRefConflict(uitext.LocaleFrom(ctx)))
	case ref.AssetID == "" && ref.StagedToken == "":
		return apperr.InvalidInputOn([]string{field}, "%s", sellertext.KycDocumentRefMissing(uitext.LocaleFrom(ctx)))
	}
	return nil
}

// validateDocumentInput 在轉正之前（不打 media）檢查請求：每個指定恰好是現有的圖片或新的 staged token，同一個 staged token
// 在整個請求只能出現一次，錯誤指向後出現的那個欄位。
func validateDocumentInput(ctx context.Context, in model.KYCDraftInput) error {
	if in.CompanyDocuments == nil {
		return nil
	}
	if len(*in.CompanyDocuments) > model.MaxCompanyDocuments {
		return apperr.InvalidInputOn([]string{"CompanyDocuments"}, "公司文件最多 %d 張", model.MaxCompanyDocuments)
	}
	seen := map[string]bool{}
	for i, ref := range *in.CompanyDocuments {
		field := documentField(i)
		if err := validateRef(ctx, ref, field); err != nil {
			return err
		}
		if ref.StagedToken != "" {
			if seen[ref.StagedToken] {
				return apperr.InvalidInputOn([]string{field}, "%s", sellertext.KycDocumentDuplicate(uitext.LocaleFrom(ctx)))
			}
			seen[ref.StagedToken] = true
		}
	}
	return nil
}

// newToken 是請求裡一個要轉正的 staged token，field 是它在草稿輸入的欄位（公司文件帶請求清單的索引）。
type newToken struct{ field, token string }

// newTokens 依請求的順序列出要轉正的 staged token。
func newTokens(in model.KYCDraftInput) []newToken {
	var out []newToken
	if in.CompanyDocuments != nil {
		for i, ref := range *in.CompanyDocuments {
			if ref.StagedToken != "" {
				out = append(out, newToken{documentField(i), ref.StagedToken})
			}
		}
	}
	return out
}

// confirmUploads 在寫入的 transaction 內把全部 staged token 確認。media 拒絕的 token（無效、過期或不屬於這個帳號）與不是機敏上傳的證件
// （證件必須是機敏上傳：media 在機敏通道加密落庫、只能由擁有者授權後讀）各是那個證件位置的欄位錯誤，一次全部回報。
func (s *Service) confirmUploads(ctx context.Context, in model.KYCDraftInput) ([]uploaded, error) {
	tokens := newTokens(in)
	items := make([]model.ConfirmItem, len(tokens))
	for i, t := range tokens {
		items[i] = model.ConfirmItem{Field: t.field, Token: t.token}
	}
	assets, err := model.ConfirmAll(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, items, media.VisibilitySecure)
	if err != nil {
		return nil, err
	}
	out := make([]uploaded, len(assets))
	for i, a := range assets {
		out[i] = uploaded{field: tokens[i].field, token: tokens[i].token, asset: a}
	}
	return out, nil
}

// mergeDocuments 把請求的公司文件合併到目前的文件並整組寫回（沒有任何改變時不寫）。請求裡出現的清單是狀態，不是指令：整個換成請求的
// 清單（順序就是位置，上限 model.MaxCompanyDocuments）；沒有出現時不動。現有的圖片必須是這份申請目前的文件。
// 改成個人賣家時文件一併清掉。回傳整組新文件，以及不再被任何位置指著的 asset（呼叫方 MarkDeleted，位置之間移動的文件不算）。
func (s *Service) mergeDocuments(ctx context.Context, q *store.Queries, row store.SellerKycApplication, in model.KYCDraftInput, ups []uploaded) (docs []store.SellerKycDocument, retired []uuid.UUID, err error) {
	current, err := q.KYCDocuments(ctx, row.ApplicationID)
	if err != nil {
		return nil, nil, err
	}
	next := slices.Clone(current)
	if row.SellerType == string(model.KYCIndividual) {
		next = nil
	}
	fresh := make(map[string]media.ConfirmedAsset, len(ups))
	for _, up := range ups {
		fresh[up.token] = up.asset
	}
	if in.CompanyDocuments != nil {
		if len(*in.CompanyDocuments) > 0 && row.SellerType != string(model.KYCCompany) {
			return nil, nil, apperr.InvalidInputOn([]string{"CompanyDocuments"}, "只有企業賣家上傳公司文件")
		}
		next = nil
		for i, ref := range *in.CompanyDocuments {
			if ref.StagedToken != "" {
				// 機敏上傳已在 finalizeUploads 檢查過。
				asset := fresh[ref.StagedToken]
				next = append(next, store.SellerKycDocument{ApplicationID: row.ApplicationID, Position: int32(i), AssetID: asset.ID, ObjectKey: asset.ObjectKey}) //nolint:gosec // 0 到 MaxCompanyDocuments
				continue
			}
			refID, perr := uuid.Parse(ref.AssetID)
			if perr != nil {
				return nil, nil, apperr.InvalidInputOn([]string{documentField(i)}, "%s", sellertext.KycDocumentNotCurrent(uitext.LocaleFrom(ctx)))
			}
			j := slices.IndexFunc(current, func(d store.SellerKycDocument) bool { return d.AssetID == refID })
			if j < 0 || row.SellerType != string(model.KYCCompany) {
				return nil, nil, apperr.InvalidInputOn([]string{documentField(i)}, "%s", sellertext.KycDocumentNotCurrent(uitext.LocaleFrom(ctx)))
			}
			d := current[j]
			d.Position = int32(i) //nolint:gosec // 0 到 MaxCompanyDocuments
			next = append(next, d)
		}
	}
	// 保底：不同的 staged token 轉正成同一個 asset 時也一樣，同一張圖片在整份申請只能是一個文件。
	owner := map[uuid.UUID]bool{}
	for _, d := range next {
		if owner[d.AssetID] {
			return nil, nil, apperr.InvalidInputOn([]string{documentField(int(d.Position))}, "%s", sellertext.KycDocumentDuplicate(uitext.LocaleFrom(ctx)))
		}
		owner[d.AssetID] = true
	}
	slices.SortFunc(next, func(a, b store.SellerKycDocument) int { return int(a.Position) - int(b.Position) })
	if slices.EqualFunc(current, next, func(a, b store.SellerKycDocument) bool { return a == b }) {
		return current, nil, nil
	}
	if err := q.DeleteKYCDocuments(ctx, row.ApplicationID); err != nil {
		return nil, nil, err
	}
	kept := make(map[uuid.UUID]bool, len(next))
	for _, d := range next {
		kept[d.AssetID] = true
		if err := q.InsertKYCDocument(ctx, store.InsertKYCDocumentParams(d)); err != nil {
			return nil, nil, err
		}
	}
	for _, d := range current {
		if !kept[d.AssetID] {
			retired = append(retired, d.AssetID)
		}
	}
	return next, retired, nil
}

// retireApplicationDocuments 把申請目前的全部證件 MarkDeleted（申請取消是終態：取消的申請沒有目前的資產）；必須在寫入申請的 transaction 內。
func retireApplicationDocuments(ctx context.Context, q *store.Queries, m model.Media, applicationID uuid.UUID) error {
	docs, err := q.KYCDocuments(ctx, applicationID)
	if err != nil {
		return err
	}
	return markDeleted(ctx, m, assetIDs(docs))
}

func assetIDs(docs []store.SellerKycDocument) []uuid.UUID {
	out := make([]uuid.UUID, len(docs))
	for i, d := range docs {
		out[i] = d.AssetID
	}
	return out
}

// markDeleted 標記 assets 刪除；沒有資產時不呼叫 media。
func markDeleted(ctx context.Context, m model.Media, assets []uuid.UUID) error {
	if len(assets) == 0 {
		return nil
	}
	return m.MarkDeleted(ctx, assets)
}
