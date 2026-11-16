package buyer

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

type merchantCursor struct {
	Spent   int64 `json:"spent"`
	StoreID int64 `json:"store_id"`
}

type spendingCursor struct {
	OrderTime time.Time `json:"order_time"`
	OrderNo   string    `json:"order_no"`
}

func memberLess(a, b MerchantSummary) bool {
	spentA, spentB := a.Spent-a.Refunded, b.Spent-b.Refunded
	if spentA != spentB {
		return spentA > spentB
	}
	return a.StoreID > b.StoreID
}

func (h *App) ListMemberCards(ctx context.Context, req *connect.Request[pb.ListMemberCardsRequest]) (*connect.Response[pb.ListMemberCardsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	pageSize, err := pagination.PageSize(req.Msg.GetPageSize(), 20, 100)
	if err != nil {
		return nil, err
	}
	s := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListMemberCardsProcedure, buyerID)
	after, err := rpc.PageCursor[merchantCursor](s, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if after != nil && (after.StoreID <= 0 || after.Spent < 0) {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	rows, err := h.ports.Orders.MerchantSummary(ctx, buyerID)
	if err != nil {
		return nil, apperr.Unavailable("暫時無法讀取消費摘要")
	}
	seen := map[int64]bool{}
	for _, row := range rows {
		if row.StoreID <= 0 || row.Spent < 0 || row.Refunded < 0 || row.Refunded > row.Spent || seen[row.StoreID] {
			return nil, apperr.Unavailable("消費摘要格式不合法")
		}
		seen[row.StoreID] = true
	}
	sort.Slice(rows, func(i, j int) bool { return memberLess(rows[i], rows[j]) })
	start := 0
	if after != nil {
		for start < len(rows) {
			row := rows[start]
			if row.Spent-row.Refunded < after.Spent || row.Spent-row.Refunded == after.Spent && row.StoreID < after.StoreID {
				break
			}
			start++
		}
	}
	rows = rows[start:]
	hasNext := len(rows) > int(pageSize)
	if hasNext {
		rows = rows[:pageSize]
	}
	storeIDs := make([]int64, len(rows))
	for i, row := range rows {
		storeIDs[i] = row.StoreID
	}
	stores, err := h.storeCards(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	vips, err := h.ports.CRM.VIPStatuses(ctx, buyerID, storeIDs)
	if err != nil {
		return nil, apperr.Unavailable("暫時無法讀取會員狀態")
	}
	out := &pb.ListMemberCardsResponse{MemberCards: make([]*pb.MemberCard, len(rows))}
	for i, row := range rows {
		out.MemberCards[i] = &pb.MemberCard{StoreId: rpc.FormatID(row.StoreID), Store: stores[row.StoreID], TotalSpent: row.Spent - row.Refunded, Vip: vips[row.StoreID]}
	}
	if hasNext {
		last := rows[len(rows)-1]
		out.NextPageToken = rpc.TokenFor(s, merchantCursor{Spent: last.Spent - last.Refunded, StoreID: last.StoreID})
	}
	return connect.NewResponse(out), nil
}

func recordLess(a, b SpendingRecord) bool {
	if !a.OrderTime.Equal(b.OrderTime) {
		return a.OrderTime.After(b.OrderTime)
	}
	return a.OrderNo > b.OrderNo
}

// allSpendingRecords 逐頁讀完舊訂單來源，讓對外清單在單體接手排序前使用穩定的 keyset。
func (h *App) allSpendingRecords(ctx context.Context, buyerID, storeID int64) ([]SpendingRecord, error) {
	// 訂單明細 RPC 沒有 page size 上限；100 是 order-service 其他清單使用的有界批量。
	const batchSize = 100
	var rows []SpendingRecord
	var total int64 = -1
	seen := map[string]bool{}
	for page := 1; ; page++ {
		if page > math.MaxInt/batchSize {
			return nil, apperr.Unavailable("消費紀錄過多")
		}
		part, err := h.ports.Orders.SpendingRecords(ctx, buyerID, storeID, page, batchSize)
		if err != nil {
			return nil, apperr.Unavailable("暫時無法讀取消費紀錄")
		}
		if part.Total < 0 || len(part.Records) > batchSize || total >= 0 && total != part.Total {
			return nil, apperr.Unavailable("消費紀錄頁面不一致")
		}
		total = part.Total
		for _, row := range part.Records {
			if row.OrderNo == "" || row.OrderTime.IsZero() || row.Amount < 0 || row.Refunded < 0 || seen[row.OrderNo] {
				return nil, apperr.Unavailable("消費紀錄格式不合法")
			}
			seen[row.OrderNo] = true
		}
		rows = append(rows, part.Records...)
		if int64(len(rows)) > total || len(part.Records) < batchSize && int64(len(rows)) != total {
			return nil, apperr.Unavailable("消費紀錄頁面不一致")
		}
		if int64(len(rows)) == total {
			return rows, nil
		}
	}
}

func addAmount(total, value int64) (int64, error) {
	if value > math.MaxInt64-total {
		return 0, apperr.Unavailable("消費總額超出範圍")
	}
	return total + value, nil
}

func (h *App) ListSpendingRecords(ctx context.Context, req *connect.Request[pb.ListSpendingRecordsRequest]) (*connect.Response[pb.ListSpendingRecordsResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	pageSize, err := pagination.PageSize(req.Msg.GetPageSize(), 20, 100)
	if err != nil {
		return nil, err
	}
	s := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListSpendingRecordsProcedure, buyerID) + "\x00" + strconv.FormatInt(storeID, 10)
	after, err := rpc.PageCursor[spendingCursor](s, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if after != nil && (after.OrderTime.IsZero() || after.OrderNo == "") {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	rows, err := h.allSpendingRecords(ctx, buyerID, storeID)
	if err != nil {
		return nil, err
	}
	var spent, refunded int64
	for _, row := range rows {
		spent, err = addAmount(spent, row.Amount)
		if err != nil {
			return nil, err
		}
		refunded, err = addAmount(refunded, row.Refunded)
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(rows, func(i, j int) bool { return recordLess(rows[i], rows[j]) })
	start := 0
	if after != nil {
		key := SpendingRecord{OrderNo: after.OrderNo, OrderTime: after.OrderTime}
		for start < len(rows) && !recordLess(key, rows[start]) {
			start++
		}
	}
	rows = rows[start:]
	hasNext := len(rows) > int(pageSize)
	if hasNext {
		rows = rows[:pageSize]
	}
	out := &pb.ListSpendingRecordsResponse{Records: make([]*pb.SpendingRecord, len(rows)), TotalSpent: spent, TotalRefunded: refunded}
	for i, row := range rows {
		out.Records[i] = &pb.SpendingRecord{OrderNumber: row.OrderNo, Amount: row.Amount, RefundedAmount: row.Refunded, CouponApplied: row.CouponApplied, OrderTime: timestamppb.New(row.OrderTime)}
	}
	if hasNext {
		last := rows[len(rows)-1]
		out.NextPageToken = rpc.TokenFor(s, spendingCursor{OrderTime: last.OrderTime, OrderNo: last.OrderNo})
	}
	return connect.NewResponse(out), nil
}
