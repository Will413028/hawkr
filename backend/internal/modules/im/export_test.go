package im

// SetLiveHooks 設定即時通道的測試縫：afterRegister 在 Subscribe 登記之後、補洞之前；afterBackfill 在第一輪補洞之後、Ready 之前。
// 必須在任何 Subscribe 之前呼叫。
func SetLiveHooks(m *Module, afterRegister, afterBackfill func()) {
	m.hub.AfterRegister, m.hub.AfterBackfill = afterRegister, afterBackfill
}

// LiveReceived 是這個模組的監聽端收到的通知總數。
func LiveReceived(m *Module) int64 { return m.hub.Received() }
