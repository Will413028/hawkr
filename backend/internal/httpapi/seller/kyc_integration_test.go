//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	adminsellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpsellerpkg "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 測試用的身分證號（格式與檢查碼都正確，不是真實的人）。
const testID = "A123456789"

type fakeRegistry struct {
	reg   gcis.Registration
	found bool
	err   error
	calls int
}

func (r *fakeRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	r.calls++
	return r.reg, r.found, r.err
}

var draftPaths = []string{
	"individual.id_card_no", "real_name", "birth_date", "english_name", "bank_name", "bank_code", "bank_account",
	"product_type", "id_card_front", "id_card_back", "bank_book",
}

// completeDraft 是一份完整、合法的個人賣家草稿請求。
func completeDraft(idNo string) *pb.SaveKYCDraftRequest {
	return &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{
			Subject:  &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{IdCardNo: idNo}},
			RealName: "王小明", BirthDate: &date.Date{Year: 1990, Month: 5, Day: 6}, EnglishName: "Wang Farm", BankName: "臺灣銀行", BankCode: "004", BankAccount: "012345678901", ProductType: "蔬果",
			IdCardFront: &pb.KYCDocument{StagedToken: "front"}, IdCardBack: &pb.KYCDocument{StagedToken: "back"}, BankBook: &pb.KYCDocument{StagedToken: "book"},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: draftPaths},
	}
}

func saveDraft(t *testing.T, c sellerconnect.SellerAppServiceClient, req *pb.SaveKYCDraftRequest) *pb.KYCApplication {
	t.Helper()
	resp, err := c.SaveKYCDraft(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetApplication()
}

// detailTypes 回傳錯誤帶的 details 型別名稱。
func detailTypes(t *testing.T, err error) []string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	var out []string
	for _, d := range ce.Details() {
		out = append(out, d.Type())
	}
	return out
}

func hasRetryInfo(t *testing.T, err error) bool {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return false
	}
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			t.Fatal(e)
		}
		if info, ok := v.(*errdetails.RetryInfo); ok && info.GetRetryDelay().AsDuration() > 0 {
			return true
		}
	}
	return false
}

// 草稿、送出與取消：沒有申請時 GetMyKYC 是 not_found；建立（不帶 etag）與修改（帶 etag，mask 選到的欄位）；身分證號與
// 銀行帳號只有遮蔽後的值；證件只有位置；過期的 etag 是 VERSION_CONFLICT；取消之後又回到沒有申請。
func TestKYCDraftSubmitAndCancel(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	_, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")

	created := saveDraft(t, f.app, completeDraft(testID))
	if created.GetId() == "" || created.GetStatus() != pb.KYCStatus_KYC_STATUS_DRAFT || created.GetEtag() == "" || created.GetIndividual().GetIdCardNoMasked() != "A1******89" ||
		created.GetBankAccountMasked() != "********8901" || created.GetIndividual().GetIdCardNo() != "" || created.GetBankAccount() != "" || created.GetBirthDate().GetYear() != 1990 ||
		created.GetIdCardFront().GetAssetId() != "asset-front" || created.GetBankBook().GetDownloadUri() != "/api/v1/app/media/secure_download/asset-book" || created.GetFlow().GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_DRAFT {
		t.Fatalf("created=%v", created)
	}
	got, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
	if err != nil || got.Msg.GetApplication().GetId() != created.GetId() || got.Msg.GetApplication().GetEtag() != created.GetEtag() {
		t.Fatalf("got=%v err=%v", got, err)
	}

	// 沒有帶 etag 又已經有申請：already_exists（VERSION_CONFLICT），不是再建一份；帶過期的 etag 才是 aborted。
	_, err = f.app.SaveKYCDraft(ctx, connect.NewRequest(completeDraft(testID)))
	wantCode(t, err, connect.CodeAlreadyExists, "VERSION_CONFLICT")

	// 只改 mask 選到的欄位；沒選到的（銀行、證號）不動，清掉的欄位（english_name）回到空。
	updated := saveDraft(t, f.app, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: created.GetEtag(), RealName: "王大明", ProductType: "不在 mask"},
		UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"real_name", "english_name"}},
	})
	if updated.GetRealName() != "王大明" || updated.GetEnglishName() != "" || updated.GetProductType() != "蔬果" || updated.GetIndividual().GetIdCardNoMasked() != "A1******89" ||
		updated.GetEtag() == created.GetEtag() {
		t.Fatalf("updated=%v", updated)
	}
	_, err = f.app.SaveKYCDraft(ctx, connect.NewRequest(&pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: created.GetEtag(), RealName: "過期的"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"real_name"}},
	}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")

	// 日期不合法：整筆不寫入，錯誤指向請求的欄位。
	_, err = f.app.SaveKYCDraft(ctx, connect.NewRequest(&pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: updated.GetEtag(), BirthDate: &date.Date{Year: 2999, Month: 1, Day: 1}, RealName: "不該寫入"},
		UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"birth_date", "real_name"}},
	}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "application.birth_date" {
		t.Fatalf("field=%q", field)
	}
	if got, _ := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{})); got.Msg.GetApplication().GetRealName() != "王大明" {
		t.Fatalf("不合法的請求留下了部分寫入：%v", got)
	}

	// 補回清掉的英文名稱（送出要求必填欄位齊全）。
	restored := saveDraft(t, f.app, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: updated.GetEtag(), EnglishName: "Wang Farm"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"english_name"}},
	})
	submitted, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: restored.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	if err != nil || submitted.Msg.GetApplication().GetStatus() != pb.KYCStatus_KYC_STATUS_SUBMITTED || submitted.Msg.GetApplication().GetSubmitTime() == nil ||
		submitted.Msg.GetApplication().GetFlow().GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_SUBMITTED {
		t.Fatalf("submitted=%v err=%v", submitted, err)
	}
	_, err = f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
		t.Fatalf("field=%q", field)
	}

	cancelled, err := f.app.CancelKYC(ctx, connect.NewRequest(&pb.CancelKYCRequest{Etag: submitted.Msg.GetApplication().GetEtag(), Reason: "改天再填"}))
	if err != nil || cancelled.Msg.GetApplication().GetStatus() != pb.KYCStatus_KYC_STATUS_CANCELLED {
		t.Fatalf("cancelled=%v err=%v", cancelled, err)
	}
	// 取消的回應與重讀一致：重讀是「沒有申請」（not_found，client 當成 NOT_STARTED、可以開始＝能重送、沒有原因、沒有 PayUni 狀態）。
	if flow := cancelled.Msg.GetApplication().GetFlow(); flow.GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_NOT_STARTED || !flow.GetCanResubmit() ||
		flow.GetIdentityLocked() || len(flow.GetResubmitUnavailabilities()) != 0 || flow.GetPayuni() != nil {
		t.Fatalf("取消的回應與「沒有申請」的重讀不一致：%v", flow)
	}
	_, err = f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")
}

