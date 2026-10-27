package amego

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// AMEGO 端點路徑常數。這一組同時是 §4.5.9 情境 B 的 nginx 白名單來源：
// 15 個路徑一對一，多一條稀釋白名單原則、少一條切換那天靜默 502。
const (
	EndpointIssue           = "/json/f0401"
	EndpointVoid            = "/json/f0501"
	EndpointAllowance       = "/json/g0401"
	EndpointVoidAllowance   = "/json/g0501"
	EndpointInvoiceQuery    = "/json/invoice_query"
	EndpointInvoiceList     = "/json/invoice_list"
	EndpointInvoiceFile     = "/json/invoice_file"
	EndpointInvoiceStatus   = "/json/invoice_status"
	EndpointAllowanceQuery  = "/json/allowance_query"
	EndpointAllowanceStatus = "/json/allowance_status"
	EndpointBarcode         = "/json/barcode"
	EndpointBanQuery        = "/json/ban_query"
	EndpointTrackAll        = "/json/track_all"
	EndpointTrackStatus     = "/json/track_status"
	EndpointServerTime      = "/json/time"
)

// Endpoints 是本 package 會打的全部路徑，供 §13 的 nginx 白名單對拍測試使用。
func Endpoints() []string {
	return []string{
		EndpointIssue, EndpointVoid, EndpointAllowance, EndpointVoidAllowance,
		EndpointInvoiceQuery, EndpointInvoiceList, EndpointInvoiceFile, EndpointInvoiceStatus,
		EndpointAllowanceQuery, EndpointAllowanceStatus, EndpointBarcode, EndpointBanQuery,
		EndpointTrackAll, EndpointTrackStatus, EndpointServerTime,
	}
}

// scalarString 讀一個「有時是 JSON 字串、有時是 JSON 數字」的欄位，並保留**字面值**。
//
// AMEGO 的 `code` 在查詢類回 Number、在 g0401／g0501／f0701 回 String（§4.5.1 事實 3）；
// `invoice_date` 文件標 String 而實際回 Number。以 int 綁 code 會在折讓的錯誤路徑上
// unmarshal 失敗 —— 而那正是最需要讀到錯誤碼的路徑。
//
// 保留字面值（而非轉成 int 再格式化）是刻意的：7 位數的碼經過 float64 會失真，
// 而分類完全靠這個字串做精確比對。
func unmarshalScalar(b []byte) (string, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return "", nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return "", fmt.Errorf("amego: decode scalar string: %w", err)
		}
		return s, nil
	}
	return string(b), nil
}

// providerCode 是 AMEGO 的回應碼，一律以字串承載（落庫也是字串，§4.5.12）。
type providerCode string

// UnmarshalJSON 吃 number 與 string 兩種型別。
func (p *providerCode) UnmarshalJSON(b []byte) error {
	s, err := unmarshalScalar(b)
	if err != nil {
		return err
	}
	*p = providerCode(s)
	return nil
}

// flexString 同 providerCode 的寬容解析，用在 invoice_date 這種型別漂移的欄位。
type flexString string

// UnmarshalJSON 吃 number 與 string 兩種型別。
func (f *flexString) UnmarshalJSON(b []byte) error {
	s, err := unmarshalScalar(b)
	if err != nil {
		return err
	}
	*f = flexString(s)
	return nil
}

// ── 四種 envelope（§4.5.1 事實 2：不可用單一泛型 envelope）────────────────────

