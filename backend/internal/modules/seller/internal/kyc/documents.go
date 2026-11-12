package kyc

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// 公司補充文件的 media 引用以 ref_id = application_id 登記（CERTIFICATION，7111）。轉正在 transaction 外、不建立引用；寫入業務列的
// transaction 內把被換掉的文件記成退場並排入同步（mediaref），申請取消與個資清理把全部文件記成退場；引用只由同步 job 登記與解除。

// uploaded 是已轉正的圖片，以它的 staged token 對應回請求。
type uploaded struct {
	field string
	token string
	asset media.Asset
}

// documentField 是公司文件在草稿輸入的欄位：帶它在清單的索引（單一元素的問題指到那一張，整個清單的問題如張數上限指 "CompanyDocuments"）。
func documentField(i int) string { return apperr.IndexedField("CompanyDocuments", i) }

func validateRef(ref model.KYCDocumentRef, field string) error {
	switch {
	case ref.AssetID != "" && ref.StagedToken != "":
		return apperr.InvalidInputOn([]string{field}, "證件只能指定現有的圖片或新上傳的圖片其中之一")
	case ref.AssetID == "" && ref.StagedToken == "":
		return apperr.InvalidInputOn([]string{field}, "必須提供圖片")
	}
	return nil
}

// validateDocumentInput 在轉正之前（不打 media）檢查請求：每個指定恰好是現有的圖片或新的 staged token，同一個 staged token
// 在整個請求只能出現一次，錯誤指向後出現的那個欄位。
func validateDocumentInput(in model.KYCDraftInput) error {
	if in.CompanyDocuments == nil {
		return nil
	}
	if len(*in.CompanyDocuments) > model.MaxCompanyDocuments {
		return apperr.InvalidInputOn([]string{"CompanyDocuments"}, "公司文件最多 %d 張", model.MaxCompanyDocuments)
	}
	seen := map[string]bool{}
	for i, ref := range *in.CompanyDocuments {
		field := documentField(i)
		if err := validateRef(ref, field); err != nil {
			return err
		}
		if ref.StagedToken != "" {
			if seen[ref.StagedToken] {
				return apperr.InvalidInputOn([]string{field}, "同一張圖片不能放進兩個證件位置")
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

// finalizeUploads 把全部 staged token 轉正（轉正不建立引用，失敗的請求已轉正的資產由 media 的 GC 在建立滿 7 天後回收）。media
// 拒絕的 token（無效、過期或不屬於這個帳號）與不是機敏上傳的證件（證件必須是機敏上傳：media 在機敏通道加密落庫、只能由擁有者授權後
// 讀）各是那個證件位置的欄位錯誤，一次全部回報。
func (s *Service) finalizeUploads(ctx context.Context, in model.KYCDraftInput) ([]uploaded, error) {
	tokens := newTokens(in)
	items := make([]media.Item, len(tokens))
	for i, t := range tokens {
		items[i] = media.Item{Field: media.Field{Name: t.field}, Token: t.token}
	}
	assets, err := media.FinalizeFields(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, media.RefTypeCertification, items, func(_ int, a media.Asset) string {
		if !a.Secure {
			return "證件必須以機敏上傳，請重新上傳"
		}
		return ""
	})
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
// 清單（順序就是位置，上限 model.MaxCompanyDocuments）；沒有出現時不動。現有的圖片必須是這份申請目前的文件；新轉正的圖片不能是已退場的。
// 改成個人賣家時文件一併清掉。回傳整組新文件，以及不再被任何位置指著的 asset（退場，位置之間移動的文件不算）。
func (s *Service) mergeDocuments(ctx context.Context, q *store.Queries, row store.SellerKycApplication, in model.KYCDraftInput, ups []uploaded) (docs []store.SellerKycDocument, retired []string, err error) {
	ref := mediaref.CertificationRef(row.ApplicationID)
	for _, up := range ups {
		if err := mediaref.Reject(ctx, q, ref, media.SyncCandidate{Field: up.field, AssetID: up.asset.ID}); err != nil {
			return nil, nil, err
		}
	}
	current, err := q.KYCDocuments(ctx, row.ApplicationID)
	if err != nil {
		return nil, nil, err
	}
	next := slices.Clone(current)
	if row.SellerType == string(model.KYCIndividual) {
		next = nil
	}
	fresh := make(map[string]media.Asset, len(ups))
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
			j := slices.IndexFunc(current, func(d store.SellerKycDocument) bool { return d.AssetID == ref.AssetID })
			if j < 0 || row.SellerType != string(model.KYCCompany) {
				return nil, nil, apperr.InvalidInputOn([]string{documentField(i)}, "指定的圖片不是這份申請目前的證件")
			}
			d := current[j]
			d.Position = int32(i) //nolint:gosec // 0 到 MaxCompanyDocuments
			next = append(next, d)
		}
	}
	// 保底：不同的 staged token 轉正成同一個 asset 時也一樣，同一張圖片在整份申請只能是一個文件。
	owner := map[string]bool{}
	for _, d := range next {
		if owner[d.AssetID] {
			return nil, nil, apperr.InvalidInputOn([]string{documentField(int(d.Position))}, "同一張圖片不能放進兩個證件位置")
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
	kept := make(map[string]bool, len(next))
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

// retireApplicationDocuments 把申請目前的全部證件記成退場並排入同步（申請取消是終態：取消的申請沒有目前的資產）。
func retireApplicationDocuments(ctx context.Context, q *store.Queries, enqueuer jobs.Enqueuer, now time.Time, applicationID uuid.UUID) error {
	docs, err := q.KYCDocuments(ctx, applicationID)
	if err != nil {
		return err
	}
	retired := make([]string, len(docs))
	for i, d := range docs {
		retired[i] = d.AssetID
	}
	return mediaref.Changed(ctx, q, enqueuer, now, mediaref.CertificationRef(applicationID), retired...)
}
