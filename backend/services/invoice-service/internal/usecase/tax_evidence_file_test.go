package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 後台稅籍佐證下載的授權述詞：store 的稅務資格存在、該位置有佐證，才向 media 讀。
// 這組替身依 store／issuer 區分資料，無條件回同一份的替身會讓「store_id 有沒有被用」零鑑別力。

type taxEvidenceIssuers struct {
	domain.IssuerRepository
	issuerByStore map[int64]int64
}

func (f *taxEvidenceIssuers) GetByRef(_ context.Context, _ string, storeID int64) (domain.Issuer, error) {
	id, ok := f.issuerByStore[storeID]
	if !ok {
		return domain.Issuer{}, pkgerrors.NewBusiness(bizcode.InvoiceProviderAccountNotFound, "no issuer")
	}
	return domain.Issuer{ID: id}, nil
}

type taxEvidenceProfiles struct {
	domain.SellerTaxProfileRepository
	byIssuer map[int64]domain.SellerTaxProfile
}

func (f *taxEvidenceProfiles) GetByIssuer(_ context.Context, issuerID int64) (domain.SellerTaxProfile, error) {
	p, ok := f.byIssuer[issuerID]
	if !ok {
		return domain.SellerTaxProfile{}, pkgerrors.NewBusiness(bizcode.SellerTaxProfileNotFound, "no profile")
	}
	return p, nil
}

type taxEvidenceAudit struct {
	domain.AuditRepository
	entries []domain.NewAdminAuditLogEntry
	err     error
	// order 記錄稽核與 media 的先後，證明「先記錄、再讀內容」。
	order *[]string
}

func (f *taxEvidenceAudit) AppendAdminAuditLog(_ context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	if f.order != nil {
		*f.order = append(*f.order, "audit")
	}
	if f.err != nil {
		return 0, f.err
	}
	f.entries = append(f.entries, in)
	return int64(len(f.entries)), nil
}

type taxEvidenceMedia struct {
	order *[]string
	// allowed 的 key 是 "objectKey|profileID"，與 media 的授權三元組同形。
	allowed map[string]bool
	calls   []string
	err     error
}

func (f *taxEvidenceMedia) ReadTaxEvidence(_ context.Context, objectKey string, profileID int64) ([]byte, string, string, error) {
	if f.order != nil {
		*f.order = append(*f.order, "media")
	}
	k := fmt.Sprintf("%s|%d", objectKey, profileID)
	f.calls = append(f.calls, k)
	if f.err != nil {
		return nil, "", "", f.err
	}
	if !f.allowed[k] {
		return nil, "", "", media.ErrAssetNotFound
	}
	return []byte("bytes:" + objectKey), "image/png", "證明.png", nil
}

func profileWithKeys(id int64, keys ...string) domain.SellerTaxProfile {
	raw, _ := json.Marshal(keys)
	return domain.SellerTaxProfile{ID: id, EvidenceObjectKeys: raw}
}

const testAdmin = int64(8800)

func newTaxEvidenceFixture() (*TaxEvidenceFile, *taxEvidenceMedia, *taxEvidenceAudit) {
	issuers := &taxEvidenceIssuers{issuerByStore: map[int64]int64{100: 10, 200: 20, 300: 30}}
	profiles := &taxEvidenceProfiles{byIssuer: map[int64]domain.SellerTaxProfile{
		10: profileWithKeys(5000, "secure/certification/a.png", "secure/certification/b.png"),
		20: profileWithKeys(6000, "secure/certification/c.png"),
		30: profileWithKeys(7000),
	}}
	m := &taxEvidenceMedia{allowed: map[string]bool{
		"secure/certification/a.png|5000": true,
		"secure/certification/b.png|5000": true,
		"secure/certification/c.png|6000": true,
	}}
	order := &[]string{}
	m.order = order
	a := &taxEvidenceAudit{order: order}
	return NewTaxEvidenceFile(issuers, profiles, a, m, zap.NewNop()), m, a
}

func TestTaxEvidenceFile_ReadsDocumentAtPosition(t *testing.T) {
	uc, m, _ := newTaxEvidenceFixture()
	for _, tc := range []struct {
		store, doc int64
		wantCall   string
		wantData   string
	}{
		{100, 1, "secure/certification/a.png|5000", "bytes:secure/certification/a.png"},
		{100, 2, "secure/certification/b.png|5000", "bytes:secure/certification/b.png"},
		{200, 1, "secure/certification/c.png|6000", "bytes:secure/certification/c.png"},
	} {
		m.calls = nil
		got, err := uc.Read(context.Background(), testAdmin, tc.store, tc.doc)
		if err != nil {
			t.Fatalf("store=%d doc=%d: %v", tc.store, tc.doc, err)
		}
		if string(got.Data) != tc.wantData || got.ContentType != "image/png" || got.FileName != "證明.png" {
			t.Errorf("store=%d doc=%d: got %q %q %q", tc.store, tc.doc, got.Data, got.ContentType, got.FileName)
		}
		if len(m.calls) != 1 || m.calls[0] != tc.wantCall {
			t.Errorf("store=%d doc=%d: media 呼叫 %v, want [%s]", tc.store, tc.doc, m.calls, tc.wantCall)
		}
	}
}

