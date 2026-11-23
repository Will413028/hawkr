//go:build integration

package quote_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// noAcceptanceOrJob 檢查失敗的接受沒有留下任何東西：沒有接受單、沒有 job。
func (f *fixture) noAcceptanceOrJob() {
	f.t.Helper()
	if n, j := f.count("procurement.buyer_quote_acceptances"), len(f.provisioningJobs()); n != 0 || j != 0 {
		f.t.Fatalf("失敗的接受不該留下東西：接受單 %d、job %d", n, j)
	}
}

func TestAcceptFreezesThePriceAndEnqueuesTheOrderJobInTheSameTransaction(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	in := f.acceptIn(r, 1)
	a, err := f.service().Accept(t.Context(), in)
	must(t, err)
	if a.Replayed || a.SellerResponseID != r || a.Version != 1 || a.AcceptedDealPrice != 415 || !a.AcceptedAt.Equal(start) || a.Provisioning != model.ProvisioningCreating || a.OrderNo != nil || a.PaymentDeadline != nil {
		t.Fatalf("回應 = %+v", a)
	}
	var goods, shipping, tax, price int64
	var state, sellerHash, payloadHash string
	var storeID int64
	must(t, f.d.Pool().QueryRow(t.Context(), `SELECT goods_amount, shipping_amount, tax_amount, accepted_deal_price, state, seller_facing_hash, seller_response_payload_hash, store_id
		FROM procurement.buyer_quote_acceptances WHERE id = $1`, a.ID).Scan(&goods, &shipping, &tax, &price, &state, &sellerHash, &payloadHash, &storeID))
	if goods != 400 || shipping != 10 || tax != 5 || price != 415 || state != "ACTIVE" || sellerHash != f.hash || len(payloadHash) != 64 || storeID != 11 {
		t.Fatalf("接受單 = %d %d %d %d %s %s %s %d", goods, shipping, tax, price, state, sellerHash, payloadHash, storeID)
	}
	if jobs := f.provisioningJobs(); len(jobs) != 1 || jobs[0] != a.ID.String() {
		t.Fatalf("要恰好一個以接受單 ID 為參數的建單 job：%v", jobs)
	}
}

func TestAcceptReplayReturnsTheFirstAcceptanceWithoutANewRowOrJob(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	in := f.acceptIn(r, 1)
	first, err := f.service().Accept(t.Context(), in)
	must(t, err)
	f.exec(`UPDATE procurement.buyer_quote_acceptances SET order_provisioning_state = 'ORDER_CREATED', order_no = 'HK1', payment_deadline = $2 WHERE id = $1`, first.ID, start.Add(time.Hour))

	f.now = start.Add(48 * time.Hour) // 重送時版本還有效與否都不影響回放（回放先於其餘檢查）
	again, err := f.service().Accept(t.Context(), in)
	must(t, err)
	if !again.Replayed || again.ID != first.ID || again.Provisioning != model.ProvisioningOrderCreated || again.OrderNo == nil || *again.OrderNo != "HK1" || again.PaymentDeadline == nil {
		t.Fatalf("重送 = %+v", again)
	}
	if f.count("procurement.buyer_quote_acceptances") != 1 || len(f.provisioningJobs()) != 1 {
		t.Fatal("重送不該多一筆接受單或 job")
	}
}

func TestAcceptReplayWithDifferentContentIsRejected(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400}, version{goods: 410})
	g := f.respond(f.publication, 12, version{goods: 400})
	in := f.acceptIn(r, 2)
	_, err := f.service().Accept(t.Context(), in)
	must(t, err)
	for _, edit := range map[string]func(*quote.AcceptInput){
		"版本":   func(i *quote.AcceptInput) { i.ExpectedVersion = 1 },
		"回價":   func(i *quote.AcceptInput) { i.ResponseID = g },
		"hash": func(i *quote.AcceptInput) { i.ExpectedSellerFacingHash = "other" },
		"需求":   func(i *quote.AcceptInput) { i.RequestID = uuid.NewV7() },
	} {
		changed := in
		edit(&changed)
		_, err := f.service().Accept(t.Context(), changed)
		wantInvalid(t, err, "client_request_id")
	}
	if f.count("procurement.buyer_quote_acceptances") != 1 || len(f.provisioningJobs()) != 1 {
		t.Fatal("內容不同的重送不該寫入")
	}
}

