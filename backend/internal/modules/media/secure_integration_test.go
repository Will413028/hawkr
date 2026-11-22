//go:build integration

package media_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"slices"
	"strconv"
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

// uploadSecure 像 client 一樣：BeginSecureUpload，再依回傳的段大小依序送每一段。
func uploadSecure(t *testing.T, f *fixture, who media.Uploader, data []byte) media.SecureUploadResult {
	t.Helper()
	return uploadSecureVia(t, f.m, who, data)
}

func uploadSecureVia(t *testing.T, m *media.Module, who media.Uploader, data []byte) media.SecureUploadResult {
	t.Helper()
	b, err := m.BeginSecureUpload(t.Context(), who, int64(len(data)))
	if err != nil {
		t.Fatalf("BeginSecureUpload：%v", err)
	}
	var last media.SecureChunkResult
	for i := range b.ChunkCount {
		start := i * b.ChunkSize
		end := min(start+b.ChunkSize, len(data))
		last, err = m.PutSecureChunk(t.Context(), who, b.Token, i, bytes.NewReader(data[start:end]), int64(end-start))
		if err != nil {
			t.Fatalf("PutSecureChunk %d：%v", i, err)
		}
	}
	if !last.Complete {
		t.Fatalf("送完所有段後沒有完成：%+v", last)
	}
	return media.SecureUploadResult{Token: b.Token, ContentType: last.ContentType, SizeBytes: last.SizeBytes, ExpiresAt: last.ExpiresAt}
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

// objectKeyOf 回傳資產列的 object_key（公開資產是單一物件的 key，機敏資產是前綴 secure/<資產 ID>/）。
func objectKeyOf(t *testing.T, f *fixture, assetID uuid.UUID) string {
	t.Helper()
	var k string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT object_key FROM media.assets WHERE id = $1`, assetID).Scan(&k); err != nil {
		t.Fatal(err)
	}
	return k
}

// chunkKeys 回傳資產各段的物件 key（依段序號）。
func chunkKeys(t *testing.T, f *fixture, assetID uuid.UUID) []string {
	t.Helper()
	rows, err := f.d.Pool().Query(t.Context(), `SELECT object_key FROM media.secure_chunks WHERE asset_id = $1 ORDER BY idx`, assetID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
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

// ==================== BeginSecureUpload 與 PutSecureChunk ====================

// noObjects 斷言儲存裡沒有任何物件、也沒有任何 Put（被拒的輸入不寫物件）。
func noObjects(t *testing.T, f *fixture) {
	t.Helper()
	if len(f.store.puts) != 0 || len(f.store.Keys()) != 0 {
		t.Fatalf("被拒的輸入寫了物件：puts=%d keys=%v", len(f.store.puts), f.store.Keys())
	}
}

// threeChunkData 是剛好三段的內容：兩個滿段加一個 1 位元組的最後一段，內容隨位置變化（對調的段不會恰好相同）。
func threeChunkData() []byte {
	data := make([]byte, 2*media.SecureChunkSize+1)
	for i := range data {
		data[i] = byte(i*7 + i/4096)
	}
	copy(data, pdfOf(100))
	return data
}

func beginSecure(t *testing.T, f *fixture, who media.Uploader, size int) media.SecureBeginResult {
	t.Helper()
	b, err := f.m.BeginSecureUpload(t.Context(), who, int64(size))
	if err != nil {
		t.Fatalf("BeginSecureUpload：%v", err)
	}
	return b
}

func putChunk(f *fixture, who media.Uploader, token string, index int, data []byte) (media.SecureChunkResult, error) {
	return f.m.PutSecureChunk(f.t.Context(), who, token, index, bytes.NewReader(data), int64(len(data)))
}

func nextIndexOf(t *testing.T, err error) string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v，預期 apperr.Error", err)
	}
	return ae.Metadata["next_index"]
}

func TestSecureUpload_BeginReturnsChunkSizeAndCount(t *testing.T) {
	f := setup(t)
	b := beginSecure(t, f, owner, 2*media.SecureChunkSize+1)
	if b.ChunkSize != media.SecureChunkSize || b.ChunkCount != 3 || b.Token == "" || !b.ExpiresAt.Equal(f.clock.Now().Add(media.UploadTTL)) {
		t.Fatalf("結果 %+v", b)
	}
	for size, want := range map[int]int{1: 1, media.SecureChunkSize: 1, media.SecureChunkSize + 1: 2, int(media.MaxSecureUploadSize): 25} {
		if got := beginSecure(t, f, owner, size).ChunkCount; got != want {
			t.Errorf("size=%d：段總數 %d，預期 %d", size, got, want)
		}
	}
	noObjects(t, f)
}

func TestSecureUpload_BeginRejectsSizeOutOfRangeInvalidUploaderAndTransaction(t *testing.T) {
	f := setup(t)
	for _, size := range []int64{0, -1, media.MaxSecureUploadSize + 1} {
		_, err := f.m.BeginSecureUpload(t.Context(), owner, size)
		wantKind(t, err, apperr.KindInvalidInput)
	}
	_, err := f.m.BeginSecureUpload(t.Context(), media.Uploader{}, 10)
	wantKind(t, err, apperr.KindInvalidInput)
	err = f.d.InTx(t.Context(), func(ctx context.Context) error {
		_, err := f.m.BeginSecureUpload(ctx, owner, 10)
		return err
	})
	if !errors.Is(err, media.ErrInTransaction) {
		t.Fatalf("err = %v", err)
	}
	noWrites(t, f)
	noObjects(t, f)
}

// 三段的檔案上傳後讀回位元組相同（B1-3c1 驗收）；每段是各自的物件、各自的 key、各自的 nonce。
func TestSecureUpload_ThreeChunksReadBackIdentical(t *testing.T) {
	f := setup(t)
	data := threeChunkData()
	res := uploadSecure(t, f, owner, data)
	if res.ContentType != "application/pdf" || res.SizeBytes != int64(len(data)) {
		t.Fatalf("結果 %+v", res)
	}
	_, assetID := secureRow(t, f, res.Token)
	keys := chunkKeys(t, f, assetID)
	if len(keys) != 3 {
		t.Fatalf("段 key %v", keys)
	}
	for i, k := range keys {
		if want := "secure/" + assetID.String() + "/" + strconv.Itoa(i) + "/"; !strings.HasPrefix(k, want) {
			t.Fatalf("第 %d 段的 key %s，預期前綴 %s", i, k, want)
		}
	}
	a := confirmSecure(t, f, owner, res.Token)
	rc, err := f.m.ReadSecure(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, data) {
		t.Fatalf("讀回 %d 位元組，內容與上傳的不同（原 %d）", len(got), len(data))
	}
	if calls := f.store.callsInTx(); len(calls) != 0 {
		t.Fatalf("在 transaction 內呼叫儲存：%v", calls)
	}
}

func TestSecureUpload_RecordsKekIDWrappedDEKAndChunkCount(t *testing.T) {
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
	var chunkCount int
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT kek_id, wrapped_dek, object_key, state, content_type, size_bytes, chunk_count FROM media.assets WHERE id = $1`, assetID).Scan(&kekID, &wrapped, &key, &state, &ct, &size, &chunkCount); err != nil {
		t.Fatal(err)
	}
	if kekID == nil || *kekID != "k1" || len(wrapped) == 0 || key != "secure/"+assetID.String()+"/" || state != "STAGED" || ct != "application/pdf" || size != int64(len(data)) || chunkCount != 1 {
		t.Fatalf("資產列 kek_id=%v wrapped=%d key=%s state=%s ct=%s size=%d chunks=%d", kekID, len(wrapped), key, state, ct, size, chunkCount)
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
	if got := f.store.Keys(); len(got) != 1 || got[0] != chunkKeys(t, f, assetID)[0] {
		t.Fatalf("儲存裡的 key = %v，預期只有第 0 段 %v", got, chunkKeys(t, f, assetID))
	}
}

func TestSecureUpload_AcceptsImagesAndPDF(t *testing.T) {
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

// 宣告 PDF 實為任意位元組：第 0 段在寫任何物件之前被拒，會話轉 REJECTED。
func TestSecureUpload_FirstChunkNotPDFOrImageRejectsSessionAndWritesNoObject(t *testing.T) {
	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic")...)
	for name, data := range map[string][]byte{
		"fake-pdf":   []byte("%PD this is not a pdf"),
		"plain-text": []byte("hello world"),
		"heic":       heic,
		"zip":        {'P', 'K', 3, 4, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			b := beginSecure(t, f, owner, len(data))
			_, err := putChunk(f, owner, b.Token, 0, data)
			wantKind(t, err, apperr.KindInvalidInput)
			noObjects(t, f)
			var state string
			var reason *string
			if err := f.d.Pool().QueryRow(t.Context(), `SELECT state, reject_reason FROM media.staged_uploads`).Scan(&state, &reason); err != nil {
				t.Fatal(err)
			}
			if state != "REJECTED" || reason == nil || *reason != "UNRECOGNIZED_FORMAT" {
				t.Fatalf("state=%s reason=%v", state, reason)
			}
			if assetCount(t, f) != 0 {
				t.Fatal("被拒的會話建了資產列")
			}
			// 被拒之後同一個 token 不再收段。
			_, err = putChunk(f, owner, b.Token, 0, pdfOf(len(data)))
			wantKind(t, err, apperr.KindFailedPrecondition)
		})
	}
}

// 只有第 0 段看 magic bytes：後面的段是任意位元組（密文串流的後半本來就沒有檔頭）。
func TestSecureUpload_LaterChunksAreNotSniffed(t *testing.T) {
	f := setup(t)
	uploadSecure(t, f, owner, threeChunkData())
}

func TestSecureUpload_PutSecureRejectsEmptyAndOversizeBodyWithoutWrites(t *testing.T) {
	f := setup(t)
	_, err := f.m.PutSecure(t.Context(), owner, bytes.NewReader(nil))
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = f.m.PutSecure(t.Context(), owner, &endless{prefix: []byte("%PDF-1.7\n")})
	wantKind(t, err, apperr.KindInvalidInput)
	noWrites(t, f)
	noObjects(t, f)
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

func TestSecureUpload_PutSecureChunkRefusesTransactionAndInvalidUploader(t *testing.T) {
	f := setup(t)
	b := beginSecure(t, f, owner, 100)
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		_, err := f.m.PutSecureChunk(ctx, owner, b.Token, 0, bytes.NewReader(pdfOf(100)), 100)
		return err
	})
	if !errors.Is(err, media.ErrInTransaction) {
		t.Fatalf("err = %v", err)
	}
	_, err = f.m.PutSecureChunk(t.Context(), media.Uploader{}, b.Token, 0, bytes.NewReader(pdfOf(100)), 100)
	wantKind(t, err, apperr.KindInvalidInput)
	noObjects(t, f)
}

