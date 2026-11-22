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
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func secureService(t *testing.T) (*Service, *db.DB, *storage.Fake) {
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
		DB: d, Jobs: jc, Store: st, Keyring: ring, NewID: uuid.NewV7, Now: func() time.Time { return now },
		Process: func(context.Context, ProcessInput) (ProcessResult, error) {
			return ProcessResult{}, errors.New("unused")
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return svc, d, st
}

// 最後的 transaction 回錯誤、其實已經提交：abortSecure 看到列是 READY，照成功回傳，而且不刪物件。
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
	got, err := svc.abortSecure(t.Context(), id, key, res, errors.New("commit：連線中斷"))
	if err != nil || got.Token != res.Token {
		t.Fatalf("abortSecure = %+v, %v，預期照成功回 token", got, err)
	}
	if _, err := st.Head(t.Context(), key); err != nil {
		t.Fatalf("物件被刪了：%v", err)
	}
	var state string
	if err := d.Pool().QueryRow(t.Context(), `SELECT state FROM media.staged_uploads WHERE id = $1`, id).Scan(&state); err != nil || state != "READY" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

// 列已經不在 UPLOADED 也不是 READY（例如已被 GC 標為 DELETING）：回原本的錯誤，不刪物件（物件的主人是 GC）。
func TestAbortSecure_RowNeitherUploadedNorReadyReturnsCauseAndKeepsObject(t *testing.T) {
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
	if _, err := svc.abortSecure(t.Context(), id, key, SecureResult{}, cause); !errors.Is(err, cause) {
		t.Fatalf("err = %v", err)
	}
	if _, err := st.Head(t.Context(), key); err != nil {
		t.Fatalf("物件被刪了：%v", err)
	}
}
