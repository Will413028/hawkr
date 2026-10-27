package domain

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
)

// 法定期限是**自然日**、末日 23:59:59 Asia/Taipei、假日不延長。
//
// 這一支的鑑別力在「48–72 小時」那兩列：把算式寫成 invoice_date + 48h 的話，
// 00:00 收款那一列會早算約 24 小時，而 23:59 那一列剛好對——只驗一個時點會全綠。
func TestStatutoryDeadline_B2CFallsBetween48And72Hours(t *testing.T) {
	cases := []struct {
		name    string
		paidUTC time.Time
		wantH   float64
	}{
		// 台北 2026-08-30 00:00:00 ＝ UTC 2026-08-29 16:00:00
		{"收款在台北當日 00:00（最寬鬆）", time.Date(2026, 8, 29, 16, 0, 0, 0, time.UTC), 72},
		// 台北 2026-08-30 23:59:59
		{"收款在台北當日 23:59:59（最保守）", time.Date(2026, 8, 30, 15, 59, 59, 0, time.UTC), 48},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := InvoiceDateTaipei(tc.paidUTC)
			got, ok := StatutoryDeadline(d, codemeta.InvoiceDocumentKindB2c.Value)
			if !ok {
				t.Fatal("B2C 必須有法定期限")
			}
			hours := got.Sub(tc.paidUTC).Hours()
			if hours < tc.wantH-1 || hours > tc.wantH+1 {
				t.Fatalf("剩餘 %.2f 小時，want ≈ %.0f（deadline=%v）", hours, tc.wantH, got)
			}
			lt := got.In(timeutil.LocationTaipei)
			if lt.Hour() != 23 || lt.Minute() != 59 || lt.Second() != 59 {
				t.Errorf("末日不是 23:59:59 Asia/Taipei：%v", lt)
			}
		})
	}
}

func TestStatutoryDeadline_KindDrivesDaysAndPresence(t *testing.T) {
	d := InvoiceDateTaipei(time.Date(2026, 8, 29, 16, 0, 0, 0, time.UTC)) // 台北 08-30

	b2c, ok := StatutoryDeadline(d, codemeta.InvoiceDocumentKindB2c.Value)
	if !ok {
		t.Fatal("B2C 應有期限")
	}
	b2b, ok := StatutoryDeadline(d, codemeta.InvoiceDocumentKindB2b.Value)
	if !ok {
		t.Fatal("B2B 應有期限")
	}
	if diff := b2b.Sub(b2c); diff != 5*24*time.Hour {
		t.Errorf("B2B 與 B2C 相差 %v，want 5 個自然日（7 − 2）", diff)
	}
	// chk_invdoc_deadline_presence 的另一半：RECEIPT 無法定期限。
	if _, ok := StatutoryDeadline(d, codemeta.InvoiceDocumentKindReceipt.Value); ok {
		t.Error("RECEIPT 不得有法定期限，否則 chk_invdoc_deadline_presence 當場 23514")
	}
}

// 跨月／跨年不得因為「同月加天數」而算錯（time.Date 會自動進位，這一支釘住它）。
func TestStatutoryDeadline_CrossesMonthAndYear(t *testing.T) {
	// 台北 2026-12-31
	d := InvoiceDateTaipei(time.Date(2026, 12, 30, 16, 0, 0, 0, time.UTC))
	got, ok := StatutoryDeadline(d, codemeta.InvoiceDocumentKindB2b.Value)
	if !ok {
		t.Fatal("B2B 應有期限")
	}
	lt := got.In(timeutil.LocationTaipei)
	if lt.Year() != 2027 || lt.Month() != time.January || lt.Day() != 7 {
		t.Fatalf("跨年算錯：%v", lt)
	}
}
