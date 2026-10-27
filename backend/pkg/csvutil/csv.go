// Package csvutil writes CSV data as an HTTP download in a form Excel opens
// correctly: UTF-8 with a byte-order mark, since Excel otherwise mis-detects
// the charset of non-ASCII (e.g. Chinese) content and shows garbled text.
package csvutil

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/httputil"
)

// utf8BOM makes Excel detect the file as UTF-8 instead of guessing a legacy
// charset (which garbles CJK text). Other CSV consumers (Go's encoding/csv,
// most other spreadsheet apps) already ignore or strip a leading BOM.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// riskyFormulaPrefixes are the leading characters spreadsheet apps (Excel,
// Google Sheets, LibreOffice) treat as the start of a formula. See
// EscapeField.
const riskyFormulaPrefixes = "=+-@"

// EscapeField neutralizes a field value against CSV/spreadsheet formula
// injection (OWASP CSV Injection): if s starts with a character a
// spreadsheet app would interpret as a formula prefix, a leading tab is
// added so it opens as plain text. Apply this to values sourced from user
// input (e.g. seller name, product name, review content) before passing
// them to WriteCSV/WriteResponse; it is not applied automatically because
// doing so unconditionally would also mangle legitimate numeric fields that
// start with '-' (e.g. a negative amount).
func EscapeField(s string) string {
	if s != "" && strings.ContainsRune(riskyFormulaPrefixes, rune(s[0])) {
		return "\t" + s
	}
	return s
}

// WriteCSV writes headers followed by rows as a UTF-8-BOM-prefixed CSV
// document to w.
func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	bw := bufio.NewWriter(w)
	if _, err := bw.Write(utf8BOM); err != nil {
		return fmt.Errorf("csvutil: write BOM: %w", err)
	}
	cw := csv.NewWriter(bw)
	if err := cw.Write(headers); err != nil {
		return fmt.Errorf("csvutil: write header: %w", err)
	}
	for i, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csvutil: write row %d: %w", i, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("csvutil: flush csv writer: %w", err)
	}
	return bw.Flush()
}

// SetDownloadHeaders sets the response headers so a browser saves the
// response body as a CSV file attachment named filename instead of
// rendering it inline.
func SetDownloadHeaders(w http.ResponseWriter, filename string) {
	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	// 走 httputil 的 RFC 6266 雙參數輸出：只給 filename= 的話中文檔名會亂碼
	// （%q 逃逸得了引號，但對非 ASCII 無能為力）。與 invoice-service 的
	// 檔案下載共用同一支，避免兩份編碼實作漂移。
	h.Set("Content-Disposition", httputil.ContentDisposition("attachment", filename, "download.csv"))
	h.Set("Cache-Control", "no-cache")
}

// WriteResponse sets CSV download headers on w, then streams headers and
// rows as a UTF-8-BOM-prefixed CSV body. It must be called before any other
// write to w, since setting headers after the body has started is a no-op
// on most http.ResponseWriter implementations (including Gin's).
func WriteResponse(w http.ResponseWriter, filename string, headers []string, rows [][]string) error {
	SetDownloadHeaders(w, filename)
	return WriteCSV(w, headers, rows)
}
