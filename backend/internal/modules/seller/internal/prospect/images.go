package prospect

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// 品牌圖片的 media 引用以 ref_id = prospect_id 登記（SELLER_PROSPECT_LOGO 7122、SELLER_PROSPECT_COVER 7123，不與正式店家的
// 7103、7105 共用）。轉正在 transaction 外、不建立引用；寫入業務列的 transaction 內把被換掉的圖（以及封存、合併時清掉的圖）
// 記成退場並排入同步（mediaref），引用只由同步 job 登記與解除。

type imageKind struct {
	field   string
	refType string
	assetOf func(store.SellerProspect) *string
	upload  func(model.UpdateProspectInput) *model.ProspectImageUpload
	set     func(*store.UpdateProspectParams, imageUpload)
}

var (
	logoKind = imageKind{
		field: "Logo", refType: media.RefTypeSellerProspectLogo,
		assetOf: func(p store.SellerProspect) *string { return p.LogoAssetID },
		upload:  func(in model.UpdateProspectInput) *model.ProspectImageUpload { return in.Logo },
		set: func(p *store.UpdateProspectParams, u imageUpload) {
			p.SetLogo, p.LogoKey, p.LogoAssetID, p.LogoSourceUrl = true, &u.asset.ObjectKey, &u.asset.ID, u.sourceURL
		},
	}
	coverKind = imageKind{
		field: "Cover", refType: media.RefTypeSellerProspectCover,
		assetOf: func(p store.SellerProspect) *string { return p.CoverAssetID },
		upload:  func(in model.UpdateProspectInput) *model.ProspectImageUpload { return in.Cover },
		set: func(p *store.UpdateProspectParams, u imageUpload) {
			p.SetCover, p.CoverKey, p.CoverAssetID, p.CoverSourceUrl = true, &u.asset.ObjectKey, &u.asset.ID, u.sourceURL
		},
	}
)

// imageUpload 是一次換圖：轉正之後 asset 才有值。
type imageUpload struct {
	kind      imageKind
	token     string
	sourceURL *string
	asset     media.Asset
}

// retireImages 在封存或合併清掉業者的圖片欄位時，把目前的 logo 與封面記成退場並排入同步。封存之後的還原不會把圖帶回來，
// 所以被清掉的圖不會再被寫回。
func retireImages(ctx context.Context, q *store.Queries, enqueuer jobs.Enqueuer, now time.Time, row store.SellerProspect) error {
	for _, kind := range []imageKind{logoKind, coverKind} {
		var retired []string
		if asset := kind.assetOf(row); asset != nil {
			retired = append(retired, *asset)
		}
		if len(retired) == 0 {
			continue
		}
		if err := mediaref.Changed(ctx, q, enqueuer, now, mediaref.ProspectRef(kind.refType, row.ProspectID), retired...); err != nil {
			return err
		}
	}
	return nil
}

// finalizeUploads 轉正 Update 要換的每一張圖（沒有要換的回傳空）：先驗證每個上傳（token 不可空白、來源網址）、確認不在 transaction 內，
// 再做所有不需寫入的檢查（planUpdate），最後才轉正；被拒絕的請求不消耗 token。轉正不建立引用，所以中途失敗不需要補償。
func (s *Service) finalizeUploads(ctx context.Context, in model.UpdateProspectInput, contacts *[]model.ContactPoint, categories []string) ([]imageUpload, error) {
	var uploads []imageUpload
	for _, kind := range []imageKind{logoKind, coverKind} {
		up := kind.upload(in)
		if up == nil {
			continue
		}
		if strings.TrimSpace(up.StagedToken) == "" {
			return nil, apperr.InvalidInputOn([]string{kind.field + ".StagedToken"}, "必須提供圖片")
		}
		sourceURL, err := normalizeSourceURL(up.SourceURL, kind.field+".SourceURL")
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, imageUpload{kind: kind, token: up.StagedToken, sourceURL: sourceURL})
	}
	if len(uploads) == 0 {
		return nil, nil
	}
	if _, ok := db.TxFrom(ctx); ok {
		return nil, apperr.FailedPrecondition("換圖必須在 transaction 外開始")
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.ProspectByID(ctx, in.ProspectID)
	if err != nil {
		return nil, notFound(err)
	}
	if _, err := planUpdate(ctx, q, row, in, contacts, categories); err != nil {
		return nil, err
	}
	for i := range uploads {
		asset, err := media.FinalizeField(ctx, s.d.Media, media.Uploader{ID: in.AdminID, Admin: true}, media.Field{Name: uploads[i].kind.field + ".StagedToken"}, uploads[i].token, uploads[i].kind.refType)
		if err != nil {
			return nil, err
		}
		uploads[i].asset = asset
	}
	return uploads, nil
}

// normalizeSourceURL 驗證素材的來源頁面：只作稽核，必須是 https 且有主機，不超過 2048 字；沒有值是 nil。
func normalizeSourceURL(raw, field string) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(raw) > 2048 {
		return nil, apperr.InvalidInputOn([]string{field}, "素材來源網址必須是 https 網址")
	}
	return &raw, nil
}
