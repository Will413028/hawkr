package turn

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/reply"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Answer 是買家對消歧卡的回答（ClarificationResponse 的 option_id 與 free_text）；兩者恰有一個。
type Answer struct {
	OptionID string
	FreeText string
}

// OutcomeKind 是套用答案的結果種類。
type OutcomeKind int

const (
	// OutcomeInvalid 是答案無效，Reason 說明原因；呼叫端回可分類的錯誤，不改變中斷。
	OutcomeInvalid OutcomeKind = iota + 1
	// OutcomeResolved 是品項已解析，Resolution 有值。
	OutcomeResolved
	// OutcomeInterrupt 是還要再問，Next 是新的中斷 payload（內含已渲染的卡）。
	OutcomeInterrupt
)

// InvalidReason 是答案無效的原因代碼。
type InvalidReason string

const (
	InvalidNeedExactlyOne   InvalidReason = "NEED_EXACTLY_ONE_OF_OPTION_OR_TEXT"
	InvalidBlankText        InvalidReason = "BLANK_FREE_TEXT"
	InvalidOptionNotOnCard  InvalidReason = "OPTION_NOT_ON_CARD"
	InvalidUnknownStage     InvalidReason = "UNKNOWN_STAGE"
	InvalidResolutionReject InvalidReason = "RESOLUTION_REJECTED"
)

// Outcome 是 ApplyAnswer 的結果。
type Outcome struct {
	Kind       OutcomeKind
	Reason     InvalidReason
	Resolution *slots.Resolution
	Next       *InterruptPayload
	card *pendingCard
}

// CheckAnswer 是回答的形狀檢查（A9-D30(1)）：不讀分類樹，ResumeRun 在送出時同步用它，executor 的 ApplyAnswer 再用一次；回空字串表示形狀合法。
// 檢查：中斷的階段認得、option_id 與 free_text 恰一個、文字不是只有空白、選項在 payload 的卡上。
func CheckAnswer(p InterruptPayload, a Answer) InvalidReason {
	if p.Stage != StageGroup && p.Stage != StageItem && p.Stage != StageDescribe {
		return InvalidUnknownStage
	}
	text := trimtext.Trim(a.FreeText)
	switch {
	case a.OptionID != "" && text != "":
		return InvalidNeedExactlyOne
	case a.OptionID == "" && a.FreeText == "":
		return InvalidNeedExactlyOne
	case a.OptionID == "" && text == "":
		return InvalidBlankText
	case a.OptionID != "" && !onCard(p, a.OptionID):
		return InvalidOptionNotOnCard
	}
	return ""
}

// ApplyAnswer 把買家的回答套到中斷 payload：純函式，唯一的 I/O 是 resolver 讀分類樹（A9-D28(1)）。
// 不信任 client：選項只認 payload 卡上的，分類 ID 由 payload 的候選決定。
func ApplyAnswer(ctx context.Context, rs Resolver, l uitext.Locale, now time.Time, p InterruptPayload, a Answer) (Outcome, error) {
	if r := CheckAnswer(p, a); r != "" {
		return invalid(r), nil
	}
	if a.OptionID == "" {
		return applyText(ctx, rs, l, now, p, trimtext.Trim(a.FreeText))
	}
	if a.OptionID == planner.ItemOtherOptionID {
		return interrupt(l, describeCard(p.Query)), nil
	}
	c, ok := candidateOf(p, a.OptionID)
	if !ok {
		return invalid(InvalidOptionNotOnCard), nil
	}
	if p.Stage == StageGroup {
		return applyGroup(ctx, rs, l, now, p, c)
	}
	m, ok, err := rs.Leaf(ctx, c.CategoryID)
	if err != nil {
		return Outcome{}, fmt.Errorf("turn：查分類：%w", err)
	}
	if ok {
		if cur := usable(p.Query, []resolver.Match{m}, now); len(cur) == 1 {
			return resolvedOutcome(*cur[0].set.Resolution), nil
		}
	}
	res, err := rs.Resolve(ctx, p.Query)
	if err != nil {
		return Outcome{}, fmt.Errorf("turn：解析品項：%w", err)
	}
	current := usable(p.Query, res.Matches, now)
	if p.Root != nil {
		var inRoot []confirmed
		for _, h := range current {
			if h.match.RootID == *p.Root {
				inRoot = append(inRoot, h)
			}
		}
		current = inRoot
	}
	if len(current) == 0 {
		return interrupt(l, describeCard(p.Query)), nil
	}
	return decideMatches(l, p.Query, current), nil
}

