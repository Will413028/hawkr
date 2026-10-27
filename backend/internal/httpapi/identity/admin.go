package identity

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Admin 實作 IdentityAdminService。授權（只有管理員）由 rpc 的 interceptor 依 service 名稱檢查。
type Admin struct {
	m        *identity.Module
	d        *delegation.Module
	deletion *accountdeletion.Module
}

var _ identityv1alpha1connect.IdentityAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m *identity.Module, d *delegation.Module, deletion *accountdeletion.Module) *Admin {
	return &Admin{m: m, d: d, deletion: deletion}
}

func (h *Admin) GetSession(ctx context.Context, _ *connect.Request[adminpb.GetSessionRequest]) (*connect.Response[adminpb.GetSessionResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return nil, errors.New("httpapi/identity：後台的請求沒有管理員，interceptor 沒有掛上")
	}
	a, err := h.m.Admin(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetSessionResponse{
		Admin: &adminpb.Admin{Id: rpc.FormatID(a.ID), Email: a.Email},
	}), nil
}

func (h *Admin) ListAccounts(ctx context.Context, req *connect.Request[adminpb.ListAccountsRequest]) (*connect.Response[adminpb.ListAccountsResponse], error) {
	role := rpc.EnumFromProto[identity.Role](req.Msg.GetRole())
	if role != "" && role != identity.RoleSeller {
		return nil, rpc.Error(connect.CodeInvalidArgument, "role %s 不是已知的角色", req.Msg.GetRole())
	}
	f := identity.AccountFilter{Role: role, Query: req.Msg.GetQuery(), Email: req.Msg.GetEmail()}
	// page token 只能用在同一組篩選：換了篩選就從第一頁開始。
	scope := rpc.FilterScope(identityv1alpha1connect.IdentityAdminServiceListAccountsProcedure, string(f.Role), f.Query, f.Email)
	key, err := rpc.DecodePageToken(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	var after *identity.AccountCursor
	if key != nil {
		if after, err = decodeAccountCursor(key); err != nil {
			return nil, err
		}
	}
	page, err := h.m.ListAccounts(ctx, identity.ListAccountsInput{Filter: f, After: after, Size: req.Msg.GetPageSize()})
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListAccountsResponse{
		Accounts:      make([]*adminpb.Account, len(page.Accounts)),
		NextPageToken: rpc.EncodePageToken(scope, encodeAccountCursor(page.Next)),
		TotalSize:     int32(min(page.Total, int64(^uint32(0)>>1))), //nolint:gosec // count is nonnegative and capped at MaxInt32
	}
	for i, a := range page.Accounts {
		out.Accounts[i] = accountProto(a)
	}
	return connect.NewResponse(out), nil
}

// accountProto 轉成後台的帳號；ListAccounts 與 GetAccount 共用。
func accountProto(a identity.Account) *adminpb.Account {
	roles := make([]pb.Role, len(a.Roles))
	for j, r := range a.Roles {
		roles[j] = rpc.EnumToProto[pb.Role](r)
	}
	return &adminpb.Account{
		Id: rpc.FormatID(a.ID), Email: a.Email, Roles: roles, Suspended: a.Suspended,
		CreateTime: timestamppb.New(a.CreatedAt),
	}
}

// GetAccount 以 ID 讀一個存活的帳號。已刪除的帳號（墓碑）與不存在的都回 not_found，與 ListAccounts 只列存活帳號一致。
func (h *Admin) GetAccount(ctx context.Context, req *connect.Request[adminpb.GetAccountRequest]) (*connect.Response[adminpb.GetAccountResponse], error) {
	id, err := rpc.ParseID("account_id", req.Msg.GetAccountId())
	if err != nil {
		return nil, err
	}
	a, err := h.m.Account(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Deleted {
		return nil, apperr.NotFound("帳號 %d 不存在", id)
	}
	return connect.NewResponse(&adminpb.GetAccountResponse{Account: accountProto(a)}), nil
}

// deleteAccountPaths 把 accountdeletion 回報的欄位對應到請求的路徑。
var deleteAccountPaths = rpc.FieldPaths{"Reason": "reason", "AccountID": "account_id"}

// DeleteAccount 以管理員為申請者刪除整個帳號：accountdeletion 在同一個 transaction 內把買家與目前的店家當參與者關閉並刪帳號，
// 有未完成的訂單時帳號、買家與店家一起進入等待（管理員也不能略過）。每次受理都記管理員與理由。
func (h *Admin) DeleteAccount(ctx context.Context, req *connect.Request[adminpb.DeleteAccountRequest]) (*connect.Response[adminpb.DeleteAccountResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return nil, errors.New("httpapi/identity：後台的請求沒有管理員，interceptor 沒有掛上")
	}
	accountID, err := rpc.ParseID("account_id", req.Msg.GetAccountId())
	if err != nil {
		return nil, err
	}
	adminID := p.ID
	// 理由的驗證與正規化只在 accountdeletion（和關店的理由同一條規則），錯誤帶欄位，這裡只對應到請求的路徑。
	state, err := h.deletion.Request(ctx, accountdeletion.RequestInput{
		AccountID: accountID, Requester: accountdeletion.Admin, OperatorAdminID: &adminID, Reason: req.Msg.GetReason(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, deleteAccountPaths)
	}
	result := pb.DeleteAccountResponse_DELETION_STATE_DELETED
	if state == accountdeletion.PendingOrders {
		result = pb.DeleteAccountResponse_DELETION_STATE_PENDING_ORDERS
	}
	return connect.NewResponse(&adminpb.DeleteAccountResponse{State: result}), nil
}

// SignOut 在管理員登出之前撤銷他進行中的代行（設計書 56 §4.5）；登出本身由 Access 處理。
func (h *Admin) SignOut(ctx context.Context, _ *connect.Request[adminpb.SignOutRequest]) (*connect.Response[adminpb.SignOutResponse], error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return nil, errors.New("httpapi/identity：後台的請求沒有管理員，interceptor 沒有掛上")
	}
	if err := h.d.RevokeDelegationsOfAdmin(ctx, p.ID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.SignOutResponse{}), nil
}

// 帳號清單的排序鍵是 (created_at, id)：16 bytes，建立時間的 Unix 奈秒加上 ID。
func encodeAccountCursor(c *identity.AccountCursor) []byte {
	if c == nil {
		return nil
	}
	b := binary.BigEndian.AppendUint64(nil, uint64(c.CreatedAt.UnixNano()))
	return binary.BigEndian.AppendUint64(b, uint64(c.ID)) //nolint:gosec // cursor encodes the signed ID as unchanged bits
}

func decodeAccountCursor(b []byte) (*identity.AccountCursor, error) {
	if len(b) != 16 {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 無法使用，請從第一頁重新讀取")
	}
	return &identity.AccountCursor{
		CreatedAt: time.Unix(0, int64(binary.BigEndian.Uint64(b[:8]))).UTC(), //nolint:gosec // cursor restores the original signed bits
		ID:        int64(binary.BigEndian.Uint64(b[8:])),                     //nolint:gosec // cursor restores the original signed bits
	}, nil
}
