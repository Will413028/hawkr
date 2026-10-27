package grpc

import (
	"strconv"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

func toSellerInfoProto(p *dto.SellerProfile, mediaBaseURL string) *pb.SellerInfo {
	if p == nil {
		return nil
	}
	return &pb.SellerInfo{
		AccountId:                  p.AccountID,
		Name:                       p.StoreName,
		LogoKey:                    p.Logo,
		Description:                p.Description,
		ContactPhone:               p.ContactPhone,
		City:                       p.City,
		District:                   p.District,
		AddressDetail:              p.AddressDetail,
		PostalCode:                 p.PostalCode,
		Rating:                     p.Rating,
		CoverKey:                   p.CoverKey,
		Prize:                      p.Prize,
		OnboardingCompleted:        p.OnboardingCompleted,
		Nickname:                   p.Nickname,
		Email:                      p.Email,
		AccountType:                p.AccountType,
		Phone:                      p.Phone,
		SellerType:                 p.SellerType,
		Birthday:                   p.Birthday,
		Gender:                     p.Gender,
		OverallCertificationStatus: p.OverallCertificationStatus,
		SeoTitle:                   p.SeoTitle,
		SeoDescription:             p.SeoDescription,
		WebsiteUrl:                 p.WebsiteURL,
		IsProcessedFood:            p.IsProcessedFood,
		FoodBusinessRegNo:          p.FoodBusinessRegNo,
		StoreId:                    strconv.FormatInt(p.StoreID, 10),
		StoreCode:                  strconv.FormatInt(p.StoreCode, 10),
		LifecycleStatus:            p.Status,
		Version:                    strconv.FormatInt(p.Version, 10),
		CurrentCertificationId: func() string {
			if p.CurrentCertificationID != nil {
				return strconv.FormatInt(*p.CurrentCertificationID, 10)
			}
			return ""
		}(),
	}
}

func toCertificationApplicationProto(app *dto.CertificationApplication) *pb.CertificationApplicationData {
	if app == nil {
		return nil
	}
	data := &pb.CertificationApplicationData{
		Id:                   strconv.FormatInt(app.ID, 10),
		AccountId:            strconv.FormatInt(app.AccountID, 10),
		ApplicationNo:        app.ApplicationNo,
		Status:               app.Status,
		SellerType:           app.SellerType,
		CurrentStep:          int32(app.CurrentStep),
		TotalSteps:           int32(app.TotalSteps),
		StepDataJson:         app.StepDataJSON,
		CreatedAt:            formatTime(app.CreatedAt),
		UpdatedAt:            formatTime(app.UpdatedAt),
		RealName:             app.RealName,
		IdCardNo:             app.IDCardNo,
		BirthDate:            timeutil.FormatROCDate(app.BirthDate),
		IdCardFrontAssetId:   app.IDCardFrontAssetID,
		IdCardBackAssetId:    app.IDCardBackAssetID,
		BankBookAssetId:      app.BankBookAssetID,
		DocumentAssetIds:     app.DocumentAssetIDs,
		BankName:             app.BankName,
		BankCode:             app.BankCode,
		BankAccount:          app.BankAccount,
		CompanyName:          app.CompanyName,
		TaxId:                app.TaxID,
		CompanyCity:          app.CompanyCity,
		CompanyDistrict:      app.CompanyDistrict,
		CompanyAddressDetail: app.CompanyAddressDetail,
		CompanyType:          app.CompanyType,
		CompanyRegisterDate:  timeutil.FormatROCDate(app.CompanyRegisterDate),
		CompanyEmail:         app.CompanyEmail,
		CompanyPhone:         app.CompanyPhone,
		ProductType:          app.ProductType,
		FoodBusinessRegNo:    app.FoodBusinessRegNo,
		IdCardIssueDate:      timeutil.FormatROCDate(app.IDCardIssueDate),
		IdCardIssueType:      app.IDCardIssueType,
		IdCardIssueLocation:  app.IDCardIssueLocation,
		PayuniWebsiteUrl:     app.PayuniWebsiteURL,
	}
	if app.SubmittedAt != nil {
		data.SubmittedAt = formatTime(*app.SubmittedAt)
	}
	if app.RejectedAt != nil {
		data.RejectedAt = formatTime(*app.RejectedAt)
	}
	return data
}

func toPayuniStatusProto(profile *dto.SellerProfile, ps *dto.PayuniStatus) *pb.PayuniStatus {
	return &pb.PayuniStatus{
		AccountId:              strconv.FormatInt(profile.AccountID, 10),
		PayuniMerId:            ps.PayuniMerID,
		PayuniStatus:           ps.Status,
		MerStatus:              ps.MerStatus,
		MemberStatus:           ps.MemberStatus,
		CreditStatus:           ps.CreditStatus,
		PayuniMerStatus:        domain.PayuniMerStatus(profile.PayuniStatus).String(),
		ApprovalStatus:         profile.ApprovalStatus,
		Reason:                 ps.Reason,
		OverallStatus:          profile.OverallCertificationStatus,
		ApplepayStatus:         ps.ApplePayStatus,
		GooglepayStatus:        ps.GooglePayStatus,
		AtmStatus:              ps.ATMStatus,
		ErrorMessage:           ps.ErrorMessage,
		RegisteredAt:           unixOrZero(ps.RegisteredAt),
		ApprovedAt:             unixOrZero(ps.ApprovedAt),
		LastSyncedAt:           unixOrZero(ps.LastSyncedAt),
		Freshness:              ps.Freshness,
		AllowedActions:         ps.AllowedActions,
		MerchantProfileVersion: strconv.FormatInt(ps.MerchantProfileVersion, 10),
		ContactName:            ps.ContactName,
		ContactMobile:          ps.ContactMobile,
		ContactTel:             ps.ContactTel,
		MerEmail:               ps.MerEmail,
		MerAddr:                ps.MerAddr,
		MerAddrPost:            ps.MerAddrPost,
		MerTel:                 ps.MerTel,
		ErrorCode:              ps.ErrorCode,
		ErrorDisplayMessage:    ps.ErrorDisplayMessage,
		ErrorRemedy:            ps.ErrorRemedy,
		RetryStep:              ps.RetryStep,
	}
}

func unixOrZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}