func applyGroup(ctx context.Context, rs Resolver, l uitext.Locale, now time.Time, p InterruptPayload, group PayloadCandidate) (Outcome, error) {
	res, err := rs.Resolve(ctx, p.Query)
	if err != nil {
		return Outcome{}, fmt.Errorf("turn：解析品項：%w", err)
	}
	var inGroup []resolver.Match
	for _, m := range res.Matches {
		if m.RootID == group.CategoryID && m.Leaf {
			inGroup = append(inGroup, m)
		}
	}
	ms := usable(p.Query, inGroup, now)
	if len(ms) == 0 {
		leaves, err := rs.Leaves(ctx, group.CategoryID)
		if err != nil {
			return Outcome{}, fmt.Errorf("turn：讀大類的末層品項：%w", err)
		}
		ms = usable(p.Query, leaves, now)
		if len(ms) == 0 {
			return interrupt(l, describeCard(p.Query)), nil
		}
		return interrupt(l, buildCard(p.Query, matchesOf(ms))), nil
	}
	return decideMatches(l, p.Query, ms), nil
}

func applyText(ctx context.Context, rs Resolver, l uitext.Locale, now time.Time, p InterruptPayload, text string) (Outcome, error) {
	res, err := rs.Resolve(ctx, text)
	if err != nil {
		return Outcome{}, fmt.Errorf("turn：解析品項：%w", err)
	}
	term := res.BuyerTerm
	ms := usable(term, res.Matches, now)
	if len(ms) > 0 {
		return decideMatches(l, term, ms), nil
	}
	if p.Stage == StageDescribe {
		r, ok := accept(p.Query, slots.Resolution{Kind: slots.BuyerDescribed, Label: text}, now)
		if !ok {
			return invalid(InvalidResolutionReject), nil
		}
		return resolvedOutcome(r), nil
	}
	return interrupt(l, describeCard(p.Query)), nil
}

func decideMatches(l uitext.Locale, query string, ms []confirmed) Outcome {
	if len(ms) == 1 {
		return resolvedOutcome(*ms[0].set.Resolution)
	}
	return interrupt(l, buildCard(query, matchesOf(ms)))
}

func invalid(r InvalidReason) Outcome { return Outcome{Kind: OutcomeInvalid, Reason: r} }

func resolvedOutcome(r slots.Resolution) Outcome {
	return Outcome{Kind: OutcomeResolved, Resolution: &r}
}

func interrupt(l uitext.Locale, c *pendingCard) Outcome {
	p := c.payload
	p.Question = *questionWire(reply.Question(l, c.card))
	return Outcome{Kind: OutcomeInterrupt, Next: &p, card: &pendingCard{payload: p, card: c.card}}
}

func onCard(p InterruptPayload, id string) bool {
	for _, o := range p.Question.Options {
		if o.OptionID == id {
			return true
		}
	}
	return false
}

func candidateOf(p InterruptPayload, id string) (PayloadCandidate, bool) {
	for _, c := range p.Candidates {
		if planner.ItemOptionID(planner.ItemCandidate{CategoryID: c.CategoryID}) == id {
			return c, true
		}
	}
	return PayloadCandidate{}, false
}

func accept(query string, r slots.Resolution, now time.Time) (slots.Resolution, bool) {
	applied, _ := slots.Set{Item: &query}.Apply(slots.Patch{Resolution: &r}, now)
	if applied.Resolution == nil {
		return slots.Resolution{}, false
	}
	return *applied.Resolution, true
}

func usable(query string, ms []resolver.Match, now time.Time) []confirmed {
	return confirmable(slots.Set{Item: &query}, ms, now)
}

func matchesOf(cs []confirmed) []resolver.Match {
	out := make([]resolver.Match, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.match)
	}
	return out
}
