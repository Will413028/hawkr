package usecase

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
)

// PlatformLaneSearcher 查詢 product-service 的平台商品做為候選（doc 55 D2：兩條 lane 都做）。
// 只用 item（→ Keyword）做伺服器端篩選——budget_per_unit／quantity／delivery_city／
// delivery_date 四個維度都證不出來（前者是分母不同、後三者是 product-service 沒有
// 可篩選欄位），一律進 UnverifiableFacts（見 candidate-matching-a3 計畫 Design
// Decisions #3，以及 doc 55 §6.2 對單價預算的 2026-09-19 修訂）。
//
// #2377：每個候選另外帶「這個商品屬於哪個商家」的賣家身分快照（store_id ＋ 店名／logo／
// store_code），讓網頁把候選呈現成「找到哪幾家商家」，不必逐候選反查 product-service。
// 店家資料一律走一次 BatchGetSellerMeta（呼叫次數是 O(1)，不隨候選數或商家數成長）。
//
// 子系統 1（2026-08-29）：新增 rankingFacts 讀取本地 seller_ranking_facts（同樣 O(1)
// 呼叫，同一批 storeIDs 一次查完），做三件事——① 依 store_id 去重候選（修 T16：
// procurement_candidates 沒有 UNIQUE index，同店多商品會產生多筆重複候選）；② 用真正
// 隨查詢字詞變化的相關性分數取代原本恆定的 len(matched)*3（修 T15）；③ 曝光門檻過濾
// ＋等級加權排序。rankingFacts 查詢失敗 fail-open（同 attachSellerMeta 的降級哲學）：
// 排序退化成只看相關性，候選不會因為這張表打不到就整批消失。
type PlatformLaneSearcher struct {
	client       usecase.ProductClient
	seller       usecase.SellerClient
	rankingFacts repository.RankingFactsRepository
	now          func() time.Time
}

func NewPlatformLaneSearcher(client usecase.ProductClient, seller usecase.SellerClient, rankingFacts repository.RankingFactsRepository, now func() time.Time) *PlatformLaneSearcher {
	return &PlatformLaneSearcher{client: client, seller: seller, rankingFacts: rankingFacts, now: now}
}

// platformLaneTierWeight is how many relevance-score points one tier_rank
// level is worth. 已知延後決定的事項（spec §7）：實際數值待實測資料調整，這裡先
// 給一個讓等級明顯可見、但不會完全蓋過相關性的起始值——搜「香蕉」時，一個完全不
// 相關的高等級賣家（relevance=0）加權後仍不會贏過一個中等相關度的免費賣家。
const platformLaneTierWeight = 2

// relevanceScore 計算 item 名稱與查詢關鍵字的重疊 token 數（修 T15：舊版
// len(matched)*3 在同一次查詢下對所有候選恆定，因為 matched 只反映「用了哪些
// slot」，跟 item.Name 有多符合關鍵字完全無關）。
func relevanceScore(keyword, itemName string) int {
	score := 0
	name := strings.ToLower(itemName)
	for _, tok := range strings.Fields(strings.ToLower(keyword)) {
		if tok != "" && strings.Contains(name, tok) {
			score++
		}
	}
	return score
}

