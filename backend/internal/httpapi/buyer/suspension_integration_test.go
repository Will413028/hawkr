//go:build integration

package buyer_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	identitypb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	sellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) suspend(t *testing.T) {
	t.Helper()
	if _, _, err := f.id.SuspendAccount(context.Background(), identity.SuspendInput{AccountID: f.accountID, Category: policy.Other, Reason: "測試"}); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) clerkJobs(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT count(*) FROM river.river_job WHERE kind = 'identity.clerk_user'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 停權的帳號仍能登入：GetSession 成功並顯示停權中，讀取成功，公開方法成功，寫入回 ACCOUNT_SUSPENDED；停權不排 Clerk 的 job；
// 解除之後恢復。
func TestSuspendedAccountHasRestrictedAccess(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s, err := f.identity.GetSession(ctx, connect.NewRequest(&identitypb.GetSessionRequest{}))
	if err != nil || s.Msg.GetSuspension() != nil {
		t.Fatalf("未停權的 GetSession = %v, %v", s, err)
	}
	before := time.Now().Add(-time.Minute)
	f.suspend(t)
	s, err = f.identity.GetSession(ctx, connect.NewRequest(&identitypb.GetSessionRequest{}))
	if err != nil || s.Msg.GetAccount().GetId() == "" || s.Msg.GetSuspension() == nil || s.Msg.GetSuspension().GetSuspendedAt().AsTime().Before(before) {
		t.Fatalf("停權的 GetSession = %v, %v，預期成功並顯示停權中", s, err)
	}
	if _, err := f.app.ListAddresses(ctx, connect.NewRequest(&pb.ListAddressesRequest{})); err != nil {
		t.Fatalf("停權的帳號讀取 = %v", err)
	}
	public := sellerconnect.NewSellerPublicServiceClient(f.http, f.base, clientOptions(f.buyerToken)...)
	if _, err := public.GetSellerAgreements(ctx, connect.NewRequest(&sellerpb.GetSellerAgreementsRequest{})); err != nil {
		t.Fatalf("停權的帳號呼叫公開方法 = %v", err)
	}
	_, err = f.app.AddFavorite(ctx, connect.NewRequest(&pb.AddFavoriteRequest{ProductId: "11"}))
	wantCode(t, err, connect.CodePermissionDenied, "ACCOUNT_SUSPENDED")
	_, err = f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("王小明收件人")}))
	wantCode(t, err, connect.CodePermissionDenied, "ACCOUNT_SUSPENDED")
	if n := f.clerkJobs(t); n != 0 {
		t.Fatalf("停權排了 %d 個 Clerk job", n)
	}

	if _, _, err := f.id.UnsuspendAccount(ctx, f.accountID, "申覆成立"); err != nil {
		t.Fatal(err)
	}
	s, err = f.identity.GetSession(ctx, connect.NewRequest(&identitypb.GetSessionRequest{}))
	if err != nil || s.Msg.GetSuspension() != nil {
		t.Fatalf("解除後的 GetSession = %v, %v", s, err)
	}
	if _, err := f.app.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{Address: address("王小明收件人")})); err != nil {
		t.Fatalf("解除後的寫入 = %v", err)
	}
}

// 停權的帳號可以刪除自己的帳號（自助）。
func TestSuspendedAccountCanDeleteItself(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.suspend(t)
	resp, err := f.identity.DeleteAccount(context.Background(), connect.NewRequest(&identitypb.DeleteAccountRequest{}))
	if err != nil || resp.Msg.GetState() != identitypb.DeleteAccountResponse_DELETION_STATE_DELETED {
		t.Fatalf("停權的帳號刪帳 = %v, %v", resp, err)
	}
	if account, err := f.id.Account(context.Background(), f.accountID); err != nil || !account.Deleted {
		t.Fatalf("account=%+v err=%v", account, err)
	}
}
