package usecase

import (
	"errors"
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TestLooksLikePDF 釘住 §8.1.5 P8 的第二句硬要求。
//
// 🔴 為什麼非有不可（P8 逐字）：AMEGO 的 `file_url` 過期或被竄改時回
// **HTTP 200 ＋ 中文純文字**。只看狀態碼會把那段文字當成 PDF 存進自有儲存，
// 而業務表會記下 pdf_object_key ⇒ 買家點下載才發現拿到壞檔，
// 且此時 AMEGO 那份可能已超過 180 天、再也抓不回來。
func TestLooksLikePDF(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
		want        bool
	}{
		{"content-type 正確", "application/pdf", "anything", true},
		{"magic bytes 正確（content-type 缺）", "", "%PDF-1.7\n...", true},
		{"magic bytes 正確（content-type 錯）", "text/html", "%PDF-1.4", true},
		// 🔴 這一格是 P8 要防的那個：HTTP 200 ＋ 中文純文字。
		{"過期回中文純文字 → 必須擋下", "text/plain; charset=utf-8", "檔案已過期，請重新查詢", false},
		{"HTML 錯誤頁 → 必須擋下", "text/html", "<html><body>error</body></html>", false},
		{"空 body → 必須擋下", "", "", false},
		// magic bytes 只認**前綴**：PDF 出現在中間不算（那是把 PDF 嵌在錯誤頁裡的形狀）。
		{"%PDF- 在中間不算", "text/html", "error %PDF- error", false},
	}
	for _, c := range cases {
		if got := looksLikePDF(c.contentType, []byte(c.body)); got != c.want {
			t.Errorf("%s: looksLikePDF(%q, %q)=%v want %v", c.name, c.contentType, c.body, got, c.want)
		}
	}
}

// TestDownloadStyleFor 釘住 §8.1.5 第 3 條：「沒打統編的發票只有 0」。
//
// 🔴 這是 2026-08-31 dev 實測抓到的真 bug：我原本寫死 DownloadStyle:1，
// 而那張 B2C 發票的 recipient_tax_id 是十個 0 ⇒ AMEGO 連續回 code 53
// 「該發票類型不可下載」，眼看要燒完 8 次重試然後以 Error 級「放棄」收場，
// 而根本沒有任何東西壞掉。
//
// ⚠️ AMEGO **不會幫你擋**：同一條規格寫著「對 download_style 無值域驗證，
// 傳 9 仍回 code:0 並產出 URL」——但「沒統編卻傳非 0」它會回 53。
func TestDownloadStyleFor(t *testing.T) {
	cases := []struct {
		name  string
		taxID string
		want  int
	}{
		{"十個 0（B2C 無統編佔位）→ 0", "0000000000", 0},
		{"空字串 → 0", "", 0},
		{"有真統編 → 1", "12345678", 1},
	}
	for _, c := range cases {
		got := downloadStyleFor(domain.Document{RecipientTaxID: c.taxID})
		if got != c.want {
			t.Errorf("%s: downloadStyleFor(%q)=%d want %d", c.name, c.taxID, got, c.want)
		}
	}
}

// TestIsNotDownloadable 釘住 code 53 的分流。
//
// 🔴 §8.1.5 第 4 條逐字：「53 在官方錯誤表出現兩次且語意不同，**不可只靠 code 分支，
// 必須同時記錄 msg**」。兩個語意（載具中獎前無 PDF ⇒ 9312 正常業務分支／該類型
// 不可下載）的共同點是「再重試 8 次也不會變」⇒ 一律停止且**不燒 attempt**。
//
// 為什麼不能讓它燒完 8 次：那條路徑會印 Error 級的「放棄」log，而那是留給
// 「法規要 5 年、AMEGO 只給 180 天，這份沒補就永久取不回」的真實風險用的。
// 把正常業務分支混進去，第二次就會被當成狼來了。
func TestIsNotDownloadable(t *testing.T) {
	if !isNotDownloadable(&amego.APIError{Code: "53", Msg: "該發票類型不可下載"}) {
		t.Error("code 53 應判為不可下載")
	}
	if !isNotDownloadable(&amego.APIError{Code: "53", Msg: "載具發票中獎前無法下載"}) {
		t.Error("code 53 的另一個語意也應判為不可下載（不可只靠 msg 分支）")
	}
	if isNotDownloadable(&amego.APIError{Code: "10", Msg: "系統維護中"}) {
		t.Error("code 10 是暫時性故障，應該重試而不是停止")
	}
	if isNotDownloadable(errors.New("connection reset")) {
		t.Error("非 APIError 應該重試")
	}
}
