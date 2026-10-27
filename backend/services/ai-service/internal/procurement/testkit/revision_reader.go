package testkit

import (
	"context"
	"encoding/json"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// FakeRevisionReader 是 pricing.RevisionReader 的共用替身（GetSlots／
// GetSnapshot），供 pricing 與 usecase 測試共用建構——value receiver，呼叫端
// 一律用值字面量 FakeRevisionReader{...} 直接傳入。Resolution 是品項解析
// 結果，住在 Slots 裡（ADR 2026-09-04-crop-resolution-into-slots），獨立成
// 一個 fixture 欄位讓測試不必自己把它塞進 Slots 字面量；nil 代表這個 request
// 品項還沒解析完成。
type FakeRevisionReader struct {
	Slots      domain.Slots
	Resolution *domain.ItemResolution
	RevisionID int64
	Err        error
	// SnapshotSlots 只在少數測試需要「GetSnapshot 帶的 slots 跟 GetSlots 回的
	// 不同」時設定，用來證明讀取端讀的是哪一支方法。真實 repository.Repo 兩支
	// 方法必然一致（GetSlots 就是 GetSnapshot 的 SlotsJSON 反序列化），所以
	// 預設 nil＝兩者同源，只有刻意製造分歧的測試才會設定它。
	SnapshotSlots *domain.Slots
}

func (f FakeRevisionReader) GetSlots(ctx context.Context, requestID int64, buyerID identity.BuyerID) (domain.Slots, error) {
	s := f.Slots
	s.ItemResolution = f.Resolution
	return s, f.Err
}

// GetSnapshot 回傳的 Revision 帶 SlotsJSON——對齊真實 repository.Repo.GetSnapshot
// （revision 列本來就存 slots_json，GetSlots 只是它的反序列化包裝）。原本這個
// fake 只回 ID，於是任何從 rev.SlotsJSON 讀 slots 的呼叫端在測試裡都拿不到值。
func (f FakeRevisionReader) GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	if f.Err != nil {
		return repository.Request{}, repository.Revision{}, f.Err
	}
	s := f.Slots
	// 只在 Resolution 非 nil 時覆寫：這個 fake 的既有用法是「Slots 不帶解析結果、
	// 由 Resolution 欄位另外供給」，但把解析結果直接寫在 Slots 字面量裡的 fixture
	// 同樣合法，無條件覆寫會把後者清成 nil。
	if f.Resolution != nil {
		s.ItemResolution = f.Resolution
	}
	if f.SnapshotSlots != nil {
		s = *f.SnapshotSlots
	}
	slotsJSON, err := json.Marshal(s)
	if err != nil {
		return repository.Request{}, repository.Revision{}, err
	}
	return repository.Request{}, repository.Revision{ID: f.RevisionID, SlotsJSON: slotsJSON}, nil
}
