package kyc

import (
	"context"
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// BindingData 是 payuni.KYCSource：回傳核准的申請裡 bind_merchant 要送的資料（身分證號在這裡解密，只存在於 job
// 執行期間的記憶體）。申請不是已核准的就回錯誤。
func (s *Service) BindingData(ctx context.Context, applicationID uuid.UUID) (payuni.BindingKYC, error) {
	row, err := store.New(s.d.DB.Conn(ctx)).KYCApplicationByID(ctx, applicationID)
	if err != nil {
		return payuni.BindingKYC{}, fmt.Errorf("seller：讀取 KYC 申請 %s：%w", applicationID, err)
	}
	if row.Status != string(model.KYCStatusApproved) {
		return payuni.BindingKYC{}, fmt.Errorf("seller：KYC 申請 %s 的狀態是 %s，不能綁定", applicationID, row.Status)
	}
	column, enc := colIDCardNo, row.IDCardNoEnc
	if row.SellerType == string(model.KYCCompany) {
		column, enc = colRepIDCardNo, row.RepIDCardNoEnc
	}
	idNumber, err := s.p.open(row.ApplicationID, column, enc)
	if err != nil {
		return payuni.BindingKYC{}, err
	}
	return payuni.BindingKYC{
		SellerType: model.KYCSellerType(row.SellerType), RealName: model.Deref(row.RealName), EnglishName: model.Deref(row.EnglishName),
		IDNumber: model.Deref(idNumber), ProductType: model.Deref(row.ProductType),
		WebsiteURL: model.Deref(row.WebsiteUrl), CompanyName: model.Deref(row.CompanyName), TaxID: model.Deref(row.TaxID),
	}, nil
}

// CompanyPhone 是 payuni.KYCSource：申請上的公司電話（沒有時空字串）。不要求申請是核准的狀態：同步（update_merchant）在申請回到草稿
// 修正的期間也會執行。
func (s *Service) CompanyPhone(ctx context.Context, applicationID uuid.UUID) (string, error) {
	row, err := store.New(s.d.DB.Conn(ctx)).KYCApplicationByID(ctx, applicationID)
	if err != nil {
		return "", fmt.Errorf("seller：讀取 KYC 申請 %s：%w", applicationID, err)
	}
	return model.Deref(row.CompanyPhone), nil
}
