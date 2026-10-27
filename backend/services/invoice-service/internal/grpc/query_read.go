package grpc

import (
	"context"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是 §6.1 剩餘唯讀 RPC 的 Layer 5（買家 B04/B06/B07/B09、賣家 S01/S06/S07/S08/S09/S12
// ＋ 62R、ADM A01/A02/A04/A05/A07/A10/A12）。形狀與 query.go 逐條相同：
//
//	① 業務錯誤走 envelope（回 code≠0 ＋ 空 data），不走 gRPC error
//	② 契約違規（身分缺席／id 解析不了／日期格式／視圖名不認得／page_size 超上限）走
//	   codes.InvalidArgument —— 那不是「業務上不成立」，是呼叫端把契約打破了
//	③ 本層不做擁有者比對：述詞在 SQL，本層只搬型別

// ---------------------------------------------------------------------------
// 買家：B09 發票中心（端點 68／69）與 B05 文件詳情（端點 9）
// ---------------------------------------------------------------------------

// ListBuyerInvoiceDocuments 是 B09 買家發票中心（端點 68）。
func (s *InvoiceServer) ListBuyerInvoiceDocuments(
	ctx context.Context, req *pb.ListBuyerInvoiceDocumentsRequest,
) (*pb.ListBuyerInvoiceDocumentsResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	dates, err := parseDates(req.GetDateFrom(), req.GetDateTo())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListBuyerDocuments(ctx, usecase.BuyerDocumentsInput{
		BuyerID:      buyerID,
		Dates:        dates,
		Status:       req.GetStatus(),
		DocumentKind: req.GetDocumentKind(),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListBuyerInvoiceDocumentsResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_buyer_documents.failed", err)
	}

	records := make([]*pb.BuyerInvoiceDocumentItem, 0, len(out.Records))
	for _, item := range out.Records {
		records = append(records, &pb.BuyerInvoiceDocumentItem{
			Summary: s.toSummary(item.Document, item.RecipientNote),
			// 開立人 ＝ 付款當下的快照，不回查現況（§5.1.5）。58A §B09 逐字把它標為
			// 硬需求：「付款商、平台與發票開立人不能混為一談」⇒ 不得省略、
			// 也不得用「Hawkr」統一填充。
			IssuerName: item.Document.IssuerNameSnapshot,
		})
	}
	return &pb.ListBuyerInvoiceDocumentsResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListBuyerInvoiceDocumentsData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// GetBuyerInvoiceDocument 是 B05 的 document-scoped 入口（端點 69）。
//
// 不帶 order_no：方向三（平台 → 買家）的發票沒有訂單，帶了就填不出來。
func (s *InvoiceServer) GetBuyerInvoiceDocument(
	ctx context.Context, req *pb.GetBuyerInvoiceDocumentRequest,
) (*pb.GetInvoiceDocumentResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}

	view, err := s.query.GetBuyerDocument(ctx, buyerID, documentID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceDocumentResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_buyer_document.failed", err)
	}
	return &pb.GetInvoiceDocumentResponse{Code: 0, Message: msgSuccess, Data: s.toDetail(view)}, nil
}

// GetInvoiceDocument 是 B05 的 order-scoped 入口（端點 9）。
//
// order_no 不匹配走**同一個** 9301（§8.1.4(c)）：另開一個碼等於在回應裡放一個
// 可以區分「不是你的」與「不存在」的旗標。
func (s *InvoiceServer) GetInvoiceDocument(
	ctx context.Context, req *pb.GetInvoiceDocumentRequest,
) (*pb.GetInvoiceDocumentResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}

	view, err := s.query.GetBuyerOrderDocument(ctx, buyerID, req.GetOrderNo(), documentID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceDocumentResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_order_document.failed", err)
	}
	return &pb.GetInvoiceDocumentResponse{Code: 0, Message: msgSuccess, Data: s.toDetail(view)}, nil
}

// ---------------------------------------------------------------------------
// 買家：B04 訂單發票（端點 8）
// ---------------------------------------------------------------------------

// GetOrderInvoice 回一張訂單的發票現況（B04，端點 8）。
//
// 🔴 **documents 是 repeated 且必須全帶**：一張訂單在 VOID_AND_REISSUE 之後會有多份
// 文件（舊的 SUPERSEDED、新的指回鏈頭）。只取第一筆的實作會讓買家看到作廢前的舊發票，
// 而畫面完全正常 —— 那正是 ListOrderDocumentsForBuyer 被寫成 :many 的理由。
func (s *InvoiceServer) GetOrderInvoice(
	ctx context.Context, req *pb.GetOrderInvoiceRequest,
) (*pb.GetOrderInvoiceResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	if req.GetOrderNo() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order_no: %v", errEmptyID)
	}

	view, err := s.query.GetOrderInvoice(ctx, buyerID, req.GetOrderNo())
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetOrderInvoiceResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_order_invoice.failed", err)
	}

	documents := make([]*pb.InvoiceDocumentSummary, 0, len(view.Documents))
	latestKind := ""
	for i, d := range view.Documents {
		if i == 0 {
			// view.Documents[0] 就是 usecase 那邊的 latest（排序見 ListOrderDocumentsForBuyer）。
			latestKind = d.Document.DocumentKind
		}
		documents = append(documents, s.toSummaryWithCarrier(d.Document, d.CarrierSummary, d.CarrierType))
	}
	return &pb.GetOrderInvoiceResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.OrderInvoiceData{
			OrderNo:              view.OrderNo,
			InvoiceMode:          view.Mode,
			State:                view.State,
			StateNotice:          view.StateNotice,
			Documents:            documents,
			CanRequestCorrection: view.CanRequestCorrection,
			CanFileComplaint:     view.CanFileComplaint,
			ComplaintOpensAt:     fmtTime(view.ComplaintOpensAt),
			ExistingComplaintId:  fmtID(view.ExistingComplaintID),
			// 端點 12 的唯一入口。fmtID(0) 是空字串 ⇒ 從未申請過時前端不畫任何區塊。
			LatestCorrectionId:  fmtID(view.LatestCorrectionID),
			NotifiedEmailMasked: view.NotifiedEmailMasked,
			SnapshotVersion:     fmtInt32(view.SnapshotVersion),
			// #3031：這一欄原本恆為空字串，理由寫著「設計書全文沒有指定這句文案」
			// ——**那句話只成立於 58 號**：定版字串在 **58A §B04** 的畫面表逐字給了
			// （「物流費用說明｜逐字顯示後端 shippingFeeNotice（定版字串：…）」），
			// 而 B04 是 info banner 恆顯示的那一句 ⇒ 空字串等於那條 banner 沒有內容。
			// 常數放 usecase 與 §6.1.3 的文案表同層，兩個畫面（B04／B05）共用一份。
			//
			// 🔴 **依 document_kind 選**（#3026）：原句逐字提到兩次「發票」，
			// 掛在收據上就是把收據講成發票。dev 實測 O481452453 拿到的是收據
			// R000000000003，而這條 banner 說「本張發票的銷售額」。
			ShippingFeeNotice: usecase.ShippingFeeNoticeFor(latestKind),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// 買家：B06 修改申請（端點 12）與 B07 申訴列表（端點 13 GET）
// ---------------------------------------------------------------------------

// GetInvoiceCorrection 回買家自己的一件修改申請（端點 12）。
func (s *InvoiceServer) GetInvoiceCorrection(
	ctx context.Context, req *pb.GetInvoiceCorrectionRequest,
) (*pb.InvoiceCorrectionResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	correctionID, err := parseID(req.GetCorrectionId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "correction_id: %v", err)
	}
	if req.GetOrderNo() == "" {
		// order_no 是三段擁有者述詞之一（§8.1.4(b)）⇒ 缺席不是「不篩」而是契約破了。
		// 當成「不篩」會把三段述詞降成兩段，而那一段正是防「ADM 事後改過買受人」的。
		return nil, status.Errorf(codes.InvalidArgument, "order_no: %v", errEmptyID)
	}

	row, err := s.query.GetBuyerCorrection(ctx, buyerID, req.GetOrderNo(), correctionID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceCorrectionResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_correction.failed", err)
	}
	return &pb.InvoiceCorrectionResponse{Code: 0, Message: msgSuccess, Data: toCorrectionData(row)}, nil
}