func (s *PlatformLaneSearcher) Search(ctx context.Context, slots domain.Slots) ([]repository.Candidate, error) {
	keyword := strings.TrimSpace(joinNonEmpty(slots.Item))
	if keyword == "" {
		return nil, nil
	}

	// ⛔ 單價預算不映射到 PriceMaxNTD：兩者分母不同。買家的預算分母是 slots.Unit
	// （元／KG、元／台斤…），ProductList 的 price_min／price_max 是 SKU 售價的彙總
	// （proto 的 ProductListItem 明文「不含 skus」），沒有可換算的重量基準。硬篩的
	// 症狀是等價問法分岔：100/KG 與 60/台斤指同一個價位，卻得到不同的候選集。
	//
	// doc02 §5.3.4 另外寫明 price_max_filter 是「商品價格區間與篩選區間**重疊**即
	// 命中」，所以它連「這張候選在預算內」都證不出來。
	//
	// 這是現有資料能力下的正式語意，不是暫時的 fail-open；要真正依預算篩選需要
	// producer 端提供 unit-aware search 或 SKU 基準投影（見 wiki 的 D02）。
	// PriceMaxNTD 本身的 SKU 售價語意保留給其他合法 caller。
	filter := usecase.ProductSearchFilter{Keyword: keyword}

	items, err := s.client.SearchProducts(ctx, filter)
	if err != nil {
		return nil, err
	}

	now := s.now()
	// 兩個 fact 列表都以「空的非 nil slice」起頭，不是 var 宣告的 nil slice：
	// json.Marshal(nil slice) 產出 `null`，json.Marshal([]T{}) 才產出 `[]`。這兩個值
	// 會原樣經由 handler 的 json.RawMessage 進到 API 回應，client 端 `facts.length`
	// 對 null 會直接爆。unverifiable 目前就走得到這條路（只填 item 時三個維度都沒填），
	// matched 則是靠上面 keyword 非空的前置條件擋著——但那是一個外部不變式，不該由
	// 序列化正確性去依賴它，兩個一律初始化（同 turn.go 對 rejected 的處理，
	// 也是 sqlc.yaml emit_empty_slices: true 的同一條約定）。
	matched := make([]domain.MatchFact, 0, 3)
	if slots.Item != nil && strings.TrimSpace(*slots.Item) != "" {
		matched = append(matched, domain.MatchFactFor(domain.SlotItem))
	}
	matchedJSON, _ := json.Marshal(matched)

	unverifiable := make([]domain.UnverifiableFact, 0, 4)
	// 預算跟數量／配送地一樣，是「現有資料證不出來、要向賣家確認」的維度。
	for _, key := range []domain.SlotKey{domain.SlotBudgetPerUnit, domain.SlotQuantity, domain.SlotDeliveryCity, domain.SlotDeliveryDate} {
		if slotHasValue(slots, key) {
			unverifiable = append(unverifiable, domain.UnverifiableFact{
				Key: key, Reason: domain.NeedConfirmReason(key, slots),
			})
		}
	}
	unverifiableJSON, _ := json.Marshal(unverifiable)

	// T16 去重：一店一筆代表候選，同店多商品只留相關性最高的那筆。
	type scored struct {
		candidate repository.Candidate
		relevance int
	}
	byStore := make(map[identity.StoreID]scored, len(items))
	storeOrder := make([]identity.StoreID, 0, len(items)) // first-seen order, stable tie-break input

	for _, item := range items {
		relevance := relevanceScore(keyword, item.Name)
		c := repository.Candidate{
			Lane: string(domain.LanePlatform), SourceID: strconv.FormatInt(item.ID, 10),
			DisplayName: item.Name, Score: relevance,
			MatchedFactsJSON: matchedJSON, UnverifiableFactsJSON: unverifiableJSON,
			FactsAsOf: now,
			// #2377 賣家身分快照，語意不變：facts_as_of 當下屬於誰，⛔ 不是授權判準。
			StoreID: item.StoreID,
		}
		existing, seen := byStore[item.StoreID]
		if !seen || relevance > existing.relevance {
			byStore[item.StoreID] = scored{candidate: c, relevance: relevance}
		}
		if !seen {
			storeOrder = append(storeOrder, item.StoreID)
		}
	}

	facts, err := s.rankingFacts.BatchGet(ctx, storeOrder)
	if err != nil {
		pkglogger.Warn(ctx, "procurement: seller_ranking_facts BatchGet failed; ranking falls back to relevance only",
			zap.Error(err), zap.Int("storeCount", len(storeOrder)))
		facts = nil
	}

	// 曝光門檻只認「本月」的計數。seller_ranking_facts 的歸零是寫端順帶做的
	// （下一筆 order.completed 進來時才 `CASE WHEN count_period = $2 … ELSE 1`），
	// 沒有排程器會在月初主動把每一列歸零，所以某店在 M 月撞到上限、M+1 月完全沒有
	// 成交時，那一列會原封不動停在 M 月的計數上。少了這個期間比對，它會在 M+1 月
	// 乃至往後每一個沒有成交的月份持續被排除——愈是賣不動就愈永遠不被看見。
	// 期間不符時視同計數為 0（只是還沒歸零，不是本月真的賣到上限）。
	// 用注入的 s.now() 而非 time.Now()，維持可測試性。
	currentPeriod := now.UTC().Format("2006-01")

	out := make([]repository.Candidate, 0, len(storeOrder))
	for _, storeID := range storeOrder {
		sc := byStore[storeID]
		fact, hasFact := facts[storeID]
		if hasFact && fact.MonthlyOrderCap != nil &&
			fact.CountPeriod == currentPeriod &&
			fact.MonthlyCompletedOrders >= *fact.MonthlyOrderCap {
			continue // 曝光門檻：本月已達上限的店家不出現（spec §6 步驟 3）
		}
		tierRank := 0
		if hasFact {
			tierRank = fact.TierRank
		}
		sc.candidate.Score = sc.relevance + tierRank*platformLaneTierWeight
		out = append(out, sc.candidate)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	for i := range out {
		out[i].Rank = i
	}

	s.attachSellerMeta(ctx, out)
	return out, nil
}

// attachSellerMeta 用一次 BatchGetSellerMeta 補齊所有候選的商家顯示欄位（#2377 驗收
// 條件 4：呼叫次數 O(1)，不是 O(候選數) 也不是 O(商家數)）。
//
// 失敗不讓整條 lane 失敗：storeId 才是前端分組的阻塞欄位，它來自 product-service 已經
// 拿到的 StoreID，這裡補的店名／logo／store_code 只是免去前端再打一輪的便利品。
// seller-service 掛掉時候選照樣可用，名片退化成「只有商家 ID」——這與
// recommend_usecase.go 對同一支 RPC 的既有處置一致。
func (s *PlatformLaneSearcher) attachSellerMeta(ctx context.Context, candidates []repository.Candidate) {
	if s.seller == nil || len(candidates) == 0 {
		return
	}
	seen := make(map[identity.StoreID]struct{}, len(candidates))
	storeIDs := make([]identity.StoreID, 0, len(candidates))
	for _, c := range candidates {
		if c.StoreID == 0 {
			continue
		}
		if _, dup := seen[c.StoreID]; dup {
			continue
		}
		seen[c.StoreID] = struct{}{}
		storeIDs = append(storeIDs, c.StoreID)
	}
	if len(storeIDs) == 0 {
		return
	}

	metas, err := s.seller.BatchGetSellerMeta(ctx, storeIDs)
	if err != nil {
		pkglogger.Warn(ctx, "procurement: BatchGetSellerMeta failed; candidates keep storeId only",
			zap.Error(err), zap.Int("storeCount", len(storeIDs)))
		return
	}
	for i := range candidates {
		meta := metas[candidates[i].StoreID]
		if meta == nil {
			continue
		}
		candidates[i].SellerName = meta.SellerName
		candidates[i].LogoKey = meta.LogoKey
		candidates[i].StoreCode = meta.StoreCode
	}
}

func joinNonEmpty(parts ...*string) string {
	var out []string
	for _, p := range parts {
		if p != nil && strings.TrimSpace(*p) != "" {
			out = append(out, strings.TrimSpace(*p))
		}
	}
	return strings.Join(out, " ")
}

func slotHasValue(s domain.Slots, key domain.SlotKey) bool {
	switch key {
	case domain.SlotBudgetPerUnit:
		return s.BudgetPerUnit != nil
	case domain.SlotQuantity:
		return s.Quantity != nil
	case domain.SlotDeliveryCity:
		return s.DeliveryCity != nil
	case domain.SlotDeliveryDate:
		return s.DeliveryDate != nil
	}
	return false
}