// 沒有店家的帳號、停權的店家：KYC 的方法依宣告的能力被擋下。
func TestKYCAndPayUniFollowTheDeclaredCapabilities(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, err := f.other.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.other.SaveKYCDraft(ctx, connect.NewRequest(completeDraft(testID)))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = f.other.LookupBusinessRegistration(ctx, connect.NewRequest(&pb.LookupBusinessRegistrationRequest{TaxId: "33330333"}))
	wantCode(t, err, connect.CodeNotFound, "")

	f.applyComplete(t, f.app)
	draft := saveDraft(t, f.app, completeDraft(testID))
	f.suspendStore(t, f.storeID(t))
	// 停權：讀自己的申請仍可以，寫入與統編查詢不行。
	if _, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{})); err != nil {
		t.Fatal(err)
	}
	_, err = f.app.SaveKYCDraft(ctx, connect.NewRequest(&pb.SaveKYCDraftRequest{Application: &pb.KYCApplication{Etag: draft.GetEtag(), RealName: "x"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"real_name"}}}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: draft.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.CancelKYC(ctx, connect.NewRequest(&pb.CancelKYCRequest{Etag: draft.GetEtag()}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	_, err = f.app.LookupBusinessRegistration(ctx, connect.NewRequest(&pb.LookupBusinessRegistrationRequest{TaxId: "33330333"}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
}

// 身分比對沒有通過（別的帳號有生效中處分的身分）時回固定的文案：沒有欄位、原因或帳號的細節，details 只有給使用者看的文字。
func TestSubmitKYCIdentityConflictIsGeneric(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.other)
	theirs := saveDraft(t, f.other, completeDraft(testID))
	if _, err := f.other.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: theirs.GetEtag(), AcceptedAgreements: kycAgreementsPB()})); err != nil {
		t.Fatal(err)
	}
	theirStore, err := f.module.CurrentStore(ctx, f.otherID)
	if err != nil {
		t.Fatal(err)
	}
	f.suspendStore(t, theirStore.StoreID)

	f.applyComplete(t, f.app)
	mine := saveDraft(t, f.app, completeDraft(testID))
	_, err = f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: mine.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	if got := detailTypes(t, err); len(got) != 1 || got[0] != "google.rpc.LocalizedMessage" {
		t.Fatalf("details=%v", got)
	}
	if ce.Message() != "failed_precondition: 無法通過認證，請聯絡客服" && ce.Message() != "無法通過認證，請聯絡客服" {
		t.Fatalf("message=%q", ce.Message())
	}
	for _, leak := range []string{testID, "身分證", "統編", "停權", strconv.FormatInt(f.otherID, 10)} {
		if strings.Contains(ce.Error(), leak) {
			t.Fatalf("錯誤洩漏了 %q：%v", leak, ce)
		}
	}
	if got, _ := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{})); got.Msg.GetApplication().GetStatus() != pb.KYCStatus_KYC_STATUS_DRAFT {
		t.Fatalf("身分比對失敗的申請不該被送出：%v", got)
	}
}

// SubmitKYC 每個帳號每小時最多 5 次（在入口，不論結果）：第 6 次是 resource_exhausted 帶 RetryInfo，handler 與模組不被呼叫；
// 別的帳號有自己的額度。
// 同意的文件不對時，錯誤到 client 手上的樣子：版本落後是 failed_precondition 加 reason AGREEMENT_OUTDATED；種類不對是
// invalid_argument，欄位指向 accepted_agreements。兩者都不送出，之後帶對的同意照樣能送出同一份草稿。
func TestSubmitKYCAgreementErrorsReachTheClient(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	draft := saveDraft(t, f.app, completeDraft(testID))
	outdated := kycAgreementsPB()
	outdated[1].Version = "2026-01-01"
	_, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: draft.GetEtag(), AcceptedAgreements: outdated}))
	wantCode(t, err, connect.CodeFailedPrecondition, "AGREEMENT_OUTDATED")
	for name, in := range map[string][]*pb.AgreementAcceptance{"沒有": nil, "缺一種": kycAgreementsPB()[:1], "開店的種類": pbAcceptances(seller.CurrentApplyAcceptances())} {
		_, err = f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: draft.GetEtag(), AcceptedAgreements: in}))
		if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "accepted_agreements" {
			t.Fatalf("%s：field=%q", name, field)
		}
	}
	submitted, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: draft.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	if err != nil || submitted.Msg.GetApplication().GetStatus() != pb.KYCStatus_KYC_STATUS_SUBMITTED {
		t.Fatalf("submitted=%v err=%v", submitted, err)
	}
}

func TestSubmitKYCIsRateLimitedPerAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	draft := saveDraft(t, f.app, completeDraft(testID))
	// 過期的 etag：每次都是版本衝突，仍然計一次。
	stale := &pb.SubmitKYCRequest{Etag: "99", AcceptedAgreements: kycAgreementsPB()}
	for i := range 5 {
		_, err := f.app.SubmitKYC(ctx, connect.NewRequest(stale))
		wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
		if i == 4 {
			break
		}
	}
	_, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: draft.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	wantCode(t, err, connect.CodeResourceExhausted, "")
	if !hasRetryInfo(t, err) {
		t.Fatalf("沒有 RetryInfo：%v", err)
	}
	if got, _ := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{})); got.Msg.GetApplication().GetStatus() != pb.KYCStatus_KYC_STATUS_DRAFT {
		t.Fatalf("超過額度的請求不該到達模組：%v", got)
	}
	// 別的帳號不受影響。
	f.applyComplete(t, f.other)
	theirs := saveDraft(t, f.other, completeDraft("F131104093"))
	if _, err := f.other.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: theirs.GetEtag(), AcceptedAgreements: kycAgreementsPB()})); err != nil {
		t.Fatalf("別的帳號被限流：%v", err)
	}
}

