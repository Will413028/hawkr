//go:build !integration

package measure

// requestHooks 在正式程式是空的（測試的版本見 hooks_integration.go）。
type requestHooks struct{}

func (requestHooks) preRead() {}
func (requestHooks) locked()  {}
