//go:build integration

package media_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// clock 是注入的時間；測試不看 wall-clock。
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// spyStore 記錄呼叫儲存時 ctx 有沒有帶 transaction，並可以讓指定 key 的刪除失敗。
type spyStore struct {
	*storage.Fake
	mu         sync.Mutex
	inTx       []string
	failDelete map[string]error
	failPut    error
	// puts 記錄每個 Put 收到的位元組（含失敗的），deleted 記錄每個 Delete 的 key。
	puts    map[string][]byte
	deleted []string
	// afterPut 在 Put 成功之後、回傳之前呼叫（讓測試在「物件已寫、尚未推進」的空檔插入並行請求）。
	afterPut func(key string)
}

func newSpyStore() *spyStore {
	return &spyStore{Fake: storage.NewFake(), failDelete: map[string]error{}, puts: map[string][]byte{}}
}

func (s *spyStore) note(ctx context.Context, op string) {
	if _, ok := db.TxFrom(ctx); ok {
		s.mu.Lock()
		s.inTx = append(s.inTx, op)
		s.mu.Unlock()
	}
}

func (s *spyStore) callsInTx() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.inTx...)
}

func (s *spyStore) PresignPut(ctx context.Context, in storage.PresignPutInput) (storage.PresignedPut, error) {
	s.note(ctx, "PresignPut")
	return s.Fake.PresignPut(ctx, in)
}

func (s *spyStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.note(ctx, "Get")
	return s.Fake.Get(ctx, key)
}

func (s *spyStore) Head(ctx context.Context, key string) (storage.ObjectInfo, error) {
	s.note(ctx, "Head")
	return s.Fake.Head(ctx, key)
}

func (s *spyStore) Put(ctx context.Context, in storage.PutInput) error {
	s.note(ctx, "Put")
	data, err := io.ReadAll(in.Body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.puts[in.Key] = data
	failPut := s.failPut
	s.mu.Unlock()
	if failPut != nil {
		return failPut
	}
	in.Body = bytes.NewReader(data)
	if err := s.Fake.Put(ctx, in); err != nil {
		return err
	}
	s.mu.Lock()
	hook := s.afterPut
	s.mu.Unlock()
	if hook != nil {
		hook(in.Key)
	}
	return nil
}

func (s *spyStore) setAfterPut(f func(key string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.afterPut = f
}

func (s *spyStore) List(ctx context.Context, prefix string) ([]string, error) {
	s.note(ctx, "List")
	return s.Fake.List(ctx, prefix)
}

func (s *spyStore) Delete(ctx context.Context, key string) error {
	s.note(ctx, "Delete")
	s.mu.Lock()
	s.deleted = append(s.deleted, key)
	err := s.failDelete[key]
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.Fake.Delete(ctx, key)
}

func (s *spyStore) setDeleteFailure(key string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		delete(s.failDelete, key)
		return
	}
	s.failDelete[key] = err
}

func (s *spyStore) has(key string) bool {
	_, err := s.Fake.Head(context.Background(), key)
	return err == nil
}

type fixture struct {
	keyring *media.Keyring
	t       *testing.T
	d       *db.DB
	jobs    *jobs.Client
	store   *spyStore
	clock   *clock
	m       *media.Module
	process media.ProcessFunc
}

var (
	owner = media.Uploader{ID: 1001}
	other = media.Uploader{ID: 1002}
)

func setup(t *testing.T) *fixture {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "media", Dir: media.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, d: d, jobs: jc, store: newSpyStore(), clock: newClock()}
	f.process = f.copyProcess
	f.keyring = mustKeyring(t, kek("k1", 1))
	f.m = f.module(f.clock)
	return f
}

