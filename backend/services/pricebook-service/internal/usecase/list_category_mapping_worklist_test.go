package usecase_test

import (
	"context"
	"testing"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

func TestListCategoryMappingWorklist_CategoryWithNoRow_ListedAsUnmapped(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		listByStatusesFn: func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
			return nil, nil
		},
		listAllCategoryIDsFn: func(ctx context.Context) ([]string, error) {
			return nil, nil
		},
	}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-new", Name: "新分類", Status: usecase.CategoryNodeStatusActive}},
	}}
	u := usecase.NewListCategoryMappingWorklist(repo, src)

	items, err := u.Do(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "cat-new", items[0].CategoryID)
	require.Equal(t, domain.CategoryMappingUnmapped, items[0].Status)
	require.Nil(t, items[0].UpdatedBy)
	// 兩棵樹用途不同，兩次呼叫都要打對（只斷言最後一次會漏掉差集那棵）。
	require.Contains(t, src.calls, treeCall{status: "active", includeDeleted: false},
		"差集那批只看現行有效分類——把停用分類灌進待處理清單是另一種壞法")
	require.Contains(t, src.calls, treeCall{status: "", includeDeleted: true},
		"已登記那批的名稱查表必須含停用分類，否則分類被停用後該列變成無名列（#2818 同 class）")
}

// 已登記的列，其分類之後被停用 —— 名稱仍要顯示得出來（#2818 同 class）。
// 陰性同時鎖住：那個停用分類**不得**因此被當成「可補 mapping」灌進待處理清單。
func TestListCategoryMappingWorklist_ExistingRowWithDeactivatedCategory_KeepsName(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		listByStatusesFn: func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
			return []domain.CategoryMapping{
				{CategoryID: "cat-gone", Status: domain.CategoryMappingUnmapped, UpdatedBy: 7},
			}, nil
		},
		listAllCategoryIDsFn: func(ctx context.Context) ([]string, error) {
			return []string{"cat-gone"}, nil
		},
	}
	src := &fakeCategoryTreeSource{
		treeByCall: map[treeCall]*categorypb.GetCategoryTreeResponse{
			// active 樹：停用的分類整個不在裡面（product-service 是先濾 ACTIVE 才建樹）
			{status: "active", includeDeleted: false}: {Categories: nil},
			// 全樹：停用的分類還在，名字查得到
			{status: "", includeDeleted: true}: {Categories: []*categorypb.CategoryNode{
				{Id: "cat-gone", Name: "已停用分類", Status: usecase.CategoryNodeStatusInactive},
			}},
		},
	}
	u := usecase.NewListCategoryMappingWorklist(repo, src)

	items, err := u.Do(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1, "已登記的列不該消失")
	require.Equal(t, "cat-gone", items[0].CategoryID)
	require.Equal(t, "已停用分類", items[0].CategoryName,
		"名稱查表若沿用 active 樹，這裡會是空字串——admin 只看得到一個沒有名字的 id")
}

// 陰性：停用且**未**登記的分類不得出現在待處理清單。
// 沒有這一條，「把 nameByIDAll 直接拿去跑差集」的寫法會編譯過、上面那條也會綠。
func TestListCategoryMappingWorklist_DeactivatedUnmappedCategory_NotOffered(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		listByStatusesFn: func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
			return nil, nil
		},
		listAllCategoryIDsFn: func(ctx context.Context) ([]string, error) { return nil, nil },
	}
	src := &fakeCategoryTreeSource{
		treeByCall: map[treeCall]*categorypb.GetCategoryTreeResponse{
			{status: "active", includeDeleted: false}: {Categories: []*categorypb.CategoryNode{
				{Id: "cat-live", Name: "現行分類", Status: usecase.CategoryNodeStatusActive},
			}},
			{status: "", includeDeleted: true}: {Categories: []*categorypb.CategoryNode{
				{Id: "cat-live", Name: "現行分類", Status: usecase.CategoryNodeStatusActive},
				{Id: "cat-dead", Name: "停用分類", Status: usecase.CategoryNodeStatusInactive},
			}},
		},
	}
	u := usecase.NewListCategoryMappingWorklist(repo, src)

	items, err := u.Do(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "cat-live", items[0].CategoryID,
		"停用分類沒有補 mapping 的意義，不得被差集灌進來")
}

