package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// 本檔是所有**唯讀視圖**（分頁列表 ＋ owner-scoped 明細）的 thin adapter。
//
// 為什麼獨立成一檔而不是散進各個 *_repo.go：這一批的正確性條件與寫入面完全不同
// （擁有者述詞、count↔list 述詞一致、UI 分頁名 → DB 狀態集合的展開），放在一起才
// 看得出「每一支 List 都配著一支 Count，而且兩者吃的是同一個 Query struct」。
// 方法仍掛在原本的 repository struct 上——聚合的存取面沒有分裂。
//
// 三條本層自己的規則：
//
//	① 擁有者參數（buyerID／storeID）**只會被原樣塞進 Params**，本層不做任何比對。
//	   任何一支若出現 `if got.BuyerID != buyerID` 就代表述詞寫錯了（§8.1.4(b)）。
//	② 可選篩選一律「旗標 ＋ 值」成對傳。用零值當「不篩」的哨兵會在零值本身合法時
//	   靜默篩掉正確的資料。
//	③ Count 與 List 共用同一個 Query struct ⇒ 兩者的參數不可能漂移。
//	   （述詞本身的一致由 SQL 側保證，見 db/queries 各支的註解。）

// ---------------------------------------------------------------------------
// 參數轉換
// ---------------------------------------------------------------------------

// optInt64 把「可選的 int64 篩選」拆成 SQL 需要的 (旗標, 值)。
func optInt64(v *int64) (bool, int64) {
	if v == nil {
		return false, 0
	}
	return true, *v
}

// optTime 同上。旗標為 false 時值是 SQL NULL：述詞那一段不會被求值
// （`NOT false OR <cmp>` 恆真），而 NULL 比較的結果是 NULL 不是 true，
// 就算被求值也不會誤放行。
func optTime(v *time.Time) (bool, pgtype.Timestamptz) {
	if v == nil {
		return false, ts(time.Time{})
	}
	return true, ts(*v)
}

// derefString 把 nullable text 攤平成空字串。
//
// 對外契約本來就用空字串表示「沒有」（proto 全檔沒有 optional，`omitempty` 也只在
// 少數欄位），所以攤平在這一層做一次，比讓每個呼叫端各自解 nil 安全。
func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// ---------------------------------------------------------------------------
// operation 讀取視圖的共同列形狀
//
// 五支 operation 視圖（賣家單筆／ADM 單筆／62R 列表／ADM 列表／priority 列表）
// 刻意 SELECT 完全相同的欄位集合，就是為了共用這一個轉換：形狀不同的話每支都要
// 一個 mapper，而少填一個欄位的症狀是「該欄恆為零值」——零值與真實值無法分辨。
// ---------------------------------------------------------------------------

type operationViewRow struct {
	operation                gen.InvoiceOperation
	issuerRefID              int64
	documentStatus           string
	documentSourceRef        *string
	documentManualReasonCode *string
	documentKind             string
	externalInvoiceNo        *string
	externalInvoiceDate      pgtype.Date
	externalReportedAt       *time.Time
}

func (r operationViewRow) toDomain() domain.OperationView {
	return domain.OperationView{
		Operation:                toOperation(r.operation),
		IssuerRefID:              r.issuerRefID,
		DocumentStatus:           r.documentStatus,
		DocumentSourceRef:        derefString(r.documentSourceRef),
		DocumentManualReasonCode: derefString(r.documentManualReasonCode),
		DocumentKind:             r.documentKind,
		ExternalInvoiceNo:        derefString(r.externalInvoiceNo),
		ExternalInvoiceDate:      pgxutil.DateValue(r.externalInvoiceDate),
		ExternalReportedAt:       r.externalReportedAt,
	}
}

