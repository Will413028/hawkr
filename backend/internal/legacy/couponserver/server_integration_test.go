//go:build integration

package couponserver

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var base = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

const (
	adminID = int64(7)
	storeA  = int64(501)
	buyer1  = int64(1001)
	buyer2  = int64(1002)
)

type openStores struct{}

func (openStores) StoreSuspended(context.Context, int64) (bool, error) { return false, nil }

type noMembers struct{}

func (noMembers) IsMember(context.Context, int64, int64) (bool, error) { return false, nil }

type fixture struct {
	t   *testing.T
	db  *db.DB
	m   *coupon.Module
	srv *Server
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "coupon", Dir: coupon.Migrations()}))
	m := coupon.New(coupon.Deps{DB: d, Stores: openStores{}, Members: noMembers{}, NewID: uuid.NewV7, Now: func() time.Time { return base }})
	return &fixture{t: t, db: d, m: m, srv: New(m, slog.New(slog.NewTextHandler(io.Discard, nil)))}
}

// campaign 建立並發布 admin 的全平台現金折抵 100 的活動；threshold 是門檻。
func (f *fixture) campaign(threshold int64) coupon.Campaign {
	f.t.Helper()
	c, err := f.m.CreateCampaign(context.Background(), coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "活動", DiscountType: coupon.DiscountCash, DiscountAmount: 100, ThresholdAmount: threshold, StartAt: base.Add(-time.Hour), EndAt: base.Add(24 * time.Hour)},
		TargetType:   coupon.TargetAll, Issuer: coupon.Issuer{AdminID: adminID},
	})
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.m.PublishCampaign(context.Background(), c.ID); err != nil {
		f.t.Fatal(err)
	}
	return c
}

func (f *fixture) claim(c coupon.Campaign, buyer int64) coupon.Voucher {
	f.t.Helper()
	r, err := f.m.Claim(context.Background(), c.ID, buyer)
	if err != nil {
		f.t.Fatal(err)
	}
	return r.Voucher
}

