//go:build integration

package buyer_test

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	httpcatalog "github.com/Will413028/hawkr/backend/internal/httpapi/catalog"
	"github.com/Will413028/hawkr/backend/internal/httpapi/catalog/catalogtest"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type feedModule struct {
	httpbuyer.Module
	account, buyerID int64
}

func (m feedModule) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	if accountID != m.account {
		return 0, connect.NewError(connect.CodeNotFound, nil)
	}
	return m.buyerID, nil
}

type feedCards struct{}

func (feedCards) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		name := "店" + strconv.FormatInt(id, 10)
		out[i] = seller.StoreCard{StoreID: id, StoreCode: id, Name: &name, Status: seller.CardOpen}
	}
	return out, nil
}

type nothing struct {
	blocks.Authors
	blocks.Reviews
	blocks.Messages
	blocks.BuyerAccounts
	blocks.StoreReader
}

// 追蹤中的動態只含呼叫者追蹤的店家的公開商品：另一家沒追蹤的店、追蹤後變成不可曝光的店、草稿與下架的商品都不在；
// 真的 buyer 模組讀追蹤關係、真的 catalog 模組依店家清單查商品，店家（7000 起）、買家（5000 起）與帳號（1001 起）的號碼互不相同。
func TestListFollowedStoreProductsReturnsOnlyFollowedStoresProducts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := catalogtest.New(t, pg)
	followed, followedAcct := w.Store()
	suspended, suspendedAcct := w.Store()
	other, otherAcct := w.Store()
	mine := []uuid.UUID{
		w.Active(followed, followedAcct, w.Input("追蹤店 1", 2)).ID,
		w.Active(followed, followedAcct, w.Input("追蹤店 2", 2)).ID,
		w.Active(followed, followedAcct, w.Input("追蹤店 3", 2)).ID,
	}
	w.Create(followed, followedAcct, w.Input("追蹤店的草稿", 2))
	archived := w.Active(followed, followedAcct, w.Input("追蹤店的下架品", 2))
	w.SetStatus(followed, archived.ID, "ARCHIVED")
	hiddenProduct := w.Active(suspended, suspendedAcct, w.Input("停權店的商品", 2))
	notFollowed := w.Active(other, otherAcct, w.Input("沒追蹤的店", 2))
	w.SetExposable(suspended, false)

	w.Exec(`ALTER TABLE buyer.buyers ALTER COLUMN buyer_id RESTART WITH 5000`)
	const account = 42
	var buyerID int64
	if err := w.DB.Pool().QueryRow(ctx, `INSERT INTO buyer.buyers (account_id, display_name, created_at, updated_at) VALUES ($1, '買家', $2, $2) RETURNING buyer_id`, account, w.Now()).Scan(&buyerID); err != nil {
		t.Fatal(err)
	}
	w.Exec(`INSERT INTO buyer.seller_subscriptions (buyer_id, store_id, created_at) VALUES ($1, $2, $4), ($1, $3, $4)`, buyerID, followed, suspended, w.Now())

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	presenter, err := httpcatalog.NewPresenter(feedCards{}, characterize.Stats(t), characterize.Images(t))
	if err != nil {
		t.Fatal(err)
	}
	ports := httpbuyer.Ports{Followed: httpbuyer.FollowedProducts{
		Follows: buyer.New(buyer.Deps{DB: w.DB, Now: time.Now, Logger: log}), Catalog: w.Module, Cards: presenter, Now: w.Now,
	}}
	var u nothing
	app, err := httpbuyer.NewApp(feedModule{account: account, buyerID: buyerID}, characterize.Images(t), ports, characterize.Stats(t),
		blocks.Sources{Authors: u, Reviews: u, Messages: u, Identities: blocks.Identities{Buyers: u, Stores: u}})
	if err != nil {
		t.Fatal(err)
	}
	signed := rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAccount, ID: account})

	var got []string
	token := ""
	for range 10 {
		r, err := app.ListFollowedStoreProducts(signed, connect.NewRequest(&pb.ListFollowedStoreProductsRequest{PageSize: 2, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range r.Msg.GetProducts() {
			got = append(got, p.GetId())
			if p.GetStore().GetId() != strconv.FormatInt(followed, 10) {
				t.Fatalf("商品 %s 的店家 = %v，預期追蹤的店 %d", p.GetId(), p.GetStore().GetId(), followed)
			}
		}
		if token = r.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	if len(got) != len(mine) {
		t.Fatalf("動態 = %v，預期剛好是追蹤店的 %d 個上架商品（翻頁不重不漏）", got, len(mine))
	}
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range mine {
		if !seen[id.String()] {
			t.Fatalf("漏掉追蹤店的商品 %s：%v", id, got)
		}
	}
	for _, id := range []uuid.UUID{notFollowed.ID, hiddenProduct.ID, archived.ID} {
		if seen[id.String()] {
			t.Fatalf("不該出現的商品 %s：%v", id, got)
		}
	}
	w.Exec(`DELETE FROM buyer.seller_subscriptions`)
	empty, err := app.ListFollowedStoreProducts(signed, connect.NewRequest(&pb.ListFollowedStoreProductsRequest{}))
	if err != nil || len(empty.Msg.GetProducts()) != 0 || empty.Msg.GetNextPageToken() != "" {
		t.Fatalf("沒有追蹤 = %v, %v", empty, err)
	}
}
