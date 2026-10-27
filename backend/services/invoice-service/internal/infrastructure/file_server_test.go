package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

// TestContentDispositionEncodesNonASCII 釘住 backend#3149 之後的新前提：
// 這裡的檔名**不再由我方產生**，A4 解鎖證據走的是使用者上傳的原始檔名，
// 而 ADM 操作者上傳的取消證據幾乎必然是中文。
//
// 期望值一律硬寫字面值——不呼叫 pkg/httputil 的編碼函式求期望，
// 否則那兩支寫錯時測試會跟著錯。
func TestContentDispositionEncodesNonASCII(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		fileName string
		want     string
	}{
		{
			// 5 個中文字 = 15 bytes ⇒ 15 個底線（按「字數」算會寫成 9 個，那是錯的）
			name:     "純中文檔名：filename 退化成底線，真正的名字在 filename*",
			fileName: "作廢申請書.pdf",
			want:     `attachment; filename="_______________.pdf"; filename*=UTF-8''%E4%BD%9C%E5%BB%A2%E7%94%B3%E8%AB%8B%E6%9B%B8.pdf`,
		},
		{
			name:     "純 ASCII：兩個參數都是原名",
			fileName: "invoice-AA10003101.pdf",
			want:     `attachment; filename="invoice-AA10003101.pdf"; filename*=UTF-8''invoice-AA10003101.pdf`,
		},
		{
			name:     "中英混合",
			fileName: "INV58R6-證明.pdf",
			want:     `attachment; filename="INV58R6-______.pdf"; filename*=UTF-8''INV58R6-%E8%AD%89%E6%98%8E.pdf`,
		},
		{
			name:     "引號與反斜線先被 sanitizeFileName 剝掉，不會提前收尾",
			fileName: `a"b\c.pdf`,
			want:     `attachment; filename="abc.pdf"; filename*=UTF-8''abc.pdf`,
		},
		{
			// 空格是 ASCII 0x20 ⇒ 不換底線；quoted-string 內的空格本來就合法
			name:     "CR/LF 被剝掉，不構成 header injection",
			fileName: "evil\r\nX-Injected: 1.pdf",
			want:     `attachment; filename="evilX-Injected: 1.pdf"; filename*=UTF-8''evilX-Injected%3A%201.pdf`,
		},
		{
			name:     "sanitize 後為空 → 退回預設名",
			fileName: `"""`,
			want:     `attachment; filename="invoice.pdf"; filename*=UTF-8''invoice.pdf`,
		},
		{
			name:     "disposition=inline 時 kind 換掉，編碼規則不變",
			query:    "?disposition=inline",
			fileName: "預覽.pdf",
			want:     `inline; filename="______.pdf"; filename*=UTF-8''%E9%A0%90%E8%A6%BD.pdf`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/x"+tc.query, nil)
			if got := contentDisposition(r, tc.fileName); got != tc.want {
				t.Errorf("contentDisposition()\n got = %s\nwant = %s", got, tc.want)
			}
		})
	}
}

type taxEvidenceFakeIssuers struct {
	domain.IssuerRepository
}

func (taxEvidenceFakeIssuers) GetByRef(_ context.Context, _ string, storeID int64) (domain.Issuer, error) {
	if storeID != 100 {
		return domain.Issuer{}, pkgerrors.NewBusiness(bizcode.InvoiceProviderAccountNotFound, "no issuer")
	}
	return domain.Issuer{ID: 10}, nil
}

type taxEvidenceFakeProfiles struct {
	domain.SellerTaxProfileRepository
}

func (taxEvidenceFakeProfiles) GetByIssuer(_ context.Context, _ int64) (domain.SellerTaxProfile, error) {
	raw, _ := json.Marshal([]string{"secure/certification/a.png"})
	return domain.SellerTaxProfile{ID: 5000, EvidenceObjectKeys: raw}, nil
}

type taxEvidenceFakeAudit struct {
	domain.AuditRepository
	actors []int64
}

func (f *taxEvidenceFakeAudit) AppendAdminAuditLog(_ context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	f.actors = append(f.actors, in.ActorAccountID)
	return 1, nil
}

