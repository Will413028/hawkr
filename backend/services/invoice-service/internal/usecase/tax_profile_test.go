package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔驗 M3 稅務資格狀態機寫入面的**業務規則**（§6.1.7 端點 18／19／20／22 ＋
// §6.1.12 端點 40／41 ＝ §7.3.3 的 T01–T16）。型別邊界與 Tier A 的回應形狀在
// internal/grpc/tax_profile_test.go；真 SQL 的 CAS 述詞與 CHECK 在
// internal/repository/postgres/tax_profile_integration_test.go。
//
// 🔴 fake 的述詞**逐字照 db/queries/seller_tax_profiles.sql 寫**（allowed-from、
// version、extension_count、cooldown_until）。fake 寫寬鬆一點，這裡每一支守衛測試
// 就只是在驗 fake 自己，而正式路徑的 rows=0 一個都不會被抓到。
//
// ⚠️ fake **沒有 CHECK 約束**（chk_stp_pending_window／chk_stp_required_needs_taxid…）
// ⇒ 對那些不變式零鑑別力。它們由 -tags=integration 的真 SQL 測試守。

const (
	tStoreA  int64 = 2685914682
	tStoreB  int64 = 2632049831
	tIssuerA int64 = 5101
	tProfile int64 = 7101
	tAdmin   int64 = 9001
	tSeller  int64 = 4001
)

var tNow = time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type tIssuers struct {
	domain.IssuerRepository
	byRef       map[int64]domain.Issuer
	created     []domain.NewIssuer
	displayName []string
}

func newIssuers(refs ...int64) *tIssuers {
	r := &tIssuers{byRef: map[int64]domain.Issuer{}}
	for i, ref := range refs {
		r.byRef[ref] = domain.Issuer{ID: tIssuerA + int64(i), IssuerType: "SELLER", IssuerRefID: ref}
	}
	return r
}

// GetByRef 照 GetInvoiceIssuerByRef：查無回 9330（repository 的 notFound 映射），
// usecase 必須把它翻成 9320，否則「還沒有 issuer」與「有 issuer 但沒送過審」對外可分辨。
func (r *tIssuers) GetByRef(_ context.Context, _ string, refID int64) (domain.Issuer, error) {
	row, ok := r.byRef[refID]
	if !ok {
		return domain.Issuer{}, pkgerrors.NewBusiness(bizcode.InvoiceProviderAccountNotFound, "not found")
	}
	return row, nil
}

func (r *tIssuers) Create(_ context.Context, in domain.NewIssuer) (domain.Issuer, bool, error) {
	r.created = append(r.created, in)
	if _, dup := r.byRef[in.IssuerRefID]; dup {
		return domain.Issuer{}, false, nil // ON CONFLICT DO NOTHING
	}
	row := domain.Issuer{ID: in.ID, IssuerType: in.IssuerType, IssuerRefID: in.IssuerRefID, DisplayName: in.DisplayName}
	r.byRef[in.IssuerRefID] = row
	return row, true, nil
}

func (r *tIssuers) UpdateDisplayName(_ context.Context, _ int64, name string, _ time.Time) (bool, error) {
	r.displayName = append(r.displayName, name)
	return true, nil
}

// tProfiles 是 seller_tax_profiles 的 fake，每一支寫入方法的述詞逐字照 SQL。
type tProfiles struct {
	domain.SellerTaxProfileRepository
	byIssuer map[int64]domain.SellerTaxProfile
	created  int
	// casFails 讓每一句 CAS 都回 rows=0，模擬「讀與寫之間被另一個 pod 改掉了」。
	//
	// 🔴 它存在的理由是變異驗證抓到的：Decide 的 Go 端相容表是**第一道**防線，
	// 它在正常路徑上就把不相容的決策擋掉了 ⇒ CAS 的 !ok 分支在測試裡永遠走不到，
	// 把 taxCASResult 改成「rows=0 回 nil」（靜默失敗）整組測試照樣綠。
	// 沒有這個旗標，那條分支就是沒有任何東西在守。
	casFails bool
	// submitCalls／resetCalls 讓陰性測試斷言「零寫入」而不只是「狀態沒變」。
	//
	// 🔴 兩者不可互相取代：ResetToDraft 對 REJECTED 的列**會**改狀態，如果 9325 的
	// 判定被挪進交易內，狀態最終仍會 rollback 回原值 ⇒ 只斷言 ReviewStatus 的測試
	// 照樣綠。要抓到「判定位置變了」只能數呼叫次數。
	submitCalls int
	resetCalls  int
}

func newProfiles(rows ...domain.SellerTaxProfile) *tProfiles {
	r := &tProfiles{byIssuer: map[int64]domain.SellerTaxProfile{}}
	for _, row := range rows {
		r.byIssuer[row.IssuerID] = row
	}
	return r
}

func (r *tProfiles) GetByIssuer(_ context.Context, issuerID int64) (domain.SellerTaxProfile, error) {
	row, ok := r.byIssuer[issuerID]
	if !ok {
		return domain.SellerTaxProfile{}, pkgerrors.NewBusiness(bizcode.SellerTaxProfileNotFound, "not found")
	}
	return row, nil
}

func (r *tProfiles) GetByID(_ context.Context, id int64) (domain.SellerTaxProfile, error) {
	for _, row := range r.byIssuer {
		if row.ID == id {
			return row, nil
		}
	}
	return domain.SellerTaxProfile{}, pkgerrors.NewBusiness(bizcode.SellerTaxProfileNotFound, "not found")
}

// ConfirmReceiptAuthorization 重現端點 29 那一句 CAS 的兩個 WHERE 述詞
// （資格必須是 APPROVED_INVOICE_EXEMPT ＋ 樂觀鎖）。無條件回 true 會讓 9335／9322
// 兩支陰性全綠而什麼都沒驗到。
func (r *tProfiles) ConfirmReceiptAuthorization(_ context.Context, in domain.ConfirmReceiptAuthorization) (bool, error) {
	if r.casFails {
		return false, nil
	}
	for issuerID, row := range r.byIssuer {
		if row.ID != in.ID {
			continue
		}
		if row.ReviewStatus != codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value {
			return false, nil
		}
		if row.Version != in.ExpectedVersion {
			return false, nil
		}
		row.ReceiptAuthorizedAt = in.AuthorizedAt
		row.ReceiptConsentVersion = in.ConsentVersion
		row.Version++
		r.byIssuer[issuerID] = row
		return true, nil
	}
	return false, nil
}

func (r *tProfiles) Create(_ context.Context, id, issuerID int64, now time.Time) (domain.SellerTaxProfile, bool, error) {
	if _, dup := r.byIssuer[issuerID]; dup {
		return domain.SellerTaxProfile{}, false, nil
	}
	r.created++
	row := domain.SellerTaxProfile{
		ID: id, IssuerID: issuerID,
		ReviewStatus:      codemeta.SellerTaxReviewStatusDraft.Value,
		ClaimedObligation: codemeta.SellerTaxObligationUnknown.Value,
		// DDL DEFAULT：evidence_object_keys '[]'、default_tax_category 1、version 0。
		EvidenceObjectKeys:      json.RawMessage(`[]`),
		DefaultTaxCategory:      1,
		PendingOrderLimitCount:  50,
		PendingOrderLimitAmount: 100000,
		CreatedAt:               now, UpdatedAt: now,
	}
	r.byIssuer[issuerID] = row
	return row, true, nil
}

// mutate 找出 id 對應的列，套用 fn，並模擬 SQL 的 version = version + 1。
func (r *tProfiles) mutate(id int64, guard func(domain.SellerTaxProfile) bool, fn func(*domain.SellerTaxProfile)) bool {
	if r.casFails {
		return false
	}
	for k, row := range r.byIssuer {
		if row.ID != id {
			continue
		}
		if !guard(row) {
			return false
		}
		fn(&row)
		row.Version++
		r.byIssuer[k] = row
		return true
	}
	return false
}

// ResetToDraft 照 ResetSellerTaxProfileToDraft：allowed-from ＋ 冷卻，**無 version 述詞**。
func (r *tProfiles) ResetToDraft(_ context.Context, id int64, now time.Time) (bool, error) {
	r.resetCalls++
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return (p.ReviewStatus == "REJECTED" || p.ReviewStatus == "EXPIRED") &&
			(p.CooldownUntil == nil || !p.CooldownUntil.After(now))
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "DRAFT"
		p.SubmittedAt, p.PendingValidUntil, p.CooldownUntil = nil, nil, nil
		p.ExtensionCount = 0
	}), nil
}

func (r *tProfiles) UpdateDraft(_ context.Context, in domain.UpdateSellerTaxDraft) (bool, error) {
	keys, _ := json.Marshal(in.EvidenceObjectKeys)
	return r.mutate(in.ID, func(p domain.SellerTaxProfile) bool {
		return p.ReviewStatus == "DRAFT" && p.Version == in.ExpectedVersion
	}, func(p *domain.SellerTaxProfile) {
		p.ClaimedObligation = in.ClaimedObligation
		p.EntityTaxType, p.LegalName = snil(in.EntityTaxType), snil(in.LegalName)
		p.TaxID, p.ResponsiblePerson = snil(in.TaxID), snil(in.ResponsiblePerson)
		p.BusinessAddress = snil(in.BusinessAddress)
		p.EvidenceObjectKeys = keys
	}), nil
}

// Submit 模擬 T02 的**單一句** UPDATE：五欄同進同出。
//
// 🔴 attestation 兩欄與狀態／時鐘欄寫在同一個 mutate 內是刻意的鏡像——真實 SQL
// 就是一句（見 db/queries/seller_tax_profiles.sql）。若把它們拆成兩次 mutate，
// fake 會比 production 寬鬆，而「送審成功 ⇒ 條款必已簽」的測試就會失去鑑別力。
func (r *tProfiles) Submit(_ context.Context, id, ver int64, attestationVersion string, attestedBy int64, until, now time.Time) (bool, error) {
	r.submitCalls++
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return p.ReviewStatus == "DRAFT" && p.Version == ver
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "SUBMITTED"
		n, u := now, until
		p.SubmittedAt, p.PendingValidUntil = &n, &u
		if attestationVersion != "" {
			v, at := attestationVersion, now
			p.AttestationVersion, p.AttestedAt = &v, &at
			// #3031：三元組同一句寫（真實 SQL 就是一句）。0 ⇒ NULL。
			if attestedBy != 0 {
				by := attestedBy
				p.AttestedByAccountID = &by
			}
		}
	}), nil
}

