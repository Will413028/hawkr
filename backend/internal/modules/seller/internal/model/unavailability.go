package model

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// AppealUnavailability 是賣家此刻不能申覆的一個原因與它給使用者看的一句話；API 的 AppealUnavailability.message 與 SubmitAppeal 被拒絕時錯誤的
// LocalizedMessage 都是 Message，只有 AppealUnavailabilities 這一個產生點。
type AppealUnavailability struct {
	Reason  AppealUnavailableReason
	Message uitext.Message
}

// RemediationUnavailability 是賣家此刻不能請求解除的一個原因與它給使用者看的一句話；產生點只有 RemediationUnavailabilities。
type RemediationUnavailability struct {
	Reason  RemediationUnavailableReason
	Message uitext.Message
}

// AppealUnavailabilities 是賣家此刻不能申覆的原因與句子（語系 l），順序同 AppealUnavailableReasons；空表示可以申覆。
func (m Measure) AppealUnavailabilities(l uitext.Locale) []AppealUnavailability {
	var out []AppealUnavailability
	for _, r := range m.AppealUnavailableReasons() {
		out = append(out, AppealUnavailability{Reason: r, Message: appealUnavailableText(l, r)})
	}
	return out
}

// appealUnavailableText 是原因給使用者看的一句話；不認得的原因回零值（Text 是空字串）。
func appealUnavailableText(l uitext.Locale, r AppealUnavailableReason) uitext.Message {
	switch r {
	case AppealReasonRevoked:
		return text.AppealUnavailableRevoked(l)
	case AppealReasonAlreadyAppealed:
		return text.AppealUnavailableAlreadyAppealed(l)
	case AppealReasonWindowEnded:
		return text.AppealUnavailableWindowEnded(l)
	case AppealReasonRequestPending:
		return text.AppealUnavailableRequestPending(l)
	}
	return uitext.Message{}
}

// RemediationUnavailabilities 是賣家此刻不能請求解除的原因與句子（語系 l），順序同 RemediationUnavailableReasons；空表示可以請求。
// NOT_ACTIVE 依措施是否已撤銷選句；COOLDOWN 帶冷卻結束時間（台北時間，秒數進位到分鐘）。
func (m Measure) RemediationUnavailabilities(l uitext.Locale) []RemediationUnavailability {
	var out []RemediationUnavailability
	for _, r := range m.RemediationUnavailableReasons() {
		out = append(out, RemediationUnavailability{Reason: r, Message: m.remediationUnavailableText(l, r)})
	}
	return out
}

func (m Measure) remediationUnavailableText(l uitext.Locale, r RemediationUnavailableReason) uitext.Message {
	switch r {
	case ReasonRemediationPending:
		return text.RemediationUnavailableRemediationPending(l)
	case ReasonAppealPending:
		return text.RemediationUnavailableAppealPending(l)
	case ReasonCooldown:
		next := m.NextRemediationTime()
		if next == nil {
			return uitext.Message{}
		}
		return text.RemediationUnavailableCooldown(l, text.RemediationUnavailableCooldownArgs{Time: cooldownTime(l, *next)})
	case ReasonNotRisk:
		return text.RemediationUnavailableNotRisk(l)
	case ReasonNotActive:
		if m.Status != MeasureStatusEnforced {
			return text.RemediationUnavailableNotActiveRevoked(l)
		}
		return text.RemediationUnavailableNotActive(l)
	}
	return uitext.Message{}
}

// cooldownTime 是冷卻結束時間的顯示：台北時間、秒數進位到分鐘（顯示的時間不會早於真正可以再提出的時間）。
func cooldownTime(l uitext.Locale, t time.Time) string {
	return uitext.FormatDateTime(l, twtime.MinuteCeil(t))
}

// OpeningUnavailability 是帳號此刻不能申請開店的一個原因與它給使用者看的一句話；GetMyStoreOpening 的 message 與 ApplyStore 被拒絕時錯誤的
// LocalizedMessage 都是 Message（OpeningBlocker.Text）。
type OpeningUnavailability struct {
	Blocker OpeningBlocker
	Message uitext.Message
}
