package seller

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// prospectModule 記下每個指令收到的輸入，回傳設定好的業者；err 非 nil 時指令回這個錯誤。
type prospectModule struct {
	ProspectModule
	prospect model.Prospect
	err      error
	list     model.ProspectListInput
	create   model.CreateProspectInput
	preview  model.ProspectPreviewInput
	update   model.UpdateProspectInput
	archive  model.ArchiveProspectInput
	restore  model.RestoreProspectInput
	merge    model.MergeProspectsInput
	convert  model.ConvertProspectInput
	dupAfter *model.DuplicateCursor
	taxAfter *model.TaxIDMatchCursor
	dupPage  model.DuplicatePage
	taxPage  model.TaxIDMatchPage
	page     model.ProspectPage
	mark     model.MarkProspectsDistinctInput
	calls    int
}

func (m *prospectModule) Prospects(_ context.Context, in model.ProspectListInput) (model.ProspectPage, error) {
	m.list = in
	return m.page, m.err
}

func (m *prospectModule) CreateProspect(_ context.Context, in model.CreateProspectInput) (model.Prospect, error) {
	m.create, m.calls = in, m.calls+1
	return m.prospect, m.err
}

func (m *prospectModule) PreviewProspect(_ context.Context, in model.ProspectPreviewInput) ([]model.ProspectMatch, error) {
	m.preview = in
	return []model.ProspectMatch{{Prospect: m.prospect, Tier: model.MatchContact, MatchedOn: []model.MatchReason{model.ReasonEmail}}}, m.err
}

func (m *prospectModule) Prospect(context.Context, int64) (model.Prospect, error) {
	return m.prospect, m.err
}

func (m *prospectModule) UpdateProspect(_ context.Context, in model.UpdateProspectInput) (model.Prospect, error) {
	m.update, m.calls = in, m.calls+1
	return m.prospect, m.err
}

func (m *prospectModule) ArchiveProspect(_ context.Context, in model.ArchiveProspectInput) (model.Prospect, error) {
	m.archive, m.calls = in, m.calls+1
	return m.prospect, m.err
}

func (m *prospectModule) RestoreProspect(_ context.Context, in model.RestoreProspectInput) (model.Prospect, error) {
	m.restore, m.calls = in, m.calls+1
	return m.prospect, m.err
}

func (m *prospectModule) MergeProspects(_ context.Context, in model.MergeProspectsInput) (model.Prospect, error) {
	m.merge, m.calls = in, m.calls+1
	return m.prospect, m.err
}

func (m *prospectModule) ConvertProspect(_ context.Context, in model.ConvertProspectInput) (model.Prospect, error) {
	m.convert, m.calls = in, m.calls+1
	return m.prospect, m.err
}

func (m *prospectModule) MarkProspectsDistinct(_ context.Context, in model.MarkProspectsDistinctInput) error {
	m.mark, m.calls = in, m.calls+1
	return m.err
}

func (m *prospectModule) DuplicateProspects(_ context.Context, _ int32, after *model.DuplicateCursor) (model.DuplicatePage, error) {
	m.dupAfter = after
	return m.dupPage, m.err
}

func (m *prospectModule) ProspectTaxIDMatches(_ context.Context, _ int32, after *model.TaxIDMatchCursor) (model.TaxIDMatchPage, error) {
	m.taxAfter = after
	return m.taxPage, m.err
}

var prospectNow = time.Date(2026, 10, 4, 2, 0, 0, 0, time.UTC)

func str(s string) *string { return &s }

// fullProspect 是每個欄位都有值的業者（含單筆讀取才有的聯絡方式與分類）。
func fullProspect() model.Prospect {
	store, merged, creator, updater := int64(77), int64(5), int64(11), int64(12)
	converted := prospectNow.Add(time.Hour)
	return model.Prospect{
		ID: 9, Version: 4, DisplayName: "王記農場", VendorType: model.VendorBusiness, Status: model.ProspectStatusConverted,
		OutreachStatus: model.OutreachConverted, TaxID: str("22223223"), ConvertedStore: &store, ConvertedAt: &converted, MergedInto: &merged,
		Category: str("蔬果"), Region: str("宜蘭"), ProductSummary: str("有機蔬菜"), Positioning: str("中高價位"), ResearchSummary: str("研究"),
		PriorityLetter: str("A"), PriorityStars: 3, LogoKey: str("prospect/logo.jpg"), LogoSourceURL: str("https://example.com/logo"),
		CoverKey: str("prospect/cover.jpg"), CoverSourceURL: str("https://example.com/cover"), CreatedByAdminID: &creator, UpdatedByAdminID: &updater,
		CreatedAt: prospectNow, UpdatedAt: prospectNow.Add(time.Minute),
		Contacts:    []model.ContactPoint{{Kind: model.ContactEmail, Value: "a@example.com", Primary: true}, {Kind: model.ContactWebsite, Value: "https://wang.example.com", Primary: true}},
		CategoryIDs: []string{"c1", "c2"},
	}
}

func prospectHandler(t *testing.T, m *prospectModule) *Prospects {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return NewProspects(m, images)
}

