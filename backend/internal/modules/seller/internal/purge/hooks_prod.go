//go:build !integration

package purge

type hooks struct{}

func (hooks) afterCandidates() {}

func (hooks) afterPurgeCandidates() {}

func (hooks) beforePurgeBatch() {}
