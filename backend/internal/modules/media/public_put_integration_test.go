//go:build integration

package media_test

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // 測試用的 Content-MD5
	"encoding/base64"
	"errors"
	stdimage "image"
	"image/png"
	"runtime"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPutPublic_ValidImageIsConfirmableAndBecomesActivePublic(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatalf("PutPublic：%v", err)
	}
	if res.ContentType != "image/png" || res.SizeBytes <= 0 || res.Token == "" {
		t.Fatalf("結果不對：%+v", res)
	}
	a, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatalf("Confirm：%v", err)
	}
	if a.Visibility != media.VisibilityPublic || a.ContentType != "image/png" || a.SizeBytes != res.SizeBytes {
		t.Fatalf("確認的資產不對：%+v", a)
	}
	if st := assetState(t, f, a.ID); st != "ACTIVE" {
		t.Fatalf("資產狀態 = %s，預期 ACTIVE", st)
	}
	if got := objectKeyOf(t, f, a.ID); !f.store.has(got) {
		t.Fatalf("公開物件 %s 不在儲存裡", got)
	}
}

func TestPutPublic_RejectsPDFAndFakeImagesWithoutObjectsOrRows(t *testing.T) {
	cases := map[string][]byte{
		"pdf":      []byte("%PDF-1.7\nsecret"),
		"fake-png": append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte("not an image")...),
		"empty":    nil,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			_, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(data))
			wantKind(t, err, apperr.KindInvalidInput)
			noWrites(t, f)
			if n := assetCount(t, f); n != 0 {
				t.Fatalf("建了 %d 筆資產", n)
			}
		})
	}
}

func TestPutPublic_OtherUploaderCannotConfirm(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = confirmInTx(f, other, res.Token)
	wantKind(t, err, apperr.KindNotFound)
}

func TestPutPublic_RefusedInsideATransaction(t *testing.T) {
	f := setup(t)
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		_, err := f.m.PutPublic(ctx, owner, bytes.NewReader(pngBytes(t)))
		return err
	})
	if !errors.Is(err, media.ErrInTransaction) {
		t.Fatalf("err = %v，預期 ErrInTransaction", err)
	}
	noWrites(t, f)
}

func TestPutSecure_PDFIsEncryptedAndReadsBackAfterConfirm(t *testing.T) {
	f := setup(t)
	data := pdfOf(5000)
	res, err := f.m.PutSecure(t.Context(), owner, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("PutSecure：%v", err)
	}
	if res.ContentType != "application/pdf" || res.SizeBytes != int64(len(data)) {
		t.Fatalf("結果不對：%+v", res)
	}
	for _, p := range f.store.puts {
		if bytes.Contains(p, []byte(secretMarker)) {
			t.Fatal("儲存收到了明文")
		}
	}
	for _, k := range f.store.Keys() {
		rc, err := f.store.Get(t.Context(), k)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(readAll(t, rc), []byte(secretMarker)) {
			t.Fatalf("物件 %s 含明文", k)
		}
	}
	a := confirmSecure(t, f, owner, res.Token)
	rc, err := f.m.ReadSecure(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, data) {
		t.Fatal("讀回的內容與原始位元組不同")
	}
}

func TestPutSecure_UnrecognizedContentIsRejected(t *testing.T) {
	f := setup(t)
	_, err := f.m.PutSecure(t.Context(), owner, bytes.NewReader([]byte("just some text")), 14)
	wantKind(t, err, apperr.KindInvalidInput)
	noObjects(t, f)
}

func TestPutSecure_OtherUploaderCannotConfirm(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutSecure(t.Context(), owner, bytes.NewReader(pdfOf(100)), int64(len(pdfOf(100))))
	if err != nil {
		t.Fatal(err)
	}
	_, err = confirmInTx(f, other, res.Token)
	wantKind(t, err, apperr.KindNotFound)
}

func holdSecureSlots(t *testing.T, f *fixture) (release func(), wait func()) {
	t.Helper()
	started := make(chan struct{}, media.SecureUploadConcurrency)
	rel := make(chan struct{})
	var wg sync.WaitGroup
	for range media.SecureUploadConcurrency {
		wg.Add(1)
		prefix := pdfOf(10)
		b := beginSecure(t, f, owner, len(prefix))
		go func() {
			defer wg.Done()
			if _, err := f.m.PutSecureChunk(t.Context(), owner, b.Token, 0, &gate{prefix: prefix, started: started, release: rel}, int64(len(prefix))); err != nil {
				t.Errorf("佔著名額的段應該成功：%v", err)
			}
		}()
	}
	for range media.SecureUploadConcurrency {
		<-started
	}
	return func() { close(rel) }, wg.Wait
}

// waitSignalCtx 在名額等待分支（upload.acquire）開始等 ctx.Done() 時發出訊號；資料庫呼叫也會讀 Done()，不算，所以只認呼叫堆疊裡有 acquire 的那次。
type waitSignalCtx struct {
	context.Context
	waiting chan struct{}
}

func (c *waitSignalCtx) Done() <-chan struct{} {
	if calledFromAcquire() {
		select {
		case c.waiting <- struct{}{}:
		default:
		}
	}
	return c.Context.Done()
}

func calledFromAcquire() bool {
	pcs := make([]uintptr, 32)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		fr, more := frames.Next()
		if strings.HasSuffix(fr.Function, "upload.acquire") {
			return true
		}
		if !more {
			return false
		}
	}
}