// toCorrectionData 折出 B06 的進度卡。
//
// handling_mode 與 status 是兩個不同的問題、值域刻意零交集（§6.1.5）⇒ 兩欄各自直傳，
// 不需要（也不存在）DTO↔DB 映射層。
//
// 🔴 status_notice 在 2026-09-02 之前恆為空字串，理由寫著「這一欄沒有被規定的內容」
// ——**那是把兩件事混成一件**（#3031）：58A §B06 逐字寫著「`statusNotice`→成功 toast 的
// body（逐字）」，前端**顯示**它，只是不**解析**它去決定行為（那是 rejectReasonCode 的
// 職責）。恆為空的結果是那個 toast 沒有內文。文案表在
// usecase.CorrectionStatusNotices，規約與 orderInvoiceStateNotices 同源。
func toCorrectionData(r domain.CorrectionRequest) *pb.InvoiceCorrectionData {
	return &pb.InvoiceCorrectionData{
		CorrectionId: fmtID(r.ID),
		HandlingMode: str(r.HandlingMode),
		Status:       r.Status,
		CreatedAt:    fmtTimeValue(r.CreatedAt),
		StatusNotice: usecase.CorrectionStatusNotices[r.Status],
		// status=REJECTED 時必填、其餘空字串（DB 的 chk 已保證這個對應）。
		RejectReasonCode: str(r.RejectReasonCode),
	}
}

// ListOrderInvoiceComplaints 回這張訂單底下這位買家的申訴案（端點 13 的 GET）。
//
// 它是「拿到 9310 之後要看得到既有案件」的落點；清單端點的 Tier A 正確形狀是空 records。
func (s *InvoiceServer) ListOrderInvoiceComplaints(
	ctx context.Context, req *pb.ListOrderInvoiceComplaintsRequest,
) (*pb.ListInvoiceComplaintsResponse, error) {
	buyerID, err := requirePrincipal(req.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	if req.GetOrderNo() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order_no: %v", errEmptyID)
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListBuyerOrderComplaints(ctx, buyerID, req.GetOrderNo(), page, pageSize)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListInvoiceComplaintsResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_order_complaints.failed", err)
	}
	return s.complaintListResponse(ctx, out, toBuyerComplaintInfo, false), nil
}

// complaintListResponse 組裝清單 envelope。
//
// 🔴 conv 必填、沒有預設值：買家面（端點 14）必須傳 toBuyerComplaintInfo，賣家面
// （端點 34）傳 toComplaintInfo。給一個預設值等於讓「忘了傳」靜默走到某一邊，
// 而走錯的那一邊（買家拿到 ADM 全文）不會有任何症狀（見 complaint_buyer_view.go）。
//
// #3017：改成 method 是為了拿到 ctx 去補 buyer_masked_name。**補名字放在這裡而不是
// 各呼叫端**：這是清單路徑的唯一收斂點，放在這裡就沒有人能忘記。
// 買家面（端點 14）不補——那是買家自己的名字，58A 只在賣家面（S09／S10）與
// ADM 面（A06／A07）消費這一欄，為它多打一次 RPC 沒有任何人會看到。
func (s *InvoiceServer) complaintListResponse(
	ctx context.Context,
	out usecase.ComplaintPage, conv func(usecase.ComplaintView) *pb.InvoiceComplaintInfo,
	withBuyerNames bool,
) *pb.ListInvoiceComplaintsResponse {
	records := make([]*pb.InvoiceComplaintInfo, 0, len(out.Records))
	for _, c := range out.Records {
		records = append(records, conv(c))
	}
	if withBuyerNames {
		s.fillBuyerNames(ctx, records, out.Records)
	}
	return &pb.ListInvoiceComplaintsResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListInvoiceComplaintsData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}
}

// ---------------------------------------------------------------------------
// 賣家：S01 總覽（端點 16）
// ---------------------------------------------------------------------------