func (r *tProfiles) Attest(_ context.Context, id, ver int64, version string, attestedBy int64, now time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return p.ReviewStatus == "SUBMITTED" && p.Version == ver
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "SELF_ISSUE_PENDING"
		v, n := version, now
		p.AttestationVersion, p.AttestedAt = &v, &n
		if attestedBy != 0 {
			by := attestedBy
			p.AttestedByAccountID = &by
		}
	}), nil
}

func (r *tProfiles) RequestSupplement(_ context.Context, id, ver int64, note string, _ time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return p.ReviewStatus == "UNDER_REVIEW" && p.Version == ver
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "SUPPLEMENT_REQUIRED"
		p.ReviewNote = snil(note)
	}), nil
}

// Approve **鏡射兩條 DB CHECK**（chk_stp_required_needs_taxid／chk_stp_exempt_no_taxid）。
//
// 🔴 為什麼替身要做這件事（#3089 B 組）：這兩條 CHECK 是 ApproveSellerTaxProfile
// 那句 SQL 的檔頭逐字指名的守衛——「差別由 DB 層擋 ⇒ 共用一句不會放行錯誤組合
// （傳錯值當場 23514）」。替身不鏡射它們，測試就會**對一個在真實 DB 上必定 23514
// 的組合回綠**，而 23514 冒泡到使用者面前就是 HTTP 500。
//
// 實測：本鏡射加上之前，TestTaxProfile_Decide_CompatibilityMatrix 對
// 「EXEMPT ＋ profile 帶統編」是綠的，而那正是 #3089 B 組第 5 項的實例
// （dev 實打 HTTP 500）。**替身沒鏡射真實守衛**是本輪清理的缺陷類之一。
func (r *tProfiles) Approve(_ context.Context, in domain.ApproveSellerTaxProfile) (bool, error) {
	for _, row := range r.byIssuer {
		if row.ID != in.ID {
			continue
		}
		switch in.NextStatus {
		case codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value:
			if row.TaxID == nil || row.LegalName == nil {
				return false, fmt.Errorf("tProfiles: 違反 chk_stp_required_needs_taxid "+
					"（真實 DB 回 SQLSTATE 23514 並讓整個交易 rollback）：tax_id=%v legal_name=%v",
					row.TaxID, row.LegalName)
			}
		case codemeta.SellerTaxReviewStatusApprovedInvoiceExempt.Value:
			if row.TaxID != nil {
				return false, fmt.Errorf("tProfiles: 違反 chk_stp_exempt_no_taxid "+
					"（真實 DB 回 SQLSTATE 23514 並讓整個交易 rollback）：tax_id=%v", *row.TaxID)
			}
		}
	}
	return r.mutate(in.ID, func(p domain.SellerTaxProfile) bool {
		return (p.ReviewStatus == "UNDER_REVIEW" || p.ReviewStatus == "SELF_ISSUE_PENDING") &&
			p.Version == in.ExpectedVersion
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = in.NextStatus
		n, u := in.Now, in.ApprovalValidUntil
		p.ApprovedAt, p.ApprovalValidUntil, p.CooldownUntil = &n, &u, nil
		// 核定依據文件與核定寫在**同一句** CAS（§6.1.9）：分兩句的話第二句的
		// expected_version 會變成「第一句 +1」，那是在 Go 端重建 SQL 已經算過的算式。
		p.ReceiptApprovalBasisKey, p.ReceiptApprovalBasisAssetID = in.ApprovalBasisKey, in.ApprovalBasisAssetID
	}), nil
}

func (r *tProfiles) Reject(_ context.Context, id, ver int64, note string, cooldown, _ time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return (p.ReviewStatus == "UNDER_REVIEW" || p.ReviewStatus == "SELF_ISSUE_PENDING") && p.Version == ver
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "REJECTED"
		c := cooldown
		p.CooldownUntil, p.ReviewNote = &c, snil(note)
	}), nil
}

func (r *tProfiles) ExtendPending(_ context.Context, id, ver int64, until time.Time, note string, _ time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return p.ReviewStatus == "SELF_ISSUE_PENDING" && p.ExtensionCount == 0 && p.Version == ver
	}, func(p *domain.SellerTaxProfile) {
		u := until
		p.PendingValidUntil, p.ExtensionCount, p.ReviewNote = &u, 1, snil(note)
	}), nil
}

func (r *tProfiles) ReopenReview(_ context.Context, id, ver int64, reason string, _ time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		switch p.ReviewStatus {
		case "EXPIRED", "APPROVED_INVOICE_REQUIRED", "APPROVED_INVOICE_EXEMPT", "REVOKED":
			return p.Version == ver
		}
		return false
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "UNDER_REVIEW"
		p.RecheckReason, p.CooldownUntil = snil(reason), nil
	}), nil
}

func (r *tProfiles) Revoke(_ context.Context, id, ver int64, note string, _ time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return (p.ReviewStatus == "APPROVED_INVOICE_REQUIRED" || p.ReviewStatus == "APPROVED_INVOICE_EXEMPT") &&
			p.Version == ver
	}, func(p *domain.SellerTaxProfile) {
		p.ReviewStatus = "REVOKED"
		p.ReviewNote = snil(note)
	}), nil
}

func (r *tProfiles) KeepUnderReview(_ context.Context, id, ver int64, note string, _ time.Time) (bool, error) {
	return r.mutate(id, func(p domain.SellerTaxProfile) bool {
		return p.ReviewStatus == "UNDER_REVIEW" && p.Version == ver
	}, func(p *domain.SellerTaxProfile) { p.ReviewNote = snil(note) }), nil
}

type tAudit struct {
	domain.AuditRepository
	history []domain.NewStatusHistoryEntry
	admin   []domain.NewAdminAuditLogEntry
	nextID  int64
}

func (r *tAudit) AppendStatusHistory(_ context.Context, in domain.NewStatusHistoryEntry) error {
	r.history = append(r.history, in)
	return nil
}

func (r *tAudit) AppendAdminAuditLog(_ context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	r.admin = append(r.admin, in)
	r.nextID++
	return r.nextID, nil
}

type tOutbox struct {
	domain.OutboxRepository
	events []domain.NewOutboxEvent
}

func (r *tOutbox) Insert(_ context.Context, in domain.NewOutboxEvent) (domain.OutboxEvent, error) {
	r.events = append(r.events, in)
	return domain.OutboxEvent{}, nil
}

type tEvidence struct {
	keys     []string
	assetIDs []string
	err      error
	calls    int
	released int
}

func (f *tEvidence) FinalizeEvidence(_ context.Context, tokens []string, _ int64) ([]string, []string, error) {
	if len(tokens) == 0 {
		return nil, nil, nil
	}
	f.calls++
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.keys, f.assetIDs, nil
}

func (f *tEvidence) ReleaseEvidence(context.Context, int64, []string) error {
	f.released++
	return nil
}

func snil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

type tEnv struct {
	uc       *TaxProfile
	issuers  *tIssuers
	profiles *tProfiles
	audit    *tAudit
	outbox   *tOutbox
	evidence *tEvidence
}

func newTaxEnv(t *testing.T, rows ...domain.SellerTaxProfile) *tEnv {
	t.Helper()
	e := &tEnv{
		issuers:  newIssuers(tStoreA),
		profiles: newProfiles(rows...),
		audit:    &tAudit{},
		outbox:   &tOutbox{},
		evidence: &tEvidence{},
	}
	seq := int64(0)
	e.uc = NewTaxProfile(directTx{}, e.issuers, e.profiles, e.audit, e.outbox, e.evidence,
		func() int64 { seq++; return 90000 + seq }, nil).
		WithClock(func() time.Time { return tNow })
	return e
}

// profileAt 造一列指定狀態的 profile（欄位齊全，足以通過 T02 的必填守衛）。
func profileAt(status string) domain.SellerTaxProfile {
	entity, legal, taxID := "COMPANY", "範例生鮮股份有限公司", "12345678"
	person, addr := "王小明", "台北市信義區市府路1號"
	deadline := tNow.Add(30 * 24 * time.Hour)
	submitted := tNow.Add(-30 * 24 * time.Hour)
	return domain.SellerTaxProfile{
		ID: tProfile, IssuerID: tIssuerA, ReviewStatus: status,
		ClaimedObligation: codemeta.SellerTaxObligationRequired.Value,
		EntityTaxType:     &entity, LegalName: &legal, TaxID: &taxID,
		ResponsiblePerson: &person, BusinessAddress: &addr,
		DefaultTaxCategory: 1,
		EvidenceObjectKeys: json.RawMessage(`["secure/certification/1/a.pdf"]`),
		SubmittedAt:        &submitted, PendingValidUntil: &deadline,
		PendingOrderLimitCount: 50, PendingOrderLimitAmount: 100000,
	}
}

func wantBiz(t *testing.T, err error, code int32) {
	t.Helper()
	if err == nil {
		t.Fatalf("預期業務碼 %d，實際成功——rows=0 被當成成功往下走就是靜默失敗", code)
	}
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) || int32(de.Code) != code {
		t.Fatalf("預期業務碼 %d，實際 %v", code, err)
	}
}

// ---------------------------------------------------------------------------
// 端點 18：草稿存檔（T01 ＋ 草稿）
// ---------------------------------------------------------------------------