func toOperationViews(rows []operationViewRow) []domain.OperationView {
	out := make([]domain.OperationView, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out
}

// ---------------------------------------------------------------------------
// DocumentRepository：買家面
// ---------------------------------------------------------------------------

// ListOrderDocumentsForBuyer 回一張訂單底下**全部**屬於這位買家的文件。
//
// 🔴 回 slice 不是單筆：VOID_AND_REISSUE 之後同一張訂單會有多份文件。
// GetForBuyer（:one）用在只需要一份的既有路徑，兩者不互相取代。
func (r *DocumentRepository) ListOrderDocumentsForBuyer(ctx context.Context, sourceRef string, buyerID int64) ([]domain.Document, error) {
	rows, err := r.q(ctx).ListOrderInvoiceDocumentsForBuyer(ctx, gen.ListOrderInvoiceDocumentsForBuyerParams{
		SourceRef: sp(sourceRef),
		BuyerID:   buyerID,
	})
	if err != nil {
		return nil, err
	}
	return toDocuments(rows)
}

// GetForBuyerByID 是 document-scoped 的授權讀取（端點 69／70／71）。
func (r *DocumentRepository) GetForBuyerByID(ctx context.Context, id, buyerID int64) (domain.Document, error) {
	row, err := r.q(ctx).GetBuyerInvoiceDocumentByID(ctx, gen.GetBuyerInvoiceDocumentByIDParams{
		ID:      id,
		BuyerID: buyerID,
	})
	if err != nil {
		return domain.Document{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toDocument(row)
}

func (r *DocumentRepository) buyerDocumentParams(in domain.BuyerDocumentQuery) (hasFrom bool, from pgtype.Timestamptz, hasTo bool, to pgtype.Timestamptz) {
	hasFrom, from = optTime(in.Dates.From)
	hasTo, to = optTime(in.Dates.To)
	return
}

// ListForBuyer 是端點 68（B09）的列表。
func (r *DocumentRepository) ListForBuyer(ctx context.Context, in domain.BuyerDocumentQuery) ([]domain.Document, error) {
	hasFrom, from, hasTo, to := r.buyerDocumentParams(in)
	rows, err := r.q(ctx).ListBuyerInvoiceDocuments(ctx, gen.ListBuyerInvoiceDocumentsParams{
		BuyerID:            in.BuyerID,
		HasDateFrom:        hasFrom,
		DateFrom:           from,
		HasDateTo:          hasTo,
		DateTo:             to,
		StatusFilter:       in.Status,
		DocumentKindFilter: in.DocumentKind,
		RowLimit:           in.Page.Limit,
		RowOffset:          in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	return toDocuments(rows)
}

// CountForBuyer 是端點 68 的 total。述詞與 ListForBuyer 同源。
func (r *DocumentRepository) CountForBuyer(ctx context.Context, in domain.BuyerDocumentQuery) (int64, error) {
	hasFrom, from, hasTo, to := r.buyerDocumentParams(in)
	return r.q(ctx).CountBuyerInvoiceDocuments(ctx, gen.CountBuyerInvoiceDocumentsParams{
		BuyerID:            in.BuyerID,
		HasDateFrom:        hasFrom,
		DateFrom:           from,
		HasDateTo:          hasTo,
		DateTo:             to,
		StatusFilter:       in.Status,
		DocumentKindFilter: in.DocumentKind,
	})
}

// ---------------------------------------------------------------------------
// DocumentRepository：賣家面（銷項 ＋ 進項）
// ---------------------------------------------------------------------------

// GetForSeller 是端點 31／32 的授權讀取，一併帶出 latestJobId。
func (r *DocumentRepository) GetForSeller(ctx context.Context, id, storeID int64) (domain.SellerDocumentItem, error) {
	row, err := r.q(ctx).GetSellerInvoiceDocumentByID(ctx, gen.GetSellerInvoiceDocumentByIDParams{
		ID:      id,
		StoreID: storeID,
	})
	if err != nil {
		return domain.SellerDocumentItem{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	doc, err := toDocument(row.InvoiceDocument)
	if err != nil {
		return domain.SellerDocumentItem{}, err
	}
	return domain.SellerDocumentItem{Document: doc, LatestOperationID: row.LatestOperationID}, nil
}

// ListForSeller 是端點 30（S06）的列表。
func (r *DocumentRepository) ListForSeller(ctx context.Context, in domain.SellerDocumentQuery) ([]domain.SellerDocumentItem, error) {
	hasFrom, from := optTime(in.Dates.From)
	hasTo, to := optTime(in.Dates.To)
	rows, err := r.q(ctx).ListSellerInvoiceDocuments(ctx, gen.ListSellerInvoiceDocumentsParams{
		StoreID:          in.StoreID,
		HasDateFrom:      hasFrom,
		DateFrom:         from,
		HasDateTo:        hasTo,
		DateTo:           to,
		ModeFilter:       in.Mode,
		StatusFilter:     in.Status,
		OrderNo:          in.OrderNo,
		AbnormalOnly:     in.AbnormalOnly,
		AbnormalStatuses: in.AbnormalStatuses,
		RowLimit:         in.Page.Limit,
		RowOffset:        in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.SellerDocumentItem, 0, len(rows))
	for _, row := range rows {
		doc, err := toDocument(row.InvoiceDocument)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.SellerDocumentItem{Document: doc, LatestOperationID: row.LatestOperationID})
	}
	return out, nil
}

// CountForSeller 是端點 30 的 total。
func (r *DocumentRepository) CountForSeller(ctx context.Context, in domain.SellerDocumentQuery) (int64, error) {
	hasFrom, from := optTime(in.Dates.From)
	hasTo, to := optTime(in.Dates.To)
	return r.q(ctx).CountSellerInvoiceDocuments(ctx, gen.CountSellerInvoiceDocumentsParams{
		StoreID:          in.StoreID,
		HasDateFrom:      hasFrom,
		DateFrom:         from,
		HasDateTo:        hasTo,
		DateTo:           to,
		ModeFilter:       in.Mode,
		StatusFilter:     in.Status,
		OrderNo:          in.OrderNo,
		AbnormalOnly:     in.AbnormalOnly,
		AbnormalStatuses: in.AbnormalStatuses,
	})
}

// ListRecentForIssuer 給端點 16 的固定筆數區塊。
func (r *DocumentRepository) ListRecentForIssuer(ctx context.Context, issuerID int64, limit int32) ([]domain.Document, error) {
	rows, err := r.q(ctx).ListRecentSellerInvoiceDocuments(ctx, gen.ListRecentSellerInvoiceDocumentsParams{
		IssuerID: issuerID,
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toDocuments(rows)
}

// ListReceivedForSeller 是端點 72（S12 進項）的列表。
func (r *DocumentRepository) ListReceivedForSeller(ctx context.Context, in domain.ReceivedDocumentQuery) ([]domain.ReceivedDocumentItem, error) {
	hasFrom, from := optTime(in.Dates.From)
	hasTo, to := optTime(in.Dates.To)
	rows, err := r.q(ctx).ListSellerReceivedInvoiceDocuments(ctx, gen.ListSellerReceivedInvoiceDocumentsParams{
		StoreID:      in.StoreID,
		HasDateFrom:  hasFrom,
		DateFrom:     from,
		HasDateTo:    hasTo,
		DateTo:       to,
		StatusFilter: in.Status,
		RowLimit:     in.Page.Limit,
		RowOffset:    in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ReceivedDocumentItem, 0, len(rows))
	for _, row := range rows {
		doc, err := toDocument(row.InvoiceDocument)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ReceivedDocumentItem{
			Document:             doc,
			FirstLineDescription: row.FirstLineDescription,
			LineCount:            row.LineCount,
		})
	}
	return out, nil
}

// CountReceivedForSeller 是端點 72 的 total。
func (r *DocumentRepository) CountReceivedForSeller(ctx context.Context, in domain.ReceivedDocumentQuery) (int64, error) {
	hasFrom, from := optTime(in.Dates.From)
	hasTo, to := optTime(in.Dates.To)
	return r.q(ctx).CountSellerReceivedInvoiceDocuments(ctx, gen.CountSellerReceivedInvoiceDocumentsParams{
		StoreID:      in.StoreID,
		HasDateFrom:  hasFrom,
		DateFrom:     from,
		HasDateTo:    hasTo,
		DateTo:       to,
		StatusFilter: in.Status,
	})
}

// ---------------------------------------------------------------------------
// DocumentRepository：ADM
// ---------------------------------------------------------------------------

// ListForAdmin 是端點 47（A12）的列表。StoreID 為 nil ＝ 全平台。
func (r *DocumentRepository) ListForAdmin(ctx context.Context, in domain.AdminDocumentQuery) ([]domain.AdminDocumentItem, error) {
	hasStore, storeID := optInt64(in.StoreID)
	hasFrom, from := optTime(in.Dates.From)
	hasTo, to := optTime(in.Dates.To)
	rows, err := r.q(ctx).ListInvoiceDocumentsForAdmin(ctx, gen.ListInvoiceDocumentsForAdminParams{
		HasStoreID:   hasStore,
		StoreID:      storeID,
		OrderNo:      in.OrderNo,
		StatusFilter: in.Status,
		ModeFilter:   in.Mode,
		HasDateFrom:  hasFrom,
		DateFrom:     from,
		HasDateTo:    hasTo,
		DateTo:       to,
		RowLimit:     in.Page.Limit,
		RowOffset:    in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AdminDocumentItem, 0, len(rows))
	for _, row := range rows {
		doc, err := toDocument(row.InvoiceDocument)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.AdminDocumentItem{
			Document:          doc,
			IssuerType:        row.IssuerType,
			IssuerRefID:       row.IssuerRefID,
			LatestOperationID: row.LatestOperationID,
		})
	}
	return out, nil
}

// CountForAdmin 是端點 47 的 total。
func (r *DocumentRepository) CountForAdmin(ctx context.Context, in domain.AdminDocumentQuery) (int64, error) {
	hasStore, storeID := optInt64(in.StoreID)
	hasFrom, from := optTime(in.Dates.From)
	hasTo, to := optTime(in.Dates.To)
	return r.q(ctx).CountInvoiceDocumentsForAdmin(ctx, gen.CountInvoiceDocumentsForAdminParams{
		HasStoreID:   hasStore,
		StoreID:      storeID,
		OrderNo:      in.OrderNo,
		StatusFilter: in.Status,
		ModeFilter:   in.Mode,
		HasDateFrom:  hasFrom,
		DateFrom:     from,
		HasDateTo:    hasTo,
		DateTo:       to,
	})
}

// ---------------------------------------------------------------------------
// OperationRepository：五支讀取視圖
// ---------------------------------------------------------------------------

// GetViewForSeller 是端點 33（S08）。store_id 是述詞的一部分 ⇒ 越權與不存在同回 9340。
func (r *OperationRepository) GetViewForSeller(ctx context.Context, id, storeID int64) (domain.OperationView, error) {
	row, err := r.q(ctx).GetSellerInvoiceOperationByID(ctx, gen.GetSellerInvoiceOperationByIDParams{
		ID:      id,
		StoreID: storeID,
	})
	if err != nil {
		return domain.OperationView{}, notFound(err, bizcode.InvoiceJobNotFound)
	}
	return operationViewRow{
		operation:                row.InvoiceOperation,
		issuerRefID:              row.IssuerRefID,
		documentStatus:           row.DocumentStatus,
		documentSourceRef:        row.DocumentSourceRef,
		documentManualReasonCode: row.DocumentManualReasonCode,
		documentKind:             row.DocumentKind,
		externalInvoiceNo:        row.DocumentExternalInvoiceNo,
		externalInvoiceDate:      row.DocumentExternalInvoiceDate,
		externalReportedAt:       row.DocumentExternalReportedAt,
	}.toDomain(), nil
}

// GetViewForAdmin 是端點 51（A06）。ADM 看全平台，因此**沒有**擁有者述詞——
// 與賣家版分成兩支而不是加一個「要不要檢查 store_id」的旗標：旗標式授權一個傳錯
// 就整條防線失效。
func (r *OperationRepository) GetViewForAdmin(ctx context.Context, id int64) (domain.OperationView, error) {
	row, err := r.q(ctx).GetInvoiceOperationForAdminByID(ctx, id)
	if err != nil {
		return domain.OperationView{}, notFound(err, bizcode.InvoiceJobNotFound)
	}
	return operationViewRow{
		operation:                row.InvoiceOperation,
		issuerRefID:              row.IssuerRefID,
		documentStatus:           row.DocumentStatus,
		documentSourceRef:        row.DocumentSourceRef,
		documentManualReasonCode: row.DocumentManualReasonCode,
		documentKind:             row.DocumentKind,
		externalInvoiceNo:        row.DocumentExternalInvoiceNo,
		externalInvoiceDate:      row.DocumentExternalInvoiceDate,
		externalReportedAt:       row.DocumentExternalReportedAt,
	}.toDomain(), nil
}

// ListOrderExternalIssuances 是子路由 62R。
//
// 🔴 allowedStatuses 由呼叫端以**常數**傳入，不得接成使用者可控輸入：述詞是這條
// 路由的安全邊界（放寬就退化成 S08 的等價物，而 S08 對停權賣家是 BLOCK）。
func (r *OperationRepository) ListOrderExternalIssuances(ctx context.Context, storeID int64, orderNo string, allowedStatuses []string, page domain.Page) ([]domain.OperationView, error) {
	rows, err := r.q(ctx).ListOrderExternalIssuances(ctx, gen.ListOrderExternalIssuancesParams{
		StoreID:         storeID,
		OrderNo:         orderNo,
		AllowedStatuses: allowedStatuses,
		RowLimit:        page.Limit,
		RowOffset:       page.Offset,
	})
	if err != nil {
		return nil, err
	}
	views := make([]operationViewRow, 0, len(rows))
	for _, row := range rows {
		views = append(views, operationViewRow{
			operation:                row.InvoiceOperation,
			issuerRefID:              row.IssuerRefID,
			documentStatus:           row.DocumentStatus,
			documentSourceRef:        row.DocumentSourceRef,
			documentManualReasonCode: row.DocumentManualReasonCode,
			documentKind:             row.DocumentKind,
			externalInvoiceNo:        row.DocumentExternalInvoiceNo,
			externalInvoiceDate:      row.DocumentExternalInvoiceDate,
			externalReportedAt:       row.DocumentExternalReportedAt,
		})
	}
	return toOperationViews(views), nil
}

// CountOrderExternalIssuances 是 62R 的 total。
func (r *OperationRepository) CountOrderExternalIssuances(ctx context.Context, storeID int64, orderNo string, allowedStatuses []string) (int64, error) {
	return r.q(ctx).CountOrderExternalIssuances(ctx, gen.CountOrderExternalIssuancesParams{
		StoreID:         storeID,
		OrderNo:         orderNo,
		AllowedStatuses: allowedStatuses,
	})
}

// ListViewsForAdmin 是端點 50（A05）的列表。
func (r *OperationRepository) ListViewsForAdmin(ctx context.Context, in domain.AdminOperationQuery) ([]domain.OperationView, error) {
	hasStore, storeID := optInt64(in.StoreID)
	rows, err := r.q(ctx).ListInvoiceOperationsForAdmin(ctx, gen.ListInvoiceOperationsForAdminParams{
		StatusFilters:            in.StatusFilters,
		IncludeManualDocuments:   in.IncludeManualDocuments,
		ManualDocumentStatuses:   in.ManualDocumentStatuses,
		HasStoreID:               hasStore,
		StoreID:                  storeID,
		OrderNo:                  in.OrderNo,
		OperationTypeFilter:      in.OperationType,
		HasUploadStatus:          in.HasUploadStatus,
		UploadStatusCodes:        in.UploadStatusCodes,
		UploadStatusIncludeNull:  in.UploadStatusIncludeNull,
		ManualReasonFilter:       in.ManualReason,
		ExcludeResolvedDocuments: in.ExcludeResolvedDocuments,
		ResolvedDocumentStatuses: in.ResolvedDocumentStatuses,
		RowLimit:                 in.Page.Limit,
		RowOffset:                in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	views := make([]operationViewRow, 0, len(rows))
	for _, row := range rows {
		views = append(views, operationViewRow{
			operation:                row.InvoiceOperation,
			issuerRefID:              row.IssuerRefID,
			documentStatus:           row.DocumentStatus,
			documentSourceRef:        row.DocumentSourceRef,
			documentManualReasonCode: row.DocumentManualReasonCode,
			documentKind:             row.DocumentKind,
			externalInvoiceNo:        row.DocumentExternalInvoiceNo,
			externalInvoiceDate:      row.DocumentExternalInvoiceDate,
			externalReportedAt:       row.DocumentExternalReportedAt,
		})
	}
	return toOperationViews(views), nil
}

// CountViewsForAdmin 是端點 50 的 total。
func (r *OperationRepository) CountViewsForAdmin(ctx context.Context, in domain.AdminOperationQuery) (int64, error) {
	hasStore, storeID := optInt64(in.StoreID)
	return r.q(ctx).CountInvoiceOperationsForAdmin(ctx, gen.CountInvoiceOperationsForAdminParams{
		StatusFilters:            in.StatusFilters,
		IncludeManualDocuments:   in.IncludeManualDocuments,
		ManualDocumentStatuses:   in.ManualDocumentStatuses,
		HasStoreID:               hasStore,
		StoreID:                  storeID,
		OrderNo:                  in.OrderNo,
		OperationTypeFilter:      in.OperationType,
		HasUploadStatus:          in.HasUploadStatus,
		UploadStatusCodes:        in.UploadStatusCodes,
		UploadStatusIncludeNull:  in.UploadStatusIncludeNull,
		ManualReasonFilter:       in.ManualReason,
		ExcludeResolvedDocuments: in.ExcludeResolvedDocuments,
		ResolvedDocumentStatuses: in.ResolvedDocumentStatuses,
	})
}

// ListPriorityViewsForAdmin 是端點 37 的 priorityJobs（依法定期限排序）。
func (r *OperationRepository) ListPriorityViewsForAdmin(ctx context.Context, in domain.PriorityOperationQuery) ([]domain.OperationView, error) {
	rows, err := r.q(ctx).ListPriorityInvoiceOperationsForAdmin(ctx, gen.ListPriorityInvoiceOperationsForAdminParams{
		OpenStatuses:             in.OpenStatuses,
		ManualDocumentStatuses:   in.ManualDocumentStatuses,
		ResolvedDocumentStatuses: in.ResolvedDocumentStatuses,
		RowLimit:                 in.Limit,
	})
	if err != nil {
		return nil, err
	}
	views := make([]operationViewRow, 0, len(rows))
	for _, row := range rows {
		views = append(views, operationViewRow{
			operation:                row.InvoiceOperation,
			issuerRefID:              row.IssuerRefID,
			documentStatus:           row.DocumentStatus,
			documentSourceRef:        row.DocumentSourceRef,
			documentManualReasonCode: row.DocumentManualReasonCode,
			documentKind:             row.DocumentKind,
			externalInvoiceNo:        row.DocumentExternalInvoiceNo,
			externalInvoiceDate:      row.DocumentExternalInvoiceDate,
			externalReportedAt:       row.DocumentExternalReportedAt,
		})
	}
	return toOperationViews(views), nil
}

// ---------------------------------------------------------------------------
// ComplaintRepository
// ---------------------------------------------------------------------------

// GetForSeller 是端點 35／36 的授權讀取（經 invoice_issuers 解析 store_id）。
func (r *ComplaintRepository) GetForSeller(ctx context.Context, id, storeID int64) (domain.Complaint, error) {
	row, err := r.q(ctx).GetSellerInvoiceComplaintByID(ctx, gen.GetSellerInvoiceComplaintByIDParams{
		ID:      id,
		StoreID: storeID,
	})
	if err != nil {
		return domain.Complaint{}, notFound(err, bizcode.InvoiceComplaintNotFound)
	}
	return toComplaint(row), nil
}

// ListForBuyerOrder 是端點 13 的列表。
func (r *ComplaintRepository) ListForBuyerOrder(ctx context.Context, in domain.BuyerComplaintQuery) ([]domain.Complaint, error) {
	rows, err := r.q(ctx).ListBuyerOrderInvoiceComplaints(ctx, gen.ListBuyerOrderInvoiceComplaintsParams{
		ComplainantBuyerID: in.BuyerID,
		OrderNo:            in.OrderNo,
		RowLimit:           in.Page.Limit,
		RowOffset:          in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	return toComplaints(rows), nil
}

// CountForBuyerOrder 是端點 13 的 total。
func (r *ComplaintRepository) CountForBuyerOrder(ctx context.Context, in domain.BuyerComplaintQuery) (int64, error) {
	return r.q(ctx).CountBuyerOrderInvoiceComplaints(ctx, gen.CountBuyerOrderInvoiceComplaintsParams{
		ComplainantBuyerID: in.BuyerID,
		OrderNo:            in.OrderNo,
	})
}

// ListForSeller 是端點 34（S09）的列表。
func (r *ComplaintRepository) ListForSeller(ctx context.Context, in domain.SellerComplaintQuery) ([]domain.Complaint, error) {
	rows, err := r.q(ctx).ListSellerInvoiceComplaints(ctx, gen.ListSellerInvoiceComplaintsParams{
		StoreID:       in.StoreID,
		StatusFilters: in.StatusFilters,
		SortByDue:     in.SortByDue,
		RowLimit:      in.Page.Limit,
		RowOffset:     in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	return toComplaints(rows), nil
}

// CountForSeller 是端點 34 的 total。
func (r *ComplaintRepository) CountForSeller(ctx context.Context, in domain.SellerComplaintQuery) (int64, error) {
	return r.q(ctx).CountSellerInvoiceComplaints(ctx, gen.CountSellerInvoiceComplaintsParams{
		StoreID:       in.StoreID,
		StatusFilters: in.StatusFilters,
	})
}

// ListForAdmin 是端點 58（A07）的列表，含滑動窗違規計數。
func (r *ComplaintRepository) ListForAdmin(ctx context.Context, in domain.AdminComplaintQuery) ([]domain.AdminComplaintItem, error) {
	hasStore, storeID := optInt64(in.StoreID)
	rows, err := r.q(ctx).ListInvoiceComplaintsForAdmin(ctx, gen.ListInvoiceComplaintsForAdminParams{
		ViolationStatus:      in.ViolationStatus,
		ViolationWindowStart: ts(in.ViolationWindowStart),
		StatusFilters:        in.StatusFilters,
		IncludeDueOverdue:    in.IncludeDueOverdue,
		DueOverdueStatuses:   in.DueOverdueStatuses,
		Now:                  ts(in.Now),
		HasStoreID:           hasStore,
		StoreID:              storeID,
		OrderNo:              in.OrderNo,
		SortByDue:            in.SortByDue,
		RowLimit:             in.Page.Limit,
		RowOffset:            in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AdminComplaintItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AdminComplaintItem{
			Complaint:              toComplaint(row.InvoiceComplaint),
			IssuerRefID:            row.IssuerRefID,
			ViolationCountInWindow: row.ViolationCountInWindow,
		})
	}
	return out, nil
}

// CountForAdmin 是端點 58 的 total。
func (r *ComplaintRepository) CountForAdmin(ctx context.Context, in domain.AdminComplaintQuery) (int64, error) {
	hasStore, storeID := optInt64(in.StoreID)
	return r.q(ctx).CountInvoiceComplaintsForAdmin(ctx, gen.CountInvoiceComplaintsForAdminParams{
		StatusFilters:      in.StatusFilters,
		IncludeDueOverdue:  in.IncludeDueOverdue,
		DueOverdueStatuses: in.DueOverdueStatuses,
		Now:                ts(in.Now),
		HasStoreID:         hasStore,
		StoreID:            storeID,
		OrderNo:            in.OrderNo,
	})
}

// ---------------------------------------------------------------------------
// CorrectionRequestRepository
// ---------------------------------------------------------------------------

// GetForBuyer 是端點 12 的授權讀取。三段述詞全在 SQL 裡。
func (r *CorrectionRequestRepository) GetForBuyer(ctx context.Context, id, buyerID int64, sourceRef string) (domain.CorrectionRequest, error) {
	row, err := r.q(ctx).GetInvoiceCorrectionRequestForBuyer(ctx, gen.GetInvoiceCorrectionRequestForBuyerParams{
		ID:        id,
		BuyerID:   buyerID,
		SourceRef: sourceRef,
	})
	if err != nil {
		// 端點 12 的 not-found 碼是 **9301**（InvoiceDocumentNotFound）而不是另開一個
		// correction 專用碼：§8.1.4(c) 明令不得為「越權」或這類衍生資源單獨開碼，
		// 那等於在回應裡放一個可以區分「不存在」與「不屬於你」的旗標。
		return domain.CorrectionRequest{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toCorrectionRequest(row), nil
}

// ---------------------------------------------------------------------------
// SellerTaxProfileRepository
// ---------------------------------------------------------------------------

// GetByStoreID 經 invoice_issuers 解析 store_id。
func (r *SellerTaxProfileRepository) GetByStoreID(ctx context.Context, storeID int64) (domain.SellerTaxProfile, error) {
	row, err := r.q(ctx).GetSellerTaxProfileByStoreID(ctx, storeID)
	if err != nil {
		return domain.SellerTaxProfile{}, notFound(err, bizcode.SellerTaxProfileNotFound)
	}
	return toSellerTaxProfile(row), nil
}

// ListReviewsForAdmin 是端點 38（A02）的列表。
func (r *SellerTaxProfileRepository) ListReviewsForAdmin(ctx context.Context, in domain.AdminTaxReviewQuery) ([]domain.AdminTaxReviewItem, error) {
	rows, err := r.q(ctx).ListTaxReviewsForAdmin(ctx, gen.ListTaxReviewsForAdminParams{
		OpenComplaintStatuses: in.OpenComplaintStatuses,
		StatusFilters:         in.StatusFilters,
		NearDeadlineOnly:      in.NearDeadlineOnly,
		NearDeadlineBefore:    ts(in.NearDeadlineBefore),
		ExtendedOnly:          in.ExtendedOnly,
		Keyword:               in.Keyword,
		RowLimit:              in.Page.Limit,
		RowOffset:             in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AdminTaxReviewItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AdminTaxReviewItem{
			Profile:            toSellerTaxProfile(row.SellerTaxProfile),
			StoreID:            row.IssuerRefID,
			OpenComplaintCount: row.OpenComplaintCount,
		})
	}
	return out, nil
}

// CountReviewsForAdmin 是端點 38 的 total。
func (r *SellerTaxProfileRepository) CountReviewsForAdmin(ctx context.Context, in domain.AdminTaxReviewQuery) (int64, error) {
	return r.q(ctx).CountTaxReviewsForAdmin(ctx, gen.CountTaxReviewsForAdminParams{
		StatusFilters:      in.StatusFilters,
		NearDeadlineOnly:   in.NearDeadlineOnly,
		NearDeadlineBefore: ts(in.NearDeadlineBefore),
		ExtendedOnly:       in.ExtendedOnly,
		Keyword:            in.Keyword,
	})
}

// ---------------------------------------------------------------------------
// ProviderAccountRepository
// ---------------------------------------------------------------------------

// GetByStoreID 經 invoice_issuers 解析 store_id。查無列 ⇒ 9330。
func (r *ProviderAccountRepository) GetByStoreID(ctx context.Context, storeID int64, provider string) (domain.ProviderAccount, error) {
	row, err := r.q(ctx).GetProviderAccountByStoreID(ctx, gen.GetProviderAccountByStoreIDParams{
		StoreID:  storeID,
		Provider: provider,
	})
	if err != nil {
		return domain.ProviderAccount{}, notFound(err, bizcode.InvoiceProviderAccountNotFound)
	}
	return toProviderAccount(row), nil
}

// ListForAdmin 是端點 42（A04）的列表。
//
// 🔴 Item.Account 為 nil ＝ 這個賣家還沒建 provider account 列，對外投影
// NOT_STARTED（§6.1.13）。把它當錯誤或跳過該列，上線當天會少掉全體賣家。
func (r *ProviderAccountRepository) ListForAdmin(ctx context.Context, in domain.AdminProviderAccountQuery) ([]domain.AdminProviderAccountItem, error) {
	rows, err := r.q(ctx).ListProviderAccountsForAdmin(ctx, gen.ListProviderAccountsForAdminParams{
		Provider:        in.Provider,
		StatusFilter:    in.StatusFilter,
		MatchNotStarted: in.MatchNotStarted,
		Keyword:         in.Keyword,
		RowLimit:        in.Page.Limit,
		RowOffset:       in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AdminProviderAccountItem, 0, len(rows))
	for _, row := range rows {
		item := domain.AdminProviderAccountItem{Issuer: toIssuer(row.InvoiceIssuer)}
		if row.AccountID != nil {
			item.Account = &domain.ProviderAccountSummary{
				ID:                  *row.AccountID,
				Provider:            derefString(row.Provider),
				ProviderTaxID:       derefString(row.ProviderTaxID),
				Status:              derefString(row.AccountStatus),
				StatusReason:        row.StatusReason,
				CredentialLast4:     derefString(row.CredentialLast4),
				VerifiedAt:          row.VerifiedAt,
				AutomationCutoverAt: row.AutomationCutoverAt,
				LastHealthCheckedAt: row.LastHealthCheckedAt,
			}
			if row.AccountVersion != nil {
				item.Account.Version = *row.AccountVersion
			}
			if row.AccountUpdatedAt != nil {
				item.Account.UpdatedAt = *row.AccountUpdatedAt
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// CountForAdmin 是端點 42 的 total。
func (r *ProviderAccountRepository) CountForAdmin(ctx context.Context, in domain.AdminProviderAccountQuery) (int64, error) {
	return r.q(ctx).CountProviderAccountsForAdmin(ctx, gen.CountProviderAccountsForAdminParams{
		Provider:        in.Provider,
		StatusFilter:    in.StatusFilter,
		MatchNotStarted: in.MatchNotStarted,
		Keyword:         in.Keyword,
	})
}

// ---------------------------------------------------------------------------
// AuditRepository
// ---------------------------------------------------------------------------

func adminAuditLogParams(in domain.AdminAuditLogQuery) (hasActor bool, actorID int64, hasStore bool, storeID int64, hasDoc bool, docID int64, hasJob bool, jobID int64, hasFrom bool, from pgtype.Timestamptz, hasTo bool, to pgtype.Timestamptz) {
	hasActor, actorID = optInt64(in.TargetActorAccountID)
	hasStore, storeID = optInt64(in.StoreID)
	hasDoc, docID = optInt64(in.DocumentID)
	hasJob, jobID = optInt64(in.JobID)
	hasFrom, from = optTime(in.Dates.From)
	hasTo, to = optTime(in.Dates.To)
	return
}

// ListAdminAuditLogsForAdmin 是端點 61（A10）的列表。
func (r *AuditRepository) ListAdminAuditLogsForAdmin(ctx context.Context, in domain.AdminAuditLogQuery) ([]domain.AdminAuditLogItem, error) {
	hasActor, actorID, hasStore, storeID, hasDoc, docID, hasJob, jobID, hasFrom, from, hasTo, to := adminAuditLogParams(in)
	rows, err := r.q(ctx).ListInvoiceAuditLogsForAdmin(ctx, gen.ListInvoiceAuditLogsForAdminParams{
		JobResourceType:             in.ResourceTypes.Job,
		ComplaintResourceType:       in.ResourceTypes.Complaint,
		TaxProfileResourceType:      in.ResourceTypes.TaxProfile,
		ProviderAccountResourceType: in.ResourceTypes.ProviderAccount,
		DocumentResourceType:        in.ResourceTypes.Document,
		HasTargetActor:              hasActor,
		TargetActorAccountID:        actorID,
		HasStoreID:                  hasStore,
		StoreID:                     storeID,
		OrderNo:                     in.OrderNo,
		HasDocumentID:               hasDoc,
		DocumentID:                  docID,
		HasJobID:                    hasJob,
		JobID:                       jobID,
		OperationTypeFilter:         in.OperationType,
		HasDateFrom:                 hasFrom,
		DateFrom:                    from,
		HasDateTo:                   hasTo,
		DateTo:                      to,
		RowLimit:                    in.Page.Limit,
		RowOffset:                   in.Page.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AdminAuditLogItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AdminAuditLogItem{
			Entry:                 toAdminAuditLogEntry(row.InvoiceAdminAuditLog),
			StoreID:               row.IssuerRefID,
			OrderNo:               derefString(row.DocumentSourceRef),
			ResolvedDocumentID:    row.ResolvedDocumentID,
			ResolvedOperationID:   row.ResolvedOperationID,
			ResolvedOperationType: derefString(row.ResolvedOperationType),
		})
	}
	return out, nil
}

// CountAdminAuditLogsForAdmin 是端點 61 的 total。
func (r *AuditRepository) CountAdminAuditLogsForAdmin(ctx context.Context, in domain.AdminAuditLogQuery) (int64, error) {
	hasActor, actorID, hasStore, storeID, hasDoc, docID, hasJob, jobID, hasFrom, from, hasTo, to := adminAuditLogParams(in)
	return r.q(ctx).CountInvoiceAuditLogsForAdmin(ctx, gen.CountInvoiceAuditLogsForAdminParams{
		JobResourceType:             in.ResourceTypes.Job,
		ComplaintResourceType:       in.ResourceTypes.Complaint,
		TaxProfileResourceType:      in.ResourceTypes.TaxProfile,
		ProviderAccountResourceType: in.ResourceTypes.ProviderAccount,
		DocumentResourceType:        in.ResourceTypes.Document,
		HasTargetActor:              hasActor,
		TargetActorAccountID:        actorID,
		HasStoreID:                  hasStore,
		StoreID:                     storeID,
		OrderNo:                     in.OrderNo,
		HasDocumentID:               hasDoc,
		DocumentID:                  docID,
		HasJobID:                    hasJob,
		JobID:                       jobID,
		OperationTypeFilter:         in.OperationType,
		HasDateFrom:                 hasFrom,
		DateFrom:                    from,
		HasDateTo:                   hasTo,
		DateTo:                      to,
	})
}

// ---------------------------------------------------------------------------
// OverviewRepository
// ---------------------------------------------------------------------------

// OverviewRepository 服務兩支總覽端點的計數。
//
// 它不屬於任何單一聚合（兩支各自橫跨 3–4 張表），所以獨立成一個 repository 而不是
// 硬掛在 DocumentRepository 上。
type OverviewRepository struct {
	baseRepo
}

// NewOverviewRepository 建立 repository。
func NewOverviewRepository(pool *pgxpool.Pool) *OverviewRepository {
	return &OverviewRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.OverviewRepository = (*OverviewRepository)(nil)

// SellerCounts 是端點 16（S01）的三個計數，單一 SELECT 取回。
func (r *OverviewRepository) SellerCounts(ctx context.Context, in domain.SellerOverviewCountsQuery) (domain.SellerOverviewCounts, error) {
	row, err := r.q(ctx).GetSellerInvoiceOverviewCounts(ctx, gen.GetSellerInvoiceOverviewCountsParams{
		IssuerID:                 in.IssuerID,
		PendingDocumentStatuses:  in.PendingDocumentStatuses,
		OpenComplaintStatuses:    in.OpenComplaintStatuses,
		OverdueComplaintStatuses: in.OverdueComplaintStatuses,
		Now:                      ts(in.Now),
	})
	if err != nil {
		return domain.SellerOverviewCounts{}, err
	}
	return domain.SellerOverviewCounts{
		PendingDocumentCount:  row.PendingDocumentCount,
		OpenComplaintCount:    row.OpenComplaintCount,
		OverdueComplaintCount: row.OverdueComplaintCount,
	}, nil
}

// AdminCounts 是端點 37（A01）的七個 KPI，單一 SELECT 取回。
//
// ⚠️ 第 8 個 KPI `suspended_sellers` 不在這裡：停權狀態的權威是 seller-service。
func (r *OverviewRepository) AdminCounts(ctx context.Context, in domain.AdminOverviewCountsQuery) (domain.AdminOverviewCounts, error) {
	row, err := r.q(ctx).GetInvoiceOverviewCountsForAdmin(ctx, gen.GetInvoiceOverviewCountsForAdminParams{
		TaxReviewQueueStatuses:   in.TaxReviewQueueStatuses,
		NearDeadlineBefore:       ts(in.NearDeadlineBefore),
		ProviderErrorStatuses:    in.ProviderErrorStatuses,
		PendingJobStatuses:       in.PendingJobStatuses,
		NeedsManualJobStatuses:   in.NeedsManualJobStatuses,
		ManualDocumentStatuses:   in.ManualDocumentStatuses,
		ResolvedDocumentStatuses: in.ResolvedDocumentStatuses,
		OpenComplaintStatuses:    in.OpenComplaintStatuses,
		OverdueComplaintStatuses: in.OverdueComplaintStatuses,
		Now:                      ts(in.Now),
	})
	if err != nil {
		return domain.AdminOverviewCounts{}, err
	}
	return domain.AdminOverviewCounts{
		PendingTaxReviewCount: row.PendingTaxReviewCount,
		NearDeadlineCount:     row.NearDeadlineCount,
		ProviderErrorCount:    row.ProviderErrorCount,
		PendingJobCount:       row.PendingJobCount,
		NeedsManualCount:      row.NeedsManualCount,
		OpenComplaintCount:    row.OpenComplaintCount,
		OverdueComplaintCount: row.OverdueComplaintCount,
	}, nil
}
