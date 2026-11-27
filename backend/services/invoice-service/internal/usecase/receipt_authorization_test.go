package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


const receiptBasisKey = "invoice/tax/2685914682/approval-2026.pdf"

func exemptProfile(mutate func(*domain.SellerTaxProfile)) domain.SellerTaxProfile {
	approved := tNow.Add(-30 * 24 * time.Hour)
	validUntil := tNow.Add(300 * 24 * time.Hour)
	p := domain.SellerTaxProfile{
		ID: 55, IssuerID: tIssuerA, Version: 3,
		ReviewStatus:       codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value,
		ApprovedAt:         &approved,
		ApprovalValidUntil: &validUntil,
		EvidenceObjectKeys: json.RawMessage(`["` + receiptBasisKey + `"]`),
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}


func TestReceiptAuth_Eligibility(t *testing.T) {
	revokedStatus := codemeta.SellerTaxReviewStatusRevoked.Value
	requiredStatus := codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value
	expired := tNow.Add(-time.Second)

	cases := []struct {
		name       string
		profile    domain.SellerTaxProfile
		wantElig   bool
		wantReason string
	}{
		{"核定免用發票且未屆期", exemptProfile(nil), true, ""},
		{"已撤銷", exemptProfile(func(p *domain.SellerTaxProfile) { p.ReviewStatus = revokedStatus }),
			false, receiptIneligibleApprovalRevoked},
		{"核定為應開發票", exemptProfile(func(p *domain.SellerTaxProfile) { p.ReviewStatus = requiredStatus }),
			false, receiptIneligibleNotApproved},
		{"核定已過期（狀態仍是 EXEMPT）",
			exemptProfile(func(p *domain.SellerTaxProfile) { p.ApprovalValidUntil = &expired }),
			false, receiptIneligibleApprovalExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildReceiptAuthorizationView(tc.profile, tNow)
			if got.Eligible != tc.wantElig || got.IneligibleReason != tc.wantReason {
				t.Fatalf("eligible=%v reason=%q，期望 %v／%q", got.Eligible, got.IneligibleReason, tc.wantElig, tc.wantReason)
			}
		})
	}
}


func TestReceiptAuth_ApprovalBasisThreeStates(t *testing.T) {
	assetID := int64(778899)

	t.Run("已指名且上傳者是本賣家", func(t *testing.T) {
		p := exemptProfile(func(p *domain.SellerTaxProfile) {
			key := receiptBasisKey
			p.ReceiptApprovalBasisKey = &key
			p.ReceiptApprovalBasisAssetID = &assetID
		})
		got := buildReceiptAuthorizationView(p, tNow)
		if !got.ApprovalBasisAvailable || got.ApprovalBasisUnavailableReason != "" {
			t.Fatalf("應為可用且無理由，實得 %+v", got)
		}
		if got.ApprovalBasisAssetID == nil || *got.ApprovalBasisAssetID != assetID {
			t.Fatalf("可用時必須帶 assetId（Gateway 靠它組下載路徑），實得 %v", got.ApprovalBasisAssetID)
		}
	})

	t.Run("ADM 未指名", func(t *testing.T) {
		got := buildReceiptAuthorizationView(exemptProfile(nil), tNow)
		if got.ApprovalBasisAvailable || got.ApprovalBasisUnavailableReason != receiptBasisNotDesignated {
			t.Fatalf("應為 NOT_DESIGNATED，實得 %+v", got)
		}
		if got.ApprovalBasisAssetID != nil {
			t.Fatal("不可用時不得回 assetId：那會變成一個點下去必定 404 的連結")
		}
	})

	t.Run("上傳者不是本賣家", func(t *testing.T) {
		p := exemptProfile(func(p *domain.SellerTaxProfile) {
			key := "invoice/tax/9999/admin-uploaded.pdf"
			p.ReceiptApprovalBasisKey = &key
			p.ReceiptApprovalBasisAssetID = &assetID
		})
		got := buildReceiptAuthorizationView(p, tNow)
		if got.ApprovalBasisAvailable || got.ApprovalBasisUnavailableReason != receiptBasisUploaderNotSelf {
			t.Fatalf("應為 UPLOADER_NOT_SELLER，實得 %+v", got)
		}
		if got.ApprovalBasisAssetID != nil {
			t.Fatal("不可用時不得回 assetId")
		}
		if got.ApprovalBasisKey == "" {
			t.Fatal("approvalBasisKey 是 SSOT，即使不可用也要回")
		}
	})

	t.Run("evidence 清單損毀", func(t *testing.T) {
		p := exemptProfile(func(p *domain.SellerTaxProfile) {
			key := receiptBasisKey
			p.ReceiptApprovalBasisKey = &key
			p.ReceiptApprovalBasisAssetID = &assetID
			p.EvidenceObjectKeys = json.RawMessage(`{"not":"an array"}`)
		})
		got := buildReceiptAuthorizationView(p, tNow)
		if got.ApprovalBasisAvailable {
			t.Fatal("清單解析失敗必須 fail-closed")
		}
	})
}


