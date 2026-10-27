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

// ---------------------------------------------------------------------------
// fakes
//
// 每個 fake 都**照 SQL 述詞的形狀**回答（擁有者是查詢條件的一部分），不是「查回來再比對」。
// 這樣寫是刻意的：真正守住 Tier A 的是 db/queries 的 WHERE，本層的測試驗的是
// 「repository 回同一個 error 時，handler 也吐同一個 envelope」。SQL 那一半由
// internal/usecase/query_integration_test.go 用真的 PostgreSQL 驗。
// ---------------------------------------------------------------------------

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
	// GetByRef 對 pgx.ErrNoRows 回 9330，與真實 adapter 一致。
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

// qAudit 記下每一次 append，並可被設定成寫入失敗（驗 fail-closed）。
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

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

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
		// 本檔的既有案例都不碰折讓／修改申請／總覽計數 —— 三個 fake 只實作了
		// 「空集合」。**它們不是佔位**：ALLOWANCE_APPLIED 的判準在 adjustments 上，
		// 回 nil 才是「這份文件沒有折讓」的正確答案（回錯會讓本檔的 ADM 案例
		// 靜默改變 state 投影）。有折讓的案例在 query_read_test.go。
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

// ---------------------------------------------------------------------------
// Tier A 陰性：越權必須與「不存在」位元組相同（§8.1.4(d) 的判準）
// ---------------------------------------------------------------------------

// TestGetInvoiceComplaint_TierA_ForeignIsIndistinguishableFromMissing 是本波最重要的一支。
//
// 判準**不是**「回 404」，是「兩條路徑的回應位元組相同」（§8.1.4(d) 逐字）：
// 只斷言 code 的測試擋不住「同樣 404 但 message 不同」——那是最常見的洩漏形狀，
// 而它會讓攻擊者靠回應差異枚舉出「這個 complaint_id 存在，只是不是你的」。
func TestGetInvoiceComplaint_TierA_ForeignIsIndistinguishableFromMissing(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	// (a) 買家 B 去讀買家 A 的申訴（存在，但不是他的）
	foreign, err := fx.server.GetInvoiceComplaint(ctx, &pb.GetInvoiceComplaintRequest{
		BuyerId: buyerB, OrderNo: "O000000001", ComplaintId: "5001",
	})
	if err != nil {
		t.Fatalf("越權查詢不該回 gRPC error（那會繞過 envelopeCodeToGRPC）：%v", err)
	}

	// (b) 買家 B 去讀一個根本不存在的 complaint_id
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
	// 🔴 code≠0 時 data 必須是空的：回 code=0 ＋ 空 data 對前端是「查到了但什麼都沒有」，
	// 會渲染成空卡片而不是「找不到」。
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

	// 買家 B 沒有任何資料；買家 C 也沒有。兩者的回應必須一模一樣——
	// 若 buyer_id 沒有進 SQL 的 WHERE，B 會看到 A 的那一筆而這裡當場紅。
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

// ---------------------------------------------------------------------------
// 身分欄位：0 不是「沒有這個人」，是 Gateway 沒注入
// ---------------------------------------------------------------------------

func TestQueryRPCs_RejectMissingPrincipal(t *testing.T) {
	fx := newQueryServer(t, nil)
	ctx := context.Background()

	// buyer_id = 0 若被當成合法值往下查，SQL 會安靜地回空列表 ——
	// 看起來像「這位買家沒有資料」，而真相是契約破了。
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

// ---------------------------------------------------------------------------
// 遮罩／憑證：C 類永不回顯、P 類在 ADM 面回明文並留稽核
// ---------------------------------------------------------------------------

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
	// 🔴 賣家統編是 O 類（公開營業登記資訊）⇒ 完整，不遮。欄名帶 Masked 是歷史遺留，
	// §8.1.3(b) 第二列點名這個 proto 欄位並判「三端皆完整」。
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
	// 🔴 request_id 必須是 nil：uq_invadmaudit_client_request 會把同一個
	// (actor, action, request_id) 摺成一列 ⇒ 帶了它，同一位 ADM 的第二次揭露就不留痕。
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
	// T-I12：退避參數必須進 job DTO，前端不得自行猜測。
	if job.GetRetryPolicy().GetInitialIntervalSeconds() != "30" || job.GetRetryPolicy().GetBackoffMultiplier() != "2" {
		t.Fatalf("retry_policy 必須反映服務實際使用的參數，實得 %v", job.GetRetryPolicy())
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// assertProtoBytesEqual 比對兩個回應的序列化位元組。
//
// 用位元組而不是逐欄比對，是因為「多了一個欄位」也算可分辨，而逐欄比對的測試會在
// 新增欄位時靜默失去鑑別力。
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

// assertNoSubstring 把整個回應序列化成文字後找禁字。
func assertNoSubstring(t *testing.T, m proto.Message, why string, needles ...string) {
	t.Helper()
	blob := m.(interface{ String() string }).String()
	for _, n := range needles {
		if strings.Contains(blob, n) {
			t.Fatalf("%s：回應內含 %q\n%s", why, n, blob)
		}
	}
}

// qCutovers／qUnlocks 是端點 46／51 兩個新讀取面的替身。
//
// 兩者都只覆寫會被走到的那幾支：其餘方法留 nil，被呼叫時 panic 並指名方法 ——
// 回零值會讓「這條路徑本來就不該被走到」看起來與「查無」一模一樣。
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

// qCancelled 是 invoice_cancelled_orders 的 fake（#2904）。
//
// 🔴 零值＝「這張訂單沒有被取消」＝ 舊行為，所以把它當預設 stub 傳進 NewQuery 是
// 安全的；**但不要傳 nil**——nil 會在「零 document」那條分支上 panic，而那條分支
// 正是端點 8 最常見的一格（還沒付款的訂單）。
//
// 授權述詞在 fake 裡也要重現：鍵是 (order_no, buyer_id) 兩欄。只用 order_no 當鍵的
// fake 會讓「拿別人的 order_no 問」在測試裡看起來是對的，於是那支測試永遠證明不了
// 它宣稱要證明的事。
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

// qBizDays 是行事曆鏡像的 fake：一份「已同步、沒有國定假日」的曆，
// 讓工作天倒數算得出來（週末仍然會被扣掉）。
type qBizDays struct {
	synced *time.Time
}

func (qBizDays) DeleteAll(context.Context) (int64, error)           { return 0, nil }
func (qBizDays) Insert(context.Context, time.Time, time.Time) error { return nil }
func (qBizDays) ListHolidays(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (f qBizDays) SyncedAt(context.Context) (*time.Time, error) { return f.synced, nil }

// syncedNow 讓涵蓋窗口罩得住測試用的日期。
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
		// 這一格就是 #3021：舊實作在這裡回 true ⇒ S02 渲染 CTA ⇒ 按下必回 9321。
		{codemeta.SellerTaxReviewStatusSubmitted.Value, 0, false},
		{codemeta.SellerTaxReviewStatusDraft.Value, 0, false},
		{codemeta.SellerTaxReviewStatusApprovedInvoiceRequired.Value, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.status+"/"+strconv.Itoa(int(tc.count)), func(t *testing.T) {
			// evidenceMeta 為 nil ⇒ 佐證中介資料退回空 map（本測試不驗那一段）。
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