// 非上傳者與不存在的 token 是同一個 NotFound，而且不寫物件。
func TestSecureUpload_ChunkOfAnotherUploaderOrUnknownTokenIsNotFound(t *testing.T) {
	f := setup(t)
	b := beginSecure(t, f, owner, 100)
	data := pdfOf(100)[:100]
	for name, c := range map[string]struct {
		who   media.Uploader
		token string
	}{
		"other-user":   {other, b.Token},
		"admin-twin":   {media.Uploader{ID: owner.ID, Admin: true}, b.Token},
		"bad-token":    {owner, "not-a-token"},
		"unknown-form": {owner, strings.Repeat("A", 43)},
	} {
		_, err := putChunk(f, c.who, c.token, 0, data)
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Errorf("%s：err = %v，預期 NotFound", name, err)
		}
	}
	noObjects(t, f)
}

// index 跳號：FailedPrecondition 並帶 next_index，不寫物件。
func TestSecureUpload_SkippedIndexIsFailedPreconditionWithNextIndex(t *testing.T) {
	f := setup(t)
	b := beginSecure(t, f, owner, 2*media.SecureChunkSize+1)
	_, err := putChunk(f, owner, b.Token, 2, []byte("x"))
	wantKind(t, err, apperr.KindFailedPrecondition)
	if got := nextIndexOf(t, err); got != "0" {
		t.Fatalf("next_index = %q，預期 0", got)
	}
	noObjects(t, f)
}

