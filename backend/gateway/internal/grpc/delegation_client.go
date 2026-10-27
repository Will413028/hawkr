package grpc

import (
	"context"
	"fmt"
	"time"

	pkgdelegation "github.com/Will413028/hawkr/go-pkg/delegation"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔集中 ADM 代行營運（56 號）用到的 gRPC 呼叫。它們與各服務既有的賣家自助方法
// 分開存在，理由是每一支都多帶兩樣東西：DelegationContext（授權語境）與 per-hop
// deadline（§8.5.1 (B)，逾時預算是呼叫端的責任）。把 delegation 硬塞進既有簽章會讓
// 每個賣家路徑的呼叫點都得傳一個 nil，而那個 nil 有一天會被填錯。
//
// deadlineMs <= 0 時不覆寫呼叫端的 deadline：測試 holder 沒有 config，
// 而「沒有設定」不該被翻譯成「零秒逾時」。

func withDeadline(ctx context.Context, deadlineMs int) (context.Context, context.CancelFunc) {
	if deadlineMs <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(deadlineMs)*time.Millisecond)
}

// requireCapability pins one call to the capability its domain service declares in
// S3（§6.2.4）。每一支 wrapper 在自己第一行寫死那個字面常數，是 S3 的呼叫端鏡像。
//
// 沒有這道鏡像時，「這一跳送錯 capability」的唯一偵測器是實跑：領域服務回 9262、
// 端點 100% 不可用，而所有把領域 client stub 掉的單元測試照樣全綠——stub 什麼都不驗。
// storefront PATCH 就是以這個狀態出廠的：它在 Write 內取 before 快照時沿用了寫入跳的
// context（storefront.write），而 GetSeller 宣告的是 storefront.read。
//
// 拒絕的形狀刻意做成領域拒絕的樣子（PermissionDenied ＋「[NNNN] 」前綴），讓它經
// denialFromGrpcErr 還原成業務碼；請求不會送出去——一跳與自己的契約不符是 Gateway
// 的 bug，不該讓被呼叫端去裁決。
//
// 碼是 9289 而不是 9262（#2744）。這裡的成因逐字就是上面那段講的 storefront PATCH：
// **我方掛錯路由**。回 9262「此代行工作階段未取得該項操作權限」會把它講成操作者的
// 授權問題 —— 而 requireCapability 的拒絕會以 Result=FAILED ＋ 該碼寫進
// admin_operation_audit_logs，於是事後翻帳看到的是「這位管理員權限不足」。
// **稽核說謊比回錯碼嚴重**，因為它在爭議發生時才被讀，而那時已經沒人記得真相。
//
// ⚠️ gRPC status 維持 PermissionDenied：denialFromGrpcErr 先解析前綴、只有沒有前綴
// 時才看 status，而 codes.Internal 在它的傳輸失敗清單裡會短路成 9280。
func requireCapability(d *commonpb.DelegationContext, rpc string, want pkgdelegation.Capability) error {
	got := pkgdelegation.Capability(d.GetCapability())
	if got == want {
		return nil
	}
	zap.L().Error("Delegation.capability.mismatch",
		zap.String("rpc", rpc), zap.String("want", string(want)), zap.String("got", string(got)))
	return status.Error(codes.PermissionDenied, fmt.Sprintf("[%d] delegation capability mismatch: %s requires %q but the context carries %q",
		bizcode.DelegationInternalContractMismatch, rpc, want, got))
}

// ==================== account-service：兩階段稽核與查詢 ====================

// RecordAuditIntent writes the PENDING audit row that every delegated mutation must
// have before it touches any domain state. Failure is fail-closed (9279): the caller
// must NOT proceed to the domain write.
func (c *AccountClient) RecordAuditIntent(ctx context.Context, req *accountpb.RecordAuditIntentRequest, deadlineMs int) (*accountpb.RecordAuditIntentResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.RecordAuditIntent(ctx, req)
}

// ConfirmAuditResult closes the audit row opened by RecordAuditIntent with the real
// outcome, and is what makes the seller-notification outbox row appear (only when the
// intent recorded notify_seller = true and the result is SUCCEEDED).
func (c *AccountClient) ConfirmAuditResult(ctx context.Context, req *accountpb.ConfirmAuditResultRequest, deadlineMs int) (*accountpb.ConfirmAuditResultResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ConfirmAuditResult(ctx, req)
}

