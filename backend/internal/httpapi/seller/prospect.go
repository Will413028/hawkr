package seller

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ProspectModule 是後台商家名單 handler 使用的模組方法（ADR 2026-10-04-prospect-contact-points-and-matching）。
type ProspectModule interface {
	Prospects(context.Context, model.ProspectListInput) (model.ProspectPage, error)
	CreateProspect(context.Context, model.CreateProspectInput) (model.Prospect, error)
	PreviewProspect(context.Context, model.ProspectPreviewInput) ([]model.ProspectMatch, error)
	Prospect(ctx context.Context, id int64) (model.Prospect, error)
	UpdateProspect(context.Context, model.UpdateProspectInput) (model.Prospect, error)
	ArchiveProspect(context.Context, model.ArchiveProspectInput) (model.Prospect, error)
	RestoreProspect(context.Context, model.RestoreProspectInput) (model.Prospect, error)
	MergeProspects(context.Context, model.MergeProspectsInput) (model.Prospect, error)
	ConvertProspect(context.Context, model.ConvertProspectInput) (model.Prospect, error)
	DuplicateProspects(ctx context.Context, pageSize int32, after *model.DuplicateCursor) (model.DuplicatePage, error)
	ProspectTaxIDMatches(ctx context.Context, pageSize int32, after *model.TaxIDMatchCursor) (model.TaxIDMatchPage, error)
	MarkProspectsDistinct(context.Context, model.MarkProspectsDistinctInput) error
}

// Prospects 是 SellerProspectAdminService 的 handler。授權（管理員、每支方法的權限）由 adminauth 的 interceptor 檢查。
type Prospects struct {
	m      ProspectModule
	images rpc.Images
}

var _ sellerv1alpha1connect.SellerProspectAdminServiceHandler = (*Prospects)(nil)

// NewProspects 建立後台商家名單的 handler；images 給 logo 與封面。
func NewProspects(m ProspectModule, images rpc.Images) *Prospects {
	return &Prospects{m: m, images: images}
}

// 模組輸入的欄位對應到請求的路徑。
var (
	prospectFieldPaths = rpc.FieldPaths{
		"DisplayName": "prospect.display_name", "VendorType": "prospect.vendor_type", "OutreachStatus": "prospect.outreach_status",
		"TaxID": "prospect.tax_id", "Contacts": "prospect.contacts", "Contacts[].Kind": "prospect.contacts[].kind",
		"Contacts[].Value": "prospect.contacts[].value", "Category": "prospect.category", "Region": "prospect.region",
		"ProductSummary": "prospect.product_summary", "Positioning": "prospect.positioning", "ResearchSummary": "prospect.research_summary",
		"PriorityLetter": "prospect.priority_letter", "PriorityStars": "prospect.priority_stars", "CategoryIDs": "prospect.category_ids",
		"Logo": "prospect.logo_upload", "Logo.StagedToken": "prospect.logo_upload.staged_token", "Logo.SourceURL": "prospect.logo_upload.source_url",
		"Cover": "prospect.cover_upload", "Cover.StagedToken": "prospect.cover_upload.staged_token", "Cover.SourceURL": "prospect.cover_upload.source_url",
		"ExpectedVersion": "prospect.etag",
	}
	previewProspectPaths = rpc.FieldPaths{
		"DisplayName": "display_name", "TaxID": "tax_id", "Contacts": "contacts", "Contacts[].Kind": "contacts[].kind", "Contacts[].Value": "contacts[].value",
	}
	listProspectsPaths = rpc.FieldPaths{
		"Status": "status", "OutreachStatus": "outreach_status", "VendorType": "vendor_type",
	}
	archiveProspectPaths = rpc.FieldPaths{"ExpectedVersion": "etag"}
	mergeProspectsPaths  = rpc.FieldPaths{"LoserID": "loser_id", "WinnerVersion": "winner_etag", "LoserVersion": "loser_etag"}
	convertProspectPaths = rpc.FieldPaths{"ExpectedVersion": "etag"}
	markDistinctPaths    = rpc.FieldPaths{"SecondID": "second_prospect_id"}
)

