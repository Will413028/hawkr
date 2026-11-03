package kyc

import (
	"errors"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func text(s string) *string { return &s }

// 收款聯絡信箱（D23）是送出時的必填：其餘都齊全的個人申請只缺它時，送出的違規正好指向 payout_contact_email，補上之後通過。
func TestCompleteRequiresThePayoutContactEmail(t *testing.T) {
	row := store.SellerKycApplication{SellerType: string(model.KYCIndividual)}
	err := complete(row)
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "payout_contact_email" {
		t.Fatalf("缺信箱的送出檢查=%v", err)
	}
	if got := missingFields(row); len(got) != 1 || got[0] != "payout_contact_email" {
		t.Fatalf("missing_fields=%v", got)
	}
	row.PayoutContactEmail = text("seller@example.com")
	if err := complete(row); err != nil {
		t.Fatalf("齊全的申請：%v", err)
	}
}

// 送出只要求市集登記資料（企業的公司名稱與統編）與收款聯絡信箱；身分、銀行與證件由金流商收集，不在必填之內。
func TestRequirementsAreMarketplaceDataOnly(t *testing.T) {
	var fields []string
	for _, r := range requirements {
		fields = append(fields, r.field)
	}
	if want := []string{"business.company_name", "business.tax_id", "payout_contact_email"}; !slices.Equal(fields, want) {
		t.Fatalf("requirements=%v，預期 %v", fields, want)
	}
	company := store.SellerKycApplication{SellerType: string(model.KYCCompany)}
	if got, want := missingFields(company), []string{"business.company_name", "business.tax_id", "payout_contact_email"}; !slices.Equal(got, want) {
		t.Fatalf("企業缺：%v", got)
	}
	company.CompanyName, company.TaxID, company.PayoutContactEmail = text("範例小農有限公司"), text("12345678"), text("seller@example.com")
	if err := complete(company); err != nil {
		t.Fatalf("沒有身分、銀行與證件的企業申請：%v", err)
	}
}
