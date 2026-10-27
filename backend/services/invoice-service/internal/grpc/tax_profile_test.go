package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 本檔驗 M3 寫入面的**型別邊界與 Tier A 出口形狀**；業務規則本身在
// internal/usecase/tax_profile_test.go。
//
// 🔴 核心斷言是「越權與查無的回應逐位元相同」，而且用 proto.Marshal 比對不是比 code：
// 比 code 只證明兩者同碼，證明不了 message、data 或任何未來新增的欄位沒有洩漏
// 「這家店有稅務資格，只是不是你的」。稅務資格帶統編、法定名稱、負責人與營業地址，
// 光是「存在與否」就是可枚舉的商業情報。

const (
	yStoreA int64 = 2685914682
	yStoreB int64 = 2632049831
	yIssuer int64 = 5201
	yProfID int64 = 7201
	yAdmin  int64 = 9101
)

var yNow = time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// fakes（只到 repository 邊界：本檔驗 handler，不驗 SQL）
// ---------------------------------------------------------------------------

type yIssuers struct {
	domain.IssuerRepository
	ref int64
}

func (r yIssuers) GetByRef(_ context.Context, _ string, refID int64) (domain.Issuer, error) {
	if refID != r.ref {
		// repository 的 notFound 對 issuer 映的是 9330（provider 語意）；
		// usecase 必須把它翻成 9320——那正是本檔要驗的其中一件事。
		// message 一律取自 bizcode.Message：repository 的 notFound() 就是這樣做的，
		// fake 自己打一個字串會讓 Tier A 的逐位元比對因為**測試替身**而失敗（或更糟：
		// 因為兩邊都寫同一個假字串而假綠）。
		return domain.Issuer{}, pkgerrors.NewBusiness(bizcode.InvoiceProviderAccountNotFound,
			bizcode.Message(bizcode.InvoiceProviderAccountNotFound))
	}
	return domain.Issuer{ID: yIssuer, IssuerType: "SELLER", IssuerRefID: refID}, nil
}

type yProfiles struct {
	domain.SellerTaxProfileRepository
	row     domain.SellerTaxProfile
	present bool
}

// ResetToDraft 在本檔一律回 false（冷卻未到）：handler 測試只驗出口形狀，
// 而 T01 的述詞邊界由 usecase 與 integration 測試守。
func (r yProfiles) ResetToDraft(context.Context, int64, time.Time) (bool, error) {
	return false, nil
}

// Revoke／RequestSupplement 在本檔一律回 true：handler 測試驗的是出口形狀，
// CAS 述詞由 usecase 與 integration 測試守。
func (r yProfiles) Revoke(context.Context, int64, int64, string, time.Time) (bool, error) {
	return true, nil
}

func (r yProfiles) RequestSupplement(context.Context, int64, int64, string, time.Time) (bool, error) {
	return true, nil
}

func (r yProfiles) GetByIssuer(context.Context, int64) (domain.SellerTaxProfile, error) {
	if !r.present {
		return domain.SellerTaxProfile{}, pkgerrors.NewBusiness(bizcode.SellerTaxProfileNotFound,
			bizcode.Message(bizcode.SellerTaxProfileNotFound))
	}
	return r.row, nil
}

type yAudit struct{ domain.AuditRepository }

func (yAudit) AppendStatusHistory(context.Context, domain.NewStatusHistoryEntry) error { return nil }
func (yAudit) AppendAdminAuditLog(context.Context, domain.NewAdminAuditLogEntry) (int64, error) {
	return 42, nil
}

type yOutbox struct{ domain.OutboxRepository }

func (yOutbox) Insert(context.Context, domain.NewOutboxEvent) (domain.OutboxEvent, error) {
	return domain.OutboxEvent{}, nil
}

type yEvidence struct{}

func (yEvidence) FinalizeEvidence(context.Context, []string, int64) ([]string, []string, error) {
	return nil, nil, nil
}
func (yEvidence) ReleaseEvidence(context.Context, int64, []string) error { return nil }

type yTx struct{}

