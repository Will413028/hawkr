package usecase_test

import (
	"context"
	"errors"
	"testing"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// categoryNodeStatusInactive 是 CommonStatus 的「未啟用」Value（9003），
// 對應 usecase.CategoryNodeStatusActive 的反面。
const categoryNodeStatusInactive = "9003"

// fakeCategoryTreeSource 的 node status 一律用 CommonStatus 的 **Value**
// （"9004"／"9003"），不是 Code（"ACTIVE"／"INACTIVE"）——product-service 的
// CategoryGrpcService.convertToNode 直接把 DB 的 category.status 欄位（見
// V1__init_product_schema.sql 的欄位註解：9003=軟刪除, 9004=啟用）塞進
// CategoryNode.status。fixture 用 Code 等於捏造一個真實 producer 從不送出的
// 值，會讓 usecase 的比對邏輯錯得無聲無息（這正是最終 review 抓到的 Critical
// 缺陷成因）。這裡引用 usecase.CategoryNodeStatusActive 而非再寫一次字面值，
// 讓 fixture 與 production 比對值綁在同一個來源。
type treeCall struct {
	status         string
	includeDeleted bool
}

type fakeCategoryTreeSource struct {
	tree *categorypb.GetCategoryTreeResponse
	err  error

	// treeByCall 讓 fake 依「參數」回不同的樹。沒有它，worklist 那條同時打
	// (active,false) 與 ("",true) 的路徑就只能對同一棵樹斷言 —— 那測到的是 fake
	// 的行為而不是 code 的行為，「兩棵樹用途分開」這個保證等於沒有守。
	// 未登記的組合退回 tree（既有測試不受影響）。
	treeByCall map[treeCall]*categorypb.GetCategoryTreeResponse

	// calls 記錄每一次呼叫（依序）。gotStatus/gotIncludeDeleted 是最後一次，
	// 只在單次呼叫的測試裡有意義。
	calls             []treeCall
	gotStatus         string
	gotIncludeDeleted bool
	called            bool
}

func (f *fakeCategoryTreeSource) GetCategoryTree(ctx context.Context, status string, includeDeleted bool) (*categorypb.GetCategoryTreeResponse, error) {
	call := treeCall{status: status, includeDeleted: includeDeleted}
	f.calls = append(f.calls, call)
	f.gotStatus, f.gotIncludeDeleted, f.called = status, includeDeleted, true
	if f.err != nil {
		return nil, f.err
	}
	if t, ok := f.treeByCall[call]; ok {
		return t, nil
	}
	return f.tree, nil
}

func TestUpsertCategoryMapping_UnknownCategoryID_Rejected(t *testing.T) {
	repo := &fakeCategoryMappingRepo{}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{Categories: nil}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	_, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-unknown", Status: domain.CategoryMappingUnmapped, UpdatedBy: 1,
	})
	require.True(t, errors.Is(err, usecase.ErrCategoryNotFound))
}

func TestUpsertCategoryMapping_InactiveCategoryID_Rejected(t *testing.T) {
	repo := &fakeCategoryMappingRepo{}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-deleted", Status: categoryNodeStatusInactive}},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	_, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-deleted", Status: domain.CategoryMappingUnmapped, UpdatedBy: 1,
	})
	require.True(t, errors.Is(err, usecase.ErrCategoryInactive))
}

// TestUpsertCategoryMapping_RequestsTreeIncludingDeleted 鎖住 ErrCategoryInactive
// 可達的前提：product-service 把空 status 當成 active-only，只有
// include_deleted=true 才會回停用節點。若這裡退回
// GetCategoryTree(ctx, "", false)，停用分類會被當成「查無此分類」，
// ErrCategoryInactive 變成永遠不會發生的死碼。
func TestUpsertCategoryMapping_RequestsTreeIncludingDeleted(t *testing.T) {
	repo := &fakeCategoryMappingRepo{}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-1", Status: usecase.CategoryNodeStatusActive}},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	_, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-1", Status: domain.CategoryMappingUnmapped, UpdatedBy: 1,
	})
	require.NoError(t, err)
	require.True(t, src.called)
	require.True(t, src.gotIncludeDeleted, "admin 寫入路徑必須拿到含 INACTIVE 節點的樹，才能分辨『查無』與『已停用』")
}

// TestUpsertCategoryMapping_ActiveCategoryUsesCodeMetaValue 直接鎖住 Critical
// #1：真實 producer 送的是 CommonStatus 的 Value（"9004"），不是 Code
// （"ACTIVE"）。若 usecase 改回比對 "ACTIVE"，這個測試會失敗。
func TestUpsertCategoryMapping_ActiveCategoryUsesCodeMetaValue(t *testing.T) {
	require.Equal(t, "9004", usecase.CategoryNodeStatusActive)

	repo := &fakeCategoryMappingRepo{}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-1", Status: "9004"}},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	got, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-1", Status: domain.CategoryMappingUnmapped, UpdatedBy: 1,
	})
	require.NoError(t, err, "status=\"9004\" 是 product-service 真的會送的 ACTIVE 值，不得被判成停用")
	require.Equal(t, "cat-1", got.CategoryID)
}

// TestUpsertCategoryMapping_CategoryStatusCodeStringRejected 是上面的反向鎖：
// "ACTIVE" 這個 Code 字串不是 wire 上會出現的值，不該被當成 active 放行。
func TestUpsertCategoryMapping_CategoryStatusCodeStringRejected(t *testing.T) {
	repo := &fakeCategoryMappingRepo{}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-1", Status: "ACTIVE"}},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	_, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-1", Status: domain.CategoryMappingUnmapped, UpdatedBy: 1,
	})
	require.True(t, errors.Is(err, usecase.ErrCategoryInactive))
}

