package model

import (
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func TestActionAllowedOnMatchesTheDocumentedMatrix(t *testing.T) {
	want := map[Action]map[TargetKind]bool{
		ActionHide:     {TargetComment: true, TargetForumEntry: true, TargetReview: true, TargetChatMessage: true},
		ActionWarn:     {TargetComment: true, TargetForumEntry: true, TargetBuyer: true, TargetReview: true, TargetChatMessage: true},
		ActionBan:      {TargetComment: true, TargetForumEntry: true, TargetBuyer: true, TargetReview: true, TargetChatMessage: true},
		ActionTakedown: {TargetPost: true, TargetComment: true, TargetForumEntry: true, TargetChatMessage: true, TargetReviewReply: true},
	}
	for _, a := range Actions() {
		for _, k := range TargetKinds() {
			if got := a.AllowedOn(k); got != want[a][k] {
				t.Errorf("%s 對 %s：%v，預期 %v", a, k, got, want[a][k])
			}
		}
	}
	if Action("").AllowedOn(TargetComment) || Action("DISMISS").AllowedOn(TargetComment) {
		t.Error("不認得的處置種類不能用在任何對象")
	}
}

func TestPostsAreNeverBuyerSanctionTargets(t *testing.T) {
	for _, a := range []Action{ActionHide, ActionWarn, ActionBan} {
		if a.AllowedOn(TargetPost) {
			t.Errorf("貼文的作者是店家，%s 不能用在貼文上", a)
		}
	}
}

func TestCategoryAllowedFollowsTheBranch(t *testing.T) {
	for _, c := range policy.Values() {
		if got, want := ActionTakedown.CategoryAllowed(c), c.Assessable(); got != want {
			t.Errorf("下架 %s：%v，預期 %v", c, got, want)
		}
		for _, a := range []Action{ActionHide, ActionWarn, ActionBan} {
			if got, want := a.CategoryAllowed(c), c.CommunityAssessable(); got != want {
				t.Errorf("%s %s：%v，預期 %v", a, c, got, want)
			}
		}
	}
	if ActionTakedown.CategoryAllowed(policy.Other) || ActionHide.CategoryAllowed(policy.Other) || ActionHide.CategoryAllowed("") {
		t.Error("OTHER 與空值不能當認定類別")
	}
	if !ActionHide.CategoryAllowed(policy.SexualContent) || !ActionTakedown.CategoryAllowed(policy.SexualContent) {
		t.Error("SEXUAL_CONTENT 是社群類別，兩個分支都能認定")
	}
}

func TestTargetShape(t *testing.T) {
	id := uuid.NewV7()
	for name, c := range map[string]struct {
		t     Target
		valid bool
	}{
		"留言":       {Target{Kind: TargetComment, ContentID: id}, true},
		"貼文":       {Target{Kind: TargetPost, ContentID: id}, true},
		"買家":       {Target{Kind: TargetBuyer, BuyerID: 42}, true},
		"留言沒有 ID":  {Target{Kind: TargetComment}, false},
		"留言帶買家":    {Target{Kind: TargetComment, ContentID: id, BuyerID: 1}, false},
		"買家帶內容 ID": {Target{Kind: TargetBuyer, BuyerID: 1, ContentID: id}, false},
		"買家 ID 為零": {Target{Kind: TargetBuyer}, false},
		"不認得的種類":   {Target{Kind: "CHAT", ContentID: id}, false},
	} {
		if got := c.t.Validate(); got != c.valid {
			t.Errorf("%s：Validate = %v", name, got)
		}
		if c.valid && (c.t.ContentIDPtr() == nil) == (c.t.BuyerIDPtr() == nil) {
			t.Errorf("%s：內容 ID 與買家 ID 要恰好一個有值", name)
		}
	}
}

func TestOutcomeIsDerivedFromTheCaseDecision(t *testing.T) {
	v, n := string(DecisionViolation), string(DecisionNoViolation)
	for name, c := range map[string]struct {
		decision *string
		want     Outcome
	}{"進行中": {nil, OutcomePending}, "違規": {&v, OutcomeViolation}, "不違規": {&n, OutcomeNoViolation}} {
		if got := OutcomeOf(c.decision); got != c.want {
			t.Errorf("%s：%s，預期 %s", name, got, c.want)
		}
	}
}
