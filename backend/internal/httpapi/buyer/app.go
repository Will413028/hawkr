// Package buyer 實作買家 Connect API 的傳輸與顯示資料組裝。
package buyer

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Module 是買家業務資料的讀寫介面。
type Module interface {
	listModule
	Profile(context.Context, int64) (model.Profile, error)
	CurrentBuyerID(context.Context, int64) (int64, error)
	UpdateProfile(context.Context, model.UpdateProfileInput) (model.Profile, error)
	PageAddresses(context.Context, model.AddressListInput) (model.AddressPage, error)
	GetAddress(context.Context, int64, int64) (model.Address, error)
	AddAddress(context.Context, model.AddressInput) (model.Address, error)
	UpdateAddress(context.Context, model.AddressInput) (model.Address, error)
	DeleteAddress(context.Context, int64, int64, int64) error
}

// App 是 BuyerAppService 的 handler。
type App struct {
	m      Module
	images rpc.Images
	ports  Ports
	// stats 是店家卡片評分的組裝（與店面頁同一份 *StatsSource，只用到評價與 logger）。
	stats *httpseller.StatsSource
}

var _ buyerv1alpha1connect.BuyerAppServiceHandler = (*App)(nil)

// NewApp 建立買家 handler。stats 是店家卡片評分的組裝（httpseller.NewStatsSource 建立，與店面頁同一份；nil 回錯誤）。
func NewApp(m Module, images rpc.Images, ports Ports, stats *httpseller.StatsSource) (*App, error) {
	if stats == nil {
		return nil, httpseller.ErrStatsSourceRequired
	}
	return &App{m: m, images: images, ports: ports, stats: stats}, nil
}

// accountCaller 回傳呼叫者的帳號 ID；只有顧客帳號是買家，管理員與未登入回 permission_denied。
func accountCaller(ctx context.Context) (int64, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAccount {
		return 0, rpc.Error(connect.CodePermissionDenied, "%s", "需要買家身分")
	}
	return p.ID, nil
}

// buyerInvariant 把「存活的帳號查不到現任買家」轉成 internal：帳號活著 ⇔ 買家活著（ADR
// 2026-09-30-account-created-provisions-buyer），認證過的帳號都活著，所以這是不變量被破壞（interceptor 記 error log），
// 不當成「沒有買家」。
func buyerInvariant(accountID int64, err error) error {
	if apperr.KindOf(err) == apperr.KindNotFound {
		return fmt.Errorf("httpapi/buyer：存活的帳號 %d 沒有現任買家（不變量被破壞）", accountID)
	}
	return err
}

// caller 解析呼叫者的 buyer_id：每個帳號都是買家，每個請求只查 buyer_id，不讀 profile 個資。
func (h *App) caller(ctx context.Context) (int64, error) {
	accountID, err := accountCaller(ctx)
	if err != nil {
		return 0, err
	}
	buyerID, err := h.m.CurrentBuyerID(ctx, accountID)
	if err != nil {
		return 0, buyerInvariant(accountID, err)
	}
	return buyerID, nil
}

// callerProfile 以一次查詢讀取呼叫者的現任 profile；錯誤語意與 caller 相同。
func (h *App) callerProfile(ctx context.Context) (model.Profile, error) {
	accountID, err := accountCaller(ctx)
	if err != nil {
		return model.Profile{}, err
	}
	profile, err := h.m.Profile(ctx, accountID)
	if err != nil {
		return model.Profile{}, buyerInvariant(accountID, err)
	}
	return profile, nil
}

func (h *App) toProfile(p model.Profile) *pb.Profile {
	out := &pb.Profile{Id: rpc.FormatID(p.BuyerID), DisplayName: p.DisplayName, Status: statusPB(p.Status), CreateTime: timestamppb.New(p.CreatedAt), UpdateTime: timestamppb.New(p.UpdatedAt), Etag: rpc.Etag(p.Version)}
	if p.Phone != nil {
		out.Phone = *p.Phone
	}
	if p.Gender != nil {
		g := genderPB(*p.Gender)
		out.Gender = &g
	}
	if p.BirthDate != nil {
		out.BirthDate = &date.Date{Year: int32(p.BirthDate.Year()), Month: int32(p.BirthDate.Month()), Day: int32(p.BirthDate.Day())} //nolint:gosec // time.Time calendar fields fit the Date schema
	}
	if p.City != nil {
		out.City = *p.City
	}
	if p.Job != nil {
		out.Job = *p.Job
	}
	if p.Avatar != nil {
		out.Avatar = h.images.Image(*p.Avatar)
	}
	return out
}