// RecordAuditIntentBatch is the O(1)-hop form for batch operations (§7.2.8): one
// transaction with a SAVEPOINT per item, so one failing item does not abort the rest.
func (c *AccountClient) RecordAuditIntentBatch(ctx context.Context, req *accountpb.RecordAuditIntentBatchRequest, deadlineMs int) (*accountpb.RecordAuditIntentBatchResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.RecordAuditIntentBatch(ctx, req)
}

// ConfirmAuditResultBatch is all-or-nothing by design: a failure leaves N rows in
// PENDING rather than an unknown mixture, which is the one regression this shape
// accepts and must never be reported as success.
func (c *AccountClient) ConfirmAuditResultBatch(ctx context.Context, req *accountpb.ConfirmAuditResultBatchRequest, deadlineMs int) (*accountpb.ConfirmAuditResultBatchResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ConfirmAuditResultBatch(ctx, req)
}

// ListDelegationAuditLogs backs GET .../{sessionId}/audit-logs.
func (c *AccountClient) ListDelegationAuditLogs(ctx context.Context, req *accountpb.ListAuditLogsRequest, deadlineMs int) (*accountpb.ListAuditLogsResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ListAuditLogs(ctx, req)
}

// GetDelegationSessionSummary backs BOTH summary endpoints — the ADM-authenticated
// one (session_id from the URL) and the credential-authenticated one (session_id
// resolved from the credential). One RPC on purpose: two would mean two
// implementations of "zero fulfilment, zero finance".
func (c *AccountClient) GetDelegationSessionSummary(ctx context.Context, sessionID string, deadlineMs int) (*accountpb.GetSessionSummaryResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.GetSessionSummary(ctx, &accountpb.GetSessionSummaryRequest{SessionId: sessionID})
}

// ListDelegationSessions backs GET .../delegated-operations（跨工作階段治理列表）。
// 唯讀，deadline 沿用 CurrentMs——與其他查詢同級（單張表一次 OFFSET 分頁 ＋ 一次 count）。
func (c *AccountClient) ListDelegationSessions(ctx context.Context, req *accountpb.ListSessionsRequest, deadlineMs int) (*accountpb.ListSessionsResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ListSessions(ctx, req)
}

// ListAllDelegationAuditLogs backs GET .../delegated-operations/audit-logs（跨 session）。
func (c *AccountClient) ListAllDelegationAuditLogs(ctx context.Context, req *accountpb.ListAllAuditLogsRequest, deadlineMs int) (*accountpb.ListAllAuditLogsResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ListAllAuditLogs(ctx, req)
}

// ==================== account-service：代行授權列治理（#2444） ====================
//
// 這六支與上面的稽核／工作階段呼叫不同層：它們是**治理設定**動作，發生在任何工作階段
// 之前，因此不帶 DelegationContext、不驗 capability——requireCapability 那道鏡像對它們
// 沒有意義（沒有一跳是「代行操作」）。授權者資格由 account-service 以
// admin_accounts.delegation_enabled 判定並回 9288。
//
// deadline 沿用既有兩個參數：唯讀走 CurrentMs、寫入走 CreateMs。刻意不新增 config 欄位
// ——多一個環境變數就多一處部署時可能漏設的地方，而這組端點的延遲特性與既有查詢／建立
// 完全同級（單張表的一次 CAS，最多加一次 session 連帶撤銷）。

// ListDelegationAuthorizations backs GET .../delegated-operations/authorizations.
func (c *AccountClient) ListDelegationAuthorizations(ctx context.Context, req *accountpb.ListDelegationAuthorizationsRequest, deadlineMs int) (*accountpb.ListDelegationAuthorizationsResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ListDelegationAuthorizations(ctx, req)
}

// GetDelegationAuthorization backs GET .../authorizations/{store_id}. 查無授權列不是
// 錯誤：回 code=0 ＋ data.exists=false，由 handler 原樣投影。
func (c *AccountClient) GetDelegationAuthorization(ctx context.Context, req *accountpb.GetDelegationAuthorizationRequest, deadlineMs int) (*accountpb.GetDelegationAuthorizationResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.GetDelegationAuthorization(ctx, req)
}