// 陽性：全新賣家（沒有 issuer、沒有 profile）存草稿 ⇒ 兩列都建起來，狀態 DRAFT。
//
// 這是本服務**唯一**的 invoice_issuers 建列點：它不成立，seller_tax_profiles 的 FK
// 永遠找不到對象，整條 S02 一列都建不出來。
func TestTaxProfile_SaveDraft_CreatesIssuerAndProfile(t *testing.T) {
	e := newTaxEnv(t)
	// 全新賣家：連 invoice_issuers 都還沒有那一列。
	delete(e.issuers.byRef, tStoreA)
	e.evidence.keys = []string{"secure/certification/1/a.pdf"}
	e.evidence.assetIDs = []string{"555"}

	view, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0,
		EntityType: "COMPANY", TaxID: "12345678", LegalName: "範例生鮮股份有限公司",
		ResponsibleName: "王小明", AddressCity: "台北市", AddressDistrict: "信義區", AddressDetail: "市府路1號",
		ClaimedStatus:        codemeta.SellerTaxObligationRequired.Value,
		EvidenceStagedTokens: []string{"tok-1"},
	})
	if err != nil {
		t.Fatalf("SaveDraft 失敗：%v", err)
	}
	if view.Profile.ReviewStatus != "DRAFT" {
		t.Fatalf("status = %s，want DRAFT", view.Profile.ReviewStatus)
	}
	if e.profiles.created != 1 || len(e.issuers.created) != 1 {
		t.Fatalf("issuer/profile 建列次數 = %d/%d，want 1/1", len(e.issuers.created), e.profiles.created)
	}
	// display_name ＝ 法定名稱：留著佔位字串會一路寫進 invoice_documents.issuer_name_snapshot，
	// 而發票上的銷售人名稱寫錯只能作廢重開。
	if len(e.issuers.displayName) != 1 || e.issuers.displayName[0] != "範例生鮮股份有限公司" {
		t.Fatalf("display_name 沒有同步成法定名稱：%v", e.issuers.displayName)
	}
	if got := view.Profile.EvidenceKeys(); len(got) != 1 {
		t.Fatalf("evidence_object_keys = %v，want 換發後的 1 筆", got)
	}
	// ④ 回收只在真的有換發時呼叫。
	if e.evidence.released != 1 {
		t.Fatalf("ReleaseEvidence 呼叫 %d 次，want 1", e.evidence.released)
	}
}

// 陰性（IDOR）：保留清單不是自己的 key ⇒ 400，而且**一份資產都不會被轉正**。
//
// 少了這道檢查，賣家可以把別人的 objectKey 塞進自己的佐證列，而那些 key 會被 ADM
// 在複審時下載。
func TestTaxProfile_SaveDraft_RejectsForeignEvidenceKeyBeforeFinalize(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))

	_, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0,
		EvidenceObjectKeys:   []string{"secure/certification/999/other.pdf"},
		EvidenceStagedTokens: []string{"tok-1"},
	})
	if !errors.Is(err, ErrTaxEvidenceNotOwned) {
		t.Fatalf("預期 ErrTaxEvidenceNotOwned，實際 %v", err)
	}
	if e.evidence.calls != 0 {
		t.Fatalf("越權請求不得觸發換發，實際呼叫 %d 次", e.evidence.calls)
	}
}

// 陰性（狀態機）：SUBMITTED 的賣家不得就地改內容 ⇒ 9321，且**資料未被改動**。
func TestTaxProfile_SaveDraft_StateInvalid(t *testing.T) {
	e := newTaxEnv(t, profileAt("SUBMITTED"))
	before := e.profiles.byIssuer[tIssuerA]

	_, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0, LegalName: "改個名字",
	})
	wantBiz(t, err, bizcode.SellerTaxProfileStateInvalid)
	after := e.profiles.byIssuer[tIssuerA]
	if after.Version != before.Version || derefString(after.LegalName) != derefString(before.LegalName) {
		t.Fatalf("守衛不成立時資料被改動了：%+v → %+v", before, after)
	}
}

// 陰性（樂觀鎖）：版本不符 ⇒ 9322。
func TestTaxProfile_SaveDraft_VersionConflict(t *testing.T) {
	row := profileAt("DRAFT")
	row.Version = 3
	e := newTaxEnv(t, row)

	_, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{StoreID: tStoreA, ExpectedVersion: 2})
	wantBiz(t, err, bizcode.SellerTaxProfileVersionConflict)
	if e.profiles.byIssuer[tIssuerA].Version != 3 {
		t.Fatal("版本衝突時不得寫入")
	}
}

// 陰性（換發失敗即中止）：FinalizeImages 失敗 ⇒ DB 一個字都不寫。
//
// 放行的後果是 evidence_object_keys 少了那一份，而草稿存檔看起來是成功的。
func TestTaxProfile_SaveDraft_FinalizeFailureWritesNothing(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))
	e.evidence.err = errors.New("media down")
	before := e.profiles.byIssuer[tIssuerA]

	_, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0, EvidenceStagedTokens: []string{"tok-1"},
	})
	if err == nil {
		t.Fatal("換發失敗必須中止整個請求")
	}
	if e.profiles.byIssuer[tIssuerA].Version != before.Version {
		t.Fatal("換發失敗時不得寫 DB")
	}
}

// ---------------------------------------------------------------------------
// 端點 19：送出審核（T02，必要時先 T01）
// ---------------------------------------------------------------------------

func TestTaxProfile_Submit_DraftToSubmitted(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))

	res, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreA, AcceptedTerms: true})
	if err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	if res.CooldownBlocked {
		t.Fatal("DRAFT 送審不該被冷卻擋下")
	}
	p := res.View.Profile
	if p.ReviewStatus != "SUBMITTED" {
		t.Fatalf("status = %s，want SUBMITTED", p.ReviewStatus)
	}
	// 單一時鐘：submitted_at ＝ now，pending_valid_until ＝ now + 60 天（決策 C4）。
	if p.SubmittedAt == nil || !p.SubmittedAt.Equal(tNow) {
		t.Fatalf("submitted_at = %v，want %v", p.SubmittedAt, tNow)
	}
	if p.PendingValidUntil == nil || !p.PendingValidUntil.Equal(tNow.Add(taxSubmitWindow)) {
		t.Fatalf("pending_valid_until = %v，want %v", p.PendingValidUntil, tNow.Add(taxSubmitWindow))
	}
}

// T02 守衛②：evidence_object_keys 為空 ⇒ **400 而不是 9321**（§7.3.3 逐字）。
// 兩者的下一步完全不同：一個要補檔、一個要等審核。
func TestTaxProfile_Submit_RequiresEvidence(t *testing.T) {
	row := profileAt("DRAFT")
	row.EvidenceObjectKeys = json.RawMessage(`[]`)
	e := newTaxEnv(t, row)

	_, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreA, AcceptedTerms: true})
	if !errors.Is(err, ErrTaxEvidenceRequired) {
		t.Fatalf("預期 ErrTaxEvidenceRequired，實際 %v", err)
	}
	if e.profiles.byIssuer[tIssuerA].ReviewStatus != "DRAFT" {
		t.Fatal("守衛不成立時狀態不得改變")
	}
}

// T02 守衛①：必填欄位不齊 ⇒ 400。
func TestTaxProfile_Submit_RequiresCompleteIdentity(t *testing.T) {
	row := profileAt("DRAFT")
	row.LegalName = nil
	e := newTaxEnv(t, row)

	_, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreA, AcceptedTerms: true})
	if !errors.Is(err, ErrTaxIdentityIncomplete) {
		t.Fatalf("預期 ErrTaxIdentityIncomplete，實際 %v", err)
	}
}

// T01 重申：REJECTED ＋ 冷卻已過 ⇒ 先回 DRAFT 再 T02，一次請求走完兩條邊。
func TestTaxProfile_Submit_ReapplyAfterCooldown(t *testing.T) {
	row := profileAt("REJECTED")
	past := tNow.Add(-time.Hour)
	row.CooldownUntil = &past
	e := newTaxEnv(t, row)

	res, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreA, AcceptedTerms: true})
	if err != nil {
		t.Fatalf("重申失敗：%v", err)
	}
	if res.View.Profile.ReviewStatus != "SUBMITTED" {
		t.Fatalf("status = %s，want SUBMITTED", res.View.Profile.ReviewStatus)
	}
	// 兩條邊各留一筆歷程：少了 T01 那一筆，事後看不出這是重申還是首次送審。
	if len(e.audit.history) != 2 {
		t.Fatalf("狀態歷程 %d 筆，want 2（T01 ＋ T02）", len(e.audit.history))
	}
}

// 陰性（冷卻）：REJECTED ＋ 冷卻未到 ⇒ **9324 ＋ data**（preserveData，HTTP 200），
// 不是 9321：前端要靠 data.reapplyCooldownUntil 顯示倒數。
func TestTaxProfile_Submit_CooldownBlocked(t *testing.T) {
	row := profileAt("REJECTED")
	future := tNow.Add(3 * 24 * time.Hour)
	row.CooldownUntil = &future
	e := newTaxEnv(t, row)

	res, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreA, AcceptedTerms: true})
	if err != nil {
		t.Fatalf("冷卻不是錯誤，應回 9324 ＋ data：%v", err)
	}
	if !res.CooldownBlocked {
		t.Fatal("冷卻未到卻沒有被擋下——被駁回的賣家可以立刻無限重送")
	}
	if res.View.Profile.CooldownUntil == nil {
		t.Fatal("9324 的 data 必須帶得出 reapplyCooldownUntil，否則賣家不知道何時可再試")
	}
	if e.profiles.byIssuer[tIssuerA].ReviewStatus != "REJECTED" {
		t.Fatal("冷卻擋下時狀態不得改變")
	}
}

// 陰性（Tier A）：別人的 store_id ⇒ 9320，與「查無」同一個出口。
func TestTaxProfile_Submit_ForeignStoreIsNotFound(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))
	_, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreB, AcceptedTerms: true})
	wantBiz(t, err, bizcode.SellerTaxProfileNotFound)
}

// ---------------------------------------------------------------------------
// 端點 19 × 裁決 C：條款簽署與送審是同一個動作
// ---------------------------------------------------------------------------
//
// 這一組守的是不變式：**送審成功 ⇒ 自行開票責任條款必已簽**。
//
// 它在裁決 C 之前是不可能成立的：§6.1.7／§8 要求「未接受條款不得送審」，而 T04
// （端點 22）的 allowed-from 是 SUBMITTED ⇒ 送審要先簽、簽要先送審，DRAFT 的新
// 賣家永遠走不進那個環。裁決 C 把簽署併進送審這一步來拆環。

