package buyer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeModule struct {
	Module
	profile model.Profile
	restock map[int64]bool
}

func (f fakeModule) Profile(context.Context, int64) (model.Profile, error) { return f.profile, nil }
func (f fakeModule) CurrentBuyerID(context.Context, int64) (int64, error) {
	return f.profile.BuyerID, nil
}
func (f fakeModule) UpdateProfile(context.Context, model.UpdateProfileInput) (model.Profile, error) {
	return f.profile, nil
}

// lookupModule 記下 buyer_id 的查詢次數並回傳指定結果。
type lookupModule struct {
	Module
	calls *int
	id    int64
	err   error
}

func (m lookupModule) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	*m.calls++
	if accountID != 1 {
		return 0, errors.New("wrong account")
	}
	return m.id, m.err
}

// profileModule 只允許以帳號一次讀出 profile；呼叫 CurrentBuyerID 代表 GetProfile／UpdateProfile 又多查了一次。
type profileModule struct {
	Module
	calls *int
	err   error
}

func (m profileModule) Profile(_ context.Context, accountID int64) (model.Profile, error) {
	*m.calls++
	if accountID != 1 {
		return model.Profile{}, errors.New("wrong account")
	}
	return model.Profile{BuyerID: 5007, AccountID: 1, DisplayName: "買家甲", Version: 3}, m.err
}

func (m profileModule) CurrentBuyerID(context.Context, int64) (int64, error) {
	*m.calls += 100
	return 0, errors.New("GetProfile 不該另外查 buyer_id")
}

// GetProfile 一次查詢取得 profile；存活的帳號沒有現任買家與 caller 一樣是不變量被破壞（internal），不是 NotFound。
func TestGetProfileReadsOnceWithCallerErrorSemantics(t *testing.T) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	calls := 0
	h := mustApp(t, profileModule{calls: &calls}, rpc.Images{}, Ports{})
	resp, err := h.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{}))
	if err != nil || calls != 1 || resp.Msg.GetProfile().GetId() != "5007" {
		t.Fatalf("resp=%v err=%v lookups=%d", resp, err, calls)
	}
	calls = 0
	h = mustApp(t, profileModule{calls: &calls, err: apperr.NotFound("找不到買家資料")}, rpc.Images{}, Ports{})
	var ae *apperr.Error
	if _, err := h.GetProfile(ctx, connect.NewRequest(&pb.GetProfileRequest{})); calls != 1 || apperr.KindOf(err) != apperr.KindInternal || errors.As(err, &ae) {
		t.Fatalf("err=%v lookups=%d", err, calls)
	}
}

// 每個帳號都是買家（W5）：帳號的 buyer_id 每個請求查一次，管理員與未登入不查；存活的帳號沒有現任買家是不變量被
// 破壞（沒有分類的錯誤 → internal），查詢本身失敗照原本的分類回傳。
func TestCallerResolvesBuyerIDOnlyForAccounts(t *testing.T) {
	account := func(roles ...identity.Role) context.Context {
		return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1, Roles: roles})
	}
	admin := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 1})
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		id        int64
		err       error
		wantCalls int
		wantID    int64
		wantErr   bool
		wantKind  apperr.Kind
		wantCode  connect.Code
	}{
		{name: "帳號", ctx: account(), id: 5007, wantCalls: 1, wantID: 5007},
		{name: "賣家也是買家", ctx: account(identity.RoleSeller), id: 5007, wantCalls: 1, wantID: 5007},
		{name: "管理員", ctx: admin, wantCode: connect.CodePermissionDenied},
		{name: "未登入", ctx: context.Background(), wantCode: connect.CodePermissionDenied},
		{name: "存活的帳號沒有買家", ctx: account(), err: apperr.NotFound("找不到買家資料"), wantCalls: 1, wantErr: true, wantKind: apperr.KindInternal},
		{name: "查詢失敗", ctx: account(), err: apperr.Unavailable("暫時無法使用"), wantCalls: 1, wantErr: true, wantKind: apperr.KindUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := mustApp(t, lookupModule{calls: &calls, id: tc.id, err: tc.err}, rpc.Images{}, Ports{})
			got, err := h.caller(tc.ctx)
			if calls != tc.wantCalls {
				t.Fatalf("lookups=%d want %d", calls, tc.wantCalls)
			}
			switch {
			case tc.wantCode != 0:
				// rpc.Error 的訊息以 code 開頭；轉成 Connect 的 code 在 interceptor（integration 測試經 HTTP 驗證）。
				if err == nil || !strings.HasPrefix(err.Error(), tc.wantCode.String()+": ") {
					t.Fatalf("err=%v want %v", err, tc.wantCode)
				}
			case tc.wantErr:
				var ae *apperr.Error
				if err == nil || apperr.KindOf(err) != tc.wantKind || (tc.wantKind == apperr.KindInternal && errors.As(err, &ae)) {
					t.Fatalf("err=%v want kind %v", err, tc.wantKind)
				}
			default:
				if err != nil || got != tc.wantID {
					t.Fatalf("caller=%d err=%v", got, err)
				}
			}
		})
	}
}
func (f fakeModule) BatchCheckRestock(_ context.Context, _ int64, _ []int64) (map[int64]bool, error) {
	return f.restock, nil
}

