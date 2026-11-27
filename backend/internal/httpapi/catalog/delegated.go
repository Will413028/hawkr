package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1/catalogv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// DelegatedDeclaration 是 CatalogDelegatedService 每個方法的代行宣告（API 設計規範（Connect）§二的 Delegated）：需要的代行能力，以及稽核列的動作與資源種類
// （讀取被拒絕時寫 PRODUCT_READ）。key 是生成的 procedure 常數；interceptor（httpapi/delegation）啟動時檢查它與 service 的方法一一對應。
// 寫入方法的能力與動作由該方法發的 catalog 事件種類衍生（delegation.ProductWriteFor，與稽核訂閱者共用，不在這裡再寫一份）。
var DelegatedDeclaration = httpdelegation.Declaration{
	Service: pb.File_hawkr_catalog_v1alpha1_catalog_proto.Services().ByName("CatalogDelegatedService"),
	Methods: map[string]moddelegation.DelegatedMethod{
		catalogv1alpha1connect.CatalogDelegatedServiceListStoreProductsProcedure:           productMethod(moddelegation.CapabilityProductsRead, moddelegation.AuditProductRead),
		catalogv1alpha1connect.CatalogDelegatedServiceGetStoreProductProcedure:             productMethod(moddelegation.CapabilityProductsRead, moddelegation.AuditProductRead),
		catalogv1alpha1connect.CatalogDelegatedServiceCreateStoreProductProcedure:          writeMethod(catalog.ProductCreated{}),
		catalogv1alpha1connect.CatalogDelegatedServiceUpdateStoreProductProcedure:          writeMethod(catalog.ProductUpdated{}),
		catalogv1alpha1connect.CatalogDelegatedServicePublishStoreProductProcedure:         writeMethod(catalog.ProductPublished{}),
		catalogv1alpha1connect.CatalogDelegatedServiceUnpublishStoreProductProcedure:       writeMethod(catalog.ProductUnpublished{}),
		catalogv1alpha1connect.CatalogDelegatedServiceBatchUnpublishStoreProductsProcedure: writeMethod(catalog.ProductsUnpublished{}),
	},
}

func writeMethod(e any) moddelegation.DelegatedMethod {
	w, ok := moddelegation.ProductWriteFor(e)
	if !ok {
		panic(fmt.Sprintf("httpapi/catalog：%T 不是代行會稽核的商品寫入事件", e))
	}
	return productMethod(w.Capability, w.Action)
}

func productMethod(c moddelegation.Capability, a moddelegation.AuditAction) moddelegation.DelegatedMethod {
	return moddelegation.DelegatedMethod{Capability: c, Action: a, Resource: moddelegation.ResourceProduct}
}

