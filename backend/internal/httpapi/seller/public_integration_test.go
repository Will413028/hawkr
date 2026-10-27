//go:build integration

package seller_test

import (
	"context"
	"strconv"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func mustAgreementURL(t testing.TB, ref string) string {
	t.Helper()
	u, err := characterize.WebOrigin(t).URL(ref)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// 還沒通過身分認證的店家：店面頁不公開（不存在與不公開同一個 not_found，store_id 也不是鍵），卡片也不回。
func TestPublicReadsHideAStoreThatIsNotExposable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	opened := f.apply(t)
	_, err := f.public.GetStore(ctx, connect.NewRequest(&pb.GetStoreRequest{StoreCode: opened.GetStoreCode()}))
	known := wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.public.GetStore(ctx, connect.NewRequest(&pb.GetStoreRequest{StoreCode: opened.GetId()}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.public.GetStore(ctx, connect.NewRequest(&pb.GetStoreRequest{StoreCode: "999999999"}))
	if unknown := wantCode(t, err, connect.CodeNotFound, ""); unknown != known {
		t.Fatalf("不公開（%q）與不存在（%q）的錯誤不同", known, unknown)
	}
	_, err = f.public.GetStore(ctx, connect.NewRequest(&pb.GetStoreRequest{StoreCode: "not-a-code"}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "store_code" {
		t.Fatalf("field=%q", field)
	}
	cards, err := f.public.BatchGetStores(ctx, connect.NewRequest(&pb.BatchGetStoresRequest{Ids: []string{opened.GetId(), "999999"}}))
	if err != nil || len(cards.Msg.GetStores()) != 0 {
		t.Fatalf("從沒通過認證的店家不該有卡片：%v err=%v", cards, err)
	}
}

// 商品瀏覽的打點只計訪客：登入的呼叫者收到成功但不寫入事件。
func TestRecordProductViewWritesAnEventOnlyForGuests(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	events := func() int64 { return f.dbCount(t, "SELECT count(*) FROM seller.product_view_events") }
	if _, err := f.public.RecordProductView(ctx, connect.NewRequest(&pb.RecordProductViewRequest{ProductId: strconv.Itoa(42)})); err != nil {
		t.Fatal(err)
	}
	if got := events(); got != 1 {
		t.Fatalf("訪客的打點事件=%d，預期 1", got)
	}
	if _, err := f.signedIn.RecordProductView(ctx, connect.NewRequest(&pb.RecordProductViewRequest{ProductId: "42"})); err != nil {
		t.Fatal(err)
	}
	if got := events(); got != 1 {
		t.Fatalf("登入者不該計入：事件=%d", got)
	}
}

// 入駐文件的現行版本不需要登入就讀得到（開店前就要讀），登入與否內容相同；三份依契約的順序，值就是模組常數表的現行值。
func TestGetSellerAgreementsNeedsNoLogin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	anon, err := f.public.GetSellerAgreements(ctx, connect.NewRequest(&pb.GetSellerAgreementsRequest{}))
	if err != nil {
		t.Fatalf("未登入：%v", err)
	}
	signed, err := f.signedIn.GetSellerAgreements(ctx, connect.NewRequest(&pb.GetSellerAgreementsRequest{}))
	if err != nil {
		t.Fatalf("已登入：%v", err)
	}
	if !proto.Equal(anon.Msg, signed.Msg) {
		t.Fatalf("登入與否不同：%v / %v", anon.Msg, signed.Msg)
	}
	got := anon.Msg.GetAgreements()
	if len(got) != len(model.AgreementKinds()) {
		t.Fatalf("agreements=%v", got)
	}
	for i, k := range model.AgreementKinds() {
		cur, _ := model.CurrentAgreement(k)
		a := got[i]
		if a.GetKind() != rpc.EnumToProto[pb.AgreementKind](k) || a.GetVersion() != cur.Version || a.GetUrl() != mustAgreementURL(t, cur.URL) {
			t.Errorf("[%d] %v 與常數表 %s=%+v 不一致", i, a, k, cur)
		}
	}
}
