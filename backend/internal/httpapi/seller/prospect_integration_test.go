//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminsellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func email(value string, primary bool) *adminsellerpb.ProspectContact {
	return &adminsellerpb.ProspectContact{Kind: adminsellerpb.ContactKind_CONTACT_KIND_EMAIL, Value: value, Primary: primary}
}

func newProspect(name string, contacts ...*adminsellerpb.ProspectContact) *adminsellerpb.Prospect {
	return &adminsellerpb.Prospect{DisplayName: name, VendorType: adminsellerpb.VendorType_VENDOR_TYPE_BUSINESS, Contacts: contacts}
}

func (f *adminFixture) createProspect(t *testing.T, p *adminsellerpb.Prospect, confirmed ...string) *adminsellerpb.Prospect {
	t.Helper()
	resp, err := f.prospects.CreateProspect(context.Background(), connect.NewRequest(&adminsellerpb.CreateProspectRequest{Prospect: p, ConfirmedDistinctProspectIds: confirmed}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetProspect()
}

func (f *adminFixture) getProspect(t *testing.T, id string) *adminsellerpb.Prospect {
	t.Helper()
	resp, err := f.prospects.GetProspect(context.Background(), connect.NewRequest(&adminsellerpb.GetProspectRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetProspect()
}

// errorInfoOf 取出錯誤的 ErrorInfo；沒有時回 nil。
func errorInfoOf(t *testing.T, err error) *errdetails.ErrorInfo {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 Connect 錯誤：%v", err)
	}
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			t.Fatal(e)
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			return info
		}
	}
	return nil
}

// 授權：沒有登入 unauthenticated、一般帳號 permission_denied；管理員的讀取以 GET 呼叫（NO_SIDE_EFFECTS）。
func TestProspectServiceRequiresAnAdmin(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	for name, c := range map[string]struct {
		client interface {
			ListProspects(context.Context, *connect.Request[adminsellerpb.ListProspectsRequest]) (*connect.Response[adminsellerpb.ListProspectsResponse], error)
		}
		code connect.Code
	}{"anonymous": {f.prospectsAnon, connect.CodeUnauthenticated}, "customer": {f.prospectsCustomer, connect.CodePermissionDenied}} {
		if _, err := c.client.ListProspects(ctx, connect.NewRequest(&adminsellerpb.ListProspectsRequest{})); connect.CodeOf(err) != c.code {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if _, err := f.prospectsCustomer.CreateProspect(ctx, connect.NewRequest(&adminsellerpb.CreateProspectRequest{Prospect: newProspect("x")})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("customer create err=%v", err)
	}
	f.createProspect(t, newProspect("王記農場"))
	list, err := f.prospects.ListProspects(ctx, connect.NewRequest(&adminsellerpb.ListProspectsRequest{}))
	if err != nil || len(list.Msg.GetProspects()) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if list.Msg.GetProspects()[0].GetContacts() != nil {
		t.Fatal("清單不帶 contacts")
	}
}

// 建立 → 預覽比對 → 撞到聯絡方式回 PROSPECT_MATCH_FOUND（帶相符的業者）→ 確認不是同一家才寫入；統編衝突回 PROSPECT_TAX_ID_HELD（帶持有者）。
func TestProspectCreatePreviewMatchAndConfirmThroughTheWire(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	a := f.createProspect(t, &adminsellerpb.Prospect{
		DisplayName: "王記農場", VendorType: adminsellerpb.VendorType_VENDOR_TYPE_BUSINESS, TaxId: "22223223", Category: "蔬果", PriorityStars: 2,
		Contacts: []*adminsellerpb.ProspectContact{email("hello@wang.example", true)}, CategoryIds: []string{"veg"},
	})
	if a.GetStatus() != adminsellerpb.ProspectStatus_PROSPECT_STATUS_ACTIVE || a.GetOutreachStatus() != adminsellerpb.OutreachStatus_OUTREACH_STATUS_NOT_CONTACTED ||
		a.GetEtag() == "" || a.GetCreatedByAdminId() != sid(f.adminID) || a.GetUpdatedByAdminId() != sid(f.adminID) || a.GetTaxId() != "22223223" ||
		len(a.GetContacts()) != 1 || !a.GetContacts()[0].GetPrimary() || a.GetCategoryIds()[0] != "veg" || a.GetLogoImage() != nil {
		t.Fatalf("a=%v", a)
	}
	got := f.getProspect(t, a.GetId())
	if got.GetEtag() != a.GetEtag() || got.GetDisplayName() != "王記農場" {
		t.Fatalf("get=%v", got)
	}

	preview, err := f.prospects.PreviewProspect(ctx, connect.NewRequest(&adminsellerpb.PreviewProspectRequest{DisplayName: "別名", Contacts: []*adminsellerpb.ProspectContact{email("hello@wang.example", false)}}))
	if err != nil {
		t.Fatal(err)
	}
	if m := preview.Msg.GetMatches(); len(m) != 1 || m[0].GetProspect().GetId() != a.GetId() || m[0].GetTier() != adminsellerpb.ProspectMatchTier_PROSPECT_MATCH_TIER_T2_CONTACT || len(m[0].GetMatchedOn()) == 0 {
		t.Fatalf("preview=%v", preview.Msg)
	}

	// 撞到聯絡方式：failed_precondition、PROSPECT_MATCH_FOUND、metadata 帶相符的業者；什麼都沒寫。
	b := newProspect("範例乙商行", email("hello@wang.example", true))
	_, err = f.prospects.CreateProspect(ctx, connect.NewRequest(&adminsellerpb.CreateProspectRequest{Prospect: b}))
	wantCode(t, err, connect.CodeFailedPrecondition, "PROSPECT_MATCH_FOUND")
	if info := errorInfoOf(t, err); info.GetMetadata()["prospectIds"] != a.GetId() || len(info.GetMetadata()) != 1 || info.GetDomain() != "hawkr" {
		t.Fatalf("info=%v", info)
	}
	if n := f.dbCountProspects(t); n != 1 {
		t.Fatalf("撞到比對時不能寫入：%d 筆", n)
	}
	// 確認的 ID 沒涵蓋全部相符的業者：照樣被擋。
	_, err = f.prospects.CreateProspect(ctx, connect.NewRequest(&adminsellerpb.CreateProspectRequest{Prospect: b, ConfirmedDistinctProspectIds: []string{"999999"}}))
	wantCode(t, err, connect.CodeFailedPrecondition, "PROSPECT_MATCH_FOUND")
	// 格式不對的 ID。
	_, err = f.prospects.CreateProspect(ctx, connect.NewRequest(&adminsellerpb.CreateProspectRequest{Prospect: b, ConfirmedDistinctProspectIds: []string{"abc"}}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "confirmed_distinct_prospect_ids[0]" {
		t.Fatalf("field=%q", field)
	}
	confirmed := f.createProspect(t, b, a.GetId())
	if confirmed.GetId() == a.GetId() || f.dbCountProspects(t) != 2 {
		t.Fatalf("confirmed=%v", confirmed)
	}
	// 標記過的配對不再擋：同一個聯絡方式再建立第三家，只相符第二家以外的新業者；a 與 b 的配對已記下。
	// 統編衝突：already_exists、PROSPECT_TAX_ID_HELD，metadata 的 prospectIds 是持有者。
	dup := &adminsellerpb.Prospect{DisplayName: "另一家", VendorType: adminsellerpb.VendorType_VENDOR_TYPE_BUSINESS, TaxId: "22223223"}
	_, err = f.prospects.CreateProspect(ctx, connect.NewRequest(&adminsellerpb.CreateProspectRequest{Prospect: dup}))
	wantCode(t, err, connect.CodeAlreadyExists, "PROSPECT_TAX_ID_HELD")
	if info := errorInfoOf(t, err); info.GetMetadata()["prospectIds"] != a.GetId() || len(info.GetMetadata()) != 1 || info.GetDomain() != "hawkr" {
		t.Fatalf("info=%v", info)
	}
	// 修改撞到統編：failed_precondition，同一個 reason 與 metadata。
	_, err = f.prospects.UpdateProspect(ctx, connect.NewRequest(&adminsellerpb.UpdateProspectRequest{
		Prospect: &adminsellerpb.Prospect{Id: confirmed.GetId(), Etag: confirmed.GetEtag(), TaxId: "22223223"}, UpdateMask: mask("tax_id"),
	}))
	wantCode(t, err, connect.CodeFailedPrecondition, "PROSPECT_TAX_ID_HELD")
	if info := errorInfoOf(t, err); info.GetMetadata()["prospectIds"] != a.GetId() || len(info.GetMetadata()) != 1 {
		t.Fatalf("info=%v", info)
	}
}

func (f *adminFixture) dbCountProspects(t *testing.T) int64 {
	t.Helper()
	var n int64
	f.scan(t, &n, `SELECT count(*) FROM seller.prospects`)
	return n
}

// 修改：mask、etag 衝突、聯絡方式整批替換撞到比對、以 INPUT_ONLY 的 logo_upload 換圖（media 替身轉正）、沒列進 mask 就沿用。
func TestUpdateProspectThroughTheWireWithImageUploads(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	other := f.createProspect(t, newProspect("甲商行", email("jia@example.com", true)))
	a := f.createProspect(t, newProspect("乙商行", email("yi@example.com", true)))
	update := func(p *adminsellerpb.Prospect, confirmed []string, paths ...string) (*adminsellerpb.Prospect, error) {
		resp, err := f.prospects.UpdateProspect(ctx, connect.NewRequest(&adminsellerpb.UpdateProspectRequest{Prospect: p, UpdateMask: mask(paths...), ConfirmedDistinctProspectIds: confirmed}))
		if err != nil {
			return nil, err
		}
		return resp.Msg.GetProspect(), nil
	}

	logo := &adminsellerpb.ProspectImageUpload{StagedToken: "tok-logo", SourceUrl: "https://source.example.com/logo"}
	withLogo, err := update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: a.GetEtag(), DisplayName: "乙商行（新）", Region: "宜蘭", LogoUpload: logo, Category: "不採用"}, nil, "display_name", "logo_upload", "region")
	if err != nil {
		t.Fatal(err)
	}
	wantLogo := "https://media.example.test/i/{preset}/" + media.RefTypeSellerProspectLogo + "/tok-logo"
	if withLogo.GetLogoImage().GetUriTemplate() != wantLogo || withLogo.GetLogoSourceUrl() != "https://source.example.com/logo" || withLogo.GetCoverImage() != nil ||
		withLogo.GetDisplayName() != "乙商行（新）" || withLogo.GetRegion() != "宜蘭" || withLogo.GetCategory() != "" || withLogo.GetEtag() == a.GetEtag() ||
		withLogo.GetLogoUpload() != nil || len(withLogo.GetContacts()) != 1 {
		t.Fatalf("withLogo=%v", withLogo)
	}

	// 沒列進 mask：圖片沿用；etag 過期：aborted、VERSION_CONFLICT、什麼都不寫。
	kept, err := update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: withLogo.GetEtag(), DisplayName: "乙商行（再改）"}, nil, "display_name")
	if err != nil || kept.GetLogoImage().GetUriTemplate() != wantLogo {
		t.Fatalf("kept=%v err=%v", kept, err)
	}
	_, err = update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: withLogo.GetEtag(), DisplayName: "過期"}, nil, "display_name")
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	if f.getProspect(t, a.GetId()).GetDisplayName() != "乙商行（再改）" {
		t.Fatal("過期的修改不能寫入")
	}

	// 來源網址只收 https：錯誤指到欄位；被拒絕的上傳沒有換圖。
	_, err = update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: kept.GetEtag(), CoverUpload: &adminsellerpb.ProspectImageUpload{StagedToken: "tok-cover", SourceUrl: "http://insecure.example.com"}}, nil, "cover_upload")
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "prospect.cover_upload.source_url" {
		t.Fatalf("field=%q", field)
	}
	// 列了 mask 卻沒有上傳：自相矛盾。
	_, err = update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: kept.GetEtag()}, nil, "cover_upload")
	wantCode(t, err, connect.CodeInvalidArgument, "")
	if f.getProspect(t, a.GetId()).GetCoverImage() != nil {
		t.Fatal("被拒絕的上傳不能換圖")
	}

	// 聯絡方式整批替換撞到其他業者：PROSPECT_MATCH_FOUND；確認不是同一家之後寫入，並且之後不再擋。
	clash := &adminsellerpb.Prospect{Id: a.GetId(), Etag: kept.GetEtag(), Contacts: []*adminsellerpb.ProspectContact{email("jia@example.com", true), email("yi@example.com", false)}}
	_, err = update(clash, nil, "contacts")
	wantCode(t, err, connect.CodeFailedPrecondition, "PROSPECT_MATCH_FOUND")
	if info := errorInfoOf(t, err); info.GetMetadata()["prospectIds"] != other.GetId() {
		t.Fatalf("info=%v", info)
	}
	replaced, err := update(clash, []string{other.GetId()}, "contacts")
	if err != nil || len(replaced.GetContacts()) != 2 {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	// 把聯絡方式清空（mask 列了、沒有值）。
	cleared, err := update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: replaced.GetEtag()}, nil, "contacts", "tax_id")
	if err != nil || len(cleared.GetContacts()) != 0 {
		t.Fatalf("cleared=%v err=%v", cleared, err)
	}
	// 沒有任何改變：etag 不變。
	same, err := update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: cleared.GetEtag(), DisplayName: cleared.GetDisplayName()}, nil, "display_name")
	if err != nil || same.GetEtag() != cleared.GetEtag() {
		t.Fatalf("same=%v err=%v", same, err)
	}
	// 輸入的值不合法：指到欄位、不當成有效值（招商進度列了 mask 卻沒有值）。
	_, err = update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: same.GetEtag()}, nil, "outreach_status")
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "prospect.outreach_status" {
		t.Fatalf("field=%q", field)
	}
	_, err = update(&adminsellerpb.Prospect{Id: a.GetId(), Etag: same.GetEtag(), OutreachStatus: adminsellerpb.OutreachStatus_OUTREACH_STATUS_CONVERTED}, nil, "outreach_status")
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "prospect.outreach_status" {
		t.Fatalf("field=%q", field)
	}
}

