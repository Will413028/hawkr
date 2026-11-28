package catalog

import (
	"context"
	"errors"
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
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

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

type DelegatedModule interface {
	ListSellerProducts(ctx context.Context, storeID int64, f catalog.SellerFilter, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
	Product(ctx context.Context, storeID int64, productID uuid.UUID) (catalog.Product, error)
	CreateProduct(ctx context.Context, storeID int64, uploader media.Uploader, requestID uuid.UUID, in catalog.ProductInput, actor *delegatedactor.Actor) (catalog.Product, error)
	UpdateProduct(ctx context.Context, storeID int64, uploader media.Uploader, productID uuid.UUID, version int64, in catalog.ProductInput, actor *delegatedactor.Actor) (catalog.Product, error)
	SetProductStatus(ctx context.Context, storeID int64, productID uuid.UUID, version int64, to catalog.ProductStatus, actor *delegatedactor.Actor) (catalog.Product, error)
	UnpublishProducts(ctx context.Context, storeID int64, items []catalog.VersionedProduct, actor *delegatedactor.Actor) ([]catalog.UnpublishedItem, error)
}

const maxDelegatedPageSize = 50

type Delegated struct {
	runner httpdelegation.Runner
	m      DelegatedModule
	images rpc.Images
}

var _ catalogv1alpha1connect.CatalogDelegatedServiceHandler = (*Delegated)(nil)

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
		written = &p
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	var p catalog.Product
	if res.Replayed {
		id := productID
		if res.ResourceID != "" {
			if id, err = uuid.Parse(res.ResourceID); err != nil {
				return nil, "", fmt.Errorf("httpapi/catalog：稽核列 %s 的資源 ID %q：%w", res.AuditLogID, res.ResourceID, err)
			}
		}
		if p, err = h.m.Product(ctx, res.Store.StoreID, id); err != nil {
			return nil, "", failure(err, noPaths)
		}
	} else {
		if written == nil {
			return nil, "", fmt.Errorf("httpapi/catalog：代行寫入 %s 成功卻沒有商品", res.AuditLogID)
		}
		p = *written
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

var updatableProductFields = []string{"title", "category_id", "hashtag_inputs", "gallery", "description", "axes", "skus", "low_stock_threshold"}

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
		p, err := h.m.SetProductStatus(ctx, st.StoreID, id, version, to, actor)
		out := failure(err, statusPaths)
		var ae *apperr.Error
		if errors.Is(err, catalog.ErrNotPublished) && errors.As(out, &ae) {
			_ = ae.WithMetadata("notPublishedProductIds", id.String())
		}
		return p, out
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
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b catalog.VersionedProduct) int { return a.ProductID.Compare(b.ProductID) })
	var written []catalog.UnpublishedItem
	res, err := h.runner.DelegatedWrite(ctx, c, moddelegation.DelegatedWrite{
		Op: op, ResourceName: moddelegation.BatchResourceName(len(items)),
		PayloadHash: moddelegation.DelegatedPayloadHash(moddelegation.AuditProductBatchUnpublish, sorted),
	}, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
		var err error
		written, err = h.m.UnpublishProducts(ctx, st.StoreID, items, actor)
		return failure(err, batchPaths)
	})
	if err != nil {
		return nil, err
	}
	after := make(map[uuid.UUID]int64, len(items))
	if res.Replayed {
		for _, it := range res.Items {
			id, err := uuid.Parse(it.ResourceID)
			if err != nil {
				return nil, fmt.Errorf("httpapi/catalog：稽核列 %s 的商品 ID %q：%w", res.AuditLogID, it.ResourceID, err)
			}
			after[id] = it.VersionAfter
		}
	} else {
		for _, it := range written {
			after[it.ProductID] = it.VersionAfter
		}
	}
	out := &pb.BatchUnpublishStoreProductsResponse{AuditLogId: res.AuditLogID.String(), Products: make([]*pb.UnpublishedProduct, len(items))}
	for i, it := range items {
		v, ok := after[it.ProductID]
		if !ok {
			return nil, fmt.Errorf("httpapi/catalog：批次下架 %s 的結果沒有商品 %s", res.AuditLogID, it.ProductID)
		}
		out.Products[i] = &pb.UnpublishedProduct{
			ProductId: it.ProductID.String(), Etag: catalog.Etag(v), Status: rpc.EnumToProto[pb.ProductStatus](catalog.StatusArchived),
		}
	}
	return connect.NewResponse(out), nil
}

func truncatedTitle(title string) string {
	return moddelegation.AuditName(title)
}
