//go:build integration

package seller_test

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/gcis"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	adminsellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

// submittedKYC 為一家新店建立並送出一份個人賣家的申請（沒有補充文件）。
func (f *adminFixture) submittedKYC(t *testing.T, name string) (int64, seller.Store, seller.KYCApplication) {
	t.Helper()
	accountID, s := f.newSeller(t, name)
	ctx := context.Background()
	draft, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{AccountID: accountID, SellerType: ptr(seller.KYCIndividual), ProductType: ptr("蔬果")})
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := f.module.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version})
	if err != nil {
		t.Fatal(err)
	}
	return accountID, s, submitted
}

// submittedCompanyKYC 為一家新店建立並送出一份帶兩張公司補充文件的企業賣家申請；文件放進 media 的替身（內容是 "doc<位置>-bytes"）。
func (f *adminFixture) submittedCompanyKYC(t *testing.T, name, taxID string) (int64, seller.Store, seller.KYCApplication) {
	t.Helper()
	accountID, s := f.newSeller(t, name)
	ctx := context.Background()
	draft, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{
		AccountID: accountID, SellerType: ptr(seller.KYCCompany), ProductType: ptr("蔬果"), CompanyName: ptr(name + "有限公司"), TaxID: ptr(taxID),
		CompanyDocuments: &[]seller.KYCDocumentRef{{StagedToken: "doc0-" + name}, {StagedToken: "doc1-" + name}},
	})
	if err != nil {
		t.Fatal(err)
	}
	submitted, err := f.module.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range submitted.Documents {
		f.media.PutSecure(d.ObjectKey, media.Ref{Type: media.RefTypeCertification, ID: submitted.ID.String()},
			media.SecureAsset{Data: []byte("doc" + strconv.Itoa(d.Position) + "-bytes"), ContentType: "image/jpeg"})
	}
	return accountID, s, submitted
}