// 逐欄位對照：模組的 Prospect 每個欄位都對應到 proto 的一個欄位；proto 的欄位除了 INPUT_ONLY 的上傳（只在請求）以外都要被填。
// 沒有對應的欄位：模組的 Version 轉成 etag、LogoKey 與 CoverKey 轉成 Image，ConvertedAt 轉成 convert_time。
func TestProspectMapperFillsEveryResponseField(t *testing.T) {
	h := prospectHandler(t, &prospectModule{})
	got, err := h.toProspect(fullProspect())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"logo_upload": true, "cover_upload": true}
	fields := got.ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if want[string(fd.Name())] {
			if got.ProtoReflect().Has(fd) {
				t.Errorf("%s 是 INPUT_ONLY，回應不能帶", fd.Name())
			}
			continue
		}
		if !got.ProtoReflect().Has(fd) {
			t.Errorf("proto 欄位 %s 沒有被填（模組欄位沒有對應？）", fd.Name())
		}
	}
	if got.GetId() != "9" || got.GetEtag() != "4" || got.GetTaxId() != "22223223" || got.GetConvertedStoreId() != "77" || got.GetMergedIntoId() != "5" ||
		got.GetCreatedByAdminId() != "11" || got.GetUpdatedByAdminId() != "12" || got.GetPriorityStars() != 3 ||
		got.GetStatus() != adminpb.ProspectStatus_PROSPECT_STATUS_CONVERTED || got.GetOutreachStatus() != adminpb.OutreachStatus_OUTREACH_STATUS_CONVERTED ||
		got.GetVendorType() != adminpb.VendorType_VENDOR_TYPE_BUSINESS || len(got.GetContacts()) != 2 ||
		got.GetContacts()[0].GetKind() != adminpb.ContactKind_CONTACT_KIND_EMAIL || !got.GetContacts()[0].GetPrimary() ||
		got.GetLogoImage().GetUriTemplate() != "https://media.example.test/i/{preset}/prospect/logo.jpg" ||
		got.GetCoverImage().GetUriTemplate() != "https://media.example.test/i/{preset}/prospect/cover.jpg" ||
		got.GetLogoSourceUrl() != "https://example.com/logo" || got.GetCoverSourceUrl() != "https://example.com/cover" ||
		!slices.Equal(got.GetCategoryIds(), []string{"c1", "c2"}) {
		t.Fatalf("got=%v", got)
	}
	// 沒有值的欄位：空字串、不設，不是零值冒充有值。
	empty, err := h.toProspect(model.Prospect{ID: 1, Version: 1, DisplayName: "x", VendorType: model.VendorIndividual, Status: model.ProspectStatusActive, OutreachStatus: model.OutreachNotContacted})
	if err != nil {
		t.Fatal(err)
	}
	if empty.GetLogoImage() != nil || empty.GetCoverImage() != nil || empty.GetConvertTime() != nil || empty.GetConvertedStoreId() != "" ||
		empty.GetMergedIntoId() != "" || empty.GetTaxId() != "" || empty.GetCreatedByAdminId() != "" || len(empty.GetContacts()) != 0 {
		t.Fatalf("empty=%v", empty)
	}
	// 舊資料可能存完整網址：不能組進 URI template。
	if _, err := h.toProspect(model.Prospect{LogoKey: str("https://x.example/a.jpg")}); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Fatalf("err=%v", err)
	}
}

// 模組的每個欄位都在 proto 有對應：模組的值域與契約的列舉一一對應。
func TestProspectEnumsMatchTheModule(t *testing.T) {
	for name, mismatches := range map[string][]string{
		"ProspectStatus": rpc.EnumMismatches[adminpb.ProspectStatus](model.ProspectStatusValues()),
		"VendorType":     rpc.EnumMismatches[adminpb.VendorType](model.VendorTypeValues()),
		"OutreachStatus": rpc.EnumMismatches[adminpb.OutreachStatus](model.OutreachStatusValues()),
		"ContactKind":    rpc.EnumMismatches[adminpb.ContactKind](model.ContactKindValues()),
		"MatchTier": rpc.EnumMismatches[adminpb.ProspectMatchTier]([]model.ProspectMatchTier{
			model.MatchTaxID, model.MatchContact, model.MatchName,
		}),
	} {
		if len(mismatches) > 0 {
			t.Errorf("%s：%v", name, mismatches)
		}
	}
}

// 每個錯誤欄位對應都指到請求訊息裡真的存在的路徑（proto 改名時對應表要跟著改）。
func TestProspectFieldPathsExistInTheRequests(t *testing.T) {
	for name, c := range map[string]struct {
		desc  protoreflect.MessageDescriptor
		paths rpc.FieldPaths
	}{
		"create":  {(&adminpb.CreateProspectRequest{}).ProtoReflect().Descriptor(), prospectFieldPaths},
		"update":  {(&adminpb.UpdateProspectRequest{}).ProtoReflect().Descriptor(), prospectFieldPaths},
		"preview": {(&adminpb.PreviewProspectRequest{}).ProtoReflect().Descriptor(), previewProspectPaths},
		"list":    {(&adminpb.ListProspectsRequest{}).ProtoReflect().Descriptor(), listProspectsPaths},
		"archive": {(&adminpb.ArchiveProspectRequest{}).ProtoReflect().Descriptor(), archiveProspectPaths},
		"merge":   {(&adminpb.MergeProspectsRequest{}).ProtoReflect().Descriptor(), mergeProspectsPaths},
		"convert": {(&adminpb.ConvertProspectRequest{}).ProtoReflect().Descriptor(), convertProspectPaths},
	} {
		if missing := rpc.CheckFieldPaths(c.desc, c.paths); len(missing) > 0 {
			t.Errorf("%s 缺少 %v", name, missing)
		}
	}
}

