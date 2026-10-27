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

// 本檔驗 S05 收據代製授權（端點 28／29）。
//
// 重點在兩個「三態」：資格（eligible ＋ 三個互斥的 ineligibleReason）與
// 核定依據文件（§6.1.9 明訂**不得只有兩態**）。三態少一格的後果不是報錯，
// 是賣家看到一個點下去必定 404 的連結，或看到「未核准」而其實是「核定已過期」。

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

// ---------------------------------------------------------------------------
// 端點 28：資格三態
// ---------------------------------------------------------------------------

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
		// 🔴 §5.1.2：approval_valid_until 屆滿**不改狀態** ⇒ 只看 review_status 會把
		// 已過期的核定誤判成仍有效，而收據會以一個失效的資格繼續開出去。
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

// ---------------------------------------------------------------------------
// 端點 28：核定依據文件的 fail-closed 三態
// ---------------------------------------------------------------------------

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

	// 已指名但那份文件不在賣家自己上傳的清單裡（舊資料／ADM 誤上傳）。
	// media 的授權述詞等價於「只有上傳者本人」⇒ 賣家點下載一律 404。
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
		// key 仍照實回：ADM 要看得出「指名的是哪一份」才查得出為什麼。
		if got.ApprovalBasisKey == "" {
			t.Fatal("approvalBasisKey 是 SSOT，即使不可用也要回")
		}
	})

	// evidence 清單讀不出來時一律 fail-closed（不得樂觀當成有）。
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

// ---------------------------------------------------------------------------
// 端點 29：授權與取消
// ---------------------------------------------------------------------------

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
	// 條款版本必須進 append-only 的歷程：取消授權會把 receipt_consent_version 清成 NULL，
	// 而 seller_tax_profiles 沒有歷程表 ⇒ 這是「他同意的是哪一版」唯一留得住的地方。
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
	// 兩欄必須**同時**清空（chk_stp_receipt_auth_pair）。
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

// ---------------------------------------------------------------------------
// 端點 40：ADM 指名核定依據文件（§6.1.9）
// ---------------------------------------------------------------------------

func underReviewProfile() domain.SellerTaxProfile {
	p := exemptProfile(nil)
	p.ReviewStatus = codemeta.SellerTaxReviewStatusUnderReview.Value
	p.ApprovedAt, p.ApprovalValidUntil = nil, nil
	// 🔴 賣家已經簽過收據代製授權（S05）。這一行不是為了讓測試變綠：DEC-20260905-015836
	// 選 A 之後，核定 EXEMPT 的前置條件就是「授權已成立」（applyDecision 的 9354），
	// 而底下三支測試要驗的是**依據文件**的契約，不是授權守衛——fixture 少了這一對欄位，
	// 它們會全部停在 9354，看起來還是紅／綠，卻再也驗不到自己宣稱的那件事。
	// 兩欄同進同出（chk_stp_receipt_auth_pair），只給一個在真實 DB 上是 23514。
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
	// 端點 28 的投影隨即變成可用。
	if v := buildReceiptAuthorizationView(row, tNow); !v.ApprovalBasisAvailable {
		t.Fatalf("指名之後端點 28 應回可用，實得 %+v", v)
	}
}

func TestReceiptBasis_Rejected(t *testing.T) {
	cases := map[string]struct {
		decision string
		key      string
		assetID  string
		// 🔴 只斷言 err != nil 分不出「被依據文件的守衛擋下」與「被別的守衛擋下」。
		// 2026-09-05 實測：T08 授權守衛（9354）加進 applyDecision 之後排在這些檢查
		// 之前，本表五格會全部因為那個碼而「通過」——測試仍然綠，卻不再驗依據文件。
		want error
	}{
		// 兩欄同進同出：只給一個 ⇒ DB 會回 23514（一句 500），在這裡先擋成 400。
		"只給 key":     {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value, receiptBasisKey, "", ErrTaxApprovalBasisIncomplete},
		"只給 assetId": {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value, "", "778899", ErrTaxApprovalBasisIncomplete},
		// 🔴 指名一份不是這位賣家上傳的檔案 ⇒ 賣家點下載一律 404，而端點 28 的三態
		// 只看得出 UPLOADER_NOT_SELLER、看不出是誰指錯的。寫入時就擋掉。
		"不是本賣家上傳的": {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
			"invoice/tax/9999/admin.pdf", "778899", ErrTaxApprovalBasisNotOwned},
		// 核定為「應開發票」的賣家不走收據流程，給它依據文件只會多一個永遠用不到的連結。
		"核定為應開發票卻帶依據文件": {codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
			receiptBasisKey, "778899", ErrTaxApprovalBasisNotExempt},
		"assetId 不是正整數": {codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
			receiptBasisKey, "abc", ErrTaxApprovalBasisAssetInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := underReviewProfile()
			// 🔴 「核定為應開發票」那一格需要統編＋法定名稱，否則會先撞
			// InvoiceTaxApprovalIdentityIncomplete（9353，#3049 那道守衛）而根本走不到
			// 依據文件的檢查。2026-09-05 把本表的斷言從 `err != nil` 收緊成逐格比對
			// 具體錯誤時暴露出來 —— 在那之前它一直因為錯的理由而通過。
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
			// 守衛不成立時**核定本身也不得發生**：一半成功比全部失敗難查得多。
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