// module 建另一個共用同一個資料庫與儲存、但時鐘不同的 Module（並發測試用）。
func (f *fixture) module(c *clock) *media.Module {
	return media.New(media.Deps{
		DB: f.d, Jobs: f.jobs, Store: f.store,
		Process: func(ctx context.Context, in media.ProcessInput) (media.ProcessResult, error) {
			return f.process(ctx, in)
		},
		NewID: uuid.NewV7, Now: c.Now, Keyring: f.keyring,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// copyProcess 是測試用的處理：把暫存物件原樣寫到正式 key。
func (f *fixture) copyProcess(ctx context.Context, in media.ProcessInput) (media.ProcessResult, error) {
	rc, err := f.store.Get(ctx, in.TmpKey)
	if errors.Is(err, storage.ErrNotFound) {
		return media.ProcessResult{}, media.Permanent(media.RejectSizeMismatch, err)
	}
	if err != nil {
		return media.ProcessResult{}, err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return media.ProcessResult{}, err
	}
	if err := f.store.Put(ctx, storage.PutInput{Key: in.FinalKey, Body: bytes.NewReader(data), Size: int64(len(data)), ContentType: in.DeclaredContentType}); err != nil {
		return media.ProcessResult{}, err
	}
	return media.ProcessResult{SizeBytes: int64(len(data)), ContentType: in.DeclaredContentType}, nil
}

var payload = []byte("fake image bytes")

// declaredMD5 是任意 16 位元組的 base64：fake 儲存與處理函式不驗 MD5（驗證在 storage 的 S3 測試）。
var declaredMD5 = base64.StdEncoding.EncodeToString(make([]byte, 16))

func begin(t *testing.T, f *fixture, who media.Uploader) media.BeginResult {
	t.Helper()
	res, err := f.m.BeginUpload(t.Context(), who, media.BeginInput{
		Size: int64(len(payload)), ContentType: "image/png", MD5: declaredMD5,
	})
	if err != nil {
		t.Fatalf("BeginUpload：%v", err)
	}
	return res
}

func tmpKeyOf(res media.BeginResult) string {
	return strings.TrimPrefix(res.URL, "https://fake-storage.invalid/")
}

// clientUpload 模擬 client 直傳到預簽網址。
func clientUpload(t *testing.T, f *fixture, res media.BeginResult) {
	t.Helper()
	if err := f.store.Fake.Put(t.Context(), storage.PutInput{Key: tmpKeyOf(res), Body: bytes.NewReader(payload), Size: int64(len(payload)), ContentType: "image/png"}); err != nil {
		t.Fatal(err)
	}
}

func complete(t *testing.T, f *fixture, who media.Uploader, res media.BeginResult) {
	t.Helper()
	if err := f.m.CompleteUpload(t.Context(), who, res.Token); err != nil {
		t.Fatalf("CompleteUpload：%v", err)
	}
}

// uploaded 做到 UPLOADED（處理 job 還沒跑）。
func uploaded(t *testing.T, f *fixture, who media.Uploader) media.BeginResult {
	t.Helper()
	res := begin(t, f, who)
	clientUpload(t, f, res)
	complete(t, f, who, res)
	return res
}

// ready 做到 READY：處理 job 成功。
func ready(t *testing.T, f *fixture, who media.Uploader) media.BeginResult {
	t.Helper()
	res := uploaded(t, f, who)
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatalf("ProcessUpload：%v", err)
	}
	return res
}

func confirmInTx(f *fixture, who media.Uploader, token string) (media.ConfirmedAsset, error) {
	var a media.ConfirmedAsset
	err := f.d.InTx(f.t.Context(), func(ctx context.Context) error {
		var err error
		a, err = f.m.Confirm(ctx, who, token)
		return err
	})
	return a, err
}

func uploadState(t *testing.T, f *fixture, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT state FROM media.staged_uploads WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func uploadKeys(t *testing.T, f *fixture, id uuid.UUID) (tmp string, final *string, asset *uuid.UUID) {
	t.Helper()
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT tmp_key, final_key, asset_id FROM media.staged_uploads WHERE id = $1`, id).Scan(&tmp, &final, &asset); err != nil {
		t.Fatal(err)
	}
	return tmp, final, asset
}

func assetState(t *testing.T, f *fixture, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT state FROM media.assets WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func assetCount(t *testing.T, f *fixture) int {
	t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT count(*) FROM media.assets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func jobCount(t *testing.T, f *fixture, kind string) int {
	t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT count(*) FROM river.river_job WHERE kind = $1`, kind).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// assertNoDanglingAssets：沒有任何 STAGED 或 ACTIVE 的資產指著不存在的物件。
func assertNoDanglingAssets(t *testing.T, f *fixture) {
	t.Helper()
	rows, err := f.d.Pool().Query(t.Context(), `SELECT object_key FROM media.assets WHERE state IN ('STAGED', 'ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		if !f.store.has(key) {
			t.Errorf("有效的資產指著不存在的物件 %s", key)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func wantKind(t *testing.T, err error, want apperr.Kind) {
	t.Helper()
	if err == nil || apperr.KindOf(err) != want {
		t.Fatalf("err = %v（分類 %v），預期分類 %v", err, apperr.KindOf(err), want)
	}
}

func assetIDOf(t *testing.T, f *fixture, id uuid.UUID) uuid.UUID {
	t.Helper()
	_, _, a := uploadKeys(t, f, id)
	if a == nil {
		t.Fatal("暫存列沒有資產")
	}
	return *a
}

// ==================== BeginUpload、CompleteUpload ====================

func TestBeginUpload_StoresOnlyTheTokenHashAndPresignsUnderTmp(t *testing.T) {
	f := setup(t)
	res := begin(t, f, owner)
	if !strings.HasPrefix(tmpKeyOf(res), "tmp/") || res.Headers["Content-MD5"] == "" {
		t.Fatalf("預簽結果不對：%+v", res)
	}
	var hash []byte
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT token_hash FROM media.staged_uploads WHERE id = $1`, res.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(res.Token))
	if !bytes.Equal(hash, want[:]) {
		t.Fatal("token_hash 不是 token 的 SHA-256")
	}
	if got := res.ExpiresAt.Sub(f.clock.Now()); got != media.UploadTTL {
		t.Fatalf("到期時間 = now + %v，預期 %v", got, media.UploadTTL)
	}
}

func TestCompleteUpload_EnqueuesTheProcessJobInTheSameTransaction(t *testing.T) {
	f := setup(t)
	res := begin(t, f, owner)
	errBusiness := errors.New("business row failed")
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		if err := f.m.CompleteUpload(ctx, owner, res.Token); err != nil {
			return err
		}
		return errBusiness
	})
	if !errors.Is(err, errBusiness) {
		t.Fatalf("err = %v", err)
	}
	if got := uploadState(t, f, res.ID); got != "PENDING" || jobCount(t, f, "media.process_upload") != 0 {
		t.Fatalf("回滾後 state=%s、job=%d，預期 PENDING 且沒有 job", got, jobCount(t, f, "media.process_upload"))
	}
	complete(t, f, owner, res)
	complete(t, f, owner, res) // 重入成功，不重複排 job
	if got := uploadState(t, f, res.ID); got != "UPLOADED" {
		t.Fatalf("state = %s，預期 UPLOADED", got)
	}
	if n := jobCount(t, f, "media.process_upload"); n != 1 {
		t.Fatalf("處理 job %d 個，預期 1", n)
	}
	if _, final, _ := uploadKeys(t, f, res.ID); final == nil || !strings.HasPrefix(*final, "public/") {
		t.Fatalf("正式 key = %v，預期 public/ 開頭", final)
	}
}

func TestCompleteUpload_RejectsNonUploader(t *testing.T) {
	f := setup(t)
	res := begin(t, f, owner)
	wantKind(t, f.m.CompleteUpload(t.Context(), other, res.Token), apperr.KindNotFound)
	wantKind(t, f.m.CompleteUpload(t.Context(), media.Uploader{ID: owner.ID, Admin: true}, res.Token), apperr.KindNotFound)
	if got := uploadState(t, f, res.ID); got != "PENDING" {
		t.Fatalf("state = %s，預期 PENDING", got)
	}
}

func TestStagedToken_ExpiredRejected(t *testing.T) {
	f := setup(t)
	res := begin(t, f, owner)
	f.clock.Advance(media.UploadTTL)
	wantKind(t, f.m.CompleteUpload(t.Context(), owner, res.Token), apperr.KindFailedPrecondition)
	if got := uploadState(t, f, res.ID); got != "PENDING" {
		t.Fatalf("state = %s，過期的列不該轉成 UPLOADED", got)
	}
}

func TestBeginUpload_RejectsInvalidDeclarations(t *testing.T) {
	f := setup(t)
	_, err := f.m.BeginUpload(t.Context(), owner, media.BeginInput{Size: 0, ContentType: "image/png", MD5: "x"})
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = f.m.BeginUpload(t.Context(), media.Uploader{}, media.BeginInput{})
	wantKind(t, err, apperr.KindInvalidInput)
}

// ==================== 處理 job ====================

func TestProcess_SuccessCreatesTheAssetAndDeletesTheTmpObjectAfterCommit(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	tmp, final, asset := uploadKeys(t, f, res.ID)
	if got := uploadState(t, f, res.ID); got != "READY" || asset == nil || final == nil {
		t.Fatalf("state=%s asset=%v final=%v", got, asset, final)
	}
	if got := assetState(t, f, *asset); got != "STAGED" {
		t.Fatalf("資產 state = %s，預期 STAGED", got)
	}
	if f.store.has(tmp) || !f.store.has(*final) {
		t.Fatalf("tmp 應該已刪、正式物件應該存在：tmp=%v final=%v", f.store.has(tmp), f.store.has(*final))
	}
	if st, err := f.m.GetUpload(t.Context(), owner, res.Token); err != nil || st.State != media.UploadReady {
		t.Fatalf("GetUpload = %+v, %v", st, err)
	}
}

func TestProcess_IgnoresRowsThatAreNotUploaded(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if err := f.m.ProcessUpload(t.Context(), uuid.New(), 1, media.ProcessMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if n := assetCount(t, f); n != 1 {
		t.Fatalf("資產 %d 筆，預期 1", n)
	}
}

func TestProcess_PermanentFailureRejectsImmediately(t *testing.T) {
	f := setup(t)
	f.process = func(context.Context, media.ProcessInput) (media.ProcessResult, error) {
		return media.ProcessResult{}, media.Permanent(media.RejectUnrecognizedFormat, errors.New("not an image"))
	}
	res := uploaded(t, f, owner)
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatalf("永久失敗不該要求重試：%v", err)
	}
	st, err := f.m.GetUpload(t.Context(), owner, res.Token)
	if err != nil || st.State != media.UploadRejected || st.RejectReason != media.RejectUnrecognizedFormat {
		t.Fatalf("GetUpload = %+v, %v", st, err)
	}
	if n := assetCount(t, f); n != 0 {
		t.Fatalf("被拒絕的暫存不該有資產，現有 %d", n)
	}
	_, err = confirmInTx(f, owner, res.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
}

func TestProcess_TransientFailureRetriesThenRejects(t *testing.T) {
	f := setup(t)
	f.process = func(context.Context, media.ProcessInput) (media.ProcessResult, error) {
		return media.ProcessResult{}, errors.New("r2 unavailable")
	}
	res := uploaded(t, f, owner)
	for attempt := 1; attempt < media.ProcessMaxAttempts; attempt++ {
		if err := f.m.ProcessUpload(t.Context(), res.ID, attempt, media.ProcessMaxAttempts); err == nil {
			t.Fatalf("第 %d 次暫時失敗應該回錯誤讓 River 重試", attempt)
		}
		if got := uploadState(t, f, res.ID); got != "UPLOADED" {
			t.Fatalf("第 %d 次之後 state = %s，預期仍是 UPLOADED", attempt, got)
		}
	}
	if err := f.m.ProcessUpload(t.Context(), res.ID, media.ProcessMaxAttempts, media.ProcessMaxAttempts); err != nil {
		t.Fatalf("重試用盡後標拒絕，不再回錯誤：%v", err)
	}
	st, err := f.m.GetUpload(t.Context(), owner, res.Token)
	if err != nil || st.State != media.UploadRejected || st.RejectReason != media.RejectProcessingFailed {
		t.Fatalf("GetUpload = %+v, %v", st, err)
	}
}

func TestProcess_ExpiredMidFlightLeavesNoOrphanObject(t *testing.T) {
	f := setup(t)
	res := uploaded(t, f, owner)
	_, final, _ := uploadKeys(t, f, res.ID)
	f.process = func(ctx context.Context, in media.ProcessInput) (media.ProcessResult, error) {
		out, err := f.copyProcess(ctx, in)
		f.clock.Advance(media.UploadTTL + time.Minute) // 處理的途中列過期
		return out, err
	}
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if f.store.has(*final) {
		t.Fatal("列過期後，剛寫的正式物件還在")
	}
	if n := assetCount(t, f); n != 0 {
		t.Fatalf("過期的處理不該建資產，現有 %d", n)
	}
	st, err := f.m.GetUpload(t.Context(), owner, res.Token)
	if err != nil || st.State != media.UploadRejected || st.RejectReason != media.RejectExpired {
		t.Fatalf("GetUpload = %+v, %v", st, err)
	}
	// 之後 GC 回收 tmp。
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if tmp, _, _ := uploadKeys(t, f, res.ID); f.store.has(tmp) {
		t.Fatal("GC 之後 tmp 物件還在")
	}
}

// ==================== Confirm ====================

func TestConfirm_Success(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	a, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != assetIDOf(t, f, res.ID) || a.Visibility != media.VisibilityPublic || a.SizeBytes != int64(len(payload)) || a.ContentType != "image/png" {
		t.Fatalf("Confirm = %+v", a)
	}
	if uploadState(t, f, res.ID) != "CONFIRMED" || assetState(t, f, a.ID) != "ACTIVE" {
		t.Fatal("確認後暫存列應該是 CONFIRMED、資產應該是 ACTIVE")
	}
}

func TestConfirm_RollsBackWithTheBusinessRow(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	errBusiness := errors.New("business row failed")
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		if _, err := f.m.Confirm(ctx, owner, res.Token); err != nil {
			return err
		}
		return errBusiness
	})
	if !errors.Is(err, errBusiness) {
		t.Fatalf("err = %v", err)
	}
	if got := uploadState(t, f, res.ID); got != "READY" {
		t.Fatalf("回滾後暫存列 = %s，預期仍是 READY", got)
	}
	if got := assetState(t, f, assetIDOf(t, f, res.ID)); got != "STAGED" {
		t.Fatalf("回滾後資產 = %s，預期仍是 STAGED（暫存）", got)
	}
	if _, err := confirmInTx(f, owner, res.Token); err != nil {
		t.Fatalf("回滾之後可以再確認：%v", err)
	}
}

