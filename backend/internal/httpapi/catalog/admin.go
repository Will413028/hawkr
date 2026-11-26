package catalog

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1/catalogv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

// Admin 實作 CatalogAdminService：後台的唯讀查詢（全站、任何狀態、沒刪除）。權限宣告在 httpapi/adminauth（catalog.products.list、catalog.products.get）；
// 唯讀，沒有稽核紀錄。
type Admin struct {
	m      AdminModule
	images rpc.Images
}

var _ catalogv1alpha1connect.CatalogAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(m AdminModule, images rpc.Images) (*Admin, error) {
	if m == nil {
		return nil, fmt.Errorf("httpapi/catalog：需要 catalog 模組")
	}
	return &Admin{m: m, images: images}, nil
}

func (h *Admin) ListProducts(ctx context.Context, req *connect.Request[adminpb.ListProductsRequest]) (*connect.Response[adminpb.ListProductsResponse], error) {
	msg := req.Msg
	if _, err := rpc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := checkPageSize(msg.GetPageSize()); err != nil {
		return nil, err
	}
	query := normalizeQuery(msg.GetQuery())
	if err := tooLong("query", query, maxAdminQueryRunes); err != nil {
		return nil, err
	}
	f := catalog.AdminFilter{Query: query}
	if raw := msg.GetStoreId(); raw != "" {
		id, err := rpc.ParseID("store_id", raw)
		if err != nil {
			return nil, err
		}
		f.StoreID = &id
	}
	scope := rpc.FilterScope(catalogv1alpha1connect.CatalogAdminServiceListProductsProcedure, query, msg.GetStoreId())
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, total, err := h.m.ListAdminProducts(ctx, f, after, msg.GetPageSize())
	if err != nil {
		return nil, failure(err, noPaths)
	}
	out := make([]*adminpb.ProductSummary, len(page.Items))
	for i, s := range page.Items {
		if err := rpc.ObjectKey(s.CoverKey); err != nil {
			return nil, err
		}
		out[i] = &adminpb.ProductSummary{
			Id: s.ID.String(), StoreId: rpc.FormatID(s.StoreID), Title: s.Title, Status: rpc.EnumToProto[pb.ProductStatus](s.Status),
			CoverImage: h.images.Image(s.CoverKey), MinPrice: s.MinPrice, MaxPrice: s.MaxPrice,
			FirstPublishTime: optTime(s.FirstPublishedAt), CreateTime: timestamppb.New(s.CreatedAt),
		}
	}
	return connect.NewResponse(&adminpb.ListProductsResponse{Products: out, NextPageToken: nextToken(scope, page.Next), TotalSize: total}), nil
}

func (h *Admin) GetProduct(ctx context.Context, req *connect.Request[adminpb.GetProductRequest]) (*connect.Response[adminpb.GetProductResponse], error) {
	if _, err := rpc.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := rpc.ParseUUID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	p, err := h.m.AdminProduct(ctx, id)
	if err != nil {
		return nil, failure(err, noPaths)
	}
	v, err := newView(h.images, p, false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetProductResponse{Product: &adminpb.Product{
		Id: p.ID.String(), StoreId: rpc.FormatID(p.StoreID), CategoryId: p.CategoryID.String(), Title: p.Title,
		Status: rpc.EnumToProto[pb.ProductStatus](p.Status), FirstPublishTime: optTime(p.FirstPublishedAt),
		MinPrice: p.MinPrice, MaxPrice: p.MaxPrice, SalesCount: p.SalesCount, FavoriteCount: p.FavoriteCount,
		RatingAverage: rating(p.RatingAverage, p.ReviewCount), ReviewCount: p.ReviewCount,
		Gallery: v.gallery, Description: v.description, Axes: v.axes, Skus: v.skus, Hashtags: v.hashtags,
		CreateTime: timestamppb.New(p.CreatedAt), UpdateTime: timestamppb.New(p.UpdatedAt),
	}}), nil
}
