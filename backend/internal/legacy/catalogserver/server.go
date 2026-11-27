// Package catalogserver 在 Java cart 與 order 改寫成 Go 之前，以舊的 gRPC 契約把它們對 product-service 的呼叫轉給 catalog 模組：
// cart 的 GetProductSimple、BatchGetSkuCartInfo，order 的 DeductStock、RestoreStock、BatchGetProductForCheckout。其餘 ProductService 方法一律 Unimplemented
// （GetStock 與沒人呼叫的方法不轉接；買家、賣家、後台的入口已經是 Connect）。本套件隨 Java 的呼叫端刪除。
//
// 契約與舊 product-service 相同（庫存操作用 status 字串、其餘用 code／message 的 envelope，gRPC 狀態是 OK），差異：
//   - 商品與 SKU 的 ID 是字串 UUID，不是 TSID 數字字串。
//   - 配送溫層、配送方式、重量與尺寸不在 catalog（D10）：delivery_temps、shipping_methods、temperature、weight、unit、length_cm、width_cm、height_cm 不填。
//   - 價格：price 是實際成交單價（catalog 的售價），list_price 是原價（有 compare_at_price 用它，否則同售價），sale_price 只有在有原價時才有值。
//   - 扣減與歸還的冪等鍵是 stock_tx_id：扣減用它本身，歸還用 RestoreKey(stock_tx_id) 並指向原扣減；重送回 SUCCESS（catalog 不區分第一次與重送，舊的 ALREADY_APPLIED 兩邊的呼叫端處理相同）。
//   - 歸還找不到原扣減回 NOOP_NOT_FOUND（舊契約：視為補償成功）；歸還超過原扣減量，或同一個 stock_tx_id 以不同內容重送，回 SYSTEM_ERROR（舊的程式不檢查，直接歸還）。
package catalogserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/product/productconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

const (
	codeOK           = 0
	codeInvalidParam = 400
	codeNotFound     = 404

	statusSuccess           = "SUCCESS"
	statusInsufficientStock = "INSUFFICIENT_STOCK"
	statusNoopNotFound      = "NOOP_NOT_FOUND"
	statusSystemError       = "SYSTEM_ERROR"

	maxCheckoutSkus = 100
	maxCartSkus     = 200
)

// RestoreKey 是歸還操作的冪等鍵：同一筆訂單的扣減與歸還共用 stock_tx_id，catalog 的結帳操作鍵全域唯一，所以歸還加前綴。
// order.cancelled 的 consumer 用同一個函式，與 gRPC 的 RestoreStock 重疊時只歸還一次。
func RestoreKey(stockTxID string) string { return "restore:" + stockTxID }

// Catalog 是 server 需要的 catalog 模組方法（*catalog.Module 滿足）。
type Catalog interface {
	DeductStock(ctx context.Context, key string, items []catalog.StockItem) error
	RestoreStock(ctx context.Context, key, deductKey string, items []catalog.StockItem) error
	CheckoutSnapshot(ctx context.Context, skuIDs []uuid.UUID) (map[uuid.UUID]catalog.SkuSnapshot, error)
	SkuCartInfo(ctx context.Context, skuIDs []uuid.UUID) (map[uuid.UUID]catalog.SkuSnapshot, error)
}

// Server 實作 legacy 的 ProductService 中 cart 與 order 呼叫的方法。
type Server struct {
	productconnect.UnimplementedProductServiceHandler
	catalog Catalog
	log     *slog.Logger
}

var _ productconnect.ProductServiceHandler = (*Server)(nil)

