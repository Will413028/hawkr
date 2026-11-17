package handle

import (
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func violation(a model.Action) model.DecideInput {
	return model.DecideInput{
		CaseID: uuid.NewV7(), Version: 1, Actor: adminaudit.Actor{AdminID: 3, Action: "/test/Decide"}, Decision: model.DecisionViolation, Category: policy.Spam, Action: a, Text: "理由",
	}
}

func TestValidateViolationShapes(t *testing.T) {
	takedown := violation(model.ActionTakedown)
	takedown.Severity = string(seller.SeverityValues()[0])
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
		"沒有處置":    func(in *model.DecideInput) { in.Action = "" },
		"處置不認得":   func(in *model.DecideInput) { in.Action = "DISMISS" },
		"沒有類別":    func(in *model.DecideInput) { in.Category = "" },
		"OTHER":   func(in *model.DecideInput) { in.Category = policy.Other },
		"商品類別":    func(in *model.DecideInput) { in.Category = policy.Counterfeit },
		"理由空白":    func(in *model.DecideInput) { in.Text = " 　 " },
		"理由太長":    func(in *model.DecideInput) { in.Text = strings.Repeat("字", model.MaxTextRunes+1) },
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

// 缺管理員或缺 procedure 名稱是 handler 的接線錯誤，不是使用者能修的輸入：回沒有分類的錯誤。
func TestValidateWiringErrorsAreUnclassified(t *testing.T) {
	base := model.DecideInput{CaseID: uuid.NewV7(), Version: 1, Actor: adminaudit.Actor{AdminID: 1, Action: "/test/Decide"}, Decision: model.DecisionNoViolation}
	for name, mutate := range map[string]func(*model.DecideInput){
		"沒有管理員":        func(in *model.DecideInput) { in.Actor.AdminID = 0 },
		"沒有 procedure": func(in *model.DecideInput) { in.Actor.Action = "" },
	} {
		in := base
		mutate(&in)
		_, err := Validate(in)
		if err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}

func TestValidateNoViolationCarriesNothingElse(t *testing.T) {
	ok := model.DecideInput{CaseID: uuid.NewV7(), Version: 2, Actor: adminaudit.Actor{AdminID: 1, Action: "/test/Decide"}, Decision: model.DecisionNoViolation}
	if _, err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.DecideInput){
		"類別": func(in *model.DecideInput) { in.Category = policy.Spam },
		"處置": func(in *model.DecideInput) { in.Action = model.ActionHide },
		"理由": func(in *model.DecideInput) { in.Text = "x" },
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

func TestValidateMissingActionIsAWiringErrorNotUserInput(t *testing.T) {
	in := violation(model.ActionHide)
	in.Actor.Action = ""
	if _, err := Validate(in); err == nil || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("沒有 procedure 名稱預期未分類的內部錯誤：%v", err)
	}
}

func TestCheckTakedownStoreUsesTheSnapshotStore(t *testing.T) {
	owner := func(author, store int64) social.ContentOwner {
		return social.ContentOwner{Author: social.Actor{Kind: social.ActorStore, ID: author}, StoreID: store}
	}
	if err := CheckTakedownStore(owner(5, 5), 5); err != nil {
		t.Fatalf("作者與歸屬都是快照店家：%v", err)
	}
	for name, c := range map[string]struct {
		o social.ContentOwner
		s int64
	}{
		"現在的作者不是快照店家": {owner(6, 6), 5},
		"歸屬不是快照店家":    {owner(5, 6), 5},
		"快照沒有店家":      {owner(5, 5), 0},
		"兩邊都沒有店家":     {owner(0, 0), 0},
		"作者是買家":       {social.ContentOwner{Author: social.Actor{Kind: social.ActorBuyer, ID: 5}, StoreID: 5}, 5},
	} {
		if err := CheckTakedownStore(c.o, c.s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：%v", name, err)
		}
	}
}