// 位置語意與 GetTaxReview 的 document_id 同源：同一份 profile 取 EvidenceKeys 的 i+1。
func TestTaxEvidenceFile_PositionMatchesEvidenceKeysOrder(t *testing.T) {
	uc, m, _ := newTaxEvidenceFixture()
	keys := profileWithKeys(5000, "secure/certification/a.png", "secure/certification/b.png").EvidenceKeys()
	for i, k := range keys {
		m.calls = nil
		if _, err := uc.Read(context.Background(), testAdmin, 100, int64(i+1)); err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf("%s|5000", k); m.calls[0] != want {
			t.Errorf("document_id=%d 讀到 %s, want %s", i+1, m.calls[0], want)
		}
	}
}

func TestTaxEvidenceFile_NotFoundCellsNeverReachMedia(t *testing.T) {
	uc, m, a := newTaxEvidenceFixture()
	for _, tc := range []struct {
		name       string
		store, doc int64
	}{
		{"別的 store 的位置超出範圍", 200, 2},
		{"位置 0", 100, 0},
		{"負數位置", 100, -1},
		{"超出範圍", 100, 3},
		{"未知 store", 999, 1},
		{"store 0", 0, 1},
		{"有 issuer 但沒有佐證", 300, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m.calls = nil
			_, err := uc.Read(context.Background(), testAdmin, tc.store, tc.doc)
			if !isBizCode(err, bizcode.InvoiceDocumentNotFound) {
				t.Fatalf("err = %v, want 9301", err)
			}
			if len(m.calls) != 0 {
				t.Errorf("media 不該被呼叫：%v", m.calls)
			}
			if len(a.entries) != 0 {
				t.Errorf("查無不該留存取紀錄：%d 筆", len(a.entries))
			}
		})
	}
}

// 兩個 store 各自的第 1 份必須對到各自 profile 的 key 與 id，不得串用。
func TestTaxEvidenceFile_StoreScopesTheProfile(t *testing.T) {
	uc, m, _ := newTaxEvidenceFixture()
	if _, err := uc.Read(context.Background(), testAdmin, 200, 1); err != nil {
		t.Fatal(err)
	}
	if m.calls[0] != "secure/certification/c.png|6000" {
		t.Errorf("store 200 讀到 %s", m.calls[0])
	}
}

func TestTaxEvidenceFile_MediaErrors(t *testing.T) {
	uc, m, _ := newTaxEvidenceFixture()
	m.allowed = map[string]bool{}
	_, err := uc.Read(context.Background(), testAdmin, 100, 1)
	if !isBizCode(err, bizcode.InvoiceDocumentNotFound) {
		t.Errorf("media NotFound 應折成 9301，got %v", err)
	}

	boom := errors.New("media down")
	m.err = boom
	_, err = uc.Read(context.Background(), testAdmin, 100, 1)
	if !errors.Is(err, boom) || isBizCode(err, bizcode.InvoiceDocumentNotFound) {
		t.Errorf("故障不得折成 9301，got %v", err)
	}
}

func TestTaxEvidenceFile_RecordsAccessBeforeReading(t *testing.T) {
	uc, m, a := newTaxEvidenceFixture()
	if _, err := uc.Read(context.Background(), testAdmin, 100, 2); err != nil {
		t.Fatal(err)
	}
	if len(a.entries) != 1 {
		t.Fatalf("稽核列 %d 筆, want 1", len(a.entries))
	}
	e := a.entries[0]
	if e.ActorAccountID != testAdmin || e.Action != "INVOICE_PII_REVEAL" ||
		e.ResourceType != "INVOICE_TAX_EVIDENCE" || e.ResourceID != 5000 || e.RequestID != nil {
		t.Errorf("稽核列 = %+v", e)
	}
	if !strings.Contains(e.ReasonDetail, "position=2") || !strings.Contains(e.ReasonDetail, "secure/certification/b.png") {
		t.Errorf("reason_detail = %q", e.ReasonDetail)
	}
	if got := *m.order; len(got) != 2 || got[0] != "audit" || got[1] != "media" {
		t.Errorf("順序 = %v, want [audit media]", got)
	}
}

func TestTaxEvidenceFile_AuditFailureBlocksBytes(t *testing.T) {
	uc, m, a := newTaxEvidenceFixture()
	a.err = errors.New("db down")
	_, err := uc.Read(context.Background(), testAdmin, 100, 1)
	if err == nil || isBizCode(err, bizcode.InvoiceDocumentNotFound) {
		t.Fatalf("稽核失敗應回非 9301 的錯誤（503 路徑），got %v", err)
	}
	if len(m.calls) != 0 {
		t.Errorf("稽核失敗時 media 不得被呼叫：%v", m.calls)
	}
}

func TestTaxEvidenceFile_MissingActorIsRefusedWithoutAuditOrMedia(t *testing.T) {
	uc, m, a := newTaxEvidenceFixture()
	for _, actor := range []int64{0, -1} {
		_, err := uc.Read(context.Background(), actor, 100, 1)
		if !isBizCode(err, bizcode.InvoiceDocumentNotFound) {
			t.Errorf("actor=%d: err = %v, want 9301", actor, err)
		}
	}
	if len(m.calls) != 0 || len(a.entries) != 0 {
		t.Errorf("media=%v audit=%d", m.calls, len(a.entries))
	}
}
