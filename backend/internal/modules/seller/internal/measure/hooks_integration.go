//go:build integration

package measure

// requestHooks 只在 integration build tag 下有內容：賣家請求的鎖（lockForRequest）在不加鎖的預讀之後、以及鎖與重讀都成功之後執行，
// 讓測試固定申覆與個資清理、關聯終止新增的交錯（step10e-3cb §7.3）。正式程式的版本是空的（hooks_prod.go），不編譯 hook 欄位與 setter。
type requestHooks struct {
	afterPreRead, afterLock func()
}

func (h requestHooks) preRead() {
	if h.afterPreRead != nil {
		h.afterPreRead()
	}
}

func (h requestHooks) locked() {
	if h.afterLock != nil {
		h.afterLock()
	}
}

// SetRequestHooks 設定測試用的 hook；只在建立服務之後、請求開始之前呼叫。
func (s *Service) SetRequestHooks(afterPreRead, afterLock func()) {
	s.hooks = requestHooks{afterPreRead: afterPreRead, afterLock: afterLock}
}