// GrantDelegationAuthorization backs POST .../authorizations（首建與復權共用一支）。
func (c *AccountClient) GrantDelegationAuthorization(ctx context.Context, req *accountpb.GrantDelegationAuthorizationRequest, deadlineMs int) (*accountpb.GrantDelegationAuthorizationResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.GrantDelegationAuthorization(ctx, req)
}

// UpdateDelegationAuthorization backs PUT .../authorizations/{store_id}（全量取代）。
func (c *AccountClient) UpdateDelegationAuthorization(ctx context.Context, req *accountpb.UpdateDelegationAuthorizationRequest, deadlineMs int) (*accountpb.UpdateDelegationAuthorizationResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.UpdateDelegationAuthorization(ctx, req)
}

// RevokeDelegationAuthorization backs POST .../authorizations/{store_id}/revoke。
// 它會在 account-service 的同一個交易內連帶中止該店進行中的工作階段，因此比其他寫入
// 多做一件事——deadline 仍沿用 CreateMs：連帶撤銷是同一個交易內的一句 UPDATE，不是
// 額外的網路往返。
func (c *AccountClient) RevokeDelegationAuthorization(ctx context.Context, req *accountpb.RevokeDelegationAuthorizationRequest, deadlineMs int) (*accountpb.RevokeDelegationAuthorizationResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.RevokeDelegationAuthorization(ctx, req)
}

// ListDelegationAuthorizationHistory backs GET .../authorizations/{store_id}/history.
func (c *AccountClient) ListDelegationAuthorizationHistory(ctx context.Context, req *accountpb.ListDelegationAuthorizationHistoryRequest, deadlineMs int) (*accountpb.ListDelegationAuthorizationHistoryResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.delegation.ListDelegationAuthorizationHistory(ctx, req)
}

// ==================== seller-service ====================

// GetStoreStateForGuard reads the target store's lifecycle state for 授權鏈第 8 項
// (§8.1.3 第 8 項) — "can this store still be operated on behalf of right now".
//
// It deliberately sends NO DelegationContext. This hop is the Gateway's own
// authorization judgment, taken BEFORE anything is allowed through, so there is no
// audit_log_id to carry and nothing to authorize yet; seller-service therefore takes
// the S0 branch (d == nil ⇒ existing path). Filling delegation here would get the
// call rejected as a write-context read with an empty audit_log_id.
//
// The verdict the caller reads is not only lifecycle_status. seller-service answers
// this lookup with a non-zero envelope code in two cases that both mean "not
// delegable": a soft-deleted store (the lookup filters deleted_at IS NULL ⇒ NotFound)
// and a closed one (handler.go:114 answers 4040 for public callers). Both land as
// 9265 at the call site, exactly as account-service's preflight already treats them
// (internal/client/seller/client.go:95).
func (c *SellerClient) GetStoreStateForGuard(ctx context.Context, storeID int64, deadlineMs int) (*sellerpb.SellerResponse, error) {
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.GetSeller(ctx, &sellerpb.GetSellerRequest{StoreId: storeID})
}

// ==================== product-service ====================

func (c *ProductClient) GetProductListForDelegation(ctx context.Context, req *productpb.GetProductListRequest, deadlineMs int) (*productpb.GetProductListResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.GetProductList", pkgdelegation.CapProductsRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.GetProductList(ctx, req)
}

func (c *ProductClient) GetProductForDelegation(ctx context.Context, req *productpb.GetProductRequest, deadlineMs int) (*productpb.GetProductResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.GetProduct", pkgdelegation.CapProductsRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.GetProduct(ctx, req)
}

func (c *ProductClient) CreateProductForDelegation(ctx context.Context, req *productpb.CreateProductRequest, deadlineMs int) (*productpb.CreateProductResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.CreateProduct", pkgdelegation.CapProductsWrite); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.CreateProduct(ctx, req)
}

func (c *ProductClient) UpdateProductForDelegation(ctx context.Context, req *productpb.UpdateProductRequest, deadlineMs int) (*productpb.UpdateProductResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.UpdateProduct", pkgdelegation.CapProductsWrite); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.UpdateProduct(ctx, req)
}

func (c *ProductClient) UpdateSellerProductStatusForDelegation(ctx context.Context, req *productpb.UpdateSellerProductStatusRequest, deadlineMs int) (*productpb.UpdateProductResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.UpdateSellerProductStatus", pkgdelegation.CapProductsPublish); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.UpdateSellerProductStatus(ctx, req)
}

