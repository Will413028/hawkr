package payuni

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
)

// ErrPermanent 標示重試也不會成功的失敗（未知的店家、格式不合的 MerID）：job 記錄後取消，不再重試（盤點 D14）；
// 其他錯誤都由 River 重試（盤點 D9：套用失敗不能吞掉）。
var ErrPermanent = errors.New("payuni: permanent failure")

// observation 是要記錄並套用的一筆觀測。Full 表示來源回報完整的狀態（輪詢、刷新、認領時查到的）：沒有回報的欄位就是
// 沒有值；webhook 只有 MerStatus 與說明（Full 為 false），沒有回報的欄位保留目前的值，不被清掉。
type observation struct {
	Source                           model.PayUniSource
	EventKey                         *string
	MerStatus, MemberStatus, Credit  *string
	ApplePay, GooglePay, ATM, Reason *string
	ExecutedAt                       time.Time
	Full                             bool
}

// text 去掉前後空白，空字串是沒有值。
func text(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func fromStatus(st pu.MerchantStatus, source model.PayUniSource, executedAt time.Time) observation {
	return observation{
		Source: source, MerStatus: text(st.MerStatus), MemberStatus: text(st.MemberStatus), Credit: text(st.Credit),
		ApplePay: text(st.ApplePay), GooglePay: text(st.GooglePay), ATM: text(st.ATM), Reason: text(st.Reason),
		ExecutedAt: executedAt, Full: true,
	}
}

// hasStatus 表示這筆觀測有商家或會員層級的狀態碼；沒有的（例如綁定成功的紀錄）只留紀錄，不套用。
func (o observation) hasStatus() bool { return o.MerStatus != nil || o.MemberStatus != nil }

// statusFields 是合併後要寫入的狀態欄位。
type statusFields struct {
	mer, member, credit, applePay, googlePay, atm, reason *string
}

// merge 把觀測疊到目前的狀態（鎖住註冊列之後做）：完整的觀測整組取代；部分的觀測只換它回報的 MerStatus 與說明。
func merge(cur store.SellerPayuniRegistration, o observation) statusFields {
	if o.Full {
		return statusFields{o.MerStatus, o.MemberStatus, o.Credit, o.ApplePay, o.GooglePay, o.ATM, o.Reason}
	}
	f := statusFields{cur.MerStatus, cur.MemberStatus, cur.CreditStatus, cur.ApplepayStatus, cur.GooglepayStatus, cur.AtmStatus, cur.Reason}
	if o.MerStatus != nil {
		f.mer = o.MerStatus
	}
	if o.Reason != nil {
		f.reason = o.Reason
	}
	return f
}

// record 在呼叫方的 transaction 內追加一筆觀測並套用到目前的狀態，回傳這筆觀測有沒有更新目前的狀態：
//   - (store_id, event_key) 已經有了（重複的 webhook）：什麼都不做，只套用一次。
//   - 觀測一定追加（歷史），但目前的狀態只有事件時間比 status_observed_at 新的才更新（條件式 UPDATE，亂序到達的舊通知
//     不倒退，盤點 D11、D13）。
//   - 狀態有實質變化時，在同一個 transaction 發出對應的 merchant.verification 模組事件（盤點 D10）。
//
// 呼叫方要確認店家有註冊列。
func (s *Service) record(ctx context.Context, q *store.Queries, storeID int64, o observation) (bool, error) {
	now := s.d.Now().UTC()
	ins, err := q.InsertPayUniObservation(ctx, store.InsertPayUniObservationParams{
		ObservationID: uuid.NewV7(), StoreID: storeID, Source: string(o.Source), EventKey: o.EventKey,
		MerStatus: o.MerStatus, MemberStatus: o.MemberStatus, CreditStatus: o.Credit, ApplepayStatus: o.ApplePay,
		GooglepayStatus: o.GooglePay, AtmStatus: o.ATM, Reason: o.Reason, ExecutedAt: o.ExecutedAt, Now: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !o.hasStatus() {
		return false, nil
	}
	cur, err := q.LockPayUniRegistration(ctx, storeID)
	if err != nil {
		return false, err
	}
	f := merge(cur, o)
	outcome := model.PayUniOutcomeOf(model.Deref(f.mer), model.Deref(f.member))
	executed := o.ExecutedAt
	updated, err := q.ApplyPayUniStatus(ctx, store.ApplyPayUniStatusParams{
		StoreID: storeID, MerStatus: f.mer, MemberStatus: f.member, CreditStatus: f.credit, ApplepayStatus: f.applePay,
		GooglepayStatus: f.googlePay, AtmStatus: f.atm, Reason: f.reason, Outcome: string(outcome), ExecutedAt: &executed, Now: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.announce(ctx, q, cur, updated, ins.ObservationID, now); err != nil {
		return true, err
	}
	return true, s.exposureChanged(ctx, q, storeID)
}

// exposureChanged 在狀態寫入的同一個 transaction 內重新公告可曝光並排入 website 店家頁的快取失效：PayUni 的審核狀態與綁定是
// 「可公開曝光」的輸入（KYC 流程的 APPROVED 要綁定而且 PayUni 核准），改變時店面頁會在 not_found 與有資料之間切換，下游
// 隱藏或顯示商品。審核狀態只在 record 寫入，綁定結果只在 finish 寫入，兩處各呼叫一次；重新綁定與排入綁定的意圖不改變流程是否
// APPROVED，不呼叫。公告是 transaction 裡最後一個鎖（店家、註冊之後）。
func (s *Service) exposureChanged(ctx context.Context, q *store.Queries, storeID int64) error {
	if err := s.d.Exposure.Announce(ctx, q, s.d.Now().UTC(), storeID); err != nil {
		return err
	}
	return stint.InvalidateByID(ctx, q, s.d.Jobs, storeID)
}

// announce 依狀態的變化發出 merchant.verification 事件（EventID 是觸發它的那筆觀測，穩定）。規則沿用舊
// `applyOutcome`：商家或會員層級的碼有變才通知；核准、待補件、拒絕各一個；審核中只在從待補件回來時通知；認不得的狀態
// 不通知（fail closed）。
func (s *Service) announce(ctx context.Context, q *store.Queries, before, after store.SellerPayuniRegistration, observationID uuid.UUID, now time.Time) error {
	changed := model.Deref(before.MerStatus) != model.Deref(after.MerStatus) || model.Deref(before.MemberStatus) != model.Deref(after.MemberStatus)
	if !changed {
		return nil
	}
	prev, outcome := model.PayUniOutcome(before.Outcome), model.PayUniOutcome(after.Outcome)
	owner, err := q.StoreByID(ctx, after.StoreID)
	if err != nil {
		return model.Error(err)
	}
	reason := model.Deref(after.Reason)
	switch outcome {
	case model.OutcomeApproved:
		return events.Publish(ctx, s.d.Events, model.PayUniApproved{
			OccurredAt: now, EventID: observationID, ApplicationID: after.ApplicationID, StoreID: after.StoreID, AccountID: owner.AccountID, Reason: reason,
		})
	case model.OutcomePendingInfo:
		return events.Publish(ctx, s.d.Events, model.PayUniPendingInfo{
			OccurredAt: now, EventID: observationID, ApplicationID: after.ApplicationID, StoreID: after.StoreID, AccountID: owner.AccountID, Reason: reason,
		})
	case model.OutcomeRejected:
		return events.Publish(ctx, s.d.Events, model.PayUniRejected{
			OccurredAt: now, EventID: observationID, ApplicationID: after.ApplicationID, StoreID: after.StoreID, AccountID: owner.AccountID, Reason: reason,
			StatusCode: model.PayUniStatusCode(model.Deref(after.MerStatus), model.Deref(after.MemberStatus)),
		})
	case model.OutcomeUnderReview:
		if prev == model.OutcomePendingInfo {
			return events.Publish(ctx, s.d.Events, model.PayUniUnderReview{
				OccurredAt: now, EventID: observationID, ApplicationID: after.ApplicationID, StoreID: after.StoreID, AccountID: owner.AccountID,
			})
		}
	case model.OutcomeUnrecognized:
		s.d.Logger.WarnContext(ctx, "seller：PayUni 回了沒有記載的狀態，不通知", "store_id", after.StoreID, "mer_status", model.Deref(after.MerStatus), "member_status", model.Deref(after.MemberStatus))
	case model.OutcomeNone:
	}
	return nil
}

// Receive 接收 PayUni 的 AGENT_NOTIFY（由 legacy 的 RabbitMQ consumer `sellerconsumer` 呼叫），回傳時工作已持久化（consumer 此後
// 才 ack，盤點 D9）。形狀不合（沒有 event_key、MerID 不是 HAWK 加店號、沒有 MerStatus）回 InvalidInput：重送也不會好，
// consumer 記錄後進 dead-letter；MerID 格式合法但沒有這個店家回 ErrPermanent（不重試）。
//
//   - 有 ExecuteTime：排入以 event_key 為唯一鍵的 seller.payuni_observe，套用在 job（Observe）。
//   - 沒有或格式不合的 ExecuteTime：通知的先後無從排序，所以**不套用它的內容**（用收到的時間會重新引入亂序，盤點 D13），
//     改為排入這個店家的狀態刷新（和手動刷新、AGENT04017 認領同一條「向 PayUni 查詢」的路徑，以 store_id 為唯一鍵），
//     由查到的權威狀態更新。
func (s *Service) Receive(ctx context.Context, n model.PayUniNotification) error {
	eventKey := strings.TrimSpace(n.EventKey)
	if eventKey == "" || len(eventKey) > 200 {
		return apperr.InvalidInputOn([]string{"EventKey"}, "event_key 不合法")
	}
	if _, ok := model.ParseMerID(n.MerID); !ok {
		return apperr.InvalidInputOn([]string{"MerID"}, "MerID 格式不合")
	}
	merStatus := strings.TrimSpace(n.MerStatus)
	if merStatus == "" || len(merStatus) > 20 {
		return apperr.InvalidInputOn([]string{"MerStatus"}, "缺少 MerStatus")
	}
	executedAt, hasTime := model.ParseExecuteTime(n.ExecuteTime)
	note := strings.TrimSpace(n.MerStatusNote)
	if len([]rune(note)) > 500 {
		note = string([]rune(note)[:500])
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if hasTime {
			return enqueueObserve(ctx, s.d.Jobs, ObserveArgs{
				EventKey: eventKey, MerID: n.MerID, MerStatus: merStatus, MerStatusNote: note, ExecutedAt: executedAt,
			})
		}
		reg, err := store.New(s.d.DB.Conn(ctx)).PayUniRegistrationByMerID(ctx, n.MerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w：沒有 MerID %s 的註冊（event_key %s）", ErrPermanent, n.MerID, eventKey)
		}
		if err != nil {
			return err
		}
		s.d.Logger.WarnContext(ctx, "seller：PayUni 通知沒有可用的 ExecuteTime，改為刷新狀態", "store_id", reg.StoreID, "event_key", eventKey)
		return enqueuePoll(ctx, s.d.Jobs, reg.StoreID, model.SourceRefresh)
	})
}

// Observe 是 webhook job 的內容：找到 MerID 對應的店家，追加觀測並套用。未知的 MerID 回 ErrPermanent（記錄後取消，
// 不重試）；其他失敗原樣回傳，由 River 重試。
func (s *Service) Observe(ctx context.Context, a ObserveArgs) error {
	if _, ok := model.ParseMerID(a.MerID); !ok {
		return fmt.Errorf("%w：MerID %q 格式不合（event_key %s）", ErrPermanent, a.MerID, a.EventKey)
	}
	reg, err := store.New(s.d.DB.Conn(ctx)).PayUniRegistrationByMerID(ctx, a.MerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w：沒有 MerID %s 的註冊（event_key %s）", ErrPermanent, a.MerID, a.EventKey)
	}
	if err != nil {
		return err
	}
	key := a.EventKey
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := s.record(ctx, store.New(s.d.DB.Conn(ctx)), reg.StoreID, observation{
			Source: model.SourceWebhook, EventKey: &key, MerStatus: text(a.MerStatus), Reason: text(a.MerStatusNote), ExecutedAt: a.ExecutedAt,
		})
		return err
	})
}

// Poll 向 PayUni 查一次店家的審核狀態並套用（來源是輪詢或手動刷新）。事件時間是收到回應的時間。PayUni 的業務失敗
// （例如查無商店）記 log、記下問過的時間、不重試（下次輪詢再問）；傳輸錯誤回傳，由 River 重試。沒有設定 PayUni 時什麼都
// 不做。
func (s *Service) Poll(ctx context.Context, storeID int64, source model.PayUniSource) error {
	if s.d.Client == nil {
		return nil
	}
	reg, err := store.New(s.d.DB.Conn(ctx)).PayUniRegistrationByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if reg.BindStatus != string(model.BindBound) {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	st, err := s.d.Client.MerchantStatus(callCtx, reg.MerID)
	if apiErr, isAPI := pu.AsAPIError(err); isAPI {
		s.d.Logger.WarnContext(ctx, "seller：PayUni 回了業務失敗，不套用", "store_id", storeID, "code", apiErr.Code)
		return s.touch(ctx, storeID)
	}
	if err != nil {
		return err
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := s.record(ctx, q, storeID, fromStatus(st, source, s.d.Now().UTC())); err != nil {
			return err
		}
		checked := s.d.Now().UTC()
		return q.TouchPayUniChecked(ctx, store.TouchPayUniCheckedParams{StoreID: storeID, Now: &checked})
	})
}

func (s *Service) touch(ctx context.Context, storeID int64) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		checked := s.d.Now().UTC()
		return store.New(s.d.DB.Conn(ctx)).TouchPayUniChecked(ctx, store.TouchPayUniCheckedParams{StoreID: storeID, Now: &checked})
	})
}