func (yTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// newTaxServer 組出只掛了 TaxProfile 的 InvoiceServer。
//
// present=false 讓「這個 store 有 issuer 但沒有 profile」與「連 issuer 都沒有」
// 兩條路徑都走得到——它們必須回同一個東西。
func newTaxServer(present bool, row domain.SellerTaxProfile) *InvoiceServer {
	uc := usecase.NewTaxProfile(
		yTx{}, yIssuers{ref: yStoreA}, yProfiles{row: row, present: present},
		yAudit{}, yOutbox{}, yEvidence{},
		func() int64 { return 99001 }, nil,
	).WithClock(func() time.Time { return yNow })
	return NewInvoiceServer(nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func yProfile(status string) domain.SellerTaxProfile {
	entity, legal, taxID := "COMPANY", "範例生鮮股份有限公司", "12345678"
	person, addr := "王小明", "台北市信義區市府路1號"
	deadline := yNow.Add(30 * 24 * time.Hour)
	submitted := yNow.Add(-30 * 24 * time.Hour)
	return domain.SellerTaxProfile{
		ID: yProfID, IssuerID: yIssuer, ReviewStatus: status,
		ClaimedObligation: codemeta.SellerTaxObligationRequired.Value,
		EntityTaxType:     &entity, LegalName: &legal, TaxID: &taxID,
		ResponsiblePerson: &person, BusinessAddress: &addr,
		DefaultTaxCategory: 1,
		EvidenceObjectKeys: json.RawMessage(`["secure/certification/1/a.pdf"]`),
		SubmittedAt:        &submitted, PendingValidUntil: &deadline,
		PendingOrderLimitCount: 50, PendingOrderLimitAmount: 100000,
	}
}

// ---------------------------------------------------------------------------
// Tier A：越權與查無逐位元相同
// ---------------------------------------------------------------------------

// TestTaxProfile_TierA_ForeignStoreIsByteIdenticalToMissing 驗四支賣家 RPC。
//
// 「越權」＝ 打別人的 store_id（yStoreB，解不出 issuer）；
// 「查無」＝ 自己的 store_id 但還沒有 profile 列（present=false）。
// 兩者必須回同一個 9320 envelope，因為判定發生在 SQL 的 WHERE，本層沒有第二個分支。
func TestTaxProfile_TierA_ForeignStoreIsByteIdenticalToMissing(t *testing.T) {
	cases := []struct {
		name string
		call func(*InvoiceServer, int64) (proto.Message, int32, error)
	}{
		{"SubmitSellerTaxProfile", func(s *InvoiceServer, store int64) (proto.Message, int32, error) {
			// AcceptedTerms 必帶：9325 的判定在 Tier A 之前（裁決 C，它保證未同意的
			// 請求零寫入）⇒ 不帶就永遠測不到這裡要驗的 9320。
			// 兩者都是 store 無關的碼，先後順序不洩漏任何東西。
			r, err := s.SubmitSellerTaxProfile(context.Background(), &pb.SubmitSellerTaxProfileRequest{
				StoreId: store, AcceptedTerms: true,
			})
			return r, r.GetCode(), err
		}},
		{"RequestSellerTaxExtension", func(s *InvoiceServer, store int64) (proto.Message, int32, error) {
			r, err := s.RequestSellerTaxExtension(context.Background(), &pb.RequestSellerTaxExtensionRequest{
				StoreId: store, Reason: "需要更多時間",
			})
			return r, r.GetCode(), err
		}},
		{"AcceptSelfIssueAttestation", func(s *InvoiceServer, store int64) (proto.Message, int32, error) {
			r, err := s.AcceptSelfIssueAttestation(context.Background(), &pb.AcceptSelfIssueAttestationRequest{
				StoreId: store, TermsVersion: usecase.SelfIssueTermsVersion, Accepted: true,
			})
			return r, r.GetCode(), err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 越權：store 存在於請求，但不是這台 server 認得的那一家。
			foreignSrv := newTaxServer(true, yProfile("SUBMITTED"))
			foreign, code, err := tc.call(foreignSrv, yStoreB)
			if err != nil {
				t.Fatalf("越權應回 envelope 不是 gRPC error：%v", err)
			}
			if code != bizcode.SellerTaxProfileNotFound {
				t.Fatalf("code = %d, want 9320", code)
			}
			// 查無：自己的 store，但還沒有 profile。
			missingSrv := newTaxServer(false, domain.SellerTaxProfile{})
			missing, _, err := tc.call(missingSrv, yStoreA)
			if err != nil {
				t.Fatalf("查無應回 envelope 不是 gRPC error：%v", err)
			}

			a, err := proto.Marshal(foreign)
			if err != nil {
				t.Fatal(err)
			}
			b, err := proto.Marshal(missing)
			if err != nil {
				t.Fatal(err)
			}
			if string(a) != string(b) {
				t.Fatalf("越權與查無的回應不同（洩漏了「這家店有沒有稅務資格」）：\n越權 %+v\n查無 %+v",
					foreign, missing)
			}
		})
	}
}

// ADM 兩支同理：store_id 是 path param，查無一律 9320。
func TestTaxProfile_TierA_AdminMissingStore(t *testing.T) {
	s := newTaxServer(false, domain.SellerTaxProfile{})

	dec, err := s.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
		ActorAccountId: yAdmin, StoreId: "2685914682",
		Decision: codemeta.SellerTaxReviewDecisionRequestSupplement.Value, Reason: "請補件",
	})
	if err != nil {
		t.Fatalf("查無應回 envelope：%v", err)
	}
	ext, err := s.DecideTaxExtensionForAdmin(context.Background(), &pb.DecideTaxExtensionForAdminRequest{
		ActorAccountId: yAdmin, StoreId: "2685914682", Decision: "APPROVE", Reason: "延長",
	})
	if err != nil {
		t.Fatalf("查無應回 envelope：%v", err)
	}
	a, _ := proto.Marshal(dec)
	b, _ := proto.Marshal(ext)
	if string(a) != string(b) || dec.GetCode() != bizcode.SellerTaxProfileNotFound {
		t.Fatalf("兩支 ADM RPC 的查無出口不一致：%+v / %+v", dec, ext)
	}
}

// ---------------------------------------------------------------------------
// 狀態機陰性：回明確業務碼（envelope），不是 gRPC error
// ---------------------------------------------------------------------------

func TestTaxProfile_StateInvalidReturnsEnvelope(t *testing.T) {
	// 端點 18：SUBMITTED 不得就地改內容。
	s := newTaxServer(true, yProfile("SUBMITTED"))
	resp, err := s.SaveSellerTaxProfileDraft(context.Background(), &pb.SaveSellerTaxProfileDraftRequest{
		StoreId: yStoreA, TaxIdentity: &pb.SellerTaxIdentity{LegalName: "改名"},
	})
	if err != nil {
		t.Fatalf("業務錯誤必須走 envelope（GetCode 是 Gateway 唯一的判定依據）：%v", err)
	}
	if resp.GetCode() != bizcode.SellerTaxProfileStateInvalid {
		t.Fatalf("code = %d, want 9321", resp.GetCode())
	}
	if resp.GetData() != nil {
		t.Fatalf("9321 不在 preserveData 清單內，不得帶 data：%+v", resp.GetData())
	}

	// 端點 40：DRAFT 不是任何決策的來源狀態 ⇒ 9348。
	s2 := newTaxServer(true, yProfile("DRAFT"))
	dec, err := s2.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
		ActorAccountId: yAdmin, StoreId: "2685914682",
		Decision: codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value, Reason: "核准",
	})
	if err != nil {
		t.Fatalf("業務錯誤必須走 envelope：%v", err)
	}
	if dec.GetCode() != bizcode.InvoiceTaxDecisionInvalid {
		t.Fatalf("code = %d, want 9348", dec.GetCode())
	}
}