// 陽性：勾了條款送審 ⇒ 狀態、兩個時鐘欄、條款兩欄**一起**落庫。
//
// 🔴 這一支只證明「有寫」，證明不了「同時寫」。原子性由
// TestTaxProfile_Submit_AttestationIsWrittenInTheSameStatement 守。
func TestTaxProfile_Submit_AcceptedTermsRecordsAttestation(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))

	res, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: true,
	})
	if err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	p := res.View.Profile
	if p.ReviewStatus != "SUBMITTED" {
		t.Fatalf("status = %s，want SUBMITTED", p.ReviewStatus)
	}
	if p.AttestationVersion == nil || *p.AttestationVersion != SelfIssueTermsVersion {
		t.Fatalf("attestation_version = %v，want %q——送審成功但條款沒簽，"+
			"§6.1.7／§8 的保證是假的", p.AttestationVersion, SelfIssueTermsVersion)
	}
	if p.AttestedAt == nil || !p.AttestedAt.Equal(tNow) {
		t.Fatalf("attested_at = %v，want %v", p.AttestedAt, tNow)
	}
	// 版本不是前端可控值：稽核要能指認賣家簽的是平台發布過的那一版。
	if len(e.audit.history) != 1 || e.audit.history[0].ReasonDetail == nil ||
		*e.audit.history[0].ReasonDetail != SelfIssueTermsVersion {
		t.Fatalf("送審歷程沒有留下條款版本，稽核事後查不到簽了哪一版：%+v", e.audit.history)
	}
}

// 稽核：條款簽署必須留下**是誰簽的**（自然人），不只是簽了哪一版。
//
// 🔴 裁決 C 之後端點 19 是絕大多數賣家**唯一**的簽署動作 ⇒ 這裡缺 actor，
// 那份「審核期間發票由賣家自行開立、責任在賣家」的憑據就查不到簽署主體。
// 端點 22（補簽）本來就有帶，主路徑反而沒有——這一支把兩條路徑拉齊。
func TestTaxProfile_Submit_RecordsSigningAccount(t *testing.T) {
	const actor int64 = 4242
	e := newTaxEnv(t, profileAt("DRAFT"))

	if _, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: true, AcceptedByAccountID: actor,
	}); err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	if len(e.audit.history) != 1 {
		t.Fatalf("歷程筆數 = %d，want 1：%+v", len(e.audit.history), e.audit.history)
	}
	got := e.audit.history[0].ActorAccountID
	if got == nil || *got != actor {
		t.Fatalf("送審歷程的 actor_account_id = %v，want %d —— 條款是自然人的意思表示，"+
			"查不到人的簽署記錄在爭議時舉不出主體", got, actor)
	}

	// 陰性：沒有可信身分時記成 NULL，**不得**拿 store_id 頂替——
	// 賣家換手後 store_id 會指向新任賣家，而條款是前任那個自然人簽的。
	e2 := newTaxEnv(t, profileAt("DRAFT"))
	if _, err := e2.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: true,
	}); err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	if a := e2.audit.history[0].ActorAccountID; a != nil {
		t.Fatalf("無身分時 actor 應為 NULL，實際 %d（像是被 store_id 頂替了）", *a)
	}
}

// 🔴 **原子性**：條款兩欄與狀態／時鐘欄必須由**同一句** UPDATE 寫。
//
// 鑑別力來源：fake 的 Submit 只在 attestationVersion 非空時才寫條款兩欄，且與狀態
// 同在一次 mutate 內 ⇒ 若 production 改成「先 Submit 再另外一句 Attest」，
// usecase 傳進來的 attestationVersion 會是空字串，本測試當場紅。
// 換句話說：把兩個寫入拆開，這一支就抓得到，不必等到真的有人在交易外呼叫。
func TestTaxProfile_Submit_AttestationIsWrittenInTheSameStatement(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))

	if _, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: true,
	}); err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	// 一次 Submit ＝ 一句 UPDATE。多於一次代表有人把它拆成「送審 + 補簽」兩句，
	// 那樣的組合在中途失敗時會留下「已 SUBMITTED 但沒簽條款」的列。
	if e.profiles.submitCalls != 1 {
		t.Fatalf("profiles.Submit 被呼叫 %d 次，want 1（拆成兩句就不是同一句了）",
			e.profiles.submitCalls)
	}
	row := e.profiles.byIssuer[tIssuerA]
	if row.ReviewStatus == "SUBMITTED" && (row.AttestationVersion == nil || row.AttestedAt == nil) {
		t.Fatal("列已是 SUBMITTED 但條款兩欄為空——不變式破了：" +
			"這正是「同一交易內兩句」在中途失敗時會留下的狀態")
	}
}

// 🔴 **陰性**：accepted_terms=false ⇒ 9325，且**一筆都不寫**。
//
// 兩個斷言缺一不可：
//   - 狀態不變：賣家沒有被推進 SUBMITTED
//   - repo 零呼叫：證明判定發生在交易**之前**。只斷言狀態的話，把判定挪進交易內
//     （寫了再 rollback）照樣綠，而那會讓 REJECTED 的入邊先做掉 T01 ResetToDraft。
func TestTaxProfile_Submit_RejectsUnacceptedTerms(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))
	before := e.profiles.byIssuer[tIssuerA]

	_, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: false,
	})
	wantBiz(t, err, bizcode.SellerSelfIssueTermsNotAccepted)

	after := e.profiles.byIssuer[tIssuerA]
	if after.ReviewStatus != before.ReviewStatus || after.Version != before.Version {
		t.Fatalf("9325 擋下時狀態不得改變：%s(v%d) → %s(v%d)",
			before.ReviewStatus, before.Version, after.ReviewStatus, after.Version)
	}
	if after.AttestationVersion != nil || after.AttestedAt != nil {
		t.Fatalf("未同意卻寫了條款記錄：%+v", after)
	}
	if e.profiles.submitCalls != 0 || e.profiles.resetCalls != 0 {
		t.Fatalf("未同意的請求不得觸及 repo：submit=%d reset=%d",
			e.profiles.submitCalls, e.profiles.resetCalls)
	}
	if len(e.audit.history) != 0 {
		t.Fatalf("未同意的請求不得留下狀態歷程：%+v", e.audit.history)
	}
}

// 🔴 陰性 ×「重申」入邊：REJECTED ＋ 冷卻已過 ＋ 未勾條款 ⇒ 9325，
// 且**連 T01 ResetToDraft 都不做**。
//
// 這一支是上一支抓不到的那一半：REJECTED 的路徑上 ResetToDraft 會真的改狀態，
// 判定若在交易內，賣家的 review_status 會先變 DRAFT 再 rollback ——最終值相同，
// 只有呼叫次數看得出差別。
func TestTaxProfile_Submit_UnacceptedTermsDoesNotResetRejected(t *testing.T) {
	row := profileAt("REJECTED")
	past := tNow.Add(-time.Hour)
	row.CooldownUntil = &past
	e := newTaxEnv(t, row)

	_, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: false,
	})
	wantBiz(t, err, bizcode.SellerSelfIssueTermsNotAccepted)

	if e.profiles.resetCalls != 0 {
		t.Fatalf("未同意條款卻已執行 T01 ResetToDraft（%d 次）——"+
			"9325 的判定被挪進交易內了", e.profiles.resetCalls)
	}
	if e.profiles.byIssuer[tIssuerA].ReviewStatus != "REJECTED" {
		t.Fatal("狀態不得改變")
	}
}

// T04 仍走得通（裁決 C 逐字：「T04 保留給補簽等例外路徑」）。
//
// 🔴 這不是多餘的重複：裁決把簽署搬到端點 19 之後，很容易以為 T04 就此死掉。
// 實際上端點 19 之後的列仍是 SUBMITTED（狀態值沒變），而 SUBMITTED 正是
// AttestSellerSelfIssue 的 allowed-from ⇒ 補簽路徑完整保留，且它是唯一
// 能把賣家推進 SELF_ISSUE_PENDING 的邊。
func TestTaxProfile_Submit_ThenAttestStillReachesSelfIssuePending(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))

	if _, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: true,
	}); err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	view, err := e.uc.AcceptAttestation(context.Background(), AcceptAttestationInput{
		StoreID: tStoreA, AcceptedByAccountID: tSeller,
		TermsVersion: SelfIssueTermsVersion, Accepted: true,
	})
	if err != nil {
		t.Fatalf("端點 19 之後 T04 應仍可用（補簽路徑）：%v", err)
	}
	if view.Profile.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("status = %s，want SELF_ISSUE_PENDING", view.Profile.ReviewStatus)
	}
}

// 端點 22：回送的條款版本必須是**當期**版本（58A §S04「不符後端回 400」）。
//
// 沒有這一句，賣家可以簽下一個平台從未發布過的版本字串，而那份記錄是
// 「審核期間由賣家自行開票、責任在賣家」的唯一書面憑據。
func TestTaxProfile_AcceptAttestation_RejectsStaleTermsVersion(t *testing.T) {
	e := newTaxEnv(t, profileAt("SUBMITTED"))

	_, err := e.uc.AcceptAttestation(context.Background(), AcceptAttestationInput{
		StoreID: tStoreA, TermsVersion: "2000-01-01", Accepted: true,
	})
	if !errors.Is(err, ErrTaxTermsVersionStale) {
		t.Fatalf("預期 ErrTaxTermsVersionStale，實際 %v", err)
	}
	if e.profiles.byIssuer[tIssuerA].AttestedAt != nil {
		t.Fatal("版本不符時不得寫入 attested_at")
	}
}