// TestUpsertCategoryMapping_ResolvedActiveGroupTarget_Succeeds 驗證 resolved
// 且目標 group 為 active 時寫入成功（原本只有 crop 版本的覆蓋，2026-08-30
// 起 L3 一律對 group，這裡取代原本的 crop-only 成功路徑測試）。
func TestUpsertCategoryMapping_ResolvedActiveGroupTarget_Succeeds(t *testing.T) {
	activeGroupName := "測試大類"
	repo := &fakeCategoryMappingRepo{
		activeGroupNameFn: func(ctx context.Context, code string) (*string, error) { return &activeGroupName, nil },
	}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-1", Status: usecase.CategoryNodeStatusActive}},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	groupCode := "group-active"
	got, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-1", Status: domain.CategoryMappingResolved,
		GroupCode: &groupCode, UpdatedBy: 42,
	})
	require.NoError(t, err)
	require.Equal(t, "cat-1", got.CategoryID)
	require.Equal(t, int64(42), got.UpdatedBy)
}

// TestUpsertCategoryMapping_NestedCategoryID_FoundInSubtree 驗證
// findCategoryNode 的遞迴搜尋在 categoryId 位於分類樹深層節點時仍能正確找到，
// 不是只檢查頂層節點（brief 自我檢查項目之一）。
func TestUpsertCategoryMapping_NestedCategoryID_FoundInSubtree(t *testing.T) {
	repo := &fakeCategoryMappingRepo{}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{
			{
				Id:     "cat-root",
				Status: usecase.CategoryNodeStatusActive,
				Children: []*categorypb.CategoryNode{
					{
						Id:     "cat-mid",
						Status: usecase.CategoryNodeStatusActive,
						Children: []*categorypb.CategoryNode{
							{Id: "cat-leaf", Status: usecase.CategoryNodeStatusActive},
						},
					},
				},
			},
		},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	got, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-leaf", Status: domain.CategoryMappingUnmapped, UpdatedBy: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "cat-leaf", got.CategoryID)
}

// TestUpsertCategoryMapping_ResolvedTargetGroupInactive_Rejected 驗證
// resolved 且目標 group 已停用時回 ErrTargetCodeInactive，不得靜默放行。
func TestUpsertCategoryMapping_ResolvedTargetGroupInactive_Rejected(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		activeGroupNameFn: func(ctx context.Context, code string) (*string, error) { return nil, nil },
	}
	src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
		Categories: []*categorypb.CategoryNode{{Id: "cat-1", Status: usecase.CategoryNodeStatusActive}},
	}}
	u := usecase.NewUpsertCategoryMapping(repo, src)

	groupCode := "group-inactive"
	_, err := u.Do(context.Background(), usecase.UpsertCategoryMappingInput{
		CategoryID: "cat-1", Status: domain.CategoryMappingResolved,
		GroupCode: &groupCode, UpdatedBy: 1,
	})
	require.True(t, errors.Is(err, usecase.ErrTargetCodeInactive))
}

// TestUpsertCategoryMapping_MalformedSelection_Rejected 涵蓋
// validateSelectionShape 可達的壞輸入：非 resolved 卻帶 groupCode、resolved
// 卻沒帶 groupCode。這兩種輸入在 application 層驗證出現之前都會一路撞到 DB
// 的 CHECK constraint，經 gRPC Internal 變成 HTTP 503。全部必須回
// ErrIncompleteSelection，而且不得是 ErrTargetCodeInactive（那是「目標代碼
// 停用」的語意，會給操作者錯誤的線索）。
func TestUpsertCategoryMapping_MalformedSelection_Rejected(t *testing.T) {
	groupCode := "B2"

	cases := []struct {
		name string
		in   usecase.UpsertCategoryMappingInput
	}{
		{
			name: "非 resolved 卻帶 groupCode",
			in: usecase.UpsertCategoryMappingInput{
				CategoryID: "cat-1", Status: domain.CategoryMappingUnmapped,
				GroupCode: &groupCode, UpdatedBy: 1,
			},
		},
		{
			name: "ambiguous 卻帶 groupCode",
			in: usecase.UpsertCategoryMappingInput{
				CategoryID: "cat-1", Status: domain.CategoryMappingAmbiguous,
				GroupCode: &groupCode, UpdatedBy: 1,
			},
		},
		{
			name: "resolved 但完全沒填 groupCode",
			in: usecase.UpsertCategoryMappingInput{
				CategoryID: "cat-1", Status: domain.CategoryMappingResolved,
				UpdatedBy: 1,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			activeName := "測試大類"
			repo := &fakeCategoryMappingRepo{
				activeGroupNameFn: func(ctx context.Context, code string) (*string, error) { return &activeName, nil },
				upsertFn: func(ctx context.Context, m domain.CategoryMapping) (*domain.CategoryMapping, error) {
					t.Fatal("形狀不合法的輸入不得走到 repo.Upsert（那是讓 DB CHECK constraint 變成 HTTP 503 的路徑）")
					return nil, nil
				},
			}
			src := &fakeCategoryTreeSource{tree: &categorypb.GetCategoryTreeResponse{
				Categories: []*categorypb.CategoryNode{{Id: "cat-1", Status: usecase.CategoryNodeStatusActive}},
			}}
			u := usecase.NewUpsertCategoryMapping(repo, src)

			_, err := u.Do(context.Background(), tc.in)
			require.True(t, errors.Is(err, usecase.ErrIncompleteSelection), "err = %v, want ErrIncompleteSelection", err)
			require.False(t, errors.Is(err, usecase.ErrTargetCodeInactive), "輸入形狀錯誤不得偽裝成『目標代碼停用』")
		})
	}
}