// BatchGetSellerProductsForDelegation is hop ② of the batch shape and runs BEFORE the
// audit intent exists, so its DelegationContext carries an empty audit_log_id and the
// read capability (products.read). Declaring products.publish here would make S1b
// reject every batch unpublish with 9260 — the operation would be entirely unusable
// while every unit test that only feeds write fixtures stayed green.
func (c *ProductClient) BatchGetSellerProductsForDelegation(ctx context.Context, req *productpb.BatchGetSellerProductsForDelegationRequest, deadlineMs int) (*productpb.BatchGetSellerProductsForDelegationResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.BatchGetSellerProductsForDelegation", pkgdelegation.CapProductsRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.BatchGetSellerProductsForDelegation(ctx, req)
}

func (c *ProductClient) BatchUpdateSellerProductStatusForDelegation(ctx context.Context, req *productpb.BatchUpdateSellerProductStatusRequest, deadlineMs int) (*productpb.BatchUpdateSellerProductStatusResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ProductService.BatchUpdateSellerProductStatus", pkgdelegation.CapProductsPublish); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.BatchUpdateSellerProductStatus(ctx, req)
}

// ==================== social-service ====================

func (c *SocialClient) ListMyPostsForDelegation(ctx context.Context, req *socialpb.ListMyPostsRequest, deadlineMs int) (*socialpb.PostListResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ContentService.ListMyPosts", pkgdelegation.CapPostsRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.ListMyPosts(ctx, req)
}

// GetPostDetailForDelegation must always carry owner_store_id: GetPost's SQL
// deliberately does not filter by seller (comments / moderation / hide / detail all
// share it as an existence query), so without that field Q4 has nothing to compare
// and a delegated reader could pull any seller's post detail — successfully, and
// therefore without an audit row.
func (c *SocialClient) GetPostDetailForDelegation(ctx context.Context, req *socialpb.GetPostDetailRequest, deadlineMs int) (*socialpb.PostDetailResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ContentService.GetPostDetail", pkgdelegation.CapPostsRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.GetPostDetail(ctx, req)
}

func (c *SocialClient) CreatePostForDelegation(ctx context.Context, req *socialpb.CreatePostRequest, deadlineMs int) (*socialpb.PostResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ContentService.CreatePost", pkgdelegation.CapPostsWrite); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.CreatePost(ctx, req)
}

func (c *SocialClient) UpdatePostForDelegation(ctx context.Context, req *socialpb.UpdatePostRequest, deadlineMs int) (*socialpb.PostResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ContentService.UpdatePost", pkgdelegation.CapPostsWrite); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.UpdatePost(ctx, req)
}

func (c *SocialClient) UpdateSellerPostStatusForDelegation(ctx context.Context, req *socialpb.UpdateSellerPostStatusRequest, deadlineMs int) (*socialpb.PostResponse, error) {
	if err := requireCapability(req.GetDelegation(), "ContentService.UpdateSellerPostStatus", pkgdelegation.CapPostsPublish); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.client.UpdateSellerPostStatus(ctx, req)
}

// ==================== order-service ====================

func (c *OrderClient) GetSellerOrdersForDelegation(ctx context.Context, req *orderpb.GetSellerOrdersRequest, deadlineMs int) (*orderpb.GetSellerOrdersResponse, error) {
	if err := requireCapability(req.GetDelegation(), "OrderService.GetSellerOrders", pkgdelegation.CapOrdersRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.orderClient.GetSellerOrders(ctx, req)
}

// GetSellerOrderDetailForDelegation carries reveal_recipient_contact. Everything but
// the recipient-contact endpoint leaves it at its zero value, and the zero value is
// "masked" — that is the whole reason the field is named reveal_* rather than mask_*.
func (c *OrderClient) GetSellerOrderDetailForDelegation(ctx context.Context, req *orderpb.GetSellerOrderDetailRequest, deadlineMs int) (*orderpb.GetSellerOrderDetailResponse, error) {
	if err := requireCapability(req.GetDelegation(), "OrderService.GetSellerOrderDetail", pkgdelegation.CapOrdersRead); err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, deadlineMs)
	defer cancel()
	return c.orderClient.GetSellerOrderDetail(ctx, req)
}
