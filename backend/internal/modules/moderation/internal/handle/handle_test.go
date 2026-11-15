package handle

import (
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func violation(a model.Action) model.DecideInput {
	return model.DecideInput{
		CaseID: uuid.NewV7(), Version: 1, AdminID: 3, Decision: model.DecisionViolation, Category: policy.Spam, Action: a, Text: "理由",
	}
}

func TestValidateViolationShapes(t *testing.T) {
	takedown := violation(model.ActionTakedown)
	takedown.StoreID, takedown.Severity = 5, string(seller.SeverityValues()[0])
	for name, in := range map[string]model.DecideInput{
		"隱藏": violation(model.ActionHide), "警告": violation(model.ActionWarn), "停權": violation(model.ActionBan), "下架": takedown,
	} {
		if _, err := Validate(in); err != nil {
			t.Errorf("%s：合法的輸入被拒絕：%v", name, err)
		}
	}
	bad := map[string]func(*model.DecideInput){
		"沒有案件":    func(in *model.DecideInput) { in.CaseID = uuid.UUID{} },
		"版本為零":    func(in *model.DecideInput) { in.Version = 0 },
		"沒有管理員":   func(in *model.DecideInput) { in.AdminID = 0 },
		"沒有處置":    func(in *model.DecideInput) { in.Action = "" },
		"處置不認得":   func(in *model.DecideInput) { in.Action = "DISMISS" },
		"沒有類別":    func(in *model.DecideInput) { in.Category = "" },
		"OTHER":   func(in *model.DecideInput) { in.Category = policy.Other },
		"商品類別":    func(in *model.DecideInput) { in.Category = policy.Counterfeit },
		"理由空白":    func(in *model.DecideInput) { in.Text = " 　 " },
		"理由太長":    func(in *model.DecideInput) { in.Text = strings.Repeat("字", model.MaxTextRunes+1) },
		"隱藏帶了店家":  func(in *model.DecideInput) { in.StoreID = 1 },
		"隱藏帶了嚴重度": func(in *model.DecideInput) { in.Severity = "LOW" },
	}
	for name, mutate := range bad {
		in := violation(model.ActionHide)
		mutate(&in)
		if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	for name, mutate := range map[string]func(*model.DecideInput){
		"沒有店家":   func(in *model.DecideInput) { in.StoreID = 0 },
		"沒有嚴重度":  func(in *model.DecideInput) { in.Severity = "" },
		"嚴重度不認得": func(in *model.DecideInput) { in.Severity = "SEVERE" },
	} {
		in := takedown
		mutate(&in)
		if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("下架 %s：%v", name, err)
		}
	}
}

func TestValidateNoViolationCarriesNothingElse(t *testing.T) {
	ok := model.DecideInput{CaseID: uuid.NewV7(), Version: 2, AdminID: 1, Decision: model.DecisionNoViolation}
	if _, err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.DecideInput){
		"類別": func(in *model.DecideInput) { in.Category = policy.Spam },
		"處置": func(in *model.DecideInput) { in.Action = model.ActionHide },
		"理由": func(in *model.DecideInput) { in.Text = "x" },
		"店家": func(in *model.DecideInput) { in.StoreID = 1 },
	} {
		in := ok
		mutate(&in)
		if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("不違規帶了%s：%v", name, err)
		}
	}
	in := ok
	in.Decision = ""
	if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("空的決定：%v", err)
	}
}