// 重送已接受的段：FailedPrecondition 與正確的 next_index，已寫的物件不變。
func TestSecureUpload_ResendingAnAcceptedChunkReturnsNextIndex(t *testing.T) {
	f := setup(t)
	data := threeChunkData()
	b := beginSecure(t, f, owner, len(data))
	if _, err := putChunk(f, owner, b.Token, 0, data[:b.ChunkSize]); err != nil {
		t.Fatal(err)
	}
	keysBefore := f.store.Keys()
	_, err := putChunk(f, owner, b.Token, 0, data[:b.ChunkSize])
	wantKind(t, err, apperr.KindFailedPrecondition)
	if got := nextIndexOf(t, err); got != "1" {
		t.Fatalf("next_index = %q，預期 1", got)
	}
	if got := f.store.Keys(); !slices.Equal(got, keysBefore) {
		t.Fatalf("重送後儲存的 key %v，預期不變 %v", got, keysBefore)
	}
}

// 最後一段送完之後再送任何一段：FailedPrecondition，next_index 是段總數（client 知道已經完成）。
func TestSecureUpload_AfterCompletionAnyChunkReturnsChunkCountAsNextIndex(t *testing.T) {
	f := setup(t)
	data := pdfOf(100)
	res := uploadSecure(t, f, owner, data)
	_, err := putChunk(f, owner, res.Token, 0, data)
	wantKind(t, err, apperr.KindFailedPrecondition)
	if got := nextIndexOf(t, err); got != "1" {
		t.Fatalf("next_index = %q，預期 1（段總數）", got)
	}
}

