package seller

import (
	"testing"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 後台看到的食品業者登錄：店家沒有 KYC 申請時 kyc_status 不設（optional），不是 UNSPECIFIED；有申請時照實回傳。
func TestToAdminFoodRegistrationKYCStatusPresence(t *testing.T) {
	base := model.FoodRegistrationReview{StoreID: 7, State: model.FoodRegPending}

	if none := toAdminFoodRegistration(base); none.KycStatus != nil {
		t.Fatalf("沒有 KYC 申請時 kyc_status 必須不設：%v", none.GetKycStatus())
	}

	for status, want := range map[model.KYCStatus]pb.KYCStatus{
		model.KYCStatusSubmitted: pb.KYCStatus_KYC_STATUS_SUBMITTED,
		model.KYCStatusApproved:  pb.KYCStatus_KYC_STATUS_APPROVED,
	} {
		in := base
		in.KYCStatus = &status
		got := toAdminFoodRegistration(in)
		if got.KycStatus == nil || got.GetKycStatus() != want {
			t.Fatalf("%s：kyc_status=%v want=%v", status, got.KycStatus, want)
		}
	}
}
