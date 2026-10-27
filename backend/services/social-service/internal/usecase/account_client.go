package usecase

import "context"

// AccountInfo holds the account fields needed by social-service.
type AccountInfo struct {
	Nickname string
	Avatar   string
}

// 契約（所有 Get*Profile / Get*Info 實作共用）：
//
//	(*AccountInfo, nil) → 查到了
//	(nil, nil)          → 這個 id 在該服務不存在（或該服務回了業務錯誤碼）
//	(nil, err)          → 傳輸層失敗，什麼都不知道
//
// 為什麼把「查不到」明確表達成 nil：多數 caller 只是要拿暱稱／頭像，查不到留空即可；
// 回 nil 讓 caller 能分辨「不存在」與「查到但名稱空白」，而不是拿到一個無法與真實資料區分的空物件。

// AccountClient defines the interface for account-service operations.
type AccountClient interface {
	GetAccountInfo(ctx context.Context, accountID int64) (*AccountInfo, error)
	GetAccountInfoBatch(ctx context.Context, accountIDs []int64) (map[int64]*AccountInfo, error)
	SuspendAccount(ctx context.Context, accountID int64, reason string) error
}

// PrincipalResolution is the outcome of asking a role service to map one of its
// own surrogate ids (buyer_id / store_id) back to the login principal.
//
// It exists so a WRITE path can tell "the id is not mine" (Found=false, safe to
// fall through) apart from "I could not answer" (a returned error, not safe to
// guess) — a distinction ResolvePrincipal deliberately flattens for read paths.
type PrincipalResolution struct {
	AccountID int64
	Found     bool
}

// BuyerClient defines the interface for buyer-service operations.
type BuyerClient interface {
	GetBuyerProfile(ctx context.Context, buyerID int64) (*AccountInfo, error)
	GetBuyerMetaBatch(ctx context.Context, buyerIDs []int64) (map[int64]*AccountInfo, error)
	GetSubscriberIDs(ctx context.Context, sellerID int64) ([]int64, error)
	ResolvePrincipal(ctx context.Context, buyerID int64) int64
	ResolvePrincipalStrict(ctx context.Context, buyerID int64) (PrincipalResolution, error)
}

// SellerClient defines the interface for seller-service operations.
type SellerClient interface {
	GetSellerInfo(ctx context.Context, storeID int64) (*AccountInfo, error)
	GetSellerMetaBatch(ctx context.Context, storeIDs []int64) (map[int64]*AccountInfo, error)
	ResolvePrincipal(ctx context.Context, storeID int64) int64
	ResolvePrincipalStrict(ctx context.Context, storeID int64) (PrincipalResolution, error)
}

// ReviewModerationClient calls review-service for B-135 HIDE_TARGET dispatch.
type ReviewModerationClient interface {
	HideReview(ctx context.Context, reviewID int64, reason string) error
}

// IMModerationClient calls im-service for B-135 HIDE_TARGET dispatch.
type IMModerationClient interface {
	HideMessage(ctx context.Context, messageID int64, reason string) error
}
