//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 已核准、PayUni 審核中：儲存草稿與重新送出都回同一句（原因清單的第一句），句子與 KYCFlow 帶的原因一致；這份申請沒有能自己動手的事。
func TestResubmitIsRefusedWithTheFirstReasonWhileAPayUniStateBlocksIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.boundStore(t)
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.PayUni == nil || mine.PayUni.State != PayUniState("REVIEWING") || mine.Flow.CanResubmit || len(mine.ResubmitReasons) != 1 {
		t.Fatalf("payuni=%+v flow=%+v reasons=%v", mine.PayUni, mine.Flow, mine.ResubmitReasons)
	}
	const want = "PayUni 正在審查你的資料，審查期間不能修改或重新送出商家認證。"
	if got := mine.ResubmitReasons[0].Text(uitext.ZhTW).Text; got != want {
		t.Fatalf("原因句 %q", got)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), want) {
		t.Fatalf("SaveKYCDraft：%v", err)
	}
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: mine.Version})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), want) {
		t.Fatalf("SubmitKYC：%v", err)
	}
	// 補件、核准之後也各有自己的原因，不能重送。
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	mine, err = f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.PayUni == nil || mine.PayUni.State != PayUniState("APPROVED") || len(mine.ResubmitReasons) != 1 || mine.ResubmitReasons[0] != KYCResubmitUnavailableReason("PAYUNI_APPROVED") {
		t.Fatalf("payuni=%+v reasons=%v", mine.PayUni, mine.ResubmitReasons)
	}
}

// 綁定失敗而且 PayUni 指出的是申請的身分欄位：PayUni 沒有收下這家店，身分欄位本來就解鎖，賣家改申請、重新送出、平台核准之後自動重新綁定。
func TestBindFailureOnIdentityFieldsLetsTheSellerFixAndResubmit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT03001")}
	f.sync(t, s.StoreID)
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != "AGENT03001" || f.count(t, fmt.Sprintf("SELECT count(*) FROM seller.payuni_registrations WHERE store_id = %d AND bind_failed_at IS NOT NULL", s.StoreID)) != 1 {
		t.Fatalf("registration=%+v", r)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.PayUni == nil || mine.PayUni.State != PayUniState("FIXABLE") || !mine.Flow.CanResubmit || mine.Flow.IdentityLocked || len(mine.ResubmitReasons) != 0 {
		t.Fatalf("payuni=%+v flow=%+v reasons=%v", mine.PayUni, mine.Flow, mine.ResubmitReasons)
	}
	has := func(field string) bool {
		for _, f := range mine.PayUni.FixFields {
			if f == field {
				return true
			}
		}
		return false
	}
	if !has("real_name") || !has("individual.id_card_no") || has("business.rep_id_card_no") {
		t.Fatalf("可修正的欄位（個人賣家）：%v", mine.PayUni.FixFields)
	}
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王大明")})
	must(t, err)
	if edited.Status != KYCStatusDraft || edited.PayUni == nil || edited.PayUni.State != PayUniState("FIXABLE") {
		t.Fatalf("edited=%+v payuni=%+v", edited, edited.PayUni)
	}
	again, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: edited.Version})
	must(t, err)
	must(t, approve(f, again))
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusPending || f.count(t, fmt.Sprintf("SELECT count(*) FROM seller.payuni_registrations WHERE store_id = %d AND bind_failed_at IS NOT NULL", s.StoreID)) != 0 {
		t.Fatalf("核准之後註冊要回到 PENDING 重新綁定：%+v", r)
	}
	f.payuni.bindErr = nil
	f.sync(t, s.StoreID)
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusBound {
		t.Fatalf("registration=%+v", r)
	}
}

// 綁定失敗在申請欄位上（AGENT03001）時賣家能自己改：儲存草稿把已核准的申請改回草稿之後，管理員重新綁定被擋下並說明原因
// （綁定 job 會以註冊的申請讀資料，送給 PayUni 的會是還沒審核的草稿），註冊不變；全程只走產品的 API，沒有直接改資料。
// mutation：拿掉 Rebind 的申請檢查，註冊會回到 PENDING。
func TestRebindIsRefusedWhileTheSellerEditsTheApplicationAfterABindFailure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.approvedStore(t)
	f.payuni.bindErr = []error{apiErr("AGENT03001")}
	f.sync(t, s.StoreID)
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	edited, err := f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王大明")})
	must(t, err)
	if edited.Status != KYCStatusDraft {
		t.Fatalf("儲存草稿要把申請改回草稿：%+v", edited)
	}
	_, err = f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: s.StoreID, AdminID: 9})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "賣家正在修改商家認證，重新核准後會自動重新綁定。") {
		t.Fatalf("賣家正在修改時重新綁定要被擋下並說明原因：%v", err)
	}
	if r := f.registration(t, s.StoreID); r.BindStatus != BindStatusFailed || deref(r.BindErrorCode) != "AGENT03001" {
		t.Fatalf("被擋下之後註冊不該改變：%+v", r)
	}
}

// 綁定重試用完：賣家等平台重新送出，不能自己改或重送；原因句是「平台會協助重新送出」。
func TestRetryExhaustedBindFailureIsRetryLaterAndBlocksResubmit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.approvedStore(t)
	f.payuni.bindErr = []error{errNetwork}
	if _, err := f.module.SyncPayUni(ctx, s.StoreID, true); err != nil {
		t.Fatal(err)
	}
	mine, err := f.module.MyKYC(ctx, accountID)
	must(t, err)
	if mine.PayUni == nil || mine.PayUni.State != PayUniState("RETRY_LATER") || mine.Flow.CanResubmit || len(mine.ResubmitReasons) != 1 || mine.ResubmitReasons[0] != KYCResubmitUnavailableReason("PAYUNI_RETRY_LATER") {
		t.Fatalf("payuni=%+v flow=%+v reasons=%v", mine.PayUni, mine.Flow, mine.ResubmitReasons)
	}
	_, err = f.module.SaveKYCDraft(ctx, KYCDraftInput{AccountID: accountID, ExpectedVersion: mine.Version, RealName: ptr("王小明")})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "PayUni 暫時無法處理，平台會協助重新送出，你不需要修改或重新送出。") {
		t.Fatalf("SaveKYCDraft：%v", err)
	}
}
