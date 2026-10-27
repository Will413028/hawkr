package handler

import (
	"encoding/json"
	"testing"
)

func TestFillAdminTaxDocumentPaths(t *testing.T) {
	d := SellerTaxProfileData{Documents: []TaxDocumentInfo{{DocumentId: "1"}, {DocumentId: "2"}, {DocumentId: ""}}}
	fillAdminTaxDocumentPaths(&d, "123")
	if got, want := d.Documents[0].FileDownloadPath, "/api/v1/admin/invoices/tax_reviews/123/documents/1/content"; got != want {
		t.Errorf("doc 1 path = %q, want %q", got, want)
	}
	if got, want := d.Documents[1].FileDownloadPath, "/api/v1/admin/invoices/tax_reviews/123/documents/2/content"; got != want {
		t.Errorf("doc 2 path = %q, want %q", got, want)
	}
	if d.Documents[2].FileDownloadPath != "" {
		t.Errorf("沒有 documentId 不得組路徑：%q", d.Documents[2].FileDownloadPath)
	}
}

// 賣家回應不經 fillAdminTaxDocumentPaths：JSON 不得出現 fileDownloadPath。
func TestSellerTaxDocumentJSONHasNoDownloadPath(t *testing.T) {
	raw, err := json.Marshal(TaxDocumentInfo{DocumentId: "1", FileAssetId: "9"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if _, ok := m["fileDownloadPath"]; ok {
		t.Errorf("賣家 JSON 不該有 fileDownloadPath：%s", raw)
	}
}
