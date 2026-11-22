//go:build integration

package media_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// kek 回傳 `<id>:<base64 的 32 位元組>`，內容由 seed 決定（測試不要隨機，失敗時可重現）。
func kek(id string, seed byte) string {
	return id + ":" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{seed}, 32))
}

func mustKeyring(t *testing.T, s string) *media.Keyring {
	t.Helper()
	k, err := media.ParseKeyring(s)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// withKeyring 建一個共用同一個資料庫與儲存、但金鑰環不同的 Module；ring 為 nil 時沒有金鑰環。
func (f *fixture) withKeyring(ring *media.Keyring) *media.Module {
	return media.New(media.Deps{
		DB: f.d, Jobs: f.jobs, Store: f.store, NewID: uuid.NewV7, Now: f.clock.Now, Keyring: ring,
	})
}

const secretMarker = "TOP-SECRET-PASSPORT-NUMBER-X1234567"

// pdfOf 回傳含明文標記、至少 n 位元組的假 PDF。
func pdfOf(n int) []byte {
	b := []byte("%PDF-1.7\n" + secretMarker + "\n")
	for len(b) < n {
		b = append(b, secretMarker...)
	}
	return b
}

var pngMagic = append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte(secretMarker)...)

func uploadSecure(t *testing.T, f *fixture, who media.Uploader, data []byte) media.SecureUploadResult {
	t.Helper()
	res, err := f.m.UploadSecure(t.Context(), who, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("UploadSecure：%v", err)
	}
	return res
}

// secureRow 取得 token 對應的暫存列與資產 ID（資產 ID 不回給上傳者，測試直接以 token_hash 查表，不靠排序）。
func secureRow(t *testing.T, f *fixture, token string) (uploadID, assetID uuid.UUID) {
	t.Helper()
	h := sha256.Sum256([]byte(token))
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT id, asset_id FROM media.staged_uploads WHERE token_hash = $1`, h[:]).Scan(&uploadID, &assetID); err != nil {
		t.Fatal(err)
	}
	return uploadID, assetID
}

func objectKeyOf(t *testing.T, f *fixture, assetID uuid.UUID) string {
	t.Helper()
	var k string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT object_key FROM media.assets WHERE id = $1`, assetID).Scan(&k); err != nil {
		t.Fatal(err)
	}
	return k
}

func readAll(t *testing.T, rc io.ReadCloser) []byte {
	t.Helper()
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("讀取：%v", err)
	}
	return b
}

func confirmSecure(t *testing.T, f *fixture, who media.Uploader, token string) media.ConfirmedAsset {
	t.Helper()
	a, err := confirmInTx(f, who, token)
	if err != nil {
		t.Fatalf("Confirm：%v", err)
	}
	return a
}