func TestAcceptRejectsAnExpiredVersion(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400, validUntil: start.Add(-time.Second)})
	_, err := f.service().Accept(t.Context(), f.acceptIn(r, 1))
	_ = wantPrecondition(t, err, apperr.ReasonRfqResponseExpired)
	f.noAcceptanceOrJob()
	// 剛好在有效期限那一刻仍可接受（過了才算過期）。
	on := f.respond(f.publication, 12, version{goods: 400, validUntil: start})
	_, err = f.service().Accept(t.Context(), f.acceptIn(on, 1))
	must(t, err)
}

func TestAcceptRejectsAWithdrawnVersion(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	f.exec(`UPDATE procurement.seller_responses SET withdrawn_version = 1, withdrawn_at = $2 WHERE id = $1`, r, start)
	_, err := f.service().Accept(t.Context(), f.acceptIn(r, 1))
	wantConflict(t, err, quote.TargetSellerResponse)
	f.noAcceptanceOrJob()
}

func TestAcceptRejectsAVersionThatIsNoLongerTheLatest(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400}, version{goods: 380})
	_, err := f.service().Accept(t.Context(), f.acceptIn(r, 1))
	wantConflict(t, err, quote.TargetSellerResponse)
	f.noAcceptanceOrJob()
	// 期望的版本比目前的新也一樣（買家的畫面不可能領先伺服器）。
	_, err = f.service().Accept(t.Context(), f.acceptIn(r, 3))
	wantConflict(t, err, quote.TargetSellerResponse)
	f.noAcceptanceOrJob()
}

func TestAcceptRejectsWhenTheSellerStoppedWriting(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	f.exec(`UPDATE procurement.seller_responses SET write_closed_at = $2, write_closed_reason = 'DECLINED' WHERE id = $1`, r, start)
	_, err := f.service().Accept(t.Context(), f.acceptIn(r, 1))
	_ = wantPrecondition(t, err, apperr.ReasonRfqRecipientClosed)
	f.noAcceptanceOrJob()
}

func TestAcceptComparesTheHashRecomputedFromTheFrozenPublication(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	for name, hash := range map[string]string{"別的值": "0000", "大寫": strings.ToUpper(f.hash), "前後多空白": " " + f.hash} {
		in := f.acceptIn(r, 1)
		in.ExpectedSellerFacingHash = hash
		_, err := f.service().Accept(t.Context(), in)
		if apperr.KindOf(err) != apperr.KindVersionConflict {
			t.Fatalf("%s：%v", name, err)
		}
		wantConflict(t, err, quote.TargetPublication)
	}
	f.noAcceptanceOrJob()

	// 需求改版後重新發布：買家在舊分頁帶著新發布的 hash 接受舊發布收到的報價，不符；需求改版本身不影響舊發布的 hash（由核准凍結的需求版本重算）。
	f.revise()
	_, newHash := f.publish()
	if newHash == f.hash {
		t.Fatal("需求內容不同，兩次發布的 hash 要不同")
	}
	in := f.acceptIn(r, 1)
	in.ExpectedSellerFacingHash = newHash
	_, err := f.service().Accept(t.Context(), in)
	wantConflict(t, err, quote.TargetPublication)
	f.noAcceptanceOrJob()
	a, err := f.service().Accept(t.Context(), f.acceptIn(r, 1))
	must(t, err)
	if a.AcceptedDealPrice != 415 {
		t.Fatalf("%+v", a)
	}
}