func TestPutSecure_WaitsForASlotInsteadOfFailingFast(t *testing.T) {
	f := setup(t)
	release, wait := holdSecureSlots(t, f)
	waiting := &waitSignalCtx{Context: t.Context(), waiting: make(chan struct{}, 1)}
	done := make(chan error, 1)
	go func() {
		_, err := f.m.PutSecure(waiting, owner, bytes.NewReader(pdfOf(10)), int64(len(pdfOf(10))))
		done <- err
	}()
	select {
	case <-waiting.waiting:
	case err := <-done:
		release()
		wait()
		t.Fatalf("名額全滿時 PutSecure 不該回傳：%v", err)
	}
	release()
	wait()
	if err := <-done; err != nil {
		t.Fatalf("PutSecure 應該等到名額再成功：%v", err)
	}
}

func TestPutSecure_FullSlotsReturnTheCtxErrorWhenCancelled(t *testing.T) {
	f := setup(t)
	release, wait := holdSecureSlots(t, f)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := f.m.PutSecure(ctx, owner, bytes.NewReader(pdfOf(10)), int64(len(pdfOf(10))))
	release()
	wait()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，預期 ctx 的錯誤，不是 ResourceExhausted", err)
	}
}

func TestPutPublic_InvalidUploaderIsRejectedWithoutWrites(t *testing.T) {
	f := setup(t)
	_, err := f.m.PutPublic(t.Context(), media.Uploader{}, bytes.NewReader(pngBytes(t)))
	wantKind(t, err, apperr.KindInvalidInput)
	noWrites(t, f)
}

func TestPutPublic_StagedRowDescribesTheStoredObject(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	var size int64
	var ct, finalKey, md5b64 string
	if err := f.d.Pool().QueryRow(t.Context(),
		`SELECT declared_size, declared_content_type, final_key, declared_md5 FROM media.staged_uploads`).Scan(&size, &ct, &finalKey, &md5b64); err != nil {
		t.Fatal(err)
	}
	stored := f.store.puts[finalKey]
	sum := md5.Sum(stored) //nolint:gosec // 與列上的 declared_md5 比對，不是安全用途
	if size != int64(len(stored)) || size != res.SizeBytes || ct != res.ContentType || md5b64 != base64.StdEncoding.EncodeToString(sum[:]) {
		t.Fatalf("暫存列與寫入的物件不一致：size=%d type=%s md5=%s，物件 %d 位元組", size, ct, md5b64, len(stored))
	}
}

func TestCopyPublic_ProducesAnIndependentConfirmableAssetAndLeavesTheSourceUntouched(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	cp, err := f.m.CopyPublic(t.Context(), other, src.ID)
	if err != nil {
		t.Fatalf("CopyPublic：%v", err)
	}
	if cp.Token == "" || cp.Token == res.Token || cp.ContentType != src.ContentType || cp.SizeBytes != src.SizeBytes {
		t.Fatalf("複製結果不對：%+v 對來源 %+v", cp, src)
	}
	if _, err := confirmInTx(f, owner, cp.Token); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別人的 token 不能確認複本：%v", err)
	}
	dup, err := confirmInTx(f, other, cp.Token)
	if err != nil {
		t.Fatalf("Confirm 複本：%v", err)
	}
	if dup.ID == src.ID || dup.ObjectKey == src.ObjectKey || dup.Visibility != media.VisibilityPublic {
		t.Fatalf("複本要是獨立的公開資產：%+v 對 %+v", dup, src)
	}
	if !f.store.has(dup.ObjectKey) || !f.store.has(src.ObjectKey) {
		t.Fatal("來源與複本的物件都要在儲存裡")
	}
	if err := f.d.InTx(t.Context(), func(ctx context.Context) error { return f.m.MarkDeleted(ctx, []uuid.UUID{src.ID}) }); err != nil {
		t.Fatal(err)
	}
	if st := assetState(t, f, dup.ID); st != "ACTIVE" {
		t.Fatalf("來源被刪除後複本狀態 = %s，預期 ACTIVE", st)
	}
	applyDeletions(t, f)
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if f.store.has(src.ObjectKey) || !f.store.has(dup.ObjectKey) {
		t.Fatal("實體刪除只能刪來源的物件，複本的物件要留著")
	}
}

func TestCopyPublic_RefusesSourcesThatAreNotActivePublicAssets(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CopyPublic(t.Context(), owner, uuid.NewV7()); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的來源：%v", err)
	}
	if err := f.d.InTx(t.Context(), func(ctx context.Context) error { return f.m.MarkDeleted(ctx, []uuid.UUID{src.ID}) }); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CopyPublic(t.Context(), owner, src.ID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("已標記刪除的來源：%v", err)
	}
	staged, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	var stagedAsset uuid.UUID
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT asset_id FROM media.staged_uploads WHERE token_hash = sha256($1::bytea)`, []byte(staged.Token)).Scan(&stagedAsset); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CopyPublic(t.Context(), owner, stagedAsset); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("尚未確認的來源：%v", err)
	}
}

func TestCopyPublic_AnUnconfirmedCopyIsReclaimedByGC(t *testing.T) {
	f := setup(t)
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CopyPublic(t.Context(), owner, src.ID); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(media.UploadTTL + media.StuckUploadGrace + 1)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := assetCount(t, f); n != 2 {
		t.Fatalf("資產列數 = %d，預期 2（來源與已刪的複本墓碑）", n)
	}
	if !f.store.has(src.ObjectKey) {
		t.Fatal("GC 不能刪來源的物件")
	}
}
