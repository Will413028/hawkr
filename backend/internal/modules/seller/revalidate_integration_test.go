//go:build integration

package seller_test

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// revalidations 是目前為止為這家店排入的 website 快取失效數（店家頁的 tag 是 seller:<store_code>；queue 的替身要求排入發生在
// transaction 內）。
func (f *fixture) revalidations(s Store) int {
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	n := 0
	for _, a := range f.jobs.args {
		if r, ok := a.(revalidate.Args); ok && slices.Contains(r.Tags, "seller:"+strconv.FormatInt(s.StoreCode, 10)) {
			n++
		}
	}
	return n
}

// 店面頁（GetStore）與卡片會隨「可公開曝光」的任何一個輸入改變：每個輸入的寫入者都在同一個 transaction 內排入快取失效。
// PayUni 的審核狀態只在 record 寫入、綁定結果只在 finish 寫入。
func TestPayUniStatusAndBindChangesInvalidateTheStorePage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s, sub := f.submitted(t, idA, bankAcct)
	must(t, approve(f, sub))
	before := f.revalidations(s)
	f.sync(t, s.StoreID) // 綁定完成：流程從 BINDING 往前
	if got := f.revalidations(s); got <= before {
		t.Fatalf("綁定結果沒有排入快取失效：%d → %d", before, got)
	}
	before = f.revalidations(s)
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	if got := f.revalidations(s); got != before+1 {
		t.Fatalf("PayUni 核准沒有排入快取失效：%d → %d", before, got)
	}
	before = f.revalidations(s)
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourceRefresh))
	if got := f.revalidations(s); got != before+1 {
		t.Fatalf("PayUni 拒絕沒有排入快取失效：%d → %d", before, got)
	}
}

// KYC 申請的狀態是曝光的輸入：送出、取消、核准、退回各排入一次。
func TestKYCStatusChangesInvalidateTheStorePage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.open(t)
	d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
	before := f.revalidations(s)
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	if got := f.revalidations(s); got <= before {
		t.Fatalf("送出沒有排入快取失效：%d → %d", before, got)
	}
	before = f.revalidations(s)
	must(t, approve(f, sub))
	if got := f.revalidations(s); got <= before {
		t.Fatalf("核准沒有排入快取失效：%d → %d", before, got)
	}
	accountID2, s2, sub2 := f.submitted(t, idA, bankAcct)
	_ = accountID2
	before = f.revalidations(s2)
	_, err = f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9, Issues: anIssue(), Reason: "證件模糊"})
	must(t, err)
	if got := f.revalidations(s2); got <= before {
		t.Fatalf("退回沒有排入快取失效：%d → %d", before, got)
	}
}

// 兩段清理都改變店家頁與卡片（退場清掉說明與圖片，個資清理清掉店名）：各自在自己的 transaction 內排入快取失效。
func TestPurgeInvalidatesTheStorePage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.exposable(t)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	closedAt := f.now
	before := f.revalidations(s)
	f.now = closedAt.Add(StoreRetireAfter + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 || !f.retired(t, s.StoreID) {
		t.Fatalf("退場：purged=%d err=%v", n, err)
	}
	if got := f.revalidations(s); got != before+1 {
		t.Fatalf("退場沒有排入快取失效：%d → %d", before, got)
	}
	f.now = closedAt.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("purged=%d err=%v", n, err)
	}
	if got := f.revalidations(s); got != before+2 {
		t.Fatalf("個資清理沒有排入快取失效：%d → %d", before, got)
	}
}
