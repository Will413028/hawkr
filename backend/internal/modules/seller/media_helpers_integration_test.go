//go:build integration

package seller_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	goimage "image"
	"image/png"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func testKeyring(t *testing.T) *media.Keyring {
	t.Helper()
	ring, err := media.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	must(t, err)
	return ring
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	must(t, png.Encode(&body, goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))))
	return body.Bytes()
}

// newMedia 是真的 media 模組（記憶體物件儲存）：Confirm 與 MarkDeleted 與擁有者的寫入同一個 transaction，回滾時一起回滾。
func (f *fixture) newMedia(t *testing.T) *mediaProbe {
	t.Helper()
	return &mediaProbe{Module: media.New(media.Deps{
		DB: f.db, Jobs: f.jobs, Store: media.NewMemoryStore(), Keyring: testKeyring(t), NewID: uuid.NewV7,
		Now: func() time.Time { return f.now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})}
}

func (f *fixture) assetIDOfToken(t *testing.T, token string) uuid.UUID {
	t.Helper()
	var assetID uuid.UUID
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT asset_id FROM media.staged_uploads WHERE token_hash = sha256($1::bytea)", token).Scan(&assetID))
	return assetID
}

// stagePublic 以 uploader 放一張公開 PNG，回傳可 Confirm 的 token 與資產 ID。
func (f *fixture) stagePublic(t *testing.T, uploader media.Uploader) (string, uuid.UUID) {
	t.Helper()
	put, err := f.media.PutPublic(context.Background(), uploader, bytes.NewReader(pngBytes(t)))
	must(t, err)
	return put.Token, f.assetIDOfToken(t, put.Token)
}

// logoToken 以 accountID 放一張公開圖片，回傳可當 LogoToken 的 token。
func (f *fixture) logoToken(t *testing.T, accountID int64) string {
	t.Helper()
	token, _ := f.stagePublic(t, media.Uploader{ID: accountID})
	return token
}

// stageSecure 以 uploader 放一份機敏檔（data 要是 PDF 或圖片），回傳可 Confirm 的 token 與資產 ID。
func (f *fixture) stageSecure(t *testing.T, uploader media.Uploader, data []byte) (string, uuid.UUID) {
	t.Helper()
	put, err := f.media.PutSecure(context.Background(), uploader, bytes.NewReader(data), int64(len(data)))
	must(t, err)
	return put.Token, f.assetIDOfToken(t, put.Token)
}

func (f *fixture) assetState(t *testing.T, assetID uuid.UUID) string {
	t.Helper()
	var state string
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT state FROM media.assets WHERE id=$1", assetID).Scan(&state))
	return state
}

// wantAsset 檢查資產的狀態：ACTIVE（已確認）、DELETING（已標記刪除）或 STAGED（還沒確認）。
func (f *fixture) wantAsset(t *testing.T, assetID uuid.UUID, want string) {
	t.Helper()
	if got := f.assetState(t, assetID); got != want {
		t.Fatalf("資產 %s state=%s，預期 %s", assetID, got, want)
	}
}

func (f *fixture) objectKey(t *testing.T, assetID uuid.UUID) string {
	t.Helper()
	var key string
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT object_key FROM media.assets WHERE id=$1", assetID).Scan(&key))
	return key
}

type storeAssetIDs struct{ logo, cover *uuid.UUID }

// storeAssets 讀店家列目前指著的 logo 與封面資產。
func (f *fixture) storeAssets(t *testing.T, storeID int64) storeAssetIDs {
	t.Helper()
	var out storeAssetIDs
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT logo_asset_id, cover_asset_id FROM seller.stores WHERE store_id=$1", storeID).Scan(&out.logo, &out.cover))
	return out
}

// mediaProbe 是真的 media 模組外面一層：數 ReadSecure 的呼叫，並可以注入失敗（media 說不是機敏資產、media 掛了）。
type mediaProbe struct {
	*media.Module
	mu          sync.Mutex
	reads       int
	failRead    error
	failConfirm map[string]error
}

// Confirm 對 failConfirm 裡的 token 回注入的錯誤（media 的內部錯誤），其餘交給真的模組。
func (p *mediaProbe) Confirm(ctx context.Context, uploader media.Uploader, token string) (media.ConfirmedAsset, error) {
	p.mu.Lock()
	fail := p.failConfirm[token]
	p.mu.Unlock()
	if fail != nil {
		return media.ConfirmedAsset{}, fail
	}
	return p.Module.Confirm(ctx, uploader, token)
}

func (p *mediaProbe) setFailConfirm(token string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failConfirm == nil {
		p.failConfirm = map[string]error{}
	}
	p.failConfirm[token] = err
}

func (p *mediaProbe) ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	p.mu.Lock()
	p.reads++
	fail := p.failRead
	p.mu.Unlock()
	if fail != nil {
		return nil, fail
	}
	return p.Module.ReadSecure(ctx, assetID)
}

func (p *mediaProbe) setFailRead(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failRead = err
}

func (f *fixture) readSecureCalls() int {
	f.media.mu.Lock()
	defer f.media.mu.Unlock()
	return f.media.reads
}

// stageDocs 以 accountID 為上傳者放 n 份機敏的 PDF（內容各不同），回傳 token 與資產 ID。
func (f *fixture) stageDocs(t *testing.T, accountID int64, n int) ([]string, []uuid.UUID) {
	t.Helper()
	tokens, assets := make([]string, n), make([]uuid.UUID, n)
	for i := range n {
		tokens[i], assets[i] = f.stageSecure(t, media.Uploader{ID: accountID}, docBytes(i))
	}
	return tokens, assets
}

// docBytes 是第 i 份補充文件的內容（開頭是 PDF 的 magic bytes，media 才收）。
func docBytes(i int) []byte {
	return []byte(fmt.Sprintf("%%PDF-1.4 document %d", i))
}

// documentAssets 是申請目前的補充文件的資產 ID（依位置）。
func (f *fixture) documentAssets(t *testing.T, applicationID string) []uuid.UUID {
	t.Helper()
	rows, err := f.db.Conn(context.Background()).Query(context.Background(), "SELECT asset_id FROM seller.kyc_documents WHERE application_id = $1::uuid ORDER BY position", applicationID)
	must(t, err)
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		must(t, rows.Scan(&id))
		out = append(out, id)
	}
	must(t, rows.Err())
	return out
}

// evidencePDF 是名叫 name 的佐證的內容（PDF 的 magic bytes 開頭，media 才收）。
func evidencePDF(name string) []byte { return []byte("%PDF-1.4 " + name) }

// evidence 以 uploader 放一組機敏佐證（內容是 evidencePDF(name)），回傳可 Confirm 的 token。
func (f *fixture) evidence(t *testing.T, uploader media.Uploader, names ...string) []string {
	t.Helper()
	tokens := make([]string, len(names))
	for i, name := range names {
		tokens[i], _ = f.stageSecure(t, uploader, evidencePDF(name))
	}
	return tokens
}
