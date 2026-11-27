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
			LatestCorrectionId:  fmtID(view.LatestCorrectionID),
			NotifiedEmailMasked: view.NotifiedEmailMasked,
			SnapshotVersion:     fmtInt32(view.SnapshotVersion),
			ShippingFeeNotice: usecase.ShippingFeeNoticeFor(latestKind),
		},
	}, nil
}


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

func toCorrectionData(r domain.CorrectionRequest) *pb.InvoiceCorrectionData {
	return &pb.InvoiceCorrectionData{
		CorrectionId: fmtID(r.ID),
		HandlingMode: str(r.HandlingMode),
		Status:       r.Status,
		CreatedAt:    fmtTimeValue(r.CreatedAt),
		StatusNotice: usecase.CorrectionStatusNotices[r.Status],
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

func orderNosOf(rows []usecase.DocumentListItem) []string {
	out := make([]string, 0, len(rows))
	for _, item := range rows {
		if item.Document.SourceType == sourceTypeOrder {
			out = append(out, str(item.Document.SourceRef))
		}
	}
	return out
}

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
			LatestJobId: fmtID(item.LatestOperationID),
			PaidAt: fmtTime(doc.PaidAt),
			ShipmentStatus: shipments[orderNo],
		})
	}
	return out
}

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
			FundedBy: fundedByUndetermined,
			CutoverVersion:    fmtIDPtr(doc.ProviderAccountVersion),
			ProviderRequestId: str(doc.ProviderRequestNo),
			ManualReasonCode:  str(doc.ManualReasonCode),
			LatestJobId:       fmtID(view.Detail.LatestOperationID),
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

func (s *InvoiceServer) toTaxReviewItem(v usecase.TaxReviewItemView) *pb.TaxReviewItem {
	p := v.Item.Profile
	out := &pb.TaxReviewItem{
		StoreId:       fmtID(v.Item.StoreID),
		EntityType:    str(p.EntityTaxType),
		ClaimedStatus: p.ClaimedObligation,
		ReviewStatus:  p.ReviewStatus,
		SubmittedAt:   fmtTime(p.SubmittedAt),
		DeadlineAt:    fmtTime(p.PendingValidUntil),
		RemainingDays:  fmtOptInt64(v.RemainingDays),
		ExtensionCount: fmtInt64(int64(p.ExtensionCount)),
		SelfIssueAccepted: p.AttestedAt != nil,
		ReceiptAuthorized: p.ReceiptAuthorizedAt != nil,
	}
	if v.Item.OpenComplaintCount > 0 {
		out.RiskFlags = append(out.RiskFlags, riskFlagOpenComplaints)
	}
	return out
}

const riskFlagOpenComplaints = "OPEN_COMPLAINTS"

func fmtOptInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}


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
			StoreName: names[item.StoreID],
		}
		if a := item.Account; a != nil {
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
		info.StoreId = fmtID(item.StoreID)
		records = append(records, info)
		views = append(views, item.Complaint)
	}
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
		if item.ResolvedOperationType != "" {
			row.OperationType = item.ResolvedOperationType
		}
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


func (s *InvoiceServer) toSummary(doc domain.Document, _ string) *pb.InvoiceDocumentSummary {
	summary, carrierType := carrierSummaryFallback(doc)
	return s.toSummaryWithCarrier(doc, summary, carrierType)
}

func carrierSummaryFallback(doc domain.Document) (summary, carrierType string) {
	if doc.RecipientTaxID == domain.B2CBuyerIdentifier {
		return "", ""
	}
	return doc.RecipientTaxID, ""
}

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

func (s *InvoiceServer) toJobInfoViews(rows []domain.OperationView) []*pb.InvoiceJobInfo {
	out := make([]*pb.InvoiceJobInfo, 0, len(rows))
	for _, v := range rows {
		out = append(out, s.toJobInfoView(v))
	}
	return out
}

func (s *InvoiceServer) toJobInfoView(v domain.OperationView) *pb.InvoiceJobInfo {
	op := v.Operation
	policy := s.query.RetryPolicy()
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
		CanRetry:            usecase.CanRetryOperation(op.Status),
		CanCancel:           usecase.CanCancelOperation(op.Status),
		ManualReasonCode:    v.DocumentManualReasonCode,
		ExternalInvoiceNo:   v.ExternalInvoiceNo,
		ExternalInvoiceDate: fmtDate(v.ExternalInvoiceDate),
		ReportedAt:          fmtTime(v.ExternalReportedAt),
		CanReportExternal:   op.Status == operationStatusManualProcessAllowed,
		DocumentKind: v.DocumentKind,
		NeedsFundingDecision: usecase.NeedsFundingDecision(v.DocumentStatus, v.DocumentManualReasonCode),
	}
}


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

func parseDates(from, to string) (domain.DateRange, error) {
	dates, err := usecase.ParseInvoiceDateRange(from, to)
	if err != nil {
		return domain.DateRange{}, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return dates, nil
}

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

func invalidViewError(err error, field string) error {
	if !errors.Is(err, usecase.ErrUnknownView) {
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "%s: %v", field, err)
}


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
			CanIssueAutomatically: view.CanIssueAutomatically(),
			IntakeLimit:           toIntakeLimit(view.IntakeLimit),
		},
	}, nil
}


// WithCalendarMirror 掛上行事曆鏡像的 usecase；用 setter 是因為 NewInvoiceServer 已有 19 個位置參數。
func (s *InvoiceServer) WithCalendarMirror(m *usecase.BusinessCalendarMirror) *InvoiceServer {
	s.calendar = m
	return s
}

// ReplaceBusinessDayMirror 收單體推送的涵蓋區間與假日，同一交易內整段覆蓋。
//
// 內部 RPC ⇒ 過 requireInternalCall（HMAC 簽章由 interceptor 驗）。日期不合整批拒絕回
// InvalidArgument；交易失敗回 Internal，兩者都保留舊鏡像，單體下次對帳會重送。
func (s *InvoiceServer) ReplaceBusinessDayMirror(
	ctx context.Context, req *pb.ReplaceBusinessDayMirrorRequest,
) (*pb.ReplaceBusinessDayMirrorResponse, error) {
	if err := requireInternalCall(ctx); err != nil {
		return nil, err
	}
	if s.calendar == nil {
		return nil, status.Error(codes.Unavailable, "calendar mirror not configured")
	}
	n, err := s.calendar.Replace(ctx, req.GetCoveredFrom(), req.GetCoveredTo(), req.GetHolidayDates())
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidBusinessDayMirror) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, s.internal(ctx, "invoice.calendar.replace.failed", err)
	}
	return &pb.ReplaceBusinessDayMirrorResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.ReplaceBusinessDayMirrorData{HolidayCount: fmtID(int64(n))},
	}, nil
}