func toSellerPayuniConfigProto(profile *dto.SellerProfile, ps *dto.PayuniStatus) *pb.SellerPayuniConfig {
	return &pb.SellerPayuniConfig{
		AccountId:     strconv.FormatInt(profile.AccountID, 10),
		PayuniMerId:   ps.PayuniMerID,
		IsEnabled:     ps.PaymentEnabled,
		CreditEnabled: ps.CreditStatus == "1",
		SellerName:    profile.StoreName,
	}
}

func toCertificationDraftProto(app *dto.CertificationApplication) *pb.CertificationDraft {
	if app == nil {
		return nil
	}
	return &pb.CertificationDraft{
		Id:          strconv.FormatInt(app.ID, 10),
		AccountId:   strconv.FormatInt(app.AccountID, 10),
		CurrentStep: int32(app.CurrentStep),
		TotalSteps:  int32(app.TotalSteps),
		Status:      app.Status,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return timeutil.FormatISO8601(t)
}

func toSellerShippingProfileProto(p *dto.SellerShippingProfile) *pb.SellerShippingProfile {
	return &pb.SellerShippingProfile{
		StoreId:       p.StoreID,
		SenderName:    p.SenderName,
		SenderPhone:   p.SenderPhone,
		City:          p.City,
		District:      p.District,
		AddressDetail: p.AddressDetail,
		ZipCode:       p.ZipCode,
		MerId:         p.MerID,
		ContactEmail:  p.ContactEmail,
	}
}

func toSellerMetaProto(m *dto.SellerMeta, mediaBaseURL string) *pb.SellerMeta {
	if m == nil {
		return nil
	}
	return &pb.SellerMeta{
		AccountId:         m.AccountID,
		SellerName:        m.SellerName,
		LogoKey:           m.LogoURL,
		CanAcceptOrders:   m.CanAcceptOrders,
		CanManageProducts: m.CanManageProducts,
		ZipCode:           m.ZipCode,
		City:              m.City,
		District:          m.District,
		PayuniMerId:       m.PayuniMerID,
		StoreId:           strconv.FormatInt(m.StoreID, 10),
		StoreCode:         strconv.FormatInt(m.StoreCode, 10),
	}
}
