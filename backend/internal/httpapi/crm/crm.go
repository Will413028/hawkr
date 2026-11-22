// Package crm 是 crm 模組的 Connect handler：店家的客戶與標籤（hawkr.crm.v1alpha1.CrmAppService）。這裡只做欄位的轉換、店家身分與能力的檢查
// 與顯示資料的組裝（暱稱與頭像來自 buyer、圖片網址由 rpc.Images 組成），規則在 internal/modules/crm。
package crm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/crm/v1alpha1/crmv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Reads 是客戶與標籤的讀取來源（*crm.Module）。
type Reads interface {
	ListCustomers(ctx context.Context, in crm.CustomerListInput) (crm.CustomerPage, error)
	GetCustomer(ctx context.Context, storeID, buyerID int64) (crm.Customer, error)
	ListTags(ctx context.Context, storeID int64) ([]crm.Tag, error)
}

// Writes 是客戶與標籤的寫入來源（*crm.Module）。
type Writes interface {
	SetCustomerType(ctx context.Context, storeID, buyerID int64, t crm.CustomerType) error
	CreateTag(ctx context.Context, storeID int64, name string) (crm.Tag, error)
	RenameTag(ctx context.Context, storeID int64, tagID uuid.UUID, name string) error
	DeleteTag(ctx context.Context, storeID int64, tagID uuid.UUID) error
	AddCustomerTags(ctx context.Context, storeID, buyerID int64, names []string) error
	RemoveCustomerTag(ctx context.Context, storeID, buyerID int64, tagID uuid.UUID) error
}

// Buyers 是客戶顯示資料的來源（*buyer.Module）。
type Buyers interface {
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
}

// Stores 是呼叫者目前的店與賣家能力的來源（*seller.Module）。
type Stores interface {
	ActingStore(ctx context.Context, accountID int64, c seller.Capability) (seller.Store, error)
}

// App 實作 CrmAppService。
type App struct {
	reads  Reads
	writes Writes
	buyers Buyers
	stores Stores
	images rpc.Images
	log    *slog.Logger
	// reasonOf 取出模組錯誤帶的業務原因；正式組裝是 crm.ReasonOf，測試換成固定的原因（模組的原因型別不能在模組外建構）。
	reasonOf func(error) crm.Reason
}

var _ crmv1alpha1connect.CrmAppServiceHandler = (*App)(nil)

// NewApp 建立店家的客戶 handler。能力宣告表（declarations）與 CrmAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(reads Reads, writes Writes, buyers Buyers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	return newApp(declarations, reads, writes, buyers, stores, images, log)
}

// newApp 的宣告表由參數給（測試用缺一列的表驗證啟動檢查）。
func newApp(table map[string]seller.Capability, reads Reads, writes Writes, buyers Buyers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	switch {
	case log == nil:
		return nil, fmt.Errorf("httpapi/crm：需要 logger")
	case reads == nil || writes == nil:
		return nil, fmt.Errorf("httpapi/crm：需要 Reads 與 Writes")
	case buyers == nil:
		return nil, fmt.Errorf("httpapi/crm：需要 Buyers")
	case stores == nil:
		return nil, fmt.Errorf("httpapi/crm：需要 Stores")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{reads: reads, writes: writes, buyers: buyers, stores: stores, images: images, log: log, reasonOf: crm.ReasonOf}, nil
}

// authorize 檢查呼叫者是 store_id 這家店的店主、並具備 procedure 宣告的賣家能力，回傳店家 ID。沒有登入由 interceptor 擋，管理員回 permission_denied；
// 呼叫者沒有店回 not_found，停權與前置條件沒有滿足由 seller.ActingStore 回報；店不是呼叫者的回 permission_denied（店家 ID 是公開的，不需要反列舉）。
func (h *App) authorize(ctx context.Context, procedure, rawStoreID string) (int64, error) {
	storeID, err := rpc.ParseID("store_id", rawStoreID)
	if err != nil {
		return 0, err
	}
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return 0, err
	}
	st, err := h.stores.ActingStore(ctx, p.ID, declarations[procedure])
	if err != nil {
		return 0, err
	}
	if st.StoreID != storeID {
		return 0, apperr.PermissionDenied("不是你的店")
	}
	return storeID, nil
}

// reasons 是模組的業務原因對到契約的 ErrorReason（apperr.Reason）：只有 client 會依它分支的才對應，找不到類不帶原因。
var reasons = map[crm.Reason]apperr.Reason{
	crm.ReasonTagNameTaken: apperr.ReasonCrmTagNameTaken,
	crm.ReasonTagLimit:     apperr.ReasonCrmTagLimit,
}

