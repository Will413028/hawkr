package deliver

import (
	"maps"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

func TestPushMessageGolden(t *testing.T) {
	id := uuid.MustParse("0190a000-0000-7000-8000-0000000000a1")
	full := pushMessage(plan{rendered: registry.Rendered{Title: "T", Body: "B", DeepLink: "/orders/1", StateHint: model.StateHintAfterSale}}, &id, "https://img.example/a.jpg")
	want := map[string]string{"deep_link": "/orders/1", "state_refresh_hints": "STATE_REFRESH_HINT_AFTER_SALE", "inbox_item_id": id.String()}
	if full.Title != "T" || full.Body != "B" || full.ImageURL != "https://img.example/a.jpg" || !maps.Equal(full.Data, want) {
		t.Fatalf("message = %+v", full)
	}
	bare := pushMessage(plan{rendered: registry.Rendered{Title: "T", Body: "B"}}, nil, "")
	if len(bare.Data) != 0 || bare.ImageURL != "" {
		t.Fatalf("沒有值的 key 要省略：%+v", bare)
	}
}

// 提示名稱必須等於 proto enum StateRefreshHint 的名稱。
func TestEveryStateHintMapsToAProtoEnumName(t *testing.T) {
	want := map[model.StateHint]string{
		model.StateHintOrder: "STATE_REFRESH_HINT_ORDER", model.StateHintAfterSale: "STATE_REFRESH_HINT_AFTER_SALE", model.StateHintSellerStatus: "STATE_REFRESH_HINT_SELLER_STATUS",
	}
	for _, h := range model.StateHints() {
		if hints(h) != want[h] {
			t.Errorf("%s → %q，預期 %q", h, hints(h), want[h])
		}
	}
}