func noWrites(t *testing.T, f *fixture) {
	t.Helper()
	if len(f.store.puts) != 0 || len(f.store.Keys()) != 0 {
		t.Fatalf("被拒的輸入寫了物件：puts=%d keys=%v", len(f.store.puts), f.store.Keys())
	}
	var n int
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT count(*) FROM media.staged_uploads`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("被拒的輸入建了暫存列：%d %v", n, err)
	}
}

// ==================== UploadSecure ====================

func TestUploadSecure_RecordsKekIDAndWrappedDEK(t *testing.T) {
	f := setup(t)
	data := pdfOf(1000)
	res := uploadSecure(t, f, owner, data)
	if res.Token == "" || res.ContentType != "application/pdf" || res.SizeBytes != int64(len(data)) || !res.ExpiresAt.Equal(f.clock.Now().Add(media.UploadTTL)) {
		t.Fatalf("結果 %+v", res)
	}
	uploadID, assetID := secureRow(t, f, res.Token)
	var kekID *string
	var wrapped []byte
	var key, state, ct string
	var size int64
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT kek_id, wrapped_dek, object_key, state, content_type, size_bytes FROM media.assets WHERE id = $1`, assetID).Scan(&kekID, &wrapped, &key, &state, &ct, &size); err != nil {
		t.Fatal(err)
	}
	if kekID == nil || *kekID != "k1" || len(wrapped) == 0 || !strings.HasPrefix(key, "secure/") || state != "STAGED" || ct != "application/pdf" || size != int64(len(data)) {
		t.Fatalf("資產列 kek_id=%v wrapped=%d key=%s state=%s ct=%s size=%d", kekID, len(wrapped), key, state, ct, size)
	}
	var tmp, md5 *string
	var final string
	var declaredSize int64
	var declaredType string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT tmp_key, declared_md5, final_key, declared_size, declared_content_type FROM media.staged_uploads WHERE id = $1`, uploadID).Scan(&tmp, &md5, &final, &declaredSize, &declaredType); err != nil {
		t.Fatal(err)
	}
	if tmp != nil || md5 != nil || final != key || declaredSize != int64(len(data)) || declaredType != "application/pdf" {
		t.Fatalf("暫存列 tmp=%v md5=%v final=%s size=%d type=%s", tmp, md5, final, declaredSize, declaredType)
	}
	if got := uploadState(t, f, uploadID); got != "READY" {
		t.Fatalf("state = %s", got)
	}
	if got := f.store.Keys(); len(got) != 1 || got[0] != key {
		t.Fatalf("儲存裡的 key = %v，預期只有 %s", got, key)
	}
}

func TestUploadSecure_AcceptsImagesAndPDF(t *testing.T) {
	for name, c := range map[string]struct {
		data []byte
		want string
	}{
		"pdf":  {pdfOf(10), "application/pdf"},
		"png":  {pngMagic, "image/png"},
		"jpeg": {append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, secretMarker...), "image/jpeg"},
		"gif":  {append([]byte("GIF89a"), secretMarker...), "image/gif"},
		"webp": {append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), secretMarker...), "image/webp"},
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			res := uploadSecure(t, f, owner, c.data)
			if res.ContentType != c.want {
				t.Fatalf("型別 %s，預期 %s", res.ContentType, c.want)
			}
		})
	}
}

func TestUploadSecure_RejectsFakePDFAndHEICWithoutWrites(t *testing.T) {
	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic")...)
	for name, data := range map[string][]byte{
		"fake-pdf":   []byte("%PD this is not a pdf"),
		"plain-text": []byte("hello world"),
		"heic":       heic,
		"zip":        {'P', 'K', 3, 4, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			_, err := f.m.UploadSecure(t.Context(), owner, bytes.NewReader(data))
			wantKind(t, err, apperr.KindInvalidInput)
			noWrites(t, f)
		})
	}
}

func TestUploadSecure_RejectsEmptyBodyWithoutWrites(t *testing.T) {
	f := setup(t)
	_, err := f.m.UploadSecure(t.Context(), owner, bytes.NewReader(nil))
	wantKind(t, err, apperr.KindInvalidInput)
	noWrites(t, f)
}

// endless 產生以 prefix 開頭、永遠讀不完的內容。
type endless struct {
	prefix []byte
	n      int64
}

func (e *endless) Read(p []byte) (int, error) {
	for i := range p {
		if e.n < int64(len(e.prefix)) {
			p[i] = e.prefix[e.n]
		} else {
			p[i] = 'a'
		}
		e.n++
	}
	return len(p), nil
}

func TestUploadSecure_RejectsOverSizeWithoutWrites(t *testing.T) {
	f := setup(t)
	_, err := f.m.UploadSecure(t.Context(), owner, &endless{prefix: []byte("%PDF-1.7\n")})
	wantKind(t, err, apperr.KindInvalidInput)
	noWrites(t, f)
}

func TestUploadSecure_RefusesTransaction(t *testing.T) {
	f := setup(t)
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		_, err := f.m.UploadSecure(ctx, owner, bytes.NewReader(pdfOf(10)))
		return err
	})
	if !errors.Is(err, media.ErrInTransaction) {
		t.Fatalf("err = %v", err)
	}
	noWrites(t, f)
}

func TestUploadSecure_InvalidUploader(t *testing.T) {
	f := setup(t)
	_, err := f.m.UploadSecure(t.Context(), media.Uploader{}, bytes.NewReader(pdfOf(10)))
	wantKind(t, err, apperr.KindInvalidInput)
	noWrites(t, f)
}

// gate 在第一次被讀時通知 started，之後阻塞到 release 關閉，讓上傳停在「持有名額」的狀態而不靠 sleep。
type gate struct {
	prefix  []byte
	started chan<- struct{}
	release <-chan struct{}
	once    sync.Once
	sent    bool
}

func (g *gate) Read(p []byte) (int, error) {
	g.once.Do(func() { g.started <- struct{}{} })
	if !g.sent {
		g.sent = true
		return copy(p, g.prefix), nil
	}
	<-g.release
	return 0, io.EOF
}

func TestUploadSecure_FifthConcurrentUploadIsResourceExhausted(t *testing.T) {
	f := setup(t)
	started := make(chan struct{}, media.SecureUploadConcurrency)
	release := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, media.SecureUploadConcurrency)
	for range media.SecureUploadConcurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.m.UploadSecure(t.Context(), owner, &gate{prefix: pdfOf(10), started: started, release: release})
			errs <- err
		}()
	}
	for range media.SecureUploadConcurrency {
		<-started
	}
	_, err := f.m.UploadSecure(t.Context(), owner, bytes.NewReader(pdfOf(10)))
	wantKind(t, err, apperr.KindResourceExhausted)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("佔著名額的上傳應該成功：%v", err)
		}
	}
	// 名額放回後可以再上傳。
	uploadSecure(t, f, owner, pdfOf(10))
}

func TestUploadSecure_NoPlaintextInAnyPutOrStoredObject(t *testing.T) {
	f := setup(t)
	uploadSecure(t, f, owner, pdfOf(200_000))
	if len(f.store.puts) == 0 {
		t.Fatal("沒有 Put")
	}
	for key, body := range f.store.puts {
		if bytes.Contains(body, []byte(secretMarker)) || bytes.Contains(body, []byte("%PDF-")) {
			t.Fatalf("Put %s 含明文", key)
		}
		if !strings.HasPrefix(key, "secure/") {
			t.Fatalf("機敏上傳寫了 %s", key)
		}
	}
	if calls := f.store.callsInTx(); len(calls) != 0 {
		t.Fatalf("在 transaction 內呼叫儲存：%v", calls)
	}
}

// 對應 test-map 列 95：密文開銷依 B1-D4 的格式改寫（原名 TestConfirmSecure_PassesWithCipherOverhead）。
func TestUploadSecure_CiphertextOverheadIsOutsideTheRecordedPlainSize(t *testing.T) {
	f := setup(t)
	data := pdfOf(70_000) // 兩段
	res := uploadSecure(t, f, owner, data)
	_, assetID := secureRow(t, f, res.Token)
	info, err := f.store.Head(t.Context(), objectKeyOf(t, f, assetID))
	if err != nil {
		t.Fatal(err)
	}
	// header 16 位元組、每段 16 位元組 tag
	if want := int64(len(data)) + 16 + 2*16; info.Size != want {
		t.Fatalf("物件 %d 位元組，預期 %d", info.Size, want)
	}
	var size int64
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT size_bytes FROM media.assets WHERE id = $1`, assetID).Scan(&size); err != nil || size != int64(len(data)) {
		t.Fatalf("資產 size_bytes=%d err=%v，預期明文大小 %d", size, err, len(data))
	}
}