// Refresh 是管理員手動刷新：排入同一個輪詢 job（以 store_id 為唯一鍵，已經有排隊的就合併），套用走同一個函式。
// 沒有設定 PayUni、店家還沒有註冊或還沒綁定時回 FailedPrecondition。
func (s *Service) Refresh(ctx context.Context, storeID int64) error {
	if err := model.Positive(storeID); err != nil {
		return err
	}
	if s.d.Client == nil {
		return apperr.FailedPrecondition("尚未設定 PayUni")
	}
	reg, err := s.Registration(ctx, storeID)
	if err != nil {
		return err
	}
	if reg.BindStatus != model.BindBound {
		return apperr.FailedPrecondition("PayUni 註冊還沒有綁定")
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return enqueuePoll(ctx, s.d.Jobs, storeID, model.SourceRefresh)
	})
}

// Sweep 是定期的掃描（保底，平常 webhook 與 sync job 已經夠用）：補排有資料沒送出又閒置超過一段時間的註冊（job 遺失、
// 被丟棄或寫入結果與 River 完成之間的空檔），以及排輪詢超過間隔沒問過、PayUni 還沒有定論的註冊。回傳補排的數量。
func (s *Service) Sweep(ctx context.Context) (resynced, polled int, err error) {
	now := s.d.Now().UTC()
	q := store.New(s.d.DB.Conn(ctx))
	ids, err := q.PayUniRegistrationsToResync(ctx, store.PayUniRegistrationsToResyncParams{
		IdleBefore: now.Add(-resyncIdle), BatchSize: sweepBatch,
	})
	if err != nil {
		return 0, 0, err
	}
	for _, id := range ids {
		if err := s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueueSync(ctx, s.d.Jobs, id) }); err != nil {
			return resynced, polled, err
		}
		resynced++
	}
	if s.d.Client == nil {
		return resynced, 0, nil
	}
	before := now.Add(-pollInterval)
	ids, err = q.PayUniRegistrationsToPoll(ctx, store.PayUniRegistrationsToPollParams{CheckedBefore: &before, ResubmittedAfter: now.Add(-resubmitPollWindow), BatchSize: sweepBatch})
	if err != nil {
		return resynced, 0, err
	}
	for _, id := range ids {
		if err := s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueuePoll(ctx, s.d.Jobs, id, model.SourcePoll) }); err != nil {
			return resynced, polled, err
		}
		polled++
	}
	return resynced, polled, nil
}