// 統編查詢：查無是沒有 registration 的回應，不可用是 unavailable；每個帳號每分鐘最多 10 次。
func TestLookupBusinessRegistration(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	lookup := func() (*pb.LookupBusinessRegistrationResponse, error) {
		resp, err := f.app.LookupBusinessRegistration(ctx, connect.NewRequest(&pb.LookupBusinessRegistrationRequest{TaxId: "33330333"}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}
	f.registry.found, f.registry.reg = true, gcis.Registration{
		TaxID: "33330333", Name: "某某有限公司", Address: "臺北市中正區重慶南路一段1號", OrganizationType: "有限公司",
		SetupDate: time.Date(1987, 2, 21, 0, 0, 0, 0, time.UTC),
	}
	resp, err := lookup()
	if err != nil || resp.GetRegistration().GetName() != "某某有限公司" || resp.GetRegistration().GetPostalAddress().GetTownCode() != "63000050" || resp.GetRegistration().GetSetupDate().GetYear() != 1987 {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	f.registry.found = false
	if resp, err := lookup(); err != nil || resp.GetRegistration() != nil {
		t.Fatalf("查無：resp=%v err=%v", resp, err)
	}
	f.registry.err = gcis.ErrUnavailable
	_, err = lookup()
	wantCode(t, err, connect.CodeUnavailable, "")
	f.registry.err = nil
	callsBefore := f.registry.calls
	// 格式錯與檢查碼錯都不打外部（但在入口計了額度）。
	for _, bad := range []string{"abc", "12345678"} {
		_, err = f.app.LookupBusinessRegistration(ctx, connect.NewRequest(&pb.LookupBusinessRegistrationRequest{TaxId: bad}))
		if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "tax_id" {
			t.Fatalf("field=%q", field)
		}
	}
	if f.registry.calls != callsBefore {
		t.Fatalf("格式或檢查碼不合的統編查了資料來源：%d 次", f.registry.calls-callsBefore)
	}
	// 到目前為止 5 次（含兩次格式與檢查碼錯誤，它們在入口就計了）；再 5 次用完 10 次的額度。
	for range 5 {
		if _, err := lookup(); err != nil {
			t.Fatal(err)
		}
	}
	_, err = lookup()
	wantCode(t, err, connect.CodeResourceExhausted, "")
	if !hasRetryInfo(t, err) {
		t.Fatalf("沒有 RetryInfo：%v", err)
	}
}

// PayUni：核准之前 GetMyKYC 的流程沒有 payuni；核准後讀得到賣家看到的狀態與要處理的事（KYCFlow.payuni）；送給 PayUni 的聯絡資料是店家的
// 欄位，以 UpdateMyStore 修改（過期的 etag 是 VERSION_CONFLICT，改了就登記同步），停權時仍可以讀、不能改。
func TestMyPayUniReadAndStoreContactUpdate(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	draft := saveDraft(t, f.app, completeDraft(testID))
	submitted, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: draft.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Msg.GetApplication().GetFlow().GetPayuni() != nil {
		t.Fatalf("送審中不該有 PayUni 狀態：%v", submitted.Msg.GetApplication().GetFlow())
	}
	version, err := strconv.ParseInt(submitted.Msg.GetApplication().GetEtag(), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.ApproveKYC(ctx, seller.ApproveKYCInput{
		ApplicationID: uuid.MustParse(submitted.Msg.GetApplication().GetId()), ExpectedVersion: version, AdminID: 9,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if fl := got.Msg.GetApplication().GetFlow(); fl.GetPayuni().GetState() != pb.PayUniState_PAY_UNI_STATE_REVIEWING || fl.GetPayuni().GetExplanation().GetMessage() == "" ||
		fl.GetCanResubmit() || len(fl.GetResubmitUnavailabilities()) != 1 ||
		fl.GetResubmitUnavailabilities()[0].GetReason() != pb.KYCResubmitUnavailableReason_KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_REVIEWING {
		t.Fatalf("核准之後 job 還沒有執行：flow=%v", fl)
	}
	// 測試沒有設定 PayUni：背景程序（這裡直接呼叫它的內容）把綁定記為失敗，問題是 PAYUNI_NOT_CONFIGURED（聯絡客服）。
	if _, err := f.module.SyncPayUni(ctx, f.storeID(t), true); err != nil {
		t.Fatal(err)
	}
	got, err = f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	flow := got.Msg.GetApplication().GetFlow()
	// 有註冊之後，PayUni 必填的店家欄位不能清掉：欄位違規指向 store.contact_name。
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{Etag: f.get(t).GetStore().GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"contact_name"}},
	}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "store.contact_name" {
		t.Fatalf("清掉必填欄位：field=%q err=%v", field, err)
	}
	if flow.GetStatus() != pb.KYCFlowStatus_KYC_FLOW_STATUS_BIND_FAILED || flow.GetPayuni().GetState() != pb.PayUniState_PAY_UNI_STATE_CONTACT_SUPPORT ||
		flow.GetPayuni().GetExplanation().GetLocale() != "zh-TW" || flow.GetPayuni().GetReason() != "" || len(flow.GetPayuni().GetFixFields()) != 0 ||
		flow.GetCanResubmit() || len(flow.GetResubmitUnavailabilities()) != 1 ||
		flow.GetResubmitUnavailabilities()[0].GetReason() != pb.KYCResubmitUnavailableReason_KYC_RESUBMIT_UNAVAILABLE_REASON_PAYUNI_CONTACT_SUPPORT {
		t.Fatalf("flow=%v", flow)
	}
	// 不能重送時 SaveKYCDraft 與 SubmitKYC 回的 failed_precondition 是原因清單的第一句。
	_, err = f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: got.Msg.GetApplication().GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	if wantCode(t, err, connect.CodeFailedPrecondition, ""); !strings.Contains(err.Error(), flow.GetResubmitUnavailabilities()[0].GetMessage().GetMessage()) {
		t.Fatalf("submit err=%v", err)
	}

	store := f.get(t).GetStore()
	updated, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store:      &pb.Store{Etag: store.GetEtag(), ContactName: "新的聯絡人", ContactPhone: "0223456789"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"contact_name", "contact_phone"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	after := updated.Msg.GetStore()
	if after.GetContactName() != "新的聯絡人" || after.GetContactPhone() != "0223456789" || after.GetContactMobile() != "0912345678" ||
		after.GetSupportEmail() != "owner@example.com" || after.GetEtag() == store.GetEtag() {
		t.Fatalf("after=%v", after)
	}
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{Etag: store.GetEtag(), ContactName: "過期的"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"contact_name"}},
	}))
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	for field, in := range map[string]*pb.Store{
		"store.contact_mobile": {ContactMobile: "123"}, "store.support_email": {SupportEmail: "not-an-email"},
		"store.contact_name": {ContactName: "這個聯絡人的名字實在是太長了所以超過二十個字的上限"},
	} {
		in.Etag = after.GetEtag()
		path := strings.TrimPrefix(field, "store.")
		_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{Store: in, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{path}}}))
		if got := wantCode(t, err, connect.CodeInvalidArgument, ""); got != field {
			t.Fatalf("%s：field=%q", field, got)
		}
	}

	f.suspendStore(t, f.storeID(t))
	if _, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{})); err != nil {
		t.Fatalf("停權時讀 PayUni 狀態：%v", err)
	}
	_, err = f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{Etag: after.GetEtag(), ContactName: "停權中"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"contact_name"}},
	}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
}