// 端點 19 的 9324 是全檔唯一一個「非零碼 ＋ data」的出口（preserveData／HTTP 200）。
//
// 🔴 沒有 data，gateway 的 response.Biz 會送出一個空 data，前端顯示不出倒數，
// 而賣家什麼都沒做錯。
func TestTaxProfile_Submit_CooldownCarriesData(t *testing.T) {
	row := yProfile("REJECTED")
	future := yNow.Add(72 * time.Hour)
	row.CooldownUntil = &future
	s := newTaxServer(true, row)

	resp, err := s.SubmitSellerTaxProfile(context.Background(), &pb.SubmitSellerTaxProfileRequest{
		StoreId: yStoreA, AcceptedTerms: true,
	})
	if err != nil {
		t.Fatalf("冷卻不是錯誤：%v", err)
	}
	if resp.GetCode() != bizcode.SellerTaxReapplyCooldown {
		t.Fatalf("code = %d, want 9324", resp.GetCode())
	}
	if resp.GetData() == nil || resp.GetData().GetReapplyCooldownUntil() == "" {
		t.Fatalf("9324 必須帶 data.reapplyCooldownUntil：%+v", resp.GetData())
	}
}

// 裁決 C：未勾條款 ⇒ **9325 envelope**（HTTP 409），不是 gRPC error。
//
// 🔴 走 envelope 不走 gRPC error 是有後果的選擇：EnvelopeGuardInterceptor 只看
// GetCode()，回 gRPC error 的會被 gateway 變成 5xx，而「你還沒同意條款」不是故障
// ——賣家看到「系統錯誤」就不會去點條款連結，58A §S02 要求的「就地 alert ＋ 連 S04」
// 也就無從觸發。
//
// 同時驗它**不是** InvalidArgument：400 在本檔的語意是「契約違規」（缺欄／值域／長度），
// accepted_terms=false 是一個格式完整、語意明確的商業回答。
func TestTaxProfile_Submit_UnacceptedTermsIsEnvelope9325(t *testing.T) {
	s := newTaxServer(true, yProfile("DRAFT"))

	resp, err := s.SubmitSellerTaxProfile(context.Background(), &pb.SubmitSellerTaxProfileRequest{
		StoreId: yStoreA, AcceptedTerms: false,
	})
	if err != nil {
		t.Fatalf("9325 必須走 envelope；回 gRPC error 會被 gateway 變成 5xx：%v", err)
	}
	if resp.GetCode() != bizcode.SellerSelfIssueTermsNotAccepted {
		t.Fatalf("code = %d, want 9325", resp.GetCode())
	}
	if resp.GetMessage() == "" {
		t.Fatal("9325 必須帶 message，前端要顯示它")
	}
}