// failure 把模組的錯誤轉成對外的錯誤：業務原因附上契約的 ErrorReason，欄位對到請求路徑 paths。
func (h *App) failure(err error, paths rpc.FieldPaths) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if r, ok := reasons[h.reasonOf(err)]; ok && errors.As(err, &ae) {
		ae.Reason = r
	}
	return rpc.WithFieldPaths(err, paths)
}

var (
	listPaths    = rpc.FieldPaths{"CustomerType": "type"}
	updatePaths  = rpc.FieldPaths{"CustomerType": "customer.type"}
	createPaths  = rpc.FieldPaths{"Name": "name"}
	renamePaths  = rpc.FieldPaths{"Name": "tag.name"}
	addTagsPaths = rpc.FieldPaths{"Names[]": "tag_names[]"}
	noFieldPaths = rpc.FieldPaths{}
)

// cursor 是客戶頁的 page token 內容：上一頁最後一列的排序鍵與買家 ID（排序與方向在 page token 的 scope）。
type cursor struct {
	Number  int64     `json:"n"`
	At      time.Time `json:"a"`
	BuyerID int64     `json:"b"`
}

func (c cursor) Valid() bool { return c.BuyerID > 0 }

func decodeCursor(scope, token string) (*crm.CustomerCursor, error) {
	c, err := rpc.ValidCursor[cursor](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	return &crm.CustomerCursor{Number: c.Number, At: c.At, BuyerID: c.BuyerID}, nil
}

func nextToken(scope string, next *crm.CustomerCursor) string {
	if next == nil {
		return ""
	}
	return rpc.TokenFor(scope, cursor{Number: next.Number, At: next.At, BuyerID: next.BuyerID})
}

// sortOf 把請求的排序欄位轉成模組的排序；UNSPECIFIED 是預設的首筆訂單時間，契約沒有的數字回 invalid_argument。
func sortOf(s pb.CustomerSort) (crm.SortBy, error) {
	switch s {
	case pb.CustomerSort_CUSTOMER_SORT_UNSPECIFIED, pb.CustomerSort_CUSTOMER_SORT_FIRST_ORDER_TIME:
		return crm.SortFirstOrderAt, nil
	case pb.CustomerSort_CUSTOMER_SORT_TOTAL_SPENT:
		return crm.SortTotalSpent, nil
	case pb.CustomerSort_CUSTOMER_SORT_ORDER_COUNT:
		return crm.SortOrderCount, nil
	}
	return 0, rpc.RequiredField("sort", "sort 不是已知的排序欄位")
}

func (h *App) ListCustomers(ctx context.Context, req *connect.Request[pb.ListCustomersRequest]) (*connect.Response[pb.ListCustomersResponse], error) {
	msg := req.Msg
	const procedure = crmv1alpha1connect.CrmAppServiceListCustomersProcedure
	storeID, err := h.authorize(ctx, procedure, msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	in := crm.CustomerListInput{StoreID: storeID, Desc: msg.GetDescending(), Size: msg.GetPageSize()}
	typeKey, tagKey := "", ""
	if msg.Type != nil {
		t := rpc.EnumFromProto[crm.CustomerType](msg.GetType())
		if !t.Valid() {
			return nil, rpc.RequiredField("type", "type 必須是 NORMAL 或 VIP")
		}
		in.Type, typeKey = &t, string(t)
	}
	if msg.TagId != nil {
		id, err := rpc.ParseUUID("tag_id", msg.GetTagId())
		if err != nil {
			return nil, err
		}
		in.TagID, tagKey = &id, id.String()
	}
	if in.Sort, err = sortOf(msg.GetSort()); err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(procedure, rpc.FormatID(storeID), typeKey, tagKey, strconv.Itoa(int(in.Sort)), strconv.FormatBool(in.Desc))
	if in.After, err = decodeCursor(scope, msg.GetPageToken()); err != nil {
		return nil, err
	}
	page, err := h.reads.ListCustomers(ctx, in)
	if err != nil {
		return nil, h.failure(err, listPaths)
	}
	customers, err := h.toCustomers(ctx, page.Customers)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListCustomersResponse{Customers: customers, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *App) GetCustomer(ctx context.Context, req *connect.Request[pb.GetCustomerRequest]) (*connect.Response[pb.GetCustomerResponse], error) {
	storeID, buyerID, err := h.customerKey(ctx, crmv1alpha1connect.CrmAppServiceGetCustomerProcedure, req.Msg.GetStoreId(), req.Msg.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	c, err := h.customer(ctx, storeID, buyerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetCustomerResponse{Customer: c}), nil
}

func (h *App) UpdateCustomer(ctx context.Context, req *connect.Request[pb.UpdateCustomerRequest]) (*connect.Response[pb.UpdateCustomerResponse], error) {
	msg := req.Msg
	storeID, err := h.authorize(ctx, crmv1alpha1connect.CrmAppServiceUpdateCustomerProcedure, msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	c := msg.GetCustomer()
	if c == nil {
		return nil, rpc.RequiredField("customer", "customer 必填")
	}
	buyerID, err := rpc.ParseID("customer.buyer_id", c.GetBuyerId())
	if err != nil {
		return nil, err
	}
	paths, err := rpc.WritableFields(c, msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	if !slices.Equal(paths, []string{"type"}) {
		return nil, rpc.RequiredField("update_mask", "update_mask 必須列 type（分類是唯一可以修改的欄位）")
	}
	if err := h.writes.SetCustomerType(ctx, storeID, buyerID, rpc.EnumFromProto[crm.CustomerType](c.GetType())); err != nil {
		return nil, h.failure(err, updatePaths)
	}
	out, err := h.customer(ctx, storeID, buyerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateCustomerResponse{Customer: out}), nil
}

func (h *App) CreateTag(ctx context.Context, req *connect.Request[pb.CreateTagRequest]) (*connect.Response[pb.CreateTagResponse], error) {
	storeID, err := h.authorize(ctx, crmv1alpha1connect.CrmAppServiceCreateTagProcedure, req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	t, err := h.writes.CreateTag(ctx, storeID, req.Msg.GetName())
	if err != nil {
		return nil, h.failure(err, createPaths)
	}
	return connect.NewResponse(&pb.CreateTagResponse{Tag: toTag(t)}), nil
}

func (h *App) ListTags(ctx context.Context, req *connect.Request[pb.ListTagsRequest]) (*connect.Response[pb.ListTagsResponse], error) {
	storeID, err := h.authorize(ctx, crmv1alpha1connect.CrmAppServiceListTagsProcedure, req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	tags, err := h.reads.ListTags(ctx, storeID)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	return connect.NewResponse(&pb.ListTagsResponse{Tags: toTags(tags)}), nil
}

func (h *App) UpdateTag(ctx context.Context, req *connect.Request[pb.UpdateTagRequest]) (*connect.Response[pb.UpdateTagResponse], error) {
	msg := req.Msg
	storeID, err := h.authorize(ctx, crmv1alpha1connect.CrmAppServiceUpdateTagProcedure, msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	t := msg.GetTag()
	if t == nil {
		return nil, rpc.RequiredField("tag", "tag 必填")
	}
	id, err := rpc.ParseUUID("tag.id", t.GetId())
	if err != nil {
		return nil, err
	}
	paths, err := rpc.WritableFields(t, msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	if !slices.Equal(paths, []string{"name"}) {
		return nil, rpc.RequiredField("update_mask", "update_mask 必須列 name（名稱是唯一可以修改的欄位）")
	}
	if err := h.writes.RenameTag(ctx, storeID, id, t.GetName()); err != nil {
		return nil, h.failure(err, renamePaths)
	}
	tags, err := h.reads.ListTags(ctx, storeID)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	i := slices.IndexFunc(tags, func(x crm.Tag) bool { return x.ID == id })
	if i < 0 {
		return nil, apperr.NotFound("找不到這個標籤")
	}
	return connect.NewResponse(&pb.UpdateTagResponse{Tag: toTag(tags[i])}), nil
}

func (h *App) DeleteTag(ctx context.Context, req *connect.Request[pb.DeleteTagRequest]) (*connect.Response[pb.DeleteTagResponse], error) {
	storeID, err := h.authorize(ctx, crmv1alpha1connect.CrmAppServiceDeleteTagProcedure, req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("tag_id", req.Msg.GetTagId())
	if err != nil {
		return nil, err
	}
	if err := h.writes.DeleteTag(ctx, storeID, id); err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	return connect.NewResponse(&pb.DeleteTagResponse{}), nil
}

func (h *App) AddCustomerTags(ctx context.Context, req *connect.Request[pb.AddCustomerTagsRequest]) (*connect.Response[pb.AddCustomerTagsResponse], error) {
	msg := req.Msg
	storeID, buyerID, err := h.customerKey(ctx, crmv1alpha1connect.CrmAppServiceAddCustomerTagsProcedure, msg.GetStoreId(), msg.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	if err := h.writes.AddCustomerTags(ctx, storeID, buyerID, msg.GetTagNames()); err != nil {
		return nil, h.failure(err, addTagsPaths)
	}
	c, err := h.customer(ctx, storeID, buyerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.AddCustomerTagsResponse{Customer: c}), nil
}

func (h *App) RemoveCustomerTag(ctx context.Context, req *connect.Request[pb.RemoveCustomerTagRequest]) (*connect.Response[pb.RemoveCustomerTagResponse], error) {
	msg := req.Msg
	storeID, buyerID, err := h.customerKey(ctx, crmv1alpha1connect.CrmAppServiceRemoveCustomerTagProcedure, msg.GetStoreId(), msg.GetBuyerId(), "buyer_id")
	if err != nil {
		return nil, err
	}
	tagID, err := rpc.ParseUUID("tag_id", msg.GetTagId())
	if err != nil {
		return nil, err
	}
	if err := h.writes.RemoveCustomerTag(ctx, storeID, buyerID, tagID); err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	c, err := h.customer(ctx, storeID, buyerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RemoveCustomerTagResponse{Customer: c}), nil
}

// customerKey 檢查店家身分與能力並解析買家 ID；欄位格式錯誤先於身分檢查，不查任何資料。
func (h *App) customerKey(ctx context.Context, procedure, rawStoreID, rawBuyerID, buyerField string) (storeID, buyerID int64, err error) {
	if buyerID, err = rpc.ParseID(buyerField, rawBuyerID); err != nil {
		return 0, 0, err
	}
	if storeID, err = h.authorize(ctx, procedure, rawStoreID); err != nil {
		return 0, 0, err
	}
	return storeID, buyerID, nil
}

// customer 讀一位客戶並組成回應；這家店沒有這位客戶回 not_found。
func (h *App) customer(ctx context.Context, storeID, buyerID int64) (*pb.Customer, error) {
	c, err := h.reads.GetCustomer(ctx, storeID, buyerID)
	if err != nil {
		return nil, h.failure(err, noFieldPaths)
	}
	out, err := h.toCustomers(ctx, []crm.Customer{c})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

func toTag(t crm.Tag) *pb.Tag { return &pb.Tag{Id: t.ID.String(), Name: t.Name} }

func toTags(in []crm.Tag) []*pb.Tag {
	out := make([]*pb.Tag, len(in))
	for i, t := range in {
		out[i] = toTag(t)
	}
	return out
}

// toCustomers 把一頁客戶轉成回應：暱稱與頭像以批次查詢補上；buyer 沒有回這位買家的資料（買家已不在）當成已刪除。
func (h *App) toCustomers(ctx context.Context, in []crm.Customer) ([]*pb.Customer, error) {
	ids := make([]int64, 0, len(in))
	for _, c := range in {
		if !slices.Contains(ids, c.BuyerID) {
			ids = append(ids, c.BuyerID)
		}
	}
	displays := make(map[int64]buyer.Display, len(ids))
	for start := 0; start < len(ids); start += buyer.MaxBatchSize {
		rows, err := h.buyers.BatchDisplay(ctx, ids[start:min(start+buyer.MaxBatchSize, len(ids))])
		if err != nil {
			return nil, err
		}
		for _, d := range rows {
			displays[d.BuyerID] = d
		}
	}
	out := make([]*pb.Customer, len(in))
	for i, c := range in {
		v, err := h.toCustomer(c, displays[c.BuyerID])
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (h *App) toCustomer(c crm.Customer, d buyer.Display) (*pb.Customer, error) {
	typ := rpc.EnumToProto[pb.CustomerType](c.Type)
	if typ == pb.CustomerType_CUSTOMER_TYPE_UNSPECIFIED {
		return nil, fmt.Errorf("httpapi/crm：客戶 %d 的分類 %q 不在契約的列舉", c.BuyerID, c.Type)
	}
	display := &pb.CustomerDisplay{Deleted: d.BuyerID == 0 || d.Deleted}
	if !display.Deleted {
		display.Nickname = d.DisplayName
		if d.Avatar != nil {
			if err := rpc.ObjectKey(*d.Avatar); err != nil {
				return nil, err
			}
			display.Avatar = h.images.Image(*d.Avatar)
		}
	}
	return &pb.Customer{
		BuyerId:        rpc.FormatID(c.BuyerID),
		Display:        display,
		Type:           typ,
		TotalSpent:     c.TotalSpent,
		OrderCount:     c.OrderCount,
		FirstOrderTime: timestamppb.New(c.FirstOrderAt),
		Tags:           toTags(c.Tags),
	}, nil
}