// 寫入的指令都用呼叫的管理員，ID 由字串解析，confirmed_distinct_prospect_ids 轉成整數。
func TestProspectCommandsUseTheCallingAdmin(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	ctx := adminCtx(42)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := h.CreateProspect(ctx, connect.NewRequest(&adminpb.CreateProspectRequest{
		Prospect: &adminpb.Prospect{
			DisplayName: "王記", VendorType: adminpb.VendorType_VENDOR_TYPE_BUSINESS, TaxId: "22223223", PriorityStars: 2, Etag: "9", Status: adminpb.ProspectStatus_PROSPECT_STATUS_CONVERTED,
			Contacts:    []*adminpb.ProspectContact{{Kind: adminpb.ContactKind_CONTACT_KIND_PHONE, Value: "0912345678"}},
			CategoryIds: []string{"c1"},
		},
		ConfirmedDistinctProspectIds: []string{"3", "8"},
	}))
	must(err)
	c := m.create
	if c.AdminID != 42 || c.Fields.DisplayName != "王記" || c.Fields.VendorType != model.VendorBusiness || *c.Fields.TaxID != "22223223" || c.Fields.PriorityStars != 2 ||
		c.Fields.OutreachStatus != "" || c.Fields.Category != nil || c.Fields.Region != nil || len(c.Contacts) != 1 || c.Contacts[0].Kind != model.ContactPhone ||
		!slices.Equal(c.CategoryIDs, []string{"c1"}) || !slices.Equal(c.ConfirmedDistinct, []int64{3, 8}) {
		t.Fatalf("create=%+v", c)
	}
	_, err = h.ArchiveProspect(ctx, connect.NewRequest(&adminpb.ArchiveProspectRequest{Id: "9", Etag: "4"}))
	must(err)
	_, err = h.RestoreProspect(ctx, connect.NewRequest(&adminpb.RestoreProspectRequest{Id: "9", ConfirmedDistinctProspectIds: []string{"2"}}))
	must(err)
	_, err = h.MergeProspects(ctx, connect.NewRequest(&adminpb.MergeProspectsRequest{WinnerId: "9", WinnerEtag: "4", LoserId: "10", LoserEtag: "6"}))
	must(err)
	_, err = h.ConvertProspect(ctx, connect.NewRequest(&adminpb.ConvertProspectRequest{Id: "9", StoreId: "77", Etag: "4"}))
	must(err)
	if a := m.archive; a.AdminID != 42 || a.ProspectID != 9 || a.ExpectedVersion != 4 {
		t.Errorf("archive=%+v", a)
	}
	if r := m.restore; r.AdminID != 42 || r.ProspectID != 9 || !slices.Equal(r.ConfirmedDistinct, []int64{2}) {
		t.Errorf("restore=%+v", r)
	}
	if g := m.merge; g.AdminID != 42 || g.WinnerID != 9 || g.WinnerVersion != 4 || g.LoserID != 10 || g.LoserVersion != 6 {
		t.Errorf("merge=%+v", g)
	}
	if v := m.convert; v.AdminID != 42 || v.ProspectID != 9 || v.StoreID != 77 || v.ExpectedVersion != 4 {
		t.Errorf("convert=%+v", v)
	}
}

// 格式不對的 ID 與沒有登入的管理員在呼叫模組之前就被擋下。
func TestProspectCommandsRejectBadIDsBeforeTheModule(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	ctx := adminCtx(42)
	create := func(confirmed ...string) error {
		_, err := h.CreateProspect(ctx, connect.NewRequest(&adminpb.CreateProspectRequest{
			Prospect: &adminpb.Prospect{DisplayName: "x", VendorType: adminpb.VendorType_VENDOR_TYPE_INDIVIDUAL}, ConfirmedDistinctProspectIds: confirmed,
		}))
		return err
	}
	for _, bad := range []string{"", "abc", "0", "-3", "1.5"} {
		wantInvalidArgument(t, create(bad))
		_, err := h.ArchiveProspect(ctx, connect.NewRequest(&adminpb.ArchiveProspectRequest{Id: bad, Etag: "1"}))
		wantInvalidArgument(t, err)
		_, err = h.GetProspect(ctx, connect.NewRequest(&adminpb.GetProspectRequest{Id: bad}))
		wantInvalidArgument(t, err)
		_, err = h.ConvertProspect(ctx, connect.NewRequest(&adminpb.ConvertProspectRequest{Id: "1", StoreId: bad, Etag: "1"}))
		wantInvalidArgument(t, err)
		_, err = h.MergeProspects(ctx, connect.NewRequest(&adminpb.MergeProspectsRequest{WinnerId: "1", WinnerEtag: "1", LoserId: bad, LoserEtag: "1"}))
		wantInvalidArgument(t, err)
	}
	_, err := h.CreateProspect(ctx, connect.NewRequest(&adminpb.CreateProspectRequest{}))
	wantInvalidArgument(t, err)
	_, err = h.CreateProspect(context.Background(), connect.NewRequest(&adminpb.CreateProspectRequest{Prospect: &adminpb.Prospect{}}))
	if err == nil {
		t.Fatal("沒有管理員不能建立")
	}
	if m.calls != 0 {
		t.Fatalf("被拒絕的請求不能呼叫模組：%d", m.calls)
	}
}

