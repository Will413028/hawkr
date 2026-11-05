//go:build integration

package seller_test

import (
	"context"
	"io"
	"net/http"
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

const (
	kycTestID   = "A123456789"
	kycTestBank = "012345678901"
)

// submittedKYC 為一家新店建立並送出一份個人賣家的申請；證件放進 media 的替身（內容是 "<位置>-bytes"）。
func (f *adminFixture) submittedKYC(t *testing.T, name, idCard, bank string) (int64, seller.Store, seller.KYCApplication) {
	t.Helper()
	accountID, s := f.newSeller(t, name)
	ctx := context.Background()
	draft, err := f.module.SaveKYCDraft(ctx, seller.KYCDraftInput{
		AccountID: accountID, SellerType: ptr(seller.KYCIndividual), RealName: ptr(name + "的負責人"), BirthDate: ptr("1990-05-06"), IDCardNo: ptr(idCard),
		EnglishName: ptr("Wang Farm"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr(bank),
		ProductType: ptr("蔬果"),
		IDCardFront: &seller.KYCDocumentRef{StagedToken: "front-" + name}, IDCardBack: &seller.KYCDocumentRef{StagedToken: "back-" + name}, BankBook: &seller.KYCDocumentRef{StagedToken: "book-" + name},
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
			media.SecureAsset{Data: []byte(strings.ToLower(string(d.Slot)) + "-bytes"), ContentType: "image/jpeg"})
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

// documentGET 以 token 讀證件圖片端點；token 空是沒有登入。
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

// 審核佇列、單筆檢視（只有遮蔽值、讀取不寫稽核）、核准與退回（etag、理由、欄位、審核者是呼叫的管理員）。
func TestAdminReviewsKYCOverConnect(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, storeA, appA := f.submittedKYC(t, "甲店", kycTestID, kycTestBank)
	_, _, appB := f.submittedKYC(t, "乙店", "F131104093", "999888777666")

	// 佇列：審核中的舊到新（甲先送出）；單筆檢視只有遮蔽值，讀取不寫任何稽核。
	queue, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{Status: pb.KYCStatus_KYC_STATUS_SUBMITTED, PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if len(queue.Msg.GetApplications()) != 1 || queue.Msg.GetApplications()[0].GetId() != appA.ID.String() || queue.Msg.GetNextPageToken() == "" ||
		queue.Msg.GetApplications()[0].GetStoreId() != sid(storeA.StoreID) || queue.Msg.GetApplications()[0].GetSubmitTime() == nil || queue.Msg.GetApplications()[0].GetRealName() != "甲店的負責人" {
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
	raw, _ := protojson.Marshal(got.Msg)
	text := string(raw)
	for _, forbidden := range []string{kycTestID, kycTestBank, "asset-front", "asset-book", "7111/", "download", "viewUri"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("masked response must not contain %q: %s", forbidden, text)
		}
	}
	app := got.Msg.GetApplication()
	if app.GetIndividual().GetIdCardNoMasked() != "A1******89" || app.GetBankAccountMasked() != "********8901" || len(app.GetDocuments()) != 3 ||
		app.GetFlow().GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_SUBMITTED || len(app.GetReviews()) != 1 || got.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("application=%+v", app)
	}
	var audits int64
	f.scan(t, &audits, "SELECT (SELECT count(*) FROM seller.kyc_reveals) + (SELECT count(*) FROM seller.kyc_document_accesses)")
	if audits != 0 {
		t.Fatalf("讀取不能寫稽核：%d", audits)
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
	if a.GetStatus() != pb.KYCStatus_KYC_STATUS_APPROVED || a.GetFlow().GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_BINDING || a.GetDecideTime() == nil || len(a.GetReviews()) != 2 ||
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
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_REAL_NAME, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].code" {
		t.Fatalf("unspecified code: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_REAL_NAME, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].message" {
		t.Fatalf("no message: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_REAL_NAME, DocumentAssetId: "asset-x", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].document_asset_id" {
		t.Fatalf("asset on a non document: field=%q", field)
	}
	_, err = reject(&pb.KYCIssue{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, DocumentAssetId: "asset-not-here", Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "x"})
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "issues[0].document_asset_id" {
		t.Fatalf("foreign asset: field=%q", field)
	}
	rejected, err := f.admin.RejectKYC(ctx, connect.NewRequest(&adminsellerpb.RejectKYCRequest{
		Id: appB.ID.String(), Etag: bApp.GetEtag(), Reason: "存摺不清楚", InternalNote: "疑似翻拍",
		Issues: []*pb.KYCIssue{
			{Field: pb.KYCField_KYC_FIELD_BANK_BOOK, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISSING, Message: "請補上存摺封面"},
			{Field: pb.KYCField_KYC_FIELD_BANK_ACCOUNT, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH, Message: "與存摺上的帳號不符"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	r := rejected.Msg.GetApplication()
	last := r.GetReviews()[len(r.GetReviews())-1]
	if r.GetStatus() != pb.KYCStatus_KYC_STATUS_REJECTED || last.GetAction() != adminsellerpb.KYCReviewAction_KYC_REVIEW_ACTION_REJECT || last.GetReason() != "存摺不清楚" ||
		len(last.GetIssues()) != 2 || last.GetIssues()[1].GetCode() != pb.KYCIssueCode_KYC_ISSUE_CODE_MISMATCH || last.GetIssues()[1].GetMessage() != "與存摺上的帳號不符" ||
		last.GetInternalNote() != "疑似翻拍" || len(r.GetOutstandingIssues()) != 2 || last.GetReviewerAdminId() != sid(f.adminID) {
		t.Fatalf("rejected=%+v", r)
	}
	// 佇列：兩筆都離開審核中，已核准與已退回各一。
	if left, err := f.admin.ListKYCApplications(ctx, connect.NewRequest(&adminsellerpb.ListKYCApplicationsRequest{Status: pb.KYCStatus_KYC_STATUS_SUBMITTED})); err != nil || len(left.Msg.GetApplications()) != 0 {
		t.Fatalf("left=%+v err=%v", left, err)
	}
}

// 顯示全文就是授權：到期之前、同一位管理員、這筆申請才讀得到證件圖片，每次讀取都寫稽核，回應不可快取。
func TestRevealGrantsTimeBoxedDocumentAccess(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, _, app := f.submittedKYC(t, "甲店", kycTestID, kycTestBank)
	_, _, other := f.submittedKYC(t, "乙店", "F131104093", "999888777666")
	doc := func(a seller.KYCApplication, slot string) string {
		return "/api/files/admin/seller/kyc_applications/" + a.ID.String() + "/documents/" + slot + "/0"
	}

	// 沒有授權：管理員 403；沒登入 401；買家 403；都沒有紀錄、媒體沒有被問。
	for name, c := range map[string]struct {
		token string
		want  int
	}{"admin without a grant": {f.adminToken, 403}, "visitor": {"", 401}, "customer": {f.customerToken, 403}} {
		code, header, body := f.documentGET(t, c.token, doc(app, "id_card_front"))
		if code != c.want || body == "id_card_front-bytes" || header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s：code=%d body=%q", name, code, body)
		}
	}
	var accesses int64
	f.scan(t, &accesses, "SELECT count(*) FROM seller.kyc_document_accesses")
	if accesses != 0 {
		t.Fatalf("accesses=%d", accesses)
	}

	// 顯示全文：理由必填、POST、不可快取；回全文與每張證件的檢視網址（沒有 asset ID）。
	_, err := f.admin.RevealKYCSensitive(ctx, connect.NewRequest(&adminsellerpb.RevealKYCSensitiveRequest{Id: app.ID.String(), Reason: "  "}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "reason" {
		t.Fatalf("field=%q", field)
	}
	before := f.now()
	revealed, err := f.admin.RevealKYCSensitive(ctx, connect.NewRequest(&adminsellerpb.RevealKYCSensitiveRequest{Id: app.ID.String(), Reason: "核對證件"}))
	if err != nil {
		t.Fatal(err)
	}
	r := revealed.Msg
	if r.GetIdCardNo() != kycTestID || r.GetBankAccount() != kycTestBank || r.GetRepIdCardNo() != "" || len(r.GetDocuments()) != 3 ||
		revealed.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("revealed=%+v header=%v", r, revealed.Header())
	}
	if d := r.GetExpireTime().AsTime().Sub(before); d < 15*time.Minute || d > 15*time.Minute+5*time.Second {
		t.Fatalf("授權有效 15 分鐘：%v", d)
	}
	raw, _ := protojson.Marshal(r)
	if strings.Contains(string(raw), "7111/") {
		t.Fatalf("response leaks object keys: %s", raw)
	}
	// 顯示全文的回應帶 asset_id（退回時指定某一張文件用），遮蔽的 GetKYCApplication 不帶（上面的檢查）。
	var front, frontAsset string
	for _, d := range r.GetDocuments() {
		if d.GetAssetId() == "" {
			t.Fatalf("document without asset_id: %+v", d)
		}
		if d.GetField() == pb.KYCField_KYC_FIELD_ID_CARD_FRONT {
			front, frontAsset = d.GetViewUri(), d.GetAssetId()
		}
	}
	if !strings.HasPrefix(frontAsset, "asset-front") {
		t.Fatalf("front asset_id=%q", frontAsset)
	}
	if front != doc(app, "id_card_front") {
		t.Fatalf("view uri=%q", front)
	}
	var revealRows int64
	f.scan(t, &revealRows, "SELECT count(*) FROM seller.kyc_reveals WHERE application_id=$1 AND admin_id=$2", app.ID, f.adminID)
	if revealRows != 1 {
		t.Fatalf("reveal rows=%d", revealRows)
	}

	// 授權的管理員讀得到；圖片的標頭固定。
	code, header, body := f.documentGET(t, f.adminToken, front)
	if code != 200 || body != "id_card_front-bytes" {
		t.Fatalf("code=%d body=%q", code, body)
	}
	for k, v := range map[string]string{"Content-Type": "image/jpeg", "Cache-Control": "private, no-store", "Content-Disposition": "inline", "X-Content-Type-Options": "nosniff"} {
		if header.Get(k) != v {
			t.Fatalf("%s = %q，要 %q", k, header.Get(k), v)
		}
	}
	// 別的管理員、別筆申請、不存在的位置：403 或 404，內容不外流。
	for name, c := range map[string]struct {
		token, path string
		want        int
	}{
		"another admin": {f.otherAdminToken, front, 403}, "another application": {f.adminToken, doc(other, "id_card_front"), 403},
		"no such position": {f.adminToken, "/api/files/admin/seller/kyc_applications/" + app.ID.String() + "/documents/company_document/0", 404},
		"bad slot":         {f.adminToken, "/api/files/admin/seller/kyc_applications/" + app.ID.String() + "/documents/selfie/0", 404},
	} {
		code, _, body := f.documentGET(t, c.token, c.path)
		if code != c.want || strings.Contains(body, "-bytes") {
			t.Fatalf("%s：code=%d body=%q", name, code, body)
		}
	}
	f.scan(t, &accesses, "SELECT count(*) FROM seller.kyc_document_accesses")
	if accesses != 1 {
		t.Fatalf("只有成功的那一次有讀取紀錄：%d", accesses)
	}
	// 角色被撤銷之後，即使手上的限時授權還沒到期，讀圖片也在入口被擋下（權限檢查在模組的 reveal 授權之前），沒有讀取紀錄。
	ownerChange := identity.AdminRoleChangeInput{AdminID: f.adminID, Role: adminperm.RoleOwner, Reason: "測試", Source: identity.AdminRoleSourceCLI}
	if _, err := f.id.RevokeAdminRole(ctx, ownerChange); err != nil {
		t.Fatal(err)
	}
	code, _, body = f.documentGET(t, f.adminToken, front)
	if code != 403 || strings.Contains(body, "-bytes") {
		t.Fatalf("沒有角色的管理員：code=%d body=%q", code, body)
	}
	f.scan(t, &accesses, "SELECT count(*) FROM seller.kyc_document_accesses")
	if accesses != 1 {
		t.Fatalf("被擋下的請求不寫讀取紀錄：%d", accesses)
	}
	if _, err := f.id.GrantAdminRole(ctx, ownerChange); err != nil {
		t.Fatal(err)
	}
	// 讀取紀錄寫不進去：不送內容。
	if _, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses ADD CONSTRAINT test_no_writes CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	code, _, body = f.documentGET(t, f.adminToken, doc(app, "bank_book"))
	if code != 500 || strings.Contains(body, "-bytes") {
		t.Fatalf("fail closed：code=%d body=%q", code, body)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.kyc_document_accesses DROP CONSTRAINT test_no_writes"); err != nil {
		t.Fatal(err)
	}
	// 稽核紀錄不在申請檢視裡，各自分頁；列出誰、依據哪次授權、讀了哪一張。
	listedReveals, err := f.admin.ListKYCReveals(ctx, connect.NewRequest(&adminsellerpb.ListKYCRevealsRequest{Id: app.ID.String(), PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	listedAccesses, err := f.admin.ListKYCDocumentAccesses(ctx, connect.NewRequest(&adminsellerpb.ListKYCDocumentAccessesRequest{Id: app.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	rv, ac := listedReveals.Msg.GetReveals(), listedAccesses.Msg.GetDocumentAccesses()
	if len(rv) != 1 || rv[0].GetAdminId() != sid(f.adminID) || rv[0].GetExpireTime() == nil || len(ac) != 1 || ac[0].GetAdminId() != sid(f.adminID) ||
		ac[0].GetField() != pb.KYCField_KYC_FIELD_ID_CARD_FRONT || ac[0].GetRevealId() != rv[0].GetId() ||
		listedReveals.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("reveals=%+v accesses=%+v", rv, ac)
	}
	raw, _ = protojson.Marshal(listedAccesses.Msg)
	if strings.Contains(string(raw), "asset-") {
		t.Fatalf("讀取紀錄的回應不帶 asset ID：%s", raw)
	}
	var recordedAsset string
	f.scan(t, &recordedAsset, "SELECT asset_id FROM seller.kyc_document_accesses")
	if recordedAsset != "asset-front-甲店" {
		t.Fatalf("讀取紀錄記實際送出的 asset_id：%q", recordedAsset)
	}
	for _, id := range []string{"0197d1a2-0000-7000-8000-000000000000"} {
		_, err := f.admin.ListKYCReveals(ctx, connect.NewRequest(&adminsellerpb.ListKYCRevealsRequest{Id: id}))
		wantCode(t, err, connect.CodeNotFound, "")
		_, err = f.admin.ListKYCDocumentAccesses(ctx, connect.NewRequest(&adminsellerpb.ListKYCDocumentAccessesRequest{Id: id}))
		wantCode(t, err, connect.CodeNotFound, "")
	}
	// 申請檢視不內嵌稽核紀錄。
	if view := f.getKYC(t, app.ID.String()); strings.Contains(protoText(t, view), "reveal") {
		t.Fatalf("view=%s", protoText(t, view))
	}
	// 時鐘走過 15 分鐘（不 sleep）：授權過期。
	f.skew.Store(int64(15*time.Minute + time.Second))
	code, _, body = f.documentGET(t, f.adminToken, front)
	if code != 403 || strings.Contains(body, "-bytes") {
		t.Fatalf("expired：code=%d body=%q", code, body)
	}
	f.scan(t, &accesses, "SELECT count(*) FROM seller.kyc_document_accesses")
	if accesses != 1 {
		t.Fatalf("過期的讀取不能留下紀錄：%d", accesses)
	}
}

// 新的後台方法都只有管理員能呼叫：沒登入是 unauthenticated，買家是 permission_denied，什麼都不會被寫入。
func TestAdminKYCAndPayUniMethodsRejectCustomersAndVisitorsAndWriteNothing(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, s, app := f.submittedKYC(t, "甲店", kycTestID, kycTestBank)
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
			"ListKYCReveals": func() error {
				_, err := c.ListKYCReveals(ctx, connect.NewRequest(&adminsellerpb.ListKYCRevealsRequest{Id: app.ID.String()}))
				return err
			},
			"ListKYCDocumentAccesses": func() error {
				_, err := c.ListKYCDocumentAccesses(ctx, connect.NewRequest(&adminsellerpb.ListKYCDocumentAccessesRequest{Id: app.ID.String()}))
				return err
			},
			"RevealKYCSensitive": func() error {
				_, err := c.RevealKYCSensitive(ctx, connect.NewRequest(&adminsellerpb.RevealKYCSensitiveRequest{Id: app.ID.String(), Reason: "越權"}))
				return err
			},
			"GetPayUniRegistration": func() error {
				_, err := c.GetPayUniRegistration(ctx, connect.NewRequest(&adminsellerpb.GetPayUniRegistrationRequest{StoreId: sid(s.StoreID)}))
				return err
			},
			"RebindPayUni": func() error {
				_, err := c.RebindPayUni(ctx, connect.NewRequest(&adminsellerpb.RebindPayUniRequest{StoreId: sid(s.StoreID)}))
				return err
			},
			"ListPayUniObservations": func() error {
				_, err := c.ListPayUniObservations(ctx, connect.NewRequest(&adminsellerpb.ListPayUniObservationsRequest{StoreId: sid(s.StoreID)}))
				return err
			},
			"RefreshPayUniStatus": func() error {
				_, err := c.RefreshPayUniStatus(ctx, connect.NewRequest(&adminsellerpb.RefreshPayUniStatusRequest{StoreId: sid(s.StoreID)}))
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
	var reveals int64
	f.scan(t, &status, "SELECT status FROM seller.kyc_applications WHERE application_id=$1", app.ID)
	f.scan(t, &reveals, "SELECT count(*) FROM seller.kyc_reveals")
	if status != "SUBMITTED" || reveals != 0 {
		t.Fatalf("status=%s reveals=%d", status, reveals)
	}
}

type adminSellerClient interface {
	ListKYCApplications(context.Context, *connect.Request[adminsellerpb.ListKYCApplicationsRequest]) (*connect.Response[adminsellerpb.ListKYCApplicationsResponse], error)
	GetKYCApplication(context.Context, *connect.Request[adminsellerpb.GetKYCApplicationRequest]) (*connect.Response[adminsellerpb.GetKYCApplicationResponse], error)
	ListKYCReveals(context.Context, *connect.Request[adminsellerpb.ListKYCRevealsRequest]) (*connect.Response[adminsellerpb.ListKYCRevealsResponse], error)
	ListKYCDocumentAccesses(context.Context, *connect.Request[adminsellerpb.ListKYCDocumentAccessesRequest]) (*connect.Response[adminsellerpb.ListKYCDocumentAccessesResponse], error)
	ApproveKYC(context.Context, *connect.Request[adminsellerpb.ApproveKYCRequest]) (*connect.Response[adminsellerpb.ApproveKYCResponse], error)
	RejectKYC(context.Context, *connect.Request[adminsellerpb.RejectKYCRequest]) (*connect.Response[adminsellerpb.RejectKYCResponse], error)
	RevealKYCSensitive(context.Context, *connect.Request[adminsellerpb.RevealKYCSensitiveRequest]) (*connect.Response[adminsellerpb.RevealKYCSensitiveResponse], error)
	GetPayUniRegistration(context.Context, *connect.Request[adminsellerpb.GetPayUniRegistrationRequest]) (*connect.Response[adminsellerpb.GetPayUniRegistrationResponse], error)
	RebindPayUni(context.Context, *connect.Request[adminsellerpb.RebindPayUniRequest]) (*connect.Response[adminsellerpb.RebindPayUniResponse], error)
	ListPayUniObservations(context.Context, *connect.Request[adminsellerpb.ListPayUniObservationsRequest]) (*connect.Response[adminsellerpb.ListPayUniObservationsResponse], error)
	RefreshPayUniStatus(context.Context, *connect.Request[adminsellerpb.RefreshPayUniStatusRequest]) (*connect.Response[adminsellerpb.RefreshPayUniStatusResponse], error)
	RequeryKYCRegistration(context.Context, *connect.Request[adminsellerpb.RequeryKYCRegistrationRequest]) (*connect.Response[adminsellerpb.RequeryKYCRegistrationResponse], error)
}

// PayUni：沒有核准過的店家沒有註冊；核准之後綁定失敗（這個環境沒有設定 PayUni）才能重新綁定，重送是冪等的，回傳的註冊與
// 讀取一致（帶賣家型態）；觀測紀錄分頁、先檢查店家存在。
func TestAdminPayUniMethodsOverConnect(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, s, app := f.submittedKYC(t, "甲店", kycTestID, kycTestBank)
	_, err := f.admin.GetPayUniRegistration(ctx, connect.NewRequest(&adminsellerpb.GetPayUniRegistrationRequest{StoreId: sid(s.StoreID)}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.admin.RebindPayUni(ctx, connect.NewRequest(&adminsellerpb.RebindPayUniRequest{StoreId: sid(s.StoreID)}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.admin.GetPayUniRegistration(ctx, connect.NewRequest(&adminsellerpb.GetPayUniRegistrationRequest{StoreId: "987654"}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.admin.ListPayUniObservations(ctx, connect.NewRequest(&adminsellerpb.ListPayUniObservationsRequest{StoreId: "987654"}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.admin.GetPayUniRegistration(ctx, connect.NewRequest(&adminsellerpb.GetPayUniRegistrationRequest{StoreId: "x"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")

	if _, err := f.admin.ApproveKYC(ctx, connect.NewRequest(&adminsellerpb.ApproveKYCRequest{Id: app.ID.String(), Etag: rpcEtag(app.Version)})); err != nil {
		t.Fatal(err)
	}
	pending, err := f.admin.GetPayUniRegistration(ctx, connect.NewRequest(&adminsellerpb.GetPayUniRegistrationRequest{StoreId: sid(s.StoreID)}))
	if err != nil {
		t.Fatal(err)
	}
	if reg := pending.Msg.GetRegistration(); reg.GetBindStatus() != pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_PENDING || reg.GetMerId() == "" || reg.GetApplicationId() != app.ID.String() || reg.GetStoreId() != sid(s.StoreID) {
		t.Fatalf("registration=%+v", reg)
	}
	// PENDING 時重新綁定是冪等的（回傳目前的註冊）。
	same, err := f.admin.RebindPayUni(ctx, connect.NewRequest(&adminsellerpb.RebindPayUniRequest{StoreId: sid(s.StoreID)}))
	if err != nil || same.Msg.GetRegistration().GetBindStatus() != pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_PENDING {
		t.Fatalf("same=%+v err=%v", same, err)
	}
	// 沒有設定 PayUni：背景工作把綁定記成失敗，這時才能重新綁定；重新綁定留下審核紀錄（誰）。
	if _, err := f.module.SyncPayUni(ctx, s.StoreID, false); err != nil {
		t.Fatal(err)
	}
	failed, err := f.admin.GetPayUniRegistration(ctx, connect.NewRequest(&adminsellerpb.GetPayUniRegistrationRequest{StoreId: sid(s.StoreID)}))
	if err != nil || failed.Msg.GetRegistration().GetBindStatus() != pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_FAILED || failed.Msg.GetRegistration().GetBindErrorCode() != seller.PayUniErrNotConfigured ||
		failed.Msg.GetRegistration().GetBindAttempts() != 1 || failed.Msg.GetRegistration().GetProblem().GetCode() != seller.PayUniErrNotConfigured {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	rebound, err := f.admin.RebindPayUni(ctx, connect.NewRequest(&adminsellerpb.RebindPayUniRequest{StoreId: sid(s.StoreID)}))
	if err != nil {
		t.Fatal(err)
	}
	if reg := rebound.Msg.GetRegistration(); reg.GetBindStatus() != pb.PayUniBindStatus_PAY_UNI_BIND_STATUS_PENDING || reg.GetBindErrorCode() != "" {
		t.Fatalf("rebound=%+v", reg)
	}
	view := f.getKYC(t, app.ID.String())
	last := view.GetReviews()[len(view.GetReviews())-1]
	if last.GetAction() != adminsellerpb.KYCReviewAction_KYC_REVIEW_ACTION_REBIND || last.GetReviewerAdminId() != sid(f.adminID) {
		t.Fatalf("reviews=%+v", view.GetReviews())
	}
	// 沒有設定 PayUni 的刷新是 failed_precondition。
	_, err = f.admin.RefreshPayUniStatus(ctx, connect.NewRequest(&adminsellerpb.RefreshPayUniStatusRequest{StoreId: sid(s.StoreID)}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")

	// 觀測紀錄：新到舊，一頁一筆走完不漏不重，換店的 token 不能用。
	mer := seller.FormatPayUniMerID(s.StoreCode)
	base := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	for i := range 3 {
		if err := f.module.ApplyPayUniNotification(ctx, seller.PayUniObserveArgs{EventKey: mer + ":" + string(rune('a'+i)), MerID: mer, MerStatus: "9", ExecutedAt: base.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	var got []*adminsellerpb.PayUniObservation
	token := ""
	for range 10 {
		resp, err := f.admin.ListPayUniObservations(ctx, connect.NewRequest(&adminsellerpb.ListPayUniObservationsRequest{StoreId: sid(s.StoreID), PageSize: 1, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, resp.Msg.GetObservations()...)
		if token = resp.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	if len(got) < 3 {
		t.Fatalf("observations=%d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].GetExecuteTime().AsTime().Before(got[i].GetExecuteTime().AsTime()) {
			t.Fatalf("不是新到舊：%v", got)
		}
	}
	if got[0].GetSource() != adminsellerpb.PayUniObservationSource_PAY_UNI_OBSERVATION_SOURCE_WEBHOOK || got[0].GetStatusCodes().GetMerStatus() != "9" {
		t.Fatalf("first=%+v", got[0])
	}
	_, s2 := f.newSeller(t, "別家")
	first, err := f.admin.ListPayUniObservations(ctx, connect.NewRequest(&adminsellerpb.ListPayUniObservationsRequest{StoreId: sid(s.StoreID), PageSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.admin.ListPayUniObservations(ctx, connect.NewRequest(&adminsellerpb.ListPayUniObservationsRequest{StoreId: sid(s2.StoreID), PageToken: first.Msg.GetNextPageToken()}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

func rpcEtag(version int64) string { return sid(version) + ".0" }

func protoText(t *testing.T, m proto.Message) string {
	t.Helper()
	raw, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// 證件圖片端點掛在入口的限流底下：同一個身分超過額度回 429 加 Retry-After（沒登入的訪客也算，端點在授權之前就先計數）。
func TestDocumentEndpointIsRateLimited(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	path := "/api/files/admin/seller/kyc_applications/0197d1a2-0000-7000-8000-000000000000/documents/id_card_front/0"
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
		AccountID: accountID, SellerType: ptr(seller.KYCCompany), RealName: ptr("丙店的負責人"), BirthDate: ptr("1990-05-06"),
		RepIDCardNo: ptr("X223456789"), CompanyName: ptr("範例小農有限公司"), TaxID: ptr("33330333"),
		CompanyRegisterDate: ptr("2015-03-04"), CompanyTownCode: ptr("63000050"), CompanyAddressDetail: ptr("中山路 2 號"),
		EnglishName: ptr("Wang Farm"), BankName: ptr("臺灣銀行"), BankCode: ptr("004"), BankAccount: ptr("012345678901"),
		ProductType: ptr("蔬果"),
		IDCardFront: &seller.KYCDocumentRef{StagedToken: "front-c"}, IDCardBack: &seller.KYCDocumentRef{StagedToken: "back-c"}, BankBook: &seller.KYCDocumentRef{StagedToken: "book-c"},
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
	return []*pb.KYCIssue{{Field: pb.KYCField_KYC_FIELD_REAL_NAME, Code: pb.KYCIssueCode_KYC_ISSUE_CODE_OTHER, Message: "請確認姓名"}}
}
