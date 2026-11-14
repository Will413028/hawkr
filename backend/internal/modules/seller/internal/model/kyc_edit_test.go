package model

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestCanEditKYCOnlyForDraftAndRejected(t *testing.T) {
	want := map[KYCStatus]bool{
		KYCStatusDraft: true, KYCStatusRejected: true,
		KYCStatusSubmitted: false, KYCStatusApproved: false, KYCStatusCancelled: false, "": false, "UNKNOWN": false,
	}
	for status, ok := range want {
		if got := CanEditKYC(status); got != ok {
			t.Errorf("CanEditKYC(%q)=%v，要 %v", status, got, ok)
		}
	}
	for _, status := range KYCStatusValues() {
		if _, ok := want[status]; !ok {
			t.Errorf("狀態 %q 沒有列在預期表裡", status)
		}
	}
}

func TestErrKYCNotEditableSaysWhyPerStatus(t *testing.T) {
	submitted := ErrKYCNotEditable(KYCStatusSubmitted, uitext.ZhTW)
	approved := ErrKYCNotEditable(KYCStatusApproved, uitext.ZhTW)
	for name, err := range map[string]error{"審核中": submitted, "已核准": approved} {
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s：err=%v", name, err)
		}
	}
	if !strings.Contains(submitted.Error(), "取消") {
		t.Errorf("審核中要說明先取消：%v", submitted)
	}
	if submitted.Error() == approved.Error() {
		t.Errorf("兩個狀態的說明不該相同：%v", submitted)
	}
	for _, status := range []KYCStatus{KYCStatusDraft, KYCStatusRejected, "UNKNOWN"} {
		if err := ErrKYCNotEditable(status, uitext.ZhTW); err == nil || apperr.KindOf(err) == apperr.KindFailedPrecondition {
			t.Errorf("%q 沒有不能編輯的原因，要回 internal 錯誤：%v", status, err)
		}
	}
}
