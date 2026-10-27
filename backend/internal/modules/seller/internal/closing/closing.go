// Package closing 是 stint 的關閉與恢復：隨帳號刪除的等待與關閉（accountdeletion 的參與者）、賣家自己關店、終止合作生效後由系統關店
// （termination.go），以及終止合作撤銷時恢復營業（Reopen）。關店可逆：停權不阻擋關閉，關閉之後 logo 與封面、media 的引用與下游資料都留著，到退場
// （purge 的第一段清理）才釋放並發 StoreRetired。所有關店走同一條路徑（close）：結束帳號上預告期間的預告終止（條款第 21.2 條）、移除 SELLER、發
// StoreClosed。
//
// 鎖順序：identity 帳號鎖 → LockAccountStores（帳號全部的 stint，依 store_id）。措施的寫入只鎖 LockAccountStores，所以關店不能先鎖目前的 stint 再寫措施
// （與措施寫入的升序鎖會互鎖）。
package closing

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Identity 是關店與恢復需要的 identity 公開 API：鎖帳號列（鎖順序：帳號列先於店家列）、關閉時移除 SELLER、恢復時授予 SELLER 並確認帳號仍是存活的。
type Identity interface {
	LockAccount(context.Context, int64) error
	RemoveRole(context.Context, identity.RemoveRoleInput) error
	GrantRole(context.Context, identity.GrantRoleInput) error
	Account(context.Context, int64) (identity.Account, error)
}

// Orders 回報店家有沒有未結訂單（售後處理中的也算）。訂單在 Java 期間是遠端查詢，只能在 transaction 外呼叫；
// accountdeletion 的刪帳號用同一個實作。回傳錯誤時關店一律拒絕（fail-closed）。
type Orders interface {
	HasOpenStoreOrders(ctx context.Context, storeID int64) (bool, error)
}

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Identity Identity
	Orders   Orders
	Exposure *exposure.Service
	Now      func() time.Time
}

// Service 是關閉的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// now 是寫入用的現在：截到微秒（Postgres 的 timestamptz 精度），之後與已存的時間比較才不會被奈秒的差異誤判。
func (s *Service) now() time.Time { return s.d.Now().UTC().Truncate(time.Microsecond) }

// Lock 在呼叫方的 transaction 內鎖住店家所屬帳號的店家列，不改任何東西。前置條件同 BeginDeletion。
func (s *Service) Lock(ctx context.Context, storeID int64) error {
	_, err := stint.LockAccountOf(ctx, store.New(s.d.DB.Conn(ctx)), storeID)
	return err
}

// BeginDeletion 讓目前的 stint 隨帳號刪除進入等待，同一個 transaction 發出 StorePendingDeletion 並排入快取失效；已在
// 等待時不做任何事（不重發），已關閉時成功（重送安全）。
//
// 前置條件：只由 accountdeletion 在它已以 identity.LockAccount 鎖住帳號列的 transaction 內呼叫（鎖順序：帳號列先於
// 買家列、店家列）。
func (s *Service) BeginDeletion(ctx context.Context, storeID int64) error {
	if err := model.Positive(storeID); err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		current, err := stint.LockAccountOf(ctx, q, storeID)
		if err != nil {
			return err
		}
		if current.Status != string(model.Active) {
			return nil
		}
		now := s.now()
		if err := q.MarkPendingDeletion(ctx, store.MarkPendingDeletionParams{StoreID: storeID, Now: now}); err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.StorePendingDeletion{
			EventID: uuid.NewV7(), OccurredAt: now,
			StoreID: current.StoreID, StoreCode: current.StoreCode, AccountID: current.AccountID,
		}); err != nil {
			return err
		}
		// 營運中是可曝光的輸入：等待刪除的店不再可曝光。
		if err := s.d.Exposure.Announce(ctx, q, now, current.StoreID); err != nil {
			return err
		}
		return stint.Invalidate(ctx, s.d.Jobs, current)
	})
}

// CloseForDeletion 關閉 stint；已關閉時成功（重送安全）。
//
// 前置條件同 BeginDeletion：只由 accountdeletion 在已鎖住帳號列、並在同一個 transaction 刪除帳號時呼叫。
func (s *Service) CloseForDeletion(ctx context.Context, storeID int64) error {
	if err := model.Positive(storeID); err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		current, err := stint.LockAccountOf(ctx, q, storeID)
		if err != nil {
			return err
		}
		return s.close(ctx, q, current, closer{by: model.ClosedByDeletion})
	})
}