func TestAcceptLiveLockCollisionReportsTheCheckoutAndItsOrder(t *testing.T) {
	f := newFixture(t)
	a := f.respond(f.publication, 11, version{goods: 400})
	b := f.respond(f.publication, 12, version{goods: 380})
	first, err := f.service().Accept(t.Context(), f.acceptIn(a, 1))
	must(t, err)

	_, err = f.service().Accept(t.Context(), f.acceptIn(b, 1))
	ae := wantPrecondition(t, err, apperr.ReasonRfqCheckoutAlreadyActive)
	if _, ok := ae.Metadata["orderNo"]; ok {
		t.Fatalf("訂單還沒建好時沒有 orderNo：%v", ae.Metadata)
	}
	f.exec(`UPDATE procurement.buyer_quote_acceptances SET order_provisioning_state = 'ORDER_CREATED', order_no = 'HK9' WHERE id = $1`, first.ID)
	_, err = f.service().Accept(t.Context(), f.acceptIn(b, 1))
	ae = wantPrecondition(t, err, apperr.ReasonRfqCheckoutAlreadyActive)
	if ae.Metadata["orderNo"] != "HK9" {
		t.Fatalf("orderNo = %v", ae.Metadata)
	}
	if f.count("procurement.buyer_quote_acceptances") != 1 || len(f.provisioningJobs()) != 1 {
		t.Fatal("撞 live 鎖的接受不該留下接受單或 job")
	}

	// 建單重試用盡（ORDER_FAILED）解除鎖，買家可以再接受。
	f.exec(`UPDATE procurement.buyer_quote_acceptances SET state = 'ORDER_FAILED', order_provisioning_state = 'ORDER_FAILED', order_no = NULL WHERE id = $1`, first.ID)
	second, err := f.service().Accept(t.Context(), f.acceptIn(b, 1))
	must(t, err)
	if second.ID == first.ID || len(f.provisioningJobs()) != 2 {
		t.Fatalf("%+v %v", second, f.provisioningJobs())
	}
}

func TestAcceptChecksOwnership(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	g := f.sibling(2)
	theirs := g.respond(g.publication, 21, version{goods: 400})

	in := f.acceptIn(r, 1)
	in.BuyerID = 99
	_, err := f.service().Accept(t.Context(), in)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().Accept(t.Context(), f.acceptIn(theirs, 1))
	wantKind(t, err, apperr.KindNotFound)
	in = f.acceptIn(r, 1)
	in.RequestID = g.request
	_, err = f.service().Accept(t.Context(), in)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.service().Accept(t.Context(), f.acceptIn(uuid.NewV7(), 1))
	wantKind(t, err, apperr.KindNotFound)
	f.noAcceptanceOrJob()
}

func TestAcceptValidatesRequiredInputsBeforeTouchingTheDatabase(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	for _, v := range []int32{0, -1} {
		_, err := f.service().Accept(t.Context(), f.acceptIn(r, v))
		wantInvalid(t, err, "expected_seller_response_version")
	}
	in := f.acceptIn(r, 1)
	in.ExpectedSellerFacingHash = ""
	_, err := f.service().Accept(t.Context(), in)
	wantInvalid(t, err, "expected_seller_facing_hash")
	f.noAcceptanceOrJob()
}

type failingJobs struct{}

func (failingJobs) EnqueueJob(context.Context, river.JobArgs, *river.InsertOpts) (int64, error) {
	return 0, context.DeadlineExceeded
}

func (failingJobs) CancelJob(context.Context, int64) (rivertype.JobState, bool, error) {
	return "", false, nil
}

func TestAcceptRollsBackTheAcceptanceWhenTheJobCannotBeEnqueued(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	svc := f.service()
	svc.Jobs = failingJobs{}
	_, err := svc.Accept(t.Context(), f.acceptIn(r, 1))
	if err == nil {
		t.Fatal("job 排不進去要回錯誤")
	}
	f.noAcceptanceOrJob()
	// 同一張需求之後仍能接受（沒有留下 live 鎖）。
	if _, err := f.service().Accept(t.Context(), f.acceptIn(r, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptRequiresTheJobQueue(t *testing.T) {
	f := newFixture(t)
	r := f.respond(f.publication, 11, version{goods: 400})
	svc := f.service()
	svc.Jobs = nil
	if _, err := svc.Accept(t.Context(), f.acceptIn(r, 1)); err == nil {
		t.Fatal("沒有 job 佇列是組裝錯誤")
	}
	f.noAcceptanceOrJob()
}
