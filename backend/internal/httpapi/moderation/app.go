// Package moderation 是 ModerationAppService 的 handler：登入的買家送出檢舉與讀自己的檢舉。
//
// 檢舉人一律是呼叫者的買家身分（由伺服器推導，請求不帶）；管理員等非顧客帳號的身分回 permission_denied。回應不露案件 ID（A7-D28 第 4 點）。
// 停權的帳號可以讀自己的檢舉、不能送出（動作性質宣告在 internal/httpapi/effects.go）。
package moderation

import (
	"context"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	sellermod "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Reports 是檢舉的來源（*moderation.Module）。
type Reports interface {
	SubmitReport(ctx context.Context, in moderation.SubmitInput) (moderation.SubmitResult, error)
	MyReports(ctx context.Context, buyerID int64, after *moderation.ReportCursor, size int32) (moderation.ReportPage, error)
}

// StoreReports 是店家檢舉的來源（*seller.Module）。
type StoreReports interface {
	BuyerReports(ctx context.Context, in sellermod.BuyerReportListInput) (sellermod.ReportPage, error)
}

// Buyers 把登入的帳號對到買家身分（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
}

// App 實作 ModerationAppService。
type App struct {
	reports Reports
	stores  StoreReports
	buyers  Buyers
}

var _ moderationv1alpha1connect.ModerationAppServiceHandler = (*App)(nil)

// NewApp 建立 handler。
func NewApp(reports Reports, stores StoreReports, buyers Buyers) *App {
	return &App{reports: reports, stores: stores, buyers: buyers}
}

var submitPaths = rpc.FieldPaths{
	"Target": "target", "Category": "category", "Description": "description", "RequestID": "request_id",
}

// target 把契約的對象轉成模組的 Target；種類沒給或 ID 的格式不對是呼叫方的錯（invalid_argument，指向 target 的欄位）。
func target(t *pb.ReportTarget) (moderation.Target, error) {
	kind := rpc.EnumFromProto[moderation.TargetKind](t.GetKind())
	if kind == "" {
		return moderation.Target{}, rpc.RequiredField("target.kind", "target.kind 必須是有效的對象種類")
	}
	if kind == moderation.TargetBuyer {
		id, err := rpc.ParseID("target.id", t.GetId())
		return moderation.Target{Kind: kind, BuyerID: id}, err
	}
	id, err := rpc.ParseUUID("target.id", t.GetId())
	return moderation.Target{Kind: kind, ContentID: id}, err
}

// targetPB 把模組的對象轉成契約的對象：內容用 UUID，買家用十進位整數字串。
func targetPB(t moderation.Target) *pb.ReportTarget {
	id := t.ContentID.String()
	if t.Kind == moderation.TargetBuyer {
		id = rpc.FormatID(t.BuyerID)
	}
	return &pb.ReportTarget{Kind: rpc.EnumToProto[pb.ReportTargetKind](t.Kind), Id: id}
}

