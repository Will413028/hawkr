package turn

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/reply"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Resolver 把品項用詞對到分類樹（*resolver.Resolver 滿足它）。
type Resolver interface {
	Resolve(ctx context.Context, term string) (resolver.Resolution, error)
	// Leaves 回傳大類底下的末層節點（不含大類自己），依分類樹順序。
	Leaves(ctx context.Context, rootID uuid.UUID) ([]resolver.Match, error)
	// Leaf 以分類 ID 查現行分類樹：節點存在而且是末層才回 true（不看用詞）。
	Leaf(ctx context.Context, categoryID uuid.UUID) (resolver.Match, bool, error)
}

const (
	statusClarifying       = "CLARIFYING"
	statusRequirementReady = "REQUIREMENT_READY"
	statusMatching         = "MATCHING"
)

var searchAffectingFields = []model.SlotKey{slots.ItemResolution}

const (
	maxTitleRunes   = 40
	maxPreviewRunes = 200
	maxReplyRunes   = 4000
	titleSeparator  = "｜"
)

type priorState struct {
	current   slots.Set
	prevTurn  *prevTurn
	approval  *approvalState
	requestOf string
}

type prevTurn struct {
	optionalStreak int
	skipped        []model.SlotKey
}

type approvalState struct{ onCurrentRevision bool }

type decision struct {
	next      slots.Set
	patch     slots.Patch
	rejected  []slots.Rejection
	recorded  []recordedJSON
	plan      planner.Plan
	blocks    []reply.Block
	replyText string
	question  *reply.RenderedQuestion
	missing   []model.SlotKey
	interrupt *InterruptPayload
	status    string
}

func decide(ctx context.Context, rs Resolver, l uitext.Locale, now time.Time, prior priorState, patch slots.Patch) (decision, error) {
	next, rejected := prior.current.Apply(patch, now)

	var pending *pendingCard
	if next.Item != nil && (!equalStr(prior.current.Item, next.Item) || next.Resolution == nil) {
		res, err := rs.Resolve(ctx, *next.Item)
		if err != nil {
			return decision{}, fmt.Errorf("turn：解析品項：%w", err)
		}
		cands := confirmable(next, res.Matches, now)
		if len(cands) == 1 {
			next = cands[0].set
			patch.Resolution = next.Resolution
		} else {
			ms := make([]resolver.Match, 0, len(cands))
			for _, c := range cands {
				ms = append(ms, c.match)
			}
			pending = buildCard(res.BuyerTerm, ms)
		}
	}
	return finish(l, prior, patch, next, rejected, pending), nil
}

func finish(l uitext.Locale, prior priorState, patch slots.Patch, next slots.Set, rejected []slots.Rejection, pending *pendingCard) decision {
	missing := next.MissingRequired()
	var q *planner.Question
	capExhausted := false
	skipped := map[model.SlotKey]bool{}
	if prior.prevTurn != nil {
		for _, k := range prior.prevTurn.skipped {
			skipped[k] = true
		}
	}
	if pending != nil {
		q = pending.card
	} else {
		prev := 0
		if prior.prevTurn != nil {
			prev = prior.prevTurn.optionalStreak
		}
		q, capExhausted = planner.PickNext(next, missing, skipped, prev)
	}
	return render(l, prior, patch, next, rejected, pending, q, capExhausted)
}

func render(l uitext.Locale, prior priorState, patch slots.Patch, next slots.Set, rejected []slots.Rejection, pending *pendingCard, q *planner.Question, capExhausted bool) decision {
	missing := next.MissingRequired()
	prevStreak := 0
	var prevSkipped []model.SlotKey
	if prior.prevTurn != nil {
		prevStreak = prior.prevTurn.optionalStreak
		prevSkipped = prior.prevTurn.skipped
	}

	plan := planner.Decide(planner.Input{
		Current: prior.current, Next: next, Rejected: rejected, Recorded: slots.RecordedThisTurn(patch, prior.current, next), Question: q,
		Rfq:        rfqState(prior, next),
		PrevStreak: prevStreak, Skipped: prevSkipped, CapExhausted: capExhausted,
	})
	blocks := reply.Render(l, plan, reply.Args{Missing: missing, Rejected: rejected})
	text := bounded(reply.TextFallback(blocks), maxReplyRunes)

	d := decision{
		next: next, patch: patch, rejected: rejected, plan: plan, blocks: blocks, replyText: text,
		question: reply.Question(l, plan.Question), missing: missing,
	}
	for _, r := range slots.RecordedThisTurn(patch, prior.current, next) {
		d.recorded = append(d.recorded, recordedJSON{Key: slotKey(r.Key), Label: reply.SlotLabel(l, r.Key), Value: reply.RecordedValue(l, r)})
	}
	if pending != nil {
		p := pending.payload
		p.Question = *questionWire(d.question)
		d.interrupt = &p
	}
	d.status = nextStatus(prior.requestOf, prior.current, next, next.Ready())
	return d
}

type confirmed struct {
	match resolver.Match
	set   slots.Set
}

func confirmable(next slots.Set, ms []resolver.Match, now time.Time) []confirmed {
	var out []confirmed
	for _, m := range leavesOnly(ms) {
		r := slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: m.CategoryID, Label: m.Name}
		if applied, _ := next.Apply(slots.Patch{Resolution: &r}, now); applied.Resolution != nil {
			out = append(out, confirmed{match: m, set: applied})
		}
	}
	return out
}

func rfqState(prior priorState, next slots.Set) planner.RfqState {
	switch {
	case prior.approval == nil:
		return planner.RfqNone
	case prior.approval.onCurrentRevision && sameContent(prior.current, next):
		return planner.RfqConfirmed
	}
	return planner.RfqStale
}

func nextStatus(cur string, prev, next slots.Set, ready bool) string {
	if cur == statusMatching && !searchFieldChanged(prev, next) {
		return statusMatching
	}
	if ready {
		return statusRequirementReady
	}
	return statusClarifying
}

func searchFieldChanged(prev, next slots.Set) bool {
	for _, k := range searchAffectingFields {
		if fieldDiffers(k, prev, next) {
			return true
		}
	}
	return false
}

func fieldDiffers(k model.SlotKey, a, b slots.Set) bool {
	switch k {
	case slots.ItemResolution:
		return !sameContent(slots.Set{Resolution: a.Resolution}, slots.Set{Resolution: b.Resolution})
	}
	return false
}

func autoTitle(s slots.Set) string {
	if s.Item == nil {
		return ""
	}
	t := *s.Item
	if s.DeliveryCity != nil {
		t += titleSeparator + *s.DeliveryCity
	}
	return bounded(t, maxTitleRunes)
}

func bounded(s string, n int) string {
	s = trimtext.Trim(s)
	if utf8.RuneCountInString(s) > n {
		s = trimtext.Trim(string([]rune(s)[:n]))
	}
	return s
}

func equalStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