func (h *App) toAddress(a model.Address) *pb.Address {
	return &pb.Address{
		Id:            rpc.FormatID(a.AddressID),
		ReceiverName:  a.ReceiverName,
		ReceiverPhone: a.ReceiverPhone,
		PostalAddress: &commonpb.PostalAddress{
			TownCode:      a.TownCode,
			AddressDetail: a.AddressDetail,
		},
		DefaultAddress: a.IsDefault,
		CreateTime:     timestamppb.New(a.CreatedAt),
		UpdateTime:     timestamppb.New(a.UpdatedAt),
		Etag:           rpc.Etag(a.Version),
	}
}

func statusPB(s model.Status) pb.BuyerStatus {
	switch s {
	case model.Active:
		return pb.BuyerStatus_BUYER_STATUS_ACTIVE
	case model.PendingDeletion:
		return pb.BuyerStatus_BUYER_STATUS_PENDING_DELETION
	case model.Closed:
		return pb.BuyerStatus_BUYER_STATUS_UNSPECIFIED
	}
	return pb.BuyerStatus_BUYER_STATUS_UNSPECIFIED
}

func genderPB(g model.Gender) pb.Gender {
	switch g {
	case model.Male:
		return pb.Gender_GENDER_MALE
	case model.Female:
		return pb.Gender_GENDER_FEMALE
	case model.Other:
		return pb.Gender_GENDER_OTHER
	}
	return pb.Gender_GENDER_UNSPECIFIED
}

func genderModel(g pb.Gender) (model.Gender, error) {
	switch g {
	case pb.Gender_GENDER_MALE:
		return model.Male, nil
	case pb.Gender_GENDER_FEMALE:
		return model.Female, nil
	case pb.Gender_GENDER_OTHER:
		return model.Other, nil
	}
	return "", rpc.Error(connect.CodeInvalidArgument, "%s", "gender 不合法")
}

func addressInput(buyerID, addressID, expected int64, a *pb.Address) model.AddressInput {
	var townCode, addressDetail string
	if pa := a.GetPostalAddress(); pa != nil {
		townCode = pa.GetTownCode()
		addressDetail = pa.GetAddressDetail()
	}
	return model.AddressInput{
		BuyerID:         buyerID,
		AddressID:       addressID,
		ExpectedVersion: expected,
		ReceiverName:    a.GetReceiverName(),
		ReceiverPhone:   a.GetReceiverPhone(),
		TownCode:        townCode,
		AddressDetail:   addressDetail,
		IsDefault:       a.GetDefaultAddress(),
	}
}

var profilePaths = rpc.FieldPaths{
	"DisplayName":       "profile.display_name",
	"Phone":             "profile.phone",
	"Gender":            "profile.gender",
	"BirthDate":         "profile.birth_date",
	"City":              "profile.city",
	"Job":               "profile.job",
	"AvatarStagedToken": "profile.avatar_staged_token",
}
var addressPaths = rpc.FieldPaths{
	"ReceiverName":  "address.receiver_name",
	"ReceiverPhone": "address.receiver_phone",
	"TownCode":      "address.postal_address.town_code",
	"AddressDetail": "address.postal_address.address_detail",
}

func (h *App) GetProfile(ctx context.Context, _ *connect.Request[pb.GetProfileRequest]) (*connect.Response[pb.GetProfileResponse], error) {
	profile, err := h.callerProfile(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetProfileResponse{Profile: h.toProfile(profile)}), nil
}