// New 建立 server。
func New(c Catalog, log *slog.Logger) *Server {
	if c == nil {
		panic("catalogserver: missing dependency")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Server{catalog: c, log: log}
}

func stockResponse(status, msg string, insufficient ...*pb.InsufficientItem) *connect.Response[pb.StockOperationResponse] {
	return connect.NewResponse(&pb.StockOperationResponse{Status: status, Message: msg, InsufficientItems: insufficient})
}

func parseItems(in []*pb.StockItem) ([]catalog.StockItem, string) {
	out := make([]catalog.StockItem, 0, len(in))
	for _, it := range in {
		id, err := uuid.Parse(it.GetSkuId())
		if err != nil || id == uuid.Nil() {
			return nil, fmt.Sprintf("sku_id %q 不是 UUID", it.GetSkuId())
		}
		q, err := strconv.ParseInt(it.GetQuantity(), 10, 32)
		if err != nil || q <= 0 {
			return nil, fmt.Sprintf("quantity %q 不是正整數", it.GetQuantity())
		}
		out = append(out, catalog.StockItem{SkuID: id, Quantity: int32(q)})
	}
	return out, ""
}

func checkStockRequest(txID string, items []*pb.StockItem) *connect.Response[pb.StockOperationResponse] {
	switch {
	case txID == "":
		return stockResponse(statusSystemError, "stock_tx_id 為必填欄位")
	case len(items) == 0:
		return stockResponse(statusSystemError, "items 不可為空")
	}
	return nil
}

// DeductStock 以 stock_tx_id 為冪等鍵扣減庫存。庫存不足回 INSUFFICIENT_STOCK 與不足的明細（requested、available）；SKU 不存在或已刪除（舊的程式也當庫存不足、available 為 0）
// 列出那些 SKU。參數格式錯誤、同一個 stock_tx_id 以不同內容重送、其他錯誤回 SYSTEM_ERROR，order 以 60 秒後重試處理。
func (s *Server) DeductStock(ctx context.Context, req *connect.Request[pb.DeductStockRequest]) (*connect.Response[pb.StockOperationResponse], error) {
	m := req.Msg
	if invalid := checkStockRequest(m.GetStockTxId(), m.GetItems()); invalid != nil {
		return invalid, nil
	}
	items, bad := parseItems(m.GetItems())
	if bad != "" {
		return stockResponse(statusSystemError, "參數格式錯誤: "+bad), nil
	}
	err := s.catalog.DeductStock(ctx, m.GetStockTxId(), items)
	var short *catalog.InsufficientStockError
	switch {
	case err == nil:
		return stockResponse(statusSuccess, "庫存扣減成功"), nil
	case errors.As(err, &short):
		return s.insufficient(ctx, err, items, func(it catalog.StockItem, _ catalog.SkuSnapshot, _ bool) bool { return it.SkuID == short.SkuID }), nil
	case errors.Is(err, catalog.ErrSkuNotFound):
		return s.insufficient(ctx, err, items, func(_ catalog.StockItem, snap catalog.SkuSnapshot, found bool) bool { return !found || snap.Deleted }), nil
	case errors.Is(err, catalog.ErrInvalidStockInput), errors.Is(err, catalog.ErrIdempotencyKeyReused):
		s.log.Warn("catalogserver：DeductStock 請求不合法", "stock_tx_id", m.GetStockTxId(), "err", err)
		return stockResponse(statusSystemError, "參數格式錯誤: "+err.Error()), nil
	default:
		s.log.Error("catalogserver：DeductStock 失敗", "stock_tx_id", m.GetStockTxId(), "err", err)
		return stockResponse(statusSystemError, "系統錯誤"), nil
	}
}

func (s *Server) insufficient(ctx context.Context, cause error, items []catalog.StockItem, match func(catalog.StockItem, catalog.SkuSnapshot, bool) bool) *connect.Response[pb.StockOperationResponse] {
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.SkuID
	}
	snaps, err := s.catalog.SkuCartInfo(ctx, ids)
	if err != nil {
		s.log.Warn("catalogserver：讀取不足明細的現有庫存失敗，available 以 0 回報", "err", err)
		snaps = nil
	}
	var out []*pb.InsufficientItem
	for _, it := range items {
		snap, found := snaps[it.SkuID]
		if !match(it, snap, found) {
			continue
		}
		available := int32(0)
		if found && !snap.Deleted {
			available = snap.StockQty
		}
		out = append(out, &pb.InsufficientItem{SkuId: it.SkuID.String(), RequestedQty: strconv.Itoa(int(it.Quantity)), AvailableQty: strconv.Itoa(int(available))})
	}
	if len(out) == 0 {
		return stockResponse(statusSystemError, cause.Error())
	}
	return stockResponse(statusInsufficientStock, cause.Error(), out...)
}

