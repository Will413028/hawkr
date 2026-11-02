package kyc

import (
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 收款聯絡信箱（D23）是送出時的必填：其餘都齊全的個人申請只缺它時，送出的違規正好指向 payout_contact_email，補上之後通過。
func TestCompleteRequiresThePayoutContactEmail(t *testing.T) {
	text := func(s string) *string { return &s }
	row := store.SellerKycApplication{
		SellerType: string(model.KYCIndividual), RealName: text("王小明"), EnglishName: text("Wang Farm"), BankName: text("臺灣銀行"),
		BankCode: text("004"), BankAccountEnc: []byte{1}, IDCardNoEnc: []byte{1},
	}
	docs := []store.SellerKycDocument{
		{Slot: string(model.SlotIDCardFront)}, {Slot: string(model.SlotIDCardBack)}, {Slot: string(model.SlotBankBook)},
	}
	err := complete(row, docs)
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "payout_contact_email" {
		t.Fatalf("缺信箱的送出檢查=%v", err)
	}
	if got := missingFields(row, docs); len(got) != 1 || got[0] != "payout_contact_email" {
		t.Fatalf("missing_fields=%v", got)
	}
	row.PayoutContactEmail = text("seller@example.com")
	if err := complete(row, docs); err != nil {
		t.Fatalf("齊全的申請：%v", err)
	}
}
