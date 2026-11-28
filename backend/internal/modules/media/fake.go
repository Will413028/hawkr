package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Fake 是記憶體內、不連儲存與資料庫的 media 模組替身，給擁有者模組的測試用，方法與 *Module 的同名方法同形。
// 同一個 staged token 確認得到同一個資產；Confirm 與 MarkDeleted 要求 ctx 帶 transaction，與 *Module 相同；token 以 "bad" 開頭時 Confirm
// 回 InvalidInput（token 不可用）。
type Fake struct {
	mu sync.Mutex
	// Key 決定確認後的 object key；nil 時是 public/<token>.jpg。
	Key func(token string) string
	// FailConfirm、FailMarkDeleted、FailReadSecure 非 nil 時，對應的呼叫回這個錯誤、不生效。
	FailConfirm, FailMarkDeleted, FailReadSecure error
	// Staged 是 token 上傳時的可見性；Confirm 的 want 與它不同時回與 *Module 相同的 InvalidInput（可見性不符）。不在表裡的 token 不檢查。
	Staged map[string]Visibility

	confirmed map[uuid.UUID]ConfirmedAsset
	deleted   map[uuid.UUID]bool
	secure    map[uuid.UUID][]byte
	calls     []string
	copies    int
	holds     map[fakeHold]time.Time
}

type fakeHold struct {
	asset    uuid.UUID
	source   string
	sourceID uuid.UUID
}

// NewFake 建立空的 Fake。
func NewFake() *Fake {
	return &Fake{confirmed: map[uuid.UUID]ConfirmedAsset{}, deleted: map[uuid.UUID]bool{}, secure: map[uuid.UUID][]byte{}, holds: map[fakeHold]time.Time{}}
}

// AssetFor 是 token 確認後的資產 ID。
func AssetFor(token string) uuid.UUID {
	sum := sha256.Sum256([]byte(token))
	return uuid.UUID(sum[:16])
}

func requireTx(ctx context.Context) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return ErrNoTransaction
	}
	return nil
}