// closer 是這次關店的原因：賣家自己、終止合作（帶造成關店的措施）或隨帳號刪除。
type closer struct {
	by        model.ClosedBy
	measureID *uuid.UUID
}

// CloseCurrent 關閉帳號目前的店家（賣家自己關店）；帳號與買家留著。店家要有 CapAccountClose：停權中也可以關；等待刪除的
// 店家回 FailedPrecondition（它由刪除帳號的流程關閉）。店家還有未結訂單（含未付款與售後處理中）時回 FailedPrecondition，
// 查詢失敗回 Unavailable、不關店。沒有目前的店家時回 NotFound。
func (s *Service) CloseCurrent(ctx context.Context, accountID int64) (model.Store, error) {
	if err := model.Positive(accountID); err != nil {
		return model.Store{}, err
	}
	checked, err := s.checkClosable(ctx, accountID)
	if err != nil {
		return model.Store{}, err
	}
	var closed model.Store
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Identity.LockAccount(ctx, accountID); err != nil {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		stores, err := q.LockAccountStores(ctx, accountID)
		if err != nil {
			return err
		}
		current, ok := currentOf(stores)
		if !ok {
			return apperr.NotFound("找不到店家")
		}
		if current.StoreID != checked {
			return apperr.Unavailable("店家剛剛有變動，請重試")
		}
		now := s.now()
		loaded, err := stint.Load(ctx, q, current, now)
		if err != nil {
			return err
		}
		if err := model.Authorize(loaded, model.CapAccountClose); err != nil {
			return err
		}
		if err := s.close(ctx, q, current, closer{by: model.ClosedByOwner}); err != nil {
			return err
		}
		row, err := q.StoreByID(ctx, current.StoreID)
		if err != nil {
			return err
		}
		closed, err = stint.Load(ctx, q, row, now)
		return err
	})
	if err != nil {
		return model.Store{}, err
	}
	return closed, nil
}

// currentOf 從帳號全部的 stint 挑出目前的那一個（status 不是 CLOSED；帳號最多一個）。
func currentOf(stores []store.SellerStore) (store.SellerStore, bool) {
	for _, row := range stores {
		if row.Status != string(model.Closed) {
			return row, true
		}
	}
	return store.SellerStore{}, false
}

// checkClosable 在 transaction 外確認帳號目前的店家可以關（有 CapAccountClose、沒有未結訂單），回傳那個 stint 的 store_id。
// 未結訂單是遠端查詢，不在 transaction 內：這裡的讀取不加鎖，只當前置檢查，交易內鎖住之後會重新檢查能力，並確認還是同一個 stint。
// 查完到關閉之間才進來的訂單與買家端一樣有競態（ordering 搬進單體後 port 移進 transaction）。
func (s *Service) checkClosable(ctx context.Context, accountID int64) (int64, error) {
	q := store.New(s.d.DB.Conn(ctx))
	current, err := q.CurrentStore(ctx, accountID)
	if err != nil {
		return 0, model.Error(err)
	}
	loaded, err := stint.Load(ctx, q, current, s.now())
	if err != nil {
		return 0, err
	}
	if err := model.Authorize(loaded, model.CapAccountClose); err != nil {
		return 0, err
	}
	open, err := s.d.Orders.HasOpenStoreOrders(ctx, current.StoreID)
	if err != nil {
		return 0, apperr.WithCause(apperr.Unavailable("暫時無法確認店家未完成的訂單"), err)
	}
	if open {
		return 0, apperr.FailedPrecondition("店家還有未完成的訂單，請等訂單結束後再關店")
	}
	return current.StoreID, nil
}

