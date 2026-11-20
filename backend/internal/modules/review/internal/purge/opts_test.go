package purge

import (
	"testing"

	"github.com/riverqueue/river/rivertype"
)

// 執行中的清除 job 之外又排入的那一次要排在它後面，唯一性不能含 Running。
func TestPurgeUniquenessExcludesRunning(t *testing.T) {
	t.Parallel()
	for _, st := range (PurgeAuthorArgs{}).InsertOpts().UniqueOpts.ByState {
		if st == rivertype.JobStateRunning {
			t.Fatal("ByState 含 Running：執行中排入的 job 會被丟掉")
		}
	}
}