func (h *App) UpdateProfile(ctx context.Context, req *connect.Request[pb.UpdateProfileRequest]) (*connect.Response[pb.UpdateProfileResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetProfile()
	if msg == nil {
		return nil, rpc.RequiredField("profile", "profile 必填")
	}
	v, err := rpc.RequiredVersionField(msg.GetEtag(), "profile.etag")
	if err != nil {
		return nil, err
	}
	if slices.Contains(req.Msg.GetUpdateMask().GetPaths(), "avatar_staged_token") && strings.TrimSpace(msg.GetAvatarStagedToken()) == "" {
		return nil, rpc.RequiredField("profile.avatar_staged_token", "avatar_staged_token 在 update_mask 裡時必須提供 staged token（沒有清除頭像的操作）")
	}
	paths, err := rpc.WritableFields(msg, req.Msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	in := model.UpdateProfileInput{
		BuyerID:         buyerID,
		ExpectedVersion: v,
	}
	has := func(path string) bool { return slices.Contains(paths, path) }
	text := func(path string, dst **string, val string) {
		if has(path) {
			*dst = &val
		}
	}
	text("display_name", &in.DisplayName, msg.GetDisplayName())
	text("phone", &in.Phone, msg.GetPhone())
	text("city", &in.City, msg.GetCity())
	text("job", &in.Job, msg.GetJob())
	if has("gender") {
		var gender model.Gender
		if msg.Gender != nil {
			g, err := genderModel(msg.GetGender())
			if err != nil {
				return nil, err
			}
			gender = g
		}
		in.Gender = &gender
	}
	if has("birth_date") {
		if d := msg.GetBirthDate(); d != nil {
			dateValue := time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
			in.BirthDate = &dateValue
		} else {
			empty := time.Time{}
			in.BirthDate = &empty
		}
	}
	if has("avatar_staged_token") && msg.GetAvatarStagedToken() != "" {
		token := msg.GetAvatarStagedToken()
		in.AvatarStagedToken = &token
	}
	updated, err := h.m.UpdateProfile(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, profilePaths)
	}
	return connect.NewResponse(&pb.UpdateProfileResponse{Profile: h.toProfile(updated)}), nil
}

func (h *App) ListAddresses(ctx context.Context, req *connect.Request[pb.ListAddressesRequest]) (*connect.Response[pb.ListAddressesResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListAddressesProcedure, buyerID)
	after, err := addressCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.PageAddresses(ctx, model.AddressListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &pb.ListAddressesResponse{Addresses: make([]*pb.Address, len(page.Items)), NextPageToken: encodeAddressCursor(scope, page.Next)}
	for i, a := range page.Items {
		out.Addresses[i] = h.toAddress(a)
	}
	return connect.NewResponse(out), nil
}

func encodeAddressCursor(scope string, c *model.AddressCursor) string {
	if c == nil {
		return ""
	}
	return rpc.TokenFor(scope, c)
}

func addressCursor(scope, token string) (*model.AddressCursor, error) {
	c, err := rpc.PageCursor[model.AddressCursor](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	if c.ID <= 0 || c.Time.IsZero() {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	return c, nil
}

func (h *App) GetAddress(ctx context.Context, req *connect.Request[pb.GetAddressRequest]) (*connect.Response[pb.GetAddressResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	a, err := h.m.GetAddress(ctx, buyerID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetAddressResponse{Address: h.toAddress(a)}), nil
}

func (h *App) CreateAddress(ctx context.Context, req *connect.Request[pb.CreateAddressRequest]) (*connect.Response[pb.CreateAddressResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetAddress() == nil {
		return nil, rpc.RequiredField("address", "address 必填")
	}
	if req.Msg.GetAddress().GetPostalAddress() == nil {
		return nil, rpc.RequiredField("address.postal_address", "address.postal_address 必填")
	}
	a, err := h.m.AddAddress(ctx, addressInput(buyerID, 0, 0, req.Msg.GetAddress()))
	if err != nil {
		return nil, rpc.WithFieldPaths(err, addressPaths)
	}
	return connect.NewResponse(&pb.CreateAddressResponse{Address: h.toAddress(a)}), nil
}

func (h *App) UpdateAddress(ctx context.Context, req *connect.Request[pb.UpdateAddressRequest]) (*connect.Response[pb.UpdateAddressResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetAddress()
	if msg == nil {
		return nil, rpc.RequiredField("address", "address 必填")
	}
	id, err := rpc.ParseID("id", msg.GetId())
	if err != nil {
		return nil, err
	}
	v, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	current, err := h.m.GetAddress(ctx, buyerID, id)
	if err != nil {
		return nil, err
	}
	merged := h.toAddress(current)
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	if merged.GetPostalAddress() == nil {
		return nil, rpc.RequiredField("address.postal_address", "address.postal_address 必填")
	}
	a, err := h.m.UpdateAddress(ctx, addressInput(buyerID, id, v, merged))
	if err != nil {
		return nil, rpc.WithFieldPaths(err, addressPaths)
	}
	return connect.NewResponse(&pb.UpdateAddressResponse{Address: h.toAddress(a)}), nil
}

func (h *App) DeleteAddress(ctx context.Context, req *connect.Request[pb.DeleteAddressRequest]) (*connect.Response[pb.DeleteAddressResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	v, err := rpc.VersionFromEtag(req.Msg.GetEtag())
	if err != nil {
		return nil, err
	}
	if err := h.m.DeleteAddress(ctx, buyerID, id, v); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteAddressResponse{}), nil
}