// DelegatedModule 是代行讀寫商品用到的 catalog 方法（*catalog.Module）：與賣家同一組方法（店家由 delegation 以 session 的目標店家讀出後交給模組），加上批次下架。
type DelegatedModule interface {
	ListSellerProducts(ctx context.Context, storeID int64, f catalog.SellerFilter, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
	Product(ctx context.Context, storeID int64, productID uuid.UUID) (catalog.Product, error)
	CreateProduct(ctx context.Context, storeID int64, uploader media.Uploader, requestID uuid.UUID, in catalog.ProductInput, actor *delegatedactor.Actor) (catalog.Product, error)
	UpdateProduct(ctx context.Context, storeID int64, uploader media.Uploader, productID uuid.UUID, version int64, in catalog.ProductInput, actor *delegatedactor.Actor) (catalog.Product, error)
	SetProductStatus(ctx context.Context, storeID int64, productID uuid.UUID, version int64, to catalog.ProductStatus, actor *delegatedactor.Actor) error
	UnpublishProducts(ctx context.Context, storeID int64, items []catalog.VersionedProduct, actor *delegatedactor.Actor) error
}

// maxDelegatedPageSize 是代行列表一頁的上限（proto 註解寫的 50）。
const maxDelegatedPageSize = 50

// Delegated 實作 CatalogDelegatedService：只做傳輸（解析請求、組回應）；授權、稽核、冪等與失敗紀錄在 delegation 的 DelegatedRead／DelegatedWrite，商品的規則與寫入在 catalog 模組。
// 店家只來自代行 session（DelegatedRead／DelegatedWrite 交給回呼的 seller.Store），請求沒有 store_id。
type Delegated struct {
	runner httpdelegation.Runner
	m      DelegatedModule
	images rpc.Images
}

var _ catalogv1alpha1connect.CatalogDelegatedServiceHandler = (*Delegated)(nil)

// NewDelegated 建立 handler。
func NewDelegated(runner httpdelegation.Runner, m DelegatedModule, images rpc.Images, log *slog.Logger) (*Delegated, error) {
	switch {
	case log == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 logger")
	case runner == nil:
		return nil, fmt.Errorf("httpapi/catalog：代行需要 delegation")
	case m == nil:
		return nil, fmt.Errorf("httpapi/catalog：需要 catalog 模組")
	}
	return &Delegated{runner: runner, m: m, images: images}, nil
}

var (
	delegatedCreatePaths = merge(inputPaths("product."), rpc.FieldPaths{"RequestID": "operation.request_id"})
	batchPaths           = rpc.FieldPaths{"Items": "items", "Etag": "items[].etag"}
)

// uploaderOf 是代行寫入新圖時確認 staged token 的上傳者：與代行的圖片上傳同一個（httpapi/media 的 withUploader，B1-D21）——session 的管理員，綁被代行的店家。
func uploaderOf(c moddelegation.DelegatedCaller, st seller.Store) media.Uploader {
	return media.Uploader{ID: c.Session.Actor.AdminID, Admin: true, Store: st.StoreID}
}

func (h *Delegated) ListStoreProducts(ctx context.Context, req *connect.Request[pb.ListStoreProductsRequest]) (*connect.Response[pb.ListStoreProductsResponse], error) {
	msg := req.Msg
	if err := checkPageSize(msg.GetPageSize()); err != nil {
		return nil, err
	}
	f, err := sellerFilter(msg.GetFilter())
	if err != nil {
		return nil, err
	}
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(catalogv1alpha1connect.CatalogDelegatedServiceListStoreProductsProcedure, strconv.FormatInt(c.Session.Target.StoreID, 10), string(f.Status), string(f.StockStatus), f.Query)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	size := min(msg.GetPageSize(), maxDelegatedPageSize)
	out := &pb.ListStoreProductsResponse{}
	err = h.runner.DelegatedRead(ctx, c, func(ctx context.Context, st seller.Store) error {
		page, err := h.m.ListSellerProducts(ctx, st.StoreID, f, after, size)
		if err != nil {
			return failure(err, sellerListPaths)
		}
		if out.Products, err = sellerSummaries(h.images, page.Items); err != nil {
			return err
		}
		out.NextPageToken = nextToken(scope, page.Next)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

func (h *Delegated) GetStoreProduct(ctx context.Context, req *connect.Request[pb.GetStoreProductRequest]) (*connect.Response[pb.GetStoreProductResponse], error) {
	id, err := rpc.ParseUUID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	out := &pb.GetStoreProductResponse{}
	err = h.runner.DelegatedRead(ctx, c, func(ctx context.Context, st seller.Store) error {
		p, err := h.m.Product(ctx, st.StoreID, id)
		if err != nil {
			return failure(err, noPaths)
		}
		out.Product, err = sellerProduct(h.images, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(out), nil
}

// delegatedWrite 是每個單一商品的代行寫入共同的流程：交給 delegation 檢查重送與店家，帶著代行者（明確的參數，A7-D27）執行 do（catalog 在自己的 transaction 內發帶代行者的事件，稽核同 transaction 寫入）；
// 商品由 do 回傳（建立、修改），或在 do 之後讀回（切換狀態；重送時 do 沒被呼叫，也讀回第一次寫入的商品目前的樣子）。回傳商品與稽核列 ID。
// do 內的模組錯誤先轉成對外的錯誤（failure）：delegation 以錯誤鏈上的原因寫 FAILED 列（NOT_FOUND、VERSION_CONFLICT…），而不是 INTERNAL。
func (h *Delegated) delegatedWrite(ctx context.Context, w moddelegation.DelegatedWrite, productID uuid.UUID,
	do func(ctx context.Context, c moddelegation.DelegatedCaller, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error)) (*pb.SellerProduct, string, error) {
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, "", err
	}
	var written *catalog.Product
	res, err := h.runner.DelegatedWrite(ctx, c, w, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
		p, err := do(ctx, c, st, actor)
		if err != nil {
			return err
		}
		if p.ID != (uuid.UUID{}) {
			written = &p
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	var p catalog.Product
	switch {
	case written != nil && !res.Replayed:
		p = *written
	default:
		id := productID
		if res.Replayed && res.ResourceID != "" {
			if id, err = uuid.Parse(res.ResourceID); err != nil {
				return nil, "", fmt.Errorf("httpapi/catalog：稽核列 %s 的資源 ID %q：%w", res.AuditLogID, res.ResourceID, err)
			}
		}
		if p, err = h.m.Product(ctx, res.Store.StoreID, id); err != nil {
			return nil, "", failure(err, noPaths)
		}
	}
	out, err := sellerProduct(h.images, p)
	return out, res.AuditLogID.String(), err
}

func (h *Delegated) CreateStoreProduct(ctx context.Context, req *connect.Request[pb.CreateStoreProductRequest]) (*connect.Response[pb.CreateStoreProductResponse], error) {
	msg := req.Msg
	op, err := httpdelegation.ParseOperation(msg.GetOperation())
	if err != nil {
		return nil, err
	}
	if msg.GetProduct() == nil {
		return nil, rpc.RequiredField("product", "product 必填")
	}
	in, err := toInput(msg.GetProduct())
	if err != nil {
		return nil, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg.GetProduct())
	if err != nil {
		return nil, err
	}
	out, auditID, err := h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: op, ResourceName: truncatedTitle(in.Title),
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditProductCreate, raw),
	}, uuid.UUID{}, func(ctx context.Context, c moddelegation.DelegatedCaller, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error) {
		p, err := h.m.CreateProduct(ctx, st.StoreID, uploaderOf(c, st), op.RequestID, in, actor)
		return p, failure(err, delegatedCreatePaths)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateStoreProductResponse{Product: out, AuditLogId: auditID}), nil
}

// updatableProductFields 是 UpdateStoreProduct 的 update_mask 可以列的欄位：ProductInput 最上層的每個欄位。
var updatableProductFields = []string{"title", "category_id", "hashtag_inputs", "gallery", "description", "axes", "skus", "low_stock_threshold"}

// updateMaskPaths 是 update_mask 選到的欄位（由小到大）：至少一個，只能是 ProductInput 最上層的欄位；axes 與 skus 要一起列（SKU 的選項值要對得上選項軸）。
func updateMaskPaths(in *pb.ProductInput, mask *fieldmaskpb.FieldMask) ([]string, error) {
	if len(mask.GetPaths()) == 0 {
		return nil, rpc.RequiredField("update_mask", "update_mask 必須列至少一個可以修改的欄位")
	}
	paths, err := rpc.WritableFields(in, mask)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		if !slices.Contains(updatableProductFields, p) {
			return nil, rpc.RequiredField("update_mask", "update_mask 的 "+p+" 不是可以修改的欄位（可以修改："+joinFields(updatableProductFields)+"）")
		}
	}
	if len(paths) == 0 {
		return nil, rpc.RequiredField("update_mask", "update_mask 必須列至少一個可以修改的欄位")
	}
	if slices.Contains(paths, "axes") != slices.Contains(paths, "skus") {
		return nil, rpc.RequiredField("update_mask", "update_mask 的 axes 與 skus 必須一起列")
	}
	slices.Sort(paths)
	return paths, nil
}

func joinFields(fields []string) string {
	out := ""
	for i, f := range fields {
		if i > 0 {
			out += "、"
		}
		out += f
	}
	return out
}

// maskedBytes 是請求的 ProductInput 只保留 update_mask 選到的欄位後的位元組（決定性編碼）：沒選到的欄位不影響結果，也不該影響「是不是同一個請求」。
func maskedBytes(in *pb.ProductInput, paths []string) ([]byte, error) {
	m := proto.Clone(in).ProtoReflect()
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		if fd := fields.Get(i); !slices.Contains(paths, string(fd.Name())) {
			m.Clear(fd)
		}
	}
	return proto.MarshalOptions{Deterministic: true}.Marshal(m.Interface())
}

// currentInput 是商品目前內容的寫入形狀：沒列在 update_mask 的欄位照它保留。圖片以現有的資產 ID 沿用，SKU 以選項值的組合辨識（既有組合的庫存不經這裡改，StockQty 只在新組合有意義）。
func currentInput(p catalog.Product) catalog.ProductInput {
	in := catalog.ProductInput{
		Title: p.Title, CategoryID: p.CategoryID, LowStockThreshold: p.LowStockThreshold,
		Hashtags: make([]string, len(p.Hashtags)),
	}
	for i, t := range p.Hashtags {
		in.Hashtags[i] = t.Display
	}
	for _, g := range p.Gallery {
		in.Gallery = append(in.Gallery, existingImage(g))
	}
	for _, b := range p.Description {
		if b.Image != nil {
			img := existingImage(*b.Image)
			b.Image = &img
		}
		in.Description = append(in.Description, b)
	}
	in.Axes = slices.Clone(p.Axes)
	for _, s := range p.Skus {
		in.Skus = append(in.Skus, catalog.SkuInput{Option1: s.Option1, Option2: s.Option2, Price: s.Price, CompareAtPrice: s.CompareAtPrice, StockQty: s.StockQty})
	}
	return in
}

func existingImage(i catalog.Image) catalog.Image {
	return catalog.Image{AssetID: i.AssetID, Alt: i.Alt}
}

// mergeInput 以 patch 的 paths 欄位取代 cur（目前的內容）對應的欄位。
func mergeInput(cur catalog.ProductInput, patch catalog.ProductInput, paths []string) catalog.ProductInput {
	has := func(p string) bool { return slices.Contains(paths, p) }
	if has("title") {
		cur.Title = patch.Title
	}
	if has("category_id") {
		cur.CategoryID = patch.CategoryID
	}
	if has("hashtag_inputs") {
		cur.Hashtags = patch.Hashtags
	}
	if has("gallery") {
		cur.Gallery = patch.Gallery
	}
	if has("description") {
		cur.Description = patch.Description
	}
	if has("axes") {
		cur.Axes, cur.Skus = patch.Axes, patch.Skus
	}
	if has("low_stock_threshold") {
		cur.LowStockThreshold = patch.LowStockThreshold
	}
	return cur
}

func (h *Delegated) UpdateStoreProduct(ctx context.Context, req *connect.Request[pb.UpdateStoreProductRequest]) (*connect.Response[pb.UpdateStoreProductResponse], error) {
	msg := req.Msg
	op, err := httpdelegation.ParseOperation(msg.GetOperation())
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("product_id", msg.GetProductId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	if msg.GetProduct() == nil {
		return nil, rpc.RequiredField("product", "product 必填")
	}
	paths, err := updateMaskPaths(msg.GetProduct(), msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	patch, err := toInput(msg.GetProduct())
	if err != nil {
		return nil, err
	}
	raw, err := maskedBytes(msg.GetProduct(), paths)
	if err != nil {
		return nil, err
	}
	out, auditID, err := h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: op, ResourceID: id.String(), Version: &version,
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditProductUpdate, id, version, paths, raw),
	}, id, func(ctx context.Context, c moddelegation.DelegatedCaller, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error) {
		cur, err := h.m.Product(ctx, st.StoreID, id)
		if err != nil {
			return catalog.Product{}, failure(err, noPaths)
		}
		p, err := h.m.UpdateProduct(ctx, st.StoreID, uploaderOf(c, st), id, version, mergeInput(currentInput(cur), patch, paths), actor)
		return p, failure(err, updatePaths)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateStoreProductResponse{Product: out, AuditLogId: auditID}), nil
}

// transition 是上架與下架共同的流程：operation、商品 ID 與必填的 etag，再由模組切換狀態。
func (h *Delegated) transition(ctx context.Context, op *commonpb.DelegatedOperation, productID, etag string, action moddelegation.AuditAction, to catalog.ProductStatus) (*pb.SellerProduct, string, error) {
	o, err := httpdelegation.ParseOperation(op)
	if err != nil {
		return nil, "", err
	}
	id, err := rpc.ParseUUID("product_id", productID)
	if err != nil {
		return nil, "", err
	}
	version, err := rpc.RequiredVersionField(etag, "etag")
	if err != nil {
		return nil, "", err
	}
	return h.delegatedWrite(ctx, moddelegation.DelegatedWrite{
		Op: o, ResourceID: id.String(), Version: &version, PayloadHash: moddelegation.DelegatedPayloadHash(action, id, version),
	}, id, func(ctx context.Context, _ moddelegation.DelegatedCaller, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error) {
		return catalog.Product{}, failure(h.m.SetProductStatus(ctx, st.StoreID, id, version, to, actor), statusPaths)
	})
}

func (h *Delegated) PublishStoreProduct(ctx context.Context, req *connect.Request[pb.PublishStoreProductRequest]) (*connect.Response[pb.PublishStoreProductResponse], error) {
	p, audit, err := h.transition(ctx, req.Msg.GetOperation(), req.Msg.GetProductId(), req.Msg.GetEtag(), moddelegation.AuditProductPublish, catalog.StatusActive)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.PublishStoreProductResponse{Product: p, AuditLogId: audit}), nil
}

func (h *Delegated) UnpublishStoreProduct(ctx context.Context, req *connect.Request[pb.UnpublishStoreProductRequest]) (*connect.Response[pb.UnpublishStoreProductResponse], error) {
	p, audit, err := h.transition(ctx, req.Msg.GetOperation(), req.Msg.GetProductId(), req.Msg.GetEtag(), moddelegation.AuditProductUnpublish, catalog.StatusArchived)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnpublishStoreProductResponse{Product: p, AuditLogId: audit}), nil
}

// batchItems 解析並檢查批次下架的項目：1 到 DelegationMaxBatch 項，商品 ID 不得重複，每項的 etag 必填。回傳請求順序的項目。
func batchItems(in []*pb.BatchUnpublishItem) ([]catalog.VersionedProduct, error) {
	if len(in) == 0 || len(in) > moddelegation.DelegationMaxBatch {
		return nil, rpc.RequiredField("items", fmt.Sprintf("items 必須是 1 到 %d 項", moddelegation.DelegationMaxBatch))
	}
	out := make([]catalog.VersionedProduct, len(in))
	seen := make(map[uuid.UUID]bool, len(in))
	for i, it := range in {
		id, err := rpc.ParseUUID(fmt.Sprintf("items[%d].product_id", i), it.GetProductId())
		if err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, rpc.RequiredField(fmt.Sprintf("items[%d].product_id", i), "items 的商品不得重複")
		}
		seen[id] = true
		version, err := rpc.RequiredVersionField(it.GetEtag(), fmt.Sprintf("items[%d].etag", i))
		if err != nil {
			return nil, err
		}
		out[i] = catalog.VersionedProduct{ProductID: id, Version: version}
	}
	return out, nil
}

func (h *Delegated) BatchUnpublishStoreProducts(ctx context.Context, req *connect.Request[pb.BatchUnpublishStoreProductsRequest]) (*connect.Response[pb.BatchUnpublishStoreProductsResponse], error) {
	op, err := httpdelegation.ParseOperation(req.Msg.GetOperation())
	if err != nil {
		return nil, err
	}
	items, err := batchItems(req.Msg.GetItems())
	if err != nil {
		return nil, err
	}
	c, err := httpdelegation.CallerFrom(ctx)
	if err != nil {
		return nil, err
	}
	// 請求內容雜湊與項目順序無關（依商品 ID 排序）：同一批商品與版本是同一個請求。
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b catalog.VersionedProduct) int { return a.ProductID.Compare(b.ProductID) })
	res, err := h.runner.DelegatedWrite(ctx, c, moddelegation.DelegatedWrite{
		Op: op, ResourceName: moddelegation.BatchResourceName(len(items)),
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditProductBatchUnpublish, sorted),
	}, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
		return failure(h.m.UnpublishProducts(ctx, st.StoreID, items, actor), batchPaths)
	})
	if err != nil {
		return nil, err
	}
	out := &pb.BatchUnpublishStoreProductsResponse{AuditLogId: res.AuditLogID.String(), Products: make([]*pb.UnpublishedProduct, len(items))}
	for i, it := range items {
		p, err := h.m.Product(ctx, res.Store.StoreID, it.ProductID)
		if err != nil {
			return nil, failure(err, noPaths)
		}
		out.Products[i] = &pb.UnpublishedProduct{
			ProductId: p.ID.String(), Etag: catalog.Etag(p.Version), Status: rpc.EnumToProto[pb.ProductStatus](p.Status),
		}
	}
	return connect.NewResponse(out), nil
}

// truncatedTitle 是建立失敗時稽核列的資源名稱：標題去除首尾空白後的前 60 個字元（與事件訂閱者寫成功列的名稱一致）。
func truncatedTitle(title string) string {
	return moddelegation.AuditName(title)
}