// stdEnvelope 是絕大多數 API 的形狀。
type stdEnvelope struct {
	Code providerCode    `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// pageEnvelope 是 invoice_list／allowance_list 的形狀：三個分頁欄與 data 平級。
type pageEnvelope struct {
	Code      providerCode    `json:"code"`
	Msg       string          `json:"msg"`
	PageTotal int             `json:"page_total"`
	PageNow   int             `json:"page_now"`
	DataTotal int             `json:"data_total"`
	Data      json.RawMessage `json:"data"`
}

// issueEnvelope 是 f0401 的形狀：業務結果攤在頂層、不放 data。
// 套泛型 envelope 會拿不到發票號碼。
type issueEnvelope struct {
	Code          providerCode `json:"code"`
	Msg           string       `json:"msg"`
	InvoiceNumber string       `json:"invoice_number"`
	InvoiceTime   int64        `json:"invoice_time"`
	RandomNumber  string       `json:"random_number"`
	Barcode       string       `json:"barcode"`
	QRCodeLeft    string       `json:"qrcode_left"`
	QRCodeRight   string       `json:"qrcode_right"`
	Base64Data    string       `json:"base64_data"`
}

// timeEnvelope 是 GET /json/time 的形狀：沒有 code／msg，Content-Type 是 text/plain。
// 程式一律只用 timestamp，不解析 text（text 是台北時間、文件未載明時區）。
type timeEnvelope struct {
	Timestamp int64  `json:"timestamp"`
	Text      string `json:"text"`
}

// ── Request DTO（欄名逐字照 AMEGO；不是我方命名慣例）──────────────────────────

// IssueProductItem 是 f0401 的明細行。UnitPrice／Amount 預設含稅（DetailVat=1）。
type IssueProductItem struct {
	Description string `json:"Description"`
	Quantity    string `json:"Quantity"`
	UnitPrice   string `json:"UnitPrice"`
	Amount      string `json:"Amount"`
	Unit        string `json:"Unit,omitempty"`
	Remark      string `json:"Remark"`
	TaxType     string `json:"TaxType"`
}

// IssueRequest 是 /json/f0401 的 data（**單一物件**，送陣列回 3040112）。
//
// 金額欄位一律由 internal/domain 算好後以字串傳入（§4.5.5）：本 package 零業務判斷，
// 不重算、不校正、不決定稅別。
type IssueRequest struct {
	OrderID              string             `json:"OrderId"`
	TrackAPICode         string             `json:"TrackApiCode,omitempty"`
	BuyerIdentifier      string             `json:"BuyerIdentifier"`
	BuyerName            string             `json:"BuyerName"`
	BuyerAddress         string             `json:"BuyerAddress"`
	BuyerTelephoneNumber string             `json:"BuyerTelephoneNumber"`
	BuyerEmailAddress    string             `json:"BuyerEmailAddress"`
	MainRemark           string             `json:"MainRemark"`
	CarrierType          string             `json:"CarrierType"`
	CarrierID1           string             `json:"CarrierId1"`
	CarrierID2           string             `json:"CarrierId2"`
	NPOBAN               string             `json:"NPOBAN"`
	ProductItem          []IssueProductItem `json:"ProductItem"`
	SalesAmount          string             `json:"SalesAmount"`
	FreeTaxSalesAmount   string             `json:"FreeTaxSalesAmount"`
	ZeroTaxSalesAmount   string             `json:"ZeroTaxSalesAmount"`
	TaxType              string             `json:"TaxType"`
	TaxRate              string             `json:"TaxRate"`
	TaxAmount            string             `json:"TaxAmount"`
	TotalAmount          string             `json:"TotalAmount"`
	CustomsClearanceMark string             `json:"CustomsClearanceMark,omitempty"`
	ZeroTaxRateReason    string             `json:"ZeroTaxRateReason,omitempty"`
	DetailVat            string             `json:"DetailVat,omitempty"`
	DetailAmountRound    string             `json:"DetailAmountRound,omitempty"`
}

// VoidRequest 是 /json/f0501 的一筆。data 是陣列，但我方一次只送一張：
// 多張時錯誤是「整批第 n 筆」形式的單一錯誤，無法得知部分成功／部分失敗。
type VoidRequest struct {
	CancelInvoiceNumber string `json:"CancelInvoiceNumber"`
	CancelReason        string `json:"CancelReason,omitempty"`
}

// AllowanceProductItem 是 g0401 的明細行。⚠️ UnitPrice／Amount 固定是**未稅**，
// 與 f0401 的含稅預設相反 —— 兩套 DTO 不共用金額欄位語意是刻意的。
type AllowanceProductItem struct {
	OriginalInvoiceNumber string `json:"OriginalInvoiceNumber"`
	OriginalInvoiceDate   string `json:"OriginalInvoiceDate"`
	OriginalDescription   string `json:"OriginalDescription"`
	Quantity              string `json:"Quantity"`
	UnitPrice             string `json:"UnitPrice"`
	Amount                string `json:"Amount"`
	Tax                   string `json:"Tax"`
	TaxType               string `json:"TaxType"`
}

// AllowanceRequest 是 /json/g0401 的一筆（data 是陣列，我方一次只送一張）。
type AllowanceRequest struct {
	AllowanceNumber      string                 `json:"AllowanceNumber"`
	AllowanceDate        string                 `json:"AllowanceDate"`
	AllowanceType        string                 `json:"AllowanceType"`
	BuyerIdentifier      string                 `json:"BuyerIdentifier"`
	BuyerName            string                 `json:"BuyerName"`
	BuyerAddress         string                 `json:"BuyerAddress"`
	BuyerTelephoneNumber string                 `json:"BuyerTelephoneNumber"`
	BuyerEmailAddress    string                 `json:"BuyerEmailAddress"`
	ProductItem          []AllowanceProductItem `json:"ProductItem"`
	TaxAmount            string                 `json:"TaxAmount"`
	TotalAmount          string                 `json:"TotalAmount"`
}

// VoidAllowanceRequest 是 /json/g0501 的一筆。g0501 沒有原因欄位（實測 CancelReason 無效）。
type VoidAllowanceRequest struct {
	CancelAllowanceNumber string `json:"CancelAllowanceNumber"`
}

// ListRequest 是 /json/invoice_list 的 data。
// DateSelect：1 發票日期／2 建立日期。Limit 只收 20~500（越界回 code 34）。
type ListRequest struct {
	DateSelect int    `json:"date_select"`
	DateStart  string `json:"date_start"`
	DateEnd    string `json:"date_end"`
	Limit      int    `json:"limit,omitempty"`
	Page       int    `json:"page,omitempty"`
}

// FileRequest 是 /json/invoice_file 的 data。Type 為 order 或 invoice，擇一。
// DownloadStyle 的合法值只有 {0,1,2,3,5}：AMEGO 對它不做值域驗證，
// 錯誤值會靜默產生非預期版面的 PDF ⇒ 白名單由我方守（§2.7.1）。
type FileRequest struct {
	Type          string `json:"type"`
	OrderID       string `json:"order_id,omitempty"`
	InvoiceNumber string `json:"invoice_number,omitempty"`
	DownloadStyle int    `json:"download_style"`
}

// ── Result DTO ──────────────────────────────────────────────────────────────

// IssueResult 是 f0401 成功後的業務結果。
type IssueResult struct {
	InvoiceNumber string
	InvoiceTime   int64
	RandomNumber  string
	Barcode       string
	QRCodeLeft    string
	QRCodeRight   string
}

// VoidAccepted 代表 f0501／g0501 回了 code 0 —— 那**只代表受理**。
// 真正完成靠 recheck worker 看到 invoice_type 變 C0501 / D0501。
// 型別名擋掉「回 0 ＝ 已作廢」的誤解。
type VoidAccepted struct {
	Accepted bool
}

// AllowanceResult 是 g0401 受理後的結果。AMEGO 不回折讓單號（是我方指定的）。
type AllowanceResult struct {
	AllowanceNumber string
	Accepted        bool
}

// InvoiceProductItem 是查詢回應的明細行。
type InvoiceProductItem struct {
	TaxType     int     `json:"tax_type"`
	Description string  `json:"description"`
	UnitPrice   float64 `json:"unit_price"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	Amount      float64 `json:"amount"`
	Remark      string  `json:"remark"`
}

