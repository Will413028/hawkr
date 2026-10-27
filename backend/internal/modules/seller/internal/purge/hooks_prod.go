//go:build !integration

package purge

// hooks 在正式程式是空的（測試的版本見 hooks_integration.go）。
type hooks struct{}

func (hooks) afterCandidates() {}

func (hooks) afterPurgeCandidates() {}
