// Package reviewqueue 是 ReviewQueueAdminService 的 handler（step10e-3d-design.md §3.2）：後台待辦的分頁讀取。
package reviewqueue

import (
	"context"
	"errors"
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
	// defaultPageSize 與 maxPageSize 寫在 proto 的欄位註解（ListReviewQueueRequest.page_size）。
	defaultPageSize = 50
	maxPageSize     = 100
)

// kindPermissions 是種類 → 看這一種待辦所需的既有讀取權限（E13）：待辦只列呼叫者對來源有讀取權的種類。
// 權限是來源的讀取權：通報、案件（待決定）、案件的詢問、來函（來函與告知被指名者）、措施（申覆、解除請求與複審）、
// 商家認證各自頁面的 list 權限；重新綁定 PayUni 的來源是 PayUni 註冊，它唯一的讀取權限是 get（沒有 list RPC，不新增沒有 RPC 的 list 權限）。
// 新種類沒有列在這裡，TestKindsAreOneToOneWithTheContract 失敗（它逐種類寫死期望的權限）。
var kindPermissions = map[rq.Kind]adminperm.Permission{
	rq.ReportTriage:          adminperm.SellerReportsList,
	rq.CaseDecision:          adminperm.SellerCasesList,
	rq.CaseInquiryOverdue:    adminperm.SellerCaseInquiriesList,
	rq.AuthorityOrder:        adminperm.SellerAuthorityOrdersList,
	rq.AuthorityTargetNotice: adminperm.SellerAuthorityOrdersList,
	rq.MeasureAppeal:         adminperm.SellerMeasuresList,
	rq.MeasureLiftRequest:    adminperm.SellerMeasuresList,
	rq.MeasureReview:         adminperm.SellerMeasuresList,
	rq.KYCApplication:        adminperm.SellerKYCApplicationsList,
	rq.PayUniBindRetry:       adminperm.SellerPayUniRegistrationsGet,
}

// Lister 是 handler 需要的模組方法（*reviewqueue.Module 實作它）。
type Lister interface {
	List(ctx context.Context, now time.Time, kinds []rq.Kind, after *rq.Cursor, limit int32) ([]rq.Item, error)
}

// Admin 實作 ReviewQueueAdminService。授權（reviewqueue.items.list）由 adminauth 的 interceptor 檢查；種類的權限在這裡與請求的 kinds 取交集。
type Admin struct {
	m   Lister
	now func() time.Time
	// allows 回答呼叫者有沒有某個讀取權限；預設是 Principal.AdminAllows（interceptor 與 adminauth.GuardFile 用的同一組）。
	// 現在只有 OWNER 一個角色（擁有全部權限），測試換掉它才能表達「只有某些來源的讀取權」。
	allows func(identity.Principal, adminperm.Permission) bool
}

var _ reviewqueuev1alpha1connect.ReviewQueueAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler；now 是整個請求用的時鐘（nil 時用 time.Now）。
func NewAdmin(m Lister, now func() time.Time) *Admin {
	if now == nil {
		now = time.Now
	}
	return &Admin{m: m, now: now, allows: identity.Principal.AdminAllows}
}

func (h *Admin) ListReviewQueue(ctx context.Context, req *connect.Request[adminpb.ListReviewQueueRequest]) (*connect.Response[adminpb.ListReviewQueueResponse], error) {
	msg := req.Msg
	size, err := pagination.PageSize(msg.GetPageSize(), defaultPageSize, maxPageSize)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, rpc.FieldPaths{"PageSize": "page_size"})
	}
	requested, err := requestedKinds(msg.GetKinds())
	if err != nil {
		return nil, err
	}
	// token 綁的是請求的 kinds（排序去重後），不是與權限取交集之後的：同一個人的權限在翻頁中途改變不會讓 token 失效，
	// 只是後面的頁看得到的種類跟著變。
	scope := rpc.FilterScope(reviewqueuev1alpha1connect.ReviewQueueAdminServiceListReviewQueueProcedure, kindNames(requested)...)
	after, err := rpc.ValidCursor[rq.Cursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	principal, ok := rpc.PrincipalFrom(ctx)
	if !ok || principal.Kind != identity.PrincipalAdmin {
		return nil, errors.New("httpapi/reviewqueue：後台的請求沒有管理員，interceptor 沒有掛上")
	}
	kinds := h.permittedKinds(principal, requested)
	out := &adminpb.ListReviewQueueResponse{Items: []*adminpb.ReviewQueueItem{}}
	if len(kinds) == 0 {
		return connect.NewResponse(out), nil
	}
	// 取 N＋1：多出來的那一筆只用來知道有沒有下一頁。
	items, err := h.m.List(ctx, h.now(), kinds, after, size+1)
	if err != nil {
		return nil, err
	}
	if len(items) > int(size) {
		items = items[:size]
		out.NextPageToken = rpc.TokenFor(scope, rq.CursorOf(items[size-1]))
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

// kindsPaths 是 kinds 欄位錯誤（模組端的名稱 Kinds[i]）對應到請求的路徑 kinds[i]；handler 的測試以 descriptor 驗證。
var kindsPaths = rpc.FieldPaths{"Kinds[]": "kinds[]"}

// requestedKinds 把請求的 kinds 轉成模組的種類，排序並去重；UNSPECIFIED 與不認得的數字是 INVALID_ARGUMENT。空的就是空的（展開成有權的全部在 permittedKinds）。
func requestedKinds(in []adminpb.ReviewQueueItemKind) ([]rq.Kind, error) {
	out := make([]rq.Kind, 0, len(in))
	for i, k := range in {
		kind := rpc.EnumFromProto[rq.Kind](k)
		if !kind.Valid() {
			// 欄位錯誤：repeated 單一元素的問題，field 帶索引（AIP-193）。
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

// permittedKinds 是請求的 kinds（空的是全部種類）與呼叫者有讀取權的種類的交集。沒有權限的種類不出現，不是錯誤。
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

// toProto 把一列待辦轉成契約：subject 依種類組，種類需要的對象鍵缺了是模組與來源不一致（內部錯誤），不輸出半個 subject。
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
	case rq.KYCApplication, rq.PayUniBindRetry:
		if item.KYCApplicationID == nil || item.StoreID == nil {
			return nil, missing("kyc_application_id 或 store_id")
		}
		out.Subject = &adminpb.ReviewQueueItem_KycApplication{KycApplication: &adminpb.KYCApplicationRef{StoreId: rpc.FormatID(*item.StoreID), ApplicationId: item.KYCApplicationID.String()}}
	default:
		return nil, fmt.Errorf("httpapi/reviewqueue：不認得的種類 %q", item.Kind)
	}
	return out, nil
}