func TestReceiptAuth_Confirm(t *testing.T) {
	e := newTaxEnv(t, exemptProfile(nil))

	got, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
		StoreID: tStoreA, ConsentVersion: "v2026-08", Authorized: true, ExpectedVersion: 3,
		IdempotencyKey: "idem-29",
	})
	if err != nil {
		t.Fatalf("ConfirmReceiptAuthorization 失敗：%v", err)
	}
	if got.AuthorizedAt == nil || got.ConsentVersion != "v2026-08" {
		t.Fatalf("授權後兩欄必須同時有值，實得 %+v", got)
	}
	if got.Version != 4 {
		t.Fatalf("version 應遞增到 4，實得 %d", got.Version)
	}
	if len(e.audit.history) != 1 {
		t.Fatalf("應留一列狀態歷程，實得 %d", len(e.audit.history))
	}
	if e.audit.history[0].ReasonDetail == nil || *e.audit.history[0].ReasonDetail != "consentVersion=v2026-08" {
		t.Fatalf("歷程必須記下條款版本，實得 %v", e.audit.history[0].ReasonDetail)
	}
}

func TestReceiptAuth_Revoke(t *testing.T) {
	authorized := tNow.Add(-time.Hour)
	ver := "v2026-07"
	e := newTaxEnv(t, exemptProfile(func(p *domain.SellerTaxProfile) {
		p.ReceiptAuthorizedAt = &authorized
		p.ReceiptConsentVersion = &ver
	}))

	got, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
		StoreID: tStoreA, Authorized: false, ExpectedVersion: 3,
	})
	if err != nil {
		t.Fatalf("取消授權是正常結果不是錯誤：%v", err)
	}
	if got.AuthorizedAt != nil || got.ConsentVersion != "" {
		t.Fatalf("取消後兩欄必須同時清空，實得 %+v", got)
	}
}

// 非免用統一發票核准賣家 ⇒ 9335。不是 400：賣家的輸入沒有問題，是資格不符。
func TestReceiptAuth_Confirm_NotEligibleReturns9335(t *testing.T) {
	e := newTaxEnv(t, exemptProfile(func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value
	}))

	_, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
		StoreID: tStoreA, ConsentVersion: "v2026-08", Authorized: true, ExpectedVersion: 3,
	})
	assertBizCode(t, err, bizcode.InvoiceReceiptAuthNotEligible)
	if len(e.audit.history) != 0 {
		t.Fatalf("守衛不成立時不得寫歷程，實得 %d", len(e.audit.history))
	}
}

func TestReceiptAuth_Confirm_VersionConflictReturns9322(t *testing.T) {
	e := newTaxEnv(t, exemptProfile(nil))

	_, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
		StoreID: tStoreA, ConsentVersion: "v2026-08", Authorized: true, ExpectedVersion: 2,
	})
	assertBizCode(t, err, bizcode.SellerTaxProfileVersionConflict)
	if len(e.audit.history) != 0 {
		t.Fatalf("版本不符時不得寫歷程，實得 %d", len(e.audit.history))
	}
}

// CAS 在 pre-read 之後被搶先（另一個 pod 改走了）⇒ 仍必須回 9322，不得靜默成功。
func TestReceiptAuth_Confirm_CASLostRaceReturns9322(t *testing.T) {
	e := newTaxEnv(t, exemptProfile(nil))
	e.profiles.casFails = true

	_, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
		StoreID: tStoreA, ConsentVersion: "v2026-08", Authorized: true, ExpectedVersion: 3,
	})
	assertBizCode(t, err, bizcode.SellerTaxProfileVersionConflict)
	if len(e.audit.history) != 0 {
		t.Fatalf("CAS 失敗時不得寫歷程，實得 %d", len(e.audit.history))
	}
}

func TestReceiptAuth_Confirm_RejectsBadConsentVersion(t *testing.T) {
	cases := map[string]string{
		"空白":      "   ",
		"超過 32 字": repeatRune('v', 33),
	}
	for name, ver := range cases {
		t.Run(name, func(t *testing.T) {
			e := newTaxEnv(t, exemptProfile(nil))
			if _, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
				StoreID: tStoreA, ConsentVersion: ver, Authorized: true, ExpectedVersion: 3,
			}); err == nil {
				t.Fatal("應回契約違規錯誤")
			}
			if len(e.audit.history) != 0 {
				t.Fatal("驗證失敗時不得寫歷程")
			}
		})
	}
}

