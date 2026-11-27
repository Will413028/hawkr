package deliver

import (
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
)

// 推播 data 的 key 與值的前綴：App 依這份契約解析（D6、D14），改名要同時改兩支 App。
const (
	DataDeepLink          = "deep_link"
	DataStateRefreshHints = "state_refresh_hints"
	DataInboxItemID       = "inbox_item_id"

	// StateRefreshHintPrefix 加上 model.StateHint 的值就是 proto enum StateRefreshHint 的名稱（例如 STATE_REFRESH_HINT_ORDER）。
	StateRefreshHintPrefix = "STATE_REFRESH_HINT_"
)

func pushMessage(p plan, inboxItemID *uuid.UUID, imageURL string) fcm.Message {
	data := map[string]string{}
	if p.rendered.DeepLink != "" {
		data[DataDeepLink] = p.rendered.DeepLink
	}
	if h := hints(p.rendered.StateHint); h != "" {
		data[DataStateRefreshHints] = h
	}
	if inboxItemID != nil {
		data[DataInboxItemID] = inboxItemID.String()
	}
	return fcm.Message{Title: p.rendered.Title, Body: p.rendered.Body, ImageURL: imageURL, Data: data}
}

func hints(h model.StateHint) string {
	if h == model.StateHintNone {
		return ""
	}
	return StateRefreshHintPrefix + string(h)
}