func (f *fixture) lock(v coupon.Voucher, order string) {
	f.t.Helper()
	if _, err := f.m.Lock(context.Background(), coupon.LockInput{VoucherID: v.ID, BuyerID: v.BuyerID, OrderNo: order, StoreID: storeA, OrderAmount: 1000}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) status(id uuid.UUID) string {
	f.t.Helper()
	var s string
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT status FROM coupon.coupon_vouchers WHERE id = $1", id).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) count(sql string, args ...any) int64 {
	f.t.Helper()
	var n int64
	if err := f.db.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) validate(template string, buyer int64, store, amount string) *pb.ValidateCouponResponse {
	f.t.Helper()
	r, err := f.srv.ValidateCoupon(context.Background(), connect.NewRequest(&pb.ValidateCouponRequest{TemplateId: template, BuyerId: buyer, StoreId: store, OrderAmount: amount}))
	if err != nil {
		f.t.Fatalf("ValidateCoupon：%v", err)
	}
	return r.Msg
}

func (f *fixture) release(order, instance string) *pb.ReleaseCouponResponse {
	f.t.Helper()
	r, err := f.srv.ReleaseCoupon(context.Background(), connect.NewRequest(&pb.ReleaseCouponRequest{OrderNo: order, InstanceId: instance}))
	if err != nil {
		f.t.Fatalf("ReleaseCoupon：%v", err)
	}
	return r.Msg
}

func (f *fixture) consume(order, instance string) *pb.ConsumeCouponResponse {
	f.t.Helper()
	r, err := f.srv.ConsumeCoupon(context.Background(), connect.NewRequest(&pb.ConsumeCouponRequest{OrderNo: order, InstanceId: instance, BuyerId: buyer1}))
	if err != nil {
		f.t.Fatalf("ConsumeCoupon：%v", err)
	}
	return r.Msg
}

// 活動 id 對到買家持有的那一張券：領過的通過並回折抵；沒領過回 7011 且不發券；別的買家的券不會被借用。
func TestValidateMapsTemplateToTheBuyersVoucher(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(0)
	f.claim(c, buyer1)

	got := f.validate(c.ID.String(), buyer1, "501", "1000")
	if got.GetCode() != 0 || !got.GetData().GetValid() || got.GetData().GetDiscountAmount() != "100" || got.GetData().GetTemplateId() != c.ID.String() {
		t.Fatalf("領過的券要通過並折抵 100：%+v", got)
	}

	notClaimed := f.validate(c.ID.String(), buyer2, "501", "1000")
	if notClaimed.GetCode() != 7011 || notClaimed.GetData() != nil {
		t.Fatalf("沒領過要回 7011：%+v", notClaimed)
	}
	if n := f.count("SELECT count(*) FROM coupon.coupon_vouchers WHERE buyer_id = $1", buyer2); n != 0 {
		t.Fatalf("驗券不可以當場發券，買家 2 有 %d 張", n)
	}

	for name, req := range map[string][3]string{
		"template 不是 UUID": {"12345", "501", "1000"},
		"store 空":          {c.ID.String(), "", "1000"},
		"金額 0":             {c.ID.String(), "501", "0"},
	} {
		if r := f.validate(req[0], buyer1, req[1], req[2]); r.GetCode() != 400 {
			t.Fatalf("%s：code = %d，要 400", name, r.GetCode())
		}
	}
	// 不存在的活動：買家在該活動沒有券，同樣是未領取，不是通過。
	if r := f.validate(uuid.New().String(), buyer1, "501", "1000"); r.GetCode() != 7011 {
		t.Fatalf("不存在的活動 code = %d，要 7011", r.GetCode())
	}
}

// 門檻不足、被別單鎖住都是 valid=false 加原因（code 0），不是 gRPC 錯誤。
func TestValidateReportsUnusableVouchersAsInvalid(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(500)
	v := f.claim(c, buyer1)

	if r := f.validate(c.ID.String(), buyer1, "501", "100"); r.GetCode() != 0 || r.GetData().GetValid() || r.GetData().GetDescription() == "" {
		t.Fatalf("低於門檻要 valid=false 並說明原因：%+v", r)
	}
	f.lock(v, "ORDER-1")
	if r := f.validate(c.ID.String(), buyer1, "501", "1000"); r.GetCode() != 0 || r.GetData().GetValid() || r.GetData().GetDescription() == "" {
		t.Fatalf("被鎖住的券要 valid=false 並說明原因：%+v", r)
	}
	if got := f.status(v.ID); got != "LOCKED" {
		t.Fatalf("驗券不可以改狀態：%s", got)
	}
}

// 核銷以 order_no 為鍵：不帶 instance_id 也成功，重送冪等；沒有持有任何券回 success=false、7009。
func TestConsumeByOrderNoIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(0)
	v := f.claim(c, buyer1)
	f.lock(v, "ORDER-C")

	for range 2 {
		if r := f.consume("ORDER-C", ""); !r.GetSuccess() || r.GetCode() != 0 {
			t.Fatalf("核銷（含重送）要成功：%+v", r)
		}
	}
	if got := f.status(v.ID); got != "REDEEMED" {
		t.Fatalf("狀態 = %s，要 REDEEMED", got)
	}
	if r := f.consume("NO-SUCH-ORDER", ""); r.GetSuccess() || r.GetCode() != 7009 {
		t.Fatalf("沒有持有券的訂單要 7009：%+v", r)
	}
}

