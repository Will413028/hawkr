package prospect

import (
	"context"
	"net/url"
	"strings"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 品牌圖片由 media 擁有，業務列存 object key 與資產 ID：寫入業務列的 transaction 內 Confirm 新圖，並把被換掉的圖（以及封存、合併時清掉的圖）
// MarkDeleted。

type imageKind struct {
	field   string
	assetOf func(store.SellerProspect) *uuid.UUID
	upload  func(model.UpdateProspectInput) *model.ProspectImageUpload
	set     func(*store.UpdateProspectParams, imageUpload)
}

var (
	logoKind = imageKind{
		field:   "Logo",
		assetOf: func(p store.SellerProspect) *uuid.UUID { return p.LogoAssetID },
		upload:  func(in model.UpdateProspectInput) *model.ProspectImageUpload { return in.Logo },
		set: func(p *store.UpdateProspectParams, u imageUpload) {
			p.SetLogo, p.LogoKey, p.LogoAssetID, p.LogoSourceUrl = true, &u.asset.ObjectKey, &u.asset.ID, u.sourceURL
		},
	}
	coverKind = imageKind{
		field:   "Cover",
		assetOf: func(p store.SellerProspect) *uuid.UUID { return p.CoverAssetID },
		upload:  func(in model.UpdateProspectInput) *model.ProspectImageUpload { return in.Cover },
		set: func(p *store.UpdateProspectParams, u imageUpload) {
			p.SetCover, p.CoverKey, p.CoverAssetID, p.CoverSourceUrl = true, &u.asset.ObjectKey, &u.asset.ID, u.sourceURL
		},
	}
)

// imageUpload 是一次換圖：確認之後 asset 才有值。
type imageUpload struct {
	kind      imageKind
	token     string
	sourceURL *string
	asset     media.ConfirmedAsset
}

// retireImages 在封存或合併清掉業者的圖片欄位時，把目前的 logo 與封面 MarkDeleted（必須在寫入的 transaction 內）。封存之後的還原不會把圖帶回來，
// 所以被清掉的圖不會再被寫回。
func (s *Service) retireImages(ctx context.Context, row store.SellerProspect) error {
	retired := media.RetiredAssets(media.AssetIDs(logoKind.assetOf(row), coverKind.assetOf(row)), nil)
	if len(retired) == 0 {
		return nil
	}
	return s.d.Media.MarkDeleted(ctx, retired)
}

// parseUploads 驗證 Update 要換的每一張圖（沒有要換的回傳空）：token 不可空白、來源網址合法。不打 media。
func parseUploads(in model.UpdateProspectInput) ([]imageUpload, error) {
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
	return uploads, nil
}

// confirmUploads 在寫入的 transaction 內確認每一張新圖（必須是公開圖片），結果寫回 uploads。
func (s *Service) confirmUploads(ctx context.Context, adminID int64, uploads []imageUpload) error {
	items := make([]model.ConfirmItem, len(uploads))
	for i, u := range uploads {
		items[i] = model.ConfirmItem{Field: u.kind.field + ".StagedToken", Token: u.token}
	}
	assets, err := model.ConfirmAll(ctx, s.d.Media, media.Uploader{ID: adminID, Admin: true}, items, media.VisibilityPublic)
	if err != nil {
		return err
	}
	for i := range uploads {
		uploads[i].asset = assets[i]
	}
	return nil
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
