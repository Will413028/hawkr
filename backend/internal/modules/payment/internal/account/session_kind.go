package account

import "github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"

// SessionKind 是開通 session 的形狀。
type SessionKind int

// 開通 session 的形狀。
const (
	// SessionHosted 是金流商託管的連結（Stripe 的 Account Link）。
	SessionHosted SessionKind = iota + 1
	// SessionEmbedded 是內嵌元件的 session（Stripe 的 Account Session）。
	SessionEmbedded
)

// ChooseSession 是開通 session 形狀的唯一決定處：帳戶列的 provider 是 Stripe、列保存的收件方式是平台收件、而且呼叫端宣告能承載
// Stripe Connect.js 才內嵌，其餘一律託管連結。只看帳戶列與呼叫端的宣告，不看行程設定；平台收件的帳戶走託管連結是可行的（spike Q23）。
func ChooseSession(provider string, collection model.RequirementCollection, runtimes []model.EmbeddedRuntime) SessionKind {
	if provider != StripeName || collection != model.CollectionPlatform {
		return SessionHosted
	}
	for _, r := range runtimes {
		if r == model.RuntimeStripeConnectJS {
			return SessionEmbedded
		}
	}
	return SessionHosted
}