// Update、Archive、Merge（兩邊）、Convert 的 etag 必填：沒帶回 invalid_argument、不呼叫模組；Restore 不帶 etag。
func TestProspectStateCommandsRequireAnEtagExceptRestore(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	ctx := adminCtx(42)
	_, err := h.UpdateProspect(ctx, connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: &adminpb.Prospect{Id: "9", DisplayName: "x"}}))
	wantInvalidArgument(t, err)
	_, err = h.ArchiveProspect(ctx, connect.NewRequest(&adminpb.ArchiveProspectRequest{Id: "9"}))
	wantInvalidArgument(t, err)
	_, err = h.MergeProspects(ctx, connect.NewRequest(&adminpb.MergeProspectsRequest{WinnerId: "9", LoserId: "10", LoserEtag: "1"}))
	wantInvalidArgument(t, err)
	_, err = h.MergeProspects(ctx, connect.NewRequest(&adminpb.MergeProspectsRequest{WinnerId: "9", WinnerEtag: "1", LoserId: "10"}))
	wantInvalidArgument(t, err)
	_, err = h.ConvertProspect(ctx, connect.NewRequest(&adminpb.ConvertProspectRequest{Id: "9", StoreId: "1"}))
	wantInvalidArgument(t, err)
	if m.calls != 0 {
		t.Fatalf("沒有 etag 的指令不能呼叫模組：%d", m.calls)
	}
	// 沒帶 etag 的錯誤指到 etag 欄位。
	_, err = h.ArchiveProspect(ctx, connect.NewRequest(&adminpb.ArchiveProspectRequest{Id: "9"}))
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "etag" {
		t.Fatalf("err=%v", err)
	}
	if _, err := h.RestoreProspect(ctx, connect.NewRequest(&adminpb.RestoreProspectRequest{Id: "9"})); err != nil {
		t.Fatal(err)
	}
	if m.calls != 1 {
		t.Fatalf("Restore 不帶 etag 也要送到模組：%d", m.calls)
	}
}

// 版本衝突原樣轉出（ABORTED 與 VERSION_CONFLICT 由 rpc 的轉換點負責，wire 上的結果在 integration 測試）。
func TestProspectVersionConflictPassesThroughAsTheModuleClassified(t *testing.T) {
	m := &prospectModule{err: apperr.VersionConflict("業者已被修改")}
	h := prospectHandler(t, m)
	_, err := h.ArchiveProspect(adminCtx(1), connect.NewRequest(&adminpb.ArchiveProspectRequest{Id: "9", Etag: "1"}))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindVersionConflict || ae.Reason != apperr.ReasonVersionConflict {
		t.Fatalf("err=%v", err)
	}
}

// 模組帶 reason 與 metadata 的錯誤原樣轉出（reason 由模組附上，傳輸層沒有特例；wire 上的 ErrorInfo 在 integration 測試）。
func TestProspectModuleErrorsWithReasonPassThroughUnchanged(t *testing.T) {
	matched := apperr.FailedPrecondition("相符").WithReason(apperr.ReasonProspectMatchFound).WithMetadata("prospectIds", "3,7,12")
	m := &prospectModule{err: matched}
	h := prospectHandler(t, m)
	valid := &adminpb.CreateProspectRequest{Prospect: &adminpb.Prospect{DisplayName: "x", VendorType: adminpb.VendorType_VENDOR_TYPE_INDIVIDUAL}}
	_, err := h.CreateProspect(adminCtx(1), connect.NewRequest(valid))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae != matched {
		t.Fatalf("create err=%v", err)
	}
	if _, err := h.RestoreProspect(adminCtx(1), connect.NewRequest(&adminpb.RestoreProspectRequest{Id: "9"})); !errors.As(err, &ae) || ae != matched {
		t.Fatalf("restore err=%v", err)
	}
	if _, err := h.UpdateProspect(adminCtx(1), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: &adminpb.Prospect{Id: "9", Etag: "1"}})); !errors.As(err, &ae) || ae != matched {
		t.Fatalf("update err=%v", err)
	}
}

