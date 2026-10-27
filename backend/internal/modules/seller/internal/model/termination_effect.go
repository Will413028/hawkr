package model

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// TerminationStart 是在 now 對事由 g 建立終止合作時，措施的生效時間（period.start_time）：預告終止是 now 加 NoticePeriod，其他事由立即生效（now）。
// CreateMeasure 排定生效時間與 TerminationEffect 的 EffectiveAt 都只看這一個函式。
func TerminationStart(g TerminationGround, now time.Time) time.Time {
	if g == GroundNotice {
		return now.Add(NoticePeriod)
	}
	return now
}

// TerminationEffect 是對事由 Ground 建立終止合作之後，措施何時生效：EffectiveAt 為 nil 表示立即生效，否則是排程生效的時間。
type TerminationEffect struct {
	Ground      TerminationGround
	EffectiveAt *time.Time
}

// NewTerminationEffect 是在 now 建立事由 g 的終止合作的效果（生效時間來自 TerminationStart）。
func NewTerminationEffect(g TerminationGround, now time.Time) TerminationEffect {
	e := TerminationEffect{Ground: g}
	if start := TerminationStart(g, now); start.After(now) {
		e.EffectiveAt = &start
	}
	return e
}

// NewTerminationEffects 依 grounds 的順序回傳每個事由的效果（每個事由一筆）。
func NewTerminationEffects(grounds []TerminationGround, now time.Time) []TerminationEffect {
	out := make([]TerminationEffect, len(grounds))
	for i, g := range grounds {
		out[i] = NewTerminationEffect(g, now)
	}
	return out
}

// Message 是給員工的生效時機與這個事由的建立條件那一則完整的訊息（後台員工受眾，只有 zh-TW，不拼接）：API 的 TerminationOption.explanation 只有這一個產生點。
// 排程生效帶預告期天數與生效時間（台北時間，分鐘）；重大違約帶引用的認定須存在的天數（MaterialBreachCure）；關聯帳號說明連帶撤銷。
func (e TerminationEffect) Message() uitext.Message {
	if e.EffectiveAt == nil {
		if age := e.Ground.CitedFindingMinAge(); age > 0 {
			return text.TerminationEffectImmediateMaterialBreach(text.TerminationEffectImmediateMaterialBreachArgs{Days: int(age / (24 * time.Hour))})
		}
		if e.Ground == GroundLinkedAccount {
			return text.TerminationEffectImmediateLinkedAccount()
		}
		return text.TerminationEffectImmediate()
	}
	return text.TerminationEffectScheduled(text.TerminationEffectScheduledArgs{
		Days: int(NoticePeriod / (24 * time.Hour)),
		Time: uitext.FormatDateTime(uitext.ZhTW, *e.EffectiveAt),
	})
}
