// Package moderation 是 ModerationAppService 的 handler：登入的買家送出檢舉與讀自己的檢舉，店家讀自己送出的聊天訊息檢舉。
//
// 檢舉人由伺服器推導，請求不帶：對聊天訊息是呼叫者在該訊息所屬對話的哪一方（買家方或呼叫者此刻的店），其他對象一律是呼叫者的買家身分；管理員等非顧客帳號的身分回 permission_denied。回應不露案件 ID（A7-D28 第 4 點）。
// 停權的帳號可以讀自己的檢舉、不能送出（動作性質宣告在 internal/httpapi/effects.go）。
package moderation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	sellermod "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Reports 是檢舉的來源（*moderation.Module）。
type Reports interface {
	SubmitReport(ctx context.Context, in moderation.SubmitInput) (moderation.SubmitResult, error)
	MyReports(ctx context.Context, buyerID int64, after *moderation.ReportCursor, size int32) (moderation.ReportPage, error)
	MyStoreReports(ctx context.Context, storeID int64, after *moderation.ReportCursor, size int32) (moderation.ReportPage, error)
}

// StoreReports 是店家檢舉的來源，與呼叫者此刻的店的來源（*seller.Module）。
type StoreReports interface {
	CurrentStore(ctx context.Context, accountID int64) (sellermod.Store, error)
	BuyerReports(ctx context.Context, in sellermod.BuyerReportListInput) (sellermod.ReportPage, error)
}

// Buyers 把登入的帳號對到買家身分（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
}

// Messages 是聊天訊息所屬對話的歸屬來源（*im.Module）：只用來推導檢舉人是哪一方，不進任何回應。
type Messages interface {
	MessageParties(ctx context.Context, messageID uuid.UUID) (im.ConversationParties, error)
}

// App 實作 ModerationAppService。
type App struct {
	reports  Reports
	stores   StoreReports
	buyers   Buyers
	messages Messages
	log      *slog.Logger
}

var _ moderationv1alpha1connect.ModerationAppServiceHandler = (*App)(nil)

// NewApp 建立 handler。能力宣告表（declarations）與 ModerationAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(reports Reports, stores StoreReports, buyers Buyers, messages Messages, log *slog.Logger) (*App, error) {
	if err := checkDeclarations(declarations, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{reports: reports, stores: stores, buyers: buyers, messages: messages, log: log}, nil
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
	t, err := target(req.Msg.GetTarget())
	if err != nil {
		return nil, err
	}
	var reporter moderation.SubmitInput
	if t.Kind == moderation.TargetChatMessage {
		reporter, err = h.chatReporter(ctx, t.ContentID)
	} else {
		reporter.ReporterBuyerID, err = rpc.AccountBuyerID(ctx, h.buyers)
	}
	if err != nil {
		return nil, err
	}
	requestID, err := rpc.ParseRequestID(req.Msg.GetRequestId())
	if err != nil {
		return nil, err
	}
	reporter.Target = t
	reporter.Category = rpc.EnumFromProto[policy.Category](req.Msg.GetCategory())
	reporter.Description = req.Msg.GetDescription()
	reporter.RequestID = requestID
	res, err := h.reports.SubmitReport(ctx, reporter)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, submitPaths)
	}
	return connect.NewResponse(&pb.SubmitReportResponse{
		ReportId:   res.Report.ID.String(),
		CreateTime: timestamppb.New(res.Report.CreatedAt),
	}), nil
}

// chatReporter 由訊息所屬對話的歸屬推導檢舉人：買家方是呼叫者的買家身分，店家方是呼叫者此刻的店；兩邊都不是與訊息不存在回同一個 not_found（不洩漏訊息存在）。
// 呼叫者沒有買家身分時只剩店家方的可能；兩邊都是是不變量被破壞（模組擋下自己傳訊息給自己的店）。
func (h *App) chatReporter(ctx context.Context, messageID uuid.UUID) (moderation.SubmitInput, error) {
	acct, err := rpc.RequireAccount(ctx)
	if err != nil {
		return moderation.SubmitInput{}, err
	}
	buyerID, err := h.buyers.CurrentBuyerID(ctx, acct.ID)
	if err != nil && apperr.KindOf(err) != apperr.KindNotFound {
		return moderation.SubmitInput{}, err
	}
	var store *sellermod.Store
	switch st, err := h.stores.CurrentStore(ctx, acct.ID); {
	case err == nil:
		store = &st
	case apperr.KindOf(err) != apperr.KindNotFound:
		return moderation.SubmitInput{}, err
	}
	parties, err := h.messages.MessageParties(ctx, messageID)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return moderation.SubmitInput{}, chatMessageNotFound()
		}
		return moderation.SubmitInput{}, err
	}
	party, err := rpc.ChatPartyOf(buyerID, store, parties)
	switch {
	case errors.Is(err, rpc.ErrBothChatParties):
		h.log.Error("聊天對話的兩方是同一個帳號（不變量被破壞）", "accountID", acct.ID, "messageID", messageID.String())
		return moderation.SubmitInput{}, fmt.Errorf("訊息 %s 所屬對話的買家方與店家方都是帳號 %d（不變量被破壞）", messageID, acct.ID)
	case errors.Is(err, rpc.ErrNotChatParty):
		return moderation.SubmitInput{}, chatMessageNotFound()
	case err != nil:
		return moderation.SubmitInput{}, err
	}
	if party.Kind == im.PartyStore {
		return moderation.SubmitInput{ReporterStoreID: party.Store.StoreID}, nil
	}
	return moderation.SubmitInput{ReporterBuyerID: party.BuyerID}, nil
}

func chatMessageNotFound() error { return apperr.NotFound("找不到這則訊息") }

// 合併清單的每頁筆數：預設 20、上限取兩個來源上限的較小者（合併以兩邊各回 size 筆為前提）。
const (
	defaultPageSize int32 = 20
	maxPageSize           = min(moderation.MyReportsMaxPageSize, sellermod.ReportMaxPageSize)
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

// ListMyStoreReports 列出呼叫者此刻的店以店家身分送出的檢舉（對象只有聊天訊息）：店由 seller.CurrentStore 推導，請求不帶 store_id；沒有店回 not_found。
// 店家退場後沒有此刻的店，所以讀不到（檢舉列留著）。分頁 token 以 (方法、店) 為 scope。
func (h *App) ListMyStoreReports(ctx context.Context, req *connect.Request[pb.ListMyStoreReportsRequest]) (*connect.Response[pb.ListMyStoreReportsResponse], error) {
	acct, err := rpc.RequireAccount(ctx)
	if err != nil {
		return nil, err
	}
	store, err := h.stores.CurrentStore(ctx, acct.ID)
	if err != nil {
		return nil, err
	}
	size, err := pagination.PageSize(req.Msg.GetPageSize(), defaultPageSize, moderation.MyReportsMaxPageSize)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(moderationv1alpha1connect.ModerationAppServiceListMyStoreReportsProcedure, store.StoreID)
	after, err := rpc.ValidCursor[moderation.ReportCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.reports.MyStoreReports(ctx, store.StoreID, after, size)
	if err != nil {
		return nil, err
	}
	out := &pb.ListMyStoreReportsResponse{Reports: make([]*pb.MyReportItem, len(page.Items))}
	for i, it := range page.Items {
		out.Reports[i] = communityItemPB(it)
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, *page.Next)
	}
	return connect.NewResponse(out), nil
}