// 長度檢查：宣告的 length 不等於該段應有長度（body 本身長度正確）、實際位元組比宣告短、比宣告長，各被拒且不寫物件。
func TestSecureUpload_ChunkLengthMismatchIsRejectedWithoutWrites(t *testing.T) {
	f := setup(t)
	data := pdfOf(100)
	b := beginSecure(t, f, owner, len(data))
	ctx := t.Context()
	n := int64(len(data))
	for name, c := range map[string]struct {
		body   []byte
		length int64
	}{
		"declared-length-differs": {data, n - 1},
		"body-shorter":            {data[:len(data)-1], n},
		"body-longer":             {append(bytes.Clone(data), 'x'), n},
	} {
		_, err := f.m.PutSecureChunk(ctx, owner, b.Token, 0, bytes.NewReader(c.body), c.length)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 InvalidInput", name, err)
		}
	}
	noObjects(t, f)
	// 會話沒有被拒：長度對了之後照常收。
	if _, err := putChunk(f, owner, b.Token, 0, data); err != nil {
		t.Fatalf("長度對了仍被拒：%v", err)
	}
}

// 中間段必須剛好 SecureChunkSize，最後一段是餘數。
func TestSecureUpload_MiddleChunkMustBeExactlyTheChunkSize(t *testing.T) {
	f := setup(t)
	data := threeChunkData()
	b := beginSecure(t, f, owner, len(data))
	if _, err := putChunk(f, owner, b.Token, 0, data[:b.ChunkSize-1]); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("短一個位元組的第 0 段：err = %v", err)
	}
	noObjects(t, f)
}

func TestSecureUpload_ExpiredSessionIsFailedPreconditionAndGCReclaimsIt(t *testing.T) {
	f := setup(t)
	data := threeChunkData()
	b := beginSecure(t, f, owner, len(data))
	if _, err := putChunk(f, owner, b.Token, 0, data[:b.ChunkSize]); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(media.UploadTTL)
	_, err := putChunk(f, owner, b.Token, 1, data[b.ChunkSize:2*b.ChunkSize])
	wantKind(t, err, apperr.KindFailedPrecondition)
	if len(f.store.Keys()) != 1 {
		t.Fatalf("過期的段不該被寫：%v", f.store.Keys())
	}
}