// InvoiceWait 是「未處理的排程」一筆。wait 清空才代表異動完成。
type InvoiceWait struct {
	InvoiceType string `json:"invoice_type"`
	CreateDate  int64  `json:"create_date"`
}

// InvoiceAllowanceRef 是查詢回應中掛在發票下的折讓單。
type InvoiceAllowanceRef struct {
	InvoiceType     flexString `json:"invoice_type"`
	InvoiceStatus   int        `json:"invoice_status"`
	AllowanceType   flexString `json:"allowance_type"`
	AllowanceNumber string     `json:"allowance_number"`
	AllowanceDate   flexString `json:"allowance_date"`
	TaxAmount       int64      `json:"tax_amount"`
	TotalAmount     int64      `json:"total_amount"`
}

// InvoiceDetail 是 invoice_query 的 data。
//
// ⚠️ InvoiceDate／InvoiceTime 是**台灣當地**的發票日期時間，不是 UTC 時間戳，
// mapper 層必須顯式區分、禁止與 ISO 8601 UTC 欄位互相冒充（§4.4.1、W-09）。
type InvoiceDetail struct {
	InvoiceNumber   string                `json:"invoice_number"`
	InvoiceType     string                `json:"invoice_type"`
	InvoiceStatus   int                   `json:"invoice_status"`
	InvoiceDate     flexString            `json:"invoice_date"`
	InvoiceTime     string                `json:"invoice_time"`
	BuyerIdentifier string                `json:"buyer_identifier"`
	BuyerName       string                `json:"buyer_name"`
	SalesAmount     int64                 `json:"sales_amount"`
	FreeTaxAmount   int64                 `json:"free_tax_sales_amount"`
	ZeroTaxAmount   int64                 `json:"zero_tax_sales_amount"`
	TaxType         int                   `json:"tax_type"`
	TaxRate         string                `json:"tax_rate"`
	TaxAmount       int64                 `json:"tax_amount"`
	TotalAmount     int64                 `json:"total_amount"`
	RandomNumber    string                `json:"random_number"`
	CarrierType     string                `json:"carrier_type"`
	CarrierID1      string                `json:"carrier_id1"`
	CarrierID2      string                `json:"carrier_id2"`
	NPOBAN          string                `json:"npoban"`
	CancelDate      int64                 `json:"cancel_date"`
	OrderID         string                `json:"order_id"`
	CreateDate      int64                 `json:"create_date"`
	ProductItem     []InvoiceProductItem  `json:"product_item"`
	Wait            []InvoiceWait         `json:"wait"`
	Allowance       []InvoiceAllowanceRef `json:"allowance"`
}

