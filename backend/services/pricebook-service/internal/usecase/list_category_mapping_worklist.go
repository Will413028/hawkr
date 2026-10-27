package usecase

import (
	"context"
	"sort"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

// ListCategoryMappingWorklist 是 admin 待處理清單 usecase（issue #2786 Task
// 9）：清單來源有兩類——既有 category_crop_mapping 列裡 status
// unmapped/ambiguous 的（明確待處理），以及分類樹裡完全查無列的 L3
// categoryId（賣家新建的分類，零審核 UGC，見設計書整體前提）。已有任何列
// （含 resolved）的 categoryId 一律不進差集路徑，避免重複列出。
type ListCategoryMappingWorklist struct {
	repo domain.CategoryMappingRepository
	src  categoryTreeSource
}

func NewListCategoryMappingWorklist(repo domain.CategoryMappingRepository, src categoryTreeSource) *ListCategoryMappingWorklist {
	return &ListCategoryMappingWorklist{repo: repo, src: src}
}

func (u *ListCategoryMappingWorklist) Do(ctx context.Context) ([]domain.WorklistItem, error) {
	existing, err := u.repo.ListByStatuses(ctx, []domain.CategoryMappingStatus{domain.CategoryMappingUnmapped, domain.CategoryMappingAmbiguous})
	if err != nil {
		return nil, err
	}
	mappedIDs, err := u.repo.ListAllCategoryIDs(ctx)
	if err != nil {
		return nil, err
	}
	mappedSet := make(map[string]bool, len(mappedIDs))
	for _, id := range mappedIDs {
		mappedSet[id] = true
	}

	// 兩棵樹，用途嚴格分開——合成一棵會壞掉其中一邊：
	//
	//   activeLeafNames：差集那批的**來源**。status="active" + includeDeleted=false，因為
	//     「停用的分類沒有補 mapping 的意義」（原始設計意圖）。只收葉節點，因為 resolve API
	//     只解析 L3。
	//
	//   allNames：已登記那批的**名稱查表**。含停用分類，且**刻意不套葉節點過濾**。
	//     ⚠️ 兩個過濾都不能加，理由不同：
	//       - 不濾停用：已登記的列會留在待處理清單裡，分類之後被停用不該讓它變成無名列（#2818 同 class）
	//       - 不濾葉節點：product-service 是「先濾 ACTIVE 再重建父子」（CategoryService.getCategoryTree），
	//         所以 active 樹與全樹的「葉」定義不一致——子節點全停用的 L2 在 active 樹是葉、在全樹不是。
	//         若這裡也只收葉，會出現「差集列得出這個 id、名稱卻查不到」的新破口。
	//         它只是 id→顯示名的查表，收全部節點無害（id 唯一）。
	activeTree, err := u.src.GetCategoryTree(ctx, "active", false)
	if err != nil {
		return nil, err
	}
	allTree, err := u.src.GetCategoryTree(ctx, "", true)
	if err != nil {
		// 不降級成「名稱空白」——那正是本次要修的症狀。內容錯的 worklist 比一個錯誤訊息更糟。
		return nil, err
	}
	activeLeafNames := make(map[string]string)
	collectCategoryNames(activeTree.GetCategories(), activeLeafNames)
	allNames := make(map[string]string)
	collectAllCategoryNames(allTree.GetCategories(), allNames)

	items := make([]domain.WorklistItem, 0, len(existing)+len(activeLeafNames))
	for _, m := range existing {
		updatedBy := m.UpdatedBy
		updatedAt := m.UpdatedAt
		items = append(items, domain.WorklistItem{
			CategoryID: m.CategoryID, CategoryName: allNames[m.CategoryID],
			Status: m.Status, UpdatedBy: &updatedBy, UpdatedAt: &updatedAt,
		})
	}
	for id, name := range activeLeafNames {
		if mappedSet[id] {
			continue
		}
		items = append(items, domain.WorklistItem{CategoryID: id, CategoryName: name, Status: domain.CategoryMappingUnmapped})
	}
	// 上面的差集迴圈跑的是 map，Go 的 map 迭代順序每次執行都不同，清單順序會
	// 隨機跳動。依 CategoryID 排序，讓 admin 清單每次刷新順序穩定、也讓測試
	// 能對整份結果做確定性斷言。
	sort.Slice(items, func(i, j int) bool { return items[i].CategoryID < items[j].CategoryID })
	return items, nil
}

// collectAllCategoryNames 收**每一個**節點的 id→name，不分層級也不分狀態。
// 它是查表不是候選集合，所以沒有 collectCategoryNames 那個葉節點條件（見 Do 的說明）。
func collectAllCategoryNames(nodes []*categorypb.CategoryNode, out map[string]string) {
	for _, n := range nodes {
		out[n.GetId()] = n.GetName()
		collectAllCategoryNames(n.GetChildren(), out)
	}
}

func collectCategoryNames(nodes []*categorypb.CategoryNode, out map[string]string) {
	for _, n := range nodes {
		if len(n.GetChildren()) == 0 { // 只收 L3 葉節點——resolve API 只解析 L3 categoryId，非葉節點不會被查詢
			out[n.GetId()] = n.GetName()
		}
		collectCategoryNames(n.GetChildren(), out)
	}
}