// close 在呼叫方的 transaction 內關閉已鎖定的 stint，是所有關店共用的唯一路徑：記下關店的原因、結束帳號上尚未結束的預告終止（條款第 21.2 條：
// 預告期間內關店，或生效後關店，措施都隨店家關閉結束，否則 stint.Derive 會永遠擋住 21.2 的重新申請）、移除 SELLER，並發出 StoreClosed。
// 關店可逆，所以 logo、封面與 media 的引用留著，到個資清理才釋放。停權不解除（停權以帳號為單位）。呼叫方已鎖住帳號列與帳號全部的 stint。
func (s *Service) close(ctx context.Context, q *store.Queries, current store.SellerStore, by closer) error {
	if current.Status == string(model.Closed) {
		return nil
	}
	now := s.now()
	if err := q.CloseStore(ctx, store.CloseStoreParams{StoreID: current.StoreID, Now: now}); err != nil {
		return err
	}
	if err := q.OpenStoreClosure(ctx, store.OpenStoreClosureParams{
		StoreID: current.StoreID, Now: now, ClosedBy: string(by.by), ClosedByMeasureID: by.measureID,
	}); err != nil {
		return err
	}
	ended, err := s.endNotices(ctx, q, current.AccountID, now)
	if err != nil {
		return err
	}
	if ended > 0 {
		if err := suspension.Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, current.AccountID, now); err != nil {
			return err
		}
	}
	if err := s.d.Identity.RemoveRole(ctx, identity.RemoveRoleInput{
		AccountID: current.AccountID, Role: identity.RoleSeller, SourceID: &current.StoreID,
	}); err != nil {
		return err
	}
	if err := events.Publish(ctx, s.d.Events, model.StoreClosed{
		EventID: uuid.NewV7(), OccurredAt: now,
		StoreID: current.StoreID, StoreCode: current.StoreCode, AccountID: current.AccountID,
	}); err != nil {
		return err
	}
	// 關店之後不再可曝光（也補發最後公告為可曝光的店的 false）。
	if err := s.d.Exposure.Announce(ctx, q, now, current.StoreID); err != nil {
		return err
	}
	return stint.Invalidate(ctx, s.d.Jobs, current)
}

