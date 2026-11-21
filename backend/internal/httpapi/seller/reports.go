package seller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 檢舉人的入口（SellerReportAppService）：檢舉人是買家，不是店家，所以這個 service 不掛
// 賣家能力的 interceptor（宣告表只涵蓋 SellerAppService）。送出的額度見 ReportSubmitQuotas。只能檢舉曾經公開過的店、不能檢舉自己的店是模組的規則
// （seller.SubmitReport）。

// ReportModule 是通報入口使用的模組方法。
type ReportModule interface {
	SubmitReport(context.Context, model.SubmitReportInput) (model.SubmitReportResult, error)
	BuyerReports(context.Context, model.BuyerReportListInput) (model.ReportPage, error)
}

// Buyers 解析登入帳號目前的買家（每個存活的帳號都是買家，ADR 2026-09-30-account-created-provisions-buyer）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
}

// Quota 是一個時間窗內的次數上限。
type Quota struct {
	Limit  int
	Window time.Duration
}

// ReportSubmitQuotas 是送出檢舉的兩個時間窗（10 分鐘 5 次、1 小時 10 次；治理 ADR 2026-09-18 決策六，2026-10-02 保留）：
// CreateReport 由這裡以帳號計數（Limiter，計數表滿時拒絕）。兩個時間窗全部在額度內才一起計，被拒絕的請求都不計（沒被服務的請求不燒掉
// 長的時間窗，同舊 gateway 的 DualWindowRateLimit 的理由）。
var ReportSubmitQuotas = []Quota{{Limit: 5, Window: 10 * time.Minute}, {Limit: 10, Window: time.Hour}}

// Reports 是 SellerReportAppService 的 handler。
type Reports struct {
	m       ReportModule
	buyers  Buyers
	limiter Limiter
}

var _ sellerv1alpha1connect.SellerReportAppServiceHandler = (*Reports)(nil)

// NewReports 建立通報入口的 handler；limiter 是登入通報的帳號額度，必須提供（與賣家宣告表的 limitedTo 同一個 limiter）。
func NewReports(m ReportModule, buyers Buyers, limiter Limiter) (*Reports, error) {
	if limiter == nil {
		return nil, errors.New("httpapi/seller：通報的帳號額度需要 limiter")
	}
	return &Reports{m: m, buyers: buyers, limiter: limiter}, nil
}

// countSubmit 為帳號送出通報計一次：兩個時間窗全部在額度內才一起計，任何一個超過就回 resource_exhausted（帶 RetryInfo），
// 都不計。
func (h *Reports) countSubmit(accountID int64) error {
	limits := make([]Limit, len(ReportSubmitQuotas))
	for i, q := range ReportSubmitQuotas {
		limits[i] = Limit{
			Key:   "report|" + sellerv1alpha1connect.SellerReportAppServiceCreateReportProcedure + "|" + q.Window.String() + "|" + strconv.FormatInt(accountID, 10),
			Quota: q.Limit, Window: q.Window,
		}
	}
	if ok, wait := h.limiter.Allow(limits...); !ok {
		return apperr.ResourceExhausted(wait, "檢舉太頻繁，請稍後再試")
	}
	return nil
}

var createReportPaths = rpc.FieldPaths{"EvidenceTokens[]": "evidence_staged_tokens[]"}

// callerBuyer 是登入的帳號與它目前的買家；買家不存在是不變量被破壞（存活的帳號都是買家），沒有分類（internal）。
func (h *Reports) callerBuyer(ctx context.Context) (accountID, buyerID int64, err error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return 0, 0, err
	}
	buyerID, err = h.buyers.CurrentBuyerID(ctx, p.ID)
	return p.ID, buyerID, rpc.BuyerInvariant(p.ID, err)
}

func toMyReport(r model.Report) *pb.MyReport {
	out := &pb.MyReport{
		Id: r.ID.String(), StoreId: rpc.FormatID(r.StoreID), ProductId: deref(r.ProductID), OrderNo: deref(r.OrderNo),
		Category: rpc.EnumToProto[commonpb.PolicyCategory](r.Category), Description: r.Description,
		ResolutionNote: deref(r.ResolutionNote), CreateTime: timestamppb.New(r.CreatedAt),
	}
	if r.ResolvedAt != nil {
		out.ResolveTime = timestamppb.New(*r.ResolvedAt)
	}
	if r.Outcome != nil {
		out.Outcome = rpc.EnumToProto[pb.ReportOutcome](*r.Outcome)
	}
	return out
}

// CreateReport 以呼叫者的買家身分檢舉。去重期間內同一個買家已經檢舉過、而且還沒有結果（同店、同類別、同一個台北日）時，這次的內容補充到原本那一筆，回成功：
// supplemented 為 true，report 是原本那一筆（去重的鍵含買家，所以一定是呼叫者自己的，不洩漏別人的資料），notice 說明發生了什麼；原本那一筆已有 10 則補充時
// 模組回 failed_precondition。原本那一筆已經有結果時，這次是新的檢舉。店家屬於呼叫者的帳號時模組回 invalid_argument（store_id）。
func (h *Reports) CreateReport(ctx context.Context, req *connect.Request[pb.CreateReportRequest]) (*connect.Response[pb.CreateReportResponse], error) {
	accountID, buyerID, err := h.callerBuyer(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.countSubmit(accountID); err != nil {
		return nil, err
	}
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	result, err := h.m.SubmitReport(ctx, model.SubmitReportInput{
		StoreID: storeID, ProductID: msg.GetProductId(), OrderNo: msg.GetOrderNo(),
		Category: rpc.EnumFromProto[policy.Category](msg.GetCategory()), Description: msg.GetDescription(),
		ReporterBuyerID: buyerID, AccountID: accountID, EvidenceTokens: msg.GetEvidenceStagedTokens(),
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, createReportPaths)
	}
	if result.Report == nil {
		// 送出一定帶回那一筆（新的，或補充到的原本那一筆）；沒有是模組的不變量被破壞，沒有分類（internal）。
		return nil, fmt.Errorf("httpapi/seller：登入的檢舉沒有帶回通報 %s", result.ReportID)
	}
	out := &pb.CreateReportResponse{Report: toMyReport(*result.Report), Supplemented: result.Supplemented}
	if result.Supplemented {
		out.Notice = rpc.LocalizedMessage(result.Notice)
	}
	return connect.NewResponse(out), nil
}

func (h *Reports) ListMyReports(ctx context.Context, req *connect.Request[pb.ListMyReportsRequest]) (*connect.Response[pb.ListMyReportsResponse], error) {
	_, buyerID, err := h.callerBuyer(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(sellerv1alpha1connect.SellerReportAppServiceListMyReportsProcedure, buyerID)
	after, err := rpc.ValidCursor[model.ReportCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.BuyerReports(ctx, model.BuyerReportListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &pb.ListMyReportsResponse{Reports: make([]*pb.MyReport, len(page.Items))}
	for i, r := range page.Items {
		out.Reports[i] = toMyReport(r)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}