// suspendStore 以管理員的身分對店家建立沒有固定期間的 RISK 停權（模組的操作，Connect 的後台方法另有測試）。
func (f *fixture) suspendStore(t *testing.T, storeID int64) seller.Measure {
	t.Helper()
	review := time.Now().Add(7 * 24 * time.Hour)
	res, err := f.module.CreateMeasure(context.Background(), seller.CreateMeasureInput{
		AdminID: 1, StoreID: storeID, Kind: seller.MeasureSuspension, StatementToSeller: "疑似詐欺", Note: "內部紀錄", ReviewBy: &review,
		Risk: &seller.RiskBasis{Ground: seller.RiskFraud},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Measure
}

// createMeasure 以管理員的身分在呼叫者的店家上建立一個依據認定的措施：警告，或七天的停權（模組的操作）。
func (f *fixture) createMeasure(t *testing.T, kind seller.MeasureKind) string {
	t.Helper()
	in := seller.CreateMeasureInput{
		AdminID: 1, StoreID: f.storeID(t), Kind: kind, StatementToSeller: "商品說明不實", Note: "內部紀錄",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.MisleadingInfo, Severity: seller.SeverityLow}},
	}
	if kind == seller.MeasureSuspension {
		ends := time.Now().Add(7 * 24 * time.Hour)
		in.EndsAt = &ends
	}
	res, err := f.module.CreateMeasure(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return res.Measure.ID.String()
}

// 所有店家都已關閉、沒有賣家角色的帳號仍然查得到自己的措施並提出申覆（申覆方法豁免賣家角色）；別的帳號讀不到它的措施，
// 讀、申覆都是 not_found，與不存在的措施完全相同。
func TestAnAccountWithNoOpenStoreCanListGetAndAppealItsOwnMeasures(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	id := f.createMeasure(t, seller.MeasureWarning)
	if _, err := f.app.CloseMyStore(ctx, connect.NewRequest(&pb.CloseMyStoreRequest{})); err != nil {
		t.Fatal(err)
	}
	if f.isSeller(t) {
		t.Fatal("前提：關店後帳號不該還是賣家")
	}
	_, err := f.app.GetMyStore(ctx, connect.NewRequest(&pb.GetMyStoreRequest{}))
	wantCode(t, err, connect.CodeNotFound, "")

	list, err := f.app.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{}))
	if err != nil || len(list.Msg.GetMeasures()) != 1 || list.Msg.GetMeasures()[0].GetId() != id || list.Msg.GetMeasures()[0].GetStatement() != "商品說明不實" ||
		list.Msg.GetMeasures()[0].GetState() != pb.MeasureState_MEASURE_STATE_ISSUED || !list.Msg.GetMeasures()[0].GetCanAppeal() {
		t.Fatalf("list=%v err=%v", list, err)
	}
	got, err := f.app.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: id}))
	if err != nil || len(got.Msg.GetMeasure().GetRequests()) != 0 || got.Msg.GetMeasure().GetState() != pb.MeasureState_MEASURE_STATE_ISSUED {
		t.Fatalf("got=%v err=%v", got, err)
	}

	// 別的帳號（沒有店家、不是賣家）：列表是空的，讀與申覆都與不存在的措施相同。
	otherList, err := f.other.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{}))
	if err != nil || len(otherList.Msg.GetMeasures()) != 0 {
		t.Fatalf("別的帳號看到了措施：%v err=%v", otherList, err)
	}
	_, theirRead := f.other.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: id}))
	wantCode(t, theirRead, connect.CodeNotFound, "")
	_, theirAppeal := f.other.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: got.Msg.GetMeasure().GetEtag(), Reason: "借來的申訴"}))
	wantCode(t, theirAppeal, connect.CodeNotFound, "")
	_, missing := f.other.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: uuid.NewV7().String()}))
	wantCode(t, missing, connect.CodeNotFound, "")
	var theirs, absent *connect.Error
	if !errors.As(theirRead, &theirs) || !errors.As(missing, &absent) || theirs.Message() != absent.Message() || len(theirs.Details()) != len(absent.Details()) {
		t.Fatalf("別人的措施與不存在的措施回應不同：%v / %v", theirRead, missing)
	}

	// 單張佐證被 media 拒絕：BadRequest 的欄位帶那一張的索引（AIP-193），張數上限則指整個欄位。
	_, err = f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: got.Msg.GetMeasure().GetEtag(), Reason: "x", EvidenceStagedTokens: []string{"evidence", "expired-1"}}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	if fields := badRequestFields(t, err); !slices.Equal(fields, []string{"evidence_staged_tokens[1]"}) {
		t.Fatalf("fields=%v", fields)
	}
	_, err = f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: got.Msg.GetMeasure().GetEtag(), Reason: "x", EvidenceStagedTokens: []string{"1", "2", "3", "4", "5", "6"}}))
	if fields := badRequestFields(t, err); !slices.Equal(fields, []string{"evidence_staged_tokens"}) {
		t.Fatalf("fields=%v", fields)
	}
	appealed, err := f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: got.Msg.GetMeasure().GetEtag(), Reason: "我已經下架該商品", EvidenceStagedTokens: []string{"evidence"}}))
	if err != nil {
		t.Fatal(err)
	}
	a := appealed.Msg.GetMeasure().GetRequests()[0]
	if a.GetKind() != pb.MeasureRequestKind_MEASURE_REQUEST_KIND_APPEAL || a.GetState() != pb.MeasureRequestState_MEASURE_REQUEST_STATE_PENDING || a.GetReason() != "我已經下架該商品" || a.GetSubmitTime() == nil || appealed.Msg.GetMeasure().GetCanAppeal() {
		t.Fatalf("appeal=%v", a)
	}
	_, err = f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: appealed.Msg.GetMeasure().GetEtag(), Reason: "再申訴一次"}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	_, err = f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: appealed.Msg.GetMeasure().GetEtag(), Reason: ""}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "reason" && field != "" {
		t.Fatalf("field=%q", field)
	}
}

