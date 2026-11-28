// Package reviewqueue 是 ReviewQueueAdminService 的 handler（step10e-3d-design.md §3.2）：後台待辦的分頁讀取。
package reviewqueue

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1/reviewqueuev1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	rq "github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

var kindPermissions = map[rq.Kind]adminperm.Permission{
	rq.ReportTriage:               adminperm.SellerReportsList,
	rq.CaseDecision:               adminperm.SellerCasesList,
	rq.CaseInquiryOverdue:         adminperm.SellerCaseInquiriesList,
	rq.AuthorityOrder:             adminperm.SellerAuthorityOrdersList,
	rq.AuthorityTargetNotice:      adminperm.SellerAuthorityOrdersList,
	rq.MeasureAppeal:              adminperm.SellerMeasuresList,
	rq.MeasureLiftRequest:         adminperm.SellerMeasuresList,
	rq.MeasureReview:              adminperm.SellerMeasuresList,
	rq.KYCApplication:             adminperm.SellerKYCApplicationsList,
	rq.PaymentAccountCreateFailed: adminperm.PaymentAccountsGet,
	rq.PaymentAccountPurgeStuck:   adminperm.PaymentAccountsGet,
	rq.SharedBankAccount:          adminperm.SellerMeasuresList,
	rq.ModerationCase:             adminperm.ModerationCasesList,
}

// Lister 是 handler 需要的模組方法（*reviewqueue.Module 實作它）。
type Lister interface {
	List(ctx context.Context, now time.Time, kinds []rq.Kind, after *rq.Cursor, limit int32) ([]rq.Item, error)
}

// StoreNames 是店名的來源（seller 的公開 API）：沒有店名的來源（payment 的收款帳戶待辦只有店家 ID）在這裡補上，店名已清理或店家不存在時是空字串。
type StoreNames interface {
	StoreNames(ctx context.Context, storeIDs []int64) (map[int64]string, error)
}

// Admin 實作 ReviewQueueAdminService。授權（reviewqueue.items.list）由 adminauth 的 interceptor 檢查；種類的權限在這裡與請求的 kinds 取交集。
type Admin struct {
	m      Lister
	names  StoreNames
	now    func() time.Time
	allows func(identity.Principal, adminperm.Permission) bool
}

var _ reviewqueuev1alpha1connect.ReviewQueueAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler；names 是店名的來源，now 是整個請求用的時鐘（nil 時用 time.Now）。
func NewAdmin(m Lister, names StoreNames, now func() time.Time) *Admin {
	if now == nil {
		now = time.Now
	}
	return &Admin{m: m, names: names, now: now, allows: identity.Principal.AdminAllows}
}