// 全樹的名稱查表**不套葉節點過濾**：active 樹與全樹的「葉」定義本來就不同
// （product-service 先濾 ACTIVE 再重建父子），若兩邊都只收葉，會出現
// 「差集列得出這個 id、名稱卻查不到」的破口。
func TestListCategoryMappingWorklist_NonLeafRegisteredCategory_StillNamed(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		listByStatusesFn: func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
			return []domain.CategoryMapping{
				{CategoryID: "cat-parent", Status: domain.CategoryMappingAmbiguous, UpdatedBy: 1},
			}, nil
		},
		listAllCategoryIDsFn: func(ctx context.Context) ([]string, error) {
			return []string{"cat-parent"}, nil
		},
	}
	src := &fakeCategoryTreeSource{
		treeByCall: map[treeCall]*categorypb.GetCategoryTreeResponse{
			{status: "active", includeDeleted: false}: {Categories: nil},
			// 登記時是葉，之後長出子節點 ⇒ 在全樹裡不再是葉
			{status: "", includeDeleted: true}: {Categories: []*categorypb.CategoryNode{{
				Id: "cat-parent", Name: "父分類", Status: usecase.CategoryNodeStatusActive,
				Children: []*categorypb.CategoryNode{
					{Id: "cat-child", Name: "子分類", Status: usecase.CategoryNodeStatusActive},
				},
			}}},
		},
	}
	u := usecase.NewListCategoryMappingWorklist(repo, src)

	items, err := u.Do(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "父分類", items[0].CategoryName,
		"名稱查表若套葉節點過濾，非葉的已登記分類就查不到名字")
}

func TestListCategoryMappingWorklist_CategoryWithExistingRow_NotDuplicated(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		listByStatusesFn: func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
			return nil, nil // 假設現有列是 resolved，不會出現在 ListByStatuses(unmapped/ambiguous) 的結果
		},
		listAllCategoryIDsFn: func(ctx context.Context) ([]string, error) {
			return []string{"cat-resolved"}, nil
		},
	}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-resolved", Name: "已解析分類", Status: usecase.CategoryNodeStatusActive}},
	}}
	u := usecase.NewListCategoryMappingWorklist(repo, src)

	items, err := u.Do(context.Background())
	require.NoError(t, err)
	require.Empty(t, items, "已有 mapping 列（即使是 resolved）不該出現在待處理清單，也不該被差集邏輯當成查無列重複列出")
}

// TestListCategoryMappingWorklist_OrderIsDeterministic 鎖住輸出順序：差集迴圈
// 跑的是 Go map，未排序時同一份輸入每次執行的順序都可能不同，admin 清單會無故
// 跳動。跑多輪並要求每輪結果完全一致（依 CategoryID 遞增）。
func TestListCategoryMappingWorklist_OrderIsDeterministic(t *testing.T) {
	newRepo := func() *fakeCategoryMappingRepo {
		return &fakeCategoryMappingRepo{
			listByStatusesFn: func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
				return []domain.CategoryMapping{
					{CategoryID: "cat-30", Status: domain.CategoryMappingAmbiguous, UpdatedBy: 7},
				}, nil
			},
			listAllCategoryIDsFn: func(ctx context.Context) ([]string, error) {
				return []string{"cat-30"}, nil
			},
		}
	}
	tree := &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "cat-50", Name: "五十", Status: usecase.CategoryNodeStatusActive},
		{Id: "cat-10", Name: "一十", Status: usecase.CategoryNodeStatusActive},
		{Id: "cat-40", Name: "四十", Status: usecase.CategoryNodeStatusActive},
		{Id: "cat-20", Name: "二十", Status: usecase.CategoryNodeStatusActive},
		{Id: "cat-30", Name: "三十", Status: usecase.CategoryNodeStatusActive},
	}}
	want := []string{"cat-10", "cat-20", "cat-30", "cat-40", "cat-50"}

	for i := 0; i < 20; i++ {
		u := usecase.NewListCategoryMappingWorklist(newRepo(), &fakeCategoryTreeSource{tree: tree})
		items, err := u.Do(context.Background())
		require.NoError(t, err)
		got := make([]string, 0, len(items))
		for _, it := range items {
			got = append(got, it.CategoryID)
		}
		require.Equal(t, want, got, "第 %d 輪順序與預期不符——清單順序必須是穩定的 CategoryID 遞增", i)
	}
}