// 停權中的賣家（認定的停權）：經營類的寫入被擋下，措施與申覆照樣可以；GetMyStore 的 suspended 為真，停權的內容讀
// ListMyMeasures(state=ACTIVE)。
func TestASuspendedSellerCanStillAppeal(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created := f.apply(t)
	id := f.createMeasure(t, seller.MeasureSuspension)
	_, err := f.app.UpdateMyStore(ctx, connect.NewRequest(&pb.UpdateMyStoreRequest{
		Store: &pb.Store{StoreName: "x", Etag: created.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_name"}},
	}))
	wantCode(t, err, connect.CodePermissionDenied, "SELLER_SUSPENDED")
	store, err := f.app.GetMyStore(ctx, connect.NewRequest(&pb.GetMyStoreRequest{}))
	if err != nil || !store.Msg.GetSuspended() {
		t.Fatalf("suspended=%v err=%v", store.Msg.GetSuspended(), err)
	}
	active, err := f.app.ListMyMeasures(ctx, connect.NewRequest(&pb.ListMyMeasuresRequest{State: pb.MeasureState_MEASURE_STATE_ACTIVE}))
	if err != nil || len(active.Msg.GetMeasures()) != 1 || active.Msg.GetMeasures()[0].GetId() != id || active.Msg.GetMeasures()[0].GetPeriod().GetEndTime() == nil {
		t.Fatalf("active=%v err=%v", active, err)
	}
	got, err := f.app.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: id}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: got.Msg.GetMeasure().GetEtag(), Reason: "誤判"})); err != nil {
		t.Fatal(err)
	}
}

// 申覆與後台的撤銷、裁定都要帶讀到的 etag（ADR 2026-09-25-optimistic-concurrency-etag 2026-10-04 修訂）：沒帶是 invalid_argument，
// 管理員在這之間修改了措施（UpdateMeasure）是 aborted 加 VERSION_CONFLICT；後台的指令生效後帶舊 etag 再送一次也是 aborted（不推斷
// 重送）；賣家的 etag 只隨賣家看得到的版本改變，申覆生效後重送同一個 etag 是 failed_precondition（已申覆過）。
func TestAppealAndGovernanceCommandsCarryTheMeasureEtag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	id := f.createMeasure(t, seller.MeasureSuspension)
	measureID, err := uuid.Parse(id)
	if err != nil {
		t.Fatal(err)
	}
	read := func() *pb.Measure {
		t.Helper()
		got, err := f.app.GetMyMeasure(ctx, connect.NewRequest(&pb.GetMyMeasureRequest{MeasureId: id}))
		if err != nil {
			t.Fatal(err)
		}
		return got.Msg.GetMeasure()
	}
	submit := func(etag string) (*connect.Response[pb.SubmitAppealResponse], error) {
		return f.app.SubmitAppeal(ctx, connect.NewRequest(&pb.SubmitAppealRequest{MeasureId: id, Etag: etag, Reason: "誤判"}))
	}
	first := read()
	if first.GetEtag() == "" {
		t.Fatal("賣家看到的措施沒有 etag")
	}

	_, err = submit("")
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "etag" {
		t.Fatalf("field=%q", field)
	}

	statement := "管理員改過的理由"
	admin0, err := f.module.Measure(ctx, measureID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.UpdateMeasure(ctx, seller.UpdateMeasureInput{MeasureID: measureID, AdminID: 1, ExpectedVersion: admin0.Version, StatementToSeller: &statement}); err != nil {
		t.Fatal(err)
	}
	if got := read(); got.GetEtag() == first.GetEtag() || got.GetStatement() != statement {
		t.Fatalf("修改之後 etag 必須變：%v", got)
	}
	_, err = submit(first.GetEtag())
	wantCode(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	if len(read().GetRequests()) != 0 {
		t.Fatal("過時的 etag 不能寫入申訴")
	}

	current := read().GetEtag()
	appealed, err := submit(current)
	if err != nil {
		t.Fatal(err)
	}
	_, err = submit(current)
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	if read().GetEtag() != appealed.Msg.GetMeasure().GetEtag() {
		t.Fatal("申訴重送不能寫入")
	}

	gov := httpsellerpkg.NewGovernance(f.module, rpc.Images{})
	admin := rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAdmin, ID: 1})
	decide := func(etag string) (*connect.Response[adminpb.DecideAppealResponse], error) {
		return gov.DecideAppeal(admin, connect.NewRequest(&adminpb.DecideAppealRequest{
			MeasureId: id, Etag: etag, Decision: pb.MeasureRequestState_MEASURE_REQUEST_STATE_REJECTED, Reason: "證據不足",
		}))
	}
	_, err = decide("")
	wantKind(t, err, apperr.KindInvalidInput)
	// 提出申覆起措施凍結，申覆之後唯一還能讓 etag 前進的修改是縮短結束時間（停權七天，縮成一天）。
	stored, err := f.module.Measure(ctx, measureID)
	if err != nil {
		t.Fatal(err)
	}
	staleAdmin := rpc.Etag(stored.Version)
	shorter := stored.EndsAt.Add(-24 * time.Hour)
	if _, err := f.module.UpdateMeasure(ctx, seller.UpdateMeasureInput{MeasureID: measureID, AdminID: 1, ExpectedVersion: stored.Version, ChangeEndsAt: true, EndsAt: &shorter}); err != nil {
		t.Fatal(err)
	}
	_, err = decide(staleAdmin)
	wantKind(t, err, apperr.KindVersionConflict)
	fresh := rpc.Etag(stored.Version + 1)
	decided, err := decide(fresh)
	if err != nil {
		t.Fatal(err)
	}
	_, err = decide(fresh)
	wantKind(t, err, apperr.KindVersionConflict)
	if got := read(); got.GetRequests()[0].GetState() != pb.MeasureRequestState_MEASURE_REQUEST_STATE_REJECTED {
		t.Fatalf("read=%v", got)
	}
	afterDecide := decided.Msg.GetMeasure().GetEtag()

	revoke := func(etag string) (*connect.Response[adminpb.RevokeMeasureResponse], error) {
		return gov.RevokeMeasure(admin, connect.NewRequest(&adminpb.RevokeMeasureRequest{
			Id: id, Etag: etag, Reason: adminpb.AdminRevokeReason_ADMIN_REVOKE_REASON_ADMIN_CORRECTION,
		}))
	}
	_, err = revoke("")
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = revoke(fresh)
	wantKind(t, err, apperr.KindVersionConflict)
	revoked, err := revoke(afterDecide)
	if err != nil {
		t.Fatal(err)
	}
	_, err = revoke(afterDecide)
	wantKind(t, err, apperr.KindVersionConflict)
	if read().GetState() != pb.MeasureState_MEASURE_STATE_REVOKED || revoked.Msg.GetMeasure().GetState() != pb.MeasureState_MEASURE_STATE_REVOKED {
		t.Fatal("撤銷重送不能寫入")
	}
}

