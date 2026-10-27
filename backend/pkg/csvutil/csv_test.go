package csvutil

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteCSV_HasBOMAndContent(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, []string{"name", "amount"}, [][]string{{"蘋果", "100"}}); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.Bytes()
	if !bytes.HasPrefix(out, utf8BOM) {
		t.Fatal("expected output to start with UTF-8 BOM")
	}
	if !strings.Contains(string(out), "蘋果") {
		t.Fatal("expected row content to be present")
	}
}

func TestWriteCSV_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, []string{"name"}, nil); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), utf8BOM) {
		t.Fatal("expected output to start with UTF-8 BOM even with no rows")
	}
}

func TestEscapeField(t *testing.T) {
	cases := map[string]string{
		"=SUM(A1:A2)": "\t=SUM(A1:A2)",
		"+1234":       "\t+1234",
		"-100":        "\t-100",
		"@example":    "\t@example",
		"normal":      "normal",
		"":            "",
	}
	for in, want := range cases {
		if got := EscapeField(in); got != want {
			t.Errorf("EscapeField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetDownloadHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	SetDownloadHeaders(rec, "export.csv")
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	// RFC 6266 雙參數：filename= 給舊 client，filename*= 才是真正的檔名。
	// 期望值硬寫，不呼叫 httputil.ContentDisposition 求期望。
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="export.csv"; filename*=UTF-8''export.csv` {
		t.Errorf("Content-Disposition = %q", got)
	}
}

// TestSetDownloadHeadersCJK 是這個改動存在的理由：本 package 的 doc comment 說 BOM 是為了
// 讓 Excel 正確辨識 CJK **內容**，但在此之前 CJK **檔名**進 header 會是裸 UTF-8——
// 同一個問題只解了一半。
func TestSetDownloadHeadersCJK(t *testing.T) {
	rec := httptest.NewRecorder()
	SetDownloadHeaders(rec, "訂單匯出.csv")
	// 4 個中文字 = 12 bytes ⇒ filename= 退化成 12 個底線（按「字數」算會寫成 4 個，那是錯的）
	want := `attachment; filename="____________.csv"; filename*=UTF-8''%E8%A8%82%E5%96%AE%E5%8C%AF%E5%87%BA.csv`
	if got := rec.Header().Get("Content-Disposition"); got != want {
		t.Errorf("Content-Disposition\n got = %s\nwant = %s", got, want)
	}
}

func TestWriteResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := WriteResponse(rec, "export.csv", []string{"name"}, [][]string{{"test"}}); err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), utf8BOM) {
		t.Fatal("expected body to start with UTF-8 BOM")
	}
}