func (h *Prospects) toProspect(p model.Prospect) (*adminpb.Prospect, error) {
	out := &adminpb.Prospect{
		Id: rpc.FormatID(p.ID), DisplayName: p.DisplayName, VendorType: rpc.EnumToProto[adminpb.VendorType](p.VendorType),
		Status: rpc.EnumToProto[adminpb.ProspectStatus](p.Status), OutreachStatus: rpc.EnumToProto[adminpb.OutreachStatus](p.OutreachStatus),
		TaxId: deref(p.TaxID), Category: deref(p.Category), Region: deref(p.Region), ProductSummary: deref(p.ProductSummary),
		Positioning: deref(p.Positioning), ResearchSummary: deref(p.ResearchSummary), PriorityLetter: deref(p.PriorityLetter),
		PriorityStars: p.PriorityStars, LogoSourceUrl: deref(p.LogoSourceURL), CoverSourceUrl: deref(p.CoverSourceURL),
		CreatedByAdminId: formatAdminID(p.CreatedByAdminID), UpdatedByAdminId: formatAdminID(p.UpdatedByAdminID),
		CreateTime: timestamppb.New(p.CreatedAt), UpdateTime: timestamppb.New(p.UpdatedAt), Etag: rpc.Etag(p.Version),
		CategoryIds: p.CategoryIDs,
	}
	if p.ConvertedStore != nil {
		out.ConvertedStoreId = rpc.FormatID(*p.ConvertedStore)
	}
	if p.ConvertedAt != nil {
		out.ConvertTime = timestamppb.New(*p.ConvertedAt)
	}
	if p.MergedInto != nil {
		out.MergedIntoId = rpc.FormatID(*p.MergedInto)
	}
	for _, c := range p.Contacts {
		out.Contacts = append(out.Contacts, &adminpb.ProspectContact{Kind: rpc.EnumToProto[adminpb.ContactKind](c.Kind), Value: c.Value, Primary: c.Primary})
	}
	for _, image := range []struct {
		key *string
		dst **commonpb.Image
	}{{p.LogoKey, &out.LogoImage}, {p.CoverKey, &out.CoverImage}} {
		if image.key == nil {
			continue
		}
		if err := rpc.ObjectKey(*image.key); err != nil {
			return nil, err
		}
		*image.dst = h.images.Image(*image.key)
	}
	return out, nil
}

// toContacts 把請求的聯絡方式轉成模組的輸入；類型 UNSPECIFIED 轉成空字串，由模組回報不合法。
func toContacts(in []*adminpb.ProspectContact) []model.ContactPoint {
	out := make([]model.ContactPoint, len(in))
	for i, c := range in {
		out[i] = model.ContactPoint{Kind: rpc.EnumFromProto[model.ContactKind](c.GetKind()), Value: c.GetValue(), Primary: c.GetPrimary()}
	}
	return out
}

// confirmedDistinct 解析請求的 confirmed_distinct_prospect_ids；格式不對的錯誤指到那一個元素（path[i]）。
func confirmedDistinct(raw []string, path string) ([]int64, error) {
	var out []int64
	for i, s := range raw {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			element := fmt.Sprintf("%s[%d]", path, i)
			return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{element}, "%s 必須是有效的 ID", element), rpc.FieldPaths{path + "[]": path + "[]"})
		}
		out = append(out, id)
	}
	return out, nil
}