// GetSellerInvoiceOverview 是 S01 發票與稅籍總覽（端點 16）。
//
// 🔴 **沒有任何業務碼**：賣家還沒有 issuer／稅務資格／provider 帳號都是正常狀態
// （上線當天的全體），一律回 code 0 ＋ 三態投影值。回 404 會讓每一位賣家的
// 側欄入口都是死的。
func (s *InvoiceServer) GetSellerInvoiceOverview(
	ctx context.Context, req *pb.GetSellerInvoiceOverviewRequest,
) (*pb.GetSellerInvoiceOverviewResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}

	view, err := s.query.GetSellerInvoiceOverview(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetSellerInvoiceOverviewResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_overview.failed", err)
	}

	recent := make([]*pb.InvoiceDocumentSummary, 0, len(view.RecentDocuments))
	for _, d := range view.RecentDocuments {
		recent = append(recent, s.toSummaryWithCarrier(d.Document, d.CarrierSummary, d.CarrierType))
	}
	return &pb.GetSellerInvoiceOverviewResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.SellerInvoiceOverviewData{
			TaxReviewStatus:       view.Capability.TaxReviewStatus,
			InvoiceMode:           view.Capability.InvoiceMode,
			ProviderStatus:        view.Capability.ProviderStatus,
			PendingDocumentCount:  fmtInt64(view.PendingDocumentCount),
			OpenComplaintCount:    fmtInt64(view.OpenComplaintCount),
			OverdueComplaintCount: fmtInt64(view.OverdueComplaintCount),
			DeadlineAt:            fmtTime(view.Capability.DeadlineAt),
			ExtensionCount:        fmtInt64(int64(view.Capability.ExtensionCount)),
			CanRequestExtension:   view.Capability.CanRequestExtension,
			SelfIssueEffective:    view.Capability.SelfIssueEffective,
			IntakeLimit:           toIntakeLimit(view.Capability.IntakeLimit),
			RecentDocuments:       recent,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// 賣家：S06 銷項（端點 30）／S07 明細（端點 31）／S12 進項（端點 72）
// ---------------------------------------------------------------------------

// ListSellerInvoiceDocuments 是 S06 賣家銷項紀錄（端點 30）。
func (s *InvoiceServer) ListSellerInvoiceDocuments(
	ctx context.Context, req *pb.ListSellerInvoiceDocumentsRequest,
) (*pb.ListSellerInvoiceDocumentsResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	dates, err := parseDates(req.GetDateFrom(), req.GetDateTo())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListSellerDocuments(ctx, usecase.SellerDocumentsInput{
		StoreID:      storeID,
		Dates:        dates,
		Mode:         req.GetMode(),
		Status:       req.GetStatus(),
		OrderNo:      req.GetOrderNo(),
		AbnormalOnly: req.GetAbnormalOnly(),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListSellerInvoiceDocumentsResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_seller_documents.failed", err)
	}
	shipments, degraded := s.resolveShipmentStatuses(ctx, orderNosOf(out.Records))
	return &pb.ListSellerInvoiceDocumentsResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListSellerInvoiceDocumentsData{
			Records: s.toSellerDocumentItems(out.Records, shipments), Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
			ShipmentStatusDegraded: degraded,
		},
	}, nil
}

// orderNosOf 折出這一頁所有訂單編號，供出貨狀態批次查詢（#3019）。
//
// 只取 source_type=ORDER 的列：平台代製收據與 ADM 手動開票沒有訂單，
// 把它們的空字串送進批次查詢只是白費，`dedupeStrings` 會濾掉。
func orderNosOf(rows []usecase.DocumentListItem) []string {
	out := make([]string, 0, len(rows))
	for _, item := range rows {
		if item.Document.SourceType == sourceTypeOrder {
			out = append(out, str(item.Document.SourceRef))
		}
	}
	return out
}

// toSellerDocumentItems 折出 S06／A12 共用的列。
//
// shipments ＝ order_no ⇒ 物流狀態碼（ShippingStatus CodeMeta 的 value）。
// 不在 map 內的訂單回空字串 ＝「這張訂單還沒有物流單」。
//
// 🔴 「整批查不到」**不在這裡**表達：它是 data 層的 `shipment_status_degraded`。
// 在這一層用哨兵值區分，等於往 ShippingStatus 的值域裡塞一個 CodeMeta 沒有的碼
// （§8 禁止硬編碼），而且逐列重複同一個整批性質的事實。
//
// buyer_kind 由 recipient_tax_id 推導，見下方。
func (s *InvoiceServer) toSellerDocumentItems(
	rows []usecase.DocumentListItem, shipments map[string]string,
) []*pb.SellerInvoiceDocumentItem {
	out := make([]*pb.SellerInvoiceDocumentItem, 0, len(rows))
	for _, item := range rows {
		doc := item.Document
		orderNo := ""
		if doc.SourceType == sourceTypeOrder {
			orderNo = str(doc.SourceRef)
		}
		out = append(out, &pb.SellerInvoiceDocumentItem{
			Summary:     s.toSummary(doc, ""),
			OrderNo:     orderNo,
			BuyerKind:   buyerKindOf(doc),
			InvoiceMode: doc.FulfillmentMode,
			UpdatedAt:   fmtTimeValue(doc.UpdatedAt),
			// 空字串 ＝ 這份文件還沒有任何 operation。**不得用 omitempty 省略此欄**：
			// 前端靠「有 key 但值是空字串」與「後端還沒部署這一欄」區分（58A T06），
			// 而 S08 是可定址路由 ⇒ 少了它的症狀是「看得到異常，點不進去」。
			LatestJobId: fmtID(item.LatestOperationID),
			// #3019：paid_at 從上線至今恆為空字串——proto 有這一欄、gateway 照實透傳、
			// 只有生產者從來沒填。賣家對帳時看不到「這張發票對應的款項什麼時候入的」，
			// 只能自己回訂單管理逐筆對，而 S06 正是他處理發票的主畫面。
			//
			// ⚠️ 值取自 invoice_documents.paid_at（開票當下落下的收款時點），
			// **不是** issue_not_before：兩者在自動開票路徑上同值，但在 ADM 手動開票
			// 與作廢重開的新票上不同（見 domain.Document.PaidAt）。
			//
			// 2026-09-02 之前建的列這一欄是 NULL ⇒ 回空字串。那是誠實的「不知道」，
			// 回填是一次性維運 SQL（§22），不進 application code。
			PaidAt: fmtTime(doc.PaidAt),
			// #3019：同樣是「proto 有、gateway 透傳、生產者從來沒填」。
			// 空字串 ＝ 這張訂單還沒有物流單（合法且常見：付款後尚未出貨）。
			// **整批查不到**走 data 層的 shipment_status_degraded，不在這裡塞哨兵值。
			ShipmentStatus: shipments[orderNo],
		})
	}
	return out
}

// buyerKindOf 折出 proto 的 buyer_kind（B2C／B2B）。
//
// 🔴 判準是**買受人統編**不是 document_kind：後者有第三個值 RECEIPT（平台代製收據），
// 直傳會讓 buyer_kind 出現一個它值域裡沒有的字串。B2C 的統編是固定的 10 個 0
// （chk_invdoc_b2c），domain.IsBusinessTaxID 已經在處理這個佔位值。
func buyerKindOf(doc domain.Document) string {
	if domain.IsBusinessTaxID(doc.RecipientTaxID) {
		return "B2B"
	}
	return "B2C"
}

// GetSellerInvoiceDocument 是 S07 賣家發票明細（端點 31）。
//
// 🔴 **本支是遮罩函式的第一條生產路徑**：賣家不是買家個資的資料主體 ⇒ 載具識別碼
// 依 §8.1.3(b) 必須遮罩，而遮罩發生在 usecase（它才知道 viewer 是誰）。
// 本層拿到的 CarrierSummary 已經是遮罩後的值 —— **不要在這裡再解一次密**。
func (s *InvoiceServer) GetSellerInvoiceDocument(
	ctx context.Context, req *pb.GetSellerInvoiceDocumentRequest,
) (*pb.GetSellerInvoiceDocumentResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseID(req.GetDocumentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "document_id: %v", err)
	}

	view, err := s.query.GetSellerDocument(ctx, storeID, documentID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetSellerInvoiceDocumentResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_document.failed", err)
	}

	doc := view.Detail.Document
	return &pb.GetSellerInvoiceDocumentResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.SellerInvoiceDocumentDetail{
			Detail: s.toDetail(view.Detail),
			// 🔴 fail-closed：三值 SELLER／PLATFORM／UNDETERMINED 由 coupon 的 created_by
			// 推導（§3.7 D9a E4），而本服務沒有 coupon client ⇒「不知道」的正確答案是
			// UNDETERMINED，不是猜一個 SELLER。寫成常數 "SELLER" 等於在契約層宣告
			// ADMIN 券不可能出現，真的出現時會靜默以折扣後金額開票 —— 那是稅務錯誤。
			FundedBy: fundedByUndetermined,
			// created_by 同源，同樣沒有 coupon client ⇒ 空字串（未知），不猜。
			CutoverVersion:    fmtIDPtr(doc.ProviderAccountVersion),
			ProviderRequestId: str(doc.ProviderRequestNo),
			ManualReasonCode:  str(doc.ManualReasonCode),
			LatestJobId:       fmtID(view.Detail.LatestOperationID),
			// 空字串 ＝ 現在就可以重寄（proto3 沒有 null，欄位帶 omitempty）。
			// 🔴 它是前端唯一的倒數來源：端點 32 的 429 回應帶不出 data（§6.1.10），
			// 所以按鈕的停用與倒數只能在進畫面時算好。
			NextResendAvailableAt: fmtTime(view.NextResendAvailableAt),
		},
	}, nil
}

