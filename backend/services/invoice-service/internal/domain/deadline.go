package domain

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// 法定存證期限的自然日數（§0.3、§5.1.5 取值表、§7.1.2 步驟 9）。
//
// 🔴 禁止寫成 `invoice_date + 48h`：期限是**自然日**且末日不因週六日／國定假日延長，
// B2C 實際落在 48–72 小時之間 ⇒ `+48h` 最多會早算約 24 小時。而 statutory_deadline_at
// 同時是 FetchDueInvoiceOperations 的 ORDER BY 與 §7.5.6 告警階梯的依據，算錯不會有
// 任何東西轉紅，只會讓一批發票在錯的順序被處理、在錯的時點被告警。
// 48 小時只能用在「重試預算的保守分母」（§7.5.3），不是期限本身。
const (
	StatutoryDeadlineDaysB2C = 2
	StatutoryDeadlineDaysB2B = 7
)

// StatutoryDeadline 回傳法定存證期限（UTC）。
//
// 算式：自 invoiceDateTaipei 之**翌日**起算 N 個自然日，末日 23:59:59 Asia/Taipei
// ⇒ 末日 ＝ invoiceDateTaipei + N 天的 23:59:59。
//
// 第二個回傳值為 false 代表這種文件沒有法定期限（RECEIPT）——它不是錯誤，而是
// chk_invdoc_deadline_presence 這條雙向等式的另一半：
//
//	(document_kind IN ('B2C','B2B')) = (statutory_deadline_at IS NOT NULL)
//
// ⇒ 給 RECEIPT 填一個期限、或給 B2C／B2B 留 NULL，兩者都當場 23514。
// 注意這條等式**與 status 無關**：連 NOT_APPLICABLE 的 B2C 文件也必須有期限。
func StatutoryDeadline(invoiceDateTaipei time.Time, documentKind string) (time.Time, bool) {
	var days int
	switch documentKind {
	case codemeta.InvoiceDocumentKindB2c.Value:
		days = StatutoryDeadlineDaysB2C
	case codemeta.InvoiceDocumentKindB2b.Value:
		days = StatutoryDeadlineDaysB2B
	default:
		return time.Time{}, false
	}
	d := invoiceDateTaipei.In(timeutil.LocationTaipei)
	end := time.Date(d.Year(), d.Month(), d.Day()+days, 23, 59, 59, 0, timeutil.LocationTaipei)
	return end.UTC(), true
}
