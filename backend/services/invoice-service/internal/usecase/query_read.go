package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)



const (
	listDefaultPageSize int32 = 20
	// ListMaxPageSize 由 gRPC handler 讀取，用來在型別邊界回 400。
	ListMaxPageSize int32 = 100
)

// ErrInvalidDate 是 `YYYY-MM-DD` 解析失敗。
//
// 它是**契約違規不是業務結果** ⇒ 呼叫端映成 InvalidArgument（400），不是業務碼。
// 嚴格驗證是實測出來的坑：AMEGO 的 invoice_list 對帶連字號的日期與空字串都回
// `code:0` 且回**全部歷史**，文件宣稱的 code:32 從未觸發（§6.1.10）⇒ 格式驗證
// 必須在最上游一次做對。
var ErrInvalidDate = errors.New("invoice: date must be YYYY-MM-DD")

// ParseInvoiceDateRange 把兩個 `YYYY-MM-DD`（UTC+8 日界）折成 UTC 半閉區間。
//
// 🔴 日界是**台北**不是 UTC：直接把字串當 UTC 解析，會讓每天 00:00–08:00（UTC+8）
// 開立的發票落到前一天的區間裡——錯誤只在每天的前八小時出現，任何一次 dev 驗證都
// 極難重現（同一個坑的完整說明見 domain.AmegoPeriod）。
//
// 上界取當日 23:59:59（台北），與 SQL 的 `<=` 相符；取次日 00:00 會把次日剛好落在
// 整點的那一列也算進來。
func ParseInvoiceDateRange(from, to string) (domain.DateRange, error) {
	var out domain.DateRange
	if from != "" {
		t, err := parseTaipeiDay(from)
		if err != nil {
			return domain.DateRange{}, err
		}
		utc := t.UTC()
		out.From = &utc
	}
	if to != "" {
		t, err := parseTaipeiDay(to)
		if err != nil {
			return domain.DateRange{}, err
		}
		end := t.Add(24*time.Hour - time.Second).UTC()
		out.To = &end
	}
	return out, nil
}

func parseTaipeiDay(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, timeutil.LocationTaipei)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %q", ErrInvalidDate, s)
	}
	return t, nil
}

func pageOf(page, pageSize int32) (domain.Page, int32, int32) {
	normPage, normSize, offset := pagination.NormalizePage(page, pageSize, listDefaultPageSize, ListMaxPageSize)
	return domain.Page{Limit: normSize, Offset: offset}, normPage, normSize
}


// DocumentListItem 是四個文件列表端點（68／30／72／47）的共用列。
//
// 三個推導欄各自只有一部分端點會填，未填時的零值語意寫在欄位註解上——
// **不得用零值反推「查不到」**（那正是 §6.1.13 三態表要防的形狀）。
type DocumentListItem struct {
	Document domain.Document

	// LatestOperationID ＝ 0 表示這份文件還沒有任何 operation（TSID 恆 > 0）。
	// 端點 68／72 不查它（買家與進項都沒有「查看異常」CTA）⇒ 那兩支恆為 0。
	LatestOperationID int64

	// IssuerRefID ＝ 0 表示未解析。只有端點 47（ADM）會填：買賣家面的列表
	// 擁有者已經是查詢述詞，再回一次 store_id 沒有用途。
	IssuerRefID int64

	// RecipientNote 只有端點 72（S12 進項）有值，由明細行推導。
	RecipientNote string
}

// DocumentListPage 是文件列表的一頁。
type DocumentListPage struct {
	Records  []DocumentListItem
	Total    int32
	Page     int32
	PageSize int32
}

// TimelineItem 是已投影成 CodeMeta InvoiceDocumentTimelineEvent（5 值）的一列時間軸。
type TimelineItem struct {
	OccurredAt time.Time
	Event      string
	Note       string
}

// DocumentDetailView 是買家與賣家的文件詳情（端點 9／69／31 共用）。
//
// 🔴 CarrierSummary 在**離開 usecase 之前**就已經是該 viewer 該看到的值：
// 買家（資料主體）是完整、賣家是遮罩、ADM 是明碼。遮罩由後端產出、前端不做
// （§8.1.3(c) 裁定 3）——前端遮罩時完整值仍在 response body 裡，會出現在 devtools、
// Gateway access log 的 resp_body 與前端 Sentry breadcrumb。
type DocumentDetailView struct {
	Document       domain.Document
	Issuer         domain.Issuer
	Lines          []domain.DocumentLine
	Timeline       []TimelineItem
	CarrierSummary string
	// CarrierType 是 CarrierSummary 的判別欄（#3084）：非空 ＝ summary 裝的是載具，
	// 空 ＝ 裝的是買受人統編或什麼都沒有。兩者由 carrierPresentationOf 同時決定。
	CarrierType       string
	LatestOperationID int64
}

// ComplaintPage 是申訴列表的一頁（端點 13／34 共用）。
type ComplaintPage struct {
	Records  []ComplaintView
	Total    int32
	Page     int32
	PageSize int32
}

// AdminComplaintPage 是端點 58 的一頁：申訴 ＋ 兩個只有 ADM 需要的推導欄。
type AdminComplaintPage struct {
	Records  []AdminComplaintView
	Total    int32
	Page     int32
	PageSize int32
}

// AdminComplaintView 是 A07 的一列。
//
// ViolationCountInWindow 是**滑動窗即時算**的（口徑與 CountConfirmedViolationOrders
// 逐字相同：只計 VIOLATION_CONFIRMED、以去重後的訂單數計）。⚠️ 一個月後重跑同一句
// SQL 會得到不同答案 ⇒ 要留存「當時為什麼建議停權」得寫 invoice_status_history，
// 不是靠這個欄位。
type AdminComplaintView struct {
	Complaint              ComplaintView
	StoreID                int64
	ViolationCountInWindow int64
}

// JobPage 是 job 列表的一頁（62R 與端點 50 共用）。
type JobPage struct {
	Records  []domain.OperationView
	Total    int32
	Page     int32
	PageSize int32
}


// BuyerDocumentsInput 是端點 68 的入參。
type BuyerDocumentsInput struct {
	BuyerID      int64
	Dates        domain.DateRange
	Status       string
	DocumentKind string
	Page         int32
	PageSize     int32
}

