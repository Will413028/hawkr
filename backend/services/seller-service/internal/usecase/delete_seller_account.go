package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/zap"
)

// errAlreadyDeletedRetryRole is an internal sentinel returned from inside the
// tx closure when the seller row is already terminally deleted. It is
// intercepted right after WithTx returns (see Execute) to skip straight to
// retrying RemoveSellerRole instead of surfacing as a real error.
var errAlreadyDeletedRetryRole = errors.New("seller already deleted, retry role removal")

// SellerDeletionRepository is the subset of SellerRepository needed for deletion.
type SellerDeletionRepository interface {
	GetByAccountIDForUpdate(ctx context.Context, accountID int64) (*domain.Seller, error)
	UpdateStatusByStoreID(ctx context.Context, storeID int64, status domain.SellerStatus, unlinkedAt *time.Time, deletedAt *time.Time, now time.Time) error
	// ClearPIIFields is keyed by stint PK (store_id) — class 2.
	ClearPIIFields(ctx context.Context, storeID int64, clearedAt time.Time) error
	ListPendingPIICleanup(ctx context.Context, before time.Time, limit int) ([]int64, error)
}

// SellerDeletionAuditRepository persists the privileged admin-override audit row.
type SellerDeletionAuditRepository interface {
	Insert(ctx context.Context, a *domain.SellerDeletionAudit) error
}

// DeleteSellerAccountInput carries who initiated the deletion and the admin override decision.
type DeleteSellerAccountInput struct {
	AccountID            int64
	IsAdmin              bool   // actor=ADMIN（gateway 從 DeleteActor 映射）
	OperatorID           int64  // admin account id（audit 用；IsAdmin 時有意義）
	OverrideActiveOrders bool   // 只在 IsAdmin 有效；繞過 9205
	Reason               string // override / admin 刪除理由（audit 用）
}

// DeleteSellerAccountUseCase handles seller deletion for both self and admin actors:
//   - ACTIVE + 0 active orders → DELETED（off-shelf、revoke token、publish event）
//   - ACTIVE + N>0 orders, actor=SELF 或 admin 未帶 override → reject 9205
//   - ACTIVE + N>0 orders, actor=ADMIN + override → 同 tx 寫 audit 後 DELETED
//   - DELETED → 9204（已在刪除流程）
type DeleteSellerAccountUseCase struct {
	repo       SellerDeletionRepository
	orders     ActiveMerchantOrderCountClient
	account    AccountRoleRemover
	buyers     BuyerClient
	audit      SellerDeletionAuditRepository
	outboxRepo domain.SellerDeletedOutboxRepository
	tsid       func() int64
	txManager  database.TxManager
	revalidate *revalidate.Client
}

// NewDeleteSellerAccountUseCase creates a new DeleteSellerAccountUseCase.
func NewDeleteSellerAccountUseCase(
	repo SellerDeletionRepository,
	orders ActiveMerchantOrderCountClient,
	account AccountRoleRemover,
	buyers BuyerClient,
	audit SellerDeletionAuditRepository,
	outboxRepo domain.SellerDeletedOutboxRepository,
	tsid func() int64,
	txManager database.TxManager,
	revalidateClient *revalidate.Client,
) *DeleteSellerAccountUseCase {
	return &DeleteSellerAccountUseCase{
		repo:       repo,
		orders:     orders,
		account:    account,
		buyers:     buyers,
		audit:      audit,
		outboxRepo: outboxRepo,
		tsid:       tsid,
		txManager:  txManager,
		revalidate: revalidateClient,
	}
}

