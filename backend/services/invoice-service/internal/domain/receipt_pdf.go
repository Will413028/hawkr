package domain

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// ---------------------------------------------------------------------------
// 平台代製收據的 PDF 產製（#3026，58 §S-6：INV.op 不打 AMEGO，改走本地 PDF 產生）
//
// 🔴 **這不是發票**。免用統一發票的小規模營業人依法本來就不開發票（營業稅法 §32 I
// 但書、統一發票使用辦法 §4 I-1），平台代製的是一份**購買憑證**。§L11 逐字：
// PLATFORM_RECEIPT「不是延後也不是開票失敗，是完全不同的一條線」。
//
// 🔴 **出據人是賣家、不是平台**（58 §Q-B7 已定的那一半）：版面上的銷售人是賣家的
// 法定名稱，平台只出現在頁尾的產製說明。§335 逐字「所有對外文案禁用『平台開立』
// 『平台代開』措辭」——`PLATFORM_RECEIPT` 是那條禁令的例外，但例外的是**代製**
// 這個動作，不是**出據人**。
//
// ⚠️ 版面欄位是 DEC-20260902-205712 裁決 A 採用的「最小必要集」，**不是法務定版**：
// §Q-B7「收據格式」在設計書上仍標為 blocking。改版時連同 ReceiptLayoutVersion 一起改，
// 那個版本號會落進 receipt_authorization_version 之外的稽核脈絡。
// ---------------------------------------------------------------------------

// ReceiptLayoutVersion 是收據版面的版本。改版面就改它。
//
// 它與 SelfIssueTermsVersion 是兩件事：後者是賣家簽的授權條款版本，這個是紙面長相。
const ReceiptLayoutVersion = "2026-09-02"

// ReceiptNotInvoiceNotice 是版面上那一行「這不是統一發票」的定版文字。
//
// 🔴 **不得省略**：買家拿到一張長得像單據的 PDF，若不明說它不是統一發票，
// 他可能拿去報帳而被退件——而那時候賣家與平台都已經無從補救。
const ReceiptNotInvoiceNotice = "本收據非統一發票，出據人依法免用統一發票（統一發票使用辦法第 4 條第 1 項第 1 款）。"

// ReceiptSeller 是出據人（賣家）。
type ReceiptSeller struct {
	LegalName string // 法定名稱；空字串時退回 DisplayName
	TaxID     string // 統一編號；免用者為空
	Address   string
}

// ReceiptLine 是收據上的一列品項。
type ReceiptLine struct {
	Name      string
	Quantity  int32
	UnitPrice int64
	Amount    int64
}

// ReceiptData 是產一張收據所需的全部資料。
//
// 🔴 **不含買家姓名／電話／地址／email**：收據上的買受人一律是「消費者」
// （§5.1.5 的同一條規則：recipient_name 永不含自然人真實姓名）。
// 這份 PDF 會落進 media 的 secure/ 前綴並保存 5 年，多帶一個個資欄就是多五年的風險。
type ReceiptData struct {
	ReceiptNo   string
	OrderNo     string
	IssuedAt    time.Time // 台北曆日
	Seller      ReceiptSeller
	Lines       []ReceiptLine
	TotalAmount int64
}

// ReceiptRenderer 產出收據 PDF 的位元組。
type ReceiptRenderer struct{ fontTTF []byte }

// NewReceiptRenderer 以一份 TTF 字型建出 renderer。
//
// 🔴 **必須是 TTF，TTC 不行**：alpine 的 font-noto-cjk／font-wqy-zenhei 都是 .ttc，
// 而 fpdf 對 TTC 回 "get metrics Error: not supported" 之後把字型當成未定義
// ——症狀是 OutputFileAndClose 回 "undefined font"，不是產出一張沒有中文的 PDF。
// ⇒ 字型由 Dockerfile 抓一份釘住 SHA256 的 Noto Sans TC（OFL）進 image。
func NewReceiptRenderer(fontTTF []byte) (*ReceiptRenderer, error) {
	if len(fontTTF) == 0 {
		return nil, fmt.Errorf("invoice: receipt renderer needs a TTF font (got 0 bytes)")
	}
	return &ReceiptRenderer{fontTTF: fontTTF}, nil
}