// UPLOADING 過期後 GC 刪掉已寫的段與沒推進成功的孤兒嘗試；列轉 DELETED。
func TestGC_ReclaimsExpiredUploadingSessionChunksAndOrphans(t *testing.T) {
	f := setup(t)
	data := threeChunkData()
	b := beginSecure(t, f, owner, len(data))
	if _, err := putChunk(f, owner, b.Token, 0, data[:b.ChunkSize]); err != nil {
		t.Fatal(err)
	}
	var uploadID, assetID uuid.UUID
	h := sha256.Sum256([]byte(b.Token))
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT id, planned_asset_id FROM media.staged_uploads WHERE token_hash = $1`, h[:]).Scan(&uploadID, &assetID); err != nil {
		t.Fatal(err)
	}
	orphan := "secure/" + assetID.String() + "/1/" + uuid.New().String()
	if err := f.store.Fake.Put(t.Context(), putInput(orphan, []byte("orphan"))); err != nil {
		t.Fatal(err)
	}
	if got := len(f.store.Keys()); got != 2 {
		t.Fatalf("前置：物件 %v", f.store.Keys())
	}
	f.clock.Advance(media.UploadTTL - time.Second)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if uploadState(t, f, uploadID) != "UPLOADING" || len(f.store.Keys()) != 2 {
		t.Fatal("未過期的會話被回收")
	}
	f.clock.Advance(2 * time.Second)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := uploadState(t, f, uploadID); got != "DELETED" || len(f.store.Keys()) != 0 {
		t.Fatalf("過期後 state=%s、剩下物件 %v", got, f.store.Keys())
	}
}

// 已確認的機敏資產被擁有者刪除：所有段的物件（含沒記錄的孤兒）一併被刪。
func TestMarkDeleted_RemovesEveryChunkObjectOfASecureAsset(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, threeChunkData())
	a := confirmSecure(t, f, owner, res.Token)
	orphan := "secure/" + a.ID.String() + "/0/" + uuid.New().String()
	if err := f.store.Fake.Put(t.Context(), putInput(orphan, []byte("orphan"))); err != nil {
		t.Fatal(err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := f.store.Keys(); len(got) != 0 {
		t.Fatalf("實體刪除後剩下 %v", got)
	}
	if calls := f.store.callsInTx(); len(calls) != 0 {
		t.Fatalf("在 transaction 內呼叫儲存：%v", calls)
	}
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

func TestSecureUpload_ConcurrentChunkLimitIsResourceExhausted(t *testing.T) {
	f := setup(t)
	release, wait := holdSecureSlots(t, f)
	data := pdfOf(100)
	b := beginSecure(t, f, owner, len(data))
	_, err := putChunk(f, owner, b.Token, 0, data)
	wantKind(t, err, apperr.KindResourceExhausted)
	release()
	wait()
	// 名額放回後可以再上傳。
	if _, err := putChunk(f, owner, b.Token, 0, data); err != nil {
		t.Fatalf("名額放回後仍失敗：%v", err)
	}
}

// 兩個並行請求送同一段：讀回的內容是推進成功那方的，輸家的物件被刪，輸家得到帶 next_index 的 FailedPrecondition。
func TestSecureUpload_ConcurrentSameChunkReadsBackTheWinnerAndDeletesTheLoserObject(t *testing.T) {
	f := setup(t)
	size := media.SecureChunkSize + 1
	loserData, winnerData := make([]byte, size), make([]byte, size)
	for i := range size {
		loserData[i], winnerData[i] = 'L', 'W'
	}
	copy(loserData, "%PDF-1.7\n")
	copy(winnerData, "%PDF-1.7\n")
	b := beginSecure(t, f, owner, size)

	var loserKey string
	f.store.setAfterPut(func(key string) {
		f.store.setAfterPut(nil)
		loserKey = key
		// 輸家的物件已寫入、尚未推進：贏家的整個請求在這個空檔跑完。
		if _, err := putChunk(f, owner, b.Token, 0, winnerData[:b.ChunkSize]); err != nil {
			t.Errorf("贏家的段：%v", err)
		}
	})
	_, err := putChunk(f, owner, b.Token, 0, loserData[:b.ChunkSize])
	wantKind(t, err, apperr.KindFailedPrecondition)
	if got := nextIndexOf(t, err); got != "1" {
		t.Fatalf("輸家的 next_index = %q，預期 1", got)
	}
	if loserKey == "" || f.store.has(loserKey) {
		t.Fatalf("輸家的物件 %q 應該被刪掉", loserKey)
	}
	if got := f.store.Keys(); len(got) != 1 {
		t.Fatalf("儲存裡剩下 %v，預期只有贏家的一個物件", got)
	}
	if _, err := putChunk(f, owner, b.Token, 1, winnerData[b.ChunkSize:]); err != nil {
		t.Fatal(err)
	}
	a := confirmSecure(t, f, owner, b.Token)
	rc, err := f.m.ReadSecure(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, winnerData) {
		t.Fatal("讀回的內容不是推進成功那方寫的")
	}
}

// 跳號測試（store 層）：next_index 不等於這段序號時條件式推進 0 列、next_index 不變。
func TestAdvanceSecureUpload_SkippedIndexAdvancesNothing(t *testing.T) {
	f := setup(t)
	b := beginSecure(t, f, owner, 2*media.SecureChunkSize+1)
	uploadID, _ := stagedIDs(t, f, b.Token)
	ct := "application/pdf"
	n, err := store.New(f.d.Conn(t.Context())).AdvanceSecureUpload(t.Context(), store.AdvanceSecureUploadParams{ID: uploadID, ChunkIndex: 2, ContentType: &ct, Now: f.clock.Now()})
	if err != nil || n != 0 {
		t.Fatalf("AdvanceSecureUpload(跳到 2) = %d, %v，預期 0 列", n, err)
	}
	var next int
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT next_index FROM media.staged_uploads WHERE id = $1`, uploadID).Scan(&next); err != nil || next != 0 {
		t.Fatalf("next_index = %d, %v", next, err)
	}
}