// ---------------------------------------------------------------------------
// 端點 20：賣家申請延長
// ---------------------------------------------------------------------------
// 稽核：端點 18／20 也要留下**是誰做的**（backend#3048）。
//
// 🔴 這不是「本設計不記 actor」而是同一份設計漏了兩支：同一張表、同一個 actor_type，
// 8 個 reason_code 裡 6 個記得到、2 個記不到（dev 實查 SELLER_TAX_DRAFT_CREATED 6 筆
// 全 NULL、SELLER_TAX_EXTENSION_REQUESTED 3 筆全 NULL，而 SELLER_TAX_SUBMITTED 全有）。
// 根因是那兩支的 proto request 根本沒有 actor 欄位，usecase 只好寫死 nil。
func TestTaxProfile_SaveDraft_RecordsActorOnCreation(t *testing.T) {
	const actor int64 = 5151
	e := newTaxEnv(t)
	delete(e.issuers.byRef, tStoreA) // 全新賣家 ⇒ 走 ensureProfile 的建列路徑

	if _, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0,
		EntityType: "COMPANY", TaxID: "12345678", LegalName: "範例生鮮股份有限公司",
		ResponsibleName: "王小明", AddressCity: "台北市", AddressDistrict: "信義區",
		AddressDetail:    "市府路1號",
		ClaimedStatus:    codemeta.SellerTaxObligationRequired.Value,
		SavedByAccountID: actor,
	}); err != nil {
		t.Fatalf("SaveDraft 失敗：%v", err)
	}
	if len(e.audit.history) == 0 {
		t.Fatal("建列沒留歷程")
	}
	got := e.audit.history[0].ActorAccountID
	if got == nil || *got != actor {
		t.Fatalf("SELLER_TAX_DRAFT_CREATED 的 actor_account_id = %v，want %d"+
			"——稅籍資料被誰改過是 5 年稽核軌跡的一部分", got, actor)
	}

	// 陰性：沒有可信身分時記 NULL，不得拿 store_id 頂替（賣家換手後那會指到別人）。
	e2 := newTaxEnv(t)
	delete(e2.issuers.byRef, tStoreA)
	if _, err := e2.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0,
		EntityType: "COMPANY", TaxID: "12345678", LegalName: "範例生鮮股份有限公司",
		ResponsibleName: "王小明", AddressCity: "台北市", AddressDistrict: "信義區",
		AddressDetail: "市府路1號",
		ClaimedStatus: codemeta.SellerTaxObligationRequired.Value,
	}); err != nil {
		t.Fatalf("SaveDraft 失敗：%v", err)
	}
	if len(e2.audit.history) > 0 && e2.audit.history[0].ActorAccountID != nil {
		t.Fatalf("沒有可信身分時應記 NULL，實得 %v", *e2.audit.history[0].ActorAccountID)
	}
}

func TestTaxProfile_RequestExtension_RecordsActor(t *testing.T) {
	const actor int64 = 5252
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))

	if _, err := e.uc.RequestExtension(context.Background(), RequestTaxExtensionInput{
		StoreID: tStoreA, Reason: "會計師還在準備文件", RequestedByAccountID: actor,
	}); err != nil {
		t.Fatalf("RequestExtension 失敗：%v", err)
	}
	if len(e.audit.history) != 1 {
		t.Fatalf("歷程筆數 = %d，want 1", len(e.audit.history))
	}
	got := e.audit.history[0].ActorAccountID
	if got == nil || *got != actor {
		t.Fatalf("SELLER_TAX_EXTENSION_REQUESTED 的 actor_account_id = %v，want %d"+
			"——延長把補件期限往後推最多 30 天，是誰推的必須查得到", got, actor)
	}
}

func TestTaxProfile_RequestExtension_RecordsHistoryWithoutStateChange(t *testing.T) {
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))
	before := e.profiles.byIssuer[tIssuerA]

	view, err := e.uc.RequestExtension(context.Background(), RequestTaxExtensionInput{
		StoreID: tStoreA, Reason: "會計師還在準備文件",
	})
	if err != nil {
		t.Fatalf("RequestExtension 失敗：%v", err)
	}
	// 🔴 它不改任何狀態：真正的延長是 ADM 的 T11。
	after := e.profiles.byIssuer[tIssuerA]
	if after.Version != before.Version || after.ExtensionCount != 0 ||
		!after.PendingValidUntil.Equal(*before.PendingValidUntil) {
		t.Fatalf("賣家申請不得改動 profile：%+v → %+v", before, after)
	}
	if len(e.audit.history) != 1 || e.audit.history[0].ReasonCode == nil ||
		*e.audit.history[0].ReasonCode != taxReasonExtensionRequested {
		t.Fatalf("申請沒有留痕，ADM 看不到這件事：%+v", e.audit.history)
	}
	if view.Profile.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("status = %s", view.Profile.ReviewStatus)
	}
}

// 陰性：已延長過 ⇒ 9323。UI 不渲染 CTA，但後端仍必須擋——UI 隱藏不是授權。
func TestTaxProfile_RequestExtension_AlreadyUsed(t *testing.T) {
	row := profileAt("SELF_ISSUE_PENDING")
	row.ExtensionCount = 1
	e := newTaxEnv(t, row)

	_, err := e.uc.RequestExtension(context.Background(), RequestTaxExtensionInput{
		StoreID: tStoreA, Reason: "再給我一次",
	})
	wantBiz(t, err, bizcode.SellerTaxExtensionAlreadyUsed)
}

// 陰性（狀態機）：UNDER_REVIEW 不是 EXTEND 的來源狀態 ⇒ 9321。
func TestTaxProfile_RequestExtension_StateInvalid(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	_, err := e.uc.RequestExtension(context.Background(), RequestTaxExtensionInput{
		StoreID: tStoreA, Reason: "想延長",
	})
	wantBiz(t, err, bizcode.SellerTaxProfileStateInvalid)
}

func TestTaxProfile_RequestExtension_RequiresReason(t *testing.T) {
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))
	_, err := e.uc.RequestExtension(context.Background(), RequestTaxExtensionInput{StoreID: tStoreA})
	if !errors.Is(err, ErrTaxReasonRequired) {
		t.Fatalf("預期 ErrTaxReasonRequired，實際 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 22：接受自行開票責任條款（T04）
// ---------------------------------------------------------------------------

func TestTaxProfile_AcceptAttestation_SubmittedToSelfIssuePending(t *testing.T) {
	e := newTaxEnv(t, profileAt("SUBMITTED"))
	before := e.profiles.byIssuer[tIssuerA]

	view, err := e.uc.AcceptAttestation(context.Background(), AcceptAttestationInput{
		StoreID: tStoreA, AcceptedByAccountID: tSeller, TermsVersion: SelfIssueTermsVersion, Accepted: true,
	})
	if err != nil {
		t.Fatalf("AcceptAttestation 失敗：%v", err)
	}
	if view.Profile.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("status = %s，want SELF_ISSUE_PENDING", view.Profile.ReviewStatus)
	}
	// 🔴 **不重算 pending_valid_until**：重算等於讓賣家用「停在未接受條款」把 60 天
	// 窗口無限延長（決策 C4 的單一時鐘）。
	if !view.Profile.PendingValidUntil.Equal(*before.PendingValidUntil) {
		t.Fatalf("pending_valid_until 被重算了：%v → %v", before.PendingValidUntil, view.Profile.PendingValidUntil)
	}
	// 稽核要能回答「是誰按的」——那是 account 層的事實。
	if len(e.audit.history) != 1 || e.audit.history[0].ActorAccountID == nil ||
		*e.audit.history[0].ActorAccountID != tSeller {
		t.Fatalf("條款接受人沒有留痕：%+v", e.audit.history)
	}
}

// 陰性（狀態機）：DRAFT 不是 T04 的來源狀態 ⇒ 9321，資料未改動。
func TestTaxProfile_AcceptAttestation_StateInvalid(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))
	before := e.profiles.byIssuer[tIssuerA]

	_, err := e.uc.AcceptAttestation(context.Background(), AcceptAttestationInput{
		StoreID: tStoreA, TermsVersion: SelfIssueTermsVersion, Accepted: true,
	})
	wantBiz(t, err, bizcode.SellerTaxProfileStateInvalid)
	after := e.profiles.byIssuer[tIssuerA]
	if after.Version != before.Version || after.AttestedAt != nil {
		t.Fatalf("守衛不成立時不得寫入 attested_at：%+v", after)
	}
}