// badRequestFields 回傳錯誤的 BadRequest 違規的欄位路徑。
func badRequestFields(t *testing.T, err error) []string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("not a connect error: %v", err)
	}
	var out []string
	for _, d := range ce.Details() {
		v, e := d.Value()
		if e != nil {
			t.Fatal(e)
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				out = append(out, fv.GetField())
			}
		}
	}
	slices.Sort(out)
	return out
}

// 送出時缺少的每一項都對應到 KYCApplication 的欄位（BadRequest 的違規），client 才知道缺什麼；個人與企業賣家缺的項目不同。
func TestSubmitKYCReportsEveryMissingItemAsAFieldViolation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.apply(t)
	individual := saveDraft(t, f.app, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{}}},
		UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"individual"}},
	})
	_, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: individual.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	want := []string{"bank_account", "bank_code", "bank_name", "bank_book", "english_name", "id_card_back", "id_card_front", "individual.id_card_no", "real_name"}
	slices.Sort(want)
	wantStore := []string{"store.contact_mobile", "store.contact_name", "store.support_email"}
	if got := badRequestFields(t, err); !slices.Equal(got, want) {
		t.Fatalf("個人賣家缺少的欄位（只有申請自己的欄位，店家的缺項不是欄位違規）：got=%v want=%v", got, want)
	}
	// 申請的檢視帶同一份清單（missing_fields 與 missing_store_fields），送出只在真的送審時才需要用到額度。
	if got := missingPaths(individual); !slices.Equal(got, want) {
		t.Fatalf("missing_fields 與送出的違規不一致：got=%v want=%v", got, want)
	}
	if got := missingStorePaths(individual); !slices.Equal(got, wantStore) {
		t.Fatalf("missing_store_fields 與送出的違規不一致：got=%v want=%v", got, wantStore)
	}
	if mine, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{})); err != nil || !slices.Equal(missingPaths(mine.Msg.GetApplication()), want) ||
		!slices.Equal(missingStorePaths(mine.Msg.GetApplication()), wantStore) {
		t.Fatalf("GetMyKYC：err=%v mine=%v", err, mine)
	}
	// 申請本身齊全、店家資料還缺：failed_precondition（不是欄位違規），缺哪些看 missing_store_fields；補齊之後才能送出。
	full := saveDraft(t, f.app, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{
			Etag: individual.GetEtag(), Subject: &pb.KYCApplication_Individual{Individual: &pb.IndividualKYC{IdCardNo: testID}},
			RealName: "王小明", BirthDate: &date.Date{Year: 1990, Month: 5, Day: 6}, EnglishName: "Wang Farm", BankName: "臺灣銀行", BankCode: "004",
			BankAccount: "012345678901", IdCardFront: &pb.KYCDocument{StagedToken: "front"}, IdCardBack: &pb.KYCDocument{StagedToken: "back"},
			BankBook: &pb.KYCDocument{StagedToken: "book"},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{
			"individual.id_card_no", "real_name", "birth_date", "english_name", "bank_name", "bank_code", "bank_account", "id_card_front", "id_card_back", "bank_book",
		}},
	})
	_, err = f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: full.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	wantCode(t, err, connect.CodeFailedPrecondition, "")
	if got := badRequestFields(t, err); len(got) != 0 {
		t.Fatalf("店家資料缺項不該帶欄位違規：%v", got)
	}
	// 店家資料補齊之後，缺項只剩申請自己的欄位。
	filled := f.fillContact(t, f.app)
	if got := missingStorePaths(saveDraft(t, f.app, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: full.GetEtag()}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"product_type"}},
	})); len(got) != 0 || filled.GetContactName() == "" {
		t.Fatalf("補齊聯絡資料之後 missing_store_fields=%v store=%v", got, filled)
	}

	f.applyComplete(t, f.other)
	company := saveDraft(t, f.other, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Subject: &pb.KYCApplication_Business{Business: &pb.BusinessKYC{}}},
		UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"business"}},
	})
	_, err = f.other.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: company.GetEtag(), AcceptedAgreements: kycAgreementsPB()}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	got := badRequestFields(t, err)
	for _, field := range []string{"business.company_name", "business.tax_id", "business.rep_id_card_no", "id_card_front"} {
		if !slices.Contains(got, field) {
			t.Fatalf("企業賣家缺少的欄位沒有 %s：%v", field, got)
		}
	}
	if slices.Contains(got, "individual.id_card_no") {
		t.Fatalf("企業賣家不需要個人的身分證號：%v", got)
	}
	if !slices.Equal(missingPaths(company), got) {
		t.Fatalf("企業賣家的 missing_fields 與送出的違規不一致：%v / %v", missingPaths(company), got)
	}
}