// stagedIDs 以 token 查暫存列 ID 與預配的資產 ID。
func stagedIDs(t *testing.T, f *fixture, token string) (uploadID, assetID uuid.UUID) {
	t.Helper()
	h := sha256.Sum256([]byte(token))
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT id, planned_asset_id FROM media.staged_uploads WHERE token_hash = $1`, h[:]).Scan(&uploadID, &assetID); err != nil {
		t.Fatal(err)
	}
	return uploadID, assetID
}

func TestSecureUpload_NoPlaintextInAnyPutOrStoredObject(t *testing.T) {
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
func TestSecureUpload_CiphertextOverheadIsOutsideTheRecordedPlainSize(t *testing.T) {
	f := setup(t)
	data := pdfOf(70_000) // 單一上傳段、兩個 AEAD 段
	res := uploadSecure(t, f, owner, data)
	_, assetID := secureRow(t, f, res.Token)
	info, err := f.store.Head(t.Context(), chunkKeys(t, f, assetID)[0])
	if err != nil {
		t.Fatal(err)
	}
	// header 16 位元組、每個 AEAD 段 16 位元組 tag
	if want := int64(len(data)) + 16 + 2*16; info.Size != want {
		t.Fatalf("物件 %d 位元組，預期 %d", info.Size, want)
	}
	var size int64
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT size_bytes FROM media.assets WHERE id = $1`, assetID).Scan(&size); err != nil || size != int64(len(data)) {
		t.Fatalf("資產 size_bytes=%d err=%v，預期明文大小 %d", size, err, len(data))
	}
}

// 對應 test-map 列 97：機敏物件的實際型別一律是 octet-stream（原名 TestConfirmSecure_SkipsContentTypeCheck）。
func TestSecureUpload_ObjectTypeIsOctetStreamWhileAssetKeepsRecognisedType(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pngMagic)
	_, assetID := secureRow(t, f, res.Token)
	info, err := f.store.Head(t.Context(), chunkKeys(t, f, assetID)[0])
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
func TestSecureUpload_RefusesWhenKeyringMissing(t *testing.T) {
	f := setup(t)
	m := f.withKeyring(nil)
	ctx := t.Context()
	if _, err := m.BeginSecureUpload(ctx, owner, 10); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("BeginSecureUpload err = %v", err)
	}
	if _, err := m.PutSecureChunk(ctx, owner, "x", 0, bytes.NewReader(pdfOf(10)), 10); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("PutSecureChunk err = %v", err)
	}
	if _, err := m.PutSecure(ctx, owner, bytes.NewReader(pdfOf(10))); !errors.Is(err, media.ErrNoKeyring) {
		t.Fatalf("PutSecure err = %v", err)
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
	noObjects(t, f)
}