// optionalText 是建立時的文字欄位：空字串是沒有值。
func optionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *Prospects) ListProspects(ctx context.Context, req *connect.Request[adminpb.ListProspectsRequest]) (*connect.Response[adminpb.ListProspectsResponse], error) {
	msg := req.Msg
	in := model.ProspectListInput{Category: msg.GetCategory(), Region: msg.GetRegion(), Keyword: msg.GetKeyword(), PageSize: msg.GetPageSize()}
	if v := rpc.EnumFromProto[model.ProspectStatus](msg.GetStatus()); v != "" {
		in.Status = &v
	}
	if v := rpc.EnumFromProto[model.OutreachStatus](msg.GetOutreachStatus()); v != "" {
		in.OutreachStatus = &v
	}
	if v := rpc.EnumFromProto[model.VendorType](msg.GetVendorType()); v != "" {
		in.VendorType = &v
	}
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerProspectAdminServiceListProspectsProcedure, msg.GetStatus().String(),
		msg.GetOutreachStatus().String(), msg.GetVendorType().String(), msg.GetCategory(), msg.GetRegion(), msg.GetKeyword())
	after, err := rpc.PageCursor[model.ProspectCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if after != nil && (after.ProspectID <= 0 || after.UpdatedAt.IsZero()) {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	in.After = after
	page, err := h.m.Prospects(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, listProspectsPaths)
	}
	out := &adminpb.ListProspectsResponse{Prospects: make([]*adminpb.Prospect, len(page.Items))}
	for i, p := range page.Items {
		if out.Prospects[i], err = h.toProspect(p); err != nil {
			return nil, err
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Prospects) CreateProspect(ctx context.Context, req *connect.Request[adminpb.CreateProspectRequest]) (*connect.Response[adminpb.CreateProspectResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetProspect()
	if msg == nil {
		return nil, rpc.RequiredField("prospect", "prospect 必填")
	}
	for _, upload := range []struct {
		path string
		set  bool
	}{{"prospect.logo_upload", msg.GetLogoUpload() != nil}, {"prospect.cover_upload", msg.GetCoverUpload() != nil}} {
		if upload.set {
			return nil, rpc.RequiredField(upload.path, "建立時不能換圖，建立後以 UpdateProspect 換圖")
		}
	}
	confirmed, err := confirmedDistinct(req.Msg.GetConfirmedDistinctProspectIds(), "confirmed_distinct_prospect_ids")
	if err != nil {
		return nil, err
	}
	created, err := h.m.CreateProspect(ctx, model.CreateProspectInput{
		AdminID: adminID,
		Fields: model.ProspectFields{
			DisplayName: msg.GetDisplayName(), VendorType: rpc.EnumFromProto[model.VendorType](msg.GetVendorType()),
			OutreachStatus: rpc.EnumFromProto[model.OutreachStatus](msg.GetOutreachStatus()), TaxID: optionalText(msg.GetTaxId()),
			Category: optionalText(msg.GetCategory()), Region: optionalText(msg.GetRegion()), ProductSummary: optionalText(msg.GetProductSummary()),
			Positioning: optionalText(msg.GetPositioning()), ResearchSummary: optionalText(msg.GetResearchSummary()),
			PriorityLetter: optionalText(msg.GetPriorityLetter()), PriorityStars: msg.GetPriorityStars(),
		},
		Contacts: toContacts(msg.GetContacts()), CategoryIDs: msg.GetCategoryIds(), ConfirmedDistinct: confirmed,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, prospectFieldPaths)
	}
	p, err := h.toProspect(created)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.CreateProspectResponse{Prospect: p}), nil
}

func (h *Prospects) PreviewProspect(ctx context.Context, req *connect.Request[adminpb.PreviewProspectRequest]) (*connect.Response[adminpb.PreviewProspectResponse], error) {
	msg := req.Msg
	matches, err := h.m.PreviewProspect(ctx, model.ProspectPreviewInput{DisplayName: msg.GetDisplayName(), TaxID: msg.GetTaxId(), Contacts: toContacts(msg.GetContacts())})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, previewProspectPaths)
	}
	out := &adminpb.PreviewProspectResponse{Matches: make([]*adminpb.ProspectMatch, len(matches))}
	for i, m := range matches {
		p, err := h.toProspect(m.Prospect)
		if err != nil {
			return nil, err
		}
		out.Matches[i] = &adminpb.ProspectMatch{Prospect: p, Tier: rpc.EnumToProto[adminpb.ProspectMatchTier](m.Tier), MatchedOn: matchReasons(m.MatchedOn)}
	}
	return connect.NewResponse(out), nil
}

func (h *Prospects) GetProspect(ctx context.Context, req *connect.Request[adminpb.GetProspectRequest]) (*connect.Response[adminpb.GetProspectResponse], error) {
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	found, err := h.m.Prospect(ctx, id)
	if err != nil {
		return nil, err
	}
	p, err := h.toProspect(found)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.GetProspectResponse{Prospect: p}), nil
}