// RestoreStock 依 stock_tx_id 指的原扣減歸還庫存，歸還的冪等鍵是 RestoreKey(stock_tx_id)：重送不再歸還。找不到原扣減回 NOOP_NOT_FOUND。
func (s *Server) RestoreStock(ctx context.Context, req *connect.Request[pb.RestoreStockRequest]) (*connect.Response[pb.StockOperationResponse], error) {
	m := req.Msg
	if invalid := checkStockRequest(m.GetStockTxId(), m.GetItems()); invalid != nil {
		return invalid, nil
	}
	items, bad := parseItems(m.GetItems())
	if bad != "" {
		return stockResponse(statusSystemError, "參數格式錯誤: "+bad), nil
	}
	err := s.catalog.RestoreStock(ctx, RestoreKey(m.GetStockTxId()), m.GetStockTxId(), items)
	switch {
	case err == nil:
		return stockResponse(statusSuccess, "庫存歸還成功"), nil
	case errors.Is(err, catalog.ErrDeductionNotFound):
		return stockResponse(statusNoopNotFound, "無對應扣減記錄，視為補償成功"), nil
	case errors.Is(err, catalog.ErrInvalidStockInput), errors.Is(err, catalog.ErrIdempotencyKeyReused), errors.Is(err, catalog.ErrRestoreExceedsDeduction):
		s.log.Warn("catalogserver：RestoreStock 請求不合法", "stock_tx_id", m.GetStockTxId(), "err", err)
		return stockResponse(statusSystemError, "參數格式錯誤: "+err.Error()), nil
	default:
		s.log.Error("catalogserver：RestoreStock 失敗", "stock_tx_id", m.GetStockTxId(), "err", err)
		return stockResponse(statusSystemError, "系統錯誤"), nil
	}
}

func listPrice(s catalog.SkuSnapshot) int64 {
	if s.CompareAtPrice != nil {
		return *s.CompareAtPrice
	}
	return s.Price
}