func TestTaxProfile_AcceptAttestation_RejectsUnaccepted(t *testing.T) {
	e := newTaxEnv(t, profileAt("SUBMITTED"))
	_, err := e.uc.AcceptAttestation(context.Background(), AcceptAttestationInput{
		StoreID: tStoreA, TermsVersion: SelfIssueTermsVersion, Accepted: false,
	})
	if !errors.Is(err, ErrTaxTermsNotAccepted) {
		t.Fatalf("預期 ErrTaxTermsNotAccepted，實際 %v", err)
	}
	_, err = e.uc.AcceptAttestation(context.Background(), AcceptAttestationInput{
		StoreID: tStoreA, Accepted: true,
	})
	if !errors.Is(err, ErrTaxTermsVersionInvalid) {
		t.Fatalf("預期 ErrTaxTermsVersionInvalid，實際 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 40：ADM 裁決
// ---------------------------------------------------------------------------

// 相容表逐格驗：每一列必須逐字等於 §7.3.3 對應轉移的來源狀態欄。
// 多一個值 ⇒ CAS rows=0 靜默失敗；少一個值 ⇒ 合法決策被 9348 擋掉。
func TestTaxProfile_Decide_CompatibilityMatrix(t *testing.T) {
	statuses := []string{
		"DRAFT", "SUBMITTED", "UNDER_REVIEW", "SUPPLEMENT_REQUIRED", "SELF_ISSUE_PENDING",
		"APPROVED_INVOICE_REQUIRED", "APPROVED_INVOICE_EXEMPT", "REJECTED", "EXPIRED", "REVOKED",
	}
	for decision, allowed := range taxDecisionAllowedFrom {
		for _, st := range statuses {
			p := profileAt(st)
			// 🔴 EXEMPT 的 fixture 必須**沒有**統編（chk_stp_exempt_no_taxid）。
			// 這一行不是為了讓測試變綠：帶統編的 EXEMPT 在真實 DB 上是 23514，
			// 本矩陣要驗的是「decision × status 的相容性」，不是「帶統編也可以」。
			// 替身鏡射那條 CHECK 之後，這個 fixture 缺陷才第一次現形（#3089 B 組第 5 項）。
			if decision == codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value {
				p.TaxID = nil
				// 同上一段的理由：本矩陣驗的是 decision × status 的相容性。
				// DEC-20260905-015836 選 A 之後，EXEMPT 核定另有一道授權守衛（9354），
				// fixture 不帶授權的話每一格都會停在那裡，矩陣就不再驗相容性。
				// 授權守衛自己的陰陽兩面在 TestTaxProfile_Decide_ApproveExemptRequiresReceiptAuthorization。
				authorized := tNow.Add(-24 * time.Hour)
				consent := "v1"
				p.ReceiptAuthorizedAt, p.ReceiptConsentVersion = &authorized, &consent
			}
			e := newTaxEnv(t, p)
			_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
				ActorAccountID: tAdmin, StoreID: tStoreA, Decision: decision,
				Reason: "理由", Confirmed: true,
			})
			ok := containsString(allowed, st)
			if ok && err != nil {
				t.Fatalf("%s 在 %s 應該合法，卻回 %v", decision, st, err)
			}
			if !ok {
				wantBiz(t, err, bizcode.InvoiceTaxDecisionInvalid)
				if e.profiles.byIssuer[tIssuerA].ReviewStatus != st {
					t.Fatalf("%s 在 %s 被擋下後狀態仍被改動", decision, st)
				}
			}
		}
	}
}

// 陽性：APPROVE_INVOICE_REQUIRED 寫 approved_at ＋ approval_valid_until（預設 365 天），
// 並寫一筆稽核 ＋ 一則 P11。
func TestTaxProfile_Decide_ApproveWritesAllFourArtifacts(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))

	res, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
		Reason:   "文件齊全",
	})
	if err != nil {
		t.Fatalf("Decide 失敗：%v", err)
	}
	if res.ReviewStatus != "APPROVED_INVOICE_REQUIRED" {
		t.Fatalf("status = %s", res.ReviewStatus)
	}
	p := e.profiles.byIssuer[tIssuerA]
	if p.ApprovalValidUntil == nil || !p.ApprovalValidUntil.Equal(tNow.Add(taxApprovalValidity)) {
		t.Fatalf("approval_valid_until = %v，want %v", p.ApprovalValidUntil, tNow.Add(taxApprovalValidity))
	}
	if len(e.audit.admin) != 1 || res.AuditLogID == 0 {
		t.Fatalf("ADM 稽核沒寫（5 年保存義務）：%+v", e.audit.admin)
	}
	if len(e.audit.history) != 1 {
		t.Fatalf("狀態歷程沒寫：%+v", e.audit.history)
	}
	if len(e.outbox.events) != 1 {
		t.Fatalf("P11 沒發：%+v", e.outbox.events)
	}
}

// #3049：ADM 核定「應開統一發票」而賣家沒填統編 ⇒ 必須回業務碼，不得讓
// chk_stp_required_needs_taxid 變成第一道防線（23514 冒泡成 500，並把 relation 名、
// constraint 名與 SQLSTATE 外送給瀏覽器）。
//
// 這是 A03 的**正常結果之一**不是邊角：S02 表單逐字寫「免用統一發票的自然人賣家可留空」、
// A03 逐字寫「賣家自述不是平台的判定結果」⇒ 賣家照著填、ADM 照著判就會走到這裡。
func TestTaxProfile_Decide_ApproveRequiredWithoutTaxIDIsBusinessError(t *testing.T) {
	p := profileAt("SELF_ISSUE_PENDING")
	p.ClaimedObligation = codemeta.SellerTaxObligationExempt.Value
	p.TaxID = nil
	e := newTaxEnv(t, p)

	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
		Reason:   "查得到營業登記",
	})
	wantBiz(t, err, bizcode.InvoiceTaxApprovalIdentityIncomplete)
	if got := e.profiles.byIssuer[tIssuerA].ReviewStatus; got != "SELF_ISSUE_PENDING" {
		t.Fatalf("被擋下後狀態仍被改動：%s", got)
	}
}

// legal_name 與 tax_id 同在那條 CHECK 裡，守衛必須涵蓋兩者
// ——只擋一半，另一半照樣 500。
func TestTaxProfile_Decide_ApproveRequiredWithoutLegalNameIsBusinessError(t *testing.T) {
	p := profileAt("UNDER_REVIEW")
	p.LegalName = nil
	e := newTaxEnv(t, p)

	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
		Reason:   "文件齊全",
	})
	wantBiz(t, err, bizcode.InvoiceTaxApprovalIdentityIncomplete)
}

// 🔴 陽性對照：補上統編後**必須成功**轉 APPROVED_INVOICE_REQUIRED。
// 只驗「不再 500」分不清「修好了」與「整條路徑被關掉」。
func TestTaxProfile_Decide_ApproveRequiredSucceedsOnceIdentityComplete(t *testing.T) {
	p := profileAt("SELF_ISSUE_PENDING")
	p.ClaimedObligation = codemeta.SellerTaxObligationExempt.Value
	e := newTaxEnv(t, p) // profileAt 已帶 tax_id ＋ legal_name

	res, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
		Reason:   "查得到營業登記",
	})
	if err != nil {
		t.Fatalf("資料齊備仍被擋下：%v", err)
	}
	if res.ReviewStatus != "APPROVED_INVOICE_REQUIRED" {
		t.Fatalf("status = %s", res.ReviewStatus)
	}
}

// 🔴 陰性對照：同一位沒有統編的賣家按「核定免用統一發票」仍須成功。
// chk_stp_exempt_no_taxid 反過來**禁止** EXEMPT 帶統編 ⇒ 新守衛套到這條路上就是把
// 免用統一發票的賣家全部鎖死。
func TestTaxProfile_Decide_ApproveExemptWithoutTaxIDStillSucceeds(t *testing.T) {
	p := profileAt("SELF_ISSUE_PENDING")
	p.ClaimedObligation = codemeta.SellerTaxObligationExempt.Value
	p.TaxID = nil
	// 本測試的主題是「沒有統編也要核定得了」。DEC-20260905-015836 選 A 之後，EXEMPT
	// 核定另有一道授權守衛（9354）；不補這一對欄位的話它會停在那裡，
	// 訊息還會寫「不該被新守衛波及」而其實是被另一道新守衛擋的 —— 兩件事要分開。
	authorized := tNow.Add(-24 * time.Hour)
	consent := "v1"
	p.ReceiptAuthorizedAt, p.ReceiptConsentVersion = &authorized, &consent
	e := newTaxEnv(t, p)

	res, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision:  codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
		Reason:    "自然人賣家",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("EXEMPT 這條路不該被新守衛波及：%v", err)
	}
	if res.ReviewStatus != "APPROVED_INVOICE_EXEMPT" {
		t.Fatalf("status = %s", res.ReviewStatus)
	}
}

// REJECT 必寫 cooldown_until ＝ now + 7 天。少寫它，被駁回的賣家可以立刻無限重送。
func TestTaxProfile_Decide_RejectSetsCooldown(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	if _, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionReject.Value, Reason: "統編不符",
	}); err != nil {
		t.Fatalf("Decide 失敗：%v", err)
	}
	p := e.profiles.byIssuer[tIssuerA]
	if p.CooldownUntil == nil || !p.CooldownUntil.Equal(tNow.Add(taxReapplyCooldown)) {
		t.Fatalf("cooldown_until = %v，want %v", p.CooldownUntil, tNow.Add(taxReapplyCooldown))
	}
}

// T16 自轉移：狀態不變、**不動任何時鐘欄位**，而且**不發 P11**
// （事件叫 status_changed，發一則狀態沒變的出去，下游去重鍵會撞）。
func TestTaxProfile_Decide_KeepUnderReviewTouchesNoClock(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	before := e.profiles.byIssuer[tIssuerA]

	res, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionKeepUnderReview.Value, Reason: "等補充說明",
	})
	if err != nil {
		t.Fatalf("Decide 失敗：%v", err)
	}
	if res.ReviewStatus != "UNDER_REVIEW" {
		t.Fatalf("status = %s，want 不變", res.ReviewStatus)
	}
	after := e.profiles.byIssuer[tIssuerA]
	if !after.PendingValidUntil.Equal(*before.PendingValidUntil) || after.ExtensionCount != before.ExtensionCount {
		t.Fatal("T16 不得改動任何時鐘欄位")
	}
	if len(e.outbox.events) != 0 {
		t.Fatalf("自轉移不得發 status_changed：%+v", e.outbox.events)
	}
	if len(e.audit.admin) != 1 {
		t.Fatal("T16 的價值就是留痕，稽核必須寫")
	}
}

// REVOKE 需要二次確認：confirmed 缺席 ⇒ **400 而不是 9348**（§6.1.12 逐字）。
func TestTaxProfile_Decide_RevokeNeedsConfirmation(t *testing.T) {
	e := newTaxEnv(t, profileAt("APPROVED_INVOICE_REQUIRED"))
	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionRevoke.Value, Reason: "資格不符",
	})
	if !errors.Is(err, ErrTaxConfirmRequired) {
		t.Fatalf("預期 ErrTaxConfirmRequired，實際 %v", err)
	}
	if e.profiles.byIssuer[tIssuerA].ReviewStatus != "APPROVED_INVOICE_REQUIRED" {
		t.Fatal("未二次確認時不得撤銷")
	}

	res, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionRevoke.Value, Reason: "資格不符", Confirmed: true,
	})
	if err != nil {
		t.Fatalf("二次確認後應成功：%v", err)
	}
	// 已代製收據不追溯，但要留分界時點（T-C04）。
	if res.RevokedEffectiveAt == nil || !res.RevokedEffectiveAt.Equal(tNow) {
		t.Fatalf("revokedEffectiveAt = %v，want %v", res.RevokedEffectiveAt, tNow)
	}
}