func (h *App) SubmitReport(ctx context.Context, req *connect.Request[pb.SubmitReportRequest]) (*connect.Response[pb.SubmitReportResponse], error) {
	buyerID, err := rpc.AccountBuyerID(ctx, h.buyers)
	if err != nil {
		return nil, err
	}
	t, err := target(req.Msg.GetTarget())
	if err != nil {
		return nil, err
	}
	requestID, err := rpc.ParseRequestID(req.Msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	res, err := h.reports.SubmitReport(ctx, moderation.SubmitInput{
		ReporterBuyerID: buyerID,
		Target:          t,
		Category:        rpc.EnumFromProto[policy.Category](req.Msg.GetCategory()),
		Description:     req.Msg.GetDescription(),
		RequestID:       requestID,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitPaths)
	}
	return connect.NewResponse(&pb.SubmitReportResponse{
		ReportId:   res.Report.ID.String(),
		CreateTime: timestamppb.New(res.Report.CreatedAt),
	}), nil
}

// 合併清單的每頁筆數：預設 20、上限 50。
const (
	defaultPageSize int32 = 20
	maxPageSize     int32 = 50
)

// myReportsCursor 是合併清單的 page token 內容：兩個來源各自的下一頁位置。Done 的來源已經取完（Cursor 不用）；沒有 Done 而 Cursor 為 nil 是還沒讀過（第一頁）。
type myReportsCursor struct {
	Community     *moderation.ReportCursor `json:"c,omitempty"`
	CommunityDone bool                     `json:"cd,omitempty"`
	Store         *sellermod.ReportCursor  `json:"s,omitempty"`
	StoreDone     bool                     `json:"sd,omitempty"`
}

func (c myReportsCursor) Valid() bool {
	return (c.Community == nil || c.Community.Valid()) && (c.Store == nil || c.Store.Valid()) &&
		(!c.CommunityDone || c.Community == nil) && (!c.StoreDone || c.Store == nil)
}

// mergedReport 是合併排序中的一筆：兩個來源各取到的檢舉，依 (create_time 新到舊、社群在前、id 新到舊) 排序。
type mergedReport struct {
	createdAt time.Time
	store     bool
	community moderation.ReportItem
	storeItem sellermod.Report
}

func (m mergedReport) before(o mergedReport) bool {
	if !m.createdAt.Equal(o.createdAt) {
		return m.createdAt.After(o.createdAt)
	}
	return !m.store && o.store
}

// ListMyReports 以 API composition 合併社群與店家兩個來源的檢舉（模組之間不互相依賴，組裝只在這裡）。每頁兩邊各取 size 筆（模組以多取一筆判斷還有沒有下一頁，Next 非 nil 就是還有），
// 合併後取前 size 筆；各來源的游標推進到已輸出的最後一筆，沒輸出到的來源保留原游標。來源只有在「沒有更多（Next 為 nil）而且取到的每一筆都已輸出」時才算取完。
func (h *App) ListMyReports(ctx context.Context, req *connect.Request[pb.ListMyReportsRequest]) (*connect.Response[pb.ListMyReportsResponse], error) {
	buyerID, err := rpc.AccountBuyerID(ctx, h.buyers)
	if err != nil {
		return nil, err
	}
	size, err := pagination.PageSize(req.Msg.GetPageSize(), defaultPageSize, maxPageSize)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(moderationv1alpha1connect.ModerationAppServiceListMyReportsProcedure, buyerID)
	cur, err := rpc.ValidCursor[myReportsCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if cur == nil {
		cur = &myReportsCursor{}
	}
	var community moderation.ReportPage
	if !cur.CommunityDone {
		if community, err = h.reports.MyReports(ctx, buyerID, cur.Community, size); err != nil {
			return nil, err
		}
	}
	var stores sellermod.ReportPage
	if !cur.StoreDone {
		if stores, err = h.stores.BuyerReports(ctx, sellermod.BuyerReportListInput{BuyerID: buyerID, PageSize: size, After: cur.Store}); err != nil {
			return nil, err
		}
	}
	all := make([]mergedReport, 0, len(community.Items)+len(stores.Items))
	for _, it := range community.Items {
		all = append(all, mergedReport{createdAt: it.Report.CreatedAt, community: it})
	}
	for _, r := range stores.Items {
		all = append(all, mergedReport{createdAt: r.CreatedAt, store: true, storeItem: r})
	}
	slices.SortStableFunc(all, func(a, b mergedReport) int {
		switch {
		case a.before(b):
			return -1
		case b.before(a):
			return 1
		}
		return 0
	})
	if len(all) > int(size) {
		all = all[:size]
	}
	next := *cur
	out := &pb.ListMyReportsResponse{Reports: make([]*pb.MyReportItem, len(all))}
	emittedCommunity, emittedStore := 0, 0
	for i, m := range all {
		if m.store {
			emittedStore++
			next.Store = &sellermod.ReportCursor{CreatedAt: m.storeItem.CreatedAt, ID: m.storeItem.ID}
			out.Reports[i] = storeItemPB(m.storeItem)
			continue
		}
		emittedCommunity++
		next.Community = &moderation.ReportCursor{CreatedAt: m.community.Report.CreatedAt, ID: m.community.Report.ID}
		out.Reports[i] = communityItemPB(m.community)
	}
	if !cur.CommunityDone && community.Next == nil && emittedCommunity == len(community.Items) {
		next.CommunityDone, next.Community = true, nil
	}
	if !cur.StoreDone && stores.Next == nil && emittedStore == len(stores.Items) {
		next.StoreDone, next.Store = true, nil
	}
	if !next.CommunityDone || !next.StoreDone {
		out.NextPageToken = rpc.TokenFor(scope, next)
	}
	return connect.NewResponse(out), nil
}

func communityItemPB(it moderation.ReportItem) *pb.MyReportItem {
	item := &pb.MyReportItem{
		Id:          it.Report.ID.String(),
		Category:    rpc.EnumToProto[commonpb.PolicyCategory](it.Report.Category),
		Description: it.Report.Description,
		CreateTime:  timestamppb.New(it.Report.CreatedAt),
		Subject: &pb.MyReportItem_Community{Community: &pb.CommunityReport{
			Target:  targetPB(it.Target),
			Outcome: rpc.EnumToProto[pb.ReportOutcome](it.Report.Outcome),
		}},
	}
	if it.ClosedAt != nil {
		item.ResolveTime = timestamppb.New(*it.ClosedAt)
	}
	return item
}

func storeItemPB(r sellermod.Report) *pb.MyReportItem {
	store := httpseller.ToStoreReport(r)
	return &pb.MyReportItem{
		Id: store.GetId(), Category: store.GetCategory(), Description: store.GetDescription(),
		CreateTime: store.GetCreateTime(), ResolveTime: store.GetResolveTime(),
		Subject: &pb.MyReportItem_Store{Store: store},
	}
}