// 對應 test-map 列 97：機敏物件的實際型別一律是 octet-stream（原名 TestConfirmSecure_SkipsContentTypeCheck）。
func TestUploadSecure_ObjectTypeIsOctetStreamWhileAssetKeepsRecognisedType(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pngMagic)
	_, assetID := secureRow(t, f, res.Token)
	info, err := f.store.Head(t.Context(), objectKeyOf(t, f, assetID))
	if err != nil {
		t.Fatal(err)
	}
	if info.ContentType != "application/octet-stream" {
		t.Fatalf("物件型別 %s", info.ContentType)
	}
	a := confirmSecure(t, f, owner, res.Token)
	if a.ContentType != "image/png" || a.Visibility != media.VisibilitySecure {
		t.Fatalf("確認後的資產 %+v", a)
	}
}

// 對應 test-map 列 101（原名 TestPutSecure_RefusesWhenMasterKeyMissing）。
func TestUploadSecure_RefusesWhenKeyringMissing(t *testing.T) {
	f := setup(t)
	m := f.withKeyring(nil)
	ctx := t.Context()
	if _, err := m.UploadSecure(ctx, owner, bytes.NewReader(pdfOf(10))); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("UploadSecure err = %v", err)
	}
	if _, err := m.ReadSecure(ctx, uuid.New()); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("ReadSecure err = %v", err)
	}
	if _, err := m.ReadStagedSecure(ctx, owner, "x"); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("ReadStagedSecure err = %v", err)
	}
	if _, err := m.RewrapDEKs(ctx); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("RewrapDEKs err = %v", err)
	}
	noWrites(t, f)
}

