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

	"github.com/jackc/pgx/v5"

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

func (f *fixture) stagePublic(t *testing.T, uploader media.Uploader) (string, uuid.UUID) {
	t.Helper()
	put, err := f.media.PutPublic(context.Background(), uploader, bytes.NewReader(pngBytes(t)))
	must(t, err)
	return put.Token, f.assetIDOfToken(t, put.Token)
}

func (f *fixture) logoToken(t *testing.T, accountID int64) string {
	t.Helper()
	token, _ := f.stagePublic(t, media.Uploader{ID: accountID})
	return token
}

func (f *fixture) stageSecure(t *testing.T, uploader media.Uploader, data []byte) (string, uuid.UUID) {
	t.Helper()
	put, err := f.media.PutSecure(context.Background(), uploader, bytes.NewReader(data), int64(len(data)))
	must(t, err)
	return put.Token, f.assetIDOfToken(t, put.Token)
}

func (f *fixture) assetState(t *testing.T, assetID uuid.UUID) string {
	t.Helper()
	st, err := media.AssetStateOf(context.Background(), f.db, assetID)
	must(t, err)
	return string(st)
}

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

func (f *fixture) storeAssets(t *testing.T, storeID int64) storeAssetIDs {
	t.Helper()
	var out storeAssetIDs
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT logo_asset_id, cover_asset_id FROM seller.stores WHERE store_id=$1", storeID).Scan(&out.logo, &out.cover))
	return out
}

type mediaProbe struct {
	*media.Module
	mu          sync.Mutex
	reads       int
	failRead    error
	failConfirm map[string]error
}

// Confirm 對 failConfirm 裡的 token 回注入的錯誤（media 的內部錯誤），其餘交給真的模組。
func (p *mediaProbe) Confirm(ctx context.Context, uploader media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error) {
	p.mu.Lock()
	fail := p.failConfirm[token]
	p.mu.Unlock()
	if fail != nil {
		return media.ConfirmedAsset{}, fail
	}
	return p.Module.Confirm(ctx, uploader, token, want)
}

func (p *mediaProbe) setFailConfirm(token string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failConfirm == nil {
		p.failConfirm = map[string]error{}
	}
	p.failConfirm[token] = err
}

func (p *mediaProbe) ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, string, error) {
	p.mu.Lock()
	p.reads++
	fail := p.failRead
	p.mu.Unlock()
	if fail != nil {
		return nil, "", fail
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

func (f *fixture) stageDocs(t *testing.T, accountID int64, n int) ([]string, []uuid.UUID) {
	t.Helper()
	tokens, assets := make([]string, n), make([]uuid.UUID, n)
	for i := range n {
		tokens[i], assets[i] = f.stageSecure(t, media.Uploader{ID: accountID}, docBytes(i))
	}
	return tokens, assets
}

func docBytes(i int) []byte {
	return []byte(fmt.Sprintf("%%PDF-1.4 document %d", i))
}

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

func evidencePDF(name string) []byte { return []byte("%PDF-1.4 " + name) }

func (f *fixture) evidence(t *testing.T, uploader media.Uploader, names ...string) []string {
	t.Helper()
	tokens := make([]string, len(names))
	for i, name := range names {
		tokens[i], _ = f.stageSecure(t, uploader, evidencePDF(name))
	}
	return tokens
}

func (f *fixture) activeEvidenceAssets(t *testing.T, inquiryID uuid.UUID) int {
	t.Helper()
	rows, err := f.db.Conn(context.Background()).Query(context.Background(), "SELECT asset_id FROM seller.case_inquiry_reply_evidence WHERE inquiry_id = $1", inquiryID)
	must(t, err)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	must(t, err)
	n := 0
	for _, id := range ids {
		if f.assetState(t, id) == string(media.AssetActive) {
			n++
		}
	}
	return n
}

func (f *fixture) markAssetDeleted(t *testing.T, ctx context.Context, id uuid.UUID) {
	t.Helper()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.media.MarkDeleted(ctx, []uuid.UUID{id}) }))
}