// ListBuyerDocuments 回這位買家名下的發票與收據（B09）。
//
// Tier A：`recipient_type='BUYER' AND recipient_ref_id = buyer_id` 在 SQL 的 WHERE。
// **與訂單無關**——方向三（平台 → 買家）的發票不掛在任何訂單底下，order-scoped 的
// 入口取不到它（§8.1.4(a)）。
//
// ⚠️ 本支**不解密載具**：carrier_summary 逐列解密 ＝ 每列一次額外 DB 往返
// （WithDecryptedRecipient 自己會再查一次那一列），而 B09 的六欄裡沒有載具
// （開立日期／發票號碼／開立人／類型／金額／狀態，58A §B09）。要看載具去詳情頁。
func (u *Query) ListBuyerDocuments(ctx context.Context, in BuyerDocumentsInput) (DocumentListPage, error) {
	page, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.BuyerDocumentQuery{
		BuyerID:      in.BuyerID,
		Dates:        in.Dates,
		Status:       in.Status,
		DocumentKind: in.DocumentKind,
		Page:         page,
	}
	rows, err := u.documents.ListForBuyer(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	total, err := u.documents.CountForBuyer(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	out := DocumentListPage{Records: make([]DocumentListItem, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for _, d := range rows {
		out.Records = append(out.Records, DocumentListItem{Document: d})
	}
	return out, nil
}

// GetBuyerDocument 回買家自己的一份文件（端點 69，document-scoped）。
//
// ⚠️ 查不到**不得**改用 order 述詞再查一次（§8.1.4(a)「兩套述詞不得互相回退」）：
// 那會讓「訂單是我的但發票的買受人不是我」（ADM 手動改過買受人）變成可讀。
func (u *Query) GetBuyerDocument(ctx context.Context, buyerID, documentID int64) (DocumentDetailView, error) {
	doc, err := u.documents.GetForBuyerByID(ctx, documentID, buyerID)
	if err != nil {
		return DocumentDetailView{}, err
	}
	return u.documentDetail(ctx, doc, buyerCarrierView)
}

// GetBuyerOrderDocument 回買家在某張訂單底下的一份文件（端點 9，order-scoped）。
//
// order_no 不匹配走**同一個** 9301：不得為它另開一個碼，那等於在回應裡放一個
// 可以區分「不是你的」與「不存在」的旗標（§8.1.4(c)）。
func (u *Query) GetBuyerOrderDocument(ctx context.Context, buyerID int64, orderNo string, documentID int64) (DocumentDetailView, error) {
	doc, err := u.documents.GetForBuyerByID(ctx, documentID, buyerID)
	if err != nil {
		return DocumentDetailView{}, err
	}
	if orderNo != "" && !documentBelongsToOrder(doc, orderNo) {
		return DocumentDetailView{}, documentNotFound()
	}
	return u.documentDetail(ctx, doc, buyerCarrierView)
}

func documentBelongsToOrder(doc domain.Document, orderNo string) bool {
	return doc.SourceType == sourceTypeOrder && derefString(doc.SourceRef) == orderNo
}

func documentNotFound() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceDocumentNotFound, bizcode.Message(bizcode.InvoiceDocumentNotFound))
}


type carrierView int

const (
	buyerCarrierView carrierView = iota
	sellerCarrierView
)

func (u *Query) documentDetail(ctx context.Context, doc domain.Document, view carrierView) (DocumentDetailView, error) {
	out := DocumentDetailView{Document: doc}

	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return DocumentDetailView{}, err
	}
	out.Issuer = issuer

	if out.Lines, err = u.documents.ListLines(ctx, doc.ID); err != nil {
		return DocumentDetailView{}, err
	}
	if out.Timeline, err = u.documentTimeline(ctx, doc.ID); err != nil {
		return DocumentDetailView{}, err
	}

	carrier := ""
	if doc.CarrierId1Ciphertext != nil {
		if err := u.documents.WithDecryptedRecipient(ctx, doc.ID, func(s domain.RecipientSecrets) error {
			switch view {
			case buyerCarrierView:
				carrier = s.CarrierID1
			case sellerCarrierView:
				carrier = MaskCarrierID(derefString(doc.CarrierType), s.CarrierID1)
			}
			return nil
		}); err != nil {
			return DocumentDetailView{}, err
		}
	}
	out.CarrierSummary, out.CarrierType = carrierPresentationOf(
		doc.DocumentKind, derefString(doc.CarrierType), carrier, doc.RecipientTaxID)
	return out, nil
}

func carrierPresentationOf(documentKind, amegoCarrierType, carrier, recipientTaxID string) (summary, carrierType string) {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return "", ""
	}
	if carrier != "" {
		ct := ""
		if kind, ok := domain.CarrierTypeFromAmego(amegoCarrierType); ok {
			ct = kind.Value
		}
		return carrier, ct
	}
	if recipientTaxID == domain.B2CBuyerIdentifier {
		return "", ""
	}
	return recipientTaxID, ""
}