// APPROVE_INVOICE_EXEMPT 同樣需要二次確認。
func TestTaxProfile_Decide_ApproveExemptNeedsConfirmation(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value, Reason: "免用統一發票",
	})
	if !errors.Is(err, ErrTaxConfirmRequired) {
		t.Fatalf("預期 ErrTaxConfirmRequired，實際 %v", err)
	}
}

func TestTaxProfile_Decide_UnknownDecisionAndMissingReason(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	// 值域外的字串是契約違規（400），不是 9348。
	if _, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "APPROVE_TAXABLE", Reason: "x",
	}); !errors.Is(err, ErrTaxDecisionUnknown) {
		t.Fatalf("預期 ErrTaxDecisionUnknown，實際 %v", err)
	}
	// reason 必填是授權設計的一部分（B4），不是 UX。
	if _, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionRequestSupplement.Value,
	}); !errors.Is(err, ErrTaxReasonRequired) {
		t.Fatalf("預期 ErrTaxReasonRequired，實際 %v", err)
	}
}

func TestTaxProfile_Decide_VersionConflict(t *testing.T) {
	row := profileAt("UNDER_REVIEW")
	row.Version = 7
	e := newTaxEnv(t, row)

	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionRequestSupplement.Value,
		Reason:   "請補件", ExpectedVersion: 6,
	})
	wantBiz(t, err, bizcode.SellerTaxProfileVersionConflict)
	if e.profiles.byIssuer[tIssuerA].ReviewStatus != "UNDER_REVIEW" {
		t.Fatal("版本衝突時不得寫入")
	}
}

// ---------------------------------------------------------------------------
// 端點 41：ADM 延長裁決（T11）
// ---------------------------------------------------------------------------

// 陽性：APPROVE 就地改寫 pending_valid_until ＋ extension_count = 1，**狀態不變**。
// 基準是**原到期日**不是 now：以 now 起算會讓快到期的案子被延成完整的 30 天。
func TestTaxProfile_DecideExtension_Approve(t *testing.T) {
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))
	before := e.profiles.byIssuer[tIssuerA]

	res, err := e.uc.DecideExtension(context.Background(), TaxExtensionDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "APPROVE", Reason: "賣家已提供進度",
	})
	if err != nil {
		t.Fatalf("DecideExtension 失敗：%v", err)
	}
	if res.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("T11 不得改變狀態，實際 %s", res.ReviewStatus)
	}
	p := e.profiles.byIssuer[tIssuerA]
	want := before.PendingValidUntil.AddDate(0, 0, taxExtensionDefaultDays)
	if p.PendingValidUntil == nil || !p.PendingValidUntil.Equal(want) {
		t.Fatalf("pending_valid_until = %v，want %v（原到期日 + 30 天）", p.PendingValidUntil, want)
	}
	if p.ExtensionCount != 1 {
		t.Fatalf("extension_count = %d，want 1", p.ExtensionCount)
	}
	// decision 歷程要記成 EXTEND（SellerTaxReviewDecision 的第 8 值）：
	// extensionCount 只說得出「有沒有」，說不出「誰在什麼時候用什麼理由准的」。
	if len(e.audit.admin) != 1 || e.audit.admin[0].ReasonCode != codemeta.SellerTaxReviewDecisionExtend.Value {
		t.Fatalf("稽核的 reason_code = %+v，want EXTEND", e.audit.admin)
	}
}

// 陰性：已延長過 ⇒ 9323，且期限未被改動。
func TestTaxProfile_DecideExtension_AlreadyUsed(t *testing.T) {
	row := profileAt("SELF_ISSUE_PENDING")
	row.ExtensionCount = 1
	e := newTaxEnv(t, row)
	before := *row.PendingValidUntil

	_, err := e.uc.DecideExtension(context.Background(), TaxExtensionDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "APPROVE", Reason: "再延一次",
	})
	wantBiz(t, err, bizcode.SellerTaxExtensionAlreadyUsed)
	if !e.profiles.byIssuer[tIssuerA].PendingValidUntil.Equal(before) {
		t.Fatal("守衛不成立時期限不得改動")
	}
}

// 陰性（狀態機）：UNDER_REVIEW 不是 T11 的來源狀態 ⇒ 9321。
func TestTaxProfile_DecideExtension_StateInvalid(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	_, err := e.uc.DecideExtension(context.Background(), TaxExtensionDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "APPROVE", Reason: "延長",
	})
	wantBiz(t, err, bizcode.SellerTaxProfileStateInvalid)
}

// REJECT 不改狀態，但**仍要寫 invoice_admin_audit_logs**（§6.1.12 逐字）。
func TestTaxProfile_DecideExtension_RejectStillAudits(t *testing.T) {
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))
	before := e.profiles.byIssuer[tIssuerA]

	res, err := e.uc.DecideExtension(context.Background(), TaxExtensionDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "REJECT", Reason: "理由不足",
	})
	if err != nil {
		t.Fatalf("DecideExtension 失敗：%v", err)
	}
	if res.ReviewStatus != "SELF_ISSUE_PENDING" {
		t.Fatalf("REJECT 不得改狀態，實際 %s", res.ReviewStatus)
	}
	after := e.profiles.byIssuer[tIssuerA]
	if after.Version != before.Version || after.ExtensionCount != 0 {
		t.Fatal("REJECT 不得改動 profile")
	}
	if len(e.audit.admin) != 1 {
		t.Fatal("駁回延長申請本身就是需要留痕的 ADM 行為")
	}
	if len(e.audit.history) != 0 {
		t.Fatalf("REJECT 不寫決策歷程：%+v", e.audit.history)
	}
}

func TestTaxProfile_DecideExtension_InvalidDays(t *testing.T) {
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))
	_, err := e.uc.DecideExtension(context.Background(), TaxExtensionDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "APPROVE",
		ExtendDays: 400, Reason: "延一年多",
	})
	if !errors.Is(err, ErrTaxExtendDaysInvalid) {
		t.Fatalf("預期 ErrTaxExtendDaysInvalid，實際 %v", err)
	}
}

// ---------------------------------------------------------------------------
// CAS 的 rows=0：讀與寫之間被搶先改掉
// ---------------------------------------------------------------------------

// TestTaxProfile_Decide_CASRowsZeroIsNotSuccess 守住「rows=0 不得當成功」。
//
// 🔴 Go 端的相容表是**第一道**防線，它在正常路徑上就把不相容的決策擋掉 ⇒ CAS 的
// !ok 分支只有在「另一個 pod 在同一瞬間改掉了狀態」時才走得到。本測試用 casFails
// 直接構造那一格。
//
// 少了它，把 taxCASResult 的 !ok 改成 `return nil`（靜默失敗）**整組測試照樣綠**，
// 而線上的症狀是：ADM 按下裁決、看到成功、狀態沒變、稽核卻記了一筆「已核准」。
func TestTaxProfile_Decide_CASRowsZeroIsNotSuccess(t *testing.T) {
	e := newTaxEnv(t, profileAt("UNDER_REVIEW"))
	e.profiles.casFails = true

	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision: codemeta.SellerTaxReviewDecisionRequestSupplement.Value, Reason: "請補件",
	})
	wantBiz(t, err, bizcode.InvoiceTaxDecisionInvalid)
	// 整筆必須回滾的語意：稽核與 outbox 都不得留下痕跡。
	if len(e.audit.admin) != 0 || len(e.outbox.events) != 0 {
		t.Fatalf("CAS 失敗時不得寫稽核／事件：admin=%d outbox=%d", len(e.audit.admin), len(e.outbox.events))
	}
}

// 同一形狀套在賣家側：T02 的 CAS rows=0 必須回 9321，而不是回一個「送出成功」。
func TestTaxProfile_Submit_CASRowsZeroIsNotSuccess(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))
	e.profiles.casFails = true

	_, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{StoreID: tStoreA, AcceptedTerms: true})
	wantBiz(t, err, bizcode.SellerTaxProfileStateInvalid)
	if len(e.audit.history) != 0 {
		t.Fatalf("CAS 失敗時不得寫狀態歷程：%+v", e.audit.history)
	}
}

// T11 的 CAS rows=0（extension_count 被另一個 ADM 搶先寫成 1）必須回 9323。
func TestTaxProfile_DecideExtension_CASRowsZeroIsNotSuccess(t *testing.T) {
	e := newTaxEnv(t, profileAt("SELF_ISSUE_PENDING"))
	e.profiles.casFails = true

	_, err := e.uc.DecideExtension(context.Background(), TaxExtensionDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA, Decision: "APPROVE", Reason: "延長",
	})
	wantBiz(t, err, bizcode.SellerTaxExtensionAlreadyUsed)
	if len(e.audit.admin) != 0 {
		t.Fatalf("CAS 失敗時不得寫稽核：%+v", e.audit.admin)
	}
}

// #3031：條款是**自然人的意思表示**（§6.1.7）——只有版本與時間、沒有人，
// 爭議時舉不出簽署主體。三元組必須同一句寫（真實 SQL 就是一句）。
func TestSubmit_RecordsAttestedByAccountID(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))

	res, err := e.uc.Submit(context.Background(), SubmitTaxProfileInput{
		StoreID: tStoreA, AcceptedTerms: true, AcceptedByAccountID: 987654321,
	})
	if err != nil {
		t.Fatalf("Submit 失敗：%v", err)
	}
	p := res.View.Profile
	if p.AttestedByAccountID == nil || *p.AttestedByAccountID != 987654321 {
		t.Fatalf("🔴 簽署人沒落庫：%v —— 端點 21 的 acceptedByAccountId 會是空的", p.AttestedByAccountID)
	}
	// 三元組要一起：只有人沒有版本／時間，同樣舉不出憑據。
	if p.AttestationVersion == nil || p.AttestedAt == nil {
		t.Fatalf("三元組沒有一起寫：version=%v at=%v", p.AttestationVersion, p.AttestedAt)
	}
}

