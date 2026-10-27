package media

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Fake 是記憶體內的 Media，給測試用。它照 media-service 的語意記錄引用：(asset, ref_type, ref_id) 唯一、Link 對已存在
// 的引用什麼都不做（ON CONFLICT DO NOTHING）、Unlink 只移除精確列出的引用；同一個 staged token 轉正得到同一個資產。
// 呼叫發生在 DB transaction 內時回錯誤（媒體呼叫不得在 transaction 內）。
type Fake struct {
	mu sync.Mutex
	// Key 決定轉正後的 object key；nil 時是 public/<ref_type>/<token>.jpg。
	Key func(refType, token string) string
	// AfterFinalize 在轉正完成、結果回給呼叫方之前執行：測試用它在「轉正之後、業務列 commit 之前」插入同步 job 或別的寫入。
	AfterFinalize func(ctx context.Context, a Asset)
	// BeforeApply 在 Link 或 Unlink（op 是 "link" 或 "unlink"）生效之前執行：測試用它在同步進行到一半時改變擁有者的狀態。
	BeforeApply func(ctx context.Context, op string, ref Ref, assetIDs []string)
	// FailFinalize、FailLink、FailUnlink 非 nil 時，對應的呼叫回這個錯誤、不生效。
	FailFinalize, FailLink, FailUnlink error
	// FailReadSecure 非 nil 時，ReadSecure 回這個錯誤。
	FailReadSecure error
	// BeforeReadSecure 在 ReadSecure 讀取之前執行（沒有持有鎖）：測試用它在呼叫進行中改變擁有者的狀態（例如讓同步結算）。
	BeforeReadSecure func()
	// NotSecure 為 true 時，轉正回報 is_secure=false（預設只有證件與治理佐證的 ref 類型 RefTypeCertification、
	// RefTypeGovernanceEvidence 回報機敏）。
	NotSecure bool

	assets   map[string]Asset
	secure   map[fakeSecure]SecureAsset
	refs     map[fakeRef]bool
	zombies  []func()
	zombieOn map[string]bool
	calls    []string
}

type fakeRef struct{ asset, refType, refID string }

type fakeSecure struct{ objectKey, refType, refID string }

// NewFake 建立空的 Fake。
func NewFake() *Fake {
	return &Fake{assets: map[string]Asset{}, secure: map[fakeSecure]SecureAsset{}, refs: map[fakeRef]bool{}, zombieOn: map[string]bool{}}
}

var _ Media = (*Fake)(nil)

func (f *Fake) check(ctx context.Context) error {
	if _, ok := db.TxFrom(ctx); ok {
		return errors.New("media 呼叫發生在 transaction 內")
	}
	return nil
}

// AssetFor 是 token 轉正後的 asset ID。
func AssetFor(token string) string { return "asset-" + token }

// Finalize 實作 Media。
func (f *Fake) Finalize(ctx context.Context, u Uploader, token, refType string) (Asset, error) {
	if err := f.check(ctx); err != nil {
		return Asset{}, err
	}
	if err := CheckFinalizeArgs(u, token, refType); err != nil {
		return Asset{}, fmt.Errorf("media：轉正（呼叫方的程式錯誤）：%w", err)
	}
	f.mu.Lock()
	if f.FailFinalize != nil {
		err := f.FailFinalize
		f.mu.Unlock()
		return Asset{}, err
	}
	key := fmt.Sprintf("public/%s/%s.jpg", refType, token)
	if f.Key != nil {
		key = f.Key(refType, token)
	}
	a := Asset{ID: AssetFor(token), ObjectKey: key, Secure: (refType == RefTypeCertification || refType == RefTypeGovernanceEvidence) && !f.NotSecure}
	f.assets[a.ID] = a
	f.calls = append(f.calls, "finalize "+token)
	hook := f.AfterFinalize
	f.mu.Unlock()
	if hook != nil {
		hook(ctx, a)
	}
	return a, nil
}

// Link 實作 Media。
func (f *Fake) Link(ctx context.Context, ref Ref, assetIDs []string) error {
	return f.apply(ctx, "link", ref, assetIDs, f.FailLink, func() {
		for _, id := range assetIDs {
			f.refs[fakeRef{id, ref.Type, ref.ID}] = true
		}
	})
}

// Unlink 實作 Media。
func (f *Fake) Unlink(ctx context.Context, ref Ref, assetIDs []string) error {
	return f.apply(ctx, "unlink", ref, assetIDs, f.FailUnlink, func() {
		for _, id := range assetIDs {
			delete(f.refs, fakeRef{id, ref.Type, ref.ID})
		}
	})
}

func (f *Fake) apply(ctx context.Context, op string, ref Ref, ids []string, fail error, do func()) error {
	if err := f.check(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	hook := f.BeforeApply
	f.mu.Unlock()
	if hook != nil {
		hook(ctx, op, ref, ids)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf("%s %s/%s %v", op, ref.Type, ref.ID, ids))
	if fail != nil {
		return fail
	}
	if f.zombieOn[op] {
		f.zombies = append(f.zombies, func() { do() })
		return context.DeadlineExceeded
	}
	do()
	return nil
}

// Zombie 讓之後的 op（"link" 或 "unlink"）呼叫回逾時，但請求仍留在遠端：直到 FlushZombies 才生效，模擬逾時之後才在
// 遠端執行的 RPC。
func (f *Fake) Zombie(op string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.zombieOn[op] = on
}

// FlushZombies 讓遠端還在排隊的逾時請求依序生效。
func (f *Fake) FlushZombies() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, do := range f.zombies {
		do()
	}
	f.zombies = nil
}

// Referenced 回傳 ref 目前引用的資產，已排序。
func (f *Fake) Referenced(ref Ref) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for r := range f.refs {
		if r.refType == ref.Type && r.refID == ref.ID {
			out = append(out, r.asset)
		}
	}
	sort.Strings(out)
	return out
}

// HasRef 回報 ref 是否引用 asset。
func (f *Fake) HasRef(ref Ref, assetID string) bool {
	return slices.Contains(f.Referenced(ref), assetID)
}

// Calls 回傳依序記錄的呼叫。
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// PutSecure 放一份機敏資產：只有 (objectKey, ref.Type, ref.ID) 全等的 ReadSecure 讀得到。
func (f *Fake) PutSecure(objectKey string, ref Ref, a SecureAsset) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secure[fakeSecure{objectKey, ref.Type, ref.ID}] = a
}

// ReadSecure 實作 Media：三元組全等才讀得到，其餘（查無與越權）一律 NotFound。
func (f *Fake) ReadSecure(ctx context.Context, objectKey string, ref Ref) (SecureAsset, error) {
	if err := f.check(ctx); err != nil {
		return SecureAsset{}, err
	}
	f.mu.Lock()
	hook := f.BeforeReadSecure
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf("read-secure %s/%s %s", ref.Type, ref.ID, objectKey))
	if f.FailReadSecure != nil {
		return SecureAsset{}, f.FailReadSecure
	}
	a, ok := f.secure[fakeSecure{objectKey, ref.Type, ref.ID}]
	if !ok {
		return SecureAsset{}, apperr.NotFound("找不到檔案")
	}
	return a, nil
}