func TestUploadSecure_PutFailureRejectsRowAndLeavesNoAssetOrObject(t *testing.T) {
	f := setup(t)
	f.store.mu.Lock()
	f.store.failPut = errors.New("r2 down")
	f.store.mu.Unlock()
	if _, err := f.m.UploadSecure(t.Context(), owner, bytes.NewReader(pdfOf(100))); err == nil {
		t.Fatal("Put 失敗卻成功")
	}
	var id uuid.UUID
	var state string
	var reason *string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT id, state, reject_reason FROM media.staged_uploads`).Scan(&id, &state, &reason); err != nil {
		t.Fatal(err)
	}
	if state != "REJECTED" || reason == nil || *reason != "PROCESSING_FAILED" {
		t.Fatalf("state=%s reason=%v", state, reason)
	}
	if assetCount(t, f) != 0 {
		t.Fatal("Put 失敗後有資產列")
	}
	if len(f.store.Keys()) != 0 {
		t.Fatalf("儲存裡殘留 %v", f.store.Keys())
	}
	// 沒有 token 回給 client，列又不是 READY：擁有者的 Confirm 不可能成功；GC 在過期後回收。
	f.clock.Advance(media.UploadTTL + time.Minute)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := uploadState(t, f, id); got != "DELETED" {
		t.Fatalf("GC 之後 state = %s", got)
	}
}

// insertStuckSecure 模擬行程在 Put 前後中斷：UPLOADED 的機敏列加上（可選的）secure/ 物件。
func insertStuckSecure(t *testing.T, f *fixture, withObject bool) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	key := "secure/" + uuid.New().String()
	now := f.clock.Now()
	if _, err := f.d.Pool().Exec(t.Context(), `INSERT INTO media.staged_uploads (id, token_hash, uploader_id, uploader_admin, visibility, declared_size, declared_content_type, state, final_key, expires_at, created_at, updated_at)
		VALUES ($1, $2, 1001, false, 'SECURE', 10, 'application/pdf', 'UPLOADED', $3, $4, $5, $5)`, id, bytes.Repeat([]byte{7}, 32), key, now.Add(media.UploadTTL), now); err != nil {
		t.Fatal(err)
	}
	if withObject {
		if err := f.store.Fake.Put(t.Context(), putInput(key, []byte("ciphertext"))); err != nil {
			t.Fatal(err)
		}
	}
	return id, key
}

func TestGC_ReclaimsStuckSecureUploadedAfterGraceWithoutTouchingTmp(t *testing.T) {
	f := setup(t)
	id, key := insertStuckSecure(t, f, true)
	f.clock.Advance(media.UploadTTL + media.StuckUploadGrace - time.Second)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if uploadState(t, f, id) != "UPLOADED" || !f.store.has(key) {
		t.Fatal("寬限內的機敏 UPLOADED 列被回收")
	}
	f.clock.Advance(2 * time.Second)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := uploadState(t, f, id); got != "DELETED" || f.store.has(key) {
		t.Fatalf("過寬限後 state=%s、物件仍在=%v", got, f.store.has(key))
	}
	for _, k := range f.store.deleted {
		if k == "" {
			t.Fatal("對 NULL 的 tmp_key 呼叫了儲存")
		}
	}
	if len(f.store.deleted) != 1 || f.store.deleted[0] != key {
		t.Fatalf("Delete 呼叫 %v，預期只有 %s", f.store.deleted, key)
	}
}

func TestProcess_SecureRowIsPermanentFailure(t *testing.T) {
	f := setup(t)
	id, _ := insertStuckSecure(t, f, false)
	m := f.withKeyring(f.keyring) // 內建的處理（NewProcessor），不是 fixture 的 copyProcess
	if err := m.ProcessUpload(t.Context(), id, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatalf("ProcessUpload：%v", err)
	}
	var state string
	var reason *string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT state, reject_reason FROM media.staged_uploads WHERE id = $1`, id).Scan(&state, &reason); err != nil {
		t.Fatal(err)
	}
	if state != "REJECTED" || reason == nil || *reason != "PROCESSING_FAILED" {
		t.Fatalf("state=%s reason=%v", state, reason)
	}
}

