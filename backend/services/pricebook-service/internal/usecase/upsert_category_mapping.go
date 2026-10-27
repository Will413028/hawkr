package usecase

import (
	"context"
	"errors"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

// CategoryNodeStatusActive 是 product-service 在 CategoryNode.status 送出的
// 「啟用」值。它是 CodeMeta CommonStatus 的 **Value**（9004＝啟用、9003＝未
// 啟用，SSOT 見 docs/代碼管理（CodeMeta）/業務狀態碼.md 的 CommonStatus，落地
// 見 V1__init_product_schema.sql 的 category.status 欄位註解），不是 Code
// （"ACTIVE"／"INACTIVE"）。CategoryGrpcService.convertToNode 直接把 DB 欄位值
// 原樣塞進 proto，中間沒有任何轉換。
//
// 為何不引用 pkg/codemeta：CommonStatus 目前不在 generate.sh 的
// GO_REGISTRY_ENUMS 涵蓋清單內，Go 端沒有對應常數。把它加進清單需要一併
// commit docs/代碼管理（CodeMeta）/**，而該路徑的 pre-commit hook
// （check-copies.sh）目前因為 invoice 那批 enum 尚未同步到三個前端 repo 而
// 紅燈——與本次修正完全無關，也不該由本 PR 順手改動另外三個 repo。等該漂移
// 收斂後，這個常數應改為引用 codemeta.CommonStatusActive.Value（追蹤項見 PR
// 說明）。同類的本地常數在 repo 內已有先例：
// services/buyer-service/internal/usecase/favorite.go 的 activeProductStatus。
const CategoryNodeStatusActive = "9004"

// CategoryNodeStatusInactive 是同一組 CommonStatus 的「未啟用」值（9003）。
// 目前只有測試 fixture 用得到——production 路徑判斷的是「是不是 active」，
// 不需要列舉反面。放在這裡而不是測試檔，是為了讓兩個值一起被上面那段
// 「等 codemeta 漂移收斂後改引用生成常數」的說明涵蓋。
const CategoryNodeStatusInactive = "9003"

var (
	ErrCategoryNotFound   = errors.New("category not found")
	ErrCategoryInactive   = errors.New("category is not ACTIVE")
	ErrTargetCodeInactive = errors.New("target crop/group code is not active")
	// ErrIncompleteSelection 是「輸入形狀本身不合法」——status 與
	// selection_kind／crop_code／group_code 的 oneof 組合不成立。與
	// ErrTargetCodeInactive 刻意分開：後者專指「目標代碼填了、但目前不是
	// active」，兩者混用會讓 admin 操作者收到與實際問題無關的錯誤訊息。
	ErrIncompleteSelection = errors.New("selection fields incomplete or inconsistent with status")
)

// categoryTreeSource 是驗證 categoryId 存在且 ACTIVE 用的窄 interface，只要求
// GetCategoryTree 這一個方法——*category.Client（Task 6）自然滿足，測試可塞
// fake，不需要額外 adapter。簽章與 gateway/internal/grpc/category_client.go
// 對齊（status + includeDeleted 兩個參數），因為 product-service 把「空
// status」當成 active-only 而不是「不篩選」，要看到 INACTIVE 節點只能靠
// include_deleted=true。
type categoryTreeSource interface {
	GetCategoryTree(ctx context.Context, status string, includeDeleted bool) (*categorypb.GetCategoryTreeResponse, error)
}

// UpsertCategoryMappingInput 是 admin 寫入 category_crop_mapping 一列的輸入。
// 2026-08-30 起 resolved 一律對 group，不再接受 selection_kind/crop_code。
type UpsertCategoryMappingInput struct {
	CategoryID string
	Status     domain.CategoryMappingStatus
	GroupCode  *string
	SourceNote *string
	UpdatedBy  int64
}

// UpsertCategoryMapping 是 admin 寫入 categoryId→cropCode/groupCode mapping
// 的 usecase（issue #2786 Task 8）：寫入前驗證 categoryId 是否存在且 ACTIVE
// （透過 categoryTreeSource），resolved 狀態下另外驗證目標 crop/group code
// 是否 active（透過 domain.CategoryMappingRepository）。
type UpsertCategoryMapping struct {
	repo domain.CategoryMappingRepository
	src  categoryTreeSource
}

func NewUpsertCategoryMapping(repo domain.CategoryMappingRepository, src categoryTreeSource) *UpsertCategoryMapping {
	return &UpsertCategoryMapping{repo: repo, src: src}
}

func (u *UpsertCategoryMapping) Do(ctx context.Context, in UpsertCategoryMappingInput) (domain.CategoryMapping, error) {
	// 形狀驗證放最前面：它不需要任何 I/O，錯誤輸入不必先付一趟 gRPC 往返的
	// 代價。沒有這層，格式錯誤的 admin 輸入會一路撞到 DB 的 CHECK
	// constraint，經 gRPC Internal 映射成 HTTP 503「服務不可用」，對操作者
	// 是完全誤導的錯誤。
	if err := validateSelectionShape(in); err != nil {
		return domain.CategoryMapping{}, err
	}
	// includeDeleted=true：admin 寫入路徑必須看得到 INACTIVE 節點，才能把
	// 「整棵樹都查無此 categoryId」（ErrCategoryNotFound）與「找得到但已停用」
	// （ErrCategoryInactive）分辨開來。product-service 對空 status 是
	// active-only（見 CategoryGrpcService.getCategoryTree），只傳 status 篩不
	// 出停用節點。
	tree, err := u.src.GetCategoryTree(ctx, "", true)
	if err != nil {
		return domain.CategoryMapping{}, err
	}
	node := findCategoryNode(tree.GetCategories(), in.CategoryID)
	if node == nil {
		return domain.CategoryMapping{}, ErrCategoryNotFound
	}
	// 比對 CommonStatus 的 Value 而不是 Code——比對 "ACTIVE" 會讓每一筆正常的
	// admin 寫入都被誤判成停用分類（見 CategoryNodeStatusActive 的說明）。
	if node.GetStatus() != CategoryNodeStatusActive {
		return domain.CategoryMapping{}, ErrCategoryInactive
	}
	if in.Status == domain.CategoryMappingResolved {
		name, err := u.repo.ActiveGroupName(ctx, *in.GroupCode)
		if err != nil {
			return domain.CategoryMapping{}, err
		}
		if name == nil {
			return domain.CategoryMapping{}, ErrTargetCodeInactive
		}
	}
	saved, err := u.repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID: in.CategoryID, Status: in.Status, GroupCode: in.GroupCode,
		MappingVersion: CategoryMappingVersion, SourceNote: in.SourceNote, UpdatedBy: in.UpdatedBy,
	})
	if err != nil {
		return domain.CategoryMapping{}, err
	}
	return *saved, nil
}

// validateSelectionShape：status != resolved → group_code 必須是 nil；
// status == resolved → group_code 必須非 nil。取代原本的三分支
// crop/group/未知 oneof 檢查。
func validateSelectionShape(in UpsertCategoryMappingInput) error {
	if in.Status != domain.CategoryMappingResolved {
		if in.GroupCode != nil {
			return ErrIncompleteSelection
		}
		return nil
	}
	if in.GroupCode == nil {
		return ErrIncompleteSelection
	}
	return nil
}

func findCategoryNode(nodes []*categorypb.CategoryNode, categoryID string) *categorypb.CategoryNode {
	for _, n := range nodes {
		if n.GetId() == categoryID {
			return n
		}
		if found := findCategoryNode(n.GetChildren(), categoryID); found != nil {
			return found
		}
	}
	return nil
}