type fakeProducts struct {
	cards map[int64]ProductInfo
}

func (f fakeProducts) Cards(context.Context, []int64, bool) (map[int64]ProductInfo, error) {
	return f.cards, nil
}

type fakeStores struct {
	cards  []seller.StoreCard
	public map[int64]seller.Store
}

func (f fakeStores) StoreCards(context.Context, []int64) ([]seller.StoreCard, error) {
	return f.cards, nil
}

func (f fakeStores) PublicStores(_ context.Context, ids []int64) ([]seller.Store, error) {
	var out []seller.Store
	for _, id := range ids {
		if s, ok := f.public[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func TestProductCardsRestockSubscribed(t *testing.T) {
	images, err := rpc.NewImages("http://localhost:8787")
	if err != nil {
		t.Fatal(err)
	}
	h := mustApp(t, fakeModule{restock: map[int64]bool{9: true}}, images, Ports{
		Products: fakeProducts{cards: map[int64]ProductInfo{9: {ID: 9, Name: "蘋果", StoreID: 3, MinPrice: 20, MaxPrice: 30, Available: true, ImageKey: "products/apple"}}},
		Stores:   fakeStores{cards: []seller.StoreCard{openStoreCard(3, "店家", "stores/logo")}},
	}, testStats(&countedRatings{}))
	cards, err := h.productCards(context.Background(), 1, []model.Item{{ID: 9, Time: time.Now()}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !cards[9].RestockSubscribed || !cards[9].Available || cards[9].Store == nil || cards[9].Image == nil {
		t.Fatalf("card=%+v", cards[9])
	}
}

func TestEnumMappings(t *testing.T) {
	if _, err := genderModel(pb.Gender_GENDER_UNSPECIFIED); !isInvalidArgument(err) {
		t.Fatalf("UNSPECIFIED must be invalid_argument, got %v", err)
	}
	for _, tc := range []struct {
		goValue model.Gender
		wire    pb.Gender
	}{{model.Male, pb.Gender_GENDER_MALE}, {model.Female, pb.Gender_GENDER_FEMALE}, {model.Other, pb.Gender_GENDER_OTHER}} {
		if genderPB(tc.goValue) != tc.wire {
			t.Fatal(tc)
		}
		back, err := genderModel(tc.wire)
		if err != nil || back != tc.goValue {
			t.Fatalf("gender roundtrip %v: %v", tc, err)
		}
	}
	if statusPB(model.Active) != pb.BuyerStatus_BUYER_STATUS_ACTIVE || statusPB(model.PendingDeletion) != pb.BuyerStatus_BUYER_STATUS_PENDING_DELETION {
		t.Fatal("status mapping")
	}
}

func TestBuyerServiceRequiredRequests(t *testing.T) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	h := mustApp(t, fakeModule{profile: model.Profile{BuyerID: 7, AccountID: 1, Version: 1}}, rpc.Images{}, Ports{})
	for _, call := range []func() error{
		func() error { _, err := h.GetAddress(ctx, connect.NewRequest(&pb.GetAddressRequest{})); return err },
		func() error {
			_, err := h.CreateAddress(ctx, connect.NewRequest(&pb.CreateAddressRequest{}))
			return err
		},
		func() error {
			_, err := h.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{}))
			return err
		},
		func() error {
			_, err := h.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{
				Profile:    &pb.Profile{Etag: rpc.Etag(1)},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"avatar_staged_token"}},
			}))
			return err
		},
		func() error { _, err := h.AddFavorite(ctx, connect.NewRequest(&pb.AddFavoriteRequest{})); return err },
		func() error { _, err := h.FollowStore(ctx, connect.NewRequest(&pb.FollowStoreRequest{})); return err },
	} {
		if err := call(); err == nil {
			t.Fatal("required field accepted")
		}
	}
	admin := NewAdmin(nil)
	if _, err := admin.UpdateMembershipPlan(ctx, connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{})); err == nil {
		t.Fatal("membership_plan accepted empty")
	}
}

