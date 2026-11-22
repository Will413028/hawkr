//go:build integration

package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
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
func TestAbortSecure_CommittedDespiteErrorReturnsSuccessAndKeepsObject(t *testing.T) {
	svc, d, st := secureService(t)
	res, err := svc.UploadSecure(t.Context(), model.Uploader{ID: 1001}, bytes.NewReader([]byte("%PDF-1.7\nhello")))
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
func TestAbortSecure_RowAlreadyDeletingReturnsExpiredAndKeepsObject(t *testing.T) {
	svc, d, st := secureService(t)
	if _, err := svc.UploadSecure(t.Context(), model.Uploader{ID: 1001}, bytes.NewReader([]byte("%PDF-1.7\nhello"))); err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	var key string
	if err := d.Pool().QueryRow(t.Context(), `SELECT id, final_key FROM media.staged_uploads`).Scan(&id, &key); err != nil {
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

// 收尾的 Delete 在 WithoutCancel 之後仍帶 deadline，儲存卡住時不會永久佔住名額；ctx 已取消也照樣收尾。
func TestUploadSecure_AbortDeleteRunsWithADeadlineEvenWhenTheRequestIsCancelled(t *testing.T) {
	var hs *hookStore
	svc, d, _ := secureServiceWith(t, func(f *storage.Fake) storage.Store {
		hs = &hookStore{Fake: f, putErr: errors.New("r2 掛了")}
		return hs
	})
	ctx, cancel := context.WithCancel(t.Context())
	hs.onPut = cancel // 暫存列建好之後、Put 時 request 才被取消
	if _, err := svc.UploadSecure(ctx, model.Uploader{ID: 1001}, bytes.NewReader([]byte("%PDF-1.7\nhello"))); err == nil {
		t.Fatal("預期 Put 失敗的錯誤")
	}
	if hs.deleteCalls != 1 || !hs.deleteHasDL {
		t.Fatalf("Delete 呼叫 %d 次、ctx 有 deadline=%v", hs.deleteCalls, hs.deleteHasDL)
	}
	var state string
	if err := d.Pool().QueryRow(t.Context(), `SELECT state FROM media.staged_uploads`).Scan(&state); err != nil || state != "REJECTED" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

// Put 之後暫存列被 GC 轉成 DELETING，最後的 transaction 的 MarkUploadReady 影響 0 列：UploadSecure 回「已過期」，沒有資產列，物件留給 GC。
func TestUploadSecure_LastTransactionLosesToGCReturnsExpiredAndLeavesNoAsset(t *testing.T) {
	var hs *hookStore
	var d *db.DB
	svc, d2, st := secureServiceWith(t, func(f *storage.Fake) storage.Store {
		hs = &hookStore{Fake: f}
		hs.afterPut = func() {
			if _, err := d.Pool().Exec(context.Background(), `UPDATE media.staged_uploads SET state = 'DELETING'`); err != nil {
				panic(err)
			}
		}
		return hs
	})
	d = d2
	_, err := svc.UploadSecure(t.Context(), model.Uploader{ID: 1001}, bytes.NewReader([]byte("%PDF-1.7\nhello")))
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || errors.Is(err, errLost) {
		t.Fatalf("err = %v，預期 FailedPrecondition 且不外漏 errLost", err)
	}
	var assets int
	if err := d.Pool().QueryRow(t.Context(), `SELECT count(*) FROM media.assets`).Scan(&assets); err != nil || assets != 0 {
		t.Fatalf("資產列 %d err=%v，預期 0", assets, err)
	}
	if hs.deleteCalls != 0 || len(st.Keys()) != 1 {
		t.Fatalf("輸給 GC 時不該刪物件：Delete %d 次、物件 %v", hs.deleteCalls, st.Keys())
	}
}
