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
	// Leaves 回傳大類底下的末層節點（不含大類自己），依分類樹順序。
	Leaves(ctx context.Context, rootID int64) ([]resolver.Match, error)
	// Leaf 以分類 ID 查現行分類樹：節點存在而且是末層才回 true（不看用詞）。
	Leaf(ctx context.Context, categoryID int64) (resolver.Match, bool, error)
}

// 請求的狀態（與 schema 的 CHECK 相同）。
const (
	statusClarifying       = "CLARIFYING"
	statusRequirementReady = "REQUIREMENT_READY"
	statusMatching         = "MATCHING"
)

// searchAffectingFields 是 MATCHING 時改了就退回的欄位：候選搜尋只用品項解析當輸入（A9-D29(2)）。
var searchAffectingFields = []model.SlotKey{slots.ItemResolution}

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

// finish 是 decide 套用完 patch、決定好要不要出卡之後的共同尾段：挑問題、選 act、渲染文字、決定需求狀態。
// pending 非 nil 時這一輪停在品項消歧（回答中斷的 run 也走這裡，不重新解析）。
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

// render 是 finish 挑好問題之後的尾段：選 act、渲染文字、決定需求狀態。倒回直接給它指定的問題。
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
	d.status = nextStatus(prior.requestOf, prior.current, next, next.Ready())
	return d
}

// confirmed 是一個能被 Apply 收下當品項解析的命中，與收下後的欄位組。
type confirmed struct {
	match resolver.Match
	set   slots.Set
}

// confirmable 只留下 Apply 真的會收下為解析的命中（標籤空白、分類 ID 不合法的不會）：單一命中、多個命中與零命中的處理都以這個結果為準，
// 無效的命中不會出現在消歧卡上，也不會讓解析在 Apply 裡被默默丟掉。被截斷但保留的標籤是調整，不是被拒。
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
