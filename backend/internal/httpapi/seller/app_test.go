package seller

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

type fakeModule struct {
	Module
	update     model.UpdateProfileInput
	updated    model.Store
	apply      model.ApplyInput
	applyErr   error
	completed  bool
	opening    model.StoreOpening
	openingErr error
	openingFor int64
}

func (f *fakeModule) StoreOpening(_ context.Context, accountID int64) (model.StoreOpening, error) {
	f.openingFor = accountID
	return f.opening, f.openingErr
}

func (f *fakeModule) Apply(_ context.Context, in model.ApplyInput) (model.Store, error) {
	f.apply = in
	return f.updated, f.applyErr
}

func (f *fakeModule) CompleteOnboarding(_ context.Context, storeID, version int64) (model.Store, error) {
	f.update = model.UpdateProfileInput{StoreID: storeID, ExpectedVersion: version}
	f.completed = true
	return f.updated, nil
}

func (f *fakeModule) UpdateProfile(_ context.Context, in model.UpdateProfileInput) (model.Store, error) {
	f.update = in
	return f.updated, nil
}

func ctxWithStore(s model.Store) context.Context {
	return context.WithValue(context.Background(), storeKey{}, s)
}

func currentStore() model.Store {
	text := func(s string) *string { return &s }
	approvedAt := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	return model.Store{
		Profile: model.Profile{
			StoreName: "範例小農", Description: text("介紹"), TownCode: text("63000050"),
		},
		StoreID: 11, StoreCode: 2600000001, AccountID: 7, Version: 3, Status: model.Active, Sanctions: model.NoSanctions(),
		// seller 讀出的條件：營運中、有店名與鄉鎮市區，KYC 還沒核准、沒有 logo。
		Conditions:               model.Conditions{Derived: true, Active: true, NotPendingDeletion: true, StoreName: true, TownCode: true},
		ApprovedFoodRegistration: &model.FoodRegistration{ID: uuid.MustParse("0198a0a0-0000-7000-8000-000000000001"), RegistrationNumber: "A-123", State: model.FoodRegApproved, SubmittedAt: time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC), DecidedAt: &approvedAt},
	}
}

// UpdateMyStore 只套用 update_mask 列出的欄位，其餘沿用目前的值。
func TestUpdateMyStoreAppliesTheMask(t *testing.T) {
	m := &fakeModule{updated: currentStore()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	req := connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{StoreName: "新店名", Description: "不該套用", Etag: "3"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name"}},
	})
	if _, err := h.UpdateMyStore(ctxWithStore(currentStore()), req); err != nil {
		t.Fatal(err)
	}
	in := m.update
	if in.StoreID != 11 || in.ExpectedVersion != 3 || in.StoreName == nil || *in.StoreName != "新店名" || in.Description != nil || in.TownCode != nil {
		t.Fatalf("input=%+v", in)
	}

	// address 列在 mask 但沒有值時清成空字串（模組會清成 NULL）。
	req = connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{Etag: "3"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"address"}},
	})
	if _, err := h.UpdateMyStore(ctxWithStore(currentStore()), req); err != nil {
		t.Fatal(err)
	}
	if in = m.update; in.TownCode == nil || *in.TownCode != "" || in.AddressDetail == nil || *in.AddressDetail != "" {
		t.Fatalf("clearing address failed: %+v", in)
	}

	// 只有鄉鎮沒有明細時正常送出，明細為空字串。
	req = connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{
			Address: &commonpb.PostalAddress{TownCode: "63000050"},
			Etag:    "3",
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"address"}},
	})
	if _, err := h.UpdateMyStore(ctxWithStore(currentStore()), req); err != nil {
		t.Fatal(err)
	}
	if in = m.update; in.TownCode == nil || *in.TownCode != "63000050" || in.AddressDetail == nil || *in.AddressDetail != "" {
		t.Fatalf("town only address failed: %+v", in)
	}
}