// UpdateProspect 依 update_mask 只送選到的欄位：沒選到是保留（nil），選到卻沒有值是清掉；OUTPUT_ONLY、IDENTIFIER 與 etag
// 列在 mask 裡也不當成資料。
func TestUpdateProspectSendsOnlyTheMaskedWritableFields(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	update := func(p *adminpb.Prospect, paths ...string) error {
		t.Helper()
		_, err := h.UpdateProspect(adminCtx(9), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: p, UpdateMask: mask(paths...)}))
		return err
	}
	// 只選 display_name 與 region：其他欄位即使請求裡有值也不送。
	if err := update(&adminpb.Prospect{Id: "9", Etag: "4", DisplayName: "新名", Region: "花蓮", Category: "不採用", PriorityStars: 5}, "display_name", "region"); err != nil {
		t.Fatal(err)
	}
	u := m.update
	if u.AdminID != 9 || u.ProspectID != 9 || u.ExpectedVersion != 4 || *u.DisplayName != "新名" || *u.Region != "花蓮" ||
		u.Category != nil || u.PriorityStars != nil || u.TaxID != nil || u.Contacts != nil || u.CategoryIDs != nil || u.Logo != nil || u.Cover != nil ||
		u.VendorType != nil || u.OutreachStatus != nil || u.ProductSummary != nil || u.Positioning != nil || u.ResearchSummary != nil || u.PriorityLetter != nil {
		t.Fatalf("update=%+v", u)
	}
	// 選到卻沒有值：文字是空字串（清掉），聯絡方式與分類是空清單、星數是 0。
	if err := update(&adminpb.Prospect{Id: "9", Etag: "4"}, "tax_id", "category", "contacts", "category_ids", "priority_stars"); err != nil {
		t.Fatal(err)
	}
	u = m.update
	if u.TaxID == nil || *u.TaxID != "" || u.Category == nil || *u.Category != "" || u.Contacts == nil || len(*u.Contacts) != 0 ||
		u.CategoryIDs == nil || len(*u.CategoryIDs) != 0 || u.PriorityStars == nil || *u.PriorityStars != 0 || u.DisplayName != nil {
		t.Fatalf("clear=%+v", u)
	}
	// 聯絡方式整批替換。
	if err := update(&adminpb.Prospect{Id: "9", Etag: "4", Contacts: []*adminpb.ProspectContact{
		{Kind: adminpb.ContactKind_CONTACT_KIND_EMAIL, Value: "a@example.com", Primary: true}, {Kind: adminpb.ContactKind_CONTACT_KIND_SOCIAL, Value: "@wang"},
	}}, "contacts"); err != nil {
		t.Fatal(err)
	}
	if c := *m.update.Contacts; len(c) != 2 || c[0] != (model.ContactPoint{Kind: model.ContactEmail, Value: "a@example.com", Primary: true}) || c[1].Kind != model.ContactSocial {
		t.Fatalf("contacts=%+v", c)
	}
	// 沒帶 mask：只有有值的欄位。
	_, err := h.UpdateProspect(adminCtx(9), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: &adminpb.Prospect{Id: "9", Etag: "4", Positioning: "高價"}}))
	if err != nil {
		t.Fatal(err)
	}
	if u = m.update; *u.Positioning != "高價" || u.DisplayName != nil || u.Contacts != nil || u.PriorityStars != nil {
		t.Fatalf("implicit=%+v", u)
	}
	// OUTPUT_ONLY、IDENTIFIER、etag 列在 mask 裡也不回錯、不送。
	if err := update(&adminpb.Prospect{Id: "9", Etag: "4", Status: adminpb.ProspectStatus_PROSPECT_STATUS_ARCHIVED, MergedIntoId: "3"}, "status", "merged_into_id", "id", "etag", "logo_image", "convert_time"); err != nil {
		t.Fatal(err)
	}
	u = m.update
	if u.ExpectedVersion != 4 || u.DisplayName != nil || u.VendorType != nil || u.OutreachStatus != nil || u.Logo != nil {
		t.Fatalf("outputonly=%+v", u)
	}
	// 沒有任何可寫的欄位被選到也照送給模組（模組沒有改變時不寫入）。
	calls := m.calls
	wantInvalidArgument(t, update(&adminpb.Prospect{Id: "9", Etag: "4"}, "no_such_field"))
	wantInvalidArgument(t, update(&adminpb.Prospect{Id: "9", Etag: "4"}, "contacts.value"))
	wantInvalidArgument(t, update(&adminpb.Prospect{Id: "9", DisplayName: "x"}, "display_name"))
	if m.calls != calls {
		t.Fatal("被拒絕的修改不能呼叫模組")
	}
}

// 列了 outreach_status 卻沒有值不能默默重設成 NOT_CONTACTED（模組把空值當成預設值）；有值時照送。
func TestUpdateProspectNeverTreatsAnUnsetOutreachStatusAsAValue(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	send := func(p *adminpb.Prospect, paths ...string) error {
		_, err := h.UpdateProspect(adminCtx(9), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: p, UpdateMask: mask(paths...)}))
		return err
	}
	wantInvalidArgument(t, send(&adminpb.Prospect{Id: "9", Etag: "4"}, "outreach_status"))
	if m.calls != 0 {
		t.Fatal("不能呼叫模組")
	}
	if err := send(&adminpb.Prospect{Id: "9", Etag: "4", OutreachStatus: adminpb.OutreachStatus_OUTREACH_STATUS_INVITED}, "outreach_status"); err != nil {
		t.Fatal(err)
	}
	if got := m.update.OutreachStatus; got == nil || *got != model.OutreachInvited {
		t.Fatalf("outreach=%v", got)
	}
	// 沒有值的 vendor_type 交給模組回報不合法，不當成有效值。
	if err := send(&adminpb.Prospect{Id: "9", Etag: "4"}, "vendor_type"); err != nil {
		t.Fatal(err)
	}
	if got := m.update.VendorType; got == nil || *got != "" {
		t.Fatalf("vendor=%v", got)
	}
}

