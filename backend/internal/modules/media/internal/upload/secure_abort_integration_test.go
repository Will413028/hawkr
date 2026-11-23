//go:build integration

package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/secure"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func secureService(t *testing.T) (*Service, *db.DB, *storage.Fake) {
	t.Helper()
	svc, d, st := secureServiceWith(t, func(f *storage.Fake) storage.Store { return f })
	return svc, d, st
}

// secureServiceWith 讓測試把 Fake 包一層（注入失敗、觀察 ctx 或在 Put 後改列）。
func secureServiceWith(t *testing.T, wrap func(*storage.Fake) storage.Store) (*Service, *db.DB, *storage.Fake) {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "media", Dir: os.DirFS("../../db/migrations")}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := secure.ParseKeyring("k1:AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=")
	if err != nil {
		t.Fatal(err)
	}
	st := storage.NewFake()
	now := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	svc := New(Deps{
		DB: d, Jobs: jc, Store: wrap(st), Keyring: ring, NewID: uuid.NewV7, Now: func() time.Time { return now },
		Process: func(context.Context, ProcessInput) (ProcessResult, error) {
			return ProcessResult{}, errors.New("unused")
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return svc, d, st
}

// 最後的 transaction 回錯誤、其實已經提交：abortDirect 看到列是 READY，照成功回傳，而且不刪物件。
func TestAbortDirect_CommittedDespiteErrorReturnsSuccessAndKeepsObject(t *testing.T) {
	svc, d, st := secureService(t)
	res, err := svc.PutPublic(t.Context(), model.Uploader{ID: 1001}, bytes.NewReader(tinyPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	var key string
	if err := d.Pool().QueryRow(t.Context(), `SELECT id, final_key FROM media.staged_uploads`).Scan(&id, &key); err != nil {
		t.Fatal(err)
	}
	if err := svc.abortDirect(t.Context(), id, key, errors.New("commit：連線中斷")); err != nil {
		t.Fatalf("abortDirect = %v，預期照成功回 nil（res.Token=%s）", err, res.Token)
	}
	if _, err := st.Head(t.Context(), key); err != nil {
		t.Fatalf("物件被刪了：%v", err)
	}
	var state string
	if err := d.Pool().QueryRow(t.Context(), `SELECT state FROM media.staged_uploads WHERE id = $1`, id).Scan(&state); err != nil || state != "READY" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

// 列已經被 GC 標為 DELETING（輸給 GC）：回 FailedPrecondition「已過期」，不刪物件（物件的主人是 GC）。
func TestAbortDirect_RowAlreadyDeletingReturnsExpiredAndKeepsObject(t *testing.T) {
	svc, d, st := secureService(t)
	if _, err := svc.PutPublic(t.Context(), model.Uploader{ID: 1001}, bytes.NewReader(tinyPNG(t))); err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	var key string
	if err := d.Pool().QueryRow(t.Context(), `SELECT id, final_key FROM media.staged_uploads`).Scan(&id, &key); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(t.Context(), `INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at) SELECT id, 'RECLAIMED', asset_id, now() FROM media.staged_uploads WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(t.Context(), `UPDATE media.staged_uploads SET state = 'DELETING' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("boom")
	if err := svc.abortDirect(t.Context(), id, key, cause); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("err = %v，預期 FailedPrecondition", err)
	}
	if _, err := st.Head(t.Context(), key); err != nil {
		t.Fatalf("物件被刪了：%v", err)
	}
}

// hookStore 包一層 Fake：afterPut 在 Put 成功後執行，putErr 讓 Put 失敗，Delete 記下收到的 ctx 是否有 deadline。
type hookStore struct {
	*storage.Fake
	afterPut    func()
	onPut       func()
	putErr      error
	deleteCalls int
	deleteHasDL bool
}

func (h *hookStore) Put(ctx context.Context, in storage.PutInput) error {
	if h.onPut != nil {
		h.onPut()
	}
	if h.putErr != nil {
		return h.putErr
	}
	if err := h.Fake.Put(ctx, in); err != nil {
		return err
	}
	if h.afterPut != nil {
		h.afterPut()
	}
	return nil
}

func (h *hookStore) Delete(ctx context.Context, key string) error {
	h.deleteCalls++
	_, h.deleteHasDL = ctx.Deadline()
	return h.Fake.Delete(ctx, key)
}

// 寫物件失敗時刪自己這次物件的 Delete 在 WithoutCancel 之後仍帶 deadline，儲存卡住時不會永久佔住名額；ctx 已取消也照樣收尾。
// 失敗的這一段不動會話：仍 UPLOADING、next_index 不變，client 可以重送同一段。
func TestPutSecureChunk_PutFailureCleanupRunsWithADeadlineEvenWhenTheRequestIsCancelled(t *testing.T) {
	var hs *hookStore
	svc, d, _ := secureServiceWith(t, func(f *storage.Fake) storage.Store {
		hs = &hookStore{Fake: f, putErr: errors.New("r2 掛了")}
		return hs
	})
	data := []byte("%PDF-1.7\nhello")
	b, err := svc.BeginSecureUpload(t.Context(), model.Uploader{ID: 1001}, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	hs.onPut = cancel // 暫存列建好之後、Put 時 request 才被取消
	if _, err := svc.PutSecureChunk(ctx, model.Uploader{ID: 1001}, b.Token, 0, bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("預期 Put 失敗的錯誤")
	}
	if hs.deleteCalls != 1 || !hs.deleteHasDL {
		t.Fatalf("Delete 呼叫 %d 次、ctx 有 deadline=%v", hs.deleteCalls, hs.deleteHasDL)
	}
	var state string
	var next int
	if err := d.Pool().QueryRow(t.Context(), `SELECT state, next_index FROM media.staged_uploads`).Scan(&state, &next); err != nil || state != "UPLOADING" || next != 0 {
		t.Fatalf("state=%s next_index=%d err=%v", state, next, err)
	}
}

// Put 之後暫存列被 GC 轉成 DELETING，推進影響 0 列：這一段回「已過期」，沒有資產列與段記錄，自己剛寫的物件被刪（物件不歸 GC，
// 因為它還沒記進 secure_chunks）。
func TestPutSecureChunk_LosingToGCReturnsExpiredAndDeletesItsOwnObject(t *testing.T) {
	var hs *hookStore
	var d *db.DB
	svc, d2, st := secureServiceWith(t, func(f *storage.Fake) storage.Store {
		hs = &hookStore{Fake: f}
		hs.afterPut = func() {
			if _, err := d.Pool().Exec(context.Background(), `INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at) SELECT id, 'RECLAIMED', NULL, now() FROM media.staged_uploads`); err != nil {
				panic(err)
			}
			if _, err := d.Pool().Exec(context.Background(), `UPDATE media.staged_uploads SET state = 'DELETING'`); err != nil {
				panic(err)
			}
		}
		return hs
	})
	d = d2
	data := []byte("%PDF-1.7\nhello")
	b, err := svc.BeginSecureUpload(t.Context(), model.Uploader{ID: 1001}, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PutSecureChunk(t.Context(), model.Uploader{ID: 1001}, b.Token, 0, bytes.NewReader(data), int64(len(data)))
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || errors.Is(err, errLost) {
		t.Fatalf("err = %v，預期 FailedPrecondition 且不外漏 errLost", err)
	}
	var assets, chunks int
	if err := d.Pool().QueryRow(t.Context(), `SELECT (SELECT count(*) FROM media.assets), (SELECT count(*) FROM media.secure_chunks)`).Scan(&assets, &chunks); err != nil || assets != 0 || chunks != 0 {
		t.Fatalf("資產列 %d、段記錄 %d err=%v，預期都是 0", assets, chunks, err)
	}
	if hs.deleteCalls != 1 || len(st.Keys()) != 0 {
		t.Fatalf("輸給 GC 時應刪掉自己的物件：Delete %d 次、剩下物件 %v", hs.deleteCalls, st.Keys())
	}
}

// countingBody 記下 body 被讀走的位元組數。
type countingBody struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// PutSecure 不整檔緩衝：寫第一段的物件時，body 只被讀走第一段的位元組。
func TestPutSecure_BuffersOneChunkAtATimeNotTheWholeBody(t *testing.T) {
	var hs *hookStore
	svc, _, _ := secureServiceWith(t, func(f *storage.Fake) storage.Store {
		hs = &hookStore{Fake: f}
		return hs
	})
	data := make([]byte, 2*model.SecureChunkSize+1)
	copy(data, "%PDF-1.7\n")
	body := &countingBody{r: bytes.NewReader(data)}
	var atFirstPut int64 = -1
	hs.onPut = func() {
		if atFirstPut < 0 {
			atFirstPut = body.n.Load()
		}
	}
	if _, err := svc.PutSecure(t.Context(), model.Uploader{ID: 1001}, body, int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if atFirstPut != model.SecureChunkSize {
		t.Fatalf("寫第一段時 body 已被讀走 %d 位元組，預期剛好 %d", atFirstPut, model.SecureChunkSize)
	}
}

// 推進的錯誤不是 errLost（commit 結果不明）時，不刪自己的物件，留給前綴 GC；errLost 才刪。
func TestSettleLostChunk_OnlyARollbackDeletesTheObject(t *testing.T) {
	svc, d, st := secureService(t)
	data := []byte("%PDF-1.7\nhello")
	b, err := svc.BeginSecureUpload(t.Context(), model.Uploader{ID: 1001}, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	_ = b
	var id uuid.UUID
	if err := d.Pool().QueryRow(t.Context(), `SELECT id FROM media.staged_uploads`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	row, err := svc.q(t.Context()).GetUpload(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	key := storage.NewSecureChunkKey(*row.PlannedAssetID, 0)
	put := func() {
		if err := st.Put(t.Context(), storage.PutInput{Key: key, Body: bytes.NewReader([]byte("c")), Size: 1, ContentType: "application/octet-stream"}); err != nil {
			t.Fatal(err)
		}
	}
	put()
	unknown := errors.New("commit：連線中斷")
	if err := svc.settleLostChunk(t.Context(), row, 0, key, unknown); !errors.Is(err, unknown) {
		t.Fatalf("err = %v，預期原樣回傳不明的錯誤", err)
	}
	if _, err := st.Head(t.Context(), key); err != nil {
		t.Fatalf("commit 結果不明時物件被刪了：%v", err)
	}
	if err := svc.settleLostChunk(t.Context(), row, 0, key, errLost); err == nil {
		t.Fatal("errLost 且會話仍開著，預期仍回錯誤")
	}
	if _, err := st.Head(t.Context(), key); err == nil {
		t.Fatal("確定 rollback 時物件應被刪掉")
	}
}

// RewrapDEKs 掃完上傳中的會話、掃資產之前，另一個還用舊主金鑰的行程完成了一次上傳：新資產仍要被這次重新包裹接到。
func TestRewrapDEKs_AssetCompletedBetweenThePhasesIsStillRewrapped(t *testing.T) {
	svc, d, st := secureService(t)
	ring2, err := secure.ParseKeyring("k2:AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI=,k1:AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=")
	if err != nil {
		t.Fatal(err)
	}
	deps := svc.d
	deps.Keyring = ring2
	rotated := New(deps)
	data := []byte("%PDF-1.7\nhello")
	rotated.afterRewrapUploads = func(ctx context.Context) {
		b, err := svc.BeginSecureUpload(ctx, model.Uploader{ID: 1002}, int64(len(data)))
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := svc.PutSecureChunk(ctx, model.Uploader{ID: 1002}, b.Token, 0, bytes.NewReader(data), int64(len(data))); err != nil {
			t.Error(err)
		}
	}
	if _, err := rotated.RewrapDEKs(t.Context()); err != nil {
		t.Fatal(err)
	}
	var total, stale int
	if err := d.Pool().QueryRow(t.Context(), `SELECT count(*), count(*) FILTER (WHERE kek_id <> 'k2') FROM media.assets`).Scan(&total, &stale); err != nil {
		t.Fatal(err)
	}
	if total != 1 || stale != 0 {
		t.Fatalf("資產 %d 筆、仍是舊 kek_id 的 %d 筆", total, stale)
	}
	_ = st
}
