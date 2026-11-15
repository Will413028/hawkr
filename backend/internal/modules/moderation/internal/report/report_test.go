package report

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func input() model.SubmitInput {
	return model.SubmitInput{
		ReporterBuyerID: 1, Target: model.Target{Kind: model.TargetComment, ContentID: uuid.NewV7()}, Category: policy.Spam,
	}
}

func TestValidateAcceptsExactlyTheCommunityReporterCategories(t *testing.T) {
	for _, c := range policy.Values() {
		in := input()
		in.Category = c
		_, err := Validate(context.Background(), in)
		if (err == nil) != c.CommunityReporterSelectable() {
			t.Errorf("%s：err = %v，CommunityReporterSelectable = %v", c, err, c.CommunityReporterSelectable())
		}
	}
	in := input()
	in.Category = ""
	if _, err := Validate(context.Background(), in); err == nil {
		t.Error("空的類別不能通過")
	}
}

func TestValidateRejectsMalformedInput(t *testing.T) {
	for name, mutate := range map[string]func(*model.SubmitInput){
		"沒有檢舉人":  func(in *model.SubmitInput) { in.ReporterBuyerID = 0 },
		"負的檢舉人":  func(in *model.SubmitInput) { in.ReporterBuyerID = -1 },
		"對象形狀錯":  func(in *model.SubmitInput) { in.Target.BuyerID = 5 },
		"說明太長":   func(in *model.SubmitInput) { in.Description = strings.Repeat("字", model.MaxDescriptionRunes+1) },
		"沒有對象種類": func(in *model.SubmitInput) { in.Target.Kind = "" },
	} {
		in := input()
		mutate(&in)
		if _, err := Validate(context.Background(), in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}

func TestValidateTrimsAndBoundsTheDescriptionByRunes(t *testing.T) {
	in := input()
	in.Description = "　 內容 \t"
	out, err := Validate(context.Background(), in)
	if err != nil || out.Description != "內容" {
		t.Fatalf("去頭尾空白：%q, %v", out.Description, err)
	}
	in.Description = strings.Repeat("字", model.MaxDescriptionRunes)
	if _, err := Validate(context.Background(), in); err != nil {
		t.Fatalf("剛好上限要通過（以字數計，不是 bytes）：%v", err)
	}
}