// UpdateProspect 走 API 設計規範 §七的第二種做法：Prospect 有 INPUT_ONLY 的欄位（logo_upload、cover_upload），目前的上傳不在
// 請求裡，不能先合併再整筆送。可寫的欄位集合取自 proto 的 descriptor（rpc.WritableFields），這裡沒有欄位清單：沒選到的欄位是
// nil（保留），選到卻沒有值的欄位是清掉；etag 只是版本，不是可寫的資料。
func (h *Prospects) UpdateProspect(ctx context.Context, req *connect.Request[adminpb.UpdateProspectRequest]) (*connect.Response[adminpb.UpdateProspectResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg.GetProspect()
	if msg == nil {
		return nil, rpc.RequiredField("prospect", "prospect 必填")
	}
	id, err := rpc.ParseID("prospect.id", msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(msg.GetEtag(), "prospect.etag")
	if err != nil {
		return nil, err
	}
	confirmed, err := confirmedDistinct(req.Msg.GetConfirmedDistinctProspectIds(), "confirmed_distinct_prospect_ids")
	if err != nil {
		return nil, err
	}
	paths, err := rpc.WritableFields(msg, req.Msg.GetUpdateMask())
	if err != nil {
		return nil, err
	}
	in := model.UpdateProspectInput{ProspectID: id, ExpectedVersion: version, AdminID: adminID, ConfirmedDistinct: confirmed}
	has := func(path string) bool { return slices.Contains(paths, path) }
	text := func(path string, dst **string, v string) {
		if has(path) {
			*dst = &v
		}
	}
	text("display_name", &in.DisplayName, msg.GetDisplayName())
	if has("vendor_type") {
		v := rpc.EnumFromProto[model.VendorType](msg.GetVendorType())
		in.VendorType = &v
	}
	if has("outreach_status") {
		// 模組把空值當成「回到 NOT_CONTACTED」，所以明列 mask 卻沒有值要在這裡擋下，不能默默重設進度。
		v := rpc.EnumFromProto[model.OutreachStatus](msg.GetOutreachStatus())
		if v == "" {
			return nil, rpc.RequiredField("prospect.outreach_status", "update_mask 列了 outreach_status，必須設定它的值")
		}
		in.OutreachStatus = &v
	}
	text("tax_id", &in.TaxID, msg.GetTaxId())
	text("category", &in.Category, msg.GetCategory())
	text("region", &in.Region, msg.GetRegion())
	text("product_summary", &in.ProductSummary, msg.GetProductSummary())
	text("positioning", &in.Positioning, msg.GetPositioning())
	text("research_summary", &in.ResearchSummary, msg.GetResearchSummary())
	text("priority_letter", &in.PriorityLetter, msg.GetPriorityLetter())
	if has("priority_stars") {
		v := msg.GetPriorityStars()
		in.PriorityStars = &v
	}
	if has("contacts") {
		contacts := toContacts(msg.GetContacts())
		in.Contacts = &contacts
	}
	if has("category_ids") {
		ids := slices.Clone(msg.GetCategoryIds())
		if ids == nil {
			ids = []string{}
		}
		in.CategoryIDs = &ids
	}
	for _, image := range []struct {
		path   string
		upload *adminpb.ProspectImageUpload
		dst    **model.ProspectImageUpload
	}{{"logo_upload", msg.GetLogoUpload(), &in.Logo}, {"cover_upload", msg.GetCoverUpload(), &in.Cover}} {
		if !has(image.path) {
			continue
		}
		// 圖片是狀態、但沒有「清掉」的操作（業者的圖只會被換，或隨封存與合併釋放）：列進 mask 卻沒有上傳是自相矛盾。
		if image.upload == nil {
			return nil, rpc.RequiredField("prospect."+image.path, "update_mask 列了 "+image.path+"，必須帶上傳的 staged_token")
		}
		*image.dst = &model.ProspectImageUpload{StagedToken: image.upload.GetStagedToken(), SourceURL: image.upload.GetSourceUrl()}
	}
	updated, err := h.m.UpdateProspect(ctx, in)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, prospectFieldPaths)
	}
	p, err := h.toProspect(updated)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.UpdateProspectResponse{Prospect: p}), nil
}

func (h *Prospects) ArchiveProspect(ctx context.Context, req *connect.Request[adminpb.ArchiveProspectRequest]) (*connect.Response[adminpb.ArchiveProspectResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	archived, err := h.m.ArchiveProspect(ctx, model.ArchiveProspectInput{ProspectID: id, AdminID: adminID, ExpectedVersion: version})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, archiveProspectPaths)
	}
	p, err := h.toProspect(archived)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.ArchiveProspectResponse{Prospect: p}), nil
}

func (h *Prospects) RestoreProspect(ctx context.Context, req *connect.Request[adminpb.RestoreProspectRequest]) (*connect.Response[adminpb.RestoreProspectResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	confirmed, err := confirmedDistinct(req.Msg.GetConfirmedDistinctProspectIds(), "confirmed_distinct_prospect_ids")
	if err != nil {
		return nil, err
	}
	restored, err := h.m.RestoreProspect(ctx, model.RestoreProspectInput{ProspectID: id, AdminID: adminID, ConfirmedDistinct: confirmed})
	if err != nil {
		return nil, err
	}
	p, err := h.toProspect(restored)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.RestoreProspectResponse{Prospect: p}), nil
}

func (h *Prospects) MergeProspects(ctx context.Context, req *connect.Request[adminpb.MergeProspectsRequest]) (*connect.Response[adminpb.MergeProspectsResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	winnerID, err := rpc.ParseID("winner_id", msg.GetWinnerId())
	if err != nil {
		return nil, err
	}
	loserID, err := rpc.ParseID("loser_id", msg.GetLoserId())
	if err != nil {
		return nil, err
	}
	winnerVersion, err := rpc.RequiredVersionField(msg.GetWinnerEtag(), "winner_etag")
	if err != nil {
		return nil, err
	}
	loserVersion, err := rpc.RequiredVersionField(msg.GetLoserEtag(), "loser_etag")
	if err != nil {
		return nil, err
	}
	merged, err := h.m.MergeProspects(ctx, model.MergeProspectsInput{
		WinnerID: winnerID, LoserID: loserID, AdminID: adminID, WinnerVersion: winnerVersion, LoserVersion: loserVersion,
	})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, mergeProspectsPaths)
	}
	p, err := h.toProspect(merged)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.MergeProspectsResponse{Prospect: p}), nil
}

