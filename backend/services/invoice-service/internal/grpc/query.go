package grpc

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是 §6.1 唯讀查詢類 RPC 的 Layer 5。
//
// 🔴 **業務錯誤走 envelope，不走 gRPC error**：Gateway 的 EnvelopeGuardInterceptor 靠
// GetCode() 判定業務錯誤（proto 檔頭逐字：「漏了會靜默當成功」），而 9301／9302／9309／
// 9320／9330／9340 六個 not-found 類碼已登記在 envelopeCodeToGRPC 且 preserveBizCode。
// 回 status.Error 會讓那張表整個走不到，前端拿到的 code 變成 404 而不是業務碼。
//
// 🔴 **回 code≠0 時 data 一律留空、不得回 code=0 ＋ 空 data**：後者對前端是「查到了，
// 只是什麼都沒有」，會渲染成一張空卡片而不是「找不到」。
//
// 🔴 **Tier A（§8.1.4）**：擁有者述詞在 SQL 的 WHERE（repository 層），本層**不做**
// 「查回來再比對擁有者」。本層唯一的 if 是「這個 id 存不存在」，而它與越權共用同一個
// 業務碼、同一段 message、同一行程式碼。

// ---------------------------------------------------------------------------
// 買家
// ---------------------------------------------------------------------------

// ListBuyerInvoiceProfiles 是 B08「我的發票資料」列表（端點 5）。
func (s *InvoiceServer) ListBuyerInvoiceProfiles(
	ctx context.Context, req *pb.ListBuyerInvoiceProfilesRequest,
) (*pb.ListBuyerInvoiceProfilesResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parsePaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListBuyerProfiles(ctx, buyerID, page, pageSize)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListBuyerInvoiceProfilesResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_buyer_profiles.failed", err)
	}

	records := make([]*pb.InvoiceProfileInfo, 0, len(out.Records))
	for _, p := range out.Records {
		records = append(records, toProfileInfo(p))
	}
	return &pb.ListBuyerInvoiceProfilesResponse{
		Code:    0,
		Message: msgSuccess,
		Data: &pb.ListBuyerInvoiceProfilesData{
			Records:  records,
			Total:    fmtInt32(out.Total),
			Page:     fmtInt32(out.Page),
			PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// GetInvoiceComplaint 是買家看自己的一件申訴（端點 15）。
func (s *InvoiceServer) GetInvoiceComplaint(
	ctx context.Context, req *pb.GetInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}

	view, err := s.query.GetBuyerComplaint(ctx, buyerID, req.GetOrderNo(), complaintID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_buyer_complaint.failed", err)
	}
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: toBuyerComplaintInfo(view)}, nil
}

// ---------------------------------------------------------------------------
// 賣家
// ---------------------------------------------------------------------------

// GetSellerTaxProfile 是 S02 稅務身分現況（端點 20）。
func (s *InvoiceServer) GetSellerTaxProfile(
	ctx context.Context, req *pb.GetSellerTaxProfileRequest,
) (*pb.SellerTaxProfileResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerTaxProfile(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerTaxProfileResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_tax_profile.failed", err)
	}
	return &pb.SellerTaxProfileResponse{Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, view)}, nil
}

// GetSelfIssueAttestation 是 S04 自行開票責任條款的現況（端點 23）。
func (s *InvoiceServer) GetSelfIssueAttestation(
	ctx context.Context, req *pb.GetSelfIssueAttestationRequest,
) (*pb.SelfIssueAttestationResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerTaxProfile(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SelfIssueAttestationResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_self_issue_attestation.failed", err)
	}
	return &pb.SelfIssueAttestationResponse{
		Code: 0, Message: msgSuccess, Data: toSelfIssueAttestation(view.Profile),
	}, nil
}

// GetSellerProviderAccount 是 S03 AMEGO 憑證現況（端點 24）。
func (s *InvoiceServer) GetSellerProviderAccount(
	ctx context.Context, req *pb.GetSellerProviderAccountRequest,
) (*pb.SellerProviderAccountResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	view, err := s.query.GetSellerProviderAccount(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerProviderAccountResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_provider_account.failed", err)
	}
	return &pb.SellerProviderAccountResponse{Code: 0, Message: msgSuccess, Data: toProviderAccountData(view)}, nil
}

// ---------------------------------------------------------------------------
// ADM
// ---------------------------------------------------------------------------

// GetTaxReviewForAdmin 是 A03 稅務身分審核詳情（端點 39）。
func (s *InvoiceServer) GetTaxReviewForAdmin(
	ctx context.Context, req *pb.GetTaxReviewForAdminRequest,
) (*pb.GetTaxReviewForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	view, err := s.query.GetTaxReviewForAdmin(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetTaxReviewForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_tax_review_admin.failed", err)
	}
	return &pb.GetTaxReviewForAdminResponse{Code: 0, Message: msgSuccess, Data: s.toSellerTaxProfileData(ctx, view)}, nil
}

// GetProviderAccountForAdmin 是 A04 AMEGO 串接監控詳情（端點 42）。
func (s *InvoiceServer) GetProviderAccountForAdmin(
	ctx context.Context, req *pb.GetProviderAccountForAdminRequest,
) (*pb.GetProviderAccountForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	view, err := s.query.GetProviderAccountForAdmin(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetProviderAccountForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_provider_account_admin.failed", err)
	}
	return &pb.GetProviderAccountForAdminResponse{Code: 0, Message: msgSuccess, Data: toProviderAccountData(view)}, nil
}

// GetInvoiceDocumentForAdmin 是 A06 發票詳情（端點 48）。
//
// 這一支會揭露 P 類資料（買受人 email／載具明碼、provider 往返原文）⇒ usecase 在回資料
// 之前先寫一列 INVOICE_PII_REVEAL 稽核，寫不進去就不回（fail-closed，§8.1.3(c) 裁定 2）。
func (s *InvoiceServer) GetInvoiceDocumentForAdmin(
	ctx context.Context, req *pb.GetInvoiceDocumentForAdminRequest,
) (*pb.GetInvoiceDocumentForAdminResponse, error) {
	actorID, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}
	view, err := s.query.GetDocumentForAdmin(ctx, actorID, documentID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceDocumentForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_document_admin.failed", err)
	}
	return &pb.GetInvoiceDocumentForAdminResponse{
		Code: 0, Message: msgSuccess, Data: s.toAdminDocumentDetail(view),
	}, nil
}

