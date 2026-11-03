package kyc

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func text(s string) *string { return &s }

// 送出只要求市集登記資料（企業的公司名稱與統編）；身分、銀行、證件與收款聯絡信箱不在申請的必填之內（信箱是帳號的聯絡信箱，送出時另外檢查）。
func TestRequirementsAreMarketplaceDataOnly(t *testing.T) {
	var fields []string
	for _, r := range requirements {
		fields = append(fields, r.field)
	}
	if want := []string{"business.company_name", "business.tax_id"}; !slices.Equal(fields, want) {
		t.Fatalf("requirements=%v，預期 %v", fields, want)
	}
	if err := complete(store.SellerKycApplication{SellerType: string(model.KYCIndividual)}); err != nil {
		t.Fatalf("空的個人申請：%v", err)
	}
	company := store.SellerKycApplication{SellerType: string(model.KYCCompany)}
	if got, want := missingFields(company), []string{"business.company_name", "business.tax_id"}; !slices.Equal(got, want) {
		t.Fatalf("企業缺：%v", got)
	}
	company.CompanyName, company.TaxID = text("範例小農有限公司"), text("12345678")
	if err := complete(company); err != nil {
		t.Fatalf("沒有身分、銀行與證件的企業申請：%v", err)
	}
}

type fakeIdentity struct {
	email identity.ContactEmail
	ok    bool
	err   error
}

func (f fakeIdentity) ContactEmail(context.Context, int64) (identity.ContactEmail, bool, error) {
	return f.email, f.ok, f.err
}

// 送審要求帳號有聯絡信箱：identity 說沒有（含已驗證信箱格式不合又沒自填）回 failed_precondition 與 CONTACT_EMAIL_REQUIRED；有的原樣採用 identity 給的值，seller 不再有第二份格式規則。
// mutation：requireContactEmail 不檢查 ok → 「none」失敗；seller 又自己正規化 → 「identity 的值原樣」失敗。
func TestContactEmailIsRequiredAndTakenFromIdentity(t *testing.T) {
	_, err := (&Service{d: Deps{Identity: fakeIdentity{}}}).requireContactEmail(context.Background(), 1)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonContactEmailRequired {
		t.Errorf("none：err=%v，預期 FailedPrecondition 與 CONTACT_EMAIL_REQUIRED", err)
	}
	got, err := (&Service{d: Deps{Identity: fakeIdentity{email: identity.ContactEmail{Address: "pay.out@example.com", Source: identity.ContactEmailSelfEntered}, ok: true}}}).requireContactEmail(context.Background(), 1)
	if err != nil || got != "pay.out@example.com" {
		t.Fatalf("identity 的值原樣：%q, %v", got, err)
	}
	boom := errors.New("boom")
	if _, err := (&Service{d: Deps{Identity: fakeIdentity{err: boom}}}).requireContactEmail(context.Background(), 1); !errors.Is(err, boom) {
		t.Fatalf("identity 的錯誤要原樣回傳：%v", err)
	}
}
