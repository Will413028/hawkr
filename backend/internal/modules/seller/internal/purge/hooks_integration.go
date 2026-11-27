//go:build integration

package purge

type hooks struct {
	candidates      func()
	purgeCandidates func()
	beforePurge     func()
}

func (h hooks) afterCandidates() {
	if h.candidates != nil {
		h.candidates()
	}
}

func (h hooks) afterPurgeCandidates() {
	if h.purgeCandidates != nil {
		h.purgeCandidates()
	}
}

// SetAfterCandidates 設定測試用的 hook；只在建立服務之後、清理開始之前呼叫。
func (s *Service) SetAfterCandidates(fn func()) { s.hooks.candidates = fn }

// SetAfterPurgeCandidates 設定第二段清理的測試用 hook；只在建立服務之後、清理開始之前呼叫。
func (s *Service) SetAfterPurgeCandidates(fn func()) { s.hooks.purgeCandidates = fn }

func (h hooks) beforePurgeBatch() {
	if h.beforePurge != nil {
		h.beforePurge()
	}
}

// SetBeforePurgeBatch 設定第二段清理選候選之前的測試用 hook；只在建立服務之後、清理開始之前呼叫。
func (s *Service) SetBeforePurgeBatch(fn func()) { s.hooks.beforePurge = fn }
