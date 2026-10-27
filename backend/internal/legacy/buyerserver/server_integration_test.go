//go:build integration

package buyerserver

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func TestEveryLegacyBuyerRPCReadsTheModule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "buyer", Dir: buyer.Migrations()}))
	queue, err := jobs.NewInsertOnly(db)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := buyer.New(buyer.Deps{DB: db, Events: bus, Jobs: queue, Now: time.Now, Logger: log})
	var accountID int64
	if err := db.Pool().QueryRow(ctx, "INSERT INTO identity.accounts (clerk_user_id) VALUES ($1) RETURNING id", uuid.NewV7().String()).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := b.OnAccountCreated(ctx, identity.AccountCreated{AccountID: accountID, Name: "買家甲"}); err != nil {
		t.Fatal(err)
	}
	p, err := b.Profile(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	a, err := b.AddAddress(ctx, buyer.AddressInput{BuyerID: p.BuyerID, ReceiverName: "王小明", ReceiverPhone: "0912345678", TownCode: "63000050", AddressDetail: "忠孝東路一段 1 號"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddFavorite(ctx, p.BuyerID, 11); err != nil {
		t.Fatal(err)
	}
	if err := b.Follow(ctx, p.BuyerID, 91); err != nil {
		t.Fatal(err)
	}
	if err := b.SubscribeRestock(ctx, p.BuyerID, 11); err != nil {
		t.Fatal(err)
	}
	s := New(b)
	t.Run("GetBuyerProfile", func(t *testing.T) {
		r, e := s.GetBuyerProfile(ctx, connect.NewRequest(&pb.GetBuyerProfileRequest{AccountId: accountID}))
		if e != nil || r.Msg.GetData().GetId() != strconv.FormatInt(p.BuyerID, 10) {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetBuyerProfileByBuyerID", func(t *testing.T) {
		r, e := s.GetBuyerProfileByBuyerID(ctx, connect.NewRequest(&pb.GetBuyerProfileByBuyerIDRequest{BuyerId: p.BuyerID}))
		if e != nil || r.Msg.GetData().GetAccountId() != strconv.FormatInt(accountID, 10) {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetAddressList", func(t *testing.T) {
		r, e := s.GetAddressList(ctx, connect.NewRequest(&pb.GetAddressListRequest{BuyerId: p.BuyerID}))
		if e != nil || len(r.Msg.GetData()) != 1 || r.Msg.GetData()[0].GetId() != strconv.FormatInt(a.AddressID, 10) {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetAddress", func(t *testing.T) {
		r, e := s.GetAddress(ctx, connect.NewRequest(&pb.GetAddressRequest{BuyerId: p.BuyerID, AddressId: a.AddressID}))
		if e != nil || r.Msg.GetData().GetReceiverName() != "王小明" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("ListFavoritedProductIds", func(t *testing.T) {
		r, e := s.ListFavoritedProductIds(ctx, connect.NewRequest(&pb.ListFavoritedProductIdsRequest{BuyerId: p.BuyerID}))
		if e != nil || len(r.Msg.GetProductIds()) != 1 || r.Msg.GetProductIds()[0] != 11 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("ListSubscribedSellerIds", func(t *testing.T) {
		r, e := s.ListSubscribedSellerIds(ctx, connect.NewRequest(&pb.ListSubscribedSellerIdsRequest{BuyerId: p.BuyerID}))
		if e != nil || len(r.Msg.GetStoreIds()) != 1 || r.Msg.GetStoreIds()[0] != "91" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("BatchGetBuyerDisplayByBuyerIDs", func(t *testing.T) {
		r, e := s.BatchGetBuyerDisplayByBuyerIDs(ctx, connect.NewRequest(&pb.BatchGetBuyerDisplayByBuyerIDsRequest{BuyerIds: []int64{p.BuyerID}}))
		if e != nil || r.Msg.GetBuyers()[p.BuyerID].GetDisplayName() != "買家甲" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("ResolvePrincipals", func(t *testing.T) {
		r, e := s.ResolvePrincipals(ctx, connect.NewRequest(&pb.ResolvePrincipalsRequest{Ids: []int64{p.BuyerID}}))
		if e != nil || r.Msg.GetPrincipals()[p.BuyerID] != accountID {
			t.Fatalf("%v %v", r, e)
		}
		// 不存在的 ID 只在 unresolved_ids，不在 principals 裡（不回吐 buyer_id 冒充 account_id）。
		r, e = s.ResolvePrincipals(ctx, connect.NewRequest(&pb.ResolvePrincipalsRequest{Ids: []int64{p.BuyerID, 999999}}))
		if _, ok := r.Msg.GetPrincipals()[999999]; e != nil || ok || len(r.Msg.GetUnresolvedIds()) != 1 || r.Msg.GetUnresolvedIds()[0] != 999999 || len(r.Msg.GetPrincipals()) != 1 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("BatchGetBuyerMetaByBuyerIDs", func(t *testing.T) {
		r, e := s.BatchGetBuyerMetaByBuyerIDs(ctx, connect.NewRequest(&pb.BatchGetBuyerMetaByBuyerIDsRequest{BuyerIds: []int64{p.BuyerID}}))
		if e != nil || r.Msg.GetBuyers()[p.BuyerID].GetNickname() != "買家甲" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("ListBuyerIDsByAccount", func(t *testing.T) {
		r, e := s.ListBuyerIDsByAccount(ctx, connect.NewRequest(&pb.ListBuyerIDsByAccountRequest{AccountId: accountID}))
		if e != nil || len(r.Msg.GetBuyerIds()) != 1 || r.Msg.GetBuyerIds()[0] != strconv.FormatInt(p.BuyerID, 10) {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetBuyerEntitlement", func(t *testing.T) {
		r, e := s.GetBuyerEntitlement(ctx, connect.NewRequest(&pb.GetBuyerEntitlementRequest{BuyerId: strconv.FormatInt(p.BuyerID, 10)}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetPlanCode() == "" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetSubscriberDashboardStats", func(t *testing.T) {
		r, e := s.GetSubscriberDashboardStats(ctx, connect.NewRequest(&pb.GetSubscriberDashboardStatsRequest{StoreId: 91}))
		if e != nil || r.Msg.GetTotal() != 1 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("BatchGetSubscriberCounts", func(t *testing.T) {
		r, e := s.BatchGetSubscriberCounts(ctx, connect.NewRequest(&pb.BatchGetSubscriberCountsRequest{StoreIds: []int64{91}}))
		if e != nil || r.Msg.GetCounts()[91] != 1 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("CheckFavorite", func(t *testing.T) {
		r, e := s.CheckFavorite(ctx, connect.NewRequest(&pb.CheckFavoriteRequest{BuyerId: p.BuyerID, ProductId: "11"}))
		if e != nil || !r.Msg.GetFavorited() {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("BatchCheckFavorites", func(t *testing.T) {
		r, e := s.BatchCheckFavorites(ctx, connect.NewRequest(&pb.BatchCheckFavoritesRequest{BuyerId: p.BuyerID, ProductIds: []string{"11"}}))
		if e != nil || len(r.Msg.GetFavoritedProductIds()) != 1 || r.Msg.GetFavoritedProductIds()[0] != "11" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("BatchCheckRestockSubscriptions", func(t *testing.T) {
		r, e := s.BatchCheckRestockSubscriptions(ctx, connect.NewRequest(&pb.BatchCheckRestockSubscriptionsRequest{BuyerId: p.BuyerID, ProductIds: []string{"11"}}))
		if e != nil || !r.Msg.GetSubscribed()["11"] {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetSubscribersList", func(t *testing.T) {
		r, e := s.GetSubscribersList(ctx, connect.NewRequest(&pb.GetSubscribersListRequest{StoreId: 91}))
		// id 是舊的追蹤列 ID；模組是複合鍵、沒有列 ID，所以不填，不以 buyer_id 冒充。
		if e != nil || len(r.Msg.GetData()) != 1 || r.Msg.GetData()[0].GetBuyerId() != p.BuyerID || r.Msg.GetData()[0].GetId() != 0 {
			t.Fatalf("%v %v", r, e)
		}
	})
}
