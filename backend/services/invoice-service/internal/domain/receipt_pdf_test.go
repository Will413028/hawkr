package domain

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// testFont 回測試用的字型**子集**（45 KB，見 testdata/README.md）。
//
// 🔴 這裡刻意**不 skip**：skip 不是 PASS，而 CI 上一支永遠 skip 的測試等於不存在
// （go test 不帶 -v 連 SKIP 都不印）。讀不到就 Fatal —— 檔案在版控裡，讀不到代表
// 有人把它刪了，那正是要知道的事。
//
// ⚠️ 子集只含本檔測試字串用得到的字。它驗的是「產出是不是一份真 PDF、字型子集有沒有
// 被內嵌」，不是「字型涵蓋多少漢字」；後者由 Dockerfile 的 SHA256 釘選 ＋ DI 的
// fail-closed（字型讀不起來 pod 起不來）保證。
func testFont(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "NotoSansTC-subset.ttf"))
	if err != nil {
		t.Fatalf("讀不到測試字型子集（它在版控裡，見 testdata/README.md）：%v", err)
	}
	return b
}

func sampleReceipt() ReceiptData {
	return ReceiptData{
		ReceiptNo: "R000000000042",
		OrderNo:   "O318051141",
		IssuedAt:  time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		Seller: ReceiptSeller{
			LegalName: "陳記蔬果行",
			Address:   "台北市內湖區瑞光路1號",
		},
		Lines: []ReceiptLine{
			{Name: "有機高麗菜", Quantity: 2, UnitPrice: 120, Amount: 240},
			{Name: "溫室小番茄", Quantity: 1, UnitPrice: 815, Amount: 815},
		},
		TotalAmount: 1055,
	}
}

// 🔴 產出必須是**真的 PDF**：驗收條件逐字是「可開啟的收據 PDF（%PDF- magic bytes，
// 不只是 HTTP 200）」。fpdf 對字型問題可能吐一個沒有內容的檔而不回 error。
func TestReceiptRenderer_ProducesRealPDF(t *testing.T) {
	r, err := NewReceiptRenderer(testFont(t))
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(sampleReceipt())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("不是 PDF：前 8 bytes = %q", out[:min(8, len(out))])
	}
	if !bytes.Contains(out[max(0, len(out)-1024):], []byte("%%EOF")) {
		t.Fatal("結尾沒有 EOF 標記，檔案不完整")
	}
	// 🔴 字型子集必須真的被內嵌：沒有 FontFile2 代表中文會是空白方框，
	// 而 PDF 本身完全合法、magic bytes 也對——只驗 %PDF- 抓不到這一種。
	if !bytes.Contains(out, []byte("/FontFile2")) {
		t.Fatal("PDF 沒有內嵌字型子集，中文會顯示不出來")
	}
}

// 🔴 版面版本必須留在**產物**裡（PDF metadata），而不是只留在原始碼的常數。
// 收據改版之後，已寄出的那些不會回頭 —— 屆時「這張是哪一版畫的」只能從檔案本身讀。
// 它同時是「不印在紙面上」的另一半：紙面不得出現版本字樣（那是開發者的資訊）。
func TestReceiptRenderer_EmbedsLayoutVersionInMetadata(t *testing.T) {
	r, err := NewReceiptRenderer(testFont(t))
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(sampleReceipt())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("layout="+ReceiptLayoutVersion)) {
		t.Errorf("PDF metadata 不含版面版本 %q", ReceiptLayoutVersion)
	}
}

// 🔴 沒有字型時 fail loud，不得回一份沒有中文的 PDF。
func TestNewReceiptRenderer_RefusesEmptyFont(t *testing.T) {
	if _, err := NewReceiptRenderer(nil); err == nil {
		t.Fatal("空字型應報錯")
	}
}

// 🔴 收據編號是這份文件的識別，缺它不得產出。
func TestReceiptRenderer_RefusesWithoutReceiptNo(t *testing.T) {
	r, err := NewReceiptRenderer(testFont(t))
	if err != nil {
		t.Fatal(err)
	}
	d := sampleReceipt()
	d.ReceiptNo = ""
	if _, err := r.Render(d); err == nil {
		t.Fatal("沒有 receipt_no 應報錯")
	}
}

// money 不做浮點運算（10 檔 §4：金額以「元」的整數表示）。
func TestMoney(t *testing.T) {
	cases := map[int64]string{0: "0", 5: "5", 100: "100", 1055: "1,055",
		1000000: "1,000,000", -1234: "-1,234"}
	for in, want := range cases {
		if got := money(in); got != want {
			t.Errorf("money(%d) = %q, want %q", in, got, want)
		}
	}
}

// 🔴 版面上不得出現任何買家個資：收據會落 media secure/ 並保存 5 年。
//
// 這一格用反射掃 ReceiptData（含巢狀 ReceiptSeller／ReceiptLine）的欄位名，
// 命中買家個資的字樣就紅。**為什麼不去驗產出的 PDF**：PDF 的文字是編碼後的，
// 「有沒有出現某個字串」在那一層驗不到 —— 那會變成一支永遠綠的測試。
// 攔在型別上才是這條保證真正的位置：欄位不存在，就沒有東西能被畫上去。
func TestReceiptData_HasNoBuyerPIIFields(t *testing.T) {
	banned := []string{"buyer", "recipient", "email", "phone", "mobile",
		"address", "name", "taxid", "carrier", "account"}
	// 出據人（賣家）那一側的欄位是合法的 —— 收據以賣家為出據人，
	// 他的名稱、統編與營業地址本來就要印上去。
	allowed := map[string]bool{
		"ReceiptSeller.LegalName": true,
		"ReceiptSeller.TaxID":     true,
		"ReceiptSeller.Address":   true,
		"ReceiptLine.Name":        true, // 品項名稱不是人的名字
	}

	var walk func(rt reflect.Type)
	walk = func(rt reflect.Type) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			qualified := rt.Name() + "." + f.Name
			if f.Type.Kind() == reflect.Struct && f.Type.PkgPath() == rt.PkgPath() {
				walk(f.Type)
				continue
			}
			if f.Type.Kind() == reflect.Slice &&
				f.Type.Elem().Kind() == reflect.Struct &&
				f.Type.Elem().PkgPath() == rt.PkgPath() {
				walk(f.Type.Elem())
				continue
			}
			if allowed[qualified] {
				continue
			}
			lower := strings.ToLower(f.Name)
			for _, b := range banned {
				if strings.Contains(lower, b) {
					t.Errorf("%s 看起來像買家個資欄位；收據上的買受人恆為「消費者」，"+
						"本服務也不持有買家真名。要加請先在 allowed 登記理由", qualified)
				}
			}
		}
	}
	walk(reflect.TypeOf(ReceiptData{}))
}

// 🔴 買受人恆為「消費者」：本服務不持有買家真名（§5.1.5，I-45 有測試守著）。
func TestReceiptNotice_IsExplicitAboutNotBeingAnInvoice(t *testing.T) {
	if !bytes.Contains([]byte(ReceiptNotInvoiceNotice), []byte("非統一發票")) {
		t.Fatal("那一行必須明說它不是統一發票——買家可能拿去報帳而被退件")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