// Render 產出一張收據的 PDF。
//
// 🔴 產出後**自驗 magic bytes**：fpdf 對字型問題是「回 error 或吐一個沒有內容的檔」，
// 而呼叫端會把回傳的 bytes 當收據存進 media 並寄給買家。§8.1.5 P8 對 AMEGO 的 PDF
// 也是同一條規矩（只看 status 會把錯誤頁存成 PDF），本地產製沒有理由更寬鬆。
func (r *ReceiptRenderer) Render(d ReceiptData) ([]byte, error) {
	if d.ReceiptNo == "" {
		return nil, fmt.Errorf("invoice: receipt needs a receipt_no")
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("cjk", "", r.fontTTF)
	pdf.SetFont("cjk", "", 12)
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// 🔴 版面版本進 **PDF metadata**，不印在紙面上。
	// 買家拿到的是一份對外文件，「版面版本 2026-09-02」對他沒有意義，只會讓它看起來
	// 像半成品；但改版之後要回答「這張是哪一版畫的」又非它不可 ⇒ 放在讀得到、
	// 卻不佔版面的地方。
	pdf.SetTitle("購買證明收據 "+d.ReceiptNo, true)
	// isUTF8=false：這個值全是 ASCII，傳 true 會被轉成 UTF-16BE 寫進檔案，
	// 之後任何人（含測試）拿字面值去比對都找不到它。
	pdf.SetSubject("layout="+ReceiptLayoutVersion, false)

	pdf.SetFontSize(18)
	pdf.CellFormat(0, 12, "購買證明收據", "", 1, "C", false, 0, "")
	pdf.Ln(6)

	pdf.SetFontSize(11)
	// 出據人在最上面：這份文件是誰出的，是它最重要的一件事。
	sellerName := d.Seller.LegalName
	row(pdf, "出據人（銷售人）", sellerName)
	if d.Seller.TaxID != "" {
		row(pdf, "統一編號", d.Seller.TaxID)
	}
	if d.Seller.Address != "" {
		row(pdf, "營業地址", d.Seller.Address)
	}
	// 買受人恆為「消費者」——本服務不持有買家真名（§5.1.5）。
	row(pdf, "買受人", "消費者")
	row(pdf, "收據編號", d.ReceiptNo)
	row(pdf, "訂單編號", d.OrderNo)
	row(pdf, "開立日期", d.IssuedAt.Format("2006-01-02"))
	pdf.Ln(4)

	// 品項明細
	pdf.SetFontSize(10)
	pdf.CellFormat(90, 8, "品名", "1", 0, "C", false, 0, "")
	pdf.CellFormat(20, 8, "數量", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, "單價", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, "小計", "1", 1, "C", false, 0, "")
	for _, l := range d.Lines {
		pdf.CellFormat(90, 8, l.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 8, strconv.FormatInt(int64(l.Quantity), 10), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, money(l.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, money(l.Amount), "1", 1, "R", false, 0, "")
	}
	pdf.CellFormat(140, 8, "合計", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, money(d.TotalAmount), "1", 1, "R", false, 0, "")
	pdf.Ln(6)

	pdf.SetFontSize(9)
	pdf.MultiCell(0, 5, ReceiptNotInvoiceNotice, "", "L", false)
	pdf.MultiCell(0, 5, "本收據由 Hawkr 平台依出據人授權代為產製並保存。", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("invoice: render receipt %s: %w", d.ReceiptNo, err)
	}
	out := buf.Bytes()
	// 🔴 自驗：字型出問題時 fpdf 可能吐一個沒有內容的檔而不回 error。
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		return nil, fmt.Errorf("invoice: rendered receipt %s is not a PDF (len=%d)", d.ReceiptNo, len(out))
	}
	return out, nil
}

func row(pdf *fpdf.Fpdf, label, value string) {
	pdf.CellFormat(40, 7, label, "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
}

// money 把「元」的整數折成千分位字串。**不做任何浮點運算**（10 檔 §4）。
func money(v int64) string {
	s := strconv.FormatInt(v, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
