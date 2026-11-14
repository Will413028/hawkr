package account

import (
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// SessionKind 是開通 session 的形狀。
type SessionKind int

// 開通 session 的形狀。
const (
	// SessionHosted 是金流商託管的連結（Stripe 的 Account Link）。
	SessionHosted SessionKind = iota + 1
	// SessionEmbedded 是內嵌元件的 session（Stripe 的 Account Session）。
	SessionEmbedded
)

// ChooseSession 是開通 session 形狀的唯一決定處：帳戶列保存的收件方式是平台收件、而且 provider 支援的內嵌執行環境
// （Capabilities.EmbeddedRuntimes）與呼叫端宣告的有交集才內嵌，其餘一律託管連結。只看帳戶列、provider 的能力與呼叫端的宣告，
// 不看行程設定，也不比 provider 名稱；平台收件的帳戶走託管連結是可行的（spike Q23）。
func ChooseSession(collection model.RequirementCollection, supported, declared []model.EmbeddedRuntime) SessionKind {
	if collection != model.CollectionPlatform {
		return SessionHosted
	}
	for _, r := range declared {
		if slices.Contains(supported, r) {
			return SessionEmbedded
		}
	}
	return SessionHosted
}