// 寫物件失敗：這一段回錯誤、會話不動（仍 UPLOADING、next_index 不變、沒有資產與物件）；儲存恢復後同一段可以重送完成。
func TestSecureUpload_PutFailureLeavesSessionResumable(t *testing.T) {
	f := setup(t)
	data := pdfOf(100)
	b := beginSecure(t, f, owner, len(data))
	f.store.mu.Lock()
	f.store.failPut = errors.New("r2 down")
	f.store.mu.Unlock()
	if _, err := putChunk(f, owner, b.Token, 0, data); err == nil {
		t.Fatal("Put 失敗卻成功")
	}
	uploadID, _ := stagedIDs(t, f, b.Token)
	if got := uploadState(t, f, uploadID); got != "UPLOADING" {
		t.Fatalf("state = %s", got)
	}
	if assetCount(t, f) != 0 || len(f.store.Keys()) != 0 {
		t.Fatalf("Put 失敗後資產列 %d、物件 %v", assetCount(t, f), f.store.Keys())
	}
	f.store.mu.Lock()
	f.store.failPut = nil
	f.store.mu.Unlock()
	if _, err := putChunk(f, owner, b.Token, 0, data); err != nil {
		t.Fatalf("儲存恢復後重送同一段失敗：%v", err)
	}
	if got := uploadState(t, f, uploadID); got != "READY" {
		t.Fatalf("state = %s", got)
	}
}

// insertCraftedSecure 以 SQL 建一筆機敏暫存列（測試用：模擬 GC 與處理 job 看到的列）。
func insertCraftedSecure(t *testing.T, f *fixture, state string) (uploadID, assetID uuid.UUID) {
	t.Helper()
	uploadID, assetID = uuid.New(), uuid.New()
	now := f.clock.Now()
	if _, err := f.d.Pool().Exec(t.Context(), `INSERT INTO media.staged_uploads (id, token_hash, uploader_id, uploader_admin, visibility, declared_size, declared_content_type, state, final_key,
			planned_asset_id, chunk_count, next_index, kek_id, wrapped_dek, expires_at, created_at, updated_at)
		VALUES ($1, $2, 1001, false, 'SECURE', 10, 'application/pdf', $3, $4, $5, 1, 0, 'k1', $6, $7, $8, $8)`,
		uploadID, bytes.Repeat([]byte{7}, 32), state, "secure/"+assetID.String()+"/", assetID, []byte("wrapped"), now.Add(media.UploadTTL), now); err != nil {
		t.Fatal(err)
	}
	return uploadID, assetID
}

// 機敏列走處理 job 是永久失敗（機敏上傳不經處理 job）。
func TestProcess_SecureRowIsPermanentFailure(t *testing.T) {
	f := setup(t)
	id, _ := insertCraftedSecure(t, f, "UPLOADED")
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
	replaceChunkObject(t, f, chunkKeys(t, f, assetID)[0], mutate)
}