// onboarding_completed 是 OUTPUT_ONLY：UpdateMyStore 不會改它，只有 CompleteMyStoreOnboarding 會。
func TestOnboardingIsOnlyCompletedByItsOwnMethod(t *testing.T) {
	m := &fakeModule{updated: currentStore()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	if _, err := h.UpdateMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{OnboardingCompleted: true, Etag: "3"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"onboarding_completed"}},
	})); err != nil || m.completed {
		t.Fatalf("err=%v input=%+v", err, m.update)
	}
	if _, err := h.CompleteMyStoreOnboarding(ctxWithStore(currentStore()), connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{Etag: "3"})); err != nil {
		t.Fatal(err)
	}
	if in := m.update; !m.completed || in.StoreID != 11 || in.ExpectedVersion != 3 || in.StoreName != nil || in.LogoToken != "" {
		t.Fatalf("input=%+v", in)
	}
	if _, err := h.CompleteMyStoreOnboarding(ctxWithStore(currentStore()), connect.NewRequest(&pb.CompleteMyStoreOnboardingRequest{})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("missing etag err=%v", err)
	}
}

// logo 與封面以 update_mask 列出的 staged token 換：列在 mask 裡才送給模組，沒列的不換（沿用）。
func TestUpdateMyStoreSendsTheStagedTokensListedInTheMask(t *testing.T) {
	m := &fakeModule{updated: currentStore()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	store := &pb.Store{Etag: "3", LogoStagedToken: "logo-token", CoverStagedToken: "cover-token"}
	if _, err := h.UpdateMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: store, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"logo_staged_token"}},
	})); err != nil {
		t.Fatal(err)
	}
	if in := m.update; in.LogoToken != "logo-token" || in.CoverToken != "" || in.UploaderID != 7 {
		t.Fatalf("input=%+v", in)
	}
	if _, err := h.UpdateMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: store, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name"}},
	})); err != nil {
		t.Fatal(err)
	}
	if in := m.update; in.LogoToken != "" || in.CoverToken != "" {
		t.Fatalf("tokens outside the mask were sent: %+v", in)
	}
}

// 模組輸入的欄位都對應到請求訊息裡真的存在的路徑，包含 staged token。
func TestFieldPathsPointAtRequestFields(t *testing.T) {
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   interface {
			ProtoReflect() protoreflect.Message
		}
	}{"update": {updatePaths, &pb.UpdateMyStoreRequest{}}, "apply": {applyPaths, &pb.ApplyStoreRequest{}}} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
	for _, field := range []string{"LogoToken", "CoverToken"} {
		if _, ok := updatePaths[field]; !ok {
			t.Errorf("%s 沒有對應到請求的欄位", field)
		}
	}
}

