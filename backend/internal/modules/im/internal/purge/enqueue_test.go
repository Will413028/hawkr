package purge

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
)

// 排入前的驗證（原本由已刪除的表的 CHECK 守）：身分種類要合法、id 要為正；不合法的不碰 Jobs。
func TestEnqueueEraseRejectsInvalidParties(t *testing.T) {
	t.Parallel()
	s := New(Deps{})
	for _, c := range []struct {
		kind model.PartyKind
		id   int64
	}{{model.PartyBuyer, 0}, {model.PartyStore, -1}, {model.PartyKind("OTHER"), 5}, {model.PartyKind(""), 5}} {
		if err := s.EnqueueErase(context.Background(), c.kind, c.id); err == nil {
			t.Errorf("%q/%d 應該被拒絕", c.kind, c.id)
		}
	}
}