// missingPaths 是申請的 missing_fields 對應的欄位路徑（排序）。
func missingStorePaths(a *pb.KYCApplication) []string {
	var out []string
	for _, f := range a.GetMissingStoreFields() {
		out = append(out, "store."+strings.ToLower(strings.TrimPrefix(pb.StoreField_name[int32(f)], "STORE_FIELD_")))
	}
	slices.Sort(out)
	return out
}

func missingPaths(a *pb.KYCApplication) []string {
	var out []string
	for _, f := range a.GetMissingFields() {
		name := strings.ToLower(strings.TrimPrefix(pb.KYCField_name[int32(f)], "KYC_FIELD_"))
		for _, branch := range []string{"individual_", "business_"} {
			if strings.HasPrefix(name, branch) {
				name = strings.TrimSuffix(branch, "_") + "." + strings.TrimPrefix(name, branch)
			}
		}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// 證件是狀態：換掉單張的位置、以 asset_id 保留現有的、清掉一張；回應帶有授權的下載網址，不帶 staged token。
func TestKYCDocumentsAreStateOverConnect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	created := saveDraft(t, f.app, completeDraft(testID))
	next := saveDraft(t, f.app, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{
			Etag: created.GetEtag(), IdCardFront: &pb.KYCDocument{StagedToken: "front2"}, IdCardBack: &pb.KYCDocument{AssetId: created.GetIdCardBack().GetAssetId()},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"id_card_front", "id_card_back", "bank_book"}},
	})
	if next.GetIdCardFront().GetAssetId() != "asset-front2" || next.GetIdCardBack().GetAssetId() != created.GetIdCardBack().GetAssetId() || next.GetBankBook() != nil {
		t.Fatalf("documents: front=%v back=%v book=%v", next.GetIdCardFront(), next.GetIdCardBack(), next.GetBankBook())
	}
	if next.GetIdCardFront().GetDownloadUri() != "/api/v1/app/media/secure_download/asset-front2" {
		t.Fatalf("download uri=%q", next.GetIdCardFront().GetDownloadUri())
	}
	// 指定的現有圖片不是這份申請目前的證件。
	_, err := f.app.SaveKYCDraft(ctx, connect.NewRequest(&pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: next.GetEtag(), IdCardFront: &pb.KYCDocument{AssetId: "asset-of-someone-else"}},
		UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"id_card_front"}},
	}))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "application.id_card_front" {
		t.Fatalf("field=%q", field)
	}
}

// update_mask 是 "*"：個人與企業賣家都能用（不適用的欄位沒有值時不算有提供）。
func TestSaveKYCDraftWildcardMaskOverConnect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.applyComplete(t, f.app)
	req := completeDraft(testID)
	req.UpdateMask = &fieldmaskpb.FieldMask{Paths: []string{"*"}}
	if got := saveDraft(t, f.app, req); got.GetIndividual() == nil || got.GetBusiness() != nil || got.GetRealName() != "王小明" || got.GetIndividual().GetIdCardNoMasked() == "" {
		t.Fatalf("個人賣家：%v", got)
	}
	f.applyComplete(t, f.other)
	company := saveDraft(t, f.other, &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{
			RealName: "王小明",
			Subject: &pb.KYCApplication_Business{Business: &pb.BusinessKYC{
				CompanyName: "範例小農有限公司", TaxId: "33330333", RepIdCardNo: "X223456789", CompanyDocuments: []*pb.KYCDocument{{StagedToken: "co1"}},
			}},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"*"}},
	})
	if company.GetBusiness().GetCompanyName() != "範例小農有限公司" || company.GetIndividual() != nil || company.GetBusiness().GetRepIdCardNoMasked() == "" ||
		len(company.GetBusiness().GetCompanyDocuments()) != 1 {
		t.Fatalf("企業賣家：%v", company)
	}
}

// 宣告的能力與模組實際檢查的能力一致：宣告允許的狀態，模組不能再以別的能力拒絕（宣告表是唯一的決定處，模組的檢查是深度
// 防禦，不能比宣告更嚴也不能是另一個）。
func TestDeclaredCapabilitiesMatchWhatTheModuleEnforces(t *testing.T) {
	t.Parallel()
	procedures := []string{
		sellerconnect.SellerAppServiceSaveKYCDraftProcedure, sellerconnect.SellerAppServiceGetMyKYCProcedure,
		sellerconnect.SellerAppServiceSubmitKYCProcedure, sellerconnect.SellerAppServiceCancelKYCProcedure,
	}
	for _, state := range []string{"active", "suspended", "pending deletion"} {
		t.Run(state, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			f.applyComplete(t, f.app)
			draft := saveDraft(t, f.app, completeDraft(testID))
			switch state {
			case "suspended":
				f.suspendStore(t, f.storeID(t))
			case "pending deletion":
				storeID := f.storeID(t)
				if err := f.inTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, storeID) }); err != nil {
					t.Fatal(err)
				}
			}
			current, err := f.module.CurrentStore(ctx, f.accountID)
			if err != nil {
				t.Fatal(err)
			}
			calls := map[string]func() error{
				sellerconnect.SellerAppServiceSaveKYCDraftProcedure: func() error {
					_, err := f.app.SaveKYCDraft(ctx, connect.NewRequest(&pb.SaveKYCDraftRequest{Application: &pb.KYCApplication{Etag: draft.GetEtag(), RealName: "改"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"real_name"}}}))
					return err
				},
				sellerconnect.SellerAppServiceGetMyKYCProcedure: func() error {
					_, err := f.app.GetMyKYC(ctx, connect.NewRequest(&pb.GetMyKYCRequest{}))
					return err
				},
				sellerconnect.SellerAppServiceSubmitKYCProcedure: func() error {
					_, err := f.app.SubmitKYC(ctx, connect.NewRequest(&pb.SubmitKYCRequest{Etag: "99", AcceptedAgreements: kycAgreementsPB()}))
					return err
				},
				sellerconnect.SellerAppServiceCancelKYCProcedure: func() error {
					_, err := f.app.CancelKYC(ctx, connect.NewRequest(&pb.CancelKYCRequest{Etag: "99"}))
					return err
				},
			}
			for _, procedure := range procedures {
				capability, ok := httpsellerpkg.DeclaredCapability(procedure)
				if !ok {
					t.Fatalf("%s 沒有宣告能力", procedure)
				}
				err := calls[procedure]()
				denied := connect.CodeOf(err) == connect.CodePermissionDenied || (connect.CodeOf(err) == connect.CodeFailedPrecondition && errorReason(err) == "SELLER_REQUIREMENT_MISSING")
				if allowed := current.Can(capability); allowed && denied {
					t.Errorf("%s：宣告的能力 %s 在 %s 的狀態允許，卻被拒絕：%v", procedure, capability, state, err)
				} else if !allowed && !denied {
					t.Errorf("%s：宣告的能力 %s 在 %s 的狀態不允許，卻沒有被拒絕：%v", procedure, capability, state, err)
				}
			}
		})
	}
}