// GetInvoiceJobForAdmin 是 A06 開票作業詳情（端點 51）。同樣留 PII 揭露稽核。
func (s *InvoiceServer) GetInvoiceJobForAdmin(
	ctx context.Context, req *pb.GetInvoiceJobForAdminRequest,
) (*pb.GetInvoiceJobForAdminResponse, error) {
	actorID, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseID(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "job_id: %v", err)
	}
	view, err := s.query.GetJobForAdmin(ctx, actorID, jobID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceJobForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_job_admin.failed", err)
	}
	return &pb.GetInvoiceJobForAdminResponse{Code: 0, Message: msgSuccess, Data: s.toAdminJobDetail(ctx, view)}, nil
}

// GetInvoiceComplaintForAdmin 是 A07/A08 申訴詳情（端點 59）。同樣留 PII 揭露稽核。
func (s *InvoiceServer) GetInvoiceComplaintForAdmin(
	ctx context.Context, req *pb.GetInvoiceComplaintForAdminRequest,
) (*pb.InvoiceComplaintResponse, error) {
	actorID, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}
	view, err := s.query.GetComplaintForAdmin(ctx, actorID, complaintID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_complaint_admin.failed", err)
	}
	info := toComplaintInfo(view)
	// 與端點 58（列表）同一個形狀：A08 的「賣家：{storeId}」讀這一欄（#3054 的稽查）。
	info.StoreId = fmtID(view.StoreID)
	s.fillOneBuyerName(ctx, info, view)
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: info}, nil
}

// ---------------------------------------------------------------------------
// 型別邊界
// ---------------------------------------------------------------------------

// msgSuccess 是 envelope 成功時的固定 message（§2 API 設計規範）。
const msgSuccess = "success"

// requirePrincipal 檢查 Gateway 注入的身分欄位。
//
// 🔴 0 一律拒絕，不得當成「沒有這個人」往下查：buyer_id=0 進 SQL 的 WHERE 會安靜地
// 回一個空列表，看起來像「這位買家沒有資料」。而 0 的真正意義是 **Gateway 沒注入**
// ——那是契約破了，不是查詢結果。§4 例外①（Request 內的身分欄位用 int64）只豁免型別，
// 不豁免驗證。
func requirePrincipal(v int64, field string) (int64, error) {
	if v <= 0 {
		return 0, status.Errorf(codes.InvalidArgument, "%s: %v", field, errNonPositiveID)
	}
	return v, nil
}

// parsePaging 解析分頁入參（皆為 string，§4）。空字串 ⇒ 0，交給 pagination 套預設值。
func parsePaging(page, pageSize string) (int32, int32, error) {
	p, err := parseInt32(page)
	if err != nil {
		return 0, 0, status.Errorf(codes.InvalidArgument, "page: %v", err)
	}
	ps, err := parseInt32(pageSize)
	if err != nil {
		return 0, 0, status.Errorf(codes.InvalidArgument, "page_size: %v", err)
	}
	return p, ps, nil
}

func parseInt32(s string) (int32, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// bizOf 把 usecase 的 DomainError 折成 envelope 的 (code, message)。
//
// 🔴 兩者都取自同一個 DomainError：not-found 與越權由 repository 的同一行 notFound()
// 產生，所以在這裡本來就沒有第二個分支可以回不同的東西（§8.1.4(c)）。
func bizOf(err error) (int32, string, bool) {
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		return 0, "", false
	}
	return int32(de.Code), de.DomainMessage(), true
}

// internal 記一筆並回 codes.Internal。故障不走 envelope：envelope 的非零 code 是
// 「業務上不成立」，把 DB 掛掉也塞進去會讓前端顯示一句業務文案而不是重試。
//
// 🔴 對外**只帶 event 名，不帶 err 原文**（#3049）：原本的 `%s: %v` 會把 PostgreSQL 的
// relation 名、constraint 名與 SQLSTATE 一路外送到瀏覽器（實例：
// `violates check constraint "chk_stp_required_needs_taxid" (SQLSTATE 23514)`）。
// 完整錯誤留在 log，查它的入口是回應裡的 traceId。
func (s *InvoiceServer) internal(_ context.Context, event string, err error) error {
	s.logger().Error(event, zap.Error(err))
	return status.Error(codes.Internal, event)
}

func fmtInt32(v int32) string { return strconv.FormatInt(int64(v), 10) }
func fmtInt64(v int64) string { return strconv.FormatInt(v, 10) }

// fmtID 把 id 折成對外字串；0 回空字串（proto3 沒有 null，空字串是「沒有這個關聯」）。
func fmtID(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func fmtIDPtr(v *int64) string {
	if v == nil {
		return ""
	}
	return fmtID(*v)
}

// fmtTime 一律 ISO 8601 UTC 秒精度；nil ⇒ 空字串。
func fmtTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return timeutil.FormatISO8601(*t)
}

func fmtTimeValue(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return timeutil.FormatISO8601(t)
}