// ListSellerReceivedInvoiceDocuments 是 S12 賣家進項（端點 72）。
//
// 🔴 它與端點 30 是**兩個資料集**：銷項可作廢可折讓、進項是唯讀報稅憑證。
// 進項的列裡刻意沒有任何動作相關欄位（沒有 latest_job_id／can_*）——
// 賣家對別人開給他的發票沒有作廢的法律地位。
func (s *InvoiceServer) ListSellerReceivedInvoiceDocuments(
	ctx context.Context, req *pb.ListSellerReceivedInvoiceDocumentsRequest,
) (*pb.ListSellerReceivedInvoiceDocumentsResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	dates, err := parseDates(req.GetDateFrom(), req.GetDateTo())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListSellerReceivedDocuments(ctx, usecase.ReceivedDocumentsInput{
		StoreID: storeID, Dates: dates, Status: req.GetStatus(),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListSellerReceivedInvoiceDocumentsResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_received_documents.failed", err)
	}

	records := make([]*pb.SellerReceivedInvoiceItem, 0, len(out.Records))
	for _, item := range out.Records {
		records = append(records, &pb.SellerReceivedInvoiceItem{
			Summary:    s.toSummary(item.Document, ""),
			IssuerName: item.Document.IssuerNameSnapshot,
			// 用途說明由明細行推導（proto 定案：不加 DB 欄）。
			RecipientNote: item.RecipientNote,
		})
	}
	return &pb.ListSellerReceivedInvoiceDocumentsResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListSellerReceivedInvoiceDocumentsData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// 賣家：S08 異常詳情（端點 33）與 62R
// ---------------------------------------------------------------------------

// GetSellerInvoiceJob 是 S08 賣家異常詳情（端點 33）。
func (s *InvoiceServer) GetSellerInvoiceJob(
	ctx context.Context, req *pb.GetSellerInvoiceJobRequest,
) (*pb.GetSellerInvoiceJobResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseID(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "job_id: %v", err)
	}

	view, err := s.query.GetSellerJob(ctx, storeID, jobID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetSellerInvoiceJobResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_job.failed", err)
	}
	return &pb.GetSellerInvoiceJobResponse{Code: 0, Message: msgSuccess, Data: s.toJobInfoView(view)}, nil
}

// ListOrderExternalIssuances 是子路由 62R（端點 62 的前置讀取）。
//
// 🔴 述詞寫死在 usecase、**不接受任何 status query**：放寬就退化成 S08 的等價物，
// 而 S08 對停權賣家是 BLOCK（§3 D16）—— 述詞是這條路由的安全邊界，不是效能優化。
// 查無／越權／訂單不存在一律 code 0 ＋ 空 records，不回 9340。
func (s *InvoiceServer) ListOrderExternalIssuances(
	ctx context.Context, req *pb.ListOrderExternalIssuancesRequest,
) (*pb.ListOrderExternalIssuancesResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	if req.GetOrderNo() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order_no: %v", errEmptyID)
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListOrderExternalIssuances(ctx, storeID, req.GetOrderNo(), page, pageSize)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListOrderExternalIssuancesResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_external_issuances.failed", err)
	}
	return &pb.ListOrderExternalIssuancesResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListOrderExternalIssuancesData{
			Records: s.toJobInfoViews(out.Records), Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// 賣家：S09／S10 申訴（端點 34／35）
// ---------------------------------------------------------------------------

// sellerComplaintSortCreatedDesc 是端點 34 的第二種排序（預設是 due_at_asc）。
const sellerComplaintSortCreatedDesc = "created_at_desc"

// ListSellerInvoiceComplaints 是 S09 賣家申訴列表（端點 34）。
//
// status query 是**視圖名不是狀態值**（§6.1.11）：三個視圖互斥且窮盡。
// ⚠️ 這與 ADM 側的端點 58 規則相反（那裡的 OVERDUE 刻意是交集視圖），兩處不可互相套用。
func (s *InvoiceServer) ListSellerInvoiceComplaints(
	ctx context.Context, req *pb.ListSellerInvoiceComplaintsRequest,
) (*pb.ListInvoiceComplaintsResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListSellerComplaints(ctx, usecase.SellerComplaintsInput{
		StoreID: storeID,
		View:    req.GetStatus(),
		// due_at_asc 是**預設**（最接近逾期者優先）⇒ 只有明確要求 created_at_desc
		// 才切換。用布林而不是把欄名字串傳下去：欄名進 SQL 只能靠字串拼接，那是注入面。
		SortByDue: req.GetSort() != sellerComplaintSortCreatedDesc,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		if invalid := invalidViewError(err, "status"); invalid != nil {
			return nil, invalid
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListInvoiceComplaintsResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_seller_complaints.failed", err)
	}
	return s.complaintListResponse(ctx, out, toComplaintInfo, true), nil
}

// GetSellerInvoiceComplaint 是 S10 賣家申訴詳情（端點 35）。
func (s *InvoiceServer) GetSellerInvoiceComplaint(
	ctx context.Context, req *pb.GetSellerInvoiceComplaintRequest,
) (*pb.InvoiceComplaintResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	complaintID, err := parseID(req.GetComplaintId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "complaint_id: %v", err)
	}

	view, err := s.query.GetSellerComplaint(ctx, storeID, complaintID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.InvoiceComplaintResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_complaint.failed", err)
	}
	info := toComplaintInfo(view)
	s.fillOneBuyerName(ctx, info, view)
	return &pb.InvoiceComplaintResponse{Code: 0, Message: msgSuccess, Data: info}, nil
}

// ---------------------------------------------------------------------------
// ADM：A01 營運總覽（端點 37）
// ---------------------------------------------------------------------------

// GetInvoiceOverviewForAdmin 是 A01 營運總覽（端點 37）。
//
// 🔴 8 個 KPI 是**封閉列舉**，每一個都帶著自己的口徑（definition）出去：沒有口徑的
// 儀表板數字會讓兩個人看同一個畫面得到不同結論，而 A01 是排班決定「今天先處理什麼」
// 的依據。
//
// 🔴 near_deadline 是 pending_tax_reviews 的子集、needs_manual 與 pending_jobs
// 不相交但都不是全集 ⇒ **A01 不得顯示任何合計**。
func (s *InvoiceServer) GetInvoiceOverviewForAdmin(
	ctx context.Context, req *pb.GetInvoiceOverviewForAdminRequest,
) (*pb.GetInvoiceOverviewForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}

	view, err := s.query.GetInvoiceOverviewForAdmin(ctx)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetInvoiceOverviewForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_admin_overview.failed", err)
	}

	kpis := make([]*pb.InvoiceKpiItem, 0, len(view.KPIs))
	for _, k := range view.KPIs {
		kpis = append(kpis, &pb.InvoiceKpiItem{
			Key: k.Key,
			// 🔴 取不到的 KPI 回**空字串**不是 "0"（§6.1.11a suspended_sellers 逐字：
			// 「取不到就明說取不到，不得回 0 充數」）。"0" 看起來就像「今天沒有人被停權」，
			// 而 definition 欄同時寫著它的權威在 seller-service —— 兩者必須一致。
			Value:       kpiValueOf(k),
			WindowLabel: k.WindowLabel,
			Definition:  k.Definition,
		})
	}
	return &pb.GetInvoiceOverviewForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.GetInvoiceOverviewForAdminData{
			Kpis:         kpis,
			PriorityJobs: s.toJobInfoViews(view.PriorityJobs),
		},
	}, nil
}