type taxEvidenceFakeMedia struct {
	calls int
	err   error
}

func (f *taxEvidenceFakeMedia) ReadTaxEvidence(_ context.Context, key string, profileID int64) ([]byte, string, string, error) {
	f.calls++
	if f.err != nil {
		return nil, "", "", f.err
	}
	if key != "secure/certification/a.png" || profileID != 5000 {
		return nil, "", "", media.ErrAssetNotFound
	}
	return []byte("PNGDATA"), "image/png", "證明.png", nil
}

func serveTaxEvidenceFor(t *testing.T, m *taxEvidenceFakeMedia, audit *taxEvidenceFakeAudit, path string, actor string) *httptest.ResponseRecorder {
	t.Helper()
	uc := usecase.NewTaxEvidenceFile(taxEvidenceFakeIssuers{}, taxEvidenceFakeProfiles{}, audit, m, zap.NewNop())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/invoices/tax_reviews/{store_id}/documents/{document_id}/content",
		serveTaxEvidence(FileServerParams{TaxEvidence: uc, Logger: zap.NewNop()}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if actor != "" {
		req.Header.Set("x-account-id", actor)
	}
	mux.ServeHTTP(rec, req)
	return rec
}

func TestServeTaxEvidence(t *testing.T) {
	base := "/admin/invoices/tax_reviews/"
	t.Run("成功：bytes 與 header", func(t *testing.T) {
		m := &taxEvidenceFakeMedia{}
		aud := &taxEvidenceFakeAudit{}
		rec := serveTaxEvidenceFor(t, m, aud, base+"100/documents/1/content?disposition=inline", "8800")
		if len(aud.actors) != 1 || aud.actors[0] != 8800 {
			t.Errorf("稽核 actor = %v, want [8800]（x-account-id）", aud.actors)
		}
		if rec.Code != http.StatusOK || rec.Body.String() != "PNGDATA" {
			t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("headers = %v", rec.Header())
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline; ") {
			t.Errorf("Content-Disposition = %q", got)
		}
	})
	for _, tc := range []struct{ name, path string }{
		{"別的 store", "200/documents/1/content"},
		{"位置 0", "100/documents/0/content"},
		{"超出範圍", "100/documents/2/content"},
		{"document_id 非數字", "100/documents/abc/content"},
		{"store_id 非數字", "abc/documents/1/content"},
	} {
		t.Run(tc.name+" → 404 且不呼叫 media", func(t *testing.T) {
			m := &taxEvidenceFakeMedia{}
			rec := serveTaxEvidenceFor(t, m, &taxEvidenceFakeAudit{}, base+tc.path, "8800")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("code = %d", rec.Code)
			}
			if m.calls != 0 {
				t.Errorf("media 被呼叫 %d 次", m.calls)
			}
		})
	}
	t.Run("沒有 x-account-id → 404、不稽核、不呼叫 media", func(t *testing.T) {
		m := &taxEvidenceFakeMedia{}
		aud := &taxEvidenceFakeAudit{}
		rec := serveTaxEvidenceFor(t, m, aud, base+"100/documents/1/content", "")
		if rec.Code != http.StatusNotFound || m.calls != 0 || len(aud.actors) != 0 {
			t.Errorf("code=%d media=%d audit=%v", rec.Code, m.calls, aud.actors)
		}
	})
	t.Run("media NotFound → 既有的 404 出口", func(t *testing.T) {
		m := &taxEvidenceFakeMedia{err: media.ErrAssetNotFound}
		rec := serveTaxEvidenceFor(t, m, &taxEvidenceFakeAudit{}, base+"100/documents/1/content", "8800")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":9301`) {
			t.Errorf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("media 故障 → 503 不折成 404", func(t *testing.T) {
		m := &taxEvidenceFakeMedia{err: errors.New("down")}
		rec := serveTaxEvidenceFor(t, m, &taxEvidenceFakeAudit{}, base+"100/documents/1/content", "8800")
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("code = %d", rec.Code)
		}
	})
}
