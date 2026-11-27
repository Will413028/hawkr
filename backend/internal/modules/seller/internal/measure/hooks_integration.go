//go:build integration

package measure

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