// Confirm 是 *Module.Confirm 的替身。
func (f *Fake) Confirm(ctx context.Context, u Uploader, token string, want Visibility) (ConfirmedAsset, error) {
	if err := requireTx(ctx); err != nil {
		return ConfirmedAsset{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "confirm "+token)
	if f.FailConfirm != nil {
		return ConfirmedAsset{}, f.FailConfirm
	}
	if u.ID <= 0 || strings.TrimSpace(token) == "" || strings.HasPrefix(token, "bad") {
		return ConfirmedAsset{}, apperr.InvalidInput("圖片上傳已失效，請重新上傳")
	}
	if vis, ok := f.Staged[token]; ok && vis != want {
		if want == VisibilitySecure {
			return ConfirmedAsset{}, apperr.InvalidInput("%s", text.MediaVisibilitySecureRequired(uitext.LocaleFrom(ctx)).Text)
		}
		return ConfirmedAsset{}, apperr.InvalidInput("%s", text.MediaVisibilityPublicRequired(uitext.LocaleFrom(ctx)).Text)
	}
	id := AssetFor(token)
	key := "public/" + token + ".jpg"
	contentType := "image/jpeg"
	if want == VisibilitySecure {
		key, contentType = "secure/"+id.String()+"/", "application/pdf"
	}
	if f.Key != nil {
		key = f.Key(token)
	}
	a := ConfirmedAsset{ID: id, ObjectKey: key, Visibility: want, ContentType: contentType, SizeBytes: 1}
	f.confirmed[id] = a
	return a, nil
}

// CopyPublic 是 *Module.CopyPublic 的替身：來源沒被 MarkDeleted 就成功（不要求來源是這個 Fake 確認的），回新的 token（每次呼叫不同），之後 Confirm 得到另一個資產。
func (f *Fake) CopyPublic(ctx context.Context, u Uploader, sourceID uuid.UUID) (PutPublicResult, error) {
	if _, ok := db.TxFrom(ctx); ok {
		return PutPublicResult{}, model.ErrInTransaction
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "copy-public "+sourceID.String())
	if f.deleted[sourceID] || u.ID <= 0 {
		return PutPublicResult{}, apperr.NotFound("找不到圖片")
	}
	f.copies++
	return PutPublicResult{Token: fmt.Sprintf("copy-%d-%s", f.copies, sourceID), ContentType: "image/jpeg", SizeBytes: 1}, nil
}

// MarkDeleted 是 *Module.MarkDeleted 的替身。
func (f *Fake) MarkDeleted(ctx context.Context, assetIDs []uuid.UUID) error {
	if err := requireTx(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "mark-deleted")
	if f.FailMarkDeleted != nil {
		return f.FailMarkDeleted
	}
	for _, id := range assetIDs {
		f.deleted[id] = true
	}
	return nil
}

func (f *Fake) Hold(ctx context.Context, assetIDs []uuid.UUID, source string, sourceID uuid.UUID, until time.Time) error {
	if err := requireTx(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "hold "+source)
	for _, id := range assetIDs {
		if f.deleted[id] {
			return ErrAssetGone
		}
	}
	for _, id := range assetIDs {
		k := fakeHold{asset: id, source: source, sourceID: sourceID}
		if until.After(f.holds[k]) {
			f.holds[k] = until
		}
	}
	return nil
}

func (f *Fake) ExtendHold(ctx context.Context, source string, sourceID uuid.UUID, until time.Time) error {
	if err := requireTx(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "extend-hold "+source)
	found := false
	for k, v := range f.holds {
		if k.source == source && k.sourceID == sourceID {
			found = true
			if until.After(v) {
				f.holds[k] = until
			}
		}
	}
	if !found {
		return ErrHoldNotFound
	}
	return nil
}

func (f *Fake) ReleaseHold(ctx context.Context, source string, sourceID uuid.UUID) error {
	if err := requireTx(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "release-hold "+source)
	for k := range f.holds {
		if k.source == source && k.sourceID == sourceID {
			delete(f.holds, k)
		}
	}
	return nil
}

func (f *Fake) IsHeld(assetID uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.holds {
		if k.asset == assetID {
			return true
		}
	}
	return false
}

func (f *Fake) CopyToSecure(ctx context.Context, u Uploader, sourceID uuid.UUID) (uuid.UUID, error) {
	if err := requireTx(ctx); err != nil {
		return uuid.UUID{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "copy-to-secure "+sourceID.String())
	if f.deleted[sourceID] || u.ID <= 0 {
		return uuid.UUID{}, apperr.NotFound("找不到圖片")
	}
	id := uuid.New()
	f.confirmed[id] = ConfirmedAsset{ID: id, Visibility: VisibilitySecure, SizeBytes: 1}
	return id, nil
}

// ReadSecure 是 *Module.ReadSecure 的替身：只讀得到 PutSecure 放的資產（型別由內容辨認），其餘 NotFound。
func (f *Fake) ReadSecure(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, string, error) {
	if _, ok := db.TxFrom(ctx); ok {
		return nil, "", model.ErrInTransaction
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "read-secure "+assetID.String())
	if f.FailReadSecure != nil {
		return nil, "", f.FailReadSecure
	}
	data, ok := f.secure[assetID]
	if !ok || f.deleted[assetID] {
		return nil, "", apperr.NotFound("找不到檔案")
	}
	return io.NopCloser(bytes.NewReader(data)), http.DetectContentType(data), nil
}

// PutSecure 放一份機敏資產的明文。
func (f *Fake) PutSecure(assetID uuid.UUID, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secure[assetID] = slices.Clone(data)
}

// IsConfirmed 回報 asset 是否被 Confirm 過。
func (f *Fake) IsConfirmed(assetID uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.confirmed[assetID]
	return ok
}

// IsDeleted 回報 asset 是否被 MarkDeleted 過。
func (f *Fake) IsDeleted(assetID uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleted[assetID]
}

// Calls 回傳依序記錄的呼叫。
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// MemoryStore 是記憶體的 ObjectStore（預簽網址不可連線），只給 Go 測試接線；它計算發出的預簽次數。正式啟動不會建構它。
type MemoryStore struct {
	ObjectStore
	presigns atomic.Int64
}

// NewMemoryStore 回傳空的 MemoryStore。
func NewMemoryStore() *MemoryStore { return &MemoryStore{ObjectStore: storage.NewFake()} }

// PresignPut 實作 ObjectStore。
func (s *MemoryStore) PresignPut(ctx context.Context, in storage.PresignPutInput) (storage.PresignedPut, error) {
	s.presigns.Add(1)
	return s.ObjectStore.PresignPut(ctx, in)
}

// Presigns 回傳已發出的預簽網址數。
func (s *MemoryStore) Presigns() int { return int(s.presigns.Load()) }