func (u *Query) documentTimeline(ctx context.Context, documentID int64) ([]TimelineItem, error) {
	out := make([]TimelineItem, 0, 8)

	docHistory, err := u.audit.ListStatusHistory(ctx, subjectDocument, documentID)
	if err != nil {
		return nil, err
	}
	for _, h := range docHistory {
		if ev, ok := timelineEventOf(subjectDocument, h.ToStatus); ok {
			out = append(out, TimelineItem{OccurredAt: h.OccurredAt, Event: ev, Note: derefString(h.ReasonDetail)})
		}
	}

	adjustments, err := u.adjustments.ListByDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	for _, adj := range adjustments {
		if adj.AdjustmentType != adjustmentTypeAllowance || adj.Status != adjustmentStatusCompleted {
			continue
		}
		history, err := u.audit.ListStatusHistory(ctx, subjectAdjustment, adj.ID)
		if err != nil {
			return nil, err
		}
		for _, h := range history {
			if h.ToStatus != adjustmentStatusCompleted {
				continue
			}
			out = append(out, TimelineItem{
				OccurredAt: h.OccurredAt,
				Event:      codemeta.InvoiceDocumentTimelineEventAllowanceApplied.Value,
				Note:       derefString(h.ReasonDetail),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].OccurredAt.Before(out[j].OccurredAt) })
	return out, nil
}

const subjectAdjustment = "ADJUSTMENT"


const complaintOpenDelay = 24 * time.Hour

const orderComplaintScanLimit int32 = 20

// OrderInvoiceView 是端點 8 的投影。
type OrderInvoiceView struct {
	OrderNo string
	// Mode／State／StateNotice 取自**最新一份主文件**（作廢重開鏈取最後一份）。
	Mode        string
	State       string
	StateNotice string
	// Documents 是 repeated：一張訂單在 VOID_AND_REISSUE 之後會有多份文件
	// （舊的 SUPERSEDED 帶 superseded_by_id、新的帶 original_document_id）。
	Documents []DocumentSummaryView

	SnapshotVersion     int32
	NotifiedEmailMasked string

	CanRequestCorrection bool
	CanFileComplaint     bool
	ComplaintOpensAt     *time.Time
	ExistingComplaintID  int64

	// LatestCorrectionID 是**最近一次**修改申請（不分狀態），0 ＝ 從未申請過。
	//
	// 🔴 它是端點 12 的唯一入口：買家端沒有別的管道拿得到 correction_id。
	// 少了它，送出申請之後重新載入頁面就再也看不到進度與婉拒理由
	// （2026-09-05 第 6 輪稽核：網頁的端點 12 client 因此零呼叫端）。
	// **不分狀態**取最新一筆——被婉拒的那一筆才是買家最需要看到的。
	LatestCorrectionID int64
}

// DocumentSummaryView 是 OrderInvoiceView.Documents 的一列（proto InvoiceDocumentSummary）。
//
// CarrierSummary 在這裡是**完整值**：買家是資料主體（§8.1.3(b)）。
type DocumentSummaryView struct {
	Document       domain.Document
	CarrierSummary string
	// CarrierType 見 DocumentDetailView.CarrierType。
	CarrierType string
}

// GetOrderInvoice 回一張訂單的發票現況（B04，端點 8）。
//
// Tier A：ListOrderDocumentsForBuyer 的述詞是 `source_type='ORDER' AND source_ref
// AND recipient_type='BUYER' AND recipient_ref_id`。別人的訂單得到空清單。
//
// 🔴 **查無文件不是錯誤**：訂單存在但還沒付款時本來就沒有 invoice_documents 列，
// 那一格的正確答案是 state=AWAITING_PAYMENT（§6.1.3 對照表第一列），不是 404。
func (u *Query) GetOrderInvoice(ctx context.Context, buyerID int64, orderNo string) (OrderInvoiceView, error) {
	docs, err := u.documents.ListOrderDocumentsForBuyer(ctx, orderNo, buyerID)
	if err != nil {
		return OrderInvoiceView{}, err
	}

	out := OrderInvoiceView{OrderNo: orderNo, Documents: make([]DocumentSummaryView, 0, len(docs))}
	if len(docs) == 0 {
		cancelled, err := u.cancelledOrders.IsCancelledForBuyer(ctx, orderNo, buyerID)
		if err != nil {
			return OrderInvoiceView{}, err
		}
		state := orderInvoiceStateAwaitingPayment
		if cancelled {
			state = orderInvoiceStateOrderCancelled
		}
		out.State = state
		out.StateNotice = StateNoticeFor("", state)
		return out, nil
	}

	latest := docs[0]
	out.Mode = latest.FulfillmentMode
	out.SnapshotVersion = latest.SnapshotVersion

	for _, d := range docs {
		carrier, err := u.revealCarrier(ctx, d, buyerCarrierView)
		if err != nil {
			return OrderInvoiceView{}, err
		}
		summary, carrierType := carrierPresentationOf(
			d.DocumentKind, derefString(d.CarrierType), carrier, d.RecipientTaxID)
		out.Documents = append(out.Documents, DocumentSummaryView{
			Document:       d,
			CarrierSummary: summary,
			CarrierType:    carrierType,
		})
	}

	hasAllowance, err := u.hasCompletedAllowance(ctx, latest.ID)
	if err != nil {
		return OrderInvoiceView{}, err
	}
	out.State = projectOrderInvoiceState(latest, hasAllowance)
	out.StateNotice = StateNoticeFor(latest.DocumentKind, out.State)

	if latest.RecipientEmailCiphertext != nil {
		if err := u.documents.WithDecryptedRecipient(ctx, latest.ID, func(s domain.RecipientSecrets) error {
			out.NotifiedEmailMasked = MaskEmail(s.Email)
			return nil
		}); err != nil {
			return OrderInvoiceView{}, err
		}
	}

	out.CanRequestCorrection, err = u.canRequestCorrection(ctx, latest, hasAllowance)
	if err != nil {
		return OrderInvoiceView{}, err
	}

	opensAt := complaintOpensAt(docs)
	out.ComplaintOpensAt = &opensAt

	existing, err := u.openComplaintOfOrder(ctx, buyerID, orderNo)
	if err != nil {
		return OrderInvoiceView{}, err
	}
	out.ExistingComplaintID = existing
	out.CanFileComplaint = existing == 0 && !u.now().Before(opensAt)

	if corr, err := u.corrections.GetLatestByOrder(ctx, orderNo, buyerID); err == nil {
		out.LatestCorrectionID = corr.ID
	} else if !pkgerrors.IsCode(err, bizcode.InvoiceCorrectionNotAllowed) {
		return OrderInvoiceView{}, err
	}
	return out, nil
}

func (u *Query) revealCarrier(ctx context.Context, doc domain.Document, view carrierView) (string, error) {
	if doc.CarrierId1Ciphertext == nil {
		return "", nil
	}
	var out string
	err := u.documents.WithDecryptedRecipient(ctx, doc.ID, func(s domain.RecipientSecrets) error {
		switch view {
		case buyerCarrierView:
			out = s.CarrierID1
		case sellerCarrierView:
			out = MaskCarrierID(derefString(doc.CarrierType), s.CarrierID1)
		}
		return nil
	})
	return out, err
}

func (u *Query) hasCompletedAllowance(ctx context.Context, documentID int64) (bool, error) {
	adjustments, err := u.adjustments.ListByDocument(ctx, documentID)
	if err != nil {
		return false, err
	}
	for _, adj := range adjustments {
		if adj.AdjustmentType == adjustmentTypeAllowance && adj.Status == adjustmentStatusCompleted {
			return true, nil
		}
	}
	return false, nil
}

func projectOrderInvoiceState(doc domain.Document, hasAllowance bool) string {
	if hasAllowance && doc.Status == codemeta.InvoiceDocumentStatusIssued.Value {
		return orderInvoiceStateAllowanceApplied
	}
	if state, ok := documentStatusToBuyerState[doc.Status]; ok {
		return state
	}
	return orderInvoiceStateProcessing
}

func (u *Query) canRequestCorrection(ctx context.Context, doc domain.Document, hasAllowance bool) (bool, error) {
	if doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return false, nil
	}
	switch doc.Status {
	case codemeta.InvoiceDocumentStatusPending.Value:
		ops, err := u.operations.ListByDocument(ctx, doc.ID)
		if err != nil {
			return false, err
		}
		for _, op := range ops {
			if op.Status == codemeta.InvoiceOperationStatusAwaitingTrigger.Value ||
				op.Status == codemeta.InvoiceOperationStatusReady.Value {
				return true, nil
			}
		}
		return false, nil
	case codemeta.InvoiceDocumentStatusIssued.Value:
		if hasAllowance || doc.TaxPeriodCode == nil {
			return false, nil
		}
		now := u.now()
		within, err := domain.WithinVoidWindow(*doc.TaxPeriodCode, now)
		if err != nil {
			u.logger().Warn("invoice.query.void_window.undecidable")
			return false, nil
		}
		if !within {
			return false, nil
		}
		reported, err := domain.ReportedStatus(*doc.TaxPeriodCode, now)
		if err != nil {
			u.logger().Warn("invoice.query.reported_status.undecidable")
			return false, nil
		}
		return reported == domain.ReportedStatusNotReported, nil
	default:
		return false, nil
	}
}

func (u *Query) openComplaintOfOrder(ctx context.Context, buyerID int64, orderNo string) (int64, error) {
	c, found, err := findOpenComplaint(ctx, u.complaints, buyerID, orderNo)
	if err != nil || !found {
		return 0, err
	}
	return c.ID, nil
}


// GetBuyerCorrection 回買家自己的一件修改申請（端點 12）。
//
// Tier A **三段述詞全部在 SQL**（§8.1.4(b)）：申請人 ＝ 當前買家、該 document 的
// 買受人 ＝ 當前買家、該 document 屬於 request 帶進來的那張訂單。
// 只驗其一的實作會在 ADM 事後改過買受人之後把資料交出去。
func (u *Query) GetBuyerCorrection(ctx context.Context, buyerID int64, orderNo string, correctionID int64) (domain.CorrectionRequest, error) {
	return u.corrections.GetForBuyer(ctx, correctionID, buyerID, orderNo)
}

// ListBuyerOrderComplaints 回這張訂單底下這位買家的申訴案（端點 13 的 GET，B07）。
//
// 它是「拿到 9310 之後要看得到既有案件」的落點。清單端點的 Tier A 正確形狀是
// 空 records，不是 404。
func (u *Query) ListBuyerOrderComplaints(ctx context.Context, buyerID int64, orderNo string, page, pageSize int32) (ComplaintPage, error) {
	sqlPage, normPage, normSize := pageOf(page, pageSize)
	q := domain.BuyerComplaintQuery{BuyerID: buyerID, OrderNo: orderNo, Page: sqlPage}
	rows, err := u.complaints.ListForBuyerOrder(ctx, q)
	if err != nil {
		return ComplaintPage{}, err
	}
	total, err := u.complaints.CountForBuyerOrder(ctx, q)
	if err != nil {
		return ComplaintPage{}, err
	}
	views, err := u.complaintViews(ctx, rows)
	if err != nil {
		return ComplaintPage{}, err
	}
	return ComplaintPage{Records: views, Total: clampCount(total), Page: normPage, PageSize: normSize}, nil
}

