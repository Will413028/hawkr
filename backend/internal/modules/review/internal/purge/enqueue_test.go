package purge

import (
	"context"
	"testing"
)

// 排入前的驗證（原本由已刪除的表的 CHECK 守）：買家 id 要為正；不合法的不碰 Jobs。
func TestEnqueuePurgeRejectsNonPositiveBuyer(t *testing.T) {
	t.Parallel()
	s := New(Deps{})
	for _, id := range []int64{0, -1} {
		if err := s.EnqueuePurge(context.Background(), id); err == nil {
			t.Errorf("buyer %d 應該被拒絕", id)
		}
	}
}