// 封存、還原、合併、轉成店家：etag 的規則與結果。
func TestProspectArchiveRestoreMergeConvertThroughTheWire(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()

	// 封存與還原：封存必帶 etag，還原不帶。
	c := f.createProspect(t, newProspect("丙商行"))
	_, err := f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: c.GetId()}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
		t.Fatalf("field=%q", field)
	}
	_, err = f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: c.GetId(), Etag: "999"}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	archived, err := f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: c.GetId(), Etag: c.GetEtag()}))
	if err != nil || archived.Msg.GetProspect().GetStatus() != adminsellerpb.ProspectStatus_PROSPECT_STATUS_ARCHIVED {
		t.Fatalf("archived=%v err=%v", archived, err)
	}
	// 封存生效後帶舊 etag 再送一次：aborted，不寫入（不推斷重送）。
	_, err = f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: c.GetId(), Etag: c.GetEtag()}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	// 帶目前的 etag 封存已封存的業者：回傳目前的業者。
	again, err := f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: c.GetId(), Etag: archived.Msg.GetProspect().GetEtag()}))
	if err != nil || again.Msg.GetProspect().GetEtag() != archived.Msg.GetProspect().GetEtag() {
		t.Fatalf("again=%v err=%v", again, err)
	}
	listArchived, err := f.prospects.ListProspects(ctx, connect.NewRequest(&adminsellerpb.ListProspectsRequest{Status: adminsellerpb.ProspectStatus_PROSPECT_STATUS_ARCHIVED}))
	if err != nil || len(listArchived.Msg.GetProspects()) != 1 {
		t.Fatalf("listArchived=%v err=%v", listArchived, err)
	}
	restored, err := f.prospects.RestoreProspect(ctx, connect.NewRequest(&adminsellerpb.RestoreProspectRequest{Id: c.GetId()}))
	if err != nil || restored.Msg.GetProspect().GetStatus() != adminsellerpb.ProspectStatus_PROSPECT_STATUS_ACTIVE {
		t.Fatalf("restored=%v err=%v", restored, err)
	}

	// 合併：兩邊的 etag 都必填；過期的那一邊 aborted；成功後輸家封存、指向贏家，聯絡方式併入。
	winner := f.createProspect(t, newProspect("贏家商行", email("win@example.com", true)))
	loser := f.createProspect(t, &adminsellerpb.Prospect{
		DisplayName: "輸家商行", VendorType: adminsellerpb.VendorType_VENDOR_TYPE_BUSINESS, TaxId: "22223223", Contacts: []*adminsellerpb.ProspectContact{email("lose@example.com", true)},
	})
	merge := func(winnerEtag, loserEtag string) (*adminsellerpb.Prospect, error) {
		resp, err := f.prospects.MergeProspects(ctx, connect.NewRequest(&adminsellerpb.MergeProspectsRequest{WinnerId: winner.GetId(), WinnerEtag: winnerEtag, LoserId: loser.GetId(), LoserEtag: loserEtag}))
		if err != nil {
			return nil, err
		}
		return resp.Msg.GetProspect(), nil
	}
	_, err = merge("", loser.GetEtag())
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "winner_etag" {
		t.Fatalf("field=%q", field)
	}
	_, err = merge(winner.GetEtag(), "")
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "loser_etag" {
		t.Fatalf("field=%q", field)
	}
	_, err = merge(winner.GetEtag(), "999")
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = f.prospects.MergeProspects(ctx, connect.NewRequest(&adminsellerpb.MergeProspectsRequest{WinnerId: winner.GetId(), WinnerEtag: winner.GetEtag(), LoserId: winner.GetId(), LoserEtag: winner.GetEtag()}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	merged, err := merge(winner.GetEtag(), loser.GetEtag())
	if err != nil {
		t.Fatal(err)
	}
	if merged.GetId() != winner.GetId() || len(merged.GetContacts()) != 2 || merged.GetTaxId() != "22223223" || merged.GetEtag() == winner.GetEtag() {
		t.Fatalf("merged=%v", merged)
	}
	// 合併生效後帶舊 etag 再送一次：aborted，不寫入。
	_, err = merge(winner.GetEtag(), loser.GetEtag())
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	gone := f.getProspect(t, loser.GetId())
	if gone.GetStatus() != adminsellerpb.ProspectStatus_PROSPECT_STATUS_ARCHIVED || gone.GetMergedIntoId() != winner.GetId() || gone.GetTaxId() != "" {
		t.Fatalf("gone=%v", gone)
	}
	_, err = f.prospects.RestoreProspect(ctx, connect.NewRequest(&adminsellerpb.RestoreProspectRequest{Id: loser.GetId()}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	if errorInfoOf(t, err) != nil {
		t.Fatal("被合併的不能還原：沒有 reason")
	}

	// 轉成店家：etag 必填；成功後狀態與進度都是 CONVERTED；轉成之後帶舊 etag 再送是 aborted；店家被別的業者認領是 already_exists。
	_, store := f.newSeller(t, "王記商店")
	convert := func(id, etag string) (*adminsellerpb.Prospect, error) {
		resp, err := f.prospects.ConvertProspect(ctx, connect.NewRequest(&adminsellerpb.ConvertProspectRequest{Id: id, StoreId: sid(store.StoreID), Etag: etag}))
		if err != nil {
			return nil, err
		}
		return resp.Msg.GetProspect(), nil
	}
	_, err = convert(merged.GetId(), "")
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
		t.Fatalf("field=%q", field)
	}
	_, err = f.prospects.ConvertProspect(ctx, connect.NewRequest(&adminsellerpb.ConvertProspectRequest{Id: merged.GetId(), StoreId: "999999", Etag: merged.GetEtag()}))
	wantCode(t, err, connect.CodeNotFound, "")
	converted, err := convert(merged.GetId(), merged.GetEtag())
	if err != nil {
		t.Fatal(err)
	}
	if converted.GetStatus() != adminsellerpb.ProspectStatus_PROSPECT_STATUS_CONVERTED || converted.GetOutreachStatus() != adminsellerpb.OutreachStatus_OUTREACH_STATUS_CONVERTED ||
		converted.GetConvertedStoreId() != sid(store.StoreID) || converted.GetConvertTime() == nil {
		t.Fatalf("converted=%v", converted)
	}
	_, err = convert(merged.GetId(), merged.GetEtag())
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = convert(merged.GetId(), converted.GetEtag())
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	third := f.createProspect(t, newProspect("丁商行"))
	_, err = convert(third.GetId(), third.GetEtag())
	wantCode(t, err, connect.CodeAlreadyExists, "")
	_, err = f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: merged.GetId(), Etag: converted.GetEtag()}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
}

// 疑似重複的清單：keyset 分頁，翻頁不漏不重；統編對帳可以呼叫（沒有對得上的店家時是空的）。
func TestProspectDuplicatesPageWithoutGapsOrRepeats(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	for i := range 3 {
		f.createProspect(t, newProspect("同名商行", email("dup"+string(rune('a'+i))+"@example.com", true)))
	}
	f.createProspect(t, newProspect("獨一無二商行"))
	type pair struct{ left, right string }
	var seen []pair
	token := ""
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("分頁沒有結束")
		}
		resp, err := f.prospects.ListProspectDuplicates(ctx, connect.NewRequest(&adminsellerpb.ListProspectDuplicatesRequest{PageSize: 2, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range resp.Msg.GetDuplicates() {
			seen = append(seen, pair{d.GetLeft().GetId(), d.GetRight().GetId()})
			if d.GetLeft().GetContacts() != nil {
				t.Fatal("清單不帶 contacts")
			}
		}
		if token = resp.Msg.GetNextPageToken(); token == "" {
			break
		}
		if len(resp.Msg.GetDuplicates()) != 2 {
			t.Fatalf("非最後一頁要滿頁：%d", len(resp.Msg.GetDuplicates()))
		}
	}
	if len(seen) != 3 {
		t.Fatalf("三家同名是 3 對：%v", seen)
	}
	for i, p := range seen {
		if p.left == p.right || slices.Contains(seen[:i], p) {
			t.Fatalf("重複或自己對自己：%v", seen)
		}
	}
	// 換了 token 的來源（別的清單的 token）不能用。
	_, err := f.prospects.ListProspectDuplicates(ctx, connect.NewRequest(&adminsellerpb.ListProspectDuplicatesRequest{PageToken: "!!!"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	taxes, err := f.prospects.ListProspectTaxIDMatches(ctx, connect.NewRequest(&adminsellerpb.ListProspectTaxIDMatchesRequest{}))
	if err != nil || len(taxes.Msg.GetMatches()) != 0 || taxes.Msg.GetNextPageToken() != "" {
		t.Fatalf("taxes=%v err=%v", taxes, err)
	}
}

// 統編對帳：業者的統編對得上目前店家的 KYC 申請時列出（帶店家與申請的欄位），轉成該店家之後不再列。
func TestProspectTaxIDMatchesListTheUnconnectedStoreAndDropItAfterConversion(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, store := f.newSeller(t, "王記商店")
	if _, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{AccountID: accountID, SellerType: ptr(seller.KYCCompany), CompanyName: ptr("王記有限公司"), TaxID: ptr("33330333")}); err != nil {
		t.Fatal(err)
	}
	p := f.createProspect(t, &adminsellerpb.Prospect{DisplayName: "王記有限公司", VendorType: adminsellerpb.VendorType_VENDOR_TYPE_BUSINESS, TaxId: "33330333"})
	taxes, err := f.prospects.ListProspectTaxIDMatches(ctx, connect.NewRequest(&adminsellerpb.ListProspectTaxIDMatchesRequest{}))
	if err != nil || len(taxes.Msg.GetMatches()) != 1 {
		t.Fatalf("taxes=%v err=%v", taxes, err)
	}
	m := taxes.Msg.GetMatches()[0]
	if m.GetProspect().GetId() != p.GetId() || m.GetStoreId() != sid(store.StoreID) || m.GetStoreName() != "王記商店" || m.GetKycStatus() != pb.KYCStatus_KYC_STATUS_DRAFT {
		t.Fatalf("m=%v", m)
	}
	if _, err := f.prospects.ConvertProspect(ctx, connect.NewRequest(&adminsellerpb.ConvertProspectRequest{Id: p.GetId(), StoreId: sid(store.StoreID), Etag: p.GetEtag()})); err != nil {
		t.Fatal(err)
	}
	taxes, err = f.prospects.ListProspectTaxIDMatches(ctx, connect.NewRequest(&adminsellerpb.ListProspectTaxIDMatchesRequest{}))
	if err != nil || len(taxes.Msg.GetMatches()) != 0 {
		t.Fatalf("轉成店家之後不再列：%v err=%v", taxes, err)
	}
}

// REQUIRED 是契約的宣告：從 descriptor 找出每個 REQUIRED 欄位，清掉之後錯誤要指出它的路徑（API 設計規範 §四）。
func TestProspectRequiredFieldsAreEnforced(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	existing := f.createProspect(t, newProspect("既有商行"))
	valid := map[string]func() proto.Message{
		"CreateProspect": func() proto.Message {
			return &adminsellerpb.CreateProspectRequest{Prospect: newProspect("新商行", email("new@example.com", true))}
		},
		"UpdateProspect": func() proto.Message {
			return &adminsellerpb.UpdateProspectRequest{
				Prospect: &adminsellerpb.Prospect{
					Id: existing.GetId(), Etag: existing.GetEtag(), Contacts: []*adminsellerpb.ProspectContact{email("x@example.com", true)},
					LogoUpload: &adminsellerpb.ProspectImageUpload{StagedToken: "tok"},
				},
				UpdateMask: mask("contacts", "logo_upload"),
			}
		},
		"GetProspect": func() proto.Message { return &adminsellerpb.GetProspectRequest{Id: existing.GetId()} },
		"ArchiveProspect": func() proto.Message {
			return &adminsellerpb.ArchiveProspectRequest{Id: existing.GetId(), Etag: existing.GetEtag()}
		},
		"RestoreProspect": func() proto.Message { return &adminsellerpb.RestoreProspectRequest{Id: existing.GetId()} },
		"MarkProspectsDistinct": func() proto.Message {
			return &adminsellerpb.MarkProspectsDistinctRequest{FirstProspectId: existing.GetId(), SecondProspectId: "9999"}
		},
		"MergeProspects": func() proto.Message {
			return &adminsellerpb.MergeProspectsRequest{WinnerId: existing.GetId(), WinnerEtag: "1", LoserId: "9999", LoserEtag: "1"}
		},
		"ConvertProspect": func() proto.Message {
			return &adminsellerpb.ConvertProspectRequest{Id: existing.GetId(), StoreId: "9999", Etag: existing.GetEtag()}
		},
	}
	call := func(m proto.Message) error {
		var err error
		switch r := m.(type) {
		case *adminsellerpb.CreateProspectRequest:
			_, err = f.prospects.CreateProspect(ctx, connect.NewRequest(r))
		case *adminsellerpb.UpdateProspectRequest:
			_, err = f.prospects.UpdateProspect(ctx, connect.NewRequest(r))
		case *adminsellerpb.GetProspectRequest:
			_, err = f.prospects.GetProspect(ctx, connect.NewRequest(r))
		case *adminsellerpb.ArchiveProspectRequest:
			_, err = f.prospects.ArchiveProspect(ctx, connect.NewRequest(r))
		case *adminsellerpb.RestoreProspectRequest:
			_, err = f.prospects.RestoreProspect(ctx, connect.NewRequest(r))
		case *adminsellerpb.MarkProspectsDistinctRequest:
			_, err = f.prospects.MarkProspectsDistinct(ctx, connect.NewRequest(r))
		case *adminsellerpb.MergeProspectsRequest:
			_, err = f.prospects.MergeProspects(ctx, connect.NewRequest(r))
		case *adminsellerpb.ConvertProspectRequest:
			_, err = f.prospects.ConvertProspect(ctx, connect.NewRequest(r))
		default:
			t.Fatalf("沒有處理 %T", m)
		}
		return err
	}
	svc := adminsellerpb.File_hawkr_admin_seller_v1alpha1_prospect_proto.Services().ByName("SellerProspectAdminService")
	checked := 0
	for name := range valid {
		method := svc.Methods().ByName(protoreflect.Name(name))
		if method == nil {
			t.Fatalf("service 沒有 %s", name)
		}
		// 每個 REQUIRED 的請求欄位與資源欄位（prospect.*）、聯絡方式與上傳的子欄位，各清掉一個、其餘照舊。
		var targets []struct{ path []string }
		walkRequired(method.Input(), nil, 0, func(path []string) { targets = append(targets, struct{ path []string }{path}) })
		for _, target := range targets {
			// display_name 與 vendor_type 是建立時必填（Prospect 的欄位註解）；修改只在 update_mask 選到時才用它們。
			if name == "UpdateProspect" && slices.Contains([]string{"prospect.display_name", "prospect.vendor_type"}, joinPath(target.path)) {
				continue
			}
			msg := valid[name]()
			if !clearAt(msg.ProtoReflect(), target.path) {
				continue
			}
			err := call(msg)
			if err == nil {
				t.Errorf("%s 清掉 %s 之後請求還是成功", name, joinPath(target.path))
				continue
			}
			field := strings.ReplaceAll(wantCode(t, err, connect.CodeInvalidArgument, ""), "[0]", "")
			if want := joinPath(target.path); field != want && (len(target.path) <= 1 || field != joinPath(target.path[:len(target.path)-1])) {
				t.Errorf("%s 清掉 %s：錯誤指到 %q", name, want, field)
			}
			checked++
		}
	}
	if checked < 15 {
		t.Fatalf("只檢查了 %d 個 REQUIRED 欄位，descriptor 的走訪壞了", checked)
	}
	// 有效的請求照常成功（確認上面的失敗是清掉欄位造成的）。
	if err := call(valid["GetProspect"]()); err != nil {
		t.Fatal(err)
	}
	if err := call(valid["CreateProspect"]()); err != nil {
		t.Fatal(err)
	}
}

// walkRequired 走訪訊息的 REQUIRED 欄位（含 prospect、contacts、上傳等子訊息裡的 REQUIRED 欄位）。
func walkRequired(md protoreflect.MessageDescriptor, prefix []string, depth int, visit func([]string)) {
	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		path := append(slices.Clone(prefix), string(fd.Name()))
		if hasRequired(fd) {
			visit(path)
		}
		if fd.Kind() == protoreflect.MessageKind && depth < 2 && fd.Message().FullName() != "google.protobuf.FieldMask" && fd.Message().FullName() != "google.protobuf.Timestamp" &&
			fd.Message().FullName() != "hawkr.common.v1alpha1.Image" {
			walkRequired(fd.Message(), path, depth+1, visit)
		}
	}
}

func hasRequired(fd protoreflect.FieldDescriptor) bool {
	behaviors, _ := proto.GetExtension(fd.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
	return slices.Contains(behaviors, annotations.FieldBehavior_REQUIRED)
}

// clearAt 清掉路徑上的欄位；路徑經過 repeated 時清第一個元素；路徑上的訊息不存在時回 false（沒有東西可清）。
func clearAt(m protoreflect.Message, path []string) bool {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(path[0]))
	if len(path) == 1 {
		m.Clear(fd)
		return true
	}
	switch {
	case fd.IsList():
		list := m.Get(fd).List()
		return list.Len() > 0 && clearAt(list.Get(0).Message(), path[1:])
	case !m.Has(fd):
		return false
	}
	return clearAt(m.Mutable(fd).Message(), path[1:])
}

func joinPath(path []string) string {
	out := ""
	for i, p := range path {
		if i > 0 {
			out += "."
		}
		out += p
	}
	return out
}

// 直接標記配對：名稱相同的候選不必合併就能處理，標記後從清單消失；重送成功；同一個 ID、不存在、不是進行中的被拒絕；一般帳號不能標記。
func TestMarkProspectsDistinctRemovesTheNameOnlyPairFromTheDuplicatesList(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	a := f.createProspect(t, newProspect("同名商行", email("a@example.com", true)))
	b := f.createProspect(t, newProspect("同名商行", email("b@example.com", true)))
	pairs := func() int {
		resp, err := f.prospects.ListProspectDuplicates(ctx, connect.NewRequest(&adminsellerpb.ListProspectDuplicatesRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		return len(resp.Msg.GetDuplicates())
	}
	if pairs() != 1 {
		t.Fatalf("名稱相同的一對要在清單裡")
	}
	mark := func(x, y string) error {
		_, err := f.prospects.MarkProspectsDistinct(ctx, connect.NewRequest(&adminsellerpb.MarkProspectsDistinctRequest{FirstProspectId: x, SecondProspectId: y}))
		return err
	}
	if err := mark(b.GetId(), a.GetId()); err != nil {
		t.Fatal(err)
	}
	if pairs() != 0 {
		t.Fatal("標記之後這一對要從清單消失")
	}
	if err := mark(a.GetId(), b.GetId()); err != nil {
		t.Fatalf("重送要成功：%v", err)
	}
	var n int64
	f.scan(t, &n, `SELECT count(*) FROM seller.prospect_distinct_pairs`)
	var admin int64
	f.scan(t, &admin, `SELECT confirmed_by_admin_id FROM seller.prospect_distinct_pairs`)
	if n != 1 || admin != f.adminID {
		t.Fatalf("pairs=%d admin=%d", n, admin)
	}
	// 業者本身沒有被動到（沒有 etag 也不改版本）。
	if got := f.getProspect(t, a.GetId()); got.GetEtag() != a.GetEtag() {
		t.Fatalf("標記不能改業者的版本：%s → %s", a.GetEtag(), got.GetEtag())
	}
	if field := wantCode(t, mark(a.GetId(), a.GetId()), connect.CodeInvalidArgument, ""); field != "second_prospect_id" {
		t.Fatalf("field=%q", field)
	}
	wantCode(t, mark(a.GetId(), "999999"), connect.CodeNotFound, "")
	wantCode(t, mark("abc", a.GetId()), connect.CodeInvalidArgument, "")
	archived, err := f.prospects.ArchiveProspect(ctx, connect.NewRequest(&adminsellerpb.ArchiveProspectRequest{Id: b.GetId(), Etag: b.GetEtag()}))
	if err != nil {
		t.Fatal(err)
	}
	c := f.createProspect(t, newProspect("丙商行"))
	wantCode(t, mark(c.GetId(), archived.Msg.GetProspect().GetId()), connect.CodeFailedPrecondition, "")
	if _, err := f.prospectsCustomer.MarkProspectsDistinct(ctx, connect.NewRequest(&adminsellerpb.MarkProspectsDistinctRequest{FirstProspectId: a.GetId(), SecondProspectId: c.GetId()})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("customer err=%v", err)
	}
}

// update_mask 的 "*" 要求上傳與 outreach_status 都有值（API 規範 §七：所有可寫的欄位）。
func TestUpdateProspectWildcardMaskThroughTheWire(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	p := f.createProspect(t, newProspect("丁商行", email("d@example.com", true)))
	update := func(m *adminsellerpb.Prospect) error {
		_, err := f.prospects.UpdateProspect(ctx, connect.NewRequest(&adminsellerpb.UpdateProspectRequest{Prospect: m, UpdateMask: mask("*")}))
		return err
	}
	up := &adminsellerpb.ProspectImageUpload{StagedToken: "tok"}
	full := func() *adminsellerpb.Prospect {
		return &adminsellerpb.Prospect{
			Id: p.GetId(), Etag: p.GetEtag(), DisplayName: "丁商行（新）", VendorType: adminsellerpb.VendorType_VENDOR_TYPE_BUSINESS,
			OutreachStatus: adminsellerpb.OutreachStatus_OUTREACH_STATUS_INVITED, Contacts: p.GetContacts(), LogoUpload: up, CoverUpload: up,
		}
	}
	noOutreach := full()
	noOutreach.OutreachStatus = adminsellerpb.OutreachStatus_OUTREACH_STATUS_UNSPECIFIED
	if field := wantCode(t, update(noOutreach), connect.CodeInvalidArgument, ""); field != "prospect.outreach_status" {
		t.Fatalf("field=%q", field)
	}
	noLogo := full()
	noLogo.LogoUpload = nil
	if field := wantCode(t, update(noLogo), connect.CodeInvalidArgument, ""); field != "prospect.logo_upload" {
		t.Fatalf("field=%q", field)
	}
	if got := f.getProspect(t, p.GetId()); got.GetEtag() != p.GetEtag() {
		t.Fatal("被拒絕的請求不能寫入")
	}
	if err := update(full()); err != nil {
		t.Fatal(err)
	}
	got := f.getProspect(t, p.GetId())
	if got.GetDisplayName() != "丁商行（新）" || got.GetLogoImage() == nil || got.GetCoverImage() == nil || got.GetOutreachStatus() != adminsellerpb.OutreachStatus_OUTREACH_STATUS_INVITED {
		t.Fatalf("got=%v", got)
	}
}
