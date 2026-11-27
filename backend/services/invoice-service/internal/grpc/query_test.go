package grpc

import (
	"context"
	"errors"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	buyerA = int64(2623900520)
	buyerB = int64(2688637900)
	storeA = int64(2685914682)
	storeB = int64(2632049831)
)

type qIssuers struct {
	domain.IssuerRepository
	byRef map[int64]domain.Issuer
	byID  map[int64]domain.Issuer
}

func (q qIssuers) GetByRef(_ context.Context, issuerType string, refID int64) (domain.Issuer, error) {
	if issuerType != "SELLER" {
		return domain.Issuer{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
	}
	if v, ok := q.byRef[refID]; ok {
		return v, nil
	}
	return domain.Issuer{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
}

func (q qIssuers) GetByID(_ context.Context, id int64) (domain.Issuer, error) {
	if v, ok := q.byID[id]; ok {
		return v, nil
	}
	return domain.Issuer{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
}

type qProfiles struct {
	domain.SellerTaxProfileRepository
	byIssuer map[int64]domain.SellerTaxProfile
}

func (q qProfiles) GetByIssuer(_ context.Context, issuerID int64) (domain.SellerTaxProfile, error) {
	if v, ok := q.byIssuer[issuerID]; ok {
		return v, nil
	}
	return domain.SellerTaxProfile{}, notFoundBiz(bizcode.SellerTaxProfileNotFound)
}

type qStoreAccount struct {
	storeID int64
	account domain.ProviderAccount
}

type qAccounts struct {
	domain.ProviderAccountRepository
	byIssuer map[int64]domain.ProviderAccount
	byStore  []qStoreAccount
}

func (q qAccounts) GetByIssuer(_ context.Context, issuerID int64, _ string) (domain.ProviderAccount, error) {
	if v, ok := q.byIssuer[issuerID]; ok {
		return v, nil
	}
	return domain.ProviderAccount{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
}

// GetByStoreID 照 SQL 的形狀：經 invoice_issuers 解析，查無回 9330。
// 端點 46 靠它把「這個賣家根本沒設過 AMEGO 連線」與「設過但還沒設切點」分開 ——
// 回空清單會把前者顯示成後者，而兩者的下一步不同。
func (q qAccounts) GetByStoreID(_ context.Context, storeID int64, _ string) (domain.ProviderAccount, error) {
	for _, v := range q.byStore {
		if v.storeID == storeID {
			return v.account, nil
		}
	}
	return domain.ProviderAccount{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
}

type qPrefs struct {
	domain.BuyerPreferenceRepository
	byBuyer map[int64][]domain.BuyerPreference
}

// ListByBuyer 照 SQL 的形狀：buyer_id 是述詞，別人的 buyer_id 得到空集合。
func (q qPrefs) ListByBuyer(_ context.Context, buyerID int64, limit int32) ([]domain.BuyerPreference, error) {
	rows := q.byBuyer[buyerID]
	if int32(len(rows)) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

type qComplaints struct {
	domain.ComplaintRepository
	rows []domain.Complaint
}

// GetForBuyer 照 SQL 的形狀：id AND complainant_buyer_id 兩個條件同時成立才回列。
func (q qComplaints) GetForBuyer(_ context.Context, id, buyerID int64) (domain.Complaint, error) {
	for _, c := range q.rows {
		if c.ID == id && c.ComplainantBuyerID == buyerID {
			return c, nil
		}
	}
	return domain.Complaint{}, notFoundBiz(bizcode.InvoiceComplaintNotFound)
}

func (q qComplaints) GetByID(_ context.Context, id int64) (domain.Complaint, error) {
	for _, c := range q.rows {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.Complaint{}, notFoundBiz(bizcode.InvoiceComplaintNotFound)
}

func (q qComplaints) ListEvents(context.Context, int64) ([]domain.ComplaintEvent, error) {
	return nil, nil
}

type qDocuments struct {
	domain.DocumentRepository
	byID    map[int64]domain.Document
	secrets domain.RecipientSecrets
}

func (q qDocuments) GetByID(_ context.Context, id int64) (domain.Document, error) {
	if v, ok := q.byID[id]; ok {
		return v, nil
	}
	return domain.Document{}, notFoundBiz(bizcode.InvoiceDocumentNotFound)
}

func (q qDocuments) ListLines(context.Context, int64) ([]domain.DocumentLine, error) { return nil, nil }

func (q qDocuments) WithDecryptedRecipient(_ context.Context, _ int64, fn func(domain.RecipientSecrets) error) error {
	return fn(q.secrets)
}

type qOperations struct {
	domain.OperationRepository
	byDoc map[int64][]domain.Operation
}

func (q qOperations) ListByDocument(_ context.Context, docID int64) ([]domain.Operation, error) {
	return q.byDoc[docID], nil
}

func (q qOperations) GetByID(_ context.Context, id int64) (domain.Operation, error) {
	for _, ops := range q.byDoc {
		for _, op := range ops {
			if op.ID == id {
				return op, nil
			}
		}
	}
	return domain.Operation{}, notFoundBiz(bizcode.InvoiceJobNotFound)
}

type qAudit struct {
	domain.AuditRepository
	correlationID string
	appended      []domain.NewAdminAuditLogEntry
	appendErr     error
}

// LatestProviderCorrelationID：替身回固定值（#3031）。
// 嵌入 domain.AuditRepository 只讓型別成立，實際呼叫會 nil-deref ⇒ 必須明寫。
func (q *qAudit) LatestProviderCorrelationID(_ context.Context, _ int64) (string, error) {
	return q.correlationID, nil
}

func (q *qAudit) AppendAdminAuditLog(_ context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	if q.appendErr != nil {
		return 0, q.appendErr
	}
	q.appended = append(q.appended, in)
	return int64(len(q.appended)), nil
}

func (q *qAudit) ListStatusHistory(context.Context, string, int64) ([]domain.StatusHistoryEntry, error) {
	return nil, nil
}

func (q *qAudit) ListProviderAuditLogsByOperation(context.Context, int64) ([]domain.ProviderAuditLogEntry, error) {
	return nil, nil
}

func (q *qAudit) ListAdminAuditLogsByResource(context.Context, string, int64, int32) ([]domain.AdminAuditLogEntry, error) {
	return nil, nil
}

func notFoundBiz(code int32) error {
	return newBizErr(code)
}

type qFixture struct {
	server *InvoiceServer
	audit  *qAudit
}

func newQueryServer(t *testing.T, mut func(*qFixture)) *qFixture {
	t.Helper()
	issuerA, issuerB := int64(9001), int64(9002)
	fx := &qFixture{audit: &qAudit{}}
	prefs := qPrefs{byBuyer: map[int64][]domain.BuyerPreference{
		buyerA: {{
			ID: 1, BuyerID: buyerA, PreferenceKind: "MOBILE_BARCODE",
			CarrierType: strp("3J0002"), CarrierValueMask: strp("/T••••+P"),
			CarrierValueCiphertext: strp("ENCRYPTED-BLOB"),
			IsDefault:              true, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
		}},
	}}
	q := usecase.NewQuery(
		qIssuers{
			byRef: map[int64]domain.Issuer{storeA: {ID: issuerA, IssuerType: "SELLER", IssuerRefID: storeA}},
			byID:  map[int64]domain.Issuer{issuerA: {ID: issuerA, IssuerType: "SELLER", IssuerRefID: storeA}},
		},
		qProfiles{byIssuer: map[int64]domain.SellerTaxProfile{
			issuerA: {ID: 55, IssuerID: issuerA, ReviewStatus: "APPROVED_INVOICE_REQUIRED", TaxID: strp("12345678")},
		}},
		qAccounts{byIssuer: map[int64]domain.ProviderAccount{
			issuerA: {ID: 77, IssuerID: issuerA, Provider: "AMEGO", Status: "ACTIVE",
				ProviderTaxID: "12345678", CredentialLast4: "wIM2"},
		}},
		prefs,
		qDocuments{
			byID: map[int64]domain.Document{
				3001: {ID: 3001, IssuerID: issuerA, DocumentKind: "B2C", Status: "ISSUED",
					RecipientType: "BUYER", RecipientRefID: &[]int64{buyerA}[0],
					CarrierType: strp("3J0002"), CarrierId1Ciphertext: strp("ENCRYPTED"),
					TotalAmount: 100},
			},
			secrets: domain.RecipientSecrets{Email: "buyer@example.com", CarrierID1: "/T7R3U+P"},
		},
		qOperations{byDoc: map[int64][]domain.Operation{
			3001: {{ID: 4001, DocumentID: 3001, OperationType: "ISSUE", Status: "FAILED_PERMANENT",
				OperationKey: "k-1", ProviderRequestNo: "R-1"}},
		}},
		qComplaints{rows: []domain.Complaint{
			{ID: 5001, ComplainantBuyerID: buyerA, IssuerID: issuerA, OrderNo: strp("O000000001"),
				Status: "OPEN", SlaDueAt: time.Unix(100, 0).UTC(), CreatedAt: time.Unix(1, 0).UTC()},
		}},
		fx.audit,
		qAdjustments{},
		qCorrections{},
		qOverview{},
		qBizDays{synced: syncedNow(time.Now().UTC())},
		qCutovers{},
		qUnlocks{},
		qCancelled{},
		usecase.IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		nil,
	)
	_ = issuerB
	fx.server = NewInvoiceServer(nil, q, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if mut != nil {
		mut(fx)
	}
	return fx
}

func strp(s string) *string { return &s }

// TestGetInvoiceComplaint_TierA_ForeignIsIndistinguishableFromMissing 是本波最重要的一支。
//
// 判準**不是**「回 404」，是「兩條路徑的回應位元組相同」（§8.1.4(d) 逐字）：
// 只斷言 code 的測試擋不住「同樣 404 但 message 不同」——那是最常見的洩漏形狀，
// 而它會讓攻擊者靠回應差異枚舉出「這個 complaint_id 存在，只是不是你的」。
func TestGetInvoiceComplaint_TierA_ForeignIsIndistinguishableFromMissing(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	foreign, err := fx.server.GetInvoiceComplaint(ctx, &pb.GetInvoiceComplaintRequest{
		BuyerId: buyerB, OrderNo: "O000000001", ComplaintId: "5001",
	})
	if err != nil {
		t.Fatalf("越權查詢不該回 gRPC error（那會繞過 envelopeCodeToGRPC）：%v", err)
	}

	missing, err := fx.server.GetInvoiceComplaint(ctx, &pb.GetInvoiceComplaintRequest{
		BuyerId: buyerB, OrderNo: "O000000001", ComplaintId: "9999999",
	})
	if err != nil {
		t.Fatalf("查無資料不該回 gRPC error：%v", err)
	}

	assertProtoBytesEqual(t, foreign, missing,
		"越權與不存在的回應必須完全相同（§8.1.4(c)：同一個碼、同一段 message、同一行程式碼）")

	if foreign.GetCode() != bizcode.InvoiceComplaintNotFound {
		t.Fatalf("預期業務碼 %d，實得 %d", bizcode.InvoiceComplaintNotFound, foreign.GetCode())
	}
	if foreign.GetData() != nil {
		t.Fatalf("非零業務碼不得帶 data，實得 %v", foreign.GetData())
	}
}

// TestGetInvoiceComplaint_OrderMismatchSharesTheSameNotFound 釘住第二條可能的洩漏路徑：
// 申訴確實是這位買家的，但他傳了別的 order_no。那一樣是 not-found，**不得**另開一個碼
// ——有第二個碼就等於在回應裡放一個可以區分兩者的旗標（§8.1.4(c) 的唯一裁定）。
func TestGetInvoiceComplaint_OrderMismatchSharesTheSameNotFound(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	mismatch, err := fx.server.GetInvoiceComplaint(ctx, &pb.GetInvoiceComplaintRequest{
		BuyerId: buyerA, OrderNo: "O000000002", ComplaintId: "5001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	missing, err := fx.server.GetInvoiceComplaint(ctx, &pb.GetInvoiceComplaintRequest{
		BuyerId: buyerA, OrderNo: "O000000002", ComplaintId: "9999999",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, mismatch, missing, "order_no 不匹配必須與不存在完全相同")
}

// TestListBuyerInvoiceProfiles_TierA_ForeignBuyerSeesEmpty 是列表型的 Tier A：
// 別人的 buyer_id 得到的不是錯誤，而是與「這位買家還沒建過任何資料」完全相同的空列表。
func TestListBuyerInvoiceProfiles_TierA_ForeignBuyerSeesEmpty(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	mine, err := fx.server.ListBuyerInvoiceProfiles(ctx, &pb.ListBuyerInvoiceProfilesRequest{BuyerId: buyerA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mine.GetData().GetRecords()) != 1 {
		t.Fatalf("買家 A 應該看得到自己的 1 筆，實得 %d", len(mine.GetData().GetRecords()))
	}

	other, err := fx.server.ListBuyerInvoiceProfiles(ctx, &pb.ListBuyerInvoiceProfilesRequest{BuyerId: buyerB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	unknown, err := fx.server.ListBuyerInvoiceProfiles(ctx, &pb.ListBuyerInvoiceProfilesRequest{BuyerId: 999999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(other.GetData().GetRecords()) != 0 {
		t.Fatalf("買家 B 不得看到任何資料，實得 %d 筆", len(other.GetData().GetRecords()))
	}
	assertProtoBytesEqual(t, other, unknown, "「別人的 buyer_id」與「沒有資料的 buyer_id」必須完全相同")
}

// TestGetSellerTaxProfile_TierA_ForeignStoreIsIndistinguishable 是賣家側的 Tier A 陰性。
//
// 賣家的擁有者述詞是兩段：invoice_issuers.issuer_ref_id = store_id（SQL）→ 再以 issuer_id
// 取 profile。別的 store_id 解出別的 issuer ⇒ 拿不到這一列。
//
// 🔴 這裡同時釘住「issuer 查無」與「profile 查無」回同一個碼：GetByRef 內部把 ErrNoRows
// 映成 9330（provider 語意），在稅務資格這條路徑上直接透出去，會讓「還沒有 issuer」與
// 「有 issuer 但沒送過審」對外可分辨。
func TestGetSellerTaxProfile_TierA_ForeignStoreIsIndistinguishable(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	foreign, err := fx.server.GetSellerTaxProfile(ctx, &pb.GetSellerTaxProfileRequest{StoreId: storeB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	missing, err := fx.server.GetSellerTaxProfile(ctx, &pb.GetSellerTaxProfileRequest{StoreId: 777777})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, foreign, missing, "別的 store_id 與不存在的 store_id 必須完全相同")
	if foreign.GetCode() != bizcode.SellerTaxProfileNotFound {
		t.Fatalf("預期業務碼 %d（不是 provider 的 9330），實得 %d",
			bizcode.SellerTaxProfileNotFound, foreign.GetCode())
	}
	if foreign.GetData() != nil {
		t.Fatalf("非零業務碼不得帶 data")
	}
}

// TestGetSellerProviderAccount_TierA_ForeignStoreIsIndistinguishable 同上，碼是 9330。
func TestGetSellerProviderAccount_TierA_ForeignStoreIsIndistinguishable(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	foreign, err := fx.server.GetSellerProviderAccount(ctx, &pb.GetSellerProviderAccountRequest{StoreId: storeB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	missing, err := fx.server.GetSellerProviderAccount(ctx, &pb.GetSellerProviderAccountRequest{StoreId: 777777})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, foreign, missing, "別的 store_id 與不存在的 store_id 必須完全相同")
	if foreign.GetCode() != bizcode.InvoiceProviderAccountNotFound {
		t.Fatalf("預期業務碼 %d，實得 %d", bizcode.InvoiceProviderAccountNotFound, foreign.GetCode())
	}
}

func TestQueryRPCs_RejectMissingPrincipal(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	if _, err := fx.server.ListBuyerInvoiceProfiles(ctx, &pb.ListBuyerInvoiceProfilesRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("buyer_id=0 應回 InvalidArgument，實得 %v", err)
	}
	if _, err := fx.server.GetSellerTaxProfile(ctx, &pb.GetSellerTaxProfileRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("store_id=0 應回 InvalidArgument，實得 %v", err)
	}
	if _, err := fx.server.GetInvoiceDocumentForAdmin(ctx, &pb.GetInvoiceDocumentForAdminRequest{DocumentId: "3001"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("actor_account_id=0 應回 InvalidArgument，實得 %v", err)
	}
}

// TestGetSellerProviderAccount_NeverExposesCredential 釘住 §8.1.2 層 1／§8.1.3 的 C 類規則。
//
// 判準是「整個序列化後的回應裡找不到明碼」，不是「某個欄位等於 last4」——後者擋不住
// 「有人多加了一個欄位把 app_key 帶出去」，而那正是 proto 用 reserved 90 要防的形狀。
func TestGetSellerProviderAccount_NeverExposesCredential(t *testing.T) {
	fx := newQueryServer(t, nil)
	resp, err := fx.server.GetSellerProviderAccount(context.Background(),
		&pb.GetSellerProviderAccountRequest{StoreId: storeA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.GetData().GetCredentialLast4(); got != "wIM2" {
		t.Fatalf("credential_last4 應為 wIM2，實得 %q", got)
	}
	if got := resp.GetData().GetSellerTaxId(); got != "12345678" {
		t.Fatalf("賣家統編是 O 類應回完整值，實得 %q", got)
	}
	assertNoSubstring(t, resp, "AMEGO AppKey 或其任何片段都不得出現在回應裡",
		"ENCRYPTED", "app_key", "appKey")
}

// TestListBuyerInvoiceProfiles_ReturnsMaskNotPlaintext 釘住「明碼不出服務邊界」：
// display_value 取的是已落庫的 carrier_value_mask，本路徑**不解密**。
func TestListBuyerInvoiceProfiles_ReturnsMaskNotPlaintext(t *testing.T) {
	fx := newQueryServer(t, nil)
	resp, err := fx.server.ListBuyerInvoiceProfiles(context.Background(),
		&pb.ListBuyerInvoiceProfilesRequest{BuyerId: buyerA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec := resp.GetData().GetRecords()[0]
	if rec.GetDisplayValue() != "/T••••+P" {
		t.Fatalf("display_value 應為已落庫的遮罩值，實得 %q", rec.GetDisplayValue())
	}
	assertNoSubstring(t, resp, "密文欄位不得出現在回應裡（列表路徑不解密）", "ENCRYPTED-BLOB")
}

// TestGetInvoiceDocumentForAdmin_RevealsPlaintextAndWritesAudit 釘住 §8.1.3(c) 裁定 2
// 的**兩半**：ADM 端不遮罩，而且每一次揭露都留一列 INVOICE_PII_REVEAL。
//
// 只驗其中一半是不夠的：只驗「看得到明碼」＝ 把裁定的代價那一半刪掉也不會紅；
// 只驗「有稽核」＝ 遮罩回去也不會紅，而 ADM 就查不了案。
func TestGetInvoiceDocumentForAdmin_RevealsPlaintextAndWritesAudit(t *testing.T) {
	fx := newQueryServer(t, nil)
	resp, err := fx.server.GetInvoiceDocumentForAdmin(context.Background(),
		&pb.GetInvoiceDocumentForAdminRequest{ActorAccountId: 4242, DocumentId: "3001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resp.GetData().GetSellerView().GetDetail().GetSummary().GetCarrierSummary()
	if got != "/T7R3U+P" {
		t.Fatalf("ADM 端載具應為明碼（§8.1.3(c) 裁定 2），實得 %q", got)
	}

	if len(fx.audit.appended) != 1 {
		t.Fatalf("每次揭露必須留一列稽核，實得 %d 列", len(fx.audit.appended))
	}
	entry := fx.audit.appended[0]
	if entry.Action != "INVOICE_PII_REVEAL" {
		t.Fatalf("action 必須是 INVOICE_PII_REVEAL（告警 InvoicePiiRevealSpike 靠它），實得 %q", entry.Action)
	}
	if entry.ResourceType != "INVOICE_DOCUMENT" || entry.ResourceID != 3001 {
		t.Fatalf("稽核必須指名被讀的資源，實得 %s/%d", entry.ResourceType, entry.ResourceID)
	}
	if entry.ActorAccountID != 4242 {
		t.Fatalf("稽核必須記下是誰讀的，實得 %d", entry.ActorAccountID)
	}
	if entry.RequestID != nil {
		t.Fatalf("稽核不得帶 request_id（唯一索引會讓後續揭露被去重），實得 %v", *entry.RequestID)
	}
}

// TestGetInvoiceDocumentForAdmin_FailsClosedWhenAuditFails 釘住 fail-closed：
// 稽核寫不進去就**不回明碼**。少了這一條，稽核在資料庫出問題時靜默失效而畫面完全正常
// ——那正是「機制只讓問題被發現、沒讓問題不發生」的反面。
func TestGetInvoiceDocumentForAdmin_FailsClosedWhenAuditFails(t *testing.T) {
	fx := newQueryServer(t, func(f *qFixture) { f.audit.appendErr = errors.New("audit table down") })
	resp, err := fx.server.GetInvoiceDocumentForAdmin(context.Background(),
		&pb.GetInvoiceDocumentForAdminRequest{ActorAccountId: 4242, DocumentId: "3001"})
	if err == nil {
		t.Fatalf("稽核寫入失敗時不得回資料，實得 resp=%v", resp)
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("稽核失敗是故障不是業務結果，應回 Internal，實得 %v", err)
	}
}

// TestGetInvoiceJobForAdmin_MapsStateDerivedFlags 釘住 can_retry／can_cancel 的來源是
// SQL 的 allowed-from（FAILED_PERMANENT 兩者皆可），不是「看起來像失敗的狀態」。
func TestGetInvoiceJobForAdmin_MapsStateDerivedFlags(t *testing.T) {
	fx := newQueryServer(t, nil)
	resp, err := fx.server.GetInvoiceJobForAdmin(context.Background(),
		&pb.GetInvoiceJobForAdminRequest{ActorAccountId: 4242, JobId: "4001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	job := resp.GetData().GetJob()
	if !job.GetCanRetry() || !job.GetCanCancel() {
		t.Fatalf("FAILED_PERMANENT 同時在兩支 CAS 的 allowed-from 內，兩個旗標都該是 true")
	}
	if job.GetStoreId() != "2685914682" {
		t.Fatalf("store_id 必須經 invoice_issuers 取得（invoice_operations 沒有 store 欄），實得 %q", job.GetStoreId())
	}
	if job.GetRetryPolicy().GetInitialIntervalSeconds() != "30" || job.GetRetryPolicy().GetBackoffMultiplier() != "2" {
		t.Fatalf("retry_policy 必須反映服務實際使用的參數，實得 %v", job.GetRetryPolicy())
	}
}

func assertProtoBytesEqual(t *testing.T, a, b proto.Message, why string) {
	t.Helper()
	ab, err := proto.Marshal(a)
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	bb, err := proto.Marshal(b)
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	if string(ab) != string(bb) {
		t.Fatalf("%s\n  a = %v\n  b = %v", why, a, b)
	}
}

func assertNoSubstring(t *testing.T, m proto.Message, why string, needles ...string) {
	t.Helper()
	blob := m.(interface{ String() string }).String()
	for _, n := range needles {
		if strings.Contains(blob, n) {
			t.Fatalf("%s：回應內含 %q\n%s", why, n, blob)
		}
	}
}

type qCutovers struct {
	domain.ProviderCutoverRepository
	byAccount map[int64][]domain.ProviderCutover
}

func (q qCutovers) ListForAdmin(_ context.Context, accountID int64, _ domain.Page) ([]domain.ProviderCutover, error) {
	return q.byAccount[accountID], nil
}

func (q qCutovers) CountForAdmin(_ context.Context, accountID int64) (int64, error) {
	return int64(len(q.byAccount[accountID])), nil
}

type qUnlocks struct{ domain.ManualUnlockRepository }

func (qUnlocks) GetLatest(context.Context, int64) (domain.ManualUnlock, bool, error) {
	return domain.ManualUnlock{}, false, nil
}

type qCancelled struct {
	rows map[qCancelledKey]bool
}

type qCancelledKey struct {
	orderNo string
	buyerID int64
}

func (qCancelled) Insert(context.Context, domain.CancelledOrder) (bool, error) {
	return false, nil
}

func (q qCancelled) IsCancelledForBuyer(_ context.Context, orderNo string, buyerID int64) (bool, error) {
	return q.rows[qCancelledKey{orderNo: orderNo, buyerID: buyerID}], nil
}

type qBizDays struct {
	synced *time.Time
}

func (qBizDays) DeleteAll(context.Context) (int64, error)                             { return 0, nil }
func (qBizDays) Insert(context.Context, time.Time, time.Time) error                   { return nil }
func (f qBizDays) SetCoverage(context.Context, time.Time, time.Time, time.Time) error { return nil }

// Snapshot 回「推送時間 −30 天 ～ +400 天」的涵蓋區間：這些測試只需要區間罩得住測試日期。
func (f qBizDays) Snapshot(context.Context, time.Time, time.Time) ([]time.Time, *domain.BusinessDayCoverage, error) {
	if f.synced == nil {
		return nil, nil, nil
	}
	base := *f.synced
	return nil, &domain.BusinessDayCoverage{From: base.AddDate(0, 0, -30), To: base.AddDate(0, 0, 400), SyncedAt: base}, nil
}

func syncedNow(t time.Time) *time.Time { v := t; return &v }

// #3021 的第二個賦值點：端點 17（S02 頁面實際讀的那一支）。
//
// 🔴 **這支測試存在的理由是「同一個判準有兩份實作」**：2026-09-02 修 #3021 時
// 只改了 SellerCapabilityView（端點 16），而 toSellerTaxProfileData 自己算
// `p.ExtensionCount == 0` ⇒ dev 上實測 canRequestExtension 對 SUBMITTED 的賣家
// **仍然是 true**，症狀完全沒消失。單元測試當時全綠，因為沒有任何一支看著這一格。
//
// 現在兩處都走 usecase.CanRequestTaxExtension；本測試釘住「端點 17 也吃到狀態條件」。
func TestToSellerTaxProfileData_CanRequestExtensionHonoursStatus(t *testing.T) {
	cases := []struct {
		status string
		count  int16
		want   bool
	}{
		{codemeta.SellerTaxReviewStatusSelfIssuePending.Value, 0, true},
		{codemeta.SellerTaxReviewStatusSelfIssuePending.Value, 1, false},
		{codemeta.SellerTaxReviewStatusSubmitted.Value, 0, false},
		{codemeta.SellerTaxReviewStatusDraft.Value, 0, false},
		{codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.status+"/"+strconv.Itoa(int(tc.count)), func(t *testing.T) {
			srv := &InvoiceServer{}
			got := srv.toSellerTaxProfileData(context.Background(), usecase.SellerTaxProfileView{
				StoreID: 1,
				Profile: domain.SellerTaxProfile{ReviewStatus: tc.status, ExtensionCount: tc.count},
			})
			if got.GetCanRequestExtension() != tc.want {
				t.Fatalf("canRequestExtension = %v, want %v（端點 17 的旗標必須與端點 20 的守衛一致）",
					got.GetCanRequestExtension(), tc.want)
			}
		})
	}
}
