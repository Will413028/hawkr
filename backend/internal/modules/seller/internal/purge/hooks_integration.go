//go:build integration

package purge

// hooks 只在 integration build tag 下有內容；正式程式的版本是空的（hooks_prod.go）。
type hooks struct {
	candidates      func()
	purgeCandidates func()
	foodCandidates  func()
}

// afterCandidates 在第一段（退場）第一次 RetireDueStores 之後、再判斷之前執行，讓測試固定寫入 hold 的一方與清理的交錯。
func (h hooks) afterCandidates() {
	if h.candidates != nil {
		h.candidates()
	}
}

// afterPurgeCandidates 在第二段（個資清理）第一次 RetireDueStores（second_stage）之後、再判斷之前執行（同一種交錯）。
func (h hooks) afterPurgeCandidates() {
	if h.purgeCandidates != nil {
		h.purgeCandidates()
	}
}

// afterFoodCandidates 在 ExpiredFoodRegistrations 鎖住候選之後、問 StoreHolds 之前執行（審核紀錄刪除的同一種交錯）。
func (h hooks) afterFoodCandidates() {
	if h.foodCandidates != nil {
		h.foodCandidates()
	}
}

// SetAfterCandidates 設定測試用的 hook；只在建立服務之後、清理開始之前呼叫。
func (s *Service) SetAfterCandidates(fn func()) { s.hooks.candidates = fn }

// SetAfterPurgeCandidates 設定第二段清理的測試用 hook；只在建立服務之後、清理開始之前呼叫。
func (s *Service) SetAfterPurgeCandidates(fn func()) { s.hooks.purgeCandidates = fn }

// SetAfterFoodCandidates 設定審核紀錄刪除的測試用 hook；只在建立服務之後、清理開始之前呼叫。
func (s *Service) SetAfterFoodCandidates(fn func()) { s.hooks.foodCandidates = fn }
