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


type uploaded struct {
	field string
	token string
	asset media.ConfirmedAsset
}

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

type newToken struct{ field, token string }

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

func markDeleted(ctx context.Context, m model.Media, assets []uuid.UUID) error {
	if len(assets) == 0 {
		return nil
	}
	return m.MarkDeleted(ctx, assets)
}