// GetProductSimple 讀一個 SKU 的簡化資訊（cart 加入購物車用）。sku_id 空、不是 UUID 或全零回 code=400，SKU 不存在回 404；已下架與已刪除的 SKU 照回，is_active 為假。
// max_limit 是目前庫存。
func (s *Server) GetProductSimple(ctx context.Context, req *connect.Request[pb.GetProductSimpleRequest]) (*connect.Response[pb.GetProductSimpleResponse], error) {
	reply := func(code int32, msg string, data *pb.ProductSimpleInfo) (*connect.Response[pb.GetProductSimpleResponse], error) {
		return connect.NewResponse(&pb.GetProductSimpleResponse{Code: code, Message: msg, Data: data}), nil
	}
	raw := req.Msg.GetSkuId()
	if raw == "" {
		return reply(codeInvalidParam, "sku_id 不可為空", nil)
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil() {
		return reply(codeInvalidParam, "sku_id 不是 UUID", nil)
	}
	snaps, err := s.catalog.SkuCartInfo(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, connectError(err)
	}
	snap, ok := snaps[id]
	if !ok {
		return reply(codeNotFound, "SKU 不存在", nil)
	}
	return reply(codeOK, "success", &pb.ProductSimpleInfo{
		SkuId: id.String(), StoreId: strconv.FormatInt(snap.StoreID, 10), IsActive: snap.Active, Name: snap.ProductTitle,
		Price: strconv.FormatInt(snap.Price, 10), ListPrice: strconv.FormatInt(listPrice(snap), 10),
		MaxLimit: strconv.Itoa(int(snap.StockQty)), ProductId: snap.ProductID.String(), SkuName: snap.SkuName, ImageObjectKey: snap.CoverKey,
	})
}

func parseIDs(raw []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(raw))
	for _, r := range raw {
		if id, err := uuid.Parse(r); err == nil && id != uuid.Nil() && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// BatchGetSkuCartInfo 批次讀 SKU 的即時資訊（購物車列表用，最多 200 個）：回應的 map 以 sku_id 為鍵，查無的 SKU 缺席（呼叫端走 LOOKUP_FAILED）；已刪除與已下架的照回，is_active 為假。
func (s *Server) BatchGetSkuCartInfo(ctx context.Context, req *connect.Request[pb.BatchGetSkuCartInfoRequest]) (*connect.Response[pb.BatchGetSkuCartInfoResponse], error) {
	raw := req.Msg.GetSkuIds()
	if len(raw) > maxCartSkus {
		return connect.NewResponse(&pb.BatchGetSkuCartInfoResponse{Code: codeInvalidParam, Message: "SKU ID 數量不可超過 200"}), nil
	}
	out := &pb.BatchGetSkuCartInfoResponse{Code: codeOK, Message: "查詢成功", Skus: map[string]*pb.SkuCartInfo{}}
	ids := parseIDs(raw)
	if len(ids) == 0 {
		return connect.NewResponse(out), nil
	}
	snaps, err := s.catalog.SkuCartInfo(ctx, ids)
	if err != nil {
		return nil, connectError(err)
	}
	for id, snap := range snaps {
		out.Skus[id.String()] = &pb.SkuCartInfo{
			StockQty: strconv.Itoa(int(snap.StockQty)), Price: strconv.FormatInt(snap.Price, 10),
			ListPrice: strconv.FormatInt(listPrice(snap), 10), IsActive: snap.Active,
		}
	}
	return connect.NewResponse(out), nil
}

// BatchGetProductForCheckout 批次讀結帳用的 SKU 與商品資訊（最多 100 個），接受已下架與已刪除的（is_active 為假），查無的 SKU 缺席。
// 配送方式、重量與尺寸不在 catalog（D10），不填。
func (s *Server) BatchGetProductForCheckout(ctx context.Context, req *connect.Request[pb.BatchGetProductForCheckoutRequest]) (*connect.Response[pb.BatchGetProductForCheckoutResponse], error) {
	raw := req.Msg.GetSkuIds()
	if len(raw) > maxCheckoutSkus {
		return connect.NewResponse(&pb.BatchGetProductForCheckoutResponse{Code: codeInvalidParam}), nil
	}
	out := &pb.BatchGetProductForCheckoutResponse{Code: codeOK, Data: map[string]*pb.ProductForCheckoutInfo{}}
	ids := parseIDs(raw)
	if len(ids) == 0 {
		return connect.NewResponse(out), nil
	}
	snaps, err := s.catalog.CheckoutSnapshot(ctx, ids)
	if err != nil {
		return nil, connectError(err)
	}
	for id, snap := range snaps {
		info := &pb.ProductForCheckoutInfo{
			SkuId: id.String(), ProductId: snap.ProductID.String(), StoreId: strconv.FormatInt(snap.StoreID, 10), IsActive: snap.Active,
			Price: strconv.FormatInt(snap.Price, 10), ListPrice: strconv.FormatInt(listPrice(snap), 10),
			StockQty: strconv.Itoa(int(snap.StockQty)), SkuName: snap.SkuName, ProductName: snap.ProductTitle, ImageObjectKey: snap.CoverKey,
		}
		if snap.CompareAtPrice != nil {
			info.SalePrice = strconv.FormatInt(snap.Price, 10)
		}
		out.Data[id.String()] = info
	}
	return connect.NewResponse(out), nil
}