// Execute runs the deletion flow for the seller identified by in.AccountID.
func (u *DeleteSellerAccountUseCase) Execute(ctx context.Context, in DeleteSellerAccountInput) error {
	now := time.Now() // capture before the transaction to keep timestamp consistent

	// Captured inside the transaction, spent after it. The reading path
	// (sellerCacheTags) cannot be used once this stint is soft-deleted:
	// GetSellerByAccountID filters `deleted_at IS NULL`, so a post-delete lookup
	// misses and the tag set collapses to just `seller:{account_id}` — dropping
	// the two tags the storefront page is actually cached under. The ForUpdate
	// read below deliberately has no deleted_at filter, so it still sees them.
	// stintStoreID is also the deletion's authority token: it names the stint
	// being closed, and account-service will only apply the role removal to a
	// grant that came from it (issue #3189).
	var stintStoreID, bustStoreCode int64
	err := u.txManager.WithTx(ctx, func(txCtx context.Context) error {
		seller, err := u.repo.GetByAccountIDForUpdate(txCtx, in.AccountID)
		if err != nil {
			if pkgerrors.IsNotFound(err) {
				// 業務碼透傳（envelopeCodeToGRPC 查無 9233 會落 FailedPrecondition +
				// "[9233] " 前綴，gateway writeBizCodeEnvelopeIfPrefixed 還原成
				// messageZh）——底層 repo NotFound 維持 CodeNotFound 不變，供 Create()
				// 的 IsNotFound 分支區分「首次/重新開店」，只在這個使用者可見的終端
				// 轉成中文業務碼，不動 repo 語意。
				return pkgerrors.NewBusiness(bizcode.SellerNotFound, "找不到您的商家資料")
			}
			return err
		}
		stintStoreID, bustStoreCode = seller.StoreID, seller.StoreCode
		if seller.IsClosed() {
			// 已終態：本地標記早已完成，只可能是當初 tx 外的 RemoveSellerRole 呼叫失敗
			// 而卡在半完成狀態。此處不重做狀態更新/outbox insert（會重複觸發 seller.deleted），
			// 直接跳到 tx 外重打一次 RemoveSellerRole（見下方，天生 idempotent）。
			return errAlreadyDeletedRetryRole
		}
		if !seller.IsActive() {
			// suspended_governance 等其他非 active 態：維持現行「已在刪除中」的拒絕語意不變。
			return pkgerrors.NewBusiness(bizcode.AccountAlreadyInDeletion, "帳號已在刪除流程中")
		}

		count, err := u.orders.GetActiveMerchantOrderCount(txCtx, in.AccountID)
		if err != nil {
			return err
		}
		if count > 0 && (!in.IsAdmin || !in.OverrideActiveOrders) {
			return pkgerrors.NewBusiness(bizcode.SellerHasActiveOrders,
				fmt.Sprintf("您有 %d 筆未完成訂單，請待結案後再申請刪除帳號", count))
		}
		if in.IsAdmin {
			// admin 刪除一律寫不可變稽核（不再限定「有未結訂單才寫」）：
			// 同 tx 寫入，失敗則回滾刪除，稽核不可遺漏。
			if err := u.audit.Insert(txCtx, &domain.SellerDeletionAudit{
				ID:               u.tsid(),
				AccountID:        in.AccountID,
				OperatorID:       in.OperatorID,
				ActiveOrderCount: count,
				Reason:           in.Reason,
				CreatedAt:        now,
			}); err != nil {
				return err
			}
			pkglogger.Warn(txCtx, "admin seller delete",
				zap.Int64("accountID", in.AccountID),
				zap.Int64("operatorID", in.OperatorID),
				zap.Int("activeOrderCount", count),
				zap.String("reason", in.Reason))
		}
		if err := u.repo.UpdateStatusByStoreID(txCtx, seller.StoreID, domain.SellerStatusClosed, &now, &now, now); err != nil {
			return err
		}
		row, err := domain.NewLifecycleOutboxRow(u.tsid(), in.AccountID, domain.SellerLifecycleEventDeleted, domain.SellerLifecyclePayload{
			AccountID: in.AccountID,
			StoreID:   seller.StoreID,
		}, now)
		if err != nil {
			return err
		}
		return u.outboxRepo.Insert(txCtx, row)
	})
	if err != nil {
		if errors.Is(err, errAlreadyDeletedRetryRole) {
			// Fall through to the idempotent RemoveSellerRole retry below —
			// do not surface as an error yet.
		} else {
			return err
		}
	}

	// 前端快取失效，緊接在事務之後。放這裡而不是整條流程末端，是因為此刻
	// sellers 列已經軟刪了——商家頁對使用者來說已經該消失，不該等到後面的
	// RemoveSellerRole 也成功。那一步失敗時流程會 return err（account-service
	// 不可用），若把失效綁在它後面，資料庫已刪、前端卻撐到 TTL 到期。
	//
	// 用事務內捕捉的 ids：此時再查一次只會拿到退化的 tag 集合（見上方註解）。
	// 與 outbox 事件互補——那條是給其他微服務的，這條是給 website 的 tag。
	// best-effort：失效送不出去不該讓已完成的刪除變成失敗。
	if u.revalidate.Enabled() {
		u.revalidate.BustAsync(ctx, sellerCacheTagsFrom(ctx, in.AccountID, stintStoreID, bustStoreCode))
	}

	// 查對側（buyer）真實狀態帶給 account-service 決定要不要 cascade。
	// 查不到就帶 UNKNOWN，讓 account-service fail-closed 不做不可逆的全帳號軟刪。
	hasBuyer, buyerErr := u.buyers.HasActiveBuyer(ctx, in.AccountID)
	if buyerErr != nil {
		pkglogger.Warn(ctx, "HasActiveBuyer lookup failed; passing UNKNOWN (fail-closed)",
			zap.Int64("account_id", in.AccountID), zap.Error(buyerErr))
	}
	other := rolestate.Resolve(hasBuyer, buyerErr)

	// seller.StoreID names the stint this deletion closed. account-service
	// applies the removal only if the account's current seller grant came from
	// that same stint, so a re-onboard that lands between here and the call —
	// or between here and the outbox worker's retry hours later — keeps its
	// role (issue #3189).
	if err := u.account.RemoveSellerRole(ctx, in.AccountID, other, stintStoreID); err != nil {
		return err
	}

	return nil
}