func TestConfirm_ReentryReturnsSameAsset(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	first, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	second, err := confirmInTx(f, owner, res.Token)
	if err != nil || second != first {
		t.Fatalf("重入 = %+v, %v，預期 %+v", second, err, first)
	}
}

func TestConfirm_ReentryAfterMarkDeletedIsRejected(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	a, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	_, err = confirmInTx(f, owner, res.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
}

func TestConfirm_OnlyReturnsItsOwnAsset(t *testing.T) {
	f := setup(t)
	a := ready(t, f, owner)
	b := ready(t, f, owner) // 同一個上傳者、較新的資產
	gotA, err := confirmInTx(f, owner, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := confirmInTx(f, owner, b.Token)
	if err != nil {
		t.Fatal(err)
	}
	again, err := confirmInTx(f, owner, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	if gotA.ID == gotB.ID || again.ID != gotA.ID {
		t.Fatalf("A=%v B=%v A 重入=%v：重入只能回這筆暫存的資產", gotA.ID, gotB.ID, again.ID)
	}
}

func TestConfirm_RejectsNonUploader(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	_, err := confirmInTx(f, other, res.Token)
	wantKind(t, err, apperr.KindNotFound)
	_, err = confirmInTx(f, media.Uploader{ID: owner.ID, Admin: true}, res.Token)
	wantKind(t, err, apperr.KindNotFound)
	if got := uploadState(t, f, res.ID); got != "READY" {
		t.Fatalf("非上傳者之後 state = %s，預期仍是 READY", got)
	}
}

func TestConfirm_RejectsStagingNotYetReady(t *testing.T) {
	f := setup(t)
	pending := begin(t, f, owner)
	_, err := confirmInTx(f, owner, pending.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
	up := uploaded(t, f, owner)
	_, err = confirmInTx(f, owner, up.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
	if uploadState(t, f, pending.ID) != "PENDING" || uploadState(t, f, up.ID) != "UPLOADED" {
		t.Fatal("未就緒的暫存列狀態不該被改動")
	}
}

func TestConfirm_RejectsExpiredStaging(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	f.clock.Advance(media.UploadTTL)
	_, err := confirmInTx(f, owner, res.Token)
	wantKind(t, err, apperr.KindFailedPrecondition)
	if got := uploadState(t, f, res.ID); got != "READY" {
		t.Fatalf("state = %s：過期的確認不改列，回收交給 GC", got)
	}
}

func TestConfirm_UnknownTokenNotFound(t *testing.T) {
	f := setup(t)
	for _, token := range []string{"", "short", strings.Repeat("A", 43)} {
		_, err := confirmInTx(f, owner, token)
		wantKind(t, err, apperr.KindNotFound)
	}
}

func TestConfirm_RequiresTheOwnersTransaction(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	_, err := f.m.Confirm(t.Context(), owner, res.Token)
	if !errors.Is(err, media.ErrNoTransaction) {
		t.Fatalf("err = %v，預期 ErrNoTransaction", err)
	}
	if got := uploadState(t, f, res.ID); got != "READY" {
		t.Fatalf("state = %s", got)
	}
}

func TestConfirmAndGC_ConcurrentOnlyOneSucceeds(t *testing.T) {
	f := setup(t)
	before := newClock()
	before.Advance(10 * time.Minute) // 還沒過期
	after := newClock()
	after.Advance(media.UploadTTL + time.Minute) // 已過期
	confirmer, collector := f.module(before), f.module(after)
	for i := range 12 {
		res := ready(t, f, owner)
		_, final, asset := uploadKeys(t, f, res.ID)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var confirmErr, gcErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			confirmErr = f.d.InTx(t.Context(), func(ctx context.Context) error {
				_, err := confirmer.Confirm(ctx, owner, res.Token)
				return err
			})
		}()
		go func() {
			defer wg.Done()
			<-start
			_, gcErr = collector.CollectGarbage(t.Context())
		}()
		close(start)
		wg.Wait()
		if gcErr != nil {
			t.Fatalf("第 %d 輪 GC：%v", i, gcErr)
		}
		state := uploadState(t, f, res.ID)
		if confirmErr == nil {
			if state != "CONFIRMED" || assetState(t, f, *asset) != "ACTIVE" || !f.store.has(*final) {
				t.Fatalf("第 %d 輪確認成功，卻是 state=%s asset=%s 物件存在=%v", i, state, assetState(t, f, *asset), f.store.has(*final))
			}
		} else {
			wantKind(t, confirmErr, apperr.KindFailedPrecondition)
			if state != "DELETED" || assetState(t, f, *asset) != "DELETED" || f.store.has(*final) {
				t.Fatalf("第 %d 輪 GC 贏了，卻是 state=%s asset=%s 物件存在=%v", i, state, assetState(t, f, *asset), f.store.has(*final))
			}
		}
	}
	assertNoDanglingAssets(t, f)
}

func TestConfirmAndGC_DeterministicInterleavings(t *testing.T) {
	f := setup(t)
	before := newClock()
	before.Advance(10 * time.Minute)
	after := newClock()
	after.Advance(media.UploadTTL + time.Minute)
	confirmer, collector := f.module(before), f.module(after)

	t.Run("GC commits first, Confirm is refused", func(t *testing.T) {
		res := ready(t, f, owner)
		_, final, asset := uploadKeys(t, f, res.ID)
		if got, err := collector.CollectGarbage(t.Context()); err != nil || got.Claimed < 1 {
			t.Fatalf("GC = %+v, %v", got, err)
		}
		err := f.d.InTx(t.Context(), func(ctx context.Context) error {
			_, err := confirmer.Confirm(ctx, owner, res.Token)
			return err
		})
		wantKind(t, err, apperr.KindFailedPrecondition)
		if uploadState(t, f, res.ID) != "DELETED" || assetState(t, f, *asset) != "DELETED" || f.store.has(*final) {
			t.Fatal("GC 贏了，列或物件卻沒被回收")
		}
	})

	t.Run("GC runs while Confirm holds the row lock, Confirm wins", func(t *testing.T) {
		res := ready(t, f, owner)
		_, final, asset := uploadKeys(t, f, res.ID)
		err := f.d.InTx(t.Context(), func(ctx context.Context) error {
			if _, err := confirmer.Confirm(ctx, owner, res.Token); err != nil {
				return err
			}
			// Confirm 的 transaction 還沒提交、持有列鎖；GC 以 SKIP LOCKED 略過這一列。
			_, err := collector.CollectGarbage(t.Context())
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if uploadState(t, f, res.ID) != "CONFIRMED" || assetState(t, f, *asset) != "ACTIVE" || !f.store.has(*final) {
			t.Fatal("Confirm 持鎖期間 GC 動到這一列")
		}
	})
}

// ==================== GetUpload ====================

func TestGetUpload_ReportsStateAndReasonOnlyToTheUploader(t *testing.T) {
	f := setup(t)
	res := begin(t, f, owner)
	st, err := f.m.GetUpload(t.Context(), owner, res.Token)
	if err != nil || st.State != media.UploadPending || st.RejectReason != "" || !st.ExpiresAt.Equal(res.ExpiresAt) {
		t.Fatalf("PENDING：%+v, %v", st, err)
	}
	clientUpload(t, f, res)
	complete(t, f, owner, res)
	if st, err = f.m.GetUpload(t.Context(), owner, res.Token); err != nil || st.State != media.UploadUploaded {
		t.Fatalf("UPLOADED：%+v, %v", st, err)
	}
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if st, err = f.m.GetUpload(t.Context(), owner, res.Token); err != nil || st.State != media.UploadReady {
		t.Fatalf("READY：%+v, %v", st, err)
	}
	if _, err = confirmInTx(f, owner, res.Token); err != nil {
		t.Fatal(err)
	}
	if st, err = f.m.GetUpload(t.Context(), owner, res.Token); err != nil || st.State != media.UploadConfirmed {
		t.Fatalf("CONFIRMED：%+v, %v", st, err)
	}
	_, err = f.m.GetUpload(t.Context(), other, res.Token)
	wantKind(t, err, apperr.KindNotFound)
}

// ==================== GC ====================

func TestGC_NeverDeletesConfirmedAssets(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	a, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	_, final, _ := uploadKeys(t, f, res.ID)
	f.clock.Advance(100 * media.UploadTTL)
	got, err := f.m.CollectGarbage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Claimed != 0 {
		t.Fatalf("GC 標記了 %d 列，已確認的不該被標記", got.Claimed)
	}
	if uploadState(t, f, res.ID) != "CONFIRMED" || assetState(t, f, a.ID) != "ACTIVE" || !f.store.has(*final) {
		t.Fatal("已確認的資產與它的物件必須原封不動")
	}
}

func TestGC_DeletesExpiredUnconfirmedRowsWithTheirObjects(t *testing.T) {
	f := setup(t)
	pending := begin(t, f, owner)
	clientUpload(t, f, pending) // PENDING：只有 tmp 物件
	rdy := ready(t, f, owner)
	rdyAsset := assetIDOf(t, f, rdy.ID)
	_, rdyFinal, _ := uploadKeys(t, f, rdy.ID)
	f.process = func(ctx context.Context, in media.ProcessInput) (media.ProcessResult, error) {
		_, err := f.copyProcess(ctx, in) // 處理中途寫出正式物件後才失敗
		if err != nil {
			return media.ProcessResult{}, err
		}
		return media.ProcessResult{}, media.Permanent(media.RejectChecksumMismatch, errors.New("md5"))
	}
	rej := uploaded(t, f, owner)
	if err := f.m.ProcessUpload(t.Context(), rej.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatal(err)
	}
	_, rejFinal, _ := uploadKeys(t, f, rej.ID)
	if !f.store.has(*rejFinal) {
		t.Fatal("前置條件：被拒絕的暫存已寫出正式物件")
	}
	f.clock.Advance(media.UploadTTL)
	got, err := f.m.CollectGarbage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Claimed != 3 {
		t.Fatalf("標記 %d 列，預期 3（PENDING、READY、REJECTED）", got.Claimed)
	}
	pendingTmp, _, _ := uploadKeys(t, f, pending.ID)
	for _, key := range []string{pendingTmp, *rdyFinal, *rejFinal} {
		if f.store.has(key) {
			t.Errorf("物件 %s 還在", key)
		}
	}
	for _, id := range []uuid.UUID{pending.ID, rdy.ID, rej.ID} {
		if got := uploadState(t, f, id); got != "DELETED" {
			t.Errorf("暫存列 %s = %s，預期 DELETED", id, got)
		}
	}
	if got := assetState(t, f, rdyAsset); got != "DELETED" {
		t.Errorf("暫存的資產 = %s，預期 DELETED", got)
	}
	assertNoDanglingAssets(t, f)
}

func TestGC_KeepsUnexpiredRows(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	f.clock.Advance(media.UploadTTL - time.Second)
	got, err := f.m.CollectGarbage(t.Context())
	if err != nil || got.Claimed != 0 {
		t.Fatalf("GC = %+v, %v，未過期的列不該被標記", got, err)
	}
	if uploadState(t, f, res.ID) != "READY" {
		t.Fatal("未過期的列被動了")
	}
}

func TestGC_DoesNotTouchUploadsStillBeingProcessed(t *testing.T) {
	f := setup(t)
	res := uploaded(t, f, owner)
	f.clock.Advance(media.UploadTTL + time.Minute)
	got, err := f.m.CollectGarbage(t.Context())
	if err != nil || got.Claimed != 0 {
		t.Fatalf("GC = %+v, %v，處理中（UPLOADED）的列不進 GC", got, err)
	}
	if uploadState(t, f, res.ID) != "UPLOADED" {
		t.Fatal("UPLOADED 的列被動了")
	}
}

func TestGC_ReclaimsStuckUploadedAfterGrace(t *testing.T) {
	f := setup(t)
	res := uploaded(t, f, owner)
	tmp, final, _ := uploadKeys(t, f, res.ID)
	if err := f.store.Fake.Put(t.Context(), storage.PutInput{Key: *final, Body: bytes.NewReader(payload), Size: int64(len(payload)), ContentType: "image/png"}); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(media.UploadTTL + media.StuckUploadGrace + time.Minute)
	got, err := f.m.CollectGarbage(t.Context())
	if err != nil || got.Claimed != 1 {
		t.Fatalf("GC = %+v, %v", got, err)
	}
	if uploadState(t, f, res.ID) != "DELETED" || f.store.has(tmp) || f.store.has(*final) {
		t.Fatal("卡住的 UPLOADED 沒被完整回收")
	}
}

func TestGC_KeepsUploadedWithinGrace(t *testing.T) {
	f := setup(t)
	res := uploaded(t, f, owner)
	f.clock.Advance(media.UploadTTL + media.StuckUploadGrace - time.Minute)
	got, err := f.m.CollectGarbage(t.Context())
	if err != nil || got.Claimed != 0 || uploadState(t, f, res.ID) != "UPLOADED" {
		t.Fatalf("GC = %+v, %v", got, err)
	}
}

func TestProcess_InvalidResultRejectsImmediately(t *testing.T) {
	f := setup(t)
	f.process = func(context.Context, media.ProcessInput) (media.ProcessResult, error) {
		return media.ProcessResult{}, nil
	}
	res := uploaded(t, f, owner)
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
		t.Fatal(err)
	}
	st, err := f.m.GetUpload(t.Context(), owner, res.Token)
	if err != nil || st.State != media.UploadRejected || st.RejectReason != media.RejectProcessingFailed {
		t.Fatalf("GetUpload = %+v, %v", st, err)
	}
}

func TestProcess_ShutdownOnLastAttemptDoesNotReject(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.process = func(ctx context.Context, _ media.ProcessInput) (media.ProcessResult, error) {
		cancel()
		return media.ProcessResult{}, ctx.Err()
	}
	res := uploaded(t, f, owner)
	if err := f.m.ProcessUpload(ctx, res.ID, media.ProcessMaxAttempts, media.ProcessMaxAttempts); err == nil {
		t.Fatal("關機中斷應該回錯誤讓 River 重試")
	}
	if got := uploadState(t, f, res.ID); got != "UPLOADED" {
		t.Fatalf("state = %s，預期仍是 UPLOADED", got)
	}
}

// ==================== 實體刪除與三段式 ====================

func confirmed(t *testing.T, f *fixture) (media.ConfirmedAsset, string) {
	t.Helper()
	res := ready(t, f, owner)
	a, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	_, final, _ := uploadKeys(t, f, res.ID)
	return a, *final
}

func markDeletedInTx(f *fixture, ids ...uuid.UUID) error {
	return f.d.InTx(f.t.Context(), func(ctx context.Context) error { return f.m.MarkDeleted(ctx, ids) })
}

func TestMarkDeleted_MarksOnlyConfirmedAssetsAndEnqueuesTheDeleteJob(t *testing.T) {
	f := setup(t)
	a, _ := confirmed(t, f)
	staged := ready(t, f, owner)
	stagedAsset := assetIDOf(t, f, staged.ID)
	if err := markDeletedInTx(f, a.ID, a.ID, stagedAsset, uuid.New()); err != nil { // 重複、未確認、不存在的 ID 都忽略
		t.Fatal(err)
	}
	if got := assetState(t, f, a.ID); got != "DELETING" {
		t.Fatalf("已確認的資產 = %s，預期 DELETING", got)
	}
	if got := assetState(t, f, stagedAsset); got != "STAGED" {
		t.Fatalf("未確認的資產 = %s，擁有者的刪除指令不能刪它", got)
	}
	if n := jobCount(t, f, "media.delete_assets"); n != 1 {
		t.Fatalf("刪除 job %d 個，預期 1", n)
	}
	if err := markDeletedInTx(f, a.ID); err != nil { // 冪等
		t.Fatal(err)
	}
	if err := markDeletedInTx(f); err != nil {
		t.Fatal(err)
	}
}

func TestMarkDeleted_RollsBackWithTheOwnersTransaction(t *testing.T) {
	f := setup(t)
	a, _ := confirmed(t, f)
	errBusiness := errors.New("business row failed")
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		if err := f.m.MarkDeleted(ctx, []uuid.UUID{a.ID}); err != nil {
			return err
		}
		return errBusiness
	})
	if !errors.Is(err, errBusiness) {
		t.Fatal(err)
	}
	if got := assetState(t, f, a.ID); got != "ACTIVE" || jobCount(t, f, "media.delete_assets") != 0 {
		t.Fatalf("回滾後資產 = %s、job = %d，預期 ACTIVE 且沒有 job", got, jobCount(t, f, "media.delete_assets"))
	}
	if err := f.m.MarkDeleted(t.Context(), []uuid.UUID{a.ID}); !errors.Is(err, media.ErrNoTransaction) {
		t.Fatalf("沒有 transaction 的 MarkDeleted = %v，預期 ErrNoTransaction", err)
	}
}

func TestDeleteJob_ThreeStepsDeleteTheObjectAndMarkDeleted(t *testing.T) {
	f := setup(t)
	a, final := confirmed(t, f)
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	if !f.store.has(final) {
		t.Fatal("標記刪除時物件還不該被刪（物件由 transaction 外的 job 刪）")
	}
	n, err := f.m.DeleteAssets(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("DeleteAssets = %d, %v", n, err)
	}
	if f.store.has(final) || assetState(t, f, a.ID) != "DELETED" {
		t.Fatal("物件應該已刪、資產應該是 DELETED")
	}
	if n, err := f.m.DeleteAssets(t.Context()); err != nil || n != 0 { // 冪等
		t.Fatalf("第二次 DeleteAssets = %d, %v", n, err)
	}
}

func TestDeleteJob_StorageFailureKeepsRowRetryable(t *testing.T) {
	f := setup(t)
	a, final := confirmed(t, f)
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	f.store.setDeleteFailure(final, errors.New("r2 unavailable"))
	if _, err := f.m.DeleteAssets(t.Context()); err == nil {
		t.Fatal("刪物件失敗應該回錯誤讓 job 重試")
	}
	if got := assetState(t, f, a.ID); got != "DELETING" || !f.store.has(final) {
		t.Fatalf("失敗後資產 = %s、物件存在 = %v，預期 DELETING 且物件還在（可重試）", got, f.store.has(final))
	}
	f.store.setDeleteFailure(final, nil)
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := assetState(t, f, a.ID); got != "DELETED" || f.store.has(final) {
		t.Fatalf("重試後資產 = %s、物件存在 = %v", got, f.store.has(final))
	}
}

func TestGC_StorageFailureKeepsRowRetryableAndNeverPointsAtAMissingObject(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	assetID := assetIDOf(t, f, res.ID)
	_, final, _ := uploadKeys(t, f, res.ID)
	f.store.setDeleteFailure(*final, errors.New("r2 unavailable"))
	f.clock.Advance(media.UploadTTL)
	if _, err := f.m.CollectGarbage(t.Context()); err == nil {
		t.Fatal("刪物件失敗應該回錯誤")
	}
	if uploadState(t, f, res.ID) != "DELETING" || assetState(t, f, assetID) != "DELETING" || !f.store.has(*final) {
		t.Fatal("失敗後兩個列都應該留在 DELETING，物件還在")
	}
	assertNoDanglingAssets(t, f)
	if _, err := confirmInTx(f, owner, res.Token); err == nil {
		t.Fatal("DELETING 的暫存不能確認")
	}
	f.store.setDeleteFailure(*final, nil)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if uploadState(t, f, res.ID) != "DELETED" || assetState(t, f, assetID) != "DELETED" || f.store.has(*final) {
		t.Fatal("重試後應該全部是 DELETED、物件已刪")
	}
}

func TestDeleteJob_FailureDoesNotBlockOtherRows(t *testing.T) {
	f := setup(t)
	a, aKey := confirmed(t, f)
	b, bKey := confirmed(t, f)
	if err := markDeletedInTx(f, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	f.store.setDeleteFailure(aKey, errors.New("r2 unavailable"))
	if _, err := f.m.DeleteAssets(t.Context()); err == nil {
		t.Fatal("應該回 a 的錯誤")
	}
	if assetState(t, f, a.ID) != "DELETING" || assetState(t, f, b.ID) != "DELETED" || f.store.has(bKey) {
		t.Fatal("a 失敗不該擋住 b")
	}
	f.store.setDeleteFailure(aKey, nil)
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if assetState(t, f, a.ID) != "DELETED" {
		t.Fatal("a 重試後應該是 DELETED")
	}
}

// ==================== 儲存呼叫沒有 transaction ====================

func TestStorageIsNeverCalledInsideATransaction(t *testing.T) {
	f := setup(t)
	// 走完整條路徑：上傳、處理、確認、擁有者刪除、GC 與實體刪除。
	a, _ := confirmed(t, f)
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	begin(t, f, owner)
	f.clock.Advance(media.UploadTTL)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls := f.store.callsInTx(); len(calls) != 0 {
		t.Fatalf("這些儲存呼叫的 ctx 帶著 transaction：%v", calls)
	}
}

func TestStorageEntryPointsRefuseAContextWithATransaction(t *testing.T) {
	f := setup(t)
	res := uploaded(t, f, owner)
	calls := map[string]func(ctx context.Context) error{
		"BeginUpload": func(ctx context.Context) error {
			_, err := f.m.BeginUpload(ctx, owner, media.BeginInput{Size: 1, ContentType: "image/png", MD5: base64.StdEncoding.EncodeToString(make([]byte, 16))})
			return err
		},
		"ProcessUpload":  func(ctx context.Context) error { return f.m.ProcessUpload(ctx, res.ID, 1, media.ProcessMaxAttempts) },
		"CollectGarbage": func(ctx context.Context) error { _, err := f.m.CollectGarbage(ctx); return err },
		"DeleteAssets":   func(ctx context.Context) error { _, err := f.m.DeleteAssets(ctx); return err },
	}
	for name, call := range calls {
		err := f.d.InTx(t.Context(), call)
		if !errors.Is(err, media.ErrInTransaction) {
			t.Errorf("%s 在 transaction 內 = %v，預期 ErrInTransaction", name, err)
		}
	}
	if got := uploadState(t, f, res.ID); got != "UPLOADED" {
		t.Fatalf("被拒絕的呼叫不該改列：state = %s", got)
	}
	if calls := f.store.callsInTx(); len(calls) != 0 {
		t.Fatalf("儲存在 transaction 內被呼叫：%v", calls)
	}
}

func TestProcess_LostToGCLeavesNoOrphanObject(t *testing.T) {
	for _, state := range []string{"DELETING", "DELETED"} {
		t.Run(state, func(t *testing.T) {
			f := setup(t)
			res := uploaded(t, f, owner)
			_, final, _ := uploadKeys(t, f, res.ID)
			f.process = func(ctx context.Context, in media.ProcessInput) (media.ProcessResult, error) {
				out, err := f.copyProcess(ctx, in)
				if _, e := f.d.Pool().Exec(ctx, `UPDATE media.staged_uploads SET state = $2 WHERE id = $1`, res.ID, state); e != nil {
					t.Error(e)
				}
				return out, err
			}
			if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err != nil {
				t.Fatal(err)
			}
			if f.store.has(*final) {
				t.Fatal("列已被 GC 標為 " + state + "，處理 job 寫的正式物件還在")
			}
			if got := uploadState(t, f, res.ID); got != state {
				t.Fatalf("state = %s，處理 job 不該改動 GC 的列", got)
			}
			if n := assetCount(t, f); n != 0 {
				t.Fatalf("現有 %d 個資產", n)
			}
		})
	}
}

func TestProcess_LostDeleteFailureOutsideUploadedRetries(t *testing.T) {
	f := setup(t)
	res := uploaded(t, f, owner)
	_, final, _ := uploadKeys(t, f, res.ID)
	f.store.setDeleteFailure(*final, errors.New("boom"))
	f.process = func(ctx context.Context, in media.ProcessInput) (media.ProcessResult, error) {
		out, err := f.copyProcess(ctx, in)
		if _, e := f.d.Pool().Exec(ctx, `UPDATE media.staged_uploads SET state = 'DELETED' WHERE id = $1`, res.ID); e != nil {
			t.Error(e)
		}
		return out, err
	}
	if err := f.m.ProcessUpload(t.Context(), res.ID, 1, media.ProcessMaxAttempts); err == nil {
		t.Fatal("DELETED 狀態下刪正式物件失敗，應回傳錯誤讓 River 重試")
	}
}