// INPUT_ONLY 的上傳：列進 mask 才送，帶 staged token 與來源網址；沒列就沿用；列了卻沒有上傳是自相矛盾；建立時不收。
func TestUpdateProspectImageUploadsAreInputOnlyAndNeedTheMask(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	send := func(p *adminpb.Prospect, paths ...string) error {
		_, err := h.UpdateProspect(adminCtx(9), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: p, UpdateMask: mask(paths...)}))
		return err
	}
	up := &adminpb.ProspectImageUpload{StagedToken: "tok-logo", SourceUrl: "https://example.com/p"}
	if err := send(&adminpb.Prospect{Id: "9", Etag: "4", LogoUpload: up, CoverUpload: &adminpb.ProspectImageUpload{StagedToken: "tok-cover"}}, "logo_upload"); err != nil {
		t.Fatal(err)
	}
	if got := m.update; got.Logo == nil || *got.Logo != (model.ProspectImageUpload{StagedToken: "tok-logo", SourceURL: "https://example.com/p"}) || got.Cover != nil {
		t.Fatalf("logo=%+v cover=%+v", got.Logo, got.Cover)
	}
	if err := send(&adminpb.Prospect{Id: "9", Etag: "4", LogoUpload: up, CoverUpload: &adminpb.ProspectImageUpload{StagedToken: "tok-cover"}}, "logo_upload", "cover_upload"); err != nil {
		t.Fatal(err)
	}
	if got := m.update; got.Logo == nil || got.Cover == nil || got.Cover.StagedToken != "tok-cover" || got.Cover.SourceURL != "" {
		t.Fatalf("both=%+v %+v", got.Logo, got.Cover)
	}
	// 沒帶 mask：有上傳就換。
	if _, err := h.UpdateProspect(adminCtx(9), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: &adminpb.Prospect{Id: "9", Etag: "4", CoverUpload: up}})); err != nil {
		t.Fatal(err)
	}
	if got := m.update; got.Logo != nil || got.Cover == nil || got.Cover.StagedToken != "tok-logo" {
		t.Fatalf("implicit=%+v %+v", got.Logo, got.Cover)
	}
	calls := m.calls
	wantInvalidArgument(t, send(&adminpb.Prospect{Id: "9", Etag: "4"}, "logo_upload"))
	wantInvalidArgument(t, send(&adminpb.Prospect{Id: "9", Etag: "4"}, "cover_upload"))
	_, err := h.CreateProspect(adminCtx(9), connect.NewRequest(&adminpb.CreateProspectRequest{
		Prospect: &adminpb.Prospect{DisplayName: "x", VendorType: adminpb.VendorType_VENDOR_TYPE_BUSINESS, LogoUpload: up},
	}))
	wantInvalidArgument(t, err)
	if m.calls != calls {
		t.Fatal("被拒絕的請求不能呼叫模組")
	}
}

// 清單的 page token 綁在篩選上；篩選照轉給模組；UNSPECIFIED 的篩選不送；解得開但內容不合法的 token 被拒絕。
func TestListProspectsBindsThePageTokenToTheFilters(t *testing.T) {
	cursor := &model.ProspectCursor{UpdatedAt: prospectNow, ProspectID: 9}
	m := &prospectModule{page: model.ProspectPage{Items: []model.Prospect{fullProspect()}, Next: cursor}}
	h := prospectHandler(t, m)
	req := &adminpb.ListProspectsRequest{
		PageSize: 1, Status: adminpb.ProspectStatus_PROSPECT_STATUS_ARCHIVED, VendorType: adminpb.VendorType_VENDOR_TYPE_BUSINESS,
		Category: "蔬果", Region: "宜蘭", Keyword: "王",
	}
	got, err := h.ListProspects(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.list; *in.Status != model.ProspectStatusArchived || *in.VendorType != model.VendorBusiness || in.OutreachStatus != nil ||
		in.Category != "蔬果" || in.Region != "宜蘭" || in.Keyword != "王" || in.PageSize != 1 {
		t.Fatalf("in=%+v", in)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListProspects(adminCtx(1), connect.NewRequest(req)); err != nil || m.list.After == nil || *m.list.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.list.After, err)
	}
	req.Keyword = "李"
	_, err = h.ListProspects(adminCtx(1), connect.NewRequest(req))
	wantInvalidArgument(t, err)
	req.Keyword = "王"
	req.PageToken = "不是 token"
	_, err = h.ListProspects(adminCtx(1), connect.NewRequest(req))
	wantInvalidArgument(t, err)
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerProspectAdminServiceListProspectsProcedure, req.GetStatus().String(), req.GetOutreachStatus().String(), req.GetVendorType().String(), req.GetCategory(), req.GetRegion(), req.GetKeyword())
	req.PageToken = rpc.TokenFor(scope, &model.ProspectCursor{})
	_, err = h.ListProspects(adminCtx(1), connect.NewRequest(req))
	wantInvalidArgument(t, err)
}