func (h *Prospects) ConvertProspect(ctx context.Context, req *connect.Request[adminpb.ConvertProspectRequest]) (*connect.Response[adminpb.ConvertProspectResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := rpc.ParseID("id", req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	version, err := rpc.RequiredVersionField(req.Msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	converted, err := h.m.ConvertProspect(ctx, model.ConvertProspectInput{ProspectID: id, StoreID: storeID, AdminID: adminID, ExpectedVersion: version})
	if err != nil {
		return nil, rpc.WithFieldPaths(err, convertProspectPaths)
	}
	p, err := h.toProspect(converted)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.ConvertProspectResponse{Prospect: p}), nil
}

func (h *Prospects) ListProspectDuplicates(ctx context.Context, req *connect.Request[adminpb.ListProspectDuplicatesRequest]) (*connect.Response[adminpb.ListProspectDuplicatesResponse], error) {
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerProspectAdminServiceListProspectDuplicatesProcedure)
	after, err := rpc.PageCursor[model.DuplicateCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if after != nil && (after.LeftID <= 0 || after.RightID <= 0) {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	page, err := h.m.DuplicateProspects(ctx, req.Msg.GetPageSize(), after)
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListProspectDuplicatesResponse{Duplicates: make([]*adminpb.ProspectDuplicate, len(page.Items))}
	for i, d := range page.Items {
		left, err := h.toProspect(d.Left)
		if err != nil {
			return nil, err
		}
		right, err := h.toProspect(d.Right)
		if err != nil {
			return nil, err
		}
		out.Duplicates[i] = &adminpb.ProspectDuplicate{Left: left, Right: right, MatchedOn: matchReasons(d.MatchedOn)}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Prospects) ListProspectTaxIDMatches(ctx context.Context, req *connect.Request[adminpb.ListProspectTaxIDMatchesRequest]) (*connect.Response[adminpb.ListProspectTaxIDMatchesResponse], error) {
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerProspectAdminServiceListProspectTaxIDMatchesProcedure)
	after, err := rpc.PageCursor[model.TaxIDMatchCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	if after != nil && (after.ProspectID <= 0 || after.StoreID <= 0) {
		return nil, rpc.Error(connect.CodeInvalidArgument, "%s", "page_token 不合法")
	}
	page, err := h.m.ProspectTaxIDMatches(ctx, req.Msg.GetPageSize(), after)
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListProspectTaxIDMatchesResponse{Matches: make([]*adminpb.ProspectTaxIDMatch, len(page.Items))}
	for i, m := range page.Items {
		p, err := h.toProspect(m.Prospect)
		if err != nil {
			return nil, err
		}
		out.Matches[i] = &adminpb.ProspectTaxIDMatch{
			Prospect: p, StoreId: rpc.FormatID(m.StoreID), StoreName: deref(m.StoreName), CompanyName: deref(m.CompanyName),
			KycStatus: rpc.EnumToProto[pb.KYCStatus](m.KYCStatus),
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Prospects) MarkProspectsDistinct(ctx context.Context, req *connect.Request[adminpb.MarkProspectsDistinctRequest]) (*connect.Response[adminpb.MarkProspectsDistinctResponse], error) {
	adminID, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	first, err := rpc.ParseID("first_prospect_id", req.Msg.GetFirstProspectId())
	if err != nil {
		return nil, err
	}
	second, err := rpc.ParseID("second_prospect_id", req.Msg.GetSecondProspectId())
	if err != nil {
		return nil, err
	}
	if err := h.m.MarkProspectsDistinct(ctx, model.MarkProspectsDistinctInput{FirstID: first, SecondID: second, AdminID: adminID}); err != nil {
		return nil, rpc.WithFieldPaths(err, markDistinctPaths)
	}
	return connect.NewResponse(&adminpb.MarkProspectsDistinctResponse{}), nil
}

func matchReasons(in []model.MatchReason) []adminpb.ProspectMatchReason {
	out := make([]adminpb.ProspectMatchReason, len(in))
	for i, r := range in {
		out[i] = rpc.EnumToProto[adminpb.ProspectMatchReason](r)
	}
	return out
}