func (f *adminFixture) getKYC(t *testing.T, id string) *adminsellerpb.KYCApplication {
	t.Helper()
	resp, err := f.admin.GetKYCApplication(context.Background(), connect.NewRequest(&adminsellerpb.GetKYCApplicationRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetApplication()
}

// documentGET 以 token 讀補充文件端點；token 空是沒有登入。
func (f *adminFixture) documentGET(t *testing.T, token, path string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

// 審核佇列、單筆檢視（讀申請本身不寫讀取紀錄）、核准與退回（etag、理由、欄位、審核者是呼叫的管理員）。
func TestAdminReviewsKYCOverConnect(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, storeA, appA := f.submittedKYC(t, "甲店")
	_, _, appB := f.submittedCompanyKYC(t, "乙店", "33330333")

	// 佇列：審核中的舊到新（甲先送出）；列名是店名（D37）；讀申請不寫任何讀取紀錄。
	queue, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{Status: pb.KYCStatus_KYC_STATUS_SUBMITTED, PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if len(queue.Msg.GetApplications()) != 1 || queue.Msg.GetApplications()[0].GetId() != appA.ID.String() || queue.Msg.GetNextPageToken() == "" ||
		queue.Msg.GetApplications()[0].GetStoreId() != sid(storeA.StoreID) || queue.Msg.GetApplications()[0].GetSubmitTime() == nil || queue.Msg.GetApplications()[0].GetStoreName() != "甲店" {
		t.Fatalf("queue=%+v", queue.Msg)
	}
	if got := queue.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
	page2, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{Status: pb.KYCStatus_KYC_STATUS_SUBMITTED, PageSize: 1, PageToken: queue.Msg.GetNextPageToken()}))
	if err != nil || len(page2.Msg.GetApplications()) != 1 || page2.Msg.GetApplications()[0].GetId() != appB.ID.String() || page2.Msg.GetNextPageToken() != "" {
		t.Fatalf("page2=%+v err=%v", page2, err)
	}
	for _, req := range map[string]*adminsellerpb.ListKYCApplicationsRequest{
		"draft": {Status: pb.KYCStatus_KYC_STATUS_DRAFT}, "other filter": {Query: "x", PageToken: queue.Msg.GetNextPageToken(), Status: pb.KYCStatus_KYC_STATUS_SUBMITTED},
	} {
		_, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(req))
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
	found, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{Query: "乙店"}))
	if err != nil || len(found.Msg.GetApplications()) != 1 || found.Msg.GetApplications()[0].GetId() != appB.ID.String() {
		t.Fatalf("found=%+v err=%v", found, err)
	}

	got, err := f.admin.GetKYCApplication(ctx, connect.NewRequest(&adminsellerpb.GetKYCApplicationRequest{Id: appA.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	app := got.Msg.GetApplication()
	if len(app.GetDocuments()) != 0 || app.GetIndividual() == nil || app.GetStatus() != pb.KYCStatus_KYC_STATUS_SUBMITTED || len(app.GetReviews()) != 1 ||
		got.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("application=%+v", app)
	}
	// 企業申請的補充文件：每張帶檢視網址與 asset_id，不帶儲存的 object key。
	gotB := f.getKYC(t, appB.ID.String())
	raw, _ := protojson.Marshal(gotB)
	if len(gotB.GetDocuments()) != 2 || strings.Contains(string(raw), "7111/") {
		t.Fatalf("company application=%s", raw)
	}
	for i, d := range gotB.GetDocuments() {
		if d.GetViewUri() != "/api/files/admin/seller/kyc_applications/"+appB.ID.String()+"/documents/"+strconv.Itoa(i) || !strings.HasPrefix(d.GetAssetId(), "asset-doc") ||
			d.GetField() != pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS || int(d.GetPosition()) != i {
			t.Fatalf("document=%+v", d)
		}
	}
	var audits int64
	f.scan(t, &audits, "SELECT count(*) FROM seller.kyc_document_accesses")
	if audits != 0 {
		t.Fatalf("讀申請不能寫讀取紀錄：%d", audits)
	}
	if _, err := f.admin.GetKYCApplication(ctx, connect.NewRequest(&adminsellerpb.GetKYCApplicationRequest{Id: "not-a-uuid"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := f.admin.GetKYCApplication(ctx, connect.NewRequest(&adminsellerpb.GetKYCApplicationRequest{Id: "0197d1a2-0000-7000-8000-000000000000"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("missing: %v", err)
	}

	// 核准：沒帶 etag、舊 etag、過長的 comment 都被拒絕，什麼都不寫。
	_, err = f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: appA.ID.String()}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
		t.Fatalf("field=%q", field)
	}
	_, err = f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: appA.ID.String(), Etag: "1.0"}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: appA.ID.String(), Etag: app.GetEtag(), Comment: strings.Repeat("理", 1001)}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "comment" {
		t.Fatalf("field=%q", field)
	}
	if again := f.getKYC(t, appA.ID.String()); again.GetStatus() != pb.KYCStatus_KYC_STATUS_SUBMITTED || len(again.GetReviews()) != 1 {
		t.Fatalf("a refused approval wrote something: %+v", again)
	}
	approved, err := f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: appA.ID.String(), Etag: app.GetEtag(), Comment: "資料齊全"}))
	if err != nil {
		t.Fatal(err)
	}
	a := approved.Msg.GetApplication()
	if a.GetStatus() != pb.KYCStatus_KYC_STATUS_APPROVED || a.GetDecideTime() == nil || len(a.GetReviews()) != 2 ||
		a.GetReviews()[1].GetAction() != adminsellerpb.KYCReviewAction_KYC_REVIEW_ACTION_APPROVE || a.GetReviews()[1].GetReviewerAdminId() != sid(f.adminID) || a.GetReviews()[1].GetReason() != "資料齊全" {
		t.Fatalf("approved=%+v", a)
	}
	// 核准生效後帶舊 etag 再核准或退回：aborted，不寫入（不推斷重送）。
	_, err = f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: appA.ID.String(), Etag: app.GetEtag()}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = f.admin.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{Id: appA.ID.String(), Etag: app.GetEtag(), Reason: "x", Issues: anIssue()}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	// 已核准的不能再退回。
	_, err = f.admin.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{Id: appA.ID.String(), Etag: a.GetEtag(), Reason: "x", Issues: anIssue()}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")

	// 退回：理由必填；逐項問題至少一項，欄位與原因碼不能是 UNSPECIFIED，文件要是這份申請的，違規指到 issues[i] 的欄位。
	bApp := f.getKYC(t, appB.ID.String())
	_, err = f.admin.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{Id: appB.ID.String(), Etag: bApp.GetEtag()}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "reason" {
		t.Fatalf("field=%q", field)
	}
	reject := func(issues ...*pb.KYCIssue) (*connect.Response[adminsellerpb.RejectKYCResponse], error) {
		return f.admin.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{Id: appB.ID.String(), Etag: bApp.GetEtag(), Reason: "x", Issues: issues}))
	}
	_, err = reject()
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues" {
		t.Fatalf("no issues: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].field" {
		t.Fatalf("unspecified field: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_PRODUCT_TYPE, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].code" {
		t.Fatalf("unspecified code: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_PRODUCT_TYPE, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].message" {
		t.Fatalf("no message: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_PRODUCT_TYPE, DocumentAssetId: "asset-x", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].document_asset_id" {
		t.Fatalf("asset on a non document: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, DocumentAssetId: "asset-not-here", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].document_asset_id" {
		t.Fatalf("foreign asset: field=%q", field)
	}
	rejected, err := f.admin.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{
		Id: appB.ID.String(), Etag: bApp.GetEtag(), Reason: "文件不清楚", InternalNote: "疑似翻拍",
		Issues: []*pb.KYCIssue{
			{Field: pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISSING, Message: "請補上公司登記文件"},
			{Field: pb.KYCField_KYC_FIELD_BUSINESS_TAX_ID, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH, Message: "與登記的統編不符"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	r := rejected.Msg.GetApplication()
	last := r.GetReviews()[len(r.GetReviews())-1]
	if r.GetStatus() != pb.KYCStatus_KYC_STATUS_REJECTED || last.GetAction() != adminsellerpb.KYCReviewAction_KYC_REVIEW_ACTION_REJECT || last.GetReason() != "文件不清楚" ||
		len(last.GetIssues()) != 2 || last.GetIssues()[1].GetCode() != pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH || last.GetIssues()[1].GetMessage() != "與登記的統編不符" ||
		last.GetInternalNote() != "疑似翻拍" || len(r.GetOutstandingIssues()) != 2 || last.GetReviewerAdminId() != sid(f.adminID) {
		t.Fatalf("rejected=%+v", r)
	}
	// 佇列：兩筆都離開審核中，已核准與已退回各一。
	if left, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{Status: pb.KYCStatus_KYC_STATUS_SUBMITTED})); err != nil || len(left.Msg.GetApplications()) != 0 {
		t.Fatalf("left=%+v err=%v", left, err)
	}
}

// 補充文件的讀取（D36）：有後台的 KYC 讀取權限就能讀（不要理由、不要限時授權），沒有登入、買家、沒有角色的管理員讀不到；每次讀取都寫一列
// 讀取紀錄（誰、哪一張、何時），寫不進去就不送內容；回應不可快取。
// mutation：端點不檢查 adminperm.SellerKYCDocumentsGet → 沒有角色的管理員那段失敗；不寫讀取紀錄 → 紀錄筆數的斷言失敗；先送內容再寫紀錄 → fail closed 那段失敗。
func TestKYCDocumentsNeedTheKYCReadPermissionAndEveryReadIsRecorded(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, _, app := f.submittedCompanyKYC(t, "甲店", "33330333")
	_, _, individualApp := f.submittedKYC(t, "乙店")
	doc := func(a seller.KYCApplication, position string) string {
		return "/api/files/admin/seller/kyc_applications/" + a.ID.String() + "/documents/" + position
	}
	count := func() int64 {
		var n int64
		f.scan(t, &n, "SELECT count(*) FROM seller.kyc_document_accesses")
		return n
	}

	// 沒登入 401、買家 403：沒有紀錄、媒體沒有被問。
	for name, c := range map[string]struct {
		token string
		want  int
	}{"visitor": {"", 401}, "customer": {f.customerToken, 403}} {
		code, header, body := f.documentGET(t, c.token, doc(app, "0"))
		if code != c.want || strings.Contains(body, "-bytes") || header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s：code=%d body=%q", name, code, body)
		}
	}
	if n := count(); n != 0 {
		t.Fatalf("accesses=%d", n)
	}

	// 有權限的管理員直接讀得到（不需要先顯示全文）；圖片的標頭固定；寫一列紀錄。
	code, header, body := f.documentGET(t, f.adminToken, doc(app, "0"))
	if code != 200 || body != "doc0-bytes" {
		t.Fatalf("code=%d body=%q", code, body)
	}
	for k, v := range map[string]string{"Content-Type": "image/jpeg", "Cache-Control": "private, no-store", "Content-Disposition": "inline", "X-Content-Type-Options": "nosniff"} {
		if header.Get(k) != v {
			t.Fatalf("%s = %q，要 %q", k, header.Get(k), v)
		}
	}
	if n := count(); n != 1 {
		t.Fatalf("每次讀取各一列：%d", n)
	}
	// 另一位有權限的管理員讀同一張：各寫各的一列；同一位再讀一次也是新的一列。
	if code, _, body := f.documentGET(t, f.otherAdminToken, doc(app, "1")); code != 200 || body != "doc1-bytes" {
		t.Fatalf("另一位管理員：code=%d body=%q", code, body)
	}
	if code, _, _ := f.documentGET(t, f.adminToken, doc(app, "0")); code != 200 {
		t.Fatalf("再讀一次：code=%d", code)
	}
	var admins int64
	f.scan(t, &admins, "SELECT count(DISTINCT admin_id) FROM seller.kyc_document_accesses WHERE application_id = $1", app.ID)
	if n := count(); n != 3 || admins != 2 {
		t.Fatalf("accesses=%d admins=%d", n, admins)
	}
	// 沒有這個位置、格式不對的位置、沒有補充文件的個人申請：404，不寫紀錄、內容不外流。
	for name, path := range map[string]string{"no such position": doc(app, "2"), "bad position": doc(app, "selfie"), "negative position": doc(app, "-1"), "individual": doc(individualApp, "0")} {
		code, _, body := f.documentGET(t, f.adminToken, path)
		if code != 404 || strings.Contains(body, "-bytes") {
			t.Fatalf("%s：code=%d body=%q", name, code, body)
		}
	}
	if n := count(); n != 3 {
		t.Fatalf("找不到的請求不寫讀取紀錄：%d", n)
	}
	// 角色被撤銷之後讀不到（權限在入口檢查），沒有讀取紀錄；角色補回來又讀得到。
	ownerChange := identity.AdminRoleChangeInput{AdminID: f.adminID, Role: adminperm.RoleOwner, Reason: "測試", Source: identity.AdminRoleSourceCLI}
	if _, err := f.id.RevokeAdminRole(ctx, ownerChange); err != nil {
		t.Fatal(err)
	}
	code, _, body = f.documentGET(t, f.adminToken, doc(app, "0"))
	if code != 403 || strings.Contains(body, "-bytes") {
		t.Fatalf("沒有角色的管理員：code=%d body=%q", code, body)
	}
	if n := count(); n != 3 {
		t.Fatalf("被擋下的請求不寫讀取紀錄：%d", n)
	}
	if _, err := f.id.GrantAdminRole(ctx, ownerChange); err != nil {
		t.Fatal(err)
	}
	// 讀取紀錄寫不進去：不送內容。
	if _, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses ADD CONSTRAINT test_no_writes CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	code, _, body = f.documentGET(t, f.adminToken, doc(app, "1"))
	if code != 500 || strings.Contains(body, "-bytes") {
		t.Fatalf("fail closed：code=%d body=%q", code, body)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses DROP CONSTRAINT test_no_writes"); err != nil {
		t.Fatal(err)
	}
	// 讀取紀錄不在申請檢視裡，另外分頁：誰、哪一張、何時；新到舊。
	listed, err := f.admin.ListKYCDocumentAccesses(ctx, connect.NewRequest(&adminsellerpb.ListKYCDocumentAccessesRequest{Id: app.ID.String(), PageSize: 2}))
	if err != nil {
		t.Fatal(err)
	}
	ac := listed.Msg.GetDocumentAccesses()
	if len(ac) != 2 || listed.Msg.GetNextPageToken() == "" || ac[0].GetAdminId() != sid(f.adminID) || ac[0].GetPosition() != 0 ||
		ac[0].GetField() != pb.KYCField_KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS || ac[0].GetAccessTime() == nil || !strings.HasPrefix(ac[0].GetAssetId(), "asset-doc0") {
		t.Fatalf("accesses=%+v token=%q", ac, listed.Msg.GetNextPageToken())
	}
	_, err = f.admin.ListKYCDocumentAccesses(ctx, connect.NewRequest(&adminsellerpb.ListKYCDocumentAccessesRequest{Id: "0197d1a2-0000-7000-8000-000000000000"}))
	wantCode(t, err, connect.CodeNotFound, "")
	var recordedAsset string
	f.scan(t, &recordedAsset, "SELECT asset_id FROM seller.kyc_document_accesses WHERE position = 1 LIMIT 1")
	if recordedAsset != "asset-doc1-甲店" {
		t.Fatalf("讀取紀錄記實際送出的 asset_id：%q", recordedAsset)
	}
	// 申請檢視不內嵌讀取紀錄。
	if view := f.getKYC(t, app.ID.String()); strings.Contains(protoText(t, view), "accessTime") {
		t.Fatalf("view=%s", protoText(t, view))
	}
}

// 新的後台方法都只有管理員能呼叫：沒登入是 unauthenticated，買家是 permission_denied，什麼都不會被寫入。
func TestAdminKYCMethodsRejectCustomersAndVisitorsAndWriteNothing(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, _, app := f.submittedKYC(t, "甲店")
	for name, c := range map[string]adminSellerClient{"customer": f.customer, "visitor": f.anonymous} {
		want := connect.CodePermissionDenied
		if name == "visitor" {
			want = connect.CodeUnauthenticated
		}
		calls := map[string]func() error{
			"ListKYCApplications": func() error {
				_, err := c.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{}))
				return err
			},
			"GetKYCApplication": func() error {
				_, err := c.GetKYCApplication(ctx, connect.NewRequest(&adminsellerpb.GetKYCApplicationRequest{Id: app.ID.String()}))
				return err
			},
			"ApproveKYC": func() error {
				_, err := c.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: app.ID.String(), Etag: "2.0"}))
				return err
			},
			"RejectKYC": func() error {
				_, err := c.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{Id: app.ID.String(), Etag: "2.0", Reason: "越權"}))
				return err
			},
			"ListKYCDocumentAccesses": func() error {
				_, err := c.ListKYCDocumentAccesses(ctx, connect.NewRequest(&adminsellerpb.ListKYCDocumentAccessesRequest{Id: app.ID.String()}))
				return err
			},
			"RequeryKYCRegistration": func() error {
				_, err := c.RequeryKYCRegistration(ctx, connect.NewRequest(&adminsellerpb.RequeryKYCRegistrationRequest{Id: app.ID.String()}))
				return err
			},
		}
		for method, call := range calls {
			if err := call(); connect.CodeOf(err) != want {
				t.Fatalf("%s %s: err=%v want=%v", name, method, err, want)
			}
		}
	}
	var status string
	var accesses int64
	f.scan(t, &status, "SELECT status FROM seller.kyc_applications WHERE application_id=$1", app.ID)
	f.scan(t, &accesses, "SELECT count(*) FROM seller.kyc_document_accesses")
	if status != "SUBMITTED" || accesses != 0 {
		t.Fatalf("status=%s accesses=%d", status, accesses)
	}
}

type adminSellerClient interface {
	ListKYCApplications(context.Context, *connect.Request[adminsellerpb.ListKYCApplicationsRequest]) (*connect.Response[adminsellerpb.ListKYCApplicationsResponse], error)
	GetKYCApplication(context.Context, *connect.Request[adminsellerpb.GetKYCApplicationRequest]) (*connect.Response[adminsellerpb.GetKYCApplicationResponse], error)
	ListKYCDocumentAccesses(context.Context, *connect.Request[adminsellerpb.ListKYCDocumentAccessesRequest]) (*connect.Response[adminsellerpb.ListKYCDocumentAccessesResponse], error)
	ApproveKYC(context.Context, *connect.Request[adminsellerpb.ApproveKYCRequest]) (*connect.Response[adminsellerpb.ApproveKYCResponse], error)
	RejectKYC(context.Context, *connect.Request[adminsellerpb.RejectKYCRequest]) (*connect.Response[adminsellerpb.RejectKYCResponse], error)
	RequeryKYCRegistration(context.Context, *connect.Request[adminsellerpb.RequeryKYCRegistrationRequest]) (*connect.Response[adminsellerpb.RequeryKYCRegistrationResponse], error)
}

func protoText(t *testing.T, m proto.Message) string {
	t.Helper()
	raw, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// 補充文件端點掛在入口的限流底下：同一個身分超過額度回 429 加 Retry-After（沒登入的訪客也算，端點在授權之前就先計數）。
func TestDocumentEndpointIsRateLimited(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	path := "/api/files/admin/seller/kyc_applications/0197d1a2-0000-7000-8000-000000000000/documents/0"
	limited := 0
	for range 320 {
		code, header, _ := f.documentGET(t, "", path)
		switch code {
		case http.StatusUnauthorized:
		case http.StatusTooManyRequests:
			limited++
			if header.Get("Retry-After") == "" {
				t.Fatal("429 要帶 Retry-After")
			}
		default:
			t.Fatalf("code=%d", code)
		}
	}
	if limited == 0 {
		t.Fatal("超過額度的請求沒有被限流")
	}
}

// 登記資料查詢走完整的 Connect 路徑：公司型送件排入查詢，job 寫入結果後後台看得到登記資料與比對；etag 是版本加查詢序號，
// 重新查詢產生新結果之後舊 etag 的核准是 aborted、格式錯誤是 invalid_argument，用新 etag 核准成功並在審核紀錄留下依據。
func TestAdminKYCRegistrationCheckOverConnect(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	accountID, _ := f.newSeller(t, "丙店")
	draft, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{
		AccountID: accountID, SellerType: ptr(seller.KYCCompany), CompanyName: ptr("範例小農有限公司"), TaxID: ptr("33330333"),
		CompanyRegisterDate: ptr("2015-03-04"), CompanyTownCode: ptr("63000050"), CompanyAddressDetail: ptr("中山路 2 號"),
		ProductType: ptr("蔬果"),
	})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := f.module.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version})
	if err != nil {
		t.Fatal(err)
	}
	checkIDs := func() []uuid.UUID {
		rows, err := f.db.Conn(ctx).Query(ctx, "SELECT check_id::text FROM seller.kyc_registration_checks WHERE application_id=$1 ORDER BY seq", sub.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []uuid.UUID
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			id, err := uuid.Parse(s)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, id)
		}
		return out
	}
	app := f.getKYC(t, sub.ID.String())
	if app.GetEtag() != "2.0" || app.GetRegistrationCheck() != nil || app.GetPendingRegistrationCheck().GetSeq() != 1 ||
		app.GetPendingRegistrationCheck().GetDeadlineTime() == nil || !app.GetPendingRegistrationCheck().GetDeadlineTime().AsTime().After(app.GetPendingRegistrationCheck().GetRequestTime().AsTime()) {
		t.Fatalf("查詢中：etag=%q check=%+v pending=%+v", app.GetEtag(), app.GetRegistrationCheck(), app.GetPendingRegistrationCheck())
	}
	f.registry.reply(gcis.Registration{
		TaxID: "33330333", Name: "範例小農有限公司", Address: "臺北市中正區中山路２號", SetupDate: time.Date(2015, 3, 4, 0, 0, 0, 0, time.UTC),
		Source: gcis.SourceCompany, OrganizationType: "有限公司", StatusText: "核准設立",
	}, true, nil)
	if err := f.module.RunKYCRegistryCheck(ctx, checkIDs()[0], false); err != nil {
		t.Fatal(err)
	}
	app = f.getKYC(t, sub.ID.String())
	c := app.GetRegistrationCheck()
	if app.GetEtag() != "2.1" || c.GetSeq() != 1 || c.GetOutcome() != adminsellerpb.KYCRegistrationOutcome_KYC_REGISTRATION_OUTCOME_FOUND || app.GetPendingRegistrationCheck() != nil ||
		c.GetRegistration().GetName() != "範例小農有限公司" || c.GetRegistration().GetPostalAddress().GetTownCode() != "63000050" ||
		len(c.GetMismatchedFields()) != 0 || len(c.GetUnverifiableFields()) != 0 {
		t.Fatalf("etag=%q check=%+v", app.GetEtag(), c)
	}
	// 重新查詢：已完成的一筆之後新增第二筆；第二筆還沒查完時再查一次，會取代它（CANCELLED）並新增第三筆。
	for range 2 {
		if _, err := f.admin.RequeryKYCRegistration(ctx, connect.NewRequest(&adminsellerpb.RequeryKYCRegistrationRequest{Id: sub.ID.String()})); err != nil {
			t.Fatal(err)
		}
	}
	ids := checkIDs()
	if len(ids) != 3 {
		t.Fatalf("checks=%v", ids)
	}
	f.registry.reply(gcis.Registration{}, false, nil)
	if err := f.module.RunKYCRegistryCheck(ctx, ids[2], false); err != nil {
		t.Fatal(err)
	}
	_, err = f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: sub.ID.String(), Etag: app.GetEtag()}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	for _, bad := range []string{"2", "2.x", "x.1"} {
		_, err = f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: sub.ID.String(), Etag: bad}))
		if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
			t.Fatalf("%q：field=%q", bad, field)
		}
	}
	fresh := f.getKYC(t, sub.ID.String())
	if fresh.GetEtag() != "2.3" || fresh.GetRegistrationCheck().GetOutcome() != adminsellerpb.KYCRegistrationOutcome_KYC_REGISTRATION_OUTCOME_NOT_FOUND {
		t.Fatalf("fresh etag=%q check=%+v", fresh.GetEtag(), fresh.GetRegistrationCheck())
	}
	approved, err := f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: sub.ID.String(), Etag: fresh.GetEtag()}))
	if err != nil {
		t.Fatal(err)
	}
	var basis int64
	for _, r := range approved.Msg.GetApplication().GetReviews() {
		if r.GetAction() == adminsellerpb.KYCReviewAction_KYC_REVIEW_ACTION_APPROVE {
			basis = r.GetRegistrationCheckSeq()
			if r.GetRegistrationCheckQueryTime() == nil {
				t.Fatalf("review=%+v", r)
			}
		}
	}
	if basis != 3 || approved.Msg.GetApplication().GetEtag() != "3.3" {
		t.Fatalf("basis=%d etag=%q", basis, approved.Msg.GetApplication().GetEtag())
	}
	_, err = f.admin.RequeryKYCRegistration(ctx, connect.NewRequest(&adminsellerpb.RequeryKYCRegistrationRequest{Id: sub.ID.String()}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
}

// anIssue 是測試用的一個有效問題。
func anIssue() []*pb.KYCIssue {
	return []*pb.KYCIssue{{Field: pb.KYCField_KYC_FIELD_PRODUCT_TYPE, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "請確認商品類型"}}
}