func TestUpdateProfileAvatarTokenInMaskValidation(t *testing.T) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	h := mustApp(t, fakeModule{profile: model.Profile{BuyerID: 7, AccountID: 1, Version: 1}}, rpc.Images{}, Ports{})
	_, err := h.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{
		Profile: &pb.Profile{
			Etag:              rpc.Etag(1),
			AvatarStagedToken: "   ",
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"avatar_staged_token"}},
	}))
	if err == nil {
		t.Fatal("expected error when avatar_staged_token is in mask but token is empty")
	}
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err kind = %v, want KindInvalidInput", apperr.KindOf(err))
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) == 0 || ae.FieldViolations()[0].Field != "profile.avatar_staged_token" {
		t.Fatalf("expected violation on profile.avatar_staged_token, got %v", err)
	}
}

type updateRecorderModule struct {
	fakeModule
	recorded model.UpdateProfileInput
	calls    int
}

func (m *updateRecorderModule) UpdateProfile(_ context.Context, in model.UpdateProfileInput) (model.Profile, error) {
	m.recorded = in
	m.calls++
	return m.profile, nil
}

func TestUpdateProfileWildcardMaskWithoutAvatarToken(t *testing.T) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	m := &updateRecorderModule{fakeModule: fakeModule{profile: model.Profile{BuyerID: 7, AccountID: 1, Version: 1}}}
	h := mustApp(t, m, rpc.Images{}, Ports{})
	_, err := h.UpdateProfile(ctx, connect.NewRequest(&pb.UpdateProfileRequest{
		Profile: &pb.Profile{
			DisplayName: "新暱稱",
			Etag:        rpc.Etag(1),
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"*"}},
	}))
	if err != nil {
		t.Fatalf("wildcard mask failed: %v", err)
	}
	if m.recorded.AvatarStagedToken != nil {
		t.Fatalf("expected AvatarStagedToken to be nil under wildcard mask without token, got %q", *m.recorded.AvatarStagedToken)
	}
	if m.recorded.DisplayName == nil || *m.recorded.DisplayName != "新暱稱" {
		t.Fatalf("expected DisplayName '新暱稱', got %v", m.recorded.DisplayName)
	}
}

func TestToProfileGenderIsSetOnlyWhenStored(t *testing.T) {
	h := mustApp(t, fakeModule{}, rpc.Images{}, Ports{})
	if got := h.toProfile(model.Profile{BuyerID: 7}); got.Gender != nil {
		t.Fatalf("no stored gender must leave the field unset, got %v", got.Gender)
	}
	female := model.Female
	got := h.toProfile(model.Profile{BuyerID: 7, Gender: &female})
	if got.Gender == nil || *got.Gender != pb.Gender_GENDER_FEMALE {
		t.Fatalf("stored FEMALE must read back as FEMALE, got %v", got.Gender)
	}
}