// ==================== ReadSecure ====================

// 對應 test-map 列 9、12（原名 TestReadSecure_ReturnsPlaintextForAsset、TestReadSecure_DecryptsStoredObject）。
func TestReadSecure_ReturnsPlaintextForAssetAcrossSegments(t *testing.T) {
	f := setup(t)
	data := pdfOf(200_000)
	res := uploadSecure(t, f, owner, data)
	a := confirmSecure(t, f, owner, res.Token)
	rc, err := f.m.ReadSecure(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, data) {
		t.Fatalf("讀回 %d 位元組，內容與上傳的不同（原 %d）", len(got), len(data))
	}
}

func TestReadSecure_NotReadableWhileStagedAndNotFoundForUnknown(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pdfOf(10))
	_, assetID := secureRow(t, f, res.Token)
	_, err := f.m.ReadSecure(t.Context(), assetID)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.m.ReadSecure(t.Context(), uuid.New())
	wantKind(t, err, apperr.KindNotFound)
}

func TestReadSecure_NotFoundAfterOwnerDeletes(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pdfOf(10))
	a := confirmSecure(t, f, owner, res.Token)
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.m.ReadSecure(t.Context(), a.ID)
	wantKind(t, err, apperr.KindNotFound)
}

// 對應 test-map 列 17（原名 TestReadSecure_RejectsPublicAsset）。
func TestReadSecure_RejectsPublicAsset(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	a := confirmSecure(t, f, owner, res.Token)
	if _, err := f.m.ReadSecure(t.Context(), a.ID); !errors.Is(err, media.ErrNotSecure) {
		t.Fatalf("err = %v，預期 ErrNotSecure", err)
	}
}

func TestReadSecure_RefusesTransaction(t *testing.T) {
	f := setup(t)
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		_, err := f.m.ReadSecure(ctx, uuid.New())
		return err
	})
	if !errors.Is(err, media.ErrInTransaction) {
		t.Fatalf("err = %v", err)
	}
}

// replaceObject 把資產的密文換成 mutate 之後的內容，模擬儲存層被竄改。
func replaceObject(t *testing.T, f *fixture, assetID uuid.UUID, mutate func([]byte) []byte) {
	t.Helper()
	key := objectKeyOf(t, f, assetID)
	rc, err := f.store.Get(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	ct := readAll(t, rc)
	ct = mutate(bytes.Clone(ct))
	if err := f.store.Fake.Put(t.Context(), putInput(key, ct)); err != nil {
		t.Fatal(err)
	}
}

func readFails(t *testing.T, f *fixture, assetID uuid.UUID) (delivered int) {
	t.Helper()
	rc, err := f.m.ReadSecure(t.Context(), assetID)
	if err != nil {
		return 0
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err == nil {
		t.Fatal("被竄改的密文讀得完")
	}
	return len(b)
}

// 對應 test-map 列 18、96（原名 TestReadSecure_TruncatedCiphertextFailsBeforeAnyPlaintext、TestConfirmSecure_RejectsMissingAuthTag）。
func TestReadSecure_TamperedTruncatedOrTaglessCiphertextFailsBeforeAnyPlaintext(t *testing.T) {
	cases := map[string]func([]byte) []byte{
		"truncated":       func(b []byte) []byte { return b[:len(b)-10] },
		"missing-tag":     func(b []byte) []byte { return b[:len(b)-16] },
		"flipped-first":   func(b []byte) []byte { b[20] ^= 1; return b },
		"trailing-noise":  func(b []byte) []byte { return append(b, 0) },
		"header-only":     func(b []byte) []byte { return b[:16] },
		"flipped-version": func(b []byte) []byte { b[4] ^= 1; return b },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			res := uploadSecure(t, f, owner, pdfOf(300)) // 單段：任何竄改都在第一段，之前不得交出明文
			a := confirmSecure(t, f, owner, res.Token)
			replaceObject(t, f, a.ID, mutate)
			if n := readFails(t, f, a.ID); n != 0 {
				t.Fatalf("錯誤之前交出 %d 位元組明文", n)
			}
		})
	}
}