// fmtDate 把 UTC+8 日界的日期折成 YYYY-MM-DD。
func fmtDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func firstNonEmptyStr(vals ...*string) string {
	for _, v := range vals {
		if s := str(v); s != "" {
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// mapper：申訴
// ---------------------------------------------------------------------------

// toComplaintInfo 把一件申訴折成 wire message。
//
// remaining_business_days／overdue_business_days 走 domain.BusinessCalendar（§5.1.14
// 的唯讀鏡像 ＋ 純函式），與 sla_due_at 同一套曆制。**算不出來時回空字串而不是 "0"**：
// "0" 會被前端讀成「今天到期」，而用自然日代替會偏早 ⇒ 賣家被誤判逾期 ⇒ 違規計數 +1。
// 空字串對應 58A 的「處理期限計算中」分支。
//
// ⚠️ **buyer_masked_name 不由本 mapper 填**：`invoice_complaints` 沒有買家姓名欄，
// 值要跨服務向 buyer-service 取 ⇒ 需要 ctx 與批次，而本函式是純的。
// 補值的唯一入口是 `buyer_names.go` 的 `fillBuyerNames`／`fillOneBuyerName`
// （#3017 後半，2026-09-02）；買家自己的視角（端點 13／14／15）刻意不補。
func toComplaintInfo(v usecase.ComplaintView) *pb.InvoiceComplaintInfo {
	c := v.Complaint
	out := &pb.InvoiceComplaintInfo{
		ComplaintId: fmtID(c.ID),
		// case_no ＝ 這件申訴的 TSID（#3054）。
		//
		// 🔴 這不是「找一個值來填」：58 §5.1.12 的 DDL 逐字寫著
		// `"id" bigint NOT NULL, -- TSID（對外＝案件編號）` ⇒ 案件編號的來源設計上
		// 就是本列的 id，本表**不需要**、也從來沒有第二個 case_no 欄位。
		// 之前恆為空字串 ⇒ B07 的「案件編號 {caseNo}」渲染成空白，買家向客服描述
		// 自己的案件時講不出任何識別碼（一張訂單可以有多次申訴，reopen_count 的存在
		// 說明重開是設計內的 ⇒ 只報訂單號不夠）。
		CaseNo:  fmtID(c.ID),
		OrderNo: str(c.OrderNo),
		// StoreId 由**呼叫端**視畫面需要覆寫（#3054 的稽查）：ADM 列表從 SQL join 帶回、
		// ADM 詳情由 usecase 解析 issuer 填入。買家面（B07）與賣家面（S10）的畫面都不
		// 顯示它 ⇒ 留空字串，不為了填滿契約多打一次 issuer 查詢。
		StoreId:     "",
		Status:      c.Status,
		DueAt:       fmtTimeValue(c.SlaDueAt),
		Description: str(c.Description),
		Version:     fmtInt64(c.Version),
		CreatedAt:   fmtTimeValue(c.CreatedAt),
		ReopenCount: fmtInt64(int64(c.ReopenCount)),
		// #3031：58A §S09 的申訴卡有一行「買家要的是：發票／收據」，來源是被申訴文件的
		// document_kind。恆為空字串時那一行是空的。
		ExpectedDocKind: v.ExpectedDocKind,
	}
	if v := v.RemainingBusinessDays; v != nil {
		out.RemainingBusinessDays = fmtInt32(*v)
	}
	if v := v.OverdueBusinessDays; v != nil {
		out.OverdueBusinessDays = fmtInt32(*v)
	}
	if c.Status == complaintStatusSellerResponded {
		out.BuyerReviewDueAt = fmtTime(c.BuyerReviewDueAt)
	}
	if c.Status == complaintStatusResolved {
		out.ReopenAllowedAfter = fmtTime(c.ReopenAllowedAfter)
	}
	for _, e := range v.Events {
		out.Responses = append(out.Responses, &pb.ComplaintResponseItem{
			ResponseId: fmtID(e.ID),
			Actor:      e.ActorType,
			Action:     str(e.Action),
			Note:       str(e.Message),
			OccurredAt: fmtTimeValue(e.OccurredAt),
		})
	}
	return out
}

// 兩個狀態值只在「這一欄該不該填」的判斷上用到；proto 對兩欄的要求逐字是
// 「非 SELLER_RESPONDED／非 RESOLVED 時為空字串」。
const (
	complaintStatusSellerResponded = "SELLER_RESPONDED"
	complaintStatusResolved        = "RESOLVED"
)

// ---------------------------------------------------------------------------
// mapper：賣家稅務資格
// ---------------------------------------------------------------------------

// toSellerTaxProfileData 把稅務資格折成 wire message。
//
// ⚠️ documents（TaxDocumentInfo）只填得出 file_key：evidence_object_keys 存的是
// objectKey 陣列，而 file_asset_id／file_name／file_size 要 media-service 查詢，
// 本波沒有那條 client。Gateway 靠 file_asset_id 組下載路徑 ⇒ 附件目前下載不到，
// 缺口登記在交付回報。
//
// ⚠️ adm_notes 只有一則：seller_tax_profiles.review_note 是單一 text 欄不是列表。
func (s *InvoiceServer) toSellerTaxProfileData(ctx context.Context, v usecase.SellerTaxProfileView) *pb.SellerTaxProfileData {
	p := v.Profile
	out := &pb.SellerTaxProfileData{
		StoreId:        fmtID(v.StoreID),
		ReviewStatus:   p.ReviewStatus,
		Version:        fmtInt64(p.Version),
		SubmittedAt:    fmtTime(p.SubmittedAt),
		DeadlineAt:     fmtTime(p.PendingValidUntil),
		ExtensionCount: fmtInt64(int64(p.ExtensionCount)),
		// 🔴 判準的唯一定義在 usecase.canRequestTaxExtension——**這裡不得自己算**。
		// 這一格在 2026-09-02 修 #3021 時被漏掉：當時只改了 SellerCapabilityView
		// 那一處（端點 16），而端點 17（S02 頁面實際讀的那一支）走的是本函式，
		// 於是修完之後 dev 上 canRequestExtension 對 SUBMITTED 的賣家**仍然是 true**。
		// 同一個判準有兩個獨立實作，改一個不會讓另一個轉紅——這正是它需要收斂的理由。
		CanRequestExtension:  usecase.CanRequestTaxExtension(p),
		ReapplyCooldownUntil: fmtTime(p.CooldownUntil),
		TaxIdentity:          toSellerTaxIdentity(p),
		SelfIssueAttestation: toSelfIssueAttestation(p),
		IntakeLimit:          toProfileIntakeLimit(p),
		Documents:            s.toTaxDocuments(ctx, p),
		// 收據代製授權（S05）。ADM 的 A03 靠它看見「核定之後賣家有沒有跟上」——
		// 沒跟上的話那位賣家每一筆已付訂單都落 AWAITING_MANUAL／RECEIPT_AUTH_MISSING，
		// 而那個狀態在 2026-09-05 之前沒有任何人推得動。
		//
		// ⚠️ 判準是 chk_stp_receipt_auth_pair 的那一對欄位（同進同出），
		// 不是 review_status：授權可以被賣家自己取消（端點 29 帶 authorized=false），
		// 那時狀態仍是 APPROVED_INVOICE_EXEMPT 而授權已經沒了。
		ReceiptAuthorized:   p.ReceiptAuthorizedAt != nil,
		ReceiptAuthorizedAt: fmtTime(p.ReceiptAuthorizedAt),
	}
	// 待裁決的展延申請。**掛在本函式而不是各個讀取 usecase**：五個呼叫點
	// （端點 17／39 兩支讀，端點 18／19／20／22 的寫入回應）全部經過這裡，
	// 掛在別處會出現「查詢看得到、剛按完看不到」這種只差一個入口的不一致
	// ——而端點 20 自己的回應正是賣家最需要看到它的地方。
	// s.query 為 nil 只出現在只組了寫入面的測試 server；讀取面的欄位缺席
	// 遠好過整支 RPC panic。
	if req := pendingExtensionOf(ctx, s.query, p); req != nil {
		out.PendingExtensionRequest = &pb.TaxExtensionRequestInfo{
			RequestedAt: fmtTime(&req.RequestedAt),
			Reason:      req.Reason,
		}
	}
	if note := str(p.ReviewNote); note != "" {
		out.AdmNotes = []*pb.TaxReviewNote{{
			NoteId:  fmtID(p.ID),
			Content: note,
			// 🔴 讀 review_note 專用的時間欄，**不是整列的 updated_at**（#3089 第 11 項）。
			//
			// 推動 updated_at 的是 BumpSellerTaxProfilePendingOrderCounters ——
			// **每一筆為該賣家建 document 的已付款訂單**都會打到它。dev 實測兩半：
			// 寫 review_note → 前進（對）；**只跑「接單累加」、完全沒碰 note → 也前進**（錯）。
			// 症狀是 A03 上那則備註的時間隨接單量一直往前跳，而備註一個字都沒改。
			// #3051 是同一個形狀（credential_updated_at 被健康檢查每分鐘改寫），
			// 那一次的修法就是加一個專用欄位 —— 這裡照做。
			CreatedAt: fmtTime(p.ReviewNoteAt),
		}}
	}
	return out
}

// addressDetailOf 折出 address_detail。
//
// 🔴 三段分開存（#3031）之後，business_address 是**串接後的整段**，直接回給
// address_detail 會讓前端把「台北市大安區忠孝東路一段1號」整串放進「詳細地址」那一格，
// 而縣市與行政區另外又各顯示一次。
//
// 判準是 **address_city 有沒有值**，不是「字串前綴對不對」：前綴比對要猜縣市名的寫法
// （台北市／臺北市），猜錯的症狀是地址少一截而沒有任何錯誤。
//   - 有 city（新資料）⇒ business_address 減去前兩段就是 detail
//   - 沒有 city（舊資料）⇒ 整段原樣回，前兩格留空：那是它當初唯一存得下來的形態
func addressDetailOf(p domain.SellerTaxProfile) string {
	full := str(p.BusinessAddress)
	city, district := str(p.AddressCity), str(p.AddressDistrict)
	if city == "" {
		return full
	}
	// ⚠️ 這個 early return 對「city 為空」而言是**冗餘**的（TrimPrefix 拿空前綴是 no-op），
	// 變異驗證時把它拿掉不會轉紅——那不是測試弱，是那個變異沒有鑑別力。
	// 留著是因為它把「舊資料原樣回」這個意圖寫在程式碼上，而不是靠讀者自己推導 TrimPrefix
	// 的邊界行為。真正有鑑別力的變異是把兩段的順序對調（實測轉紅）。
	rest := strings.TrimPrefix(full, city)
	rest = strings.TrimPrefix(rest, district)
	return rest
}

func toSellerTaxIdentity(p domain.SellerTaxProfile) *pb.SellerTaxIdentity {
	return &pb.SellerTaxIdentity{
		EntityType: str(p.EntityTaxType),
		// 🔴 tax_id 不遮罩：O 類（公開營業登記資訊），§8.1.3(b) 三端皆完整，
		// tax_id 是公開營業登記資訊。
		TaxId:           str(p.TaxID),
		LegalName:       str(p.LegalName),
		ResponsibleName: str(p.ResponsiblePerson),
		// #3031：三段地址分開回。address_detail 在 city／district 兩欄落地之前是
		// **串接後的整段**（那是唯一存得下來的東西）；之後的資料把三段各歸各位。
		//
		// 🔴 舊列（address_city 為 NULL）**必須退回舊行為**：把整段放進 detail、
		// 前兩格留空。改成一律拆的話 2026-09-02 之前的賣家會在 S02 看到一個
		// 少了縣市與行政區的地址，而那是他當初填對的資料。
		AddressCity:     str(p.AddressCity),
		AddressDistrict: str(p.AddressDistrict),
		AddressDetail:   addressDetailOf(p),
		ClaimedStatus:   p.ClaimedObligation,
	}
}

// toSelfIssueAttestation 折出自行開票責任條款現況。
//
// ⚠️ **terms_key 恆為空字串，而且短期內不會有值**（#3031 覆核）：它是「條款全文的
// media objectKey」，而**那份文件目前不存在於系統裡**——條款版本只是 usecase 的一個
// 日期常數（SelfIssueTermsVersion），docs/legal/ 這個目錄也還沒建。要填它得先把
// 條款全文發布成一份 media 資產，那是內容作業不是這一層補得出來的。
// 🔴 不得拿一個猜的 key 填：前端會拿它去組下載連結，而那條連結會 404。
//
// accepted_by_account_id 已於 2026-09-02 補上（#3031）：條款是**自然人的意思表示**
// （§6.1.7），只有版本與時間、沒有人，爭議時舉不出簽署主體。三元組
// （attestation_version／attested_at／attested_by_account_id）一起反正規化在
// seller_tax_profiles，兩個寫入點（T02 送審、T04 補簽）同一句寫。
//
// 🔴 terms_version 與 current_terms_version 是**兩件事**，不可合併：
// 前者是「這個賣家簽了哪一版」（未簽為空字串），後者是「現在要簽的是哪一版」（恆非空）。
// 端點 22 要求前端把當期版本原樣回送（不符即 400，58A §S04）⇒ 只給前者的話，
// 未簽過的賣家拿到空字串、回送必被拒，S04 的「同意」對新賣家從來按不成功。
//
// 🔴 **effective 與 accepted_at／terms_version 是兩個問題，不可互相推導**（裁決 C 之後）：
// 後兩者答「條款簽過沒」（＝`attested_at != nil`），effective 答「自行開票責任**生效中**沒」
// （＝`review_status == SELF_ISSUE_PENDING`，見 domain.SellerTaxProfile.SelfIssueEffective）。
// 端點 19 在送審那一句就寫 attested_at ⇒ 用它算 effective 會讓每個 SUBMITTED 的賣家
// 都被標成「生效中」，而後端同刻給的是 LEGACY_MANUAL。ADM 詳情頁把「已接受／未接受」
// 與「生效中／未生效」畫成相鄰兩列，正是要看出這個差別。
func toSelfIssueAttestation(p domain.SellerTaxProfile) *pb.SelfIssueAttestationInfo {
	return &pb.SelfIssueAttestationInfo{
		TermsVersion:        str(p.AttestationVersion),
		AcceptedAt:          fmtTime(p.AttestedAt),
		AcceptedByAccountId: fmtIDPtr(p.AttestedByAccountID),
		Effective:           p.SelfIssueEffective(),
		CurrentTermsVersion: usecase.SelfIssueTermsVersion,
	}
}

// toProfileIntakeLimit 折出待審接單上限。
//
// 非待審期間回 nil（proto InvoiceProfileInfo 的同一約定）：回一組數字會被前端讀成
// 「這個賣家有接單上限」⇒ 每一頁都渲染「待審期間接單限制 0/50 筆」，
// 而 58A 逐字要求「非待審期間為 null ⇒ 整塊不渲染」。
//
// 🔴 **述詞走 usecase.IntakeLimitApplies，不在這裡自己判**（#3089 第 10 項）。
// 舊版的述詞是「兩個 limit 欄皆為 0」——那是**欄位有沒有設定**，不是
// **現在適不適用**。dev 上 6/6 列都是 50/100000（DB 預設值）⇒ 它永不回 nil。
// 正解是 review_status ∈ {SELF_ISSUE_PENDING, SUPPLEMENT_REQUIRED}，
// 而那份判準在 usecase 早就有一份（結帳的 intakeLimitOf 在用）。
// 兩份實作必然漂移，而這一次漂移的方向是「永遠適用」。
func toProfileIntakeLimit(p domain.SellerTaxProfile) *pb.SellerIntakeLimit {
	if !usecase.IntakeLimitApplies(p) {
		return nil
	}
	return &pb.SellerIntakeLimit{
		OrderCountUsed:  fmtInt64(int64(p.PendingOrderCount)),
		OrderCountLimit: fmtInt64(int64(p.PendingOrderLimitCount)),
		AmountUsed:      fmtInt64(p.PendingOrderAmount),
		AmountLimit:     fmtInt64(p.PendingOrderLimitAmount),
		Blocked: p.PendingOrderCount >= p.PendingOrderLimitCount ||
			p.PendingOrderAmount >= p.PendingOrderLimitAmount,
	}
}

// toTaxDocuments 把 evidence_object_keys 折成附件清單。
func (s *InvoiceServer) toTaxDocuments(ctx context.Context, p domain.SellerTaxProfile) []*pb.TaxDocumentInfo {
	keys := p.EvidenceKeys()
	if len(keys) == 0 {
		return nil
	}
	// #3031：file_asset_id／file_name／file_size／uploaded_at 四欄原本恆為空字串
	// ——seller_tax_profiles 只存 objectKey 的陣列，其餘中介資料都在 media。
	//
	// file_asset_id 現在只表示「media 查得到這份資產」，不是 ADM 下載的輸入：後台沒有通用的
	// media 機敏下載，ADM 以 document_id（＋ store_id）經 invoice 的稅籍佐證端點讀（plan 8f-1）。
	// 逐頁向 media 批次查（BatchGetAssetMetadata），不在本服務另存一份會漂移的拷貝。
	meta := s.resolveEvidenceMetadata(ctx, keys, p.ID)

	out := make([]*pb.TaxDocumentInfo, 0, len(keys))
	for i, k := range keys {
		doc := &pb.TaxDocumentInfo{
			// document_id 是 evidence_object_keys 的 1 起算位置（domain.SellerTaxProfile.EvidenceKeys
			// 的順序），不是資產 id：它就是 ADM 下載端點
			// /admin/invoices/tax_reviews/{store_id}/documents/{document_id}/content 的定址，
			// 與 usecase.TaxEvidenceFile 讀的是同一個位置。
			DocumentId: fmtInt64(int64(i + 1)),
			FileKey:    k,
		}
		if m, ok := meta[k]; ok {
			doc.FileAssetId = m.AssetID
			doc.FileName = m.FileName
			doc.FileSize = m.SizeBytes
			doc.UploadedAt = m.UploadedAt
		}
		out = append(out, doc)
	}
	return out
}

// ---------------------------------------------------------------------------
// mapper：provider 帳號
// ---------------------------------------------------------------------------

// toProviderAccountData 折出 AMEGO 帳號現況。
//
// 🔴 這裡永遠不會出現 app_key 或 credential_fingerprint_prefix：proto 用 reserved 90 與
// reserved 13 占住那兩個名字，讓「不小心加回去」變成 buf build 失敗而不是靜默外洩（§8.1.2 層 1）。
//
// ⚠️ 欄名已於 2026-09-02 由 seller_tax_id_masked 改為 seller_tax_id（#3017，
// DEC-20260902-163458 選 B「直接切」）。值一直都是**完整**的：§8.1.3(b) 第二列逐字把
// 「賣家統編 provider_tax_id」判為 O 類、三端皆完整——舊欄名帶著 Masked 卻回完整值，
// 那是欄名說謊而不是值錯，所以修的是名字不是值。
func toProviderAccountData(v usecase.ProviderAccountView) *pb.SellerProviderAccountData {
	a := v.Account
	out := &pb.SellerProviderAccountData{
		Provider:        a.Provider,
		Status:          a.Status,
		StatusNotice:    str(a.StatusReason),
		Version:         fmtInt64(a.Version),
		SellerTaxId:     a.ProviderTaxID,
		CredentialLast4: a.CredentialLast4,
		// 🔴 讀憑證專用欄位，不是整列的 updated_at（#3051）：後者被健康檢查輪詢
		// 每分鐘改寫，於是這個「AppKey 更新於」永遠在跳、真的被輪換時反而看不出來。
		CredentialUpdatedAt: fmtTime(a.CredentialUpdatedAt),
		LastVerifiedAt:      fmtTime(a.VerifiedAt),
		AutomationCutoverAt: fmtTime(a.AutomationCutoverAt),
	}
	if v.HasProfile {
		out.PendingOrderCount = fmtInt64(int64(v.Profile.PendingOrderCount))
		// #3089 第 7 項：consent_version 從未被填 —— 來源一直都在
		// （seller_tax_profiles.receipt_consent_version，收據代製授權當下寫入），
		// 只是這支 mapper 漏了它。空字串在 A09／S09 會被讀成「還沒授權過」，
		// 而賣家其實簽過；授權版本正是「他簽的是哪一版」的唯一憑據。
		// 沒有 profile ⇒ 不填（與 PendingOrderCount 同一個理由：數字不可信）。
		out.ConsentVersion = str(v.Profile.ReceiptConsentVersion)
	}
	// #3031：最近一次 AMEGO 往返的 correlation_id。賣家回報問題時給客服追查用；
	// 還沒打過 AMEGO 時為空字串（合法狀態，不是缺口）。
	out.LastRequestId = v.LastRequestID
	return out
}

// ---------------------------------------------------------------------------
// mapper：ADM 文件與作業
// ---------------------------------------------------------------------------

func (s *InvoiceServer) toAdminDocumentDetail(v usecase.AdminDocumentView) *pb.AdminInvoiceDocumentDetail {
	out := &pb.AdminInvoiceDocumentDetail{
		SellerView: s.toSellerDocumentDetail(v),
	}
	for _, ex := range v.Exchanges {
		out.ProviderExchanges = append(out.ProviderExchanges, toProviderExchange(ex))
	}
	for _, op := range v.Operations {
		out.RelatedJobs = append(out.RelatedJobs, s.toJobInfo(op, v.Document, v.Issuer))
	}
	// #3031：idempotency_key 取**最近一次** operation 的 operation_key。
	//
	// invoice_operations 沒有名為 idempotency_key 的欄位，但 operation_key 就是它
	// （`domain.OperationKeyFor` 純函式推導、全域唯一、**重試不換鍵**——那正是冪等鍵的定義）。
	// ADM 拿它去比對呼叫端的請求，或在申訴／客服時當追查憑據。
	//
	// ⚠️ 取第一筆而不是串接全部：`ListByDocument` 是 created_at DESC ⇒ [0] 是最近一次，
	// 與同一個 mapper 裡 latest_job 的取法同源。一份文件沒有任何 operation 時留空字串。
	if len(v.Operations) > 0 {
		out.IdempotencyKey = v.Operations[0].OperationKey
	}
	// 端點 73 的兩個輸入。version 是 document 自己的樂觀鎖；can_resume 的述詞與寫端
	// 共用同一支函式（分家的後果是按鈕亮在一個 CAS 必定 rows=0 的狀態上，而 rows=0
	// 是靜默的 —— #3031／#3089 第 8 項兩次都是這樣）。
	out.DocumentVersion = fmtInt64(v.Document.Version)
	out.CanResumeFromManual = usecase.CanResumeDocumentFromManual(v.Document.Status, len(v.Operations))
	return out
}

// toSellerDocumentDetail 折出賣家視角的文件詳情。
//
// 🔴 funded_by 恆為 UNDETERMINED：三值 SELLER／PLATFORM／UNDETERMINED 由 coupon 的
// created_by 推導（§3.7 D9a E4 fail-closed），而本服務沒有 coupon client。
// 「不知道」的正確答案是 UNDETERMINED，不是猜一個 SELLER —— proto 逐字寫了
// 「不得寫成常數」，而 fail-closed 的那一格正是它要的常數。
func (s *InvoiceServer) toSellerDocumentDetail(v usecase.AdminDocumentView) *pb.SellerInvoiceDocumentDetail {
	doc := v.Document
	latestJob := ""
	if len(v.Operations) > 0 {
		latestJob = fmtID(v.Operations[0].ID) // ListByDocument 是 created_at DESC
	}
	return &pb.SellerInvoiceDocumentDetail{
		Detail:            s.toDocumentDetail(v),
		FundedBy:          fundedByUndetermined,
		CutoverVersion:    fmtIDPtr(doc.ProviderAccountVersion),
		ProviderRequestId: str(doc.ProviderRequestNo),
		ManualReasonCode:  str(doc.ManualReasonCode),
		LatestJobId:       latestJob,
	}
}

// fundedByUndetermined 是折抵出資方無法判定時的 fail-closed 值（§3.7 D9a）。
const fundedByUndetermined = "UNDETERMINED"

func (s *InvoiceServer) toDocumentDetail(v usecase.AdminDocumentView) *pb.InvoiceDocumentDetail {
	doc := v.Document
	out := &pb.InvoiceDocumentDetail{
		Summary:        s.toDocumentSummary(v),
		IssuerName:     doc.IssuerNameSnapshot,
		IssuerTaxId:    str(doc.IssuerTaxIDSnapshot),
		RecipientName:  doc.RecipientName,
		RecipientTaxId: doc.RecipientTaxID,
		SalesAmount:    fmtInt64(doc.SalesAmount),
		TaxAmount:      fmtInt64(doc.TaxAmount),
		TotalAmount:    fmtInt64(doc.TotalAmount),
		TaxCategory:    fmtInt64(int64(doc.TaxType)),
		InvoiceFileKey: str(doc.PdfObjectKey),
	}
	for _, l := range v.Lines {
		out.Lines = append(out.Lines, &pb.InvoiceLine{
			LineType:    l.LineType,
			Name:        l.Description,
			Quantity:    fmtInt64(int64(l.Quantity)),
			UnitPrice:   fmtInt64(l.UnitPrice),
			NetAmount:   fmtInt64(l.NetAmount),
			TaxCategory: fmtInt64(int64(l.TaxCategory)),
		})
	}
	for _, h := range v.Timeline {
		out.Timeline = append(out.Timeline, &pb.InvoiceTimelineItem{
			OccurredAt: fmtTimeValue(h.OccurredAt),
			Event:      h.ToStatus,
			Note:       str(h.ReasonDetail),
		})
	}
	return out
}

// toDocumentSummary 折出文件摘要。
//
// 🔴 carrier_summary 在**這條 ADM 路徑**上是明碼（§8.1.3(c) 裁定 2「ADM 端一律不遮罩，
// 但每次揭露都要留稽核」，稽核已由 usecase 在取資料前寫好）。同一個欄位在賣家路徑
// （端點 31，本波未落地）必須改走 usecase.MaskCarrierID —— 兩者共用 proto 欄位但**不共用
// 這個 mapper**，賣家版落地時要傳遮罩後的值進來，不是改這裡。
//
// 🔴 invoice_number 三端皆完整（O 類，§8.1.3(c) 裁定 1 逐字推翻 img-05 的遮罩畫法）：
// 它是對獎／報帳／申訴的唯一憑據，而且同時印在發票上、寄在通知信裡、存在載具裡。
func (s *InvoiceServer) toDocumentSummary(v usecase.AdminDocumentView) *pb.InvoiceDocumentSummary {
	// #3084：carrier_type 是 carrier_summary 的判別欄，兩者必須一起決定。
	// ADM 面看明碼（RevealedCarrierID），所以「非空 ＝ 這是載具」在這條路徑上成立；
	// 為空才退回 fallback（那條的 carrierType 恆空，見該函式註解）。
	carrier, carrierType := v.RevealedCarrierID, ""
	if carrier == "" {
		carrier, carrierType = carrierSummaryFallback(v.Document)
	} else if kind, ok := domain.CarrierTypeFromAmego(str(v.Document.CarrierType)); ok {
		carrierType = kind.Value
	}
	return s.toSummaryWithCarrier(v.Document, carrier, carrierType)
}

func firstNonEmptyID(vals ...*int64) string {
	for _, v := range vals {
		if s := fmtIDPtr(v); s != "" {
			return s
		}
	}
	return ""
}

func (s *InvoiceServer) toAdminJobDetail(ctx context.Context, v usecase.AdminJobView) *pb.AdminInvoiceJobDetail {
	out := &pb.AdminInvoiceJobDetail{
		Job:            s.toJobInfo(v.Operation, v.Document, v.Issuer),
		IdempotencyKey: v.Operation.OperationKey,
	}
	if len(v.Document.SourceSnapshot) > 0 {
		out.OrderSnapshotJson = string(v.Document.SourceSnapshot)
	}
	// #3031：A06 第 3 層的金額拆解。58A 逐字「與結帳 preview 同一個 DTO」⇒
	// 來源是 order_snapshot_json 的結帳分解，不是 invoice_documents 的 AMEGO 稅別分解。
	// 🔴 解不開就留 nil，**不要填一組零值**：A06 會把它畫成「商品 0 元、運費 0 元」，
	// 那是一個看起來完全正常的錯誤畫面。
	if amt, ok := usecase.ParseSnapshotAmounts(v.Document.SourceSnapshot); ok {
		out.AmountBreakdown = &pb.InvoiceAmountBreakdown{
			ProductAmount:  fmtInt64(amt.ProductAmount),
			ShippingAmount: fmtInt64(amt.ShippingAmount),
			DiscountAmount: fmtInt64(amt.DiscountAmount),
			InvoiceAmount:  fmtInt64(amt.InvoiceAmount),
			TaxCategory:    fmtInt64(int64(amt.TaxCategory)),
		}
	}
	for _, ex := range v.Exchanges {
		out.ProviderExchanges = append(out.ProviderExchanges, toProviderExchange(ex))
	}
	for _, l := range v.AuditLogs {
		out.AuditLogs = append(out.AuditLogs, toAuditLogItem(l))
	}
	// #3089 第 7 項：shipment_status 在此之前**零賦值恆空**——resolver 是活的
	// （S06／A12 的列表面實測回得出值），只有這支詳情 mapper 沒接。
	//
	// 🔴 **必須同時帶 degraded**：A12 逐字寫著「這一欄尤其不能畫：空白會被讀成
	// 『還沒出貨』」。取不到時只回空字串等於讓畫面說謊，那正是 #3019 修列表面時
	// already 付過的學費（見 shipment_status.go 檔頭）。
	if orderNo := out.Job.GetOrderNo(); orderNo != "" {
		statuses, degraded := s.resolveShipmentStatuses(ctx, []string{orderNo})
		out.ShipmentStatus = statuses[orderNo]
		out.ShipmentStatusDegraded = degraded
	}
	// manual_unlock 為 nil ＝ **從未發起過解鎖**（proto 逐字「null = 尚無解鎖請求」）。
	// 🔴 不要在這裡回一個空物件補位：58A §A06 用 `manualUnlock === null` 決定
	// 「申請解鎖」按鈕的可見性，空物件會讓那顆按鈕消失，而 A4 就沒有任何入口。
	if v.ManualUnlock != nil {
		out.ManualUnlock = toManualUnlockInfo(*v.ManualUnlock)
	}
	return out
}

// toJobInfo 折出一件開票作業。
//
// 🔴 order_no 一律由 invoice_documents.source_ref 取（且 source_type='ORDER'）：
// invoice_operations 完全沒有訂單欄（§8.1.4 末段），自行反正規化一個 order_no 就是
// 在製造第二個真實來源。
//
// 🔴 summary 用 CodeMeta 的 Label，不含 provider code（proto 逐字「不含 provider code」）：
// AMEGO 的 code 只出現在 ADM 的 provider_exchanges 那一層。
func (s *InvoiceServer) toJobInfo(op domain.Operation, doc domain.Document, issuer domain.Issuer) *pb.InvoiceJobInfo {
	orderNo := ""
	if doc.SourceType == sourceTypeOrder {
		orderNo = str(doc.SourceRef)
	}
	return s.toJobInfoView(domain.OperationView{
		Operation:   op,
		IssuerRefID: issuer.IssuerRefID,
		// 🔴 DocumentKind 不得省略。它餵兩件事：`noticeForKind`（#3066 裁決 B 的
		// 收據／發票兩句文案）與 proto 的 `document_kind`（前端分辨並列的兩個區塊，
		// **不得從 operation_type 推導**——那個 enum 沒有 RECEIPT）。
		// 漏掉不會編譯錯、不會讓任何測試轉紅，症狀是這兩個入口（端點 51 詳情、
		// 端點 48 的 relatedJobs）的 documentKind 恆為空字串，而列表面（端點 50／33）
		// 走 toJobInfoView 的 SQL 版本有值 ⇒ 同一份契約在兩個入口說不同的話。
		// dev 實測三種 operation type 皆為 ""（2026-09-05）。
		DocumentKind:             doc.DocumentKind,
		DocumentStatus:           doc.Status,
		DocumentSourceRef:        orderNo,
		DocumentManualReasonCode: str(doc.ManualReasonCode),
		ExternalInvoiceNo:        str(doc.ExternalInvoiceNo),
		ExternalInvoiceDate:      doc.ExternalInvoiceDate,
		ExternalReportedAt:       doc.ExternalReportedAt,
	})
}

const (
	sourceTypeOrder                     = "ORDER"
	operationStatusManualProcessAllowed = "MANUAL_PROCESS_ALLOWED"
	// 🔴 #3066 裁決 B：收據與發票在賣家中心是兩個並列的東西，文案各自為政。
	// 免用統一發票的賣家（document_kind=RECEIPT）永遠不會有發票 ⇒ 對他講「發票開立」
	// 是在描述一件他這輩子不會遇到的事。**兩句話而不是一句中性的**——中性化是被
	// 否決的 A 案。前端只把這兩句當 fallback，真正顯示的是這裡送出去的字串。
	shipmentUnaffectedNotice        = "發票開立與出貨進度互不影響，出貨不會因為發票異常而停下。"
	shipmentUnaffectedNoticeReceipt = "收據開立與出貨進度互不影響，出貨不會因為收據異常而停下。"
)

// noticeForKind 依 document_kind 選 info 行。
//
// 判準是 kind 不是 operation_type：InvoiceOperationType 沒有 RECEIPT
// （收據是 document_kind，proto InvoiceJobInfo 第 4 欄註解逐字寫明）。
// 未知 kind 退回發票措辭——絕大多數賣家開的是發票，且猜錯的代價不對稱。
func noticeForKind(documentKind string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return shipmentUnaffectedNoticeReceipt
	}
	return shipmentUnaffectedNotice
}

// toProviderExchange 折出一次 AMEGO 往返。
//
// 欄名帶 masked，而 invoice_provider_audit_logs 存的是**原文**（§8.1.1 裁定：儲存原文、
// 遮罩只發生在顯示層，理由是這張表存在的唯一目的是對外舉證）。ADM 是唯一讀者且已留
// PII 揭露稽核 ⇒ 這條路徑直出。**任何非 ADM 的消費者要用這兩欄，必須先過遮罩**。
func toProviderExchange(e domain.ProviderAuditLogEntry) *pb.ProviderExchangeItem {
	return &pb.ProviderExchangeItem{
		ExchangeId:   fmtID(e.ID),
		OccurredAt:   fmtTimeValue(e.OccurredAt),
		Endpoint:     e.Endpoint,
		ProviderCode: str(e.ProviderCode),
		// #3089 第 7 項：在此之前這一欄**零賦值恆空**，而 proto 註解逐字要求
		// 「必須同時保存 msg：AMEGO 的 53 在官方表出現兩次語意不同」——
		// 只給 code 的話 ADM 分不出是哪一種。成功的往返沒有 msg，回空字串是對的。
		ProviderMessage: str(e.ProviderMessage),
		// #3031：request_id ＝ invoice_provider_audit_logs.correlation_id。
		// 表上沒有名為 request_id 的欄位，correlation_id 就是它：那是這一次往返在
		// 我方 log／trace 上的識別碼，ADM 拿它去 Loki 追同一次呼叫的上下文。
		// 恆為空字串的後果是 A06 第 4 層每一列都追不回去。
		RequestId:          str(e.CorrelationID),
		MaskedRequestBody:  string(e.RequestMasked),
		MaskedResponseBody: string(e.ResponseMasked),
	}
}

func toAuditLogItem(l domain.AdminAuditLogEntry) *pb.InvoiceAuditLogItem {
	return &pb.InvoiceAuditLogItem{
		AuditLogId:     fmtID(l.ID),
		OccurredAt:     fmtTimeValue(l.OccurredAt),
		ActorAccountId: fmtID(l.ActorAccountID),
		OperationType:  l.Action,
		TargetKind:     l.ResourceType,
		TargetId:       fmtID(l.ResourceID),
		BeforeJson:     string(l.BeforeState),
		AfterJson:      string(l.AfterState),
		Reason:         l.ReasonDetail,
		RequestId:      str(l.RequestID),
	}
}

// pendingExtensionOf 把 nil query 的情形收在一處，讓 mapper 本體維持單一表達式。
func pendingExtensionOf(ctx context.Context, q *usecase.Query, p domain.SellerTaxProfile) *usecase.TaxExtensionRequest {
	if q == nil {
		return nil
	}
	return q.PendingTaxExtensionRequest(ctx, p)
}