func TestUpdateProfileGender(t *testing.T) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	other := pb.Gender_GENDER_OTHER
	unspecified := pb.Gender_GENDER_UNSPECIFIED
	run := func(msg *pb.Profile, paths []string) (*updateRecorderModule, error) {
		m := &updateRecorderModule{fakeModule: fakeModule{profile: model.Profile{BuyerID: 7, AccountID: 1, Version: 1}}}
		h := mustApp(t, m, rpc.Images{}, Ports{})
		msg.Etag = rpc.Etag(1)
		req := &pb.UpdateProfileRequest{Profile: msg}
		if paths != nil {
			req.UpdateMask = &fieldmaskpb.FieldMask{Paths: paths}
		}
		_, err := h.UpdateProfile(ctx, connect.NewRequest(req))
		return m, err
	}
	t.Run("mask gender with a value converts it", func(t *testing.T) {
		m, err := run(&pb.Profile{Gender: &other}, []string{"gender"})
		if err != nil || m.recorded.Gender == nil || *m.recorded.Gender != model.Other {
			t.Fatalf("err=%v gender=%v", err, m.recorded.Gender)
		}
	})
	t.Run("mask gender without a value clears", func(t *testing.T) {
		m, err := run(&pb.Profile{DisplayName: "x"}, []string{"gender"})
		if err != nil || m.recorded.Gender == nil || *m.recorded.Gender != "" {
			t.Fatalf("err=%v gender=%v", err, m.recorded.Gender)
		}
	})
	t.Run("mask gender with UNSPECIFIED is rejected before the module", func(t *testing.T) {
		m, err := run(&pb.Profile{Gender: &unspecified}, []string{"gender"})
		if !isInvalidArgument(err) || m.calls != 0 {
			t.Fatalf("err=%v calls=%d", err, m.calls)
		}
	})
	t.Run("no mask with UNSPECIFIED is rejected", func(t *testing.T) {
		m, err := run(&pb.Profile{Gender: &unspecified}, nil)
		if !isInvalidArgument(err) || m.calls != 0 {
			t.Fatalf("err=%v calls=%d", err, m.calls)
		}
	})
	t.Run("no mask and no gender leaves gender alone", func(t *testing.T) {
		m, err := run(&pb.Profile{DisplayName: "新暱稱"}, nil)
		if err != nil || m.recorded.Gender != nil {
			t.Fatalf("err=%v gender=%v", err, m.recorded.Gender)
		}
	})
}

// visitModule 記下 RecordVisit 收到的參數。
type visitModule struct {
	fakeModule
	calls   *[][2]int64
	visitID int64
}

func (m visitModule) CurrentBuyerID(context.Context, int64) (int64, error) { return m.visitID, nil }
func (m visitModule) RecordVisit(_ context.Context, buyerID, productID int64) error {
	*m.calls = append(*m.calls, [2]int64{buyerID, productID})
	return nil
}

// forbiddenProducts 在被呼叫時讓測試失敗。
type forbiddenProducts struct{ t *testing.T }

func (f forbiddenProducts) Cards(context.Context, []int64, bool) (map[int64]ProductInfo, error) {
	f.t.Error("RecordVisit must not call the product port")
	return nil, errors.New("product service down")
}

// RecordVisit 不在請求的關鍵路徑上解析商品：商品服務不通也要記下瀏覽（店家由 seller 的背景 job 解析）。
func TestRecordVisitDoesNotTouchTheProductPort(t *testing.T) {
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	var calls [][2]int64
	h := mustApp(t, visitModule{calls: &calls, visitID: 5007}, rpc.Images{}, Ports{Products: forbiddenProducts{t}})
	if _, err := h.RecordVisit(ctx, connect.NewRequest(&pb.RecordVisitRequest{ProductId: "9"})); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != [2]int64{5007, 9} {
		t.Fatalf("RecordVisit calls=%v", calls)
	}
}

func openStoreCard(id int64, name, logo string) seller.StoreCard {
	return seller.StoreCard{StoreID: id, StoreCode: 2600000000 + id, Name: &name, LogoKey: &logo, Status: seller.CardOpen}
}

// isInvalidArgument 讀 rpc.Error 的 code：它回的不是 *connect.Error（要到 interceptor 才轉換），
// connect.CodeOf 會得到 unknown；interceptor 的授權又需要 procedure 與呼叫者，單元測試不經過它。
func isInvalidArgument(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), connect.CodeInvalidArgument.String()+":")
}