// 端點 21 必須給得出**當期**條款版本。
//
// 🔴 沒有它，未簽過的賣家從 terms_version 只拿得到空字串，而端點 22 要求原樣回送
// ⇒ S04 的「同意」按鈕對新賣家從來按不成功（補簽路徑實際上是斷的）。
func TestTaxProfile_Attestation_ExposesCurrentTermsVersion(t *testing.T) {
	// 未簽過：terms_version 空、effective false、current_terms_version 仍必須非空。
	d := toSelfIssueAttestation(yProfile("DRAFT"))
	if d.GetCurrentTermsVersion() != usecase.SelfIssueTermsVersion {
		t.Fatalf("current_terms_version = %q，want %q（端點 22 要靠它回送）",
			d.GetCurrentTermsVersion(), usecase.SelfIssueTermsVersion)
	}
	if d.GetTermsVersion() != "" || d.GetEffective() {
		t.Fatalf("未簽過的賣家不得顯示成已簽：%+v", d)
	}

	// 已簽舊版：兩欄必須不同——合併成一欄就沒辦法表達「簽過但不是這一版」。
	signed := yProfile("SUBMITTED")
	old, at := "2000-01-01", yNow
	signed.AttestationVersion, signed.AttestedAt = &old, &at
	d2 := toSelfIssueAttestation(signed)
	if d2.GetTermsVersion() != old {
		t.Fatalf("terms_version = %q，want %q（已簽的版本）", d2.GetTermsVersion(), old)
	}
	if d2.GetCurrentTermsVersion() != usecase.SelfIssueTermsVersion {
		t.Fatalf("current_terms_version 被已簽版本蓋掉了：%q", d2.GetCurrentTermsVersion())
	}
}

// effective 錨在 review_status，不是 attested_at（裁決 C 的語意分離）。
//
// 🔴 這一支守的是一條**會靜默壞掉的業務承諾**：`effective` 若退回 `attested_at != nil`，
// 每個 SUBMITTED 的賣家都會被標成「自行開票責任生效中」，而同刻 ResolveFulfillmentMode
// 給的是 LEGACY_MANUAL／TAX_PROFILE_NOT_APPROVED、待審接單上限也不適用 ⇒ 畫面對賣家
// 做出與後端行為相反的法律告知。編譯、其他測試、CI 全部不會有任何反應。
func TestTaxProfile_AttestationEffective_AnchorsOnReviewStatus(t *testing.T) {
	at := yNow
	v := usecase.SelfIssueTermsVersion

	// 陰性（本測試的重點）：已簽 ＋ 還沒具結 ⇒ 責任**未**生效。
	for _, status := range []string{"SUBMITTED", "UNDER_REVIEW", "SUPPLEMENT_REQUIRED",
		"APPROVED_INVOICE_REQUIRED", "APPROVED_INVOICE_EXEMPT", "REJECTED", "EXPIRED", "REVOKED"} {
		p := yProfile(status)
		p.AttestationVersion, p.AttestedAt = &v, &at
		d := toSelfIssueAttestation(p)
		if d.GetEffective() {
			t.Fatalf("%s：已簽但責任尚未生效，effective 必須是 false（判準是狀態值不是 attested_at）", status)
		}
		// 同一列的「簽過沒」仍要答得出來——兩個問題分開，不是把資訊丟掉。
		if d.GetTermsVersion() != v || d.GetAcceptedAt() == "" {
			t.Fatalf("%s：簽署事實不得因為 effective=false 而消失：%+v", status, d)
		}
	}

	// 陽性：SELF_ISSUE_PENDING ＝ ResolveFulfillmentMode 唯一判出 SELLER_SELF_ISSUE 的狀態。
	p := yProfile("SELF_ISSUE_PENDING")
	p.AttestationVersion, p.AttestedAt = &v, &at
	if !toSelfIssueAttestation(p).GetEffective() {
		t.Fatal("SELF_ISSUE_PENDING 就是「自行開票責任生效中」，effective 必須是 true")
	}
}

