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

type imageUpload struct {
	kind      imageKind
	token     string
	sourceURL *string
	asset     media.ConfirmedAsset
}

func (s *Service) retireImages(ctx context.Context, row store.SellerProspect) error {
	retired := media.RetiredAssets(media.AssetIDs(logoKind.assetOf(row), coverKind.assetOf(row)), nil)
	if len(retired) == 0 {
		return nil
	}
	return s.d.Media.MarkDeleted(ctx, retired)
}

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
