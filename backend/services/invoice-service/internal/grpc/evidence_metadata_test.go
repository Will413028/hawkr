package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

type fakeEvidenceMeta struct {
	byKey map[string]media.AssetMetadata
	err   error
	got   []string
	calls int
}

func (f *fakeEvidenceMeta) BatchGetEvidenceMetadata(
	_ context.Context, keys []string, _ int64,
) (map[string]media.AssetMetadata, error) {
	f.calls++
	f.got = keys
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]media.AssetMetadata{}
	for _, k := range keys {
		if m, ok := f.byKey[k]; ok {
			out[k] = m
		}
	}
	return out, nil
}

func evidenceProfile(keys ...string) domain.SellerTaxProfile {
	raw, _ := json.Marshal(keys)
	return domain.SellerTaxProfile{ID: 7001, EvidenceObjectKeys: raw}
}

// 🔴 file_asset_id 是 Gateway 組下載路徑的唯一輸入。缺它的症狀是
// 「看得到檔名、點不下去」——與 latest_job_id 缺失時是同一個形狀。
func TestToTaxDocuments_FillsMetadata(t *testing.T) {
	r := &fakeEvidenceMeta{byKey: map[string]media.AssetMetadata{
		"secure/tax/a.pdf": {AssetID: "882527259574290339", FileName: "a.pdf",
			SizeBytes: "10240", UploadedAt: "2026-09-01T07:28:15Z"},
	}}
	s := &InvoiceServer{evidenceMeta: r, log: zap.NewNop()}

	docs := s.toTaxDocuments(context.Background(),
		evidenceProfile("secure/tax/a.pdf", "secure/tax/missing.pdf"))

	if len(docs) != 2 {
		t.Fatalf("兩個 key 應回兩列，實得 %d", len(docs))
	}
	if docs[0].GetFileAssetId() != "882527259574290339" {
		t.Fatalf("🔴 file_asset_id 沒填：%q —— 下載路徑組不出來", docs[0].GetFileAssetId())
	}
	if docs[0].GetFileName() != "a.pdf" || docs[0].GetFileSize() != "10240" ||
		docs[0].GetUploadedAt() != "2026-09-01T07:28:15Z" {
		t.Fatalf("其餘中介資料沒填：%#v", docs[0])
	}
	// 🔴 media 查不到的那一份**仍然要出現在清單裡**（file_key 是本服務自己存的）：
	// 整列消失會讓賣家以為自己的佐證不見了，而那份檔案其實還在。
	if docs[1].GetFileKey() != "secure/tax/missing.pdf" {
		t.Fatalf("查不到中介資料的那一份不得從清單消失：%#v", docs[1])
	}
	if docs[1].GetFileAssetId() != "" {
		t.Fatalf("查不到就該留空，不得猜：%q", docs[1].GetFileAssetId())
	}
}

// 降級不中止查詢：退回「只有檔案路徑」正是修這件事之前的畫面，不會更差。
func TestResolveEvidenceMetadata_Degrades(t *testing.T) {
	s := &InvoiceServer{evidenceMeta: &fakeEvidenceMeta{err: errors.New("media down")}, log: zap.NewNop()}
	if got := s.resolveEvidenceMetadata(context.Background(), []string{"k"}, 7001); len(got) != 0 {
		t.Fatalf("失敗應回空 map，got=%#v", got)
	}
	s2 := &InvoiceServer{evidenceMeta: nil, log: zap.NewNop()}
	if got := s2.resolveEvidenceMetadata(context.Background(), []string{"k"}, 7001); len(got) != 0 {
		t.Fatalf("nil resolver 應回空 map")
	}
	// taxProfileID 為 0 ⇒ 授權述詞的 ref_id 組不出來，不得發出 RPC。
	r := &fakeEvidenceMeta{}
	s3 := &InvoiceServer{evidenceMeta: r, log: zap.NewNop()}
	s3.resolveEvidenceMetadata(context.Background(), []string{"k"}, 0)
	if r.calls != 0 {
		t.Fatal("taxProfileID=0 時不該發出 RPC（ref_id 是授權述詞的一半）")
	}
}

func TestToTaxDocuments_EmptyKeysSkipsCall(t *testing.T) {
	r := &fakeEvidenceMeta{}
	s := &InvoiceServer{evidenceMeta: r, log: zap.NewNop()}
	if docs := s.toTaxDocuments(context.Background(), evidenceProfile()); docs != nil {
		t.Fatalf("沒有佐證應回 nil，實得 %#v", docs)
	}
	if r.calls != 0 {
		t.Fatal("沒有 key 時不該發出 RPC")
	}
}