// 取消授權不需要 consentVersion（那是「同意哪一版」的欄位，取消沒有這回事）。
func TestReceiptAuth_RevokeDoesNotRequireConsentVersion(t *testing.T) {
	e := newTaxEnv(t, exemptProfile(nil))
	if _, err := e.uc.ConfirmReceiptAuthorization(context.Background(), ConfirmReceiptAuthorizationInput{
		StoreID: tStoreA, Authorized: false, ExpectedVersion: 3,
	}); err != nil {
		t.Fatalf("取消授權不應要求 consentVersion：%v", err)
	}
}


func underReviewProfile() domain.SellerTaxProfile {
	p := exemptProfile(nil)
	p.ReviewStatus = codemeta.SellerTaxReviewStatusUnderReview.Value
	p.ApprovedAt, p.ApprovalValidUntil = nil, nil
	authorized := tNow.Add(-24 * time.Hour)
	consent := "v1"
	p.ReceiptAuthorizedAt, p.ReceiptConsentVersion = &authorized, &consent
	return p
}

func TestReceiptBasis_DesignatedInApproveTransaction(t *testing.T) {
	e := newTaxEnv(t, underReviewProfile())

	if _, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: 90001, StoreID: tStoreA,
		Decision:  codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
		Reason:    "檢附國稅局核定函",
		Confirmed: true, ExpectedVersion: 3,
		ApprovalBasisKey: receiptBasisKey, ApprovalBasisAssetID: "778899",
	}); err != nil {
		t.Fatalf("Decide 失敗：%v", err)
	}

	row := e.profiles.byIssuer[tIssuerA]
	if row.ReceiptApprovalBasisKey == nil || *row.ReceiptApprovalBasisKey != receiptBasisKey {
		t.Fatalf("依據文件必須與核定寫在同一句 CAS，實得 %v", row.ReceiptApprovalBasisKey)
	}
	if row.ReceiptApprovalBasisAssetID == nil || *row.ReceiptApprovalBasisAssetID != 778899 {
		t.Fatalf("assetId 必須一起寫入，實得 %v", row.ReceiptApprovalBasisAssetID)
	}
	if v := buildReceiptAuthorizationView(row, tNow); !v.ApprovalBasisAvailable {
		t.Fatalf("指名之後端點 28 應回可用，實得 %+v", v)
	}
}

func TestReceiptBasis_Rejected(t *testing.T) {
	cases := map[string]struct {
		decision string
		key      string
		assetID  string
		want error
	}{
		"只給 key":     {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value, receiptBasisKey, "", ErrTaxApprovalBasisIncomplete},
		"只給 assetId": {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value, "", "778899", ErrTaxApprovalBasisIncomplete},
		"不是本賣家上傳的": {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
			"invoice/tax/9999/admin.pdf", "778899", ErrTaxApprovalBasisNotOwned},
		"核定為應開發票卻帶依據文件": {codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
			receiptBasisKey, "778899", ErrTaxApprovalBasisNotExempt},
		"assetId 不是正整數": {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
			receiptBasisKey, "abc", ErrTaxApprovalBasisAssetInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := underReviewProfile()
			if tc.decision == codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value {
				legal, taxID := "範例生鮮股份有限公司", "12345678"
				p.LegalName, p.TaxID = &legal, &taxID
			}
			e := newTaxEnv(t, p)
			_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
				ActorAccountID: 90001, StoreID: tStoreA, Decision: tc.decision,
				Reason: "核定", Confirmed: true, ExpectedVersion: 3,
				ApprovalBasisKey: tc.key, ApprovalBasisAssetID: tc.assetID,
			})
			if err == nil {
				t.Fatal("應回契約違規錯誤")
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("預期 %v，實得 %v —— 被別道守衛擋下的話這格就沒在驗依據文件", tc.want, err)
			}
			if e.profiles.byIssuer[tIssuerA].ReviewStatus != codemeta.SellerTaxReviewStatusUnderReview.Value {
				t.Fatalf("驗證失敗時不得核定，實得 %q", e.profiles.byIssuer[tIssuerA].ReviewStatus)
			}
		})
	}
}

// 不指名（兩欄皆空）是合法的：端點 28 於是回 NOT_DESIGNATED。
func TestReceiptBasis_OptionalOnApprove(t *testing.T) {
	e := newTaxEnv(t, underReviewProfile())
	if _, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: 90001, StoreID: tStoreA,
		Decision:  codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
		Reason:    "核定",
		Confirmed: true, ExpectedVersion: 3,
	}); err != nil {
		t.Fatalf("不指名應合法：%v", err)
	}
	v := buildReceiptAuthorizationView(e.profiles.byIssuer[tIssuerA], tNow)
	if v.ApprovalBasisUnavailableReason != receiptBasisNotDesignated {
		t.Fatalf("應為 NOT_DESIGNATED，實得 %q", v.ApprovalBasisUnavailableReason)
	}
}