// InvoiceSummary 是 invoice_list 的一列（主檔，無明細／無 wait／無 allowance）。
type InvoiceSummary struct {
	InvoiceNumber string     `json:"invoice_number"`
	InvoiceType   string     `json:"invoice_type"`
	InvoiceStatus int        `json:"invoice_status"`
	InvoiceDate   flexString `json:"invoice_date"`
	InvoiceTime   string     `json:"invoice_time"`
	TotalAmount   int64      `json:"total_amount"`
	RandomNumber  string     `json:"random_number"`
	OrderID       string     `json:"order_id"`
	CreateDate    int64      `json:"create_date"`
}

// InvoicePage 是 invoice_list 的一頁。
//
// ⚠️ 分頁超界回 code 0 ＋ 空陣列、**不報錯** ⇒ 終止條件只能是「Records 為空」的顯式判定，
// 不可依賴分頁錯誤碼（W/X26）。翻頁上限由呼叫端（INV.rc）以每輪 1 頁 ＋ 游標落庫執行。
type InvoicePage struct {
	PageTotal int
	PageNow   int
	DataTotal int
	Records   []InvoiceSummary
}

// AllowanceDetailItem 是 allowance_query 的明細行（金額皆未稅）。
type AllowanceDetailItem struct {
	OriginalInvoiceNumber string     `json:"original_invoice_number"`
	OriginalInvoiceDate   flexString `json:"original_invoice_date"`
	TaxType               int        `json:"tax_type"`
	Description           string     `json:"description"`
	UnitPrice             float64    `json:"unit_price"`
	Quantity              float64    `json:"quantity"`
	Unit                  string     `json:"unit"`
	Amount                float64    `json:"amount"`
	Tax                   int64      `json:"tax"`
}

// AllowanceDetail 是 allowance_query 的 data。TotalAmount 是**未稅**合計。
type AllowanceDetail struct {
	AllowanceNumber string                `json:"allowance_number"`
	InvoiceType     string                `json:"invoice_type"`
	InvoiceStatus   int                   `json:"invoice_status"`
	AllowanceDate   flexString            `json:"allowance_date"`
	AllowanceType   flexString            `json:"allowance_type"`
	BuyerIdentifier string                `json:"buyer_identifier"`
	BuyerName       string                `json:"buyer_name"`
	TaxAmount       int64                 `json:"tax_amount"`
	TotalAmount     int64                 `json:"total_amount"`
	CancelDate      int64                 `json:"cancel_date"`
	CreateDate      int64                 `json:"create_date"`
	ProductItem     []AllowanceDetailItem `json:"product_item"`
	Wait            []InvoiceWait         `json:"wait"`
}

// UploadStatus 回答「財政部收到了沒有」，回填 provider_upload_status。
//
// 🔴 **不可用來回答「這張開過了沒有」**：超出窗口（約當期＋前兩期）的憑證靜默回
// Type=NOT_FOUND ＋ code 0 ＋ Status=-1，無任何錯誤訊號。冪等判斷一律走
// QueryInvoiceByOrderID / QueryAllowance。
type UploadStatus struct {
	Number      string
	Type        string
	Status      int
	TotalAmount int64
	TaxAmount   int64
}

// Track 是 track_all 的一列。三層巢狀已攤平，Layer 保留原值；
// Category／Now／TrackAPICode／Source／Status 只在 layer 3 出現。
type Track struct {
	Layer        int     `json:"layer"`
	Category     int     `json:"category"`
	Code         string  `json:"code"`
	Start        string  `json:"start"`
	End          string  `json:"end"`
	Now          string  `json:"now"`
	TotalBooklet int     `json:"total_booklet"`
	Remark       string  `json:"remark"`
	TrackAPICode string  `json:"TrackApiCode"`
	Source       int     `json:"source"`
	Status       int     `json:"status"`
	Data         []Track `json:"data"`
}

// TrackBalance 是 track_status 的一列（只回「API 配號」類型的字軌）。
type TrackBalance struct {
	Code         string `json:"code"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Now          string `json:"now"`
	TotalBooklet int    `json:"total_booklet"`
	UsedBooklet  int    `json:"used_booklet"`
	Status       int    `json:"status"`
}
