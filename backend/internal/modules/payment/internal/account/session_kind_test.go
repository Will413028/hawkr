package account

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// 決定表：列是平台收件、而且 provider 支援的內嵌執行環境與呼叫端宣告的有交集才內嵌，其餘一律託管連結。
func TestChooseSessionIsEmbeddedOnlyForStripePlatformRowsWhenTheCallerCanEmbed(t *testing.T) {
	connect := []model.EmbeddedRuntime{model.RuntimeStripeConnectJS}
	for _, tc := range []struct {
		name       string
		collection model.RequirementCollection
		supported  []model.EmbeddedRuntime
		declared   []model.EmbeddedRuntime
		want       SessionKind
	}{
		{"平台收件、支援、宣告內嵌", model.CollectionPlatform, connect, connect, SessionEmbedded},
		{"平台收件、支援、清單另有不認得的值", model.CollectionPlatform, connect, []model.EmbeddedRuntime{"UNKNOWN", model.RuntimeStripeConnectJS}, SessionEmbedded},
		{"平台收件、支援、呼叫端沒宣告", model.CollectionPlatform, connect, nil, SessionHosted},
		{"平台收件、支援、清單只有不認得的值", model.CollectionPlatform, connect, []model.EmbeddedRuntime{"UNKNOWN", ""}, SessionHosted},
		{"金流商收件、支援、宣告內嵌", model.CollectionProvider, connect, connect, SessionHosted},
		{"金流商收件、支援、沒宣告", model.CollectionProvider, connect, nil, SessionHosted},
		{"平台收件、provider 不支援（fake）、宣告內嵌", model.CollectionPlatform, nil, connect, SessionHosted},
		{"空的收件方式、支援、宣告內嵌", "", connect, connect, SessionHosted},
	} {
		if got := ChooseSession(tc.collection, tc.supported, tc.declared); got != tc.want {
			t.Errorf("%s：got=%d want=%d", tc.name, got, tc.want)
		}
	}
}
