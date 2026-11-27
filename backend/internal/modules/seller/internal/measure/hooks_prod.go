//go:build !integration

package measure

type requestHooks struct{}

func (requestHooks) preRead() {}
func (requestHooks) locked()  {}