// replaceChunkObject 把一個段物件換成 mutate 之後的內容。
func replaceChunkObject(t *testing.T, f *fixture, key string, mutate func([]byte) []byte) {
	t.Helper()
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

// 資產讀到的每一段都要在原位：對調兩段、刪掉最後一段（列與物件）、放進別的資產的段、改段總數，各讓讀取失敗而且不交出被動過的段的明文。
func TestReadSecure_SwappedDroppedForeignOrRecountedChunksFailToRead(t *testing.T) {
	data := threeChunkData()
	cases := map[string]func(t *testing.T, f *fixture, a media.ConfirmedAsset){
		"swapped-chunk-objects": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			keys := chunkKeys(t, f, a.ID)
			obj := func(k string) []byte {
				rc, err := f.store.Get(t.Context(), k)
				if err != nil {
					t.Fatal(err)
				}
				return readAll(t, rc)
			}
			b0, b1 := obj(keys[0]), obj(keys[1])
			if err := f.store.Fake.Put(t.Context(), putInput(keys[0], b1)); err != nil {
				t.Fatal(err)
			}
			if err := f.store.Fake.Put(t.Context(), putInput(keys[1], b0)); err != nil {
				t.Fatal(err)
			}
		},
		"swapped-chunk-rows": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.secure_chunks SET idx = CASE idx WHEN 0 THEN 1 WHEN 1 THEN 0 ELSE idx END WHERE asset_id = $1`, a.ID); err != nil {
				t.Fatal(err)
			}
		},
		"dropped-last-chunk-row": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			if _, err := f.d.Pool().Exec(t.Context(), `DELETE FROM media.secure_chunks WHERE asset_id = $1 AND idx = 2`, a.ID); err != nil {
				t.Fatal(err)
			}
		},
		"dropped-last-chunk-and-count": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			if _, err := f.d.Pool().Exec(t.Context(), `DELETE FROM media.secure_chunks WHERE asset_id = $1 AND idx = 2`, a.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.assets SET chunk_count = 2 WHERE id = $1`, a.ID); err != nil {
				t.Fatal(err)
			}
		},
		"changed-chunk-count": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.assets SET chunk_count = 4 WHERE id = $1`, a.ID); err != nil {
				t.Fatal(err)
			}
		},
		"missing-chunk-object": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			if err := f.store.Fake.Delete(t.Context(), chunkKeys(t, f, a.ID)[1]); err != nil {
				t.Fatal(err)
			}
		},
		"foreign-asset-chunk": func(t *testing.T, f *fixture, a media.ConfirmedAsset) {
			other := confirmSecure(t, f, owner, uploadSecure(t, f, owner, threeChunkData()).Token)
			foreign := chunkKeys(t, f, other.ID)[1]
			if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.secure_chunks SET object_key = $1 WHERE asset_id = $2 AND idx = 1`, foreign, a.ID); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			a := confirmSecure(t, f, owner, uploadSecure(t, f, owner, data).Token)
			mutate(t, f, a)
			rc, err := f.m.ReadSecure(t.Context(), a.ID)
			if err != nil {
				return
			}
			defer func() { _ = rc.Close() }()
			got, err := io.ReadAll(rc)
			if err == nil {
				t.Fatal("被動過的段讀得完")
			}
			if len(got) > media.SecureChunkSize {
				t.Fatalf("錯誤之前交出 %d 位元組，超過第一段", len(got))
			}
		})
	}
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
	oldKey := chunkKeys(t, f, old.ID)[0]
	rc, _ := f.store.Get(t.Context(), oldKey)
	before := readAll(t, rc)

	rotated := f.withKeyring(mustKeyring(t, kek("k2", 2)+","+kek("k1", 1)))
	// 換 primary 之後新上傳用 k2；它不該被重新包裹。
	fresh := uploadSecureVia(t, rotated, owner, pdfOf(100))
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

// 金鑰輪替：還在 UPLOADING 的會話也被重新包裹，只留新 KEK 後會話照樣完成並讀得回。
func TestRewrapDEKs_UploadingSessionIsRewrappedAndCompletesWithOnlyTheNewKek(t *testing.T) {
	f := setup(t)
	data := threeChunkData()
	b := beginSecure(t, f, owner, len(data))
	if _, err := putChunk(f, owner, b.Token, 0, data[:b.ChunkSize]); err != nil {
		t.Fatal(err)
	}
	uploadID, _ := stagedIDs(t, f, b.Token)
	rotated := f.withKeyring(mustKeyring(t, kek("k2", 2)+","+kek("k1", 1)))
	n, err := rotated.RewrapDEKs(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("RewrapDEKs = %d, %v，預期更新上傳中的會話 1 筆", n, err)
	}
	var kekID string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT kek_id FROM media.staged_uploads WHERE id = $1`, uploadID).Scan(&kekID); err != nil || kekID != "k2" {
		t.Fatalf("暫存列的 kek_id=%s err=%v", kekID, err)
	}
	onlyNew := f.withKeyring(mustKeyring(t, kek("k2", 2)))
	for i := 1; i < b.ChunkCount; i++ {
		start := i * b.ChunkSize
		end := min(start+b.ChunkSize, len(data))
		if _, err := onlyNew.PutSecureChunk(t.Context(), owner, b.Token, i, bytes.NewReader(data[start:end]), int64(end-start)); err != nil {
			t.Fatalf("只留新 KEK 後第 %d 段：%v", i, err)
		}
	}
	a := confirmSecure(t, f, owner, b.Token)
	var assetKek string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT kek_id FROM media.assets WHERE id = $1`, a.ID).Scan(&assetKek); err != nil || assetKek != "k2" {
		t.Fatalf("資產的 kek_id=%s err=%v，預期從暫存列搬來的 k2", assetKek, err)
	}
	rc, err := onlyNew.ReadSecure(t.Context(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, data) {
		t.Fatal("輪替後完成的上傳讀回的內容不同")
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
