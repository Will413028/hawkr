//go:build integration

package seller

// SetPurgeAfterCandidates 讓測試固定第一段清理（退場）與寫入 legal hold 的一方的交錯（purge.Service.SetAfterCandidates）。
func (m *Module) SetPurgeAfterCandidates(fn func()) { m.purge.SetAfterCandidates(fn) }

// SetPurgeAfterPurgeCandidates 讓測試固定第二段清理（個資）與寫入 legal hold 的一方的交錯（purge.Service.SetAfterPurgeCandidates）。
func (m *Module) SetPurgeAfterPurgeCandidates(fn func()) { m.purge.SetAfterPurgeCandidates(fn) }

// SetRequestHooks 讓測試固定賣家請求（申覆）的鎖與個資清理、關聯終止新增的交錯（measure.Service.SetRequestHooks）。
func (m *Module) SetRequestHooks(afterPreRead, afterLock func()) {
	m.measure.SetRequestHooks(afterPreRead, afterLock)
}

// SetPurgeAfterFoodCandidates 讓測試固定食品登錄審核紀錄的刪除與寫入來函保存的一方的交錯（purge.Service.SetAfterFoodCandidates）。
func (m *Module) SetPurgeAfterFoodCandidates(fn func()) { m.purge.SetAfterFoodCandidates(fn) }