func (h *Admin) fillStoreNames(ctx context.Context, items []rq.Item) error {
	var ids []int64
	for _, item := range items {
		if item.StoreID != nil && item.StoreName == "" && !slices.Contains(ids, *item.StoreID) {
			ids = append(ids, *item.StoreID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	names, err := h.names.StoreNames(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].StoreID != nil && items[i].StoreName == "" {
			items[i].StoreName = names[*items[i].StoreID]
		}
	}
	return nil
}

func (h *Admin) ListReviewQueue(ctx context.Context, req *connect.Request[adminpb.ListReviewQueueRequest]) (*connect.Response[adminpb.ListReviewQueueResponse], error) {
	msg := req.Msg
	size, err := pagination.PageSize(msg.GetPageSize(), defaultPageSize, maxPageSize)
	if err != nil {
		return nil, err
	}
	requested, err := requestedKinds(msg.GetKinds())
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(reviewqueuev1alpha1connect.ReviewQueueAdminServiceListReviewQueueProcedure, kindNames(requested)...)
	after, err := rpc.ValidCursor[rq.Cursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if _, err := rpc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	principal, _ := rpc.PrincipalFrom(ctx)
	kinds := h.permittedKinds(principal, requested)
	out := &adminpb.ListReviewQueueResponse{Items: []*adminpb.ReviewQueueItem{}}
	if len(kinds) == 0 {
		return connect.NewResponse(out), nil
	}
	items, err := h.m.List(ctx, h.now(), kinds, after, size+1)
	if err != nil {
		return nil, err
	}
	if len(items) > int(size) {
		items = items[:size]
		out.NextPageToken = rpc.TokenFor(scope, rq.CursorOf(items[size-1]))
	}
	if err := h.fillStoreNames(ctx, items); err != nil {
		return nil, err
	}
	for _, item := range items {
		pb, err := toProto(item)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, pb)
	}
	return connect.NewResponse(out), nil
}

var kindsPaths = rpc.FieldPaths{"Kinds[]": "kinds[]"}

func requestedKinds(in []adminpb.ReviewQueueItemKind) ([]rq.Kind, error) {
	out := make([]rq.Kind, 0, len(in))
	for i, k := range in {
		kind := rpc.EnumFromProto[rq.Kind](k)
		if !kind.Valid() {
			return nil, rpc.WithFieldPaths(
				apperr.InvalidInputOn([]string{fmt.Sprintf("Kinds[%d]", i)}, "kinds[%d] 必須是有效的待辦種類", i),
				kindsPaths)
		}
		out = append(out, kind)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

func kindNames(kinds []rq.Kind) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

func (h *Admin) permittedKinds(p identity.Principal, requested []rq.Kind) []rq.Kind {
	candidates := requested
	if len(candidates) == 0 {
		candidates = rq.Kinds()
	}
	var out []rq.Kind
	for _, k := range candidates {
		if perm, ok := kindPermissions[k]; ok && h.allows(p, perm) {
			out = append(out, k)
		}
	}
	return out
}

func toProto(item rq.Item) (*adminpb.ReviewQueueItem, error) {
	out := &adminpb.ReviewQueueItem{
		Kind:      rpc.EnumToProto[adminpb.ReviewQueueItemKind](item.Kind),
		StoreName: item.StoreName,
		Deadline:  &commonpb.Deadline{StartTime: timestamppb.New(item.Start), DueTime: timestamppb.New(item.Due)},
	}
	if out.Kind == adminpb.ReviewQueueItemKind_REVIEW_QUEUE_ITEM_KIND_UNSPECIFIED {
		return nil, fmt.Errorf("httpapi/reviewqueue：種類 %q 在契約沒有對應的值", item.Kind)
	}
	if item.StoreID != nil {
		out.StoreId = rpc.FormatID(*item.StoreID)
	}
	missing := func(field string) error {
		return fmt.Errorf("httpapi/reviewqueue：%s 的待辦 %s 沒有 %s", item.Kind, item.ItemID, field)
	}
	switch item.Kind {
	case rq.ReportTriage:
		if item.ReportID == nil {
			return nil, missing("report_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_ReportId{ReportId: item.ReportID.String()}
	case rq.CaseDecision:
		if item.CaseID == nil {
			return nil, missing("case_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_CaseId{CaseId: item.CaseID.String()}
	case rq.CaseInquiryOverdue:
		if item.CaseID == nil || item.InquiryID == nil {
			return nil, missing("case_id 或 inquiry_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_CaseInquiry{CaseInquiry: &adminpb.CaseInquiryRef{CaseId: item.CaseID.String(), InquiryId: item.InquiryID.String()}}
	case rq.AuthorityOrder:
		if item.AuthorityOrderID == nil {
			return nil, missing("authority_order_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_AuthorityOrderId{AuthorityOrderId: item.AuthorityOrderID.String()}
	case rq.AuthorityTargetNotice:
		if item.AuthorityOrderID == nil || item.TargetPosition == nil || item.AccountID == nil {
			return nil, missing("authority_order_id、target_position 或 account_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_AuthorityTarget{AuthorityTarget: &adminpb.AuthorityTargetRef{
			OrderId: item.AuthorityOrderID.String(), Position: int32(*item.TargetPosition), AccountId: strconv.FormatInt(*item.AccountID, 10),
		}}
	case rq.MeasureAppeal, rq.MeasureLiftRequest:
		if item.MeasureID == nil || item.RequestID == nil {
			return nil, missing("measure_id 或 request_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_MeasureRequest{MeasureRequest: &adminpb.MeasureRequestRef{MeasureId: item.MeasureID.String(), RequestId: item.RequestID.String()}}
	case rq.MeasureReview:
		if item.MeasureID == nil {
			return nil, missing("measure_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_MeasureId{MeasureId: item.MeasureID.String()}
	case rq.KYCApplication:
		if item.KYCApplicationID == nil || item.StoreID == nil {
			return nil, missing("kyc_application_id 或 store_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_KycApplication{KycApplication: &adminpb.KYCApplicationRef{StoreId: rpc.FormatID(*item.StoreID), ApplicationId: item.KYCApplicationID.String()}}
	case rq.PaymentAccountCreateFailed, rq.PaymentAccountPurgeStuck:
		if item.StoreID == nil {
			return nil, missing("store_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_PaymentAccount{PaymentAccount: &adminpb.PaymentAccountRef{StoreId: rpc.FormatID(*item.StoreID)}}
	case rq.SharedBankAccount:
		if item.MeasureID == nil || item.AccountID == nil {
			return nil, missing("measure_id 或 account_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_SharedBankAccount{SharedBankAccount: &adminpb.SharedBankAccountRef{
			MeasureId: item.MeasureID.String(), AccountId: strconv.FormatInt(*item.AccountID, 10),
		}}
	case rq.ModerationCase:
		if item.CaseID == nil {
			return nil, missing("case_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_ModerationCaseId{ModerationCaseId: item.CaseID.String()}
	default:
		return nil, fmt.Errorf("httpapi/reviewqueue：不認得的種類 %q", item.Kind)
	}
	return out, nil
}