func TestReadSecure_FailsWhenWrappedDEKMovedToAnotherAsset(t *testing.T) {
	f := setup(t)
	a1 := confirmSecure(t, f, owner, uploadSecure(t, f, owner, pdfOf(50)).Token)
	a2 := confirmSecure(t, f, owner, uploadSecure(t, f, owner, pdfOf(60)).Token)
	if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.assets SET wrapped_dek = (SELECT wrapped_dek FROM media.assets WHERE id = $1) WHERE id = $2`, a1.ID, a2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ReadSecure(t.Context(), a2.ID); err == nil {
		t.Fatal("wrapped DEK 換到別的資產 ID 仍可讀")
	}
}

func TestReadSecure_FailsForUnknownKekID(t *testing.T) {
	f := setup(t)
	a := confirmSecure(t, f, owner, uploadSecure(t, f, owner, pdfOf(50)).Token)
	if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.assets SET kek_id = 'gone' WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ReadSecure(t.Context(), a.ID); err == nil {
		t.Fatal("未知的 kek_id 仍可讀")
	}
}

// ==================== ReadStagedSecure ====================

func TestReadStagedSecure_OnlyTheUploaderCanPreviewBeforeConfirm(t *testing.T) {
	f := setup(t)
	data := pdfOf(5000)
	res := uploadSecure(t, f, owner, data)
	rc, err := f.m.ReadStagedSecure(t.Context(), owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, data) {
		t.Fatal("預覽的內容與上傳的不同")
	}
	_, err = f.m.ReadStagedSecure(t.Context(), other, res.Token)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.m.ReadStagedSecure(t.Context(), media.Uploader{ID: owner.ID, Admin: true}, res.Token)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.m.ReadStagedSecure(t.Context(), owner, "not-a-token")
	wantKind(t, err, apperr.KindNotFound)
}

func TestReadStagedSecure_RefusedAfterConfirmAndAfterExpiry(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pdfOf(100))
	confirmSecure(t, f, owner, res.Token)
	_, err := f.m.ReadStagedSecure(t.Context(), owner, res.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)

	res2 := uploadSecure(t, f, owner, pdfOf(100))
	f.clock.Advance(media.UploadTTL)
	_, err = f.m.ReadStagedSecure(t.Context(), owner, res2.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
}

func TestReadStagedSecure_RejectsPublicStaging(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	if _, err := f.m.ReadStagedSecure(t.Context(), owner, res.Token); !errors.Is(err, media.ErrNotSecure) {
		t.Fatalf("err = %v", err)
	}
}

// ==================== RewrapDEKs ====================

func TestRewrapDEKs_AfterRotationOldKekCanBeDropped(t *testing.T) {
	f := setup(t)
	old := confirmSecure(t, f, owner, uploadSecure(t, f, owner, pdfOf(200_000)).Token)
	oldKey := objectKeyOf(t, f, old.ID)
	rc, _ := f.store.Get(t.Context(), oldKey)
	before := readAll(t, rc)

	rotated := f.withKeyring(mustKeyring(t, kek("k2", 2)+","+kek("k1", 1)))
	// 換 primary 之後新上傳用 k2；它不該被重新包裹。
	fresh, err := rotated.UploadSecure(t.Context(), owner, bytes.NewReader(pdfOf(100)))
	if err != nil {
		t.Fatal(err)
	}
	_, freshAsset := secureRow(t, f, fresh.Token)
	var freshWrapped []byte
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT wrapped_dek FROM media.assets WHERE id = $1`, freshAsset).Scan(&freshWrapped); err != nil {
		t.Fatal(err)
	}

	f.store.puts = map[string][]byte{} // 之後任何 Put（含重寫同一個 key）都會留下紀錄
	n, err := rotated.RewrapDEKs(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("RewrapDEKs = %d, %v，預期只更新舊資產 1 筆", n, err)
	}
	var kekID string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT kek_id FROM media.assets WHERE id = $1`, old.ID).Scan(&kekID); err != nil || kekID != "k2" {
		t.Fatalf("kek_id=%s err=%v", kekID, err)
	}
	var freshAfter []byte
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT wrapped_dek FROM media.assets WHERE id = $1`, freshAsset).Scan(&freshAfter); err != nil || !bytes.Equal(freshWrapped, freshAfter) {
		t.Fatalf("primary 包裹的資產被動到 err=%v", err)
	}
	if n, err := rotated.RewrapDEKs(t.Context()); err != nil || n != 0 {
		t.Fatalf("重跑 = %d, %v，預期 0", n, err)
	}

	// 只留新 KEK 仍可讀，物件位元組不變，沒有任何 Put。
	onlyNew := f.withKeyring(mustKeyring(t, kek("k2", 2)))
	rc2, err := onlyNew.ReadSecure(t.Context(), old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc2); !bytes.Equal(got, pdfOf(200_000)) {
		t.Fatal("換 KEK 後內容不同")
	}
	rc3, _ := f.store.Get(t.Context(), oldKey)
	if after := readAll(t, rc3); !bytes.Equal(before, after) {
		t.Fatal("R2 物件的位元組變了")
	}
	if _, onlyOld := f.m.ReadSecure(t.Context(), old.ID); onlyOld == nil {
		t.Fatal("舊 KEK 不該再解得開 k2 包裹的 DEK")
	}
	if len(f.store.puts) != 0 {
		t.Fatalf("RewrapDEKs 寫了儲存：%d 個 key", len(f.store.puts))
	}
}