func (u *Query) complaintViews(ctx context.Context, rows []domain.Complaint) ([]ComplaintView, error) {
	now := u.now()
	from, to := now, now
	for _, c := range rows {
		from = minTime(from, c.SlaDueAt)
		to = maxTime(to, c.SlaDueAt)
	}
	cal, err := loadBusinessCalendar(ctx, u.businessDay, from, to)
	if err != nil {
		return nil, err
	}

	out := make([]ComplaintView, 0, len(rows))
	kinds := make(map[int64]string, len(rows))
	for _, c := range rows {
		events, err := u.complaints.ListEvents(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		v := viewWith(c, events, cal, now)
		v.ExpectedDocKind = u.expectedDocKind(ctx, c.DocumentID, kinds)
		out = append(out, v)
	}
	return out, nil
}


// SellerDocumentsInput 是端點 30 的入參。
type SellerDocumentsInput struct {
	StoreID      int64
	Dates        domain.DateRange
	Mode         string
	Status       string
	OrderNo      string
	AbnormalOnly bool
	Page         int32
	PageSize     int32
}

// ListSellerDocuments 回賣家自己開出的發票與收據（S06）。
//
// Tier A：歸屬鍵是 invoice_issuers.issuer_ref_id（invoice_documents 沒有 store_id 欄）
// ⇒ 一定是 join，述詞在 SQL 裡。
func (u *Query) ListSellerDocuments(ctx context.Context, in SellerDocumentsInput) (DocumentListPage, error) {
	page, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.SellerDocumentQuery{
		StoreID:      in.StoreID,
		Dates:        in.Dates,
		Mode:         in.Mode,
		Status:       in.Status,
		OrderNo:      in.OrderNo,
		AbnormalOnly: in.AbnormalOnly,
		AbnormalStatuses: abnormalDocumentStatuses,
		Page:             page,
	}
	rows, err := u.documents.ListForSeller(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	total, err := u.documents.CountForSeller(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	out := DocumentListPage{Records: make([]DocumentListItem, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for _, row := range rows {
		out.Records = append(out.Records, DocumentListItem{
			Document:          row.Document,
			LatestOperationID: row.LatestOperationID,
		})
	}
	return out, nil
}

// SellerDocumentDetailView 是端點 31 的投影。
type SellerDocumentDetailView struct {
	Detail DocumentDetailView
	// NextResendAvailableAt 是端點 32 重寄窗口的倒數終點；nil ＝ 現在就可以重寄。
	//
	// 🔴 它是前端唯一的來源：端點 32 的 429 回應**帶不出 data**（§6.1.10），
	// 所以按鈕的停用與倒數只能在進畫面時算好。少了它，按鈕永遠可按，而撞 429 時
	// 使用者拿不到「還要等多久」。
	// ⚠️ 列表端點 30 刻意不帶此欄（每列都要算一次＝N+1）。
	NextResendAvailableAt *time.Time
}

// GetSellerDocument 回賣家自己的一份文件明細（S07，端點 31）。
//
// 🔴 **這是遮罩函式的第一條生產路徑**：賣家不是買家個資的資料主體，載具識別碼
// 依 §8.1.3(b) 必須遮罩。DB 存的是密文 ⇒ 本支解密後**立刻**遮罩，明碼不離開
// WithDecryptedRecipient 的 callback。
//
// ⚠️ proto 的 SellerInvoiceDocumentDetail 沒有 email／隨機碼欄位，所以 §8.1.3(c)
// 的範例 JSON 裡的 `randomNumberMasked`／`notifyEmailMasked` 在這一支填不出來。
// 那是契約缺口不是遮罩缺口，登記在交付回報。
func (u *Query) GetSellerDocument(ctx context.Context, storeID, documentID int64) (SellerDocumentDetailView, error) {
	item, err := u.documents.GetForSeller(ctx, documentID, storeID)
	if err != nil {
		return SellerDocumentDetailView{}, err
	}
	detail, err := u.documentDetail(ctx, item.Document, sellerCarrierView)
	if err != nil {
		return SellerDocumentDetailView{}, err
	}
	detail.LatestOperationID = item.LatestOperationID
	return SellerDocumentDetailView{
		Detail:                detail,
		NextResendAvailableAt: NextResendAvailableAt(item.Document, u.now()),
	}, nil
}


// GetSellerJob 回賣家自己的一件開票作業（S08，端點 33）。
//
// 🔴 擁有者判定一律經 invoice_documents join（§8.1.4 末段）：invoice_operations
// 完全沒有 store_id 也沒有 order_no，兩者都掛在 document 上。
func (u *Query) GetSellerJob(ctx context.Context, storeID, jobID int64) (domain.OperationView, error) {
	return u.operations.GetViewForSeller(ctx, jobID, storeID)
}

var externalIssuanceStatuses = []string{
	codemeta.InvoiceOperationStatusManualProcessAllowed.Value,
	codemeta.InvoiceOperationStatusNotApplicable.Value,
	codemeta.InvoiceOperationStatusResolvedExternally.Value,
}

// ListOrderExternalIssuances 回這張訂單可回報／已回報的自行開立案件（62R）。
//
// 回清單不回單筆：一張訂單可以同時有兩筆 operation 落到 MANUAL_PROCESS_ALLOWED
// （ISSUE 結果不明、其後的 VOID 也結果不明），而兩筆的法定期限不同。靜默挑一筆的
// 後果是賣家回報了 A 卻以為 B 也結案了。
//
// 查無／越權／訂單不存在一律空 records，不回 9340（清單端點上的 Tier A 正確形狀）。
func (u *Query) ListOrderExternalIssuances(ctx context.Context, storeID int64, orderNo string, page, pageSize int32) (JobPage, error) {
	sqlPage, normPage, normSize := pageOf(page, pageSize)
	rows, err := u.operations.ListOrderExternalIssuances(ctx, storeID, orderNo, externalIssuanceStatuses, sqlPage)
	if err != nil {
		return JobPage{}, err
	}
	total, err := u.operations.CountOrderExternalIssuances(ctx, storeID, orderNo, externalIssuanceStatuses)
	if err != nil {
		return JobPage{}, err
	}
	return JobPage{Records: rows, Total: clampCount(total), Page: normPage, PageSize: normSize}, nil
}


// SellerComplaintsInput 是端點 34 的入參。
type SellerComplaintsInput struct {
	StoreID int64
	// View 是**視圖名**（AWAITING_SELLER／SELLER_RESPONDED／CLOSED），不是 DB 狀態值。
	// 空字串 ＝ 不篩。
	View string
	// SortByDue=true ⇒ due_at_asc（預設，最接近逾期者優先）。
	SortByDue bool
	Page      int32
	PageSize  int32
}

// ErrUnknownView 是視圖名不在對照表內。
//
// 它是 InvalidArgument 不是空結果：把未知視圖靜默當成「不篩」會讓打錯字的前端
// 拿到全部案件，而把它當成空集合會讓人以為「這個分頁今天沒有案件」。
var ErrUnknownView = errors.New("invoice: unknown view name")

// ListSellerComplaints 回這位賣家收到的申訴（S09，端點 34）。
func (u *Query) ListSellerComplaints(ctx context.Context, in SellerComplaintsInput) (ComplaintPage, error) {
	statuses, err := expandView(sellerComplaintViews, in.View)
	if err != nil {
		return ComplaintPage{}, err
	}
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.SellerComplaintQuery{
		StoreID:       in.StoreID,
		StatusFilters: statuses,
		SortByDue:     in.SortByDue,
		Page:          sqlPage,
	}
	rows, err := u.complaints.ListForSeller(ctx, q)
	if err != nil {
		return ComplaintPage{}, err
	}
	total, err := u.complaints.CountForSeller(ctx, q)
	if err != nil {
		return ComplaintPage{}, err
	}
	views, err := u.complaintViews(ctx, rows)
	if err != nil {
		return ComplaintPage{}, err
	}
	return ComplaintPage{Records: views, Total: clampCount(total), Page: normPage, PageSize: normSize}, nil
}

// GetSellerComplaint 回這位賣家收到的一件申訴（S10，端點 35）。
//
// Tier A：擁有者述詞在 SQL（invoice_issuers.issuer_ref_id）⇒ 越權與不存在皆 9309。
func (u *Query) GetSellerComplaint(ctx context.Context, storeID, complaintID int64) (ComplaintView, error) {
	c, err := u.complaints.GetForSeller(ctx, complaintID, storeID)
	if err != nil {
		return ComplaintView{}, err
	}
	return u.complaintView(ctx, c)
}

func expandView[T any](table map[string]T, view string) (T, error) {
	var zero T
	if view == "" {
		return zero, nil
	}
	v, ok := table[view]
	if !ok {
		return zero, fmt.Errorf("%w: %q", ErrUnknownView, view)
	}
	return v, nil
}


// ReceivedDocumentsInput 是端點 72 的入參。
type ReceivedDocumentsInput struct {
	StoreID  int64
	Dates    domain.DateRange
	Status   string
	Page     int32
	PageSize int32
}

// ListSellerReceivedDocuments 回別人開給這位賣家的發票（S12 進項）。
//
// 🔴 它與端點 30 是**兩個資料集，不是同一支加 flag**：銷項可作廢可折讓、
// 進項是唯讀報稅憑證，法律角色不同。授權述詞也不同
// （recipient_type='SELLER' 對 issuer_ref_id）。
func (u *Query) ListSellerReceivedDocuments(ctx context.Context, in ReceivedDocumentsInput) (DocumentListPage, error) {
	page, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.ReceivedDocumentQuery{
		StoreID: in.StoreID,
		Dates:   in.Dates,
		Status:  in.Status,
		Page:    page,
	}
	rows, err := u.documents.ListReceivedForSeller(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	total, err := u.documents.CountReceivedForSeller(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	out := DocumentListPage{Records: make([]DocumentListItem, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for _, row := range rows {
		out.Records = append(out.Records, DocumentListItem{
			Document:      row.Document,
			RecipientNote: recipientNoteOf(row),
		})
	}
	return out, nil
}

func recipientNoteOf(row domain.ReceivedDocumentItem) string {
	if row.FirstLineDescription == "" {
		return ""
	}
	if row.LineCount > 1 {
		return fmt.Sprintf("%s等 %d 項", row.FirstLineDescription, row.LineCount)
	}
	return row.FirstLineDescription
}


const taxReviewStatusNotStarted = "NOT_STARTED"

const overviewProbeAmount int64 = 1

// SellerCapabilityView 是「這位賣家目前的開票能力」——端點 16 與內部薄查詢
// GetSellerInvoiceCapabilitySimple 共用的那一半。
//
// 🔴 兩支共用同一個推導是刻意的：各算一次的症狀是「seller-service 說這位賣家可以
// 自動開票、發票模組說不行」，而兩邊都不會報錯（同 checkout 的 ResolveMode 一律經
// domain.ResolveFulfillmentMode 那條理由）。
//
// IssuerID ＝ 0 表示這位賣家還沒有 invoice_issuers 列（從未進過發票流程），
// 呼叫端據此跳過所有需要 issuer_id 的計數查詢。
type SellerCapabilityView struct {
	IssuerID int64

	TaxReviewStatus string
	InvoiceMode     string
	ProviderStatus  string

	DeadlineAt          *time.Time
	ExtensionCount      int32
	CanRequestExtension bool
	SelfIssueEffective  bool
	IntakeLimit         CheckoutIntakeLimit
}

// CanIssueAutomatically 回報平台現在會不會替這位賣家自動開票。
//
// 🔴 唯一定義是履約模式落在 AMEGO_AUTO —— 那已經是 §7.1.3 決策樹（賣家資格 ＋
// provider 狀態 ＋ 切點）的結論。另湊一組 if（例如「review_status 是 APPROVED_*
// 就算可以」）會漏掉連線被 SUSPENDED／REVOKED 的那一格，而那一格每一筆都開不出來。
func (v SellerCapabilityView) CanIssueAutomatically() bool {
	return v.InvoiceMode == codemeta.InvoiceFulfillmentModeAmegoAuto.Value
}

// SellerOverviewView 是端點 16 的投影：能力那一半 ＋ 三個計數 ＋ 最近五份文件。
type SellerOverviewView struct {
	Capability SellerCapabilityView

	PendingDocumentCount  int64
	OpenComplaintCount    int64
	OverdueComplaintCount int64

	RecentDocuments []DocumentSummaryView
}

const sellerOverviewRecentLimit int32 = 5

// GetSellerCapability 回這位賣家的開票能力（端點 16 的一半、內部薄查詢的全部）。
//
// 🔴 **沒有任何業務碼**：賣家還沒有 issuer／稅務資格／provider 帳號都是正常狀態
// （上線當天的全體），一律回三態投影值。回 404 的話上線當天每一位賣家的側欄入口
// 都是死的，而 seller-service 那一側會把「還沒設定」讀成「查詢失敗」。
func (u *Query) GetSellerCapability(ctx context.Context, storeID int64) (SellerCapabilityView, error) {
	out := SellerCapabilityView{
		TaxReviewStatus: taxReviewStatusNotStarted,
		ProviderStatus:  projectProviderStatus(false, ""),
	}

	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
			out.InvoiceMode = u.resolveOverviewMode(domain.SellerTaxProfile{}, domain.ProviderAccount{})
			return out, nil
		}
		return SellerCapabilityView{}, err
	}
	out.IssuerID = issuer.ID

	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	switch {
	case err == nil:
		out.TaxReviewStatus = profile.ReviewStatus
		out.DeadlineAt = profile.PendingValidUntil
		out.ExtensionCount = int32(profile.ExtensionCount)
		out.CanRequestExtension = CanRequestTaxExtension(profile)
		out.SelfIssueEffective = profile.SelfIssueEffective()
		out.IntakeLimit = intakeLimitOf(profile)
	case isBizCode(err, bizcode.SellerTaxProfileNotFound):
	default:
		return SellerCapabilityView{}, err
	}

	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
	switch {
	case err == nil:
		out.ProviderStatus = projectProviderStatus(true, account.Status)
	case isBizCode(err, bizcode.InvoiceProviderAccountNotFound):
	default:
		return SellerCapabilityView{}, err
	}

	out.InvoiceMode = u.resolveOverviewMode(profile, account)
	return out, nil
}

func (u *Query) resolveOverviewMode(profile domain.SellerTaxProfile, account domain.ProviderAccount) string {
	return domain.ResolveFulfillmentMode(domain.FulfillmentInput{
		TotalActualAmount:   overviewProbeAmount,
		SellerReviewStatus:  profile.ReviewStatus,
		ProviderStatus:      account.Status,
		AutomationCutoverAt: account.AutomationCutoverAt,
		InvoiceDate:         u.now(),
	}).FulfillmentMode
}

// GetSellerInvoiceOverview 回賣家的發票與稅籍總覽（S01，端點 16）。
//
// 它是「能力 ＋ 三個計數 ＋ 最近五份文件」。三個計數走**單一 SELECT**（OverviewRepository）：
// 拆成三次往返會讓每個計數落在不同的時間點，總覽頁出現「待處理 3 筆、但清單只有 2 筆」
// 這種間歇性自相矛盾，而它沒有任何測試抓得到。
func (u *Query) GetSellerInvoiceOverview(ctx context.Context, storeID int64) (SellerOverviewView, error) {
	capability, err := u.GetSellerCapability(ctx, storeID)
	if err != nil {
		return SellerOverviewView{}, err
	}
	out := SellerOverviewView{Capability: capability, RecentDocuments: []DocumentSummaryView{}}
	if capability.IssuerID == 0 {
		return out, nil
	}

	counts, err := u.overview.SellerCounts(ctx, domain.SellerOverviewCountsQuery{
		IssuerID:                 capability.IssuerID,
		PendingDocumentStatuses:  pendingDocumentStatuses,
		OpenComplaintStatuses:    openComplaintStatuses,
		OverdueComplaintStatuses: overdueComplaintStatuses,
		Now:                      u.now(),
	})
	if err != nil {
		return SellerOverviewView{}, err
	}
	out.PendingDocumentCount = counts.PendingDocumentCount
	out.OpenComplaintCount = counts.OpenComplaintCount
	out.OverdueComplaintCount = counts.OverdueComplaintCount

	recent, err := u.documents.ListRecentForIssuer(ctx, capability.IssuerID, sellerOverviewRecentLimit)
	if err != nil {
		return SellerOverviewView{}, err
	}
	for _, d := range recent {
		summary, carrierType := carrierPresentationOf(d.DocumentKind, "", "", d.RecipientTaxID)
		out.RecentDocuments = append(out.RecentDocuments, DocumentSummaryView{
			Document:       d,
			CarrierSummary: summary,
			CarrierType:    carrierType,
		})
	}
	return out, nil
}


// AdminDocumentsInput 是端點 47 的入參。
//
// StoreID 是 *int64：nil ＝ 全平台。它在 ADM 面是**篩選**不是授權——這正是它不能
// 與端點 30 共用一支查詢的理由（後者的 store_id 是必填的授權述詞）。
type AdminDocumentsInput struct {
	StoreID  *int64
	OrderNo  string
	Status   string
	Mode     string
	Dates    domain.DateRange
	Page     int32
	PageSize int32
}

// ListDocumentsForAdmin 回全平台的發票文件（A12，端點 47）。
func (u *Query) ListDocumentsForAdmin(ctx context.Context, in AdminDocumentsInput) (DocumentListPage, error) {
	page, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.AdminDocumentQuery{
		StoreID: in.StoreID,
		OrderNo: in.OrderNo,
		Status:  in.Status,
		Mode:    in.Mode,
		Dates:   in.Dates,
		Page:    page,
	}
	rows, err := u.documents.ListForAdmin(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	total, err := u.documents.CountForAdmin(ctx, q)
	if err != nil {
		return DocumentListPage{}, err
	}
	out := DocumentListPage{Records: make([]DocumentListItem, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for _, row := range rows {
		item := DocumentListItem{Document: row.Document, LatestOperationID: row.LatestOperationID}
		if row.IssuerType == codemeta.InvoiceIssuerTypeSeller.Value {
			item.IssuerRefID = row.IssuerRefID
		}
		out.Records = append(out.Records, item)
	}
	return out, nil
}


// AdminJobsInput 是端點 50 的入參。
type AdminJobsInput struct {
	// Tab 是六個分頁名之一，空字串 ＝ 不篩。
	Tab           string
	StoreID       *int64
	OrderNo       string
	OperationType string
	// UploadStatus 是四個投影值之一（PENDING／UPLOADED／NOT_FOUND／FAILED），
	// 與 Tab **正交**、不是第七個分頁。空字串 ＝ 不篩。
	UploadStatus string
	ManualReason string
	Page         int32
	PageSize     int32
}

// ListJobsForAdmin 回全平台的開票作業（A05，端點 50）。
func (u *Query) ListJobsForAdmin(ctx context.Context, in AdminJobsInput) (JobPage, error) {
	statuses, err := expandView(adminJobTabs, in.Tab)
	if err != nil {
		return JobPage{}, err
	}
	upload, err := expandView(uploadStatusProjections, in.UploadStatus)
	if err != nil {
		return JobPage{}, err
	}
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.AdminOperationQuery{
		StatusFilters: statuses,
		IncludeManualDocuments: in.Tab == adminJobNeedsManualTab,
		ManualDocumentStatuses: manualDocumentStatuses,
		ExcludeResolvedDocuments: in.Tab == adminJobNeedsManualTab,
		ResolvedDocumentStatuses: resolvedDocumentStatuses,
		StoreID:                  in.StoreID,
		OrderNo:                  in.OrderNo,
		OperationType:            in.OperationType,
		HasUploadStatus:          in.UploadStatus != "",
		UploadStatusCodes:        upload.Codes,
		UploadStatusIncludeNull:  upload.IncludeNull,
		ManualReason:             in.ManualReason,
		Page:                     sqlPage,
	}
	rows, err := u.operations.ListViewsForAdmin(ctx, q)
	if err != nil {
		return JobPage{}, err
	}
	total, err := u.operations.CountViewsForAdmin(ctx, q)
	if err != nil {
		return JobPage{}, err
	}
	return JobPage{Records: rows, Total: clampCount(total), Page: normPage, PageSize: normSize}, nil
}


// AdminComplaintsInput 是端點 58 的入參。
type AdminComplaintsInput struct {
	// Tab 是五個 tab 名之一，空字串 ＝ 不篩。**五個 tab 不互斥**（OVERDUE 是橫切視圖）。
	Tab       string
	StoreID   *int64
	OrderNo   string
	SortByDue bool
	Page      int32
	PageSize  int32
}

const violationWindowDays = 90

// ListComplaintsForAdmin 回全平台的申訴案（A07，端點 58）。
func (u *Query) ListComplaintsForAdmin(ctx context.Context, in AdminComplaintsInput) (AdminComplaintPage, error) {
	tab, err := expandView(adminComplaintTabs, in.Tab)
	if err != nil {
		return AdminComplaintPage{}, err
	}
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	now := u.now()
	q := domain.AdminComplaintQuery{
		StatusFilters:      tab.Statuses,
		IncludeDueOverdue:  tab.IncludeDueOverdue,
		DueOverdueStatuses: tab.DueOverdue,
		Now:                now,
		StoreID:            in.StoreID,
		OrderNo:            in.OrderNo,
		SortByDue:          in.SortByDue,
		ViolationStatus:      violationComplaintStatus,
		ViolationWindowStart: now.AddDate(0, 0, -violationWindowDays),
		Page:                 sqlPage,
	}
	rows, err := u.complaints.ListForAdmin(ctx, q)
	if err != nil {
		return AdminComplaintPage{}, err
	}
	total, err := u.complaints.CountForAdmin(ctx, q)
	if err != nil {
		return AdminComplaintPage{}, err
	}
	plain := make([]domain.Complaint, 0, len(rows))
	for _, row := range rows {
		plain = append(plain, row.Complaint)
	}
	views, err := u.complaintViews(ctx, plain)
	if err != nil {
		return AdminComplaintPage{}, err
	}
	out := AdminComplaintPage{Records: make([]AdminComplaintView, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for i, row := range rows {
		out.Records = append(out.Records, AdminComplaintView{
			Complaint:              views[i],
			StoreID:                row.IssuerRefID,
			ViolationCountInWindow: row.ViolationCountInWindow,
		})
	}
	return out, nil
}


// AdminTaxReviewsInput 是端點 38 的入參。
type AdminTaxReviewsInput struct {
	// Filter 是六個 filter 名之一，空字串 ＝ 不篩。**六個不互斥**（交集視圖）。
	Filter   string
	Keyword  string
	Page     int32
	PageSize int32
}

const nearDeadlineWindow = 7 * 24 * time.Hour

// TaxReviewPage 是端點 38 的一頁。
type TaxReviewPage struct {
	Records  []TaxReviewItemView
	Total    int32
	Page     int32
	PageSize int32
}

// TaxReviewItemView 是 A02 的一列。
//
// RemainingDays 在這一層算而不是在 mapper：它需要「現在幾點」，而時鐘是注入的
// （WithClock）。把它留給 mapper 就等於讓一個拿不到注入時鐘的地方做時間運算，
// 那種測試只能靠 sleep 或本機時區，兩者都不穩定（CLAUDE.md §21）。
//
// nil ＝ **沒有期限**（非待審狀態），不是 0 —— 0 會被前端讀成「今天到期」。
type TaxReviewItemView struct {
	Item          domain.AdminTaxReviewItem
	RemainingDays *int64
}

// ListTaxReviewsForAdmin 回稅務身分審核佇列（A02，端點 38）。
func (u *Query) ListTaxReviewsForAdmin(ctx context.Context, in AdminTaxReviewsInput) (TaxReviewPage, error) {
	filter, err := expandView(adminTaxReviewFilters, in.Filter)
	if err != nil {
		return TaxReviewPage{}, err
	}
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.AdminTaxReviewQuery{
		StatusFilters:         filter.Statuses,
		NearDeadlineOnly:      filter.NearDeadline,
		NearDeadlineBefore:    u.now().Add(nearDeadlineWindow),
		ExtendedOnly:          filter.Extended,
		Keyword:               in.Keyword,
		OpenComplaintStatuses: openComplaintStatuses,
		Page:                  sqlPage,
	}
	rows, err := u.profiles.ListReviewsForAdmin(ctx, q)
	if err != nil {
		return TaxReviewPage{}, err
	}
	total, err := u.profiles.CountReviewsForAdmin(ctx, q)
	if err != nil {
		return TaxReviewPage{}, err
	}
	now := u.now()
	out := TaxReviewPage{Records: make([]TaxReviewItemView, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for _, row := range rows {
		out.Records = append(out.Records, TaxReviewItemView{
			Item:          row,
			RemainingDays: remainingDaysUntil(row.Profile.PendingValidUntil, now),
		})
	}
	return out, nil
}

func remainingDaysUntil(deadline *time.Time, now time.Time) *int64 {
	if deadline == nil || deadline.IsZero() {
		return nil
	}
	days := int64(deadline.Sub(now) / (24 * time.Hour))
	return &days
}


// AdminProviderAccountsInput 是端點 42 的入參。
type AdminProviderAccountsInput struct {
	// Status 是含三態投影的狀態值（NOT_STARTED／CONFIG_ERROR 不落 DB）。空字串 ＝ 不篩。
	Status   string
	Keyword  string
	Page     int32
	PageSize int32
}

// ProviderAccountPage 是端點 42 的一頁。
type ProviderAccountPage struct {
	Records  []ProviderAccountListItem
	Total    int32
	Page     int32
	PageSize int32
}

// ProviderAccountListItem 是 A04 的一列。
//
// Status 已是三態投影後的值；Account 為 nil ⇒ 這位賣家還沒建 provider account 列
// （對外 NOT_STARTED）。把它當成錯誤或跳過該列，上線當天會少掉全體賣家。
type ProviderAccountListItem struct {
	StoreID int64
	Status  string
	Account *domain.ProviderAccountSummary
}

// ListProviderAccountsForAdmin 回全平台的 AMEGO 串接狀態（A04，端點 42）。
//
// ⚠️ 仍有一層漏斗：**沒有 invoice_issuers 列的賣家連查詢都不會回**（issuer 列是首次
// 開票／首次設定時才建）。要涵蓋「全平台每一個賣家」得拿 seller-service 的賣家清單來對，
// 而那條 client 不在本服務——缺口登記在交付回報，不在這裡用 join 補（跨服務 DB join 是禁區）。
func (u *Query) ListProviderAccountsForAdmin(ctx context.Context, in AdminProviderAccountsInput) (ProviderAccountPage, error) {
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.AdminProviderAccountQuery{
		Provider:     providerAMEGO,
		StatusFilter: in.Status,
		MatchNotStarted: in.Status == codemeta.InvoiceProviderAccountStatusNotStarted.Value,
		Keyword:         in.Keyword,
		Page:            sqlPage,
	}
	rows, err := u.accounts.ListForAdmin(ctx, q)
	if err != nil {
		return ProviderAccountPage{}, err
	}
	total, err := u.accounts.CountForAdmin(ctx, q)
	if err != nil {
		return ProviderAccountPage{}, err
	}
	out := ProviderAccountPage{Records: make([]ProviderAccountListItem, 0, len(rows)), Total: clampCount(total), Page: normPage, PageSize: normSize}
	for _, row := range rows {
		item := ProviderAccountListItem{StoreID: row.Issuer.IssuerRefID, Account: row.Account}
		if row.Account == nil {
			item.Status = projectProviderStatus(false, "")
		} else {
			item.Status = projectProviderStatus(true, row.Account.Status)
		}
		out.Records = append(out.Records, item)
	}
	return out, nil
}


// AdminProviderCutoversInput 是端點 46 的入參。
type AdminProviderCutoversInput struct {
	StoreID  int64
	Page     int32
	PageSize int32
}

// ProviderCutoverPage 是端點 46 的一頁。
type ProviderCutoverPage struct {
	Records  []domain.ProviderCutover
	Total    int32
	Page     int32
	PageSize int32
}

// ListProviderCutoversForAdmin 回一個賣家的自動開立切點歷史（A09，端點 46）。
//
// 🔴 查無 provider account 回 **9330**（不是空清單）：端點 46 的路徑鍵是 store_id，
// 而「這個賣家根本沒設過 AMEGO 連線」與「設過但還沒設切點」對 ADM 是兩件事——前者要去
// 看 A04 的串接狀態，後者要等賣家自己按端點 26。回空清單會把前者顯示成後者。
//
// ⚠️ 帳戶存在但一次切點都沒設過 ⇒ 空 records ＋ total=0，**那是正常結果不是錯誤**。
func (u *Query) ListProviderCutoversForAdmin(ctx context.Context, in AdminProviderCutoversInput) (ProviderCutoverPage, error) {
	account, err := u.accounts.GetByStoreID(ctx, in.StoreID, providerAMEGO)
	if err != nil {
		return ProviderCutoverPage{}, err
	}
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	rows, err := u.cutovers.ListForAdmin(ctx, account.ID, sqlPage)
	if err != nil {
		return ProviderCutoverPage{}, err
	}
	total, err := u.cutovers.CountForAdmin(ctx, account.ID)
	if err != nil {
		return ProviderCutoverPage{}, err
	}
	return ProviderCutoverPage{
		Records:  rows,
		Total:    clampCount(total),
		Page:     normPage,
		PageSize: normSize,
	}, nil
}


// AdminAuditLogsInput 是端點 61 的入參（九個 filter）。
type AdminAuditLogsInput struct {
	TargetActorAccountID *int64
	StoreID              *int64
	OrderNo              string
	DocumentID           *int64
	JobID                *int64
	OperationType        string
	Dates                domain.DateRange
	Page                 int32
	PageSize             int32
}

// AuditLogPage 是端點 61 的一頁。
type AuditLogPage struct {
	Records  []domain.AdminAuditLogItem
	Total    int32
	Page     int32
	PageSize int32
}

var auditResourceTypes = domain.AuditResourceTypes{
	TaxProfile:      "TAX_PROFILE",
	ProviderAccount: "PROVIDER_ACCOUNT",
	Job:             "JOB",
	Document:        auditResourceDocument,
	Complaint:       auditResourceComplaint,
}

// ListAuditLogsForAdmin 回 ADM 操作稽核（A10，端點 61）。
//
// ⚠️ before_state／after_state 的遮罩在**寫入時**完成（§6.1.14）：查詢時遮罩表示
// DB 裡存著明文，而本表保存 5 年 ＝ 一個五年期的明文 PII 倉庫。本支原樣回傳。
//
// ⚠️ 本支**不留 INVOICE_PII_REVEAL**：它回的是已遮罩的稽核內容，不揭露 P 類明文；
// 每次讀稽核都再寫一列稽核也會讓 InvoicePiiRevealSpike 告警永遠在響。
func (u *Query) ListAuditLogsForAdmin(ctx context.Context, in AdminAuditLogsInput) (AuditLogPage, error) {
	sqlPage, normPage, normSize := pageOf(in.Page, in.PageSize)
	q := domain.AdminAuditLogQuery{
		ResourceTypes:        auditResourceTypes,
		TargetActorAccountID: in.TargetActorAccountID,
		StoreID:              in.StoreID,
		OrderNo:              in.OrderNo,
		DocumentID:           in.DocumentID,
		JobID:                in.JobID,
		OperationType:        in.OperationType,
		Dates:                in.Dates,
		Page:                 sqlPage,
	}
	rows, err := u.audit.ListAdminAuditLogsForAdmin(ctx, q)
	if err != nil {
		return AuditLogPage{}, err
	}
	total, err := u.audit.CountAdminAuditLogsForAdmin(ctx, q)
	if err != nil {
		return AuditLogPage{}, err
	}
	return AuditLogPage{Records: rows, Total: clampCount(total), Page: normPage, PageSize: normSize}, nil
}


// KpiItem 是 A01 的一個 KPI。
//
// Available=false ⇒ **這個數字取不到**，Value 無意義。§6.1.11a 的 suspended_sellers
// 就是這一格：停權狀態的權威是 sellers.status（seller-service），發票模組只提建議、
// 不執行、也不持有那個狀態 ⇒ 在本服務湊一個本地計數等於造第二個真相，
// 而回 0 看起來就像「今天沒有人被停權」。
type KpiItem struct {
	Key         string
	Value       int64
	Available   bool
	WindowLabel string
	Definition  string
}

// AdminOverviewView 是端點 37 的投影。
type AdminOverviewView struct {
	KPIs         []KpiItem
	PriorityJobs []domain.OperationView
}

const adminPriorityJobLimit int32 = 20

var kpiDefinitions = map[string]struct{ Window, Definition string }{
	"pending_tax_reviews": {"目前", "review_status ∈ {SUBMITTED, UNDER_REVIEW, SUPPLEMENT_REQUIRED, SELF_ISSUE_PENDING} 的賣家數（＝A02 佇列全集）"},
	"near_deadline":       {"未來 7 天", "上述之中 deadlineAt − now ≤ 7 天的賣家數。**它是 pending_tax_reviews 的子集，兩者不可相加**"},
	"provider_errors":     {"目前", "providerStatus ∈ {INVALID_CREDENTIAL, PROVIDER_REJECTED, DEGRADED, CONFIG_ERROR} 的賣家數"},
	"pending_jobs":        {"目前", "A05 PENDING ＋ RETRYING 兩個分頁的 job 數"},
	"needs_manual":        {"目前", "A05 NEEDS_MANUAL 分頁的 job 數（job 狀態七值 OR 該 job 所屬 document 為 AWAITING_MANUAL）"},
	"open_complaints":     {"目前", "status ∈ {OPEN, OVERDUE, SELLER_RESPONDED, ESCALATED_TO_ADM} 的申訴數"},
	"overdue_complaints":  {"目前", "上述之中 status = OVERDUE **或** sla_due_at 已過的件數（兩者 OR：OVERDUE 由掃描寫入，掃描到期之間有一段窗口）"},
	"suspended_sellers":   {"目前", "因發票違規而停權中的賣家數。**權威是 sellers.status（seller-service），本服務不持有** ⇒ 取不到就明說取不到，不回 0"},
}

// GetInvoiceOverviewForAdmin 回 ADM 營運總覽（A01，端點 37）。
//
// 🔴 **沒有任何業務碼**：所有計數在無資料時回 0，不回 404。
// 🔴 near_deadline 是 pending_tax_reviews 的子集、needs_manual 與 pending_jobs 不相交
// 但都不是全集 ⇒ **A01 不得顯示任何合計**（口徑寫在每個 KPI 的 definition 裡）。
func (u *Query) GetInvoiceOverviewForAdmin(ctx context.Context) (AdminOverviewView, error) {
	now := u.now()
	counts, err := u.overview.AdminCounts(ctx, domain.AdminOverviewCountsQuery{
		TaxReviewQueueStatuses:   taxReviewQueueStatuses,
		NearDeadlineBefore:       now.Add(nearDeadlineWindow),
		ProviderErrorStatuses:    providerErrorStatuses,
		PendingJobStatuses:       pendingJobStatuses(),
		NeedsManualJobStatuses:   needsManualJobStatuses(),
		ManualDocumentStatuses:   manualDocumentStatuses,
		ResolvedDocumentStatuses: resolvedDocumentStatuses,
		OpenComplaintStatuses:    openComplaintStatuses,
		OverdueComplaintStatuses: overdueComplaintStatuses,
		Now:                      now,
	})
	if err != nil {
		return AdminOverviewView{}, err
	}

	out := AdminOverviewView{KPIs: []KpiItem{
		kpi("pending_tax_reviews", counts.PendingTaxReviewCount),
		kpi("near_deadline", counts.NearDeadlineCount),
		kpi("provider_errors", counts.ProviderErrorCount),
		kpi("pending_jobs", counts.PendingJobCount),
		kpi("needs_manual", counts.NeedsManualCount),
		kpi("open_complaints", counts.OpenComplaintCount),
		kpi("overdue_complaints", counts.OverdueComplaintCount),
		unavailableKpi("suspended_sellers"),
	}}

	jobs, err := u.operations.ListPriorityViewsForAdmin(ctx, domain.PriorityOperationQuery{
		OpenStatuses:           openJobStatuses(),
		ManualDocumentStatuses: manualDocumentStatuses,
		ResolvedDocumentStatuses: resolvedDocumentStatuses,
		Limit:                    adminPriorityJobLimit,
	})
	if err != nil {
		return AdminOverviewView{}, err
	}
	out.PriorityJobs = jobs
	return out, nil
}

func kpi(key string, value int64) KpiItem {
	meta := kpiDefinitions[key]
	return KpiItem{Key: key, Value: value, Available: true, WindowLabel: meta.Window, Definition: meta.Definition}
}

func unavailableKpi(key string) KpiItem {
	meta := kpiDefinitions[key]
	return KpiItem{Key: key, Available: false, WindowLabel: meta.Window, Definition: meta.Definition}
}


func clampCount(v int64) int32 {
	const maxInt32 = int64(^uint32(0) >> 1)
	if v > maxInt32 {
		return int32(maxInt32)
	}
	if v < 0 {
		return 0
	}
	return int32(v)
}

func containsString(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// CanRequestTaxExtension 是 canRequestExtension 旗標的**唯一**判準。
//
// 匯出是因為它有兩個消費者：端點 16 的 SellerCapabilityView（本檔）與端點 17 的
// toSellerTaxProfileData（internal/grpc/query.go）。後者原本自己算一份，於是 #3021
// 只修了前者時，S02 頁面實際讀的那一支完全沒變——同一個判準的兩份實作，
// 改一個不會讓另一個轉紅。
//
// 🔴 **判準必須與端點 20 的守衛逐字相同**，兩者是同一個問題的兩個出口：
// 旗標決定前端渲不渲染 CTA，守衛決定按下去成不成功。不一致的症狀不是「多一顆按鈕」，
// 而是**賣家在最需要延長的時候按下去，拿到一句「目前的稅務資格狀態不允許此操作」**
// ——既不知道為什麼，也不知道該怎麼辦（#3021，dev 實測）。
//
// 兩個條件缺一不可，而 2026-09-02 之前只有第二個：
//
//	① allowed-from 只有 SELF_ISSUE_PENDING（§6.1.12 相容表；TaxProfile.RequestExtension
//	   的同一行守衛）——SUBMITTED 期間平台正在審，補件期限不是賣家自己能動的
//	② 只能延一次（T11）——這一半原本就是對的，實測 extension_count=1 時旗標正確變 false
//
// ⚠️ 前端**不得**自己算 extension_count（seller-repo.ts 的註解已明訂），
// 所以這裡漏一個條件就沒有第二道防線。
func CanRequestTaxExtension(profile domain.SellerTaxProfile) bool {
	return profile.ReviewStatus == codemeta.SellerTaxReviewStatusSelfIssuePending.Value &&
		profile.ExtensionCount == 0
}
