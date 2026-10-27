package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// issue #2294 L-GO：seller_deleted_outbox 的多副本搶佔閘門。
//
// 修復前這支 worker 完全沒有閘門：FetchPendingDue 的 SQL 帶著
// FOR UPDATE SKIP LOCKED，但 RunOnce 把 ticker 的裸 ctx 直接傳下去、整個
// seller-service 對這條路徑一次 WithTx 都沒有 ⇒ 自動提交下 SELECT 一回傳鎖就釋放
// ⇒ 兩個副本拿到同一批列，各發一次 seller.deleted **且各打一次
// account-service RemoveSellerRole**（規範 §一 ②）。
//
// ⚠️ event_published_at / role_removed_at 不是閘門，是**斷點續傳的進度記號**：
// 它們只在同一個副本的下一輪生效，兩個副本同時跑時兩邊都還是 nil。

// 規則 ①：搶佔在副作用之前；claim 回 0 ⇒ publish 與 RemoveSellerRole 都不做。
//
// ⚠️ 鑑別點：把 processRow 裡的 claim 整段拿掉，本測試立刻轉紅。
func TestSellerDeletedOutboxWorker_ClaimLost_DoesNoSideEffect(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending:   []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
		ClaimLost: true,
	}
	pub := &fakeSellerDeletedPublisher{}
	rm := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, rm, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(repo.ClaimCalls) != 1 {
		t.Fatalf("ClaimCalls = %+v, want exactly one claim attempt", repo.ClaimCalls)
	}
	if len(pub.DeletedEvents) != 0 {
		t.Fatalf("claim 回 0（另一個副本已認領）卻仍 publish 了 %+v", pub.DeletedEvents)
	}
	if len(rm.calledWith) != 0 {
		t.Fatalf("claim 回 0 卻仍呼叫 RemoveSellerRole %+v", rm.calledWith)
	}
	if len(repo.MarkDoneCalls) != 0 || len(repo.MarkEventPublishedIDs) != 0 ||
		len(repo.MarkRoleRemovedIDs) != 0 || len(repo.UpdateAttemptCalls) != 0 {
		t.Fatalf("claim 回 0 不得有任何後續寫入：done=%+v published=%+v roleRemoved=%+v attempt=%+v",
			repo.MarkDoneCalls, repo.MarkEventPublishedIDs, repo.MarkRoleRemovedIDs, repo.UpdateAttemptCalls)
	}
}

// 已 publish 過、只剩 RemoveSellerRole 的續傳列，同樣要先過閘門。
// 這一支釘住「閘門必須蓋住整列的處理」——放在 event_published_at 分支之內
// 會讓續傳路徑完全沒有跨副本互斥，兩個副本各打一次 RemoveSellerRole。
func TestSellerDeletedOutboxWorker_ClaimLost_AlsoGuardsRoleRemovalResume(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending:   []*domain.SellerDeletedOutbox{newEventPublishedRow(2, 2002, 0)},
		ClaimLost: true,
	}
	pub := &fakeSellerDeletedPublisher{}
	rm := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, rm, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.ClaimCalls) != 1 {
		t.Fatalf("續傳列也必須先 claim，實測 ClaimCalls=%+v", repo.ClaimCalls)
	}
	if len(rm.calledWith) != 0 {
		t.Fatalf("claim 回 0 卻仍呼叫 RemoveSellerRole %+v", rm.calledWith)
	}
}

// claim 本身失敗（DB 不可用）也一樣不做副作用——該列留在候選集合，下一 tick 重來。
func TestSellerDeletedOutboxWorker_ClaimError_DoesNoSideEffect(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending:  []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
		ClaimErr: errors.New("connection refused"),
	}
	pub := &fakeSellerDeletedPublisher{}
	rm := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, rm, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.DeletedEvents) != 0 || len(rm.calledWith) != 0 {
		t.Fatalf("claim 失敗卻仍做了副作用：events=%+v role=%+v", pub.DeletedEvents, rm.calledWith)
	}
}

// 規則 ③：寫進去的是**未來時間**（lease），不是永久旗標。
func TestSellerDeletedOutboxWorker_ClaimWritesFutureLease(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{newPendingSellerRow(1, 1001, 0)},
	}
	pub := &fakeSellerDeletedPublisher{}
	rm := &fakeSellerRoleRemover{}

	before := time.Now()
	w := NewSellerDeletedOutboxWorker(repo, pub, rm, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	after := time.Now()

	if len(repo.ClaimCalls) != 1 {
		t.Fatalf("ClaimCalls = %+v, want exactly one", repo.ClaimCalls)
	}
	c := repo.ClaimCalls[0]
	if !c.LeaseUntil.After(before.Add(sdoClaimLease-time.Second)) ||
		c.LeaseUntil.After(after.Add(sdoClaimLease)) {
		t.Fatalf("租約 %v 不在 [before+lease, after+lease] 內（lease=%v）——"+
			"寫 NULL／過去／旗標值都會落在這個區間外", c.LeaseUntil, sdoClaimLease)
	}
	if !c.ClaimableBefore.Before(c.LeaseUntil) {
		t.Fatalf("CAS 上界 %v 必須早於租約 %v", c.ClaimableBefore, c.LeaseUntil)
	}
}

// 規範 §三 第 6 點：租約基準必須在 claim 當下取，不能在批次開始時取一次。
//
// 本支的單筆工作含**兩次 gRPC 往返**（HasActiveBuyer 3s ＋ RemoveSellerRole
// sdoRoleRemovalTimeout 10s）＋一次 publish，sdoFetchBatch=50
// ⇒ 批次上界 ≈ 11 分鐘 ≫ 3 分鐘租約，判定式（batchSize × 單筆最壞耗時 > 租約）**成立**
// ⇒ 共用 tick 起點時這個 bug 一定會發生，不是機率問題。
func TestSellerDeletedOutboxWorker_LeaseBaselineIsPerItemNotPerTick(t *testing.T) {
	repo := &fakeSellerDeletedOutboxRepo{
		pending: []*domain.SellerDeletedOutbox{
			newPendingSellerRow(1, 1001, 0),
			newPendingSellerRow(2, 1002, 0),
		},
	}
	pub := &fakeSellerDeletedPublisher{delayFirst: 40 * time.Millisecond}
	rm := &fakeSellerRoleRemover{}

	w := NewSellerDeletedOutboxWorker(repo, pub, rm, &fakeSellerRoleGranter{}, &fakeCurrentStint{}, &fakeWorkerBuyerClient{}, testOutboxMetrics)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(repo.ClaimCalls) != 2 {
		t.Fatalf("ClaimCalls = %+v, want two", repo.ClaimCalls)
	}
	gap := repo.ClaimCalls[1].LeaseUntil.Sub(repo.ClaimCalls[0].LeaseUntil)
	if gap < 20*time.Millisecond {
		t.Fatalf("第二筆的租約只比第一筆晚 %v。兩者相等 ⇒ 租約基準是 tick 起點 ⇒ "+
			"排在後面的項目在 claim 當下就已經吃掉前面所有項目的耗時，批次夠長時租約直接是過去時間。", gap)
	}
	if !repo.ClaimCalls[0].ClaimableBefore.Equal(repo.ClaimCalls[1].ClaimableBefore) {
		t.Fatalf("CAS 上界應共用 tick 起點，實測 %v vs %v",
			repo.ClaimCalls[0].ClaimableBefore, repo.ClaimCalls[1].ClaimableBefore)
	}
}