// ApplyStore 的請求只有模組的 ApplyInput 接受的欄位，每一個都送給模組；來源 IP 取自入口放進 ctx 的值、User-Agent 取自請求的
// header，不在請求訊息裡。
func TestApplyStoreSendsExactlyTheFieldsTheModuleAccepts(t *testing.T) {
	m := &fakeModule{updated: currentStore()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	req := connect.NewRequest(&pb.ApplyStoreRequest{
		StoreName: "新店", Description: "介紹", ContactPhone: "0912345678",
		Address: &commonpb.PostalAddress{
			TownCode:      "63000050",
			AddressDetail: "一號",
		},
		Prize: "金牌",
		AcceptedAgreements: []*pb.AgreementAcceptance{
			{Kind: pb.AgreementKind_AGREEMENT_KIND_SELLER_TERMS, Version: "2026-10-15"},
		},
	})
	req.Header().Set("User-Agent", "hk-test/1.0")
	if _, err := h.ApplyStore(rpc.WithClientIP(accountContext(7), "203.0.113.9"), req); err != nil {
		t.Fatal(err)
	}
	want := model.ApplyInput{
		AccountID: 7, StoreName: "新店", Description: "介紹", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "一號", Prize: "金牌",
		AcceptedAgreements: []model.AgreementAcceptance{{Kind: model.AgreementSellerTerms, Version: "2026-10-15"}},
		ClientIP:           "203.0.113.9", UserAgent: "hk-test/1.0",
	}
	if !reflect.DeepEqual(m.apply, want) {
		t.Fatalf("input=%+v", m.apply)
	}
	fields := (&pb.ApplyStoreRequest{}).ProtoReflect().Descriptor().Fields()
	// ApplyStoreRequest 的 address 是 PostalAddress（包含 town_code 與 address_detail 2 個欄位），因此 proto 欄位數比 ApplyInput 少 1
	// （扣除 AccountID、ClientIP、UserAgent：來自呼叫者與傳輸層）。
	inputFields := reflect.TypeFor[model.ApplyInput]().NumField() - 3
	if fields.Len() != inputFields-1 {
		t.Fatalf("ApplyStoreRequest 有 %d 個欄位，ApplyInput 接受 %d 個", fields.Len(), inputFields)
	}
}

// 請求的同意原樣轉給模組：UNSPECIFIED 與不認得的列舉值不會變成有效的種類（模組以空字串與數字字串拒絕）。
func TestApplyStoreForwardsUnrecognizedAgreementKindsAsInvalid(t *testing.T) {
	m := &fakeModule{updated: currentStore()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	_, err := h.ApplyStore(accountContext(7), connect.NewRequest(&pb.ApplyStoreRequest{
		StoreName: "新店",
		AcceptedAgreements: []*pb.AgreementAcceptance{
			{Kind: pb.AgreementKind_AGREEMENT_KIND_UNSPECIFIED, Version: "2026-10-07"},
			{Kind: pb.AgreementKind(99), Version: "2026-10-07"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := m.apply.AcceptedAgreements
	if len(got) != 2 || got[0].Kind != "" || got[1].Kind != "99" {
		t.Fatalf("AcceptedAgreements=%+v", got)
	}
	for _, a := range got {
		if slices.Contains(model.AgreementKinds(), a.Kind) {
			t.Fatalf("無效的種類被當成有效：%+v", a)
		}
	}
}

// ApplyStore 的模組錯誤原樣交回；沒有呼叫者是 internal，不是給使用者的訊息。
func TestApplyStoreErrorsAndMissingPrincipal(t *testing.T) {
	m := &fakeModule{applyErr: apperr.InvalidInputOn([]string{"StoreName"}, "店名必填")}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	_, err := h.ApplyStore(accountContext(7), connect.NewRequest(&pb.ApplyStoreRequest{}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
	if _, err := h.ApplyStore(context.Background(), connect.NewRequest(&pb.ApplyStoreRequest{StoreName: "x"})); apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("err=%v", err)
	}
}

// GetMyStore 每個能力回報一項（含 PUBLIC_LISTING），依 SellerCapability 的數字順序；缺的條件依 SellerRequirement 的數字順序，
// 啟用的能力沒有缺項。
func TestGetMyStoreListsEveryCapabilityStatusInProtoNumberOrder(t *testing.T) {
	m := &fakeModule{}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	resp, err := h.GetMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	caps := resp.Msg.GetCapabilities()
	if len(caps) != len(model.AllCapabilities()) {
		t.Fatalf("capabilities=%v", caps)
	}
	for i, st := range caps {
		if want := capabilityPB(model.AllCapabilities()[i]); st.GetCapability() != want {
			t.Fatalf("position %d is %v, want %v", i, st.GetCapability(), want)
		}
		if st.GetActive() != (len(st.GetMissing()) == 0) || !slices.IsSorted(st.GetMissing()) {
			t.Fatalf("status %v is inconsistent", st)
		}
	}
	byCapability := func(c pb.SellerCapability) *pb.CapabilityStatus { return CapabilityStatusOf(t, caps, c) }
	if st := byCapability(pb.SellerCapability_SELLER_CAPABILITY_STOREFRONT_WRITE); !st.GetActive() {
		t.Fatalf("storefront write=%v", st)
	}
	if st := byCapability(pb.SellerCapability_SELLER_CAPABILITY_SHIPPING_SETUP); st.GetActive() || !slices.Equal(st.GetMissing(), []pb.SellerRequirement{pb.SellerRequirement_SELLER_REQUIREMENT_KYC_APPROVED}) {
		t.Fatalf("shipping setup=%v", st)
	}
	wantListing := []pb.SellerRequirement{pb.SellerRequirement_SELLER_REQUIREMENT_KYC_APPROVED, pb.SellerRequirement_SELLER_REQUIREMENT_LOGO}
	if st := byCapability(pb.SellerCapability_SELLER_CAPABILITY_PUBLIC_LISTING); st.GetActive() || !slices.Equal(st.GetMissing(), wantListing) {
		t.Fatalf("public listing=%v", st)
	}
}

type fakeFollowers struct {
	input    buyer.SubscribersInput
	page     buyer.Page
	displays []buyer.Display
}

func (f *fakeFollowers) Subscribers(_ context.Context, in buyer.SubscribersInput) (buyer.Page, error) {
	f.input = in
	return f.page, nil
}

func (f *fakeFollowers) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) {
	return f.displays, nil
}

// ListMyFollowers 讀呼叫者現任店家的粉絲；page token 只對同一家店有效，已刪除的買家只有 id。
func TestListMyFollowersPagesTheActingStoresFollowers(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	name := "小明"
	f := &fakeFollowers{
		page:     buyer.Page{Items: []buyer.Item{{ID: 5, Time: at}, {ID: 4, Time: at}}, Next: &buyer.Cursor{ID: 4, Time: at}},
		displays: []buyer.Display{{BuyerID: 5, DisplayName: &name}, {BuyerID: 4, Deleted: true}},
	}
	h := mustApp(t, &fakeModule{}, f, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	resp, err := h.ListMyFollowers(ctxWithStore(currentStore()), connect.NewRequest(&pb.ListMyFollowersRequest{PageSize: 2}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg
	if f.input.StoreID != 11 || f.input.PageSize != 2 || f.input.After != nil || len(got.GetFollowers()) != 2 ||
		got.GetFollowers()[0].GetBuyer().GetDisplayName() != "小明" || !got.GetFollowers()[1].GetBuyer().GetDeleted() || got.GetNextPageToken() == "" {
		t.Fatalf("input=%+v resp=%v", f.input, got)
	}
	if _, err := h.ListMyFollowers(ctxWithStore(currentStore()), connect.NewRequest(&pb.ListMyFollowersRequest{PageToken: got.GetNextPageToken()})); err != nil || f.input.After == nil || f.input.After.ID != 4 {
		t.Fatalf("err=%v input=%+v", err, f.input)
	}
	other := currentStore()
	other.StoreID = 12
	if _, err := h.ListMyFollowers(ctxWithStore(other), connect.NewRequest(&pb.ListMyFollowersRequest{PageToken: got.GetNextPageToken()})); err == nil {
		t.Fatalf("token from another store accepted: %v", err)
	}
}

func TestUpdateMyStoreRequiresTheStoreAndTheEtag(t *testing.T) {
	h := mustApp(t, &fakeModule{}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	for name, store := range map[string]*pb.Store{"沒有 store": nil, "沒有 etag": {StoreName: "x"}} {
		_, err := h.UpdateMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.UpdateMyStoreRequest{Store: store}))
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}

// 需要店家的 handler 拿不到 ctx 裡的店家是宣告與 handler 對不上：沒有分類的錯誤（internal），不放行。
func TestHandlersWithoutAnActingStoreFailClosed(t *testing.T) {
	h := mustApp(t, &fakeModule{}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	_, err := h.GetMyStore(context.Background(), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("err=%v", err)
	}
	if _, err := h.CloseMyStore(context.Background(), connect.NewRequest(&pb.CloseMyStoreRequest{})); apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("err=%v", err)
	}
}

// logo_staged_token 或 cover_staged_token 列在 update_mask 卻是空的：契約沒有清除圖片的操作，回 invalid_argument 指向該欄位。
func TestUpdateMyStoreRejectsAnEmptyStagedTokenInTheMask(t *testing.T) {
	for _, path := range []string{"logo_staged_token", "cover_staged_token"} {
		m := &fakeModule{updated: currentStore()}
		h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
		_, err := h.UpdateMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.UpdateMyStoreRequest{
			Store: &pb.Store{Etag: "3"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{path}},
		}))
		if apperr.KindOf(err) != apperr.KindInvalidInput || (m.update != model.UpdateProfileInput{}) || !strings.Contains(err.Error(), path) {
			t.Fatalf("%s: err=%v update=%+v", path, err, m.update)
		}
	}
}

// 買家的頭像是完整網址（舊資料）時不組成 URI template：與其他買家清單同一個 guard，回 Unavailable。
func TestListMyFollowersRejectsAvatarsThatAreNotObjectKeys(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	name, avatar := "小明", "https://cdn.example.com/a.png"
	f := &fakeFollowers{
		page:     buyer.Page{Items: []buyer.Item{{ID: 5, Time: at}}},
		displays: []buyer.Display{{BuyerID: 5, DisplayName: &name, Avatar: &avatar}},
	}
	h := mustApp(t, &fakeModule{}, f, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	_, err := h.ListMyFollowers(ctxWithStore(currentStore()), connect.NewRequest(&pb.ListMyFollowersRequest{}))
	if apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("err=%v", err)
	}
}

// 解得開但內容不合法的 page token（沒有 ID）回 invalid_argument，與買家清單同一個檢查（rpc.ValidCursor）。
func TestListMyFollowersRejectsAMalformedCursor(t *testing.T) {
	f := &fakeFollowers{}
	h := mustApp(t, &fakeModule{}, f, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	scope := rpc.PageScope(sellerv1alpha1connect.SellerAppServiceListMyFollowersProcedure, 11)
	token := rpc.TokenFor(scope, buyer.Cursor{})
	_, err := h.ListMyFollowers(ctxWithStore(currentStore()), connect.NewRequest(&pb.ListMyFollowersRequest{PageToken: token}))
	if err == nil || !strings.HasPrefix(err.Error(), "invalid_argument") || f.input.StoreID != 0 {
		t.Fatalf("err=%v", err)
	}
}

// UpdateMyStore 不再接受 food_business：食品業者登錄只有 SubmitFoodRegistration 這一條寫入路徑，未知的路徑是 invalid_argument。
func TestUpdateMyStoreRejectsTheRemovedFoodBusinessPath(t *testing.T) {
	m := &fakeModule{updated: currentStore()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	_, err := h.UpdateMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{Etag: "3"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"food_business"}},
	}))
	if err == nil || !strings.HasPrefix(err.Error(), "invalid_argument") || m.update.StoreID != 0 {
		t.Fatalf("err=%v update=%+v", err, m.update)
	}
}

// Store 輸出核准的登錄與未結束的那一筆（待審放 pending_food_registration、被退回放 rejected_food_registration，各自只在該狀態設值）；沒有時都不設（不是空訊息）。
// mutation：被退回放進 pending_food_registration → 斷言 rejected 有值與 pending 為 nil 失敗。
func TestStoreOutputsTheApprovedRegistrationAndTheRequest(t *testing.T) {
	st := currentStore()
	decided := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	st.OpenFoodRegistration = &model.FoodRegistration{
		ID: uuid.MustParse("0198a0a0-0000-7000-8000-000000000002"), RegistrationNumber: "A-456", State: model.FoodRegRejected,
		SubmittedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), DecidedAt: &decided, RejectReason: "字號與登錄資料不符",
	}
	h := mustApp(t, &fakeModule{updated: st}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	got, err := h.GetMyStore(ctxWithStore(st), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	reg, req := got.Msg.GetStore().GetApprovedFoodRegistration(), got.Msg.GetStore().GetRejectedFoodRegistration()
	if reg.GetRegistrationNumber() != "A-123" || reg.GetId() == "" || !reg.GetDecideTime().AsTime().Equal(*st.ApprovedFoodRegistration.DecidedAt) || reg.GetState() != pb.FoodRegistrationState_FOOD_REGISTRATION_STATE_APPROVED {
		t.Fatalf("registration=%v", reg)
	}
	if req.GetState() != pb.FoodRegistrationState_FOOD_REGISTRATION_STATE_REJECTED || req.GetRegistrationNumber() != "A-456" ||
		req.GetRejectReason() != "字號與登錄資料不符" || !req.GetDecideTime().AsTime().Equal(decided) || req.GetId() == "" {
		t.Fatalf("request=%v", req)
	}
	if got.Msg.GetStore().PendingFoodRegistration != nil {
		t.Fatalf("被退回的登錄不能放進 pending_food_registration：%v", got.Msg.GetStore())
	}

	none := currentStore()
	none.ApprovedFoodRegistration = nil
	got, err = h.GetMyStore(ctxWithStore(none), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetStore().ApprovedFoodRegistration != nil || got.Msg.GetStore().PendingFoodRegistration != nil || got.Msg.GetStore().RejectedFoodRegistration != nil {
		t.Fatalf("store=%v", got.Msg.GetStore())
	}
}

// GetMyStore 把伺服器算好的 revoked_food_registration 原樣帶出（字號、撤銷時間、理由），與同時存在的待審請求並存；核准中時不設。
func TestGetMyStoreCarriesTheRevokedRegistrationAndTheNewerRequest(t *testing.T) {
	st := currentStore()
	st.ApprovedFoodRegistration = nil
	ended := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	decidedAt := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	st.RevokedFoodRegistration = &model.FoodRegistration{
		ID: uuid.MustParse("0198a0a0-0000-7000-8000-000000000003"), RegistrationNumber: "A-789", State: model.FoodRegEnded,
		SubmittedAt: time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC), DecidedAt: &decidedAt, EndedAt: &ended, EndReason: "字號與食藥署資料不符",
	}
	st.OpenFoodRegistration = &model.FoodRegistration{
		ID: uuid.MustParse("0198a0a0-0000-7000-8000-000000000004"), RegistrationNumber: "A-789", State: model.FoodRegPending,
		SubmittedAt: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
	}
	h := mustApp(t, &fakeModule{updated: st}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	got, err := h.GetMyStore(ctxWithStore(st), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	revoked := got.Msg.GetStore().GetRevokedFoodRegistration()
	if revoked.GetRegistrationNumber() != "A-789" || revoked.GetEndReason() != "字號與食藥署資料不符" || !revoked.GetEndTime().AsTime().Equal(ended) || revoked.GetId() == "" ||
		revoked.GetState() != pb.FoodRegistrationState_FOOD_REGISTRATION_STATE_ENDED {
		t.Fatalf("revoked=%v", revoked)
	}
	if got.Msg.GetStore().GetApprovedFoodRegistration() != nil || got.Msg.GetStore().GetPendingFoodRegistration().GetRegistrationNumber() != "A-789" ||
		got.Msg.GetStore().RejectedFoodRegistration != nil {
		t.Fatalf("store=%v", got.Msg.GetStore())
	}
	approved := currentStore()
	got, err = h.GetMyStore(ctxWithStore(approved), connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetStore().GetRevokedFoodRegistration() != nil {
		t.Fatalf("沒有撤銷時不設：%v", got.Msg.GetStore().GetRevokedFoodRegistration())
	}
}

// GetMyStore 的 suspended 與 capabilities 的 NOT_SUSPENDED 缺項由同一個 stint.Derive 結果（Store.Suspended）產生：兩者永遠一起改變。
func TestGetMyStoreSuspendedAgreesWithTheNotSuspendedRequirement(t *testing.T) {
	for _, suspended := range []bool{false, true} {
		st := currentStore()
		st.Sanctions = sanctionsOf(suspended)
		m := &fakeModule{}
		h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
		resp, err := h.GetMyStore(ctxWithStore(st), connect.NewRequest(&pb.GetMyStoreRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		missing := false
		for _, c := range resp.Msg.GetCapabilities() {
			if slices.Contains(c.GetMissing(), pb.SellerRequirement_SELLER_REQUIREMENT_NOT_SUSPENDED) {
				missing = true
			}
		}
		if resp.Msg.GetSuspended() != suspended || missing != suspended {
			t.Fatalf("Store.Suspended=%v：suspended=%v，能力缺 NOT_SUSPENDED=%v", suspended, resp.Msg.GetSuspended(), missing)
		}
	}
}

func sanctionsOf(suspended bool) *model.Sanctions {
	if !suspended {
		return model.NoSanctions()
	}
	return model.SanctionedBy(model.EffectiveMeasure{Kind: model.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: model.BasisViolation})
}

// GetMyStoreOpening 把模組的可用性與原因（依優先序）原樣對到契約：帳號是呼叫者、原因順序不變、可以開店時沒有原因；沒有呼叫者是沒有分類的錯誤；
// 列舉與模組的原因一對一。
func TestGetMyStoreOpeningMapsTheModuleAnswer(t *testing.T) {
	var unavailable []model.OpeningUnavailability
	for _, b := range []model.OpeningBlocker{model.OpeningTerminated, model.OpeningSuspended, model.OpeningAccountDeleting, model.OpeningStoreExists} {
		unavailable = append(unavailable, model.OpeningUnavailability{Blocker: b, Message: b.Text(uitext.ZhTW)})
	}
	m := &fakeModule{opening: model.StoreOpening{Unavailable: unavailable}}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	got, err := h.GetMyStoreOpening(accountContext(7), connect.NewRequest(&pb.GetMyStoreOpeningRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	want := []pb.StoreOpeningUnavailableReason{
		pb.StoreOpeningUnavailableReason_STORE_OPENING_UNAVAILABLE_REASON_TERMINATED, pb.StoreOpeningUnavailableReason_STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED,
		pb.StoreOpeningUnavailableReason_STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING, pb.StoreOpeningUnavailableReason_STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS,
	}
	if m.openingFor != 7 || got.Msg.GetCanApplyStore() || len(got.Msg.GetUnavailabilities()) != len(want) {
		t.Fatalf("for=%d got=%v", m.openingFor, got.Msg)
	}
	for i, u := range got.Msg.GetUnavailabilities() {
		if u.GetReason() != want[i] || u.GetMessage().GetLocale() != "zh-TW" || u.GetMessage().GetMessage() != unavailable[i].Blocker.Text(uitext.ZhTW).Text || u.GetMessage().GetMessage() == "" {
			t.Fatalf("unavailabilities[%d]=%v", i, u)
		}
	}
	m.opening = model.StoreOpening{CanApply: true}
	got, err = h.GetMyStoreOpening(accountContext(7), connect.NewRequest(&pb.GetMyStoreOpeningRequest{}))
	if err != nil || !got.Msg.GetCanApplyStore() || len(got.Msg.GetUnavailabilities()) != 0 {
		t.Fatalf("可以開店：%v %v", got, err)
	}
	if _, err := h.GetMyStoreOpening(context.Background(), connect.NewRequest(&pb.GetMyStoreOpeningRequest{})); apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("沒有呼叫者：%v", err)
	}
	if got := rpc.EnumMismatches[pb.StoreOpeningUnavailableReason](model.OpeningBlockerValues()); len(got) != 0 {
		t.Fatalf("StoreOpeningUnavailableReason：%v", got)
	}
}