// 釋放以 order_no 為鍵：instance_id 省略或帶錯的值都釋放持有這筆訂單的那張券；回應遺失後不帶 instance_id 重送仍成功；
// 釋放後別的訂單可以鎖；沒有持有券的訂單（鎖券回應遺失、訂單沒記 instance id）也回成功。
func TestReleaseByOrderNoIsIdempotentWithoutInstanceID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	c := f.campaign(0)
	v := f.claim(c, buyer1)
	f.lock(v, "ORDER-R")

	if r := f.release("ORDER-R", uuid.New().String()); !r.GetSuccess() || r.GetCode() != 0 {
		t.Fatalf("以 order_no 釋放（instance_id 是別的值）要成功：%+v", r)
	}
	if got := f.status(v.ID); got != "AVAILABLE" {
		t.Fatalf("釋放後狀態 = %s，要 AVAILABLE", got)
	}
	// 回應遺失後的重送：沒有 instance_id。
	if r := f.release("ORDER-R", ""); !r.GetSuccess() {
		t.Fatalf("重送要成功：%+v", r)
	}
	f.lock(v, "ORDER-R2") // 釋放後別的訂單鎖得住
	if r := f.release("ORDER-R", ""); !r.GetSuccess() {
		t.Fatalf("舊訂單的第三次重送要成功：%+v", r)
	}
	if got := f.status(v.ID); got != "LOCKED" {
		t.Fatalf("舊訂單的重送不可以解開新訂單的鎖：%s", got)
	}
	// 回應遺失、訂單沒有 instance id 的情形：只有 order_no 也要釋得掉。
	v2 := f.claim(c, buyer2)
	f.lock(v2, "ORDER-R4")
	if r := f.release("ORDER-R4", ""); !r.GetSuccess() {
		t.Fatalf("沒有 instance_id 的釋放要成功：%+v", r)
	}
	if got := f.status(v2.ID); got != "AVAILABLE" {
		t.Fatalf("沒有 instance_id 的釋放沒有解鎖：%s", got)
	}
	if r := f.release("NEVER-LOCKED", ""); !r.GetSuccess() {
		t.Fatalf("沒有持有券的訂單要成功：%+v", r)
	}
	if r := f.release("", ""); r.GetCode() != 400 {
		t.Fatalf("空的 order_no 要 400：%+v", r)
	}
}

func (f *fixture) grant(source, buyer string) *pb.GrantSellerCompensationCouponResponse {
	f.t.Helper()
	r, err := f.srv.GrantSellerCompensationCoupon(context.Background(), connect.NewRequest(&pb.GrantSellerCompensationCouponRequest{
		StoreId: "501", BuyerId: buyer, Amount: "100", MinOrderAmount: "600", Message: "抱歉", RefCaseNo: "AS-1", SourceId: source,
	}))
	if err != nil {
		f.t.Fatalf("GrantSellerCompensationCoupon：%v", err)
	}
	return r.Msg
}

// 補償券以 source_id 去重：同一個售後單重送只建一張並回同一個 template_id；不同售後單各一張；缺 source_id 是 400；
// 同一個 source_id 給不同買家是 400，不會多發。
func TestCompensationDedupesBySourceID(t *testing.T) {
	t.Parallel()
	f := setup(t)

	first := f.grant("CASE-1", "1001")
	if first.GetCode() != 0 || first.GetData().GetTemplateId() == "" || first.GetData().GetAmount() != "100" || first.GetData().GetMinOrderAmount() != "600" {
		t.Fatalf("第一次：%+v", first)
	}
	again := f.grant("CASE-1", "1001")
	if again.GetCode() != 0 || again.GetData().GetTemplateId() != first.GetData().GetTemplateId() {
		t.Fatalf("重送要回同一張：%+v / %+v", first, again)
	}
	if n := f.count("SELECT count(*) FROM coupon.coupon_vouchers WHERE origin = 'COMPENSATION'"); n != 1 {
		t.Fatalf("同一個 source_id 重送後有 %d 張，要 1", n)
	}
	other := f.grant("CASE-2", "1001")
	if other.GetCode() != 0 || other.GetData().GetTemplateId() == first.GetData().GetTemplateId() {
		t.Fatalf("不同 source_id 要另發一張：%+v", other)
	}
	if n := f.count("SELECT count(*) FROM coupon.coupon_vouchers WHERE origin = 'COMPENSATION'"); n != 2 {
		t.Fatalf("兩個售後單後有 %d 張，要 2", n)
	}
	if r := f.grant("", "1001"); r.GetCode() != 400 {
		t.Fatalf("缺 source_id 要 400：%+v", r)
	}
	if r := f.grant("CASE-1", "1002"); r.GetCode() != 400 {
		t.Fatalf("同一個 source_id 給別的買家要 400：%+v", r)
	}
	if n := f.count("SELECT count(*) FROM coupon.coupon_vouchers WHERE origin = 'COMPENSATION'"); n != 2 {
		t.Fatalf("被拒的請求不可以發券：%d 張", n)
	}
}
