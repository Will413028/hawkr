package account

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// 決定表：provider 是 Stripe、列是平台收件、而且清單含 STRIPE_CONNECT_JS 才內嵌，其餘一律託管連結。
func TestChooseSessionIsEmbeddedOnlyForStripePlatformRowsWhenTheCallerCanEmbed(t *testing.T) {
	connect := []model.EmbeddedRuntime{model.RuntimeStripeConnectJS}
	for _, tc := range []struct {
		name       string
		provider   string
		collection model.RequirementCollection
		runtimes   []model.EmbeddedRuntime
		want       SessionKind
	}{
		{"stripe 平台收件、宣告內嵌", StripeName, model.CollectionPlatform, connect, SessionEmbedded},
		{"stripe 平台收件、清單另有不認得的值", StripeName, model.CollectionPlatform, []model.EmbeddedRuntime{"UNKNOWN", model.RuntimeStripeConnectJS}, SessionEmbedded},
		{"stripe 平台收件、空清單", StripeName, model.CollectionPlatform, nil, SessionHosted},
		{"stripe 平台收件、清單只有不認得的值", StripeName, model.CollectionPlatform, []model.EmbeddedRuntime{"UNKNOWN", ""}, SessionHosted},
		{"stripe 金流商收件、宣告內嵌", StripeName, model.CollectionProvider, connect, SessionHosted},
		{"stripe 金流商收件、空清單", StripeName, model.CollectionProvider, nil, SessionHosted},
		{"fake 平台收件、宣告內嵌", FakeName, model.CollectionPlatform, connect, SessionHosted},
		{"fake 金流商收件、宣告內嵌", FakeName, model.CollectionProvider, connect, SessionHosted},
		{"空的收件方式、宣告內嵌", StripeName, "", connect, SessionHosted},
	} {
		if got := ChooseSession(tc.provider, tc.collection, tc.runtimes); got != tc.want {
			t.Errorf("%s：got=%d want=%d", tc.name, got, tc.want)
		}
	}
}