func kpiValueOf(k usecase.KpiItem) string {
	if !k.Available {
		return ""
	}
	return fmtInt64(k.Value)
}

// ---------------------------------------------------------------------------
// ADM：A02 稅務審核佇列（端點 38）
// ---------------------------------------------------------------------------

// ListTaxReviewsForAdmin 是 A02 稅務身分審核佇列（端點 38）。
//
// filter 是**視圖名不是狀態值**，且六個 filter **不互斥**（NEAR_DEADLINE 與 EXTENDED
// 是交集視圖）⇒ 六個 filter 的筆數加總 ≠ 總筆數，A02 的 UI 不得顯示「合計」。
func (s *InvoiceServer) ListTaxReviewsForAdmin(
	ctx context.Context, req *pb.ListTaxReviewsForAdminRequest,
) (*pb.ListTaxReviewsForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListTaxReviewsForAdmin(ctx, usecase.AdminTaxReviewsInput{
		Filter: req.GetFilter(), Keyword: req.GetKeyword(), Page: page, PageSize: pageSize,
	})
	if err != nil {
		if invalid := invalidViewError(err, "filter"); invalid != nil {
			return nil, invalid
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListTaxReviewsForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_tax_reviews_admin.failed", err)
	}

	storeIDs := make([]int64, 0, len(out.Records))
	for _, item := range out.Records {
		storeIDs = append(storeIDs, item.Item.StoreID)
	}
	names := s.resolveStoreNames(ctx, storeIDs)

	records := make([]*pb.TaxReviewItem, 0, len(out.Records))
	for _, item := range out.Records {
		row := s.toTaxReviewItem(item)
		row.StoreName = names[item.Item.StoreID]
		records = append(records, row)
	}
	return &pb.ListTaxReviewsForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListTaxReviewsForAdminData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// toTaxReviewItem 折出 A02 的一列。
//
// ⚠️ **store_name 不在這裡填**：它要跨服務批次取（#3017），逐列取就是一列一次
// gRPC 往返。呼叫端拿整頁的 store_id 打一次 BatchGetSellerMeta，再回填進來。
// 取不到時是空字串——那是降級，不是這一欄不存在。
//
// ⚠️ risk_flags 是**封閉列舉**（TAX_ID_MISMATCH_KYC／REPEAT_APPLICATION／OPEN_COMPLAINTS），
// 不留「其他」—— 沒有定義的風險標記等於沒有標記。本波只填得出 OPEN_COMPLAINTS：
// 另兩個的資料源（KYC 認證單統編、90 天內送審次數）不在本服務。
func (s *InvoiceServer) toTaxReviewItem(v usecase.TaxReviewItemView) *pb.TaxReviewItem {
	p := v.Item.Profile
	out := &pb.TaxReviewItem{
		StoreId:       fmtID(v.Item.StoreID),
		EntityType:    str(p.EntityTaxType),
		ClaimedStatus: p.ClaimedObligation,
		ReviewStatus:  p.ReviewStatus,
		SubmittedAt:   fmtTime(p.SubmittedAt),
		DeadlineAt:    fmtTime(p.PendingValidUntil),
		// 無期限（非待審狀態）回空字串而不是 "0"：後者會被前端讀成「今天到期」。
		RemainingDays:  fmtOptInt64(v.RemainingDays),
		ExtensionCount: fmtInt64(int64(p.ExtensionCount)),
		// 自行開票責任條款**是否已簽**——本欄問的是簽署事實，不是責任是否生效中。
		// 裁決 C 之後兩者不再等價：SUBMITTED 的列已簽（端點 19 同一句 UPDATE 寫的）
		// 但責任尚未生效（`SelfIssueEffective()` 只認 SELF_ISSUE_PENDING）。A02 佇列
		// 的「已接受／未接受」badge 要的正是簽署事實 ⇒ 這裡維持讀 attested_at。
		SelfIssueAccepted: p.AttestedAt != nil,
		// 收據代製授權是否已簽。裁決 DEC-20260905-015836 之後賣家在審核期間就簽得了，
		// 而 T08（核定免用統一發票）**要求已授權** ⇒ 佇列上看得見這一欄，ADM 才不必
		// 逐一點進 A03 才知道哪些列現在核定得了。語意與 A03 的 receipt_authorized
		// 逐字相同（同一個 receipt_authorized_at 是否非空）。
		ReceiptAuthorized: p.ReceiptAuthorizedAt != nil,
	}
	if v.Item.OpenComplaintCount > 0 {
		out.RiskFlags = append(out.RiskFlags, riskFlagOpenComplaints)
	}
	return out
}

// riskFlagOpenComplaints 是 §6.1.12 定版三值中本服務答得出來的那一個。
const riskFlagOpenComplaints = "OPEN_COMPLAINTS"

// fmtOptInt64 折出可缺席的整數；nil ⇒ 空字串（proto3 沒有 null）。
func fmtOptInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// ---------------------------------------------------------------------------
// ADM：A04 AMEGO 串接監控（端點 42）
// ---------------------------------------------------------------------------

// ListProviderAccountsForAdmin 是 A04 AMEGO 串接監控（端點 42）。
//
// 🔴 status 是**含三態投影**的值：查無 provider account 列 ⇒ NOT_STARTED（上線當天
// 的全體），有列但 status 為未知值 ⇒ CONFIG_ERROR。照需求書字面把空值一律當成
// 配置錯誤，上線當天每一個既有賣家都會被標成配置錯誤（§6.1.13）。
func (s *InvoiceServer) ListProviderAccountsForAdmin(
	ctx context.Context, req *pb.ListProviderAccountsForAdminRequest,
) (*pb.ListProviderAccountsForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListProviderAccountsForAdmin(ctx, usecase.AdminProviderAccountsInput{
		Status: req.GetStatus(), Keyword: req.GetKeyword(), Page: page, PageSize: pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListProviderAccountsForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_provider_accounts_admin.failed", err)
	}

	storeIDs := make([]int64, 0, len(out.Records))
	for _, item := range out.Records {
		storeIDs = append(storeIDs, item.StoreID)
	}
	names := s.resolveStoreNames(ctx, storeIDs)

	records := make([]*pb.ProviderAccountItem, 0, len(out.Records))
	for _, item := range out.Records {
		row := &pb.ProviderAccountItem{
			StoreId: fmtID(item.StoreID),
			Status:  item.Status,
			// store_name 跨服務批次取（#3017）；取不到退回空字串，A04 顯示 store_id。
			StoreName: names[item.StoreID],
		}
		if a := item.Account; a != nil {
			// 🔴 這裡永遠不會出現 app_key 或 fingerprint（§8.1.1(f) U2）：
			// C 類憑證單獨持有即可對外開票，任何端、任何情況都不回顯。
			// 欄名已改為 seller_tax_id（#3017）：值一直是完整 8 碼，
			// §8.1.3(b) 第二列點名這個欄位並判為 O 類、三端皆完整。
			row.SellerTaxId = a.ProviderTaxID
			row.CredentialLast4 = a.CredentialLast4
			row.LastVerifiedAt = fmtTime(a.VerifiedAt)
			row.AutomationCutoverAt = fmtTime(a.AutomationCutoverAt)
		}
		records = append(records, row)
	}
	return &pb.ListProviderAccountsForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListProviderAccountsForAdminData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// ADM：A12 文件（端點 47）／A05 工作佇列（端點 50）
// ---------------------------------------------------------------------------

// ListInvoiceDocumentsForAdmin 是 A12 全平台發票文件（端點 47）。
//
// store_id 在這裡是**篩選不是授權**（不給 ＝ 全平台）——這正是它不能與端點 30
// 共用一支查詢的理由：後者的 store_id 是必填的授權述詞。
func (s *InvoiceServer) ListInvoiceDocumentsForAdmin(
	ctx context.Context, req *pb.ListInvoiceDocumentsForAdminRequest,
) (*pb.ListInvoiceDocumentsForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	dates, err := parseDates(req.GetDateFrom(), req.GetDateTo())
	if err != nil {
		return nil, err
	}
	storeID, err := parseFilterID(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListDocumentsForAdmin(ctx, usecase.AdminDocumentsInput{
		StoreID: storeID, OrderNo: req.GetOrderNo(), Status: req.GetStatus(),
		Mode: req.GetMode(), Dates: dates, Page: page, PageSize: pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListInvoiceDocumentsForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_documents_admin.failed", err)
	}
	shipments, degraded := s.resolveShipmentStatuses(ctx, orderNosOf(out.Records))
	return &pb.ListInvoiceDocumentsForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListInvoiceDocumentsForAdminData{
			Records: s.toSellerDocumentItems(out.Records, shipments), Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
			ShipmentStatusDegraded: degraded,
		},
	}, nil
}

// ListInvoiceJobsForAdmin 是 A05 工作與異常佇列（端點 50）。
//
// 🔴 tab 是六個分頁名，它們是 20 個 job 狀態的**分割**（聯集 ＝ 20、兩兩交集 ＝ 空）。
// 🔴 upload_status 與 tab **正交、不是第七個分頁**：它是「發票開出來了，但財政部端
// 沒收到」那一類的唯一入口 —— 那些 job 落 COMPLETED，與真正完成的混在同一頁。
func (s *InvoiceServer) ListInvoiceJobsForAdmin(
	ctx context.Context, req *pb.ListInvoiceJobsForAdminRequest,
) (*pb.ListInvoiceJobsForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	storeID, err := parseFilterID(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListJobsForAdmin(ctx, usecase.AdminJobsInput{
		Tab: req.GetTab(), StoreID: storeID, OrderNo: req.GetOrderNo(),
		OperationType: req.GetOperationType(), UploadStatus: req.GetUploadStatus(),
		ManualReason: req.GetManualReason(), Page: page, PageSize: pageSize,
	})
	if err != nil {
		if invalid := invalidViewError(err, "tab/upload_status"); invalid != nil {
			return nil, invalid
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListInvoiceJobsForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_jobs_admin.failed", err)
	}
	return &pb.ListInvoiceJobsForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListInvoiceJobsForAdminData{
			Records: s.toJobInfoViews(out.Records), Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// ADM：A07 申訴（端點 58）／A10 稽核（端點 61）
// ---------------------------------------------------------------------------

// ListInvoiceComplaintsForAdmin 是 A07 申訴處理佇列（端點 58）。
//
// 🔴 五個 tab **不互斥**（OVERDUE 是橫切視圖）⇒ 加總 ≠ 總筆數，A07 的 UI 不得顯示
// 「合計」、也不得用「總數 − 其他四頁」倒算任何一頁。這與端點 50 的六分頁**規則相反**。
func (s *InvoiceServer) ListInvoiceComplaintsForAdmin(
	ctx context.Context, req *pb.ListInvoiceComplaintsForAdminRequest,
) (*pb.ListInvoiceComplaintsResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	storeID, err := parseFilterID(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListComplaintsForAdmin(ctx, usecase.AdminComplaintsInput{
		Tab: req.GetTab(), StoreID: storeID, OrderNo: req.GetOrderNo(),
		SortByDue: req.GetSort() != sellerComplaintSortCreatedDesc,
		Page:      page, PageSize: pageSize,
	})
	if err != nil {
		if invalid := invalidViewError(err, "tab"); invalid != nil {
			return nil, invalid
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListInvoiceComplaintsResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_complaints_admin.failed", err)
	}

	records := make([]*pb.InvoiceComplaintInfo, 0, len(out.Records))
	views := make([]usecase.ComplaintView, 0, len(out.Records))
	for _, item := range out.Records {
		info := toComplaintInfo(item.Complaint)
		// store_id 在 ADM 面填得出來（列表 join 了 invoice_issuers）；
		// 買賣家面填不出來（那兩支的 view 不帶 issuer），見 toComplaintInfo 的註解。
		info.StoreId = fmtID(item.StoreID)
		records = append(records, info)
		views = append(views, item.Complaint)
	}
	// #3017：A06／A07 的每一張申訴卡都要認得出人，一次 RPC 補整頁。
	s.fillBuyerNames(ctx, records, views)
	return &pb.ListInvoiceComplaintsResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListInvoiceComplaintsData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// ListInvoiceAuditLogsForAdmin 是 A10 稽核查詢（端點 61）。
//
// ⚠️ before_json／after_json 的遮罩在**寫入時**完成（§6.1.14）：查詢時遮罩表示
// DB 裡存著明文，而本表保存 5 年 ＝ 一個五年期的明文 PII 倉庫。本支原樣回傳。
func (s *InvoiceServer) ListInvoiceAuditLogsForAdmin(
	ctx context.Context, req *pb.ListInvoiceAuditLogsForAdminRequest,
) (*pb.ListInvoiceAuditLogsForAdminResponse, error) {
	if _, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id"); err != nil {
		return nil, err
	}
	page, pageSize, err := parseListPaging(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	dates, err := parseDates(req.GetDateFrom(), req.GetDateTo())
	if err != nil {
		return nil, err
	}
	targetActor, err := parseFilterID(req.GetTargetActorAccountId(), "target_actor_account_id")
	if err != nil {
		return nil, err
	}
	storeID, err := parseFilterID(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	documentID, err := parseFilterID(req.GetDocumentId(), "document_id")
	if err != nil {
		return nil, err
	}
	jobID, err := parseFilterID(req.GetJobId(), "job_id")
	if err != nil {
		return nil, err
	}

	out, err := s.query.ListAuditLogsForAdmin(ctx, usecase.AdminAuditLogsInput{
		TargetActorAccountID: targetActor, StoreID: storeID, OrderNo: req.GetOrderNo(),
		DocumentID: documentID, JobID: jobID, OperationType: req.GetOperationType(),
		Dates: dates, Page: page, PageSize: pageSize,
	})
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.ListInvoiceAuditLogsForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.list_audit_logs_admin.failed", err)
	}

	records := make([]*pb.InvoiceAuditLogItem, 0, len(out.Records))
	for _, item := range out.Records {
		row := toAuditLogItem(item.Entry)
		// operation_type 在本表叫 action（多型稽核），而 A10 的篩選是以 operation
		// 的 operation_type 為準 ⇒ 解析得到時以解析值為準，解析不到就留 action。
		if item.ResolvedOperationType != "" {
			row.OperationType = item.ResolvedOperationType
		}
		// 🔴 terms_version／result **仍然恆為空字串**（#3031 覆核後維持）：
		// invoice_admin_audit_logs 沒有對應欄（逐欄查過），而
		//   · result 填 "SUCCESS" 會讓失敗的操作看起來都成功——稽核表上最不能說的謊
		//   · terms_version 只對「條款相關」那幾種 action 有意義，對其餘 action 是空的，
		//     猜一個當期版本會讓每一列都像簽過同一版
		// 兩者要真的填得出來得先在該表加欄並由寫入端填，不是查詢端能補的。
		records = append(records, row)
	}
	return &pb.ListInvoiceAuditLogsForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ListInvoiceAuditLogsForAdminData{
			Records: records, Total: fmtInt32(out.Total),
			Page: fmtInt32(out.Page), PageSize: fmtInt32(out.PageSize),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// 共用 mapper
// ---------------------------------------------------------------------------

// toSummary 折出不需要載具的摘要（列表路徑）。
//
// ⚠️ **列表刻意不解密載具**：WithDecryptedRecipient 內部會再查一次那一列 ⇒ 逐列解密
// ＝ 每列一次額外 DB 往返，而四個列表端點的欄位表裡都沒有載具（58A §B09／§S06／§S12）。
// 要看載具去詳情頁。
func (s *InvoiceServer) toSummary(doc domain.Document, _ string) *pb.InvoiceDocumentSummary {
	summary, carrierType := carrierSummaryFallback(doc)
	return s.toSummaryWithCarrier(doc, summary, carrierType)
}

// carrierSummaryFallback 是沒有解密時的摘要值 ＋ 它的判別欄（#3084）。
//
// B2C 的買受人統編是固定的 10 個 0（chk_invdoc_b2c）⇒ 直接吐出去會讓每一張二聯式
// 發票的載具摘要都顯示成一串零。
//
// 🔴 carrierType **恆為空字串**，而且那是正確的：本函式服務的是列表路徑，
// 該路徑刻意不解密載具（見 toSummary）⇒ summary 裝的必然是買受人統編或什麼都沒有，
// 從來不是載具。兩個回傳值一起表達「這一列不呈現載具」，不會出現
// 「說是載具卻沒有值」那種自相矛盾。
//
// ⚠️ 本函式與 usecase.carrierPresentationOf 是同一個判斷的**兩份實作**
// （跨 package，一個要解密一個不解密）。原本兩者只算 summary 而語意各自漂移：
// 這裡的舊註解寫「proto 對本欄的約定是『無載具為空字串』」，而函式體在 B2B 時回統編
// —— 註解與行為互相矛盾，那正是 #3084 得以存在的縫。改任一邊都要回頭看另一邊。
func carrierSummaryFallback(doc domain.Document) (summary, carrierType string) {
	if doc.RecipientTaxID == domain.B2CBuyerIdentifier {
		return "", ""
	}
	return doc.RecipientTaxID, ""
}

// toSummaryWithCarrier 折出摘要，載具摘要由呼叫端決定（買家明碼／賣家遮罩／ADM 明碼）。
//
// 🔴 invoice_number 三端皆完整（O 類，§8.1.3(c) 裁定 1 逐字推翻 img-05 的遮罩畫法）：
// 它是對獎／報帳／申訴的唯一憑據，而且同時印在發票上、寄在通知信裡、存在載具裡 ——
// 遮它保護不到任何人。
func (s *InvoiceServer) toSummaryWithCarrier(doc domain.Document, carrierSummary, carrierType string) *pb.InvoiceDocumentSummary {
	available, reason := s.query.FileAvailability(doc)
	return &pb.InvoiceDocumentSummary{
		DocumentId:            fmtID(doc.ID),
		DocumentKind:          doc.DocumentKind,
		Status:                doc.Status,
		InvoiceNumber:         firstNonEmptyStr(doc.ProviderInvoiceNo, doc.ExternalInvoiceNo, doc.ReceiptNo),
		IssuedAt:              fmtTime(doc.ProviderIssuedAt),
		TotalAmount:           fmtInt64(doc.TotalAmount),
		CarrierSummary:        carrierSummary,
		CarrierType:           carrierType,
		RelatedDocumentId:     firstNonEmptyID(doc.OriginalDocumentID, doc.SupersededByID),
		FileAvailable:         available,
		FileUnavailableReason: reason,
	}
}

// toDetail 折出買家／賣家共用的文件明細。
//
// timeline 走 usecase 已投影好的 5 值（CodeMeta InvoiceDocumentTimelineEvent）：
// 直接把 to_status 塞進 event 會讓前端拿到 PENDING／ISSUING 這種時間軸上不存在的值。
func (s *InvoiceServer) toDetail(v usecase.DocumentDetailView) *pb.InvoiceDocumentDetail {
	doc := v.Document
	out := &pb.InvoiceDocumentDetail{
		Summary:        s.toSummaryWithCarrier(doc, v.CarrierSummary, v.CarrierType),
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
	for _, t := range v.Timeline {
		out.Timeline = append(out.Timeline, &pb.InvoiceTimelineItem{
			OccurredAt: fmtTimeValue(t.OccurredAt),
			Event:      t.Event,
			Note:       t.Note,
		})
	}
	return out
}

// toJobInfoViews 折出一批 job。
func (s *InvoiceServer) toJobInfoViews(rows []domain.OperationView) []*pb.InvoiceJobInfo {
	out := make([]*pb.InvoiceJobInfo, 0, len(rows))
	for _, v := range rows {
		out = append(out, s.toJobInfoView(v))
	}
	return out
}

// toJobInfoView 折出一件開票作業（三個列表端點 ＋ 端點 33 共用）。
//
// 🔴 order_no 與 store_id 一律由 **document／issuer 帶上來**（§8.1.4 末段）：
// invoice_operations 完全沒有這兩個欄位，自行反正規化一份就是在製造第二個真實來源。
//
// ⚠️ 與既有的 toJobInfo(op, doc, issuer) 是同一份契約的兩個入口：那一支服務 ADM 的
// 單筆詳情（手上已有整份 document），本支服務五支讀取視圖（SQL 只帶了必要的四欄）。
// 兩者的欄位對應必須一致，改一邊要同批改另一邊。
func (s *InvoiceServer) toJobInfoView(v domain.OperationView) *pb.InvoiceJobInfo {
	op := v.Operation
	policy := s.query.RetryPolicy()
	// summary 用 CodeMeta 的 Label，**不含 provider code**（proto 逐字）：
	// AMEGO 的 code 只出現在 ADM 的 provider_exchanges 那一層。
	summary := op.Status
	if meta, ok := codemeta.InvoiceOperationStatusFromValue(op.Status); ok {
		summary = meta.Label
	}
	return &pb.InvoiceJobInfo{
		JobId:         fmtID(op.ID),
		OrderNo:       v.DocumentSourceRef,
		StoreId:       fmtID(v.IssuerRefID),
		OperationType: op.OperationType,
		Status:        op.Status,
		Summary:       summary,
		AttemptCount:  fmtInt64(int64(op.AttemptCount)),
		// 逐列的 max_attempts 取自那一列（建列當下寫入），不是取現在的政策值：
		// 舊列可能是舊值，用政策值會讓「還剩幾次」在畫面上算錯。
		MaxAttempts:   fmtInt64(int64(op.MaxAttempts)),
		LastAttemptAt: fmtTime(op.StartedAt),
		NextRetryAt:   fmtTimeValue(op.NextAttemptAt),
		RetryPolicy: &pb.RetryPolicy{
			InitialIntervalSeconds: fmtInt64(policy.InitialIntervalSeconds),
			BackoffMultiplier:      fmtInt64(policy.BackoffMultiplier),
			MaxIntervalSeconds:     fmtInt64(policy.MaxIntervalSeconds),
			MaxAttempts:            fmtInt32(policy.MaxAttempts),
		},
		ShipmentUnaffectedNotice: noticeForKind(v.DocumentKind),
		ProviderRequestId:        op.ProviderRequestNo,
		Version:                  fmtInt64(op.Version),
		// 兩個旗標的 SSOT 是 SQL 的 allowed-from，不是「看起來像失敗的狀態」：
		// 清單放寬了，按鈕就會亮在一個 CAS 必定 rows=0 的狀態上，而 rows=0 是靜默的。
		CanRetry:            usecase.CanRetryOperation(op.Status),
		CanCancel:           usecase.CanCancelOperation(op.Status),
		ManualReasonCode:    v.DocumentManualReasonCode,
		ExternalInvoiceNo:   v.ExternalInvoiceNo,
		ExternalInvoiceDate: fmtDate(v.ExternalInvoiceDate),
		ReportedAt:          fmtTime(v.ExternalReportedAt),
		CanReportExternal:   op.Status == operationStatusManualProcessAllowed,
		// #3066 裁決 B：賣家中心用它分辨發票／收據兩個並列區塊。
		// 前端不得從 operation_type 推導——那個 enum 沒有 RECEIPT。
		DocumentKind: v.DocumentKind,
		// #3031：這一欄原本恆為 false，理由寫著「E4 的判定要 coupon 的 created_by，
		// 而本服務沒有那條 client」——**那個理由不成立**。`needsFundingDecision` 的判準
		// 只有兩欄（document.status ＝ AWAITING_MANUAL 且 manual_reason_code ＝
		// PLATFORM_FUNDED_DISCOUNT），兩欄都在 OperationView 上，不需要任何跨服務查詢。
		//
		// 🔴 恆為 false 的後果不是「少一個欄位」：proto 逐字「true ⇒ 重試前必須帶
		// funding_decision」，而端點 52 的閘門用的是同一個述詞 ⇒ ADM 永遠不會被提示要做
		// 那個裁決，按下重試就吃一個 ErrJobFundingDecisionMissing。**按鈕點了沒反應。**
		NeedsFundingDecision: usecase.NeedsFundingDecision(v.DocumentStatus, v.DocumentManualReasonCode),
	}
}

// ---------------------------------------------------------------------------
// 型別邊界
// ---------------------------------------------------------------------------

// parseListPaging 解析列表分頁，並在 page_size 超上限時回 400。
//
// 🔴 **不夾住**（58A T25 逐字：「超上限一律 400、後端不夾住」）：夾住的話前端送 500
// 會拿到 100 筆並以為自己拿到全部，而 total 說有 500 筆 ⇒ 頁數算錯，
// 且沒有任何東西會報錯。
func parseListPaging(page, pageSize string) (int32, int32, error) {
	p, ps, err := parsePaging(page, pageSize)
	if err != nil {
		return 0, 0, err
	}
	if ps > usecase.ListMaxPageSize {
		return 0, 0, status.Errorf(codes.InvalidArgument,
			"page_size: must not exceed %d", usecase.ListMaxPageSize)
	}
	return p, ps, nil
}

// parseDates 把兩個 YYYY-MM-DD（UTC+8 日界）折成 UTC 區間。
//
// 🔴 格式必須**嚴格驗證**：AMEGO 的 invoice_list 對帶連字號的日期與空字串都回
// code:0 且回**全部歷史**，文件宣稱的 code:32 從未觸發（§6.1.10 的實測坑）。
// 同一組參數會沿用到 ADM 的對帳路徑 ⇒ 驗證要在最上游一次做對。
func parseDates(from, to string) (domain.DateRange, error) {
	dates, err := usecase.ParseInvoiceDateRange(from, to)
	if err != nil {
		return domain.DateRange{}, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return dates, nil
}

// parseFilterID 解析可缺席的 id **篩選**（不是身分）。
//
// 回 nil ＝ 不篩。這與 requirePrincipal 對 0 的處置相反是刻意的：篩選欄缺席是
// 「看全部」，身分欄缺席是「Gateway 沒注入」＝契約破了。
func parseFilterID(s, field string) (*int64, error) {
	if s == "" {
		return nil, nil
	}
	v, err := parseID(s)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s: %v", field, err)
	}
	return &v, nil
}

// invalidViewError 把「視圖名不在對照表內」翻成 400。
//
// 🔴 不得靜默當成「不篩」：打錯字的前端會拿到全部案件。也不得當成空集合：
// 那看起來就像「這個分頁今天沒有案件」。
func invalidViewError(err error, field string) error {
	if !errors.Is(err, usecase.ErrUnknownView) {
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "%s: %v", field, err)
}

// ---------------------------------------------------------------------------
// 內部：賣家開票能力（薄查詢，供 seller-service／ADM 判斷）
// ---------------------------------------------------------------------------

// GetSellerInvoiceCapabilitySimple 回這位賣家目前有沒有開票能力。
//
// 🔴 它與 S01 總覽（端點 16）**讀同一份投影**，不另算一次：兩份實作漂移的症狀是
// 「seller-service 說這位賣家可以自動開票、發票模組說不行」，而兩邊都不會報錯。
// 這與 checkout 的 ResolveMode 一律經 domain.ResolveFulfillmentMode 是同一條理由。
//
// 內部 RPC ⇒ 必須過 HMAC 簽章閘（§5.2）：本支回的是賣家的稅務審核狀態與接單上限，
// 不是可以讓任何同 namespace 的 pod 拿走的東西。
func (s *InvoiceServer) GetSellerInvoiceCapabilitySimple(
	ctx context.Context, req *pb.GetSellerInvoiceCapabilitySimpleRequest,
) (*pb.GetSellerInvoiceCapabilitySimpleResponse, error) {
	if err := requireInternalCall(ctx); err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}

	// 🔴 走 GetSellerCapability 不是 GetSellerInvoiceOverview：後者還會打三個計數
	// 與最近五份文件，而本支的呼叫端（seller-service／ADM）一格都用不到。
	// 「薄查詢」不是形容詞，是它不得替別人付那兩次 DB 往返。
	view, err := s.query.GetSellerCapability(ctx, storeID)
	if err != nil {
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetSellerInvoiceCapabilitySimpleResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.query.get_seller_capability.failed", err)
	}
	return &pb.GetSellerInvoiceCapabilitySimpleResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.GetSellerInvoiceCapabilitySimpleData{
			StoreId:         fmtID(storeID),
			TaxReviewStatus: view.TaxReviewStatus,
			InvoiceMode:     view.InvoiceMode,
			ProviderStatus:  view.ProviderStatus,
			// 判準寫在 SellerCapabilityView 上，兩支端點共用同一個方法 ——
			// 在這裡另湊一組 if 會漏掉連線被 SUSPENDED／REVOKED 的那一格。
			CanIssueAutomatically: view.CanIssueAutomatically(),
			IntakeLimit:           toIntakeLimit(view.IntakeLimit),
		},
	}, nil
}
