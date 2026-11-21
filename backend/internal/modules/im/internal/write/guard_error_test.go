package write

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 兩個守門都回 NotFound 時，以寄件人那一方為準：店家寄件人雙方都停用是「寄件人已停用」，不是 RECEIVER_DELETED；
// 只有收件人停用才是 RECEIVER_DELETED；都通過回 nil。
func TestClosedGuardErrorPrefersTheSender(t *testing.T) {
	s := New(Deps{})
	l := uitext.LocaleFrom(context.Background())
	gone := apperr.NotFound("gone")
	cases := []struct {
		name            string
		sender          model.PartyKind
		buyerErr, store error
		wantNil         bool
		wantReceiver    bool
	}{
		{"都通過", model.PartyStore, nil, nil, true, false},
		{"店家寄件人、雙方停用", model.PartyStore, gone, gone, false, false},
		{"買家寄件人、雙方停用", model.PartyBuyer, gone, gone, false, false},
		{"店家寄件人、只有買家停用", model.PartyStore, gone, nil, false, true},
		{"買家寄件人、只有店家停用", model.PartyBuyer, nil, gone, false, true},
		{"店家寄件人、只有店家停用", model.PartyStore, nil, gone, false, false},
		{"買家寄件人、只有買家停用", model.PartyBuyer, gone, nil, false, false},
	}
	for _, c := range cases {
		err := s.closedGuardError(l, c.buyerErr, c.store, c.sender)
		if c.wantNil {
			if err != nil {
				t.Errorf("%s：%v", c.name, err)
			}
			continue
		}
		if apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：kind %v", c.name, apperr.KindOf(err))
			continue
		}
		if got := model.ReasonOf(err) == model.ReasonReceiverDeleted; got != c.wantReceiver {
			t.Errorf("%s：receiver_deleted=%v，預期 %v", c.name, got, c.wantReceiver)
		}
	}
}
