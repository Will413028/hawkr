package catalog

import (
	"strings"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

func toInput(in *pb.ProductInput) (catalog.ProductInput, error) {
	out := catalog.ProductInput{Title: in.GetTitle(), Hashtags: in.GetHashtagInputs(), LowStockThreshold: in.LowStockThreshold}
	if raw := in.GetCategoryId(); raw != "" {
		id, err := rpc.ParseUUID("product.category_id", raw)
		if err != nil {
			return out, err
		}
		out.CategoryID = id
	}
	for _, g := range in.GetGallery() {
		img, err := toImageInput(g, "product.gallery")
		if err != nil {
			return out, err
		}
		out.Gallery = append(out.Gallery, img)
	}
	for _, b := range in.GetDescription() {
		blk, err := toBlockInput(b)
		if err != nil {
			return out, err
		}
		out.Description = append(out.Description, blk)
	}
	for _, a := range in.GetAxes() {
		out.Axes = append(out.Axes, catalog.Axis{Name: a.GetName(), Values: a.GetValues()})
	}
	for _, s := range in.GetSkus() {
		out.Skus = append(out.Skus, catalog.SkuInput{
			Option1: s.GetOption1(), Option2: s.GetOption2(), Price: s.GetPrice(), CompareAtPrice: s.CompareAtPrice, StockQty: s.GetInitialStock(),
		})
	}
	return out, nil
}

func toImageInput(in *pb.ImageInput, path string) (catalog.Image, error) {
	img := catalog.Image{Alt: in.GetAlt()}
	switch src := in.GetSource().(type) {
	case *pb.ImageInput_StagedToken:
		img.Token = src.StagedToken
	case *pb.ImageInput_AssetId:
		id, err := rpc.ParseUUID(path, src.AssetId)
		if err != nil {
			return img, err
		}
		img.AssetID = id
	}
	return img, nil
}

func toBlockInput(in *pb.DescriptionBlock) (catalog.Block, error) {
	switch b := in.GetBlock().(type) {
	case *pb.DescriptionBlock_Paragraph:
		return catalog.Block{Kind: catalog.BlockParagraph, Text: b.Paragraph}, nil
	case *pb.DescriptionBlock_Image:
		img, err := toImageInput(b.Image, "product.description")
		if err != nil {
			return catalog.Block{}, err
		}
		return catalog.Block{Kind: catalog.BlockImage, Image: &img}, nil
	case *pb.DescriptionBlock_BulletList:
		return catalog.Block{Kind: catalog.BlockList, Items: b.BulletList.GetItems()}, nil
	case *pb.DescriptionBlock_Table:
		rows := make([][]string, len(b.Table.GetRows()))
		for i, r := range b.Table.GetRows() {
			rows[i] = r.GetCells()
		}
		return catalog.Block{Kind: catalog.BlockTable, Rows: rows}, nil
	}
	return catalog.Block{}, nil
}

func publicFilter(in *pb.ProductFilter) (catalog.PublicFilter, []string, error) {
	if in == nil {
		in = &pb.ProductFilter{}
	}
	var f catalog.PublicFilter
	if err := tooMany("filter.category_ids", len(in.GetCategoryIds()), maxFilterCategories); err != nil {
		return f, nil, err
	}
	if err := tooMany("filter.hashtags", len(in.GetHashtags()), maxFilterHashtags); err != nil {
		return f, nil, err
	}
	if err := tooMany("filter.store_ids", len(in.GetStoreIds()), maxFilterStores); err != nil {
		return f, nil, err
	}
	for _, raw := range in.GetCategoryIds() {
		id, err := rpc.ParseUUID("filter.category_ids", raw)
		if err != nil {
			return f, nil, err
		}
		f.CategoryIDs = append(f.CategoryIDs, id)
	}
	for _, raw := range in.GetStoreIds() {
		id, err := rpc.ParseID("filter.store_ids", raw)
		if err != nil {
			return f, nil, err
		}
		f.StoreIDs = append(f.StoreIDs, id)
	}
	f.PriceMin, f.PriceMax = in.PriceMin, in.PriceMax
	f.Hashtags = in.GetHashtags()
	f.HashtagAll = in.GetHashtagMatch() == pb.HashtagMatch_HASHTAG_MATCH_ALL
	scope := []string{
		strings.Join(in.GetCategoryIds(), ","), optInt(in.PriceMin), optInt(in.PriceMax),
		strings.Join(in.GetHashtags(), ","), in.GetHashtagMatch().String(), strings.Join(in.GetStoreIds(), ","),
	}
	return f, scope, nil
}

func optInt(v *int64) string {
	if v == nil {
		return ""
	}
	return rpc.FormatID(*v)
}

func requestID(raw string) (uuid.UUID, error) {
	id, err := rpc.ParseRequestID(raw)
	if err != nil {
		return uuid.UUID{}, err
	}
	if id == nil {
		return uuid.UUID{}, rpc.RequiredField("request_id", "request_id 必填")
	}
	return *id, nil
}

func normalizeQuery(q string) string { return trimtext.Trim(q) }