// ---------------------------------------------------------------------------
// 型別邊界：契約違規 ⇒ InvalidArgument（400），不借業務碼
// ---------------------------------------------------------------------------

func TestTaxProfile_ContractViolationsAreInvalidArgument(t *testing.T) {
	s := newTaxServer(true, yProfile("UNDER_REVIEW"))

	cases := []struct {
		name string
		run  func() error
	}{
		{"store_id 缺席", func() error {
			_, err := s.SubmitSellerTaxProfile(context.Background(), &pb.SubmitSellerTaxProfileRequest{})
			return err
		}},
		{"expected_version 非數字", func() error {
			_, err := s.SubmitSellerTaxProfile(context.Background(), &pb.SubmitSellerTaxProfileRequest{
				StoreId: yStoreA, ExpectedVersion: "abc",
			})
			return err
		}},
		{"decision 值域外", func() error {
			_, err := s.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
				ActorAccountId: yAdmin, StoreId: "2685914682", Decision: "APPROVE_TAXABLE", Reason: "x",
			})
			return err
		}},
		{"REVOKE 缺 confirmed", func() error {
			s2 := newTaxServer(true, yProfile("APPROVED_INVOICE_REQUIRED"))
			_, err := s2.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
				ActorAccountId: yAdmin, StoreId: "2685914682",
				Decision: codemeta.SellerTaxReviewDecisionRevoke.Value, Reason: "撤銷",
			})
			return err
		}},
		{"reason 缺席", func() error {
			_, err := s.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
				ActorAccountId: yAdmin, StoreId: "2685914682",
				Decision: codemeta.SellerTaxReviewDecisionKeepUnderReview.Value,
			})
			return err
		}},
		{"approval_valid_until 非 RFC3339", func() error {
			_, err := s.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
				ActorAccountId: yAdmin, StoreId: "2685914682",
				Decision:           codemeta.SellerTaxReviewDecisionApproveInvoiceRequired.Value,
				Reason:             "核准",
				ApprovalValidUntil: "2027-01-01",
			})
			return err
		}},
		{"送審缺佐證", func() error {
			row := yProfile("DRAFT")
			row.EvidenceObjectKeys = json.RawMessage(`[]`)
			s2 := newTaxServer(true, row)
			_, err := s2.SubmitSellerTaxProfile(context.Background(), &pb.SubmitSellerTaxProfileRequest{
				StoreId: yStoreA, AcceptedTerms: true,
			})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("want InvalidArgument，實際 %v（借業務碼會讓前端把契約違規顯示成別的意思）", err)
			}
		})
	}
}

// 陽性：ADM 裁決回 DecideTaxReviewForAdminData 的四／五欄，ID 與 version 都是 string（§4）。
func TestTaxProfile_DecideReturnsStringTypedData(t *testing.T) {
	s := newTaxServer(true, yProfile("APPROVED_INVOICE_REQUIRED"))

	resp, err := s.DecideTaxReviewForAdmin(context.Background(), &pb.DecideTaxReviewForAdminRequest{
		ActorAccountId: yAdmin, StoreId: "2685914682",
		Decision: codemeta.SellerTaxReviewDecisionRevoke.Value, Reason: "撤銷資格", Confirmed: true,
	})
	if err != nil {
		t.Fatalf("Decide 失敗：%v", err)
	}
	d := resp.GetData()
	if d.GetStoreId() != "2685914682" || d.GetAuditLogId() != "42" {
		t.Fatalf("id 欄位必須是 string：%+v", d)
	}
	// 只有 REVOKE 帶 revokedEffectiveAt（T-C04 的分界時點）。
	if d.GetRevokedEffectiveAt() == "" {
		t.Fatalf("REVOKE 必須帶 revokedEffectiveAt：%+v", d)
	}
}
