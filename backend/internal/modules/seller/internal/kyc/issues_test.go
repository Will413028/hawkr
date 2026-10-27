package kyc

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 文件類欄位依原因碼決定要不要帶 asset：UNREADABLE、EXPIRED、MISMATCH、INVALID 必須帶，MISSING 一律不能帶，OTHER 兩者皆可；
// 非文件類的欄位永遠不能帶。違反回 invalid_argument 並指到 DocumentAssetID。
func TestValidateIssuesDocumentAssetRulesByCode(t *testing.T) {
	const doc, plain = "business.company_documents", "real_name"
	cases := []struct {
		name    string
		field   string
		code    model.KYCIssueCode
		asset   string
		wantErr bool
	}{
		{"UNREADABLE 帶 asset", doc, model.IssueUnreadable, "a1", false},
		{"UNREADABLE 不帶 asset", doc, model.IssueUnreadable, "", true},
		{"EXPIRED 不帶 asset", "id_card_front", model.IssueExpired, "", true},
		{"MISMATCH 不帶 asset", "bank_book", model.IssueMismatch, "", true},
		{"INVALID 不帶 asset", doc, model.IssueInvalid, "", true},
		{"INVALID 帶 asset", doc, model.IssueInvalid, "a1", false},
		{"MISSING 不帶 asset", doc, model.IssueMissing, "", false},
		{"MISSING 帶 asset", doc, model.IssueMissing, "a1", true},
		{"OTHER 不帶 asset", doc, model.IssueOther, "", false},
		{"OTHER 帶 asset", doc, model.IssueOther, "a1", false},
		{"非文件欄位不能帶 asset", plain, model.IssueOther, "a1", true},
		{"非文件欄位的 UNREADABLE 不帶 asset 照常", plain, model.IssueUnreadable, "", false},
	}
	for _, c := range cases {
		_, err := validateIssues([]model.KYCIssue{{Field: c.field, Code: c.code, DocumentAssetID: c.asset, Message: "說明"}})
		if (err != nil) != c.wantErr {
			t.Errorf("%s：err=%v，wantErr=%v", c.name, err, c.wantErr)
			continue
		}
		if err != nil && apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：kind=%v", c.name, apperr.KindOf(err))
		}
	}
}

// 每個原因碼都有規則（必須帶、不可帶、兩者皆可三選一），新增原因碼時這裡會提醒補規則。
func TestEveryIssueCodeHasAnAssetRule(t *testing.T) {
	for _, code := range model.KYCIssueCodeValues() {
		required, forbidden := model.IssueAssetRequired(code), model.IssueAssetForbidden(code)
		if required && forbidden {
			t.Errorf("%s 同時必須帶與不可帶", code)
		}
		if code == model.IssueOther && (required || forbidden) {
			t.Errorf("OTHER 應該兩者皆可")
		}
	}
}

// 可指出問題的欄位只有一份規則：RejectKYC 的驗證與後台的 issuable_fields 都用 model.IssuableFields。
func TestIssuableFieldsFollowTheSellerType(t *testing.T) {
	contains := func(fields []string, f string) bool {
		for _, x := range fields {
			if x == f {
				return true
			}
		}
		return false
	}
	ind, biz := model.IssuableFields(model.KYCIndividual), model.IssuableFields(model.KYCCompany)
	if !contains(ind, "individual.id_card_no") || contains(ind, "business.tax_id") || !contains(ind, "real_name") {
		t.Errorf("個人：%v", ind)
	}
	if contains(biz, "individual.id_card_no") || !contains(biz, "business.tax_id") || !contains(biz, "business.company_documents") {
		t.Errorf("企業：%v", biz)
	}
	// 驗證用同一份：個人申請指企業欄位被擋，企業欄位在企業申請可用。
	if err := checkIssuesAgainstApplication([]model.KYCIssue{{Field: "business.tax_id"}}, model.KYCIndividual, nil); err == nil {
		t.Error("個人申請不能指企業欄位")
	}
	if err := checkIssuesAgainstApplication([]model.KYCIssue{{Field: "business.tax_id"}}, model.KYCCompany, nil); err != nil {
		t.Errorf("企業申請可以指統編：%v", err)
	}
}