// 重複與統編對帳的 token 同樣要有效；回傳的每一筆帶兩邊或店家的欄位。
func TestProspectPairListsMapEveryFieldAndRejectEmptyCursors(t *testing.T) {
	left, right := fullProspect(), fullProspect()
	right.ID = 10
	m := &prospectModule{
		dupPage: model.DuplicatePage{Items: []model.DuplicatePair{{Left: left, Right: right, MatchedOn: []model.MatchReason{model.ReasonEmail, model.ReasonName}}}, Next: &model.DuplicateCursor{LeftID: 9, RightID: 10}},
		taxPage: model.TaxIDMatchPage{Items: []model.TaxIDMatch{{Prospect: left, StoreID: 70, StoreName: str("店"), CompanyName: str("王記有限公司"), KYCStatus: "APPROVED"}}, Next: &model.TaxIDMatchCursor{ProspectID: 9, StoreID: 70}},
	}
	h := prospectHandler(t, m)
	dup, err := h.ListProspectDuplicates(adminCtx(1), connect.NewRequest(&adminpb.ListProspectDuplicatesRequest{PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if d := dup.Msg.GetDuplicates(); len(d) != 1 || d[0].GetLeft().GetId() != "9" || d[0].GetRight().GetId() != "10" || !slices.Equal(d[0].GetMatchedOn(), []adminpb.ProspectMatchReason{adminpb.ProspectMatchReason_PROSPECT_MATCH_REASON_EMAIL, adminpb.ProspectMatchReason_PROSPECT_MATCH_REASON_NAME}) || dup.Msg.GetNextPageToken() == "" {
		t.Fatalf("dup=%v", dup.Msg)
	}
	if _, err := h.ListProspectDuplicates(adminCtx(1), connect.NewRequest(&adminpb.ListProspectDuplicatesRequest{PageToken: dup.Msg.GetNextPageToken()})); err != nil || *m.dupAfter != (model.DuplicateCursor{LeftID: 9, RightID: 10}) {
		t.Fatalf("after=%v err=%v", m.dupAfter, err)
	}
	tax, err := h.ListProspectTaxIDMatches(adminCtx(1), connect.NewRequest(&adminpb.ListProspectTaxIDMatchesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if x := tax.Msg.GetMatches(); len(x) != 1 || x[0].GetStoreId() != "70" || x[0].GetStoreName() != "店" || x[0].GetCompanyName() != "王記有限公司" ||
		x[0].GetKycStatus() != pb.KYCStatus_KYC_STATUS_APPROVED || x[0].GetProspect().GetId() != "9" {
		t.Fatalf("tax=%v", tax.Msg)
	}
	if _, err := h.ListProspectTaxIDMatches(adminCtx(1), connect.NewRequest(&adminpb.ListProspectTaxIDMatchesRequest{PageToken: tax.Msg.GetNextPageToken()})); err != nil || *m.taxAfter != (model.TaxIDMatchCursor{ProspectID: 9, StoreID: 70}) {
		t.Fatalf("after=%v err=%v", m.taxAfter, err)
	}
	dupScope := rpc.FilterScope(sellerv1alpha1connect.SellerProspectAdminServiceListProspectDuplicatesProcedure)
	_, err = h.ListProspectDuplicates(adminCtx(1), connect.NewRequest(&adminpb.ListProspectDuplicatesRequest{PageToken: rpc.TokenFor(dupScope, &model.DuplicateCursor{})}))
	wantInvalidArgument(t, err)
	taxScope := rpc.FilterScope(sellerv1alpha1connect.SellerProspectAdminServiceListProspectTaxIDMatchesProcedure)
	_, err = h.ListProspectTaxIDMatches(adminCtx(1), connect.NewRequest(&adminpb.ListProspectTaxIDMatchesRequest{PageToken: rpc.TokenFor(taxScope, &model.TaxIDMatchCursor{})}))
	wantInvalidArgument(t, err)
}

// 預覽把請求原樣交給模組，回傳比對的層級與命中的鍵。
func TestPreviewProspectMapsTheMatches(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	got, err := h.PreviewProspect(adminCtx(1), connect.NewRequest(&adminpb.PreviewProspectRequest{
		DisplayName: "王記", TaxId: "22223223", Contacts: []*adminpb.ProspectContact{{Kind: adminpb.ContactKind_CONTACT_KIND_EMAIL, Value: "a@example.com"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.preview; in.DisplayName != "王記" || in.TaxID != "22223223" || len(in.Contacts) != 1 || in.Contacts[0].Kind != model.ContactEmail {
		t.Fatalf("preview=%+v", in)
	}
	if x := got.Msg.GetMatches(); len(x) != 1 || x[0].GetTier() != adminpb.ProspectMatchTier_PROSPECT_MATCH_TIER_T2_CONTACT || !slices.Equal(x[0].GetMatchedOn(), []adminpb.ProspectMatchReason{adminpb.ProspectMatchReason_PROSPECT_MATCH_REASON_EMAIL}) || x[0].GetProspect().GetId() != "9" {
		t.Fatalf("matches=%v", got.Msg)
	}
}

// 標記配對用呼叫的管理員，不帶 etag；ID 格式不對在呼叫模組之前就被擋下；模組的錯誤（同一個 ID 等）照欄位對應。
func TestMarkProspectsDistinctUsesTheCallingAdminAndValidatesIDs(t *testing.T) {
	m := &prospectModule{}
	h := prospectHandler(t, m)
	if _, err := h.MarkProspectsDistinct(adminCtx(42), connect.NewRequest(&adminpb.MarkProspectsDistinctRequest{FirstProspectId: "9", SecondProspectId: "3"})); err != nil {
		t.Fatal(err)
	}
	if got := m.mark; got != (model.MarkProspectsDistinctInput{FirstID: 9, SecondID: 3, AdminID: 42}) {
		t.Fatalf("mark=%+v", got)
	}
	calls := m.calls
	for _, bad := range []string{"", "abc", "0", "-1"} {
		_, err := h.MarkProspectsDistinct(adminCtx(42), connect.NewRequest(&adminpb.MarkProspectsDistinctRequest{FirstProspectId: bad, SecondProspectId: "3"}))
		wantInvalidArgument(t, err)
		_, err = h.MarkProspectsDistinct(adminCtx(42), connect.NewRequest(&adminpb.MarkProspectsDistinctRequest{FirstProspectId: "9", SecondProspectId: bad}))
		wantInvalidArgument(t, err)
	}
	if _, err := h.MarkProspectsDistinct(context.Background(), connect.NewRequest(&adminpb.MarkProspectsDistinctRequest{FirstProspectId: "9", SecondProspectId: "3"})); err == nil {
		t.Fatal("沒有管理員不能標記")
	}
	if m.calls != calls {
		t.Fatal("被拒絕的請求不能呼叫模組")
	}
	m.err = apperr.InvalidInputOn([]string{"SecondID"}, "不能把業者標記成與自己不是同一家")
	_, err := h.MarkProspectsDistinct(adminCtx(42), connect.NewRequest(&adminpb.MarkProspectsDistinctRequest{FirstProspectId: "9", SecondProspectId: "9"}))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
}

// 建立時的上傳被拒絕時，錯誤指到實際設定的那一個欄位。
func TestCreateProspectUploadRejectionNamesTheFieldThatWasSet(t *testing.T) {
	h := prospectHandler(t, &prospectModule{})
	up := &adminpb.ProspectImageUpload{StagedToken: "tok"}
	for path, p := range map[string]*adminpb.Prospect{
		"prospect.logo_upload":  {DisplayName: "x", VendorType: adminpb.VendorType_VENDOR_TYPE_BUSINESS, LogoUpload: up},
		"prospect.cover_upload": {DisplayName: "x", VendorType: adminpb.VendorType_VENDOR_TYPE_BUSINESS, CoverUpload: up},
	} {
		_, err := h.CreateProspect(adminCtx(1), connect.NewRequest(&adminpb.CreateProspectRequest{Prospect: p}))
		var ae *apperr.Error
		if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != path {
			t.Errorf("%s：err=%v", path, err)
		}
	}
}

// update_mask 的 "*" 選到所有可寫的欄位，包含上傳與 outreach_status，所以它們都必須有值；都有時全部送出。
func TestUpdateProspectWildcardMaskRequiresUploadsAndOutreachStatus(t *testing.T) {
	m := &prospectModule{prospect: fullProspect()}
	h := prospectHandler(t, m)
	send := func(p *adminpb.Prospect) error {
		_, err := h.UpdateProspect(adminCtx(9), connect.NewRequest(&adminpb.UpdateProspectRequest{Prospect: p, UpdateMask: mask("*")}))
		return err
	}
	up := &adminpb.ProspectImageUpload{StagedToken: "tok"}
	base := func() *adminpb.Prospect {
		return &adminpb.Prospect{Id: "9", Etag: "4", DisplayName: "x", VendorType: adminpb.VendorType_VENDOR_TYPE_BUSINESS, OutreachStatus: adminpb.OutreachStatus_OUTREACH_STATUS_INVITED, LogoUpload: up, CoverUpload: up}
	}
	for name, mutate := range map[string]func(*adminpb.Prospect){
		"沒有 outreach_status": func(p *adminpb.Prospect) { p.OutreachStatus = adminpb.OutreachStatus_OUTREACH_STATUS_UNSPECIFIED },
		"沒有 logo_upload":     func(p *adminpb.Prospect) { p.LogoUpload = nil },
		"沒有 cover_upload":    func(p *adminpb.Prospect) { p.CoverUpload = nil },
	} {
		p := base()
		mutate(p)
		wantInvalidArgument(t, send(p))
		if m.calls != 0 {
			t.Fatalf("%s：被拒絕的請求不能呼叫模組", name)
		}
	}
	if err := send(base()); err != nil {
		t.Fatal(err)
	}
	if u := m.update; u.Logo == nil || u.Cover == nil || *u.OutreachStatus != model.OutreachInvited || *u.DisplayName != "x" || u.Contacts == nil || u.CategoryIDs == nil {
		t.Fatalf("update=%+v", u)
	}
}

// 模組的命中原因與契約的列舉一一對應：對不上的值轉成 UNSPECIFIED 會被這裡擋下。
func TestMatchReasonsMatchTheContractEnum(t *testing.T) {
	if got := rpc.EnumMismatches[adminpb.ProspectMatchReason](model.MatchReasonValues()); len(got) != 0 {
		t.Fatalf("mismatches: %v", got)
	}
}