// endNotices 把帳號上 ENFORCED、事由是 NOTICE、尚未結束（預定生效或生效中）的終止合作各寫一版 ends_at = 關店時間（作者是系統，原因
// STORE_CLOSED），並發 MeasureEnded；回傳結束的措施數。措施列在這裡鎖住（呼叫方已鎖住帳號全部的 stint）。
func (s *Service) endNotices(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (int, error) {
	notices, err := q.OpenNoticeTerminations(ctx, accountID)
	if err != nil {
		return 0, err
	}
	cause := string(model.RevisionStoreClosed)
	for _, m := range notices {
		if _, err := q.AdvanceMeasureRevision(ctx, store.AdvanceMeasureRevisionParams{
			ID: m.ID, ExpectedVersion: m.Version, StatementToSeller: m.StatementToSeller, EndsAt: &now, SystemCause: &cause, Now: now,
		}); err != nil {
			return 0, err
		}
		if err := events.Publish(ctx, s.d.Events, model.MeasureEnded{
			EventID: uuid.NewV7(), OccurredAt: now, MeasureID: m.ID, StoreID: m.StoreID, AccountID: m.AccountID,
		}); err != nil {
			return 0, err
		}
	}
	return len(notices), nil
}

// ReopenPlan 回答撤銷終止合作之後要不要恢復帳號因終止合作而關閉的店家（要在改動任何請求與措施之前問：恢復與否不能被同一個 transaction 裡結案的待處理申覆改變，
// 否則「期滿前提出、期滿後裁定接受」的申覆會因為 legal hold 在裁定的那一刻消失而不能恢復）。revoking 是同一個 transaction 要撤銷的全部措施（含連帶撤銷的）。
// 恢復 ⇔ 帳號最近的 stint 是由終止合作關閉的、還沒退場（第一段清理之後下游已刪除），而且造成關店的那一筆（目前關店紀錄的 closed_by_measure_id，關店當時的事實）已撤銷（這次撤銷之前就已撤銷，或在 revoking 裡）、
// 撤銷之後帳號沒有任何仍生效中的終止合作（預定生效的預告終止不算：預告期間店家照常營業，由 job 之後關店）、保留期還沒滿（RetireDueStores 不選它，或申覆的 legal hold
// 擋著；與清理同一個判斷式，已撤銷的終止不延長保留期）、帳號是存活的（不在等待刪除）。被別的終止關閉的店不會因為另一筆終止撤銷而重開（條款第 21.4 條：預告終止先關店，
// 之後在同一個已關閉的店建立並撤銷 21.3，店仍因預告終止而關閉）。帳號已開新店時最近的 stint 是新店，不恢復。回傳該 stint 的 store_id；不恢復時為 0。
// 呼叫方已鎖住帳號列與帳號全部的 stint。
func (s *Service) ReopenPlan(ctx context.Context, q *store.Queries, accountID int64, revoking []uuid.UUID, now time.Time) (int64, error) {
	target, err := q.LatestStoreOfAccount(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if target.Status != string(model.Closed) || target.RetiredAt != nil {
		return 0, nil
	}
	closures, err := q.CurrentStoreClosures(ctx, []int64{target.StoreID})
	if err != nil {
		return 0, err
	}
	if len(closures) != 1 {
		return 0, fmt.Errorf("seller：已關閉的 stint %d 沒有目前的關店紀錄", target.StoreID)
	}
	closure := closures[0]
	if closure.ClosedBy != string(model.ClosedByTermination) || closure.ClosedByMeasureID == nil {
		return 0, nil
	}
	closedBy := *closure.ClosedByMeasureID
	if !slices.Contains(revoking, closedBy) {
		closing, err := q.MeasureByID(ctx, closedBy)
		if err != nil {
			return 0, err
		}
		if closing.Status != string(model.MeasureStatusRevoked) {
			return 0, nil
		}
	}
	sanctions, err := stint.Effective(ctx, q, accountID, now)
	if err != nil {
		return 0, err
	}
	for _, m := range sanctions.Measures {
		if m.Kind == model.MeasureTermination && !slices.Contains(revoking, m.MeasureID) {
			return 0, nil
		}
	}
	// 「保留期已過」＝第一段清理（退場）的時間條件成立而且沒有申覆的 legal hold（清理與恢復同一個判斷式）。主管機關來函的保存只擋清理、不改變業務結果：
	// 恢復營業不問它（step10e-3cb §2.4、TestReopenIgnoresTheAuthorityHold）。
	due, err := q.RetireDueStores(ctx, store.RetireDueStoresParams{
		StoreIds: []int64{target.StoreID}, Now: now, AppealWindowHours: model.AppealWindowHours, Hours: model.StoreRetireAfterHours, BatchLimit: 1,
	})
	if err != nil {
		return 0, err
	}
	if len(due) > 0 {
		held, err := q.StoreHolds(ctx, store.StoreHoldsParams{StoreIds: []int64{target.StoreID}, Now: now})
		if err != nil {
			return 0, err
		}
		if len(held) == 0 || !held[0].AppealHold {
			return 0, nil
		}
	}
	account, err := s.d.Identity.Account(ctx, target.AccountID)
	if err != nil {
		return 0, err
	}
	if account.Lifecycle != identity.LifecycleActive {
		return 0, nil
	}
	return target.StoreID, nil
}

// Reopen 在撤銷終止合作的 transaction 內恢復 ReopenPlan 決定的店家：同一個店家（store_id、店號、商品、KYC、資料都在）CLOSED → ACTIVE，
// 結束目前的關店紀錄（歷史留在 store_closures），重新授予 SELLER，重新公告可曝光，發 StoreReopened，並把關店期間被放棄的 PayUni 綁定補排回去。
// 措施已先撤銷（呼叫方先寫），所以公告的可曝光不含它的停權；停權期間由呼叫方稍後的 Recompute 對齊。呼叫方已鎖住帳號列與帳號全部的 stint。
func (s *Service) Reopen(ctx context.Context, q *store.Queries, storeID int64, now time.Time) error {
	ended, err := q.EndStoreClosure(ctx, store.EndStoreClosureParams{StoreID: storeID, Now: &now})
	if err != nil {
		return err
	}
	if ended != 1 {
		return apperr.FailedPrecondition("店家不是關閉狀態，不能恢復")
	}
	reopened, err := q.ReopenStore(ctx, store.ReopenStoreParams{StoreID: storeID, Now: now})
	if err != nil {
		return model.Error(err)
	}
	if err := s.d.Identity.GrantRole(ctx, identity.GrantRoleInput{
		AccountID: reopened.AccountID, Role: identity.RoleSeller, SourceID: &reopened.StoreID,
	}); err != nil {
		return err
	}
	if err := payuni.Resume(ctx, q, s.d.Jobs, storeID); err != nil {
		return err
	}
	if err := events.Publish(ctx, s.d.Events, model.StoreReopened{
		EventID: uuid.NewV7(), OccurredAt: now, StoreID: reopened.StoreID, StoreCode: reopened.StoreCode, AccountID: reopened.AccountID,
	}); err != nil {
		return err
	}
	if err := s.d.Exposure.Announce(ctx, q, now, storeID); err != nil {
		return err
	}
	return stint.Invalidate(ctx, s.d.Jobs, reopened)
}
