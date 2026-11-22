package turn

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

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
}

// 請求的狀態（與 schema 的 CHECK 相同）。
const (
	statusClarifying       = "CLARIFYING"
	statusRequirementReady = "REQUIREMENT_READY"
	statusMatching         = "MATCHING"
)

// searchAffectingFields 是改了就讓已有的候選搜尋結果失效的欄位（A9-D28(4)、ADR E9）。
// 3b-2d 實作候選搜尋時依搜尋實際用到的輸入修正這個集合。
var searchAffectingFields = []model.SlotKey{slots.ItemResolution, slots.Quantity, slots.Unit, slots.DeliveryCity, slots.DeliveryDate}

// 自動標題與預覽的長度（以 rune 計，A9-D28(3)）。
const (
	maxTitleRunes   = 40
	maxPreviewRunes = 200
	// maxReplyRunes 與 turns.assistant_reply 的 CHECK 一致。
	maxReplyRunes = 4000
	// titleSeparator 連接品項與縣市。
	titleSeparator = "｜"
)

// priorState 是這一輪開始前讀到的狀態。
type priorState struct {
	current   slots.Set
	prevTurn  *prevTurn
	preview   bool
	snapshot  *snapshotState
	requestOf string // requests.status
}

type prevTurn struct {
	optionalStreak int
	skipped        []model.SlotKey
}

// snapshotState 是需求最新的確認快照指向的版本是不是這一輪開始前的現行版本。
type snapshotState struct{ onCurrentRevision bool }

// decision 是這一輪算出來的全部結果（純資料，沒有 I/O）。
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
	// interrupt 非 nil 時這一輪停在品項消歧（等買家回答）。
	interrupt *InterruptPayload
	status    string
}

// decide 把抽取出的 patch 套到現況：Apply、需要時解析品項、挑問題、選 act、渲染文字。resolver 是這裡唯一的 I/O。
func decide(ctx context.Context, rs Resolver, l uitext.Locale, now time.Time, prior priorState, patch slots.Patch) (decision, error) {
	next, rejected := prior.current.Apply(patch, now)

	var resolved *slots.Resolution
	var pending *pendingCard
	if next.Item != nil && (!equalStr(prior.current.Item, next.Item) || next.Resolution == nil) {
		res, err := rs.Resolve(ctx, *next.Item)
		if err != nil {
			return decision{}, fmt.Errorf("turn：解析品項：%w", err)
		}
		switch len(res.Matches) {
		case 1:
			m := res.Matches[0]
			r := slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: m.CategoryID, Label: m.Name}
			resolved = &r
		default:
			pending = disambiguationPayload(res)
		}
	}
	if resolved != nil {
		patch.Resolution = resolved
		next, _ = next.Apply(slots.Patch{Resolution: resolved}, now)
	}

	missing := next.MissingRequired()
	var q *planner.Question
	capExhausted := false
	skipped := map[model.SlotKey]bool{}
	prevStreak := 0
	var prevSkipped []model.SlotKey
	if prior.prevTurn != nil {
		prevStreak = prior.prevTurn.optionalStreak
		prevSkipped = prior.prevTurn.skipped
		for _, k := range prevSkipped {
			skipped[k] = true
		}
	}
	if pending != nil {
		q = pending.card
	} else {
		q, capExhausted = planner.PickNext(next, missing, skipped, prevStreak)
	}

	plan := planner.Decide(planner.Input{
		Current: prior.current, Next: next, Rejected: rejected, Recorded: slots.RecordedThisTurn(patch, prior.current, next), Question: q,
		PricePreviewAvailable: prior.preview && sameContent(prior.current, next), Rfq: rfqState(prior, next),
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
	d.status = nextStatus(prior.requestOf, prior.current, next, len(missing) == 0 && pending == nil)
	return d, nil
}

type pendingCard struct {
	payload InterruptPayload
	card    *planner.Question
}

func disambiguationPayload(res resolver.Resolution) *pendingCard {
	cands := make([]planner.ItemCandidate, 0, len(res.Matches))
	for _, m := range res.Matches {
		cands = append(cands, planner.ItemCandidate{CategoryID: m.CategoryID, Label: m.Name, Path: m.Path})
	}
	card := planner.BuildItemDisambiguation(cands)
	shown := cands[:min(len(cands), planner.MaxItemOptions)]
	p := InterruptPayload{Stage: StageItem, Query: res.BuyerTerm, Candidates: make([]PayloadCandidate, 0, len(shown))}
	for _, c := range shown {
		p.Candidates = append(p.Candidates, PayloadCandidate{CategoryID: c.CategoryID, Label: c.Label, Path: c.Path})
	}
	return &pendingCard{payload: p, card: card}
}

// rfqState 把「需求最新的確認快照」換成 planner 的詢價狀態：快照指向這一輪開始前的現行版本、而且這一輪沒有改動內容才算有效，
// 其餘有快照的都是過期（內容變了，快照不再對得上）。
func rfqState(prior priorState, next slots.Set) planner.RfqState {
	switch {
	case prior.snapshot == nil:
		return planner.RfqNone
	case prior.snapshot.onCurrentRevision && sameContent(prior.current, next):
		return planner.RfqConfirmed
	}
	return planner.RfqStale
}

// nextStatus 決定需求的狀態（A9-D28(4)）：需求在 MATCHING 時，只有搜尋用到的欄位改了才退回；否則依齊全與否在 CLARIFYING 與 REQUIREMENT_READY 之間。
// 品項還在等買家消歧時不算齊全：沒有分類無法搜尋。
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
	case slots.Quantity:
		return !sameContent(slots.Set{Quantity: a.Quantity}, slots.Set{Quantity: b.Quantity})
	case slots.Unit:
		return !sameContent(slots.Set{Unit: a.Unit}, slots.Set{Unit: b.Unit})
	case slots.DeliveryCity:
		return !sameContent(slots.Set{DeliveryCity: a.DeliveryCity}, slots.Set{DeliveryCity: b.DeliveryCity})
	case slots.DeliveryDate:
		return !sameContent(slots.Set{DeliveryDate: a.DeliveryDate}, slots.Set{DeliveryDate: b.DeliveryDate})
	}
	return false
}

// autoTitle 是對話的自動標題：品項｜縣市（有縣市才接），以 rune 截 40；品項未知回空字串（不寫，client 顯示預設字樣）。
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

// bounded 以 rune 截斷並去頭尾空白（資料表的 CHECK 不收頭尾空白）。
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
