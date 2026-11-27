package kyc

import (
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

func dateText(d *time.Time) *string {
	if d == nil {
		return nil
	}
	s := model.FormatDate(*d)
	return &s
}

func toApplication(row store.SellerKycApplication, accountID int64) model.KYCApplication {
	return model.KYCApplication{
		ID: row.ApplicationID, StoreID: row.StoreID, AccountID: accountID, Status: model.KYCStatus(row.Status),
		Version: row.Version, SellerType: model.KYCSellerType(row.SellerType),
		ProductType: row.ProductType, WebsiteURL: row.WebsiteUrl,
		CompanyName: row.CompanyName, TaxID: row.TaxID,
		CompanyRegisterDate: dateText(row.CompanyRegisterDate), CompanyTownCode: row.CompanyTownCode,
		CompanyAddressDetail: row.CompanyAddressDetail, CompanyPhone: row.CompanyPhone,
		SubmittedAt: row.SubmittedAt, DecidedAt: row.DecidedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func toDocuments(rows []store.SellerKycDocument) []model.KYCDocument {
	out := make([]model.KYCDocument, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.KYCDocument{
			Position: int(row.Position), AssetID: row.AssetID.String(), ObjectKey: row.ObjectKey,
		})
	}
	return out
}

func toReview(row store.SellerKycReview, issues []model.KYCIssue) model.KYCReview {
	return model.KYCReview{
		ID: row.ReviewID, Action: model.ReviewAction(row.Action), ReviewerAdminID: row.ReviewerAdminID,
		Reason: row.Reason, Issues: issues, InternalNote: row.InternalNote, DecidedAt: row.DecidedAt,
	}
}

func lastRejection(reviews []store.SellerKycReview, issues map[uuid.UUID][]model.KYCIssue) *model.KYCRejection {
	r := pendingRejection(reviews)
	if r == nil {
		return nil
	}
	return &model.KYCRejection{Reason: *r.Reason, Issues: issues[r.ReviewID], DecidedAt: r.DecidedAt}
}

func insertParams(r store.SellerKycApplication, now time.Time) store.InsertKYCApplicationParams {
	return store.InsertKYCApplicationParams{
		ApplicationID: r.ApplicationID, StoreID: r.StoreID, SellerType: r.SellerType,
		ProductType: r.ProductType, WebsiteUrl: r.WebsiteUrl,
		CompanyName: r.CompanyName, TaxID: r.TaxID, CompanyRegisterDate: r.CompanyRegisterDate, CompanyTownCode: r.CompanyTownCode,
		CompanyAddressDetail: r.CompanyAddressDetail, CompanyPhone: r.CompanyPhone,
		Now: now,
	}
}

func updateParams(r store.SellerKycApplication, expectedVersion int64, now time.Time) store.UpdateKYCDraftParams {
	return store.UpdateKYCDraftParams{
		ApplicationID: r.ApplicationID, ExpectedVersion: expectedVersion, SellerType: r.SellerType,
		ProductType: r.ProductType, WebsiteUrl: r.WebsiteUrl,
		CompanyName: r.CompanyName, TaxID: r.TaxID, CompanyRegisterDate: r.CompanyRegisterDate, CompanyTownCode: r.CompanyTownCode,
		CompanyAddressDetail: r.CompanyAddressDetail, CompanyPhone: r.CompanyPhone,
		Now: now,
	}
}