// ---------------------------------------------------------------------------
// #3089 B 組：DB CHECK 由 API 觸發 500 —— 應用層沒守，交給 DB 擋
//
// 這個類的判準（票裡的機械形式）：對每條 CHECK 構造一個**合法形狀**的 API 請求
// 去違反它，斷言回業務碼／400 而不是 500。
// ---------------------------------------------------------------------------

// B 組第 4 項：賣家自填統編零格式驗證（chk_stp_tax_id）。
//
// 買家側的同型欄位一直都走 domain.IsBusinessTaxID ⇒ 兩側標準不一致，
// 而不一致的那一側是 **HTTP 500**。
func TestTaxProfile_SaveDraft_RejectsMalformedTaxID(t *testing.T) {
	for _, tc := range []struct{ name, taxID string }{
		{"七碼", "1234567"},
		{"九碼", "123456789"},
		{"英文字母", "ABCDEFGH"},
		{"含空白", "1234 678"},
		{"全形數字", "１２３４５６７８"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTaxEnv(t, profileAt("DRAFT"))
			_, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
				StoreID: tStoreA, ExpectedVersion: 0,
				EntityType: "COMPANY", TaxID: tc.taxID, LegalName: "範例生鮮股份有限公司",
				ClaimedStatus: codemeta.SellerTaxObligationRequired.Value,
			})
			if !errors.Is(err, ErrTaxIDInvalid) {
				t.Fatalf("taxID=%q 應回 ErrTaxIDInvalid（→400），實得 %v —— "+
					"少了這道守衛就是 chk_stp_tax_id 的 23514 冒泡成 500", tc.taxID, err)
			}
		})
	}
}

// 🔴 陽性對照：合法統編與**空字串**都必須放行。
// 只驗「非法會被擋」分不清「修好了」與「這個欄位再也存不進去」——
// 空字串是合法輸入（還沒填／清掉重填，chk_stp_tax_id 允許 NULL）。
func TestTaxProfile_SaveDraft_AcceptsValidAndEmptyTaxID(t *testing.T) {
	for _, tc := range []struct{ name, taxID string }{
		{"合法八碼", "12345678"},
		{"空字串＝還沒填", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTaxEnv(t, profileAt("DRAFT"))
			if _, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
				StoreID: tStoreA, ExpectedVersion: 0,
				EntityType: "COMPANY", TaxID: tc.taxID, LegalName: "範例生鮮股份有限公司",
				ClaimedStatus: codemeta.SellerTaxObligationRequired.Value,
			}); err != nil {
				t.Fatalf("taxID=%q 應被接受，實得 %v", tc.taxID, err)
			}
		})
	}
}

// B 組第 5 項：APPROVE_INVOICE_EXEMPT 沒有守衛（chk_stp_exempt_no_taxid）。
//
// **#3049 的鏡像另一半**：那一次補了 REQUIRED 缺統編這一格，而對稱的
// 「EXEMPT 卻帶著統編」原封不動地留著，可達性逐字對稱。
func TestTaxProfile_Decide_ApproveExemptWithTaxIDIsBusinessError(t *testing.T) {
	p := profileAt("UNDER_REVIEW") // profileAt 已帶 tax_id
	e := newTaxEnv(t, p)

	_, err := e.uc.Decide(context.Background(), TaxDecisionInput{
		ActorAccountID: tAdmin, StoreID: tStoreA,
		Decision:  codemeta.SellerTaxReviewDecisionApproveInvoiceExempt.Value,
		Reason:    "查得到免用統一發票核准",
		Confirmed: true,
	})
	wantBiz(t, err, bizcode.InvoiceTaxDecisionInvalid)
	if got := e.profiles.byIssuer[tIssuerA].ReviewStatus; got != "UNDER_REVIEW" {
		t.Fatalf("被擋下後狀態仍被改動：%s", got)
	}
}

// 🔴 陽性對照在 TestTaxProfile_Decide_ApproveExemptWithoutTaxIDStillSucceeds
// （沒有統編的賣家核准免開必須成功）——那一支比本節早，不重寫第二份。
// 兩支合起來才分得出「守衛正確」與「EXEMPT 這條路被整條關掉」。

// #3089 稽核 A-4：22001（欄位超長）與 CodeMeta 值域的守衛。
//
// 與 B 組三項同一個缺陷類，只是 SQLSTATE 不同（22001 vs 23514）——症狀一模一樣：
// SQLSTATE 冒泡成 HTTP 500，賣家看到「系統錯誤」而不是可以照做的下一步。
// 在此之前這四欄一個都沒守，只有 legal_name(120) 被守著。
func TestTaxProfile_SaveDraft_GuardsFieldLengthsAndEntityType(t *testing.T) {
	long := func(n int) string { return strings.Repeat("字", n) }
	for _, tc := range []struct {
		name string
		mut  func(*SaveTaxDraftInput)
		want error
	}{
		{"entityType 超過 32 字", func(i *SaveTaxDraftInput) { i.EntityType = long(33) }, ErrTaxEntityTypeInvalid},
		{"entityType 不在值域內", func(i *SaveTaxDraftInput) { i.EntityType = "SOMETHING" }, ErrTaxEntityTypeInvalid},
		{"負責人姓名超過 60 字", func(i *SaveTaxDraftInput) { i.ResponsibleName = long(61) }, ErrTaxResponsibleNameTooLong},
		{"縣市超過 32 字", func(i *SaveTaxDraftInput) { i.AddressCity = long(33) }, ErrTaxAddressPartTooLong},
		{"行政區超過 32 字", func(i *SaveTaxDraftInput) { i.AddressDistrict = long(33) }, ErrTaxAddressPartTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTaxEnv(t, profileAt("DRAFT"))
			in := SaveTaxDraftInput{
				StoreID: tStoreA, ExpectedVersion: 0,
				EntityType: "COMPANY", TaxID: "12345678", LegalName: "範例生鮮股份有限公司",
				ClaimedStatus: codemeta.SellerTaxObligationRequired.Value,
			}
			tc.mut(&in)
			if _, err := e.uc.SaveDraft(context.Background(), in); !errors.Is(err, tc.want) {
				t.Fatalf("期望 %v，實得 %v —— 少了這道守衛就是 22001 冒泡成 500", tc.want, err)
			}
		})
	}
}

// 🔴 陽性對照：邊界值與空字串都必須放行。
// 只驗「超長會被擋」分不清「修好了」與「這些欄位再也存不進去」。
func TestTaxProfile_SaveDraft_AcceptsBoundaryLengths(t *testing.T) {
	e := newTaxEnv(t, profileAt("DRAFT"))
	if _, err := e.uc.SaveDraft(context.Background(), SaveTaxDraftInput{
		StoreID: tStoreA, ExpectedVersion: 0,
		EntityType:      "COMPANY", // 值域內
		TaxID:           "12345678",
		LegalName:       "範例生鮮股份有限公司",
		ResponsibleName: strings.Repeat("字", 60), // 剛好 60
		AddressCity:     strings.Repeat("字", 32), // 剛好 32
		AddressDistrict: "",                      // 空字串＝還沒填
		ClaimedStatus:   codemeta.SellerTaxObligationRequired.Value,
	}); err != nil {
		t.Fatalf("邊界值應被接受，實得 %v", err)
	}
}

// #3089 第 11 項：admNotes[].createdAt 讀整列 updated_at。
//
// 推動 updated_at 的是 BumpSellerTaxProfilePendingOrderCounters ——
// **每一筆為該賣家建 document 的已付款訂單**都會打到它。
// 症狀是 A03 上那則備註的時間隨接單量一直往前跳，而備註一個字都沒改。
// #3051 是同一個形狀（credential_updated_at 被健康檢查每分鐘改寫）。
//
// 🔴 這一支驗的是 **SQL 的述詞**（review_note IS DISTINCT FROM …）而不是
// Go 的欄位搬運：真正的保證在那一句 CASE WHEN 上，Go 這邊只是讀它。
func TestSellerTaxProfileSQL_ReviewNoteAtOnlyMovesWhenNoteChanges(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "db", "queries", "seller_tax_profiles.sql"))
	if err != nil {
		t.Fatalf("讀不到 SQL（本測試已經在守一個不存在的東西）：%v", err)
	}
	src := string(raw)

	// 每一支寫 review_note 的 UPDATE 都必須同時推進 review_note_at。
	noteWrites := strings.Count(src, "review_note = sqlc.narg(review_note)")
	noteAtWrites := strings.Count(src, "review_note_at = CASE WHEN review_note IS DISTINCT FROM")
	if noteWrites == 0 {
		t.Fatal("一處 review_note 寫入都沒找到 —— SQL 的寫法變了，本測試已失效")
	}
	if noteAtWrites != noteWrites {
		t.Fatalf("寫 review_note 的地方有 %d 處，而推進 review_note_at 的只有 %d 處 —— "+
			"漏掉的那幾支會讓 A03 顯示一個永遠不動（或跟著別人跳）的備註時間",
			noteWrites, noteAtWrites)
	}

	// 🔴 述詞必須是 IS DISTINCT FROM 而不是無條件 now：無條件寫的話，
	// 同值重寫也算一次新備註 ⇒ 時間仍會隨無關的審核動作往前跳。
	if strings.Contains(src, "review_note_at = sqlc.arg(now)") {
		t.Error("review_note_at 被無條件寫成 now —— 那會讓同值重寫也算一次新備註")
	}

	// 陰性對照：接單累加那一支**不得**碰 review_note_at。
	// 它正是本缺陷的推動者，跟著它走等於什麼都沒修。
	i := strings.Index(src, "-- name: BumpSellerTaxProfilePendingOrderCounters")
	if i < 0 {
		t.Fatal("找不到 BumpSellerTaxProfilePendingOrderCounters —— 對照組失效")
	}
	stmt := src[i:]
	if j := strings.Index(stmt[10:], "-- name: "); j >= 0 {
		stmt = stmt[:j+10]
	}
	if strings.Contains(stmt, "review_note_at") {
		t.Error("接單累加那一支碰了 review_note_at —— 它是本缺陷的推動者，" +
			"跟著它走等於什麼都沒修")
	}
}