// 列出之後、更新之前列上的 kek_id 已被別方改掉：帶舊 kek_id 的更新影響 0 列，wrapped_dek 不被覆寫，資產仍可讀。
func TestRewrapAsset_StaleOldKekIDUpdatesNothingAndKeepsTheWrappedDEK(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pdfOf(1000))
	asset := confirmSecure(t, f, owner, res.Token)
	var before []byte
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT wrapped_dek FROM media.assets WHERE id = $1`, asset.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	stale, primary := "k0", "k2"
	n, err := store.New(f.d.Conn(t.Context())).RewrapAsset(t.Context(), store.RewrapAssetParams{
		ID: asset.ID, OldKekID: &stale, NewKekID: &primary, NewWrappedDek: []byte("overwritten"), Now: f.clock.Now(),
	})
	if err != nil || n != 0 {
		t.Fatalf("RewrapAsset = %d, %v，預期帶過期 kek_id 影響 0 列", n, err)
	}
	var after []byte
	var kekID string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT wrapped_dek, kek_id FROM media.assets WHERE id = $1`, asset.ID).Scan(&after, &kekID); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || kekID != "k1" {
		t.Fatalf("wrapped_dek 或 kek_id 被覆寫：kek_id=%s", kekID)
	}
	rc, err := f.m.ReadSecure(t.Context(), asset.ID)
	if err != nil {
		t.Fatalf("資產不再可讀：%v", err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, pdfOf(1000)) {
		t.Fatal("內容不同")
	}
}

func TestRewrapDEKs_SkipsDeletedAndReportsUnknownKek(t *testing.T) {
	f := setup(t)
	deleted := confirmSecure(t, f, owner, uploadSecure(t, f, owner, pdfOf(20)).Token)
	if err := markDeletedInTx(f, deleted.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	stranger := confirmSecure(t, f, owner, uploadSecure(t, f, owner, pdfOf(20)).Token)
	if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.assets SET kek_id = 'gone' WHERE id = $1`, stranger.ID); err != nil {
		t.Fatal(err)
	}
	rotated := f.withKeyring(mustKeyring(t, kek("k2", 2)+","+kek("k1", 1)))
	n, err := rotated.RewrapDEKs(t.Context())
	if err == nil || n != 0 {
		t.Fatalf("RewrapDEKs = %d, %v，預期 0 筆與未知 KEK 的錯誤；DELETED 的資產不該被處理", n, err)
	}
}

func TestParseKeyring_StartupFailsOnBadConfig(t *testing.T) {
	for _, s := range []string{"", "k1", "k1:AAAA", kek("k1", 1) + "," + kek("k1", 2)} {
		if _, err := media.ParseKeyring(s); err == nil {
			t.Errorf("%q 應該啟動失敗", s)
		}
	}
}

func putInput(key string, data []byte) storage.PutInput {
	return storage.PutInput{Key: key, Body: bytes.NewReader(data), Size: int64(len(data))}
}