// 切換賣家型態：設定另一支並且 update_mask 列到它的欄位，原本那一支專屬的欄位一併清掉；統編的檢查碼與登記資料查詢同一個規則。
func TestSaveKYCDraftSwitchesTheSellerTypeAndValidatesTheTaxID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.applyComplete(t, f.app)
	created := saveDraft(t, f.app, completeDraft(testID))
	if created.GetIndividual().GetIdCardNoMasked() == "" {
		t.Fatalf("created=%v", created)
	}
	bad := &pb.SaveKYCDraftRequest{
		Application: &pb.KYCApplication{Etag: created.GetEtag(), Subject: &pb.KYCApplication_Business{Business: &pb.BusinessKYC{CompanyName: "某某有限公司", TaxId: "12345678"}}},
		UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"business.company_name", "business.tax_id"}},
	}
	_, err := f.app.SaveKYCDraft(ctx, connect.NewRequest(bad))
	if field := wantCode(t, err, connect.CodeInvalidArgument, ""); field != "application.business.tax_id" {
		t.Fatalf("field=%q", field)
	}
	bad.Application.GetBusiness().TaxId = "33330333"
	switched := saveDraft(t, f.app, bad)
	if switched.GetIndividual() != nil || switched.GetBusiness().GetTaxId() != "33330333" || switched.GetBusiness().GetCompanyName() != "某某有限公司" {
		t.Fatalf("switched=%v", switched)
	}
	// 個人專屬的身分證號隨型態切換清掉；共用的欄位保留。
	if switched.GetRealName() != "王小明" || slices.Contains(missingPaths(switched), "real_name") || !slices.Contains(missingPaths(switched), "business.rep_id_card_no") {
		t.Fatalf("switched=%v missing=%v", switched, missingPaths(switched))
	}
}

// wantKind 確認錯誤是模組的指定種類（直接呼叫 handler 時還沒經過 interceptor 轉成 Connect 的錯誤）。
func wantKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("err=%v want kind=%v", err, kind)
	}
}

// kycAgreementsPB 是 SubmitKYC 要求的同意（現行版本）。
func kycAgreementsPB() []*pb.AgreementAcceptance {
	return pbAcceptances(seller.CurrentKYCAcceptances())
}

func pbAcceptances(in []model.AgreementAcceptance) []*pb.AgreementAcceptance {
	out := make([]*pb.AgreementAcceptance, 0, len(in))
	for _, a := range in {
		out = append(out, &pb.AgreementAcceptance{Kind: rpc.EnumToProto[pb.AgreementKind](a.Kind), Version: a.Version})
	}
	return out
}

// 賣家看到的停權（GetMyStore.suspended、能力 NOT_SUSPENDED）與後台看到的（ListStores 的 suspended 篩選與列上的 suspended）
// 來自同一份措施：RISK 措施生效時四者同為停權，結束或撤銷後四者同時解除。
func TestSuspendedFlagCapabilityAndAdminListChangeTogetherOnEndAndRevoke(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.apply(t)
	storeID := f.storeID(t)
	characterize.GrantAdminOwner(t, f.id, f.adminTok)
	admin := adminsellerconnect.NewSellerAdminServiceClient(f.http, f.base, clientOptions(f.adminTok)...)
	observe := func() [4]bool {
		t.Helper()
		store := f.get(t)
		blocked := slices.Contains(capabilityStatus(t, store, pb.SellerCapability_SELLER_CAPABILITY_STOREFRONT_WRITE).GetMissing(), pb.SellerRequirement_SELLER_REQUIREMENT_NOT_SUSPENDED)
		yes := true
		filtered, err := admin.ListStores(ctx, connect.NewRequest(&adminpb.ListStoresRequest{Suspended: &yes, PageSize: 50}))
		if err != nil {
			t.Fatal(err)
		}
		inFilter, flagged := false, false
		for _, item := range filtered.Msg.GetStores() {
			if item.GetStore().GetId() == strconv.FormatInt(storeID, 10) {
				inFilter, flagged = true, item.GetSuspended()
			}
		}
		return [4]bool{store.GetSuspended(), blocked, inFilter, flagged}
	}
	if got := observe(); got != [4]bool{} {
		t.Fatalf("尚未停權：%v", got)
	}
	for name, release := range map[string]func(seller.Measure){
		"end": func(m seller.Measure) {
			if _, err := f.module.EndMeasure(ctx, seller.EndMeasureInput{MeasureID: m.ID, AdminID: 1, ExpectedVersion: m.Version}); err != nil {
				t.Fatal(err)
			}
		},
		"revoke": func(m seller.Measure) {
			if _, err := f.module.RevokeMeasure(ctx, seller.RevokeMeasureInput{MeasureID: m.ID, AdminID: 1, ExpectedVersion: m.Version, Reason: seller.RevokeAdminCorrection}); err != nil {
				t.Fatal(err)
			}
		},
	} {
		m := f.suspendStore(t, storeID)
		if got := observe(); got != [4]bool{true, true, true, true} {
			t.Fatalf("%s：停權中 %v", name, got)
		}
		release(m)
		if got := observe(); got != [4]bool{} {
			t.Fatalf("%s：解除後 %v", name, got)
		}
	}
}
