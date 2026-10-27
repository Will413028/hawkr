// Package seller 實作賣家 Connect API 的傳輸與顯示資料組裝。
package seller

import (
	"context"
	"errors"
	"slices"
	"strings"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Module 是 handler 使用的店家讀寫介面。取得呼叫者目前的店家（ActingStore）不在這裡：它只由 interceptor 依授權宣告
// 呼叫（Acting），handler 從 ctx 取（storeFrom）。
type Module interface {
	LookupBusinessRegistration(ctx context.Context, taxID string) (model.Registration, bool, error)
	Apply(context.Context, model.ApplyInput) (model.Store, error)
	StoreOpening(ctx context.Context, accountID int64) (model.StoreOpening, error)
	UpdateProfile(context.Context, model.UpdateProfileInput) (model.Store, error)
	SubmitFoodRegistration(context.Context, model.SubmitFoodRegistrationInput) (model.Store, error)
	WithdrawFoodRegistration(ctx context.Context, storeID int64) (model.Store, error)
	ClearFoodRegistration(ctx context.Context, storeID int64) (model.Store, error)
	CompleteOnboarding(ctx context.Context, storeID, expectedVersion int64) (model.Store, error)
	CloseCurrent(context.Context, int64) (model.Store, error)
	SaveKYCDraft(context.Context, model.KYCDraftInput) (model.KYCApplication, error)
	MyKYC(ctx context.Context, accountID int64) (model.KYCApplication, error)
	SubmitKYC(context.Context, model.SubmitKYCInput) (model.KYCApplication, error)
	CancelKYC(context.Context, model.CancelKYCInput) (model.KYCApplication, error)
	AccountMeasures(context.Context, model.AccountMeasureListInput) (model.MeasurePage, error)
	AccountMeasure(ctx context.Context, accountID int64, measureID uuid.UUID) (model.Measure, error)
	SubmitAppeal(context.Context, model.SubmitAppealInput) (model.Measure, error)
	RequestRemediation(context.Context, model.RequestRemediationInput) (model.Measure, error)
}

// Followers 是店家粉絲的來源（buyer 模組擁有追蹤與買家的顯示資料）。
type Followers interface {
	Subscribers(context.Context, buyer.SubscribersInput) (buyer.Page, error)
	BatchDisplay(context.Context, []int64) ([]buyer.Display, error)
}

// App 是 SellerAppService 的 handler。
type App struct {
	m         Module
	followers Followers
	images    rpc.Images
	dash      Dashboard
	stats     *StatsSource
}

var _ sellerv1alpha1connect.SellerAppServiceHandler = (*App)(nil)

// NewApp 建立賣家 handler。dash 是儀表板的來源與時鐘，GetMyDashboard 用，三個來源由組裝點提供。stats 是 GetMyStore 的統計來源（粉絲與評價），
// 與前台店面頁（NewPublic）是同一份 *StatsSource（NewStatsSource 建立，nil 回錯誤）。統編的登記資料經模組（Module.LookupBusinessRegistration）。
func NewApp(m Module, followers Followers, images rpc.Images, dash Dashboard, stats *StatsSource) (*App, error) {
	if stats == nil {
		return nil, ErrStatsSourceRequired
	}
	return &App{m: m, followers: followers, images: images, dash: dash, stats: stats}, nil
}

// deref 是可選文字的值：沒有值是空字串（契約以空字串表示沒有填）。
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// errNoPrincipal 是請求通過了 rpc 的授權卻沒有呼叫者：授權鏈壞了，沒有分類（internal），不是使用者的錯。
var errNoPrincipal = errors.New("httpapi/seller：ctx 裡沒有呼叫者")

// 模組輸入的欄位對應到請求的路徑。
var (
	applyPaths = rpc.FieldPaths{
		"StoreName": "store_name", "Description": "description", "ContactPhone": "contact_phone",
		"TownCode": "address.town_code", "AddressDetail": "address.address_detail", "Prize": "prize",
		"AcceptedAgreements": "accepted_agreements",
	}
	updatePaths = rpc.FieldPaths{
		"StoreName": "store.store_name", "Description": "store.description", "ContactPhone": "store.contact_phone",
		"ContactName": "store.contact_name", "ContactMobile": "store.contact_mobile", "SupportEmail": "store.support_email",
		"TownCode": "store.address.town_code", "AddressDetail": "store.address.address_detail", "Prize": "store.prize",
		"WebsiteURL": "store.website_url",
		"LogoToken":  "store.logo_staged_token", "CoverToken": "store.cover_staged_token",
	}
)

// acceptances 轉成模組的同意：UNSPECIFIED 與不認得的值轉成模組不認得的種類，由模組回 invalid_argument。
func acceptances(in []*pb.AgreementAcceptance) []model.AgreementAcceptance {
	out := make([]model.AgreementAcceptance, 0, len(in))
	for _, a := range in {
		out = append(out, model.AgreementAcceptance{Kind: rpc.EnumFromProto[model.AgreementKind](a.GetKind()), Version: a.GetVersion()})
	}
	return out
}

func (h *App) toStore(s model.Store) *pb.Store {
	out := &pb.Store{
		Id: rpc.FormatID(s.StoreID), StoreCode: rpc.FormatID(s.StoreCode), StoreName: s.StoreName,
		Description: deref(s.Description), ContactPhone: deref(s.ContactPhone),
		ContactName: deref(s.ContactName), ContactMobile: deref(s.ContactMobile), SupportEmail: deref(s.SupportEmail),
		Prize:               deref(s.Prize),
		WebsiteUrl:          deref(s.WebsiteURL),
		OnboardingCompleted: s.OnboardingCompleted,
		Status:              rpc.EnumToProto[pb.StoreStatus](s.Status),
		CreateTime:          timestamppb.New(s.CreatedAt), UpdateTime: timestamppb.New(s.UpdatedAt), Etag: rpc.Etag(s.Version),
	}
	out.ApprovedFoodRegistration = toFoodRegistration(s.ApprovedFoodRegistration)
	// 模組的 OpenFoodRegistration 是待審或被退回的那一筆；契約以欄位表達狀態，client 不比較 state。
	if open := s.OpenFoodRegistration; open != nil {
		switch open.State {
		case model.FoodRegPending:
			out.PendingFoodRegistration = toFoodRegistration(open)
		case model.FoodRegRejected:
			out.RejectedFoodRegistration = toFoodRegistration(open)
		case model.FoodRegApproved, model.FoodRegWithdrawn, model.FoodRegEnded:
			// 模組的 OpenFoodRegistration 只會是待審或被退回；其他狀態沒有對應的欄位，不輸出。
		}
	}
	out.RevokedFoodRegistration = toFoodRegistration(s.RevokedFoodRegistration)
	if s.TownCode != nil {
		out.Address = &commonpb.PostalAddress{
			TownCode:      *s.TownCode,
			AddressDetail: deref(s.AddressDetail),
		}
	}
	if s.LogoKey != nil {
		out.Logo = h.images.Image(*s.LogoKey)
	}
	if s.CoverKey != nil {
		out.CoverImage = h.images.Image(*s.CoverKey)
	}
	return out
}

func (h *App) ApplyStore(ctx context.Context, req *connect.Request[pb.ApplyStoreRequest]) (*connect.Response[pb.ApplyStoreResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	msg := req.Msg
	var townCode, addressDetail string
	if addr := msg.GetAddress(); addr != nil {
		townCode = addr.GetTownCode()
		addressDetail = addr.GetAddressDetail()
	}
	created, err := h.m.Apply(ctx, model.ApplyInput{
		AccountID: p.ID, StoreName: msg.GetStoreName(), Description: msg.GetDescription(), ContactPhone: msg.GetContactPhone(),
		Prize: msg.GetPrize(), TownCode: townCode, AddressDetail: addressDetail,
		AcceptedAgreements: acceptances(msg.GetAcceptedAgreements()),
		ClientIP:           rpc.ClientIPFrom(ctx), UserAgent: req.Header().Get("User-Agent"),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, applyPaths)
	}
	return connect.NewResponse(&pb.ApplyStoreResponse{Store: h.toStore(created)}), nil
}

func (h *App) GetMyStoreOpening(ctx context.Context, _ *connect.Request[pb.GetMyStoreOpeningRequest]) (*connect.Response[pb.GetMyStoreOpeningResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	opening, err := h.m.StoreOpening(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := &pb.GetMyStoreOpeningResponse{CanApplyStore: opening.CanApply}
	for _, u := range opening.Unavailable {
		out.Unavailabilities = append(out.Unavailabilities, &pb.StoreOpeningUnavailability{
			Reason: rpc.EnumToProto[pb.StoreOpeningUnavailableReason](u.Blocker), Message: rpc.LocalizedMessage(u.Message),
		})
	}
	return connect.NewResponse(out), nil
}

func (h *App) GetMyStore(ctx context.Context, _ *connect.Request[pb.GetMyStoreRequest]) (*connect.Response[pb.GetMyStoreResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	// 統計是附屬資訊（各自有 statsBudget，讀不到的欄位不設），不論店是否公開都讀：賣家預覽自己的店。suspended 與 capabilities 的
	// NOT_SUSPENDED 缺項都由 current.Suspended()（stint.Derive 的同一個結果）產生。
	out := &pb.GetMyStoreResponse{Store: h.toStore(current), Stats: h.stats.Store(ctx, current.StoreID), Suspended: current.Suspended()}
	statuses := model.CapabilitiesOf(current)
	out.Capabilities = make([]*pb.CapabilityStatus, 0, len(statuses))
	for _, st := range statuses {
		status := &pb.CapabilityStatus{Capability: capabilityPB(st.Capability), Active: st.Active}
		for _, r := range st.Missing {
			status.Missing = append(status.Missing, rpc.EnumToProto[pb.SellerRequirement](r))
		}
		out.Capabilities = append(out.Capabilities, status)
	}
	return connect.NewResponse(out), nil
}

func (h *App) UpdateMyStore(ctx context.Context, req *connect.Request[pb.UpdateMyStoreRequest]) (*connect.Response[pb.UpdateMyStoreResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetStore()
	if msg == nil {
		return nil, rpc.RequiredField("store", "store 必填")
	}
	v, err := rpc.RequiredVersionField(msg.GetEtag(), "store.etag")
	if err != nil {
		return nil, err
	}
	// 契約沒有「清掉 logo 或封面」：列在 mask 卻沒有 token 是請求的錯，不當成沒有改。
	for _, t := range []struct{ path, token string }{
		{"logo_staged_token", msg.GetLogoStagedToken()}, {"cover_staged_token", msg.GetCoverStagedToken()},
	} {
		if t.token == "" && slices.Contains(req.Msg.GetUpdateMask().GetPaths(), t.path) {
			return nil, rpc.RequiredField("store."+t.path, t.path+" 在 update_mask 裡時必須提供 staged token（沒有清除 logo 或封面的操作）")
		}
	}
	paths, err := rpc.WritableFields(msg, req.Msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	in := model.UpdateProfileInput{
		StoreID:         current.StoreID,
		ExpectedVersion: v,
		UploaderID:      current.AccountID,
	}
	has := func(path string) bool { return slices.Contains(paths, path) }
	text := func(path string, dst **string, val string) {
		if has(path) {
			*dst = &val
		}
	}
	text("store_name", &in.StoreName, msg.GetStoreName())
	text("description", &in.Description, msg.GetDescription())
	text("contact_phone", &in.ContactPhone, msg.GetContactPhone())
	text("contact_name", &in.ContactName, msg.GetContactName())
	text("contact_mobile", &in.ContactMobile, msg.GetContactMobile())
	text("support_email", &in.SupportEmail, msg.GetSupportEmail())
	text("prize", &in.Prize, msg.GetPrize())
	text("website_url", &in.WebsiteURL, msg.GetWebsiteUrl())
	if has("address") {
		if addr := msg.GetAddress(); addr != nil {
			townCode := addr.GetTownCode()
			addressDetail := addr.GetAddressDetail()
			in.TownCode = &townCode
			in.AddressDetail = &addressDetail
		} else {
			empty := ""
			in.TownCode = &empty
			in.AddressDetail = &empty
		}
	}
	if has("logo_staged_token") {
		in.LogoToken = msg.GetLogoStagedToken()
	}
	if has("cover_staged_token") {
		in.CoverToken = msg.GetCoverStagedToken()
	}
	updated, err := h.m.UpdateProfile(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, updatePaths)
	}
	return connect.NewResponse(&pb.UpdateMyStoreResponse{Store: h.toStore(updated)}), nil
}

func (h *App) CompleteMyStoreOnboarding(ctx context.Context, req *connect.Request[pb.CompleteMyStoreOnboardingRequest]) (*connect.Response[pb.CompleteMyStoreOnboardingResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	v, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	updated, err := h.m.CompleteOnboarding(ctx, current.StoreID, v)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CompleteMyStoreOnboardingResponse{Store: h.toStore(updated)}), nil
}

func (h *App) ListMyFollowers(ctx context.Context, req *connect.Request[pb.ListMyFollowersRequest]) (*connect.Response[pb.ListMyFollowersResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(sellerv1alpha1connect.SellerAppServiceListMyFollowersProcedure, current.StoreID)
	after, err := rpc.ValidCursor[buyer.Cursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.followers.Subscribers(ctx, buyer.SubscribersInput{StoreID: current.StoreID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	buyerIDs := make([]int64, len(page.Items))
	for i, item := range page.Items {
		buyerIDs[i] = item.ID
	}
	displays, err := h.followers.BatchDisplay(ctx, buyerIDs)
	if err != nil {
		return nil, err
	}
	displayOf := make(map[int64]buyer.Display, len(displays))
	for _, d := range displays {
		displayOf[d.BuyerID] = d
	}
	out := &pb.ListMyFollowersResponse{Followers: make([]*pb.Follower, len(page.Items))}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	for i, item := range page.Items {
		d, ok := displayOf[item.ID]
		if !ok {
			return nil, apperr.Unavailable("買家顯示資料不完整")
		}
		card := &pb.BuyerCard{Id: rpc.FormatID(item.ID), Deleted: d.Deleted}
		if !d.Deleted {
			card.DisplayName = deref(d.DisplayName)
			if d.Avatar != nil {
				if err := rpc.ObjectKey(*d.Avatar); err != nil {
					return nil, err
				}
				card.Avatar = h.images.Image(*d.Avatar)
			}
		}
		out.Followers[i] = &pb.Follower{Buyer: card, CreateTime: timestamppb.New(item.Time)}
	}
	return connect.NewResponse(out), nil
}

func (h *App) CloseMyStore(ctx context.Context, _ *connect.Request[pb.CloseMyStoreRequest]) (*connect.Response[pb.CloseMyStoreResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	closed, err := h.m.CloseCurrent(ctx, current.AccountID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CloseMyStoreResponse{Store: h.toStore(closed)}), nil
}

// capabilityPB 把模組的能力（CAP_ACCOUNT_SELF_READ）轉成契約的列舉（SELLER_CAPABILITY_ACCOUNT_SELF_READ）。
func capabilityPB(c model.Capability) pb.SellerCapability {
	return rpc.EnumToProto[pb.SellerCapability](strings.TrimPrefix(string(c), "CAP_"))
}
