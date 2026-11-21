package report

import (
	"context"
	"errors"
	"slices"
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

func TestValidateReporterErrorPointsAtTheOffendingField(t *testing.T) {
	for name, tt := range map[string]struct {
		buyer, store int64
		want         []string
	}{
		"沒有檢舉人":   {0, 0, []string{"ReporterBuyerID", "ReporterStoreID"}},
		"兩者都有":    {1, 3, []string{"ReporterBuyerID", "ReporterStoreID"}},
		"負的買家檢舉人": {-1, 0, []string{"ReporterBuyerID"}},
		"負的店家檢舉人": {0, -3, []string{"ReporterStoreID"}},
	} {
		in := input()
		in.ReporterBuyerID, in.ReporterStoreID = tt.buyer, tt.store
		_, err := Validate(context.Background(), in)
		var ae *apperr.Error
		if !errors.As(err, &ae) {
			t.Fatalf("%s：%v", name, err)
		}
		var got []string
		for _, v := range ae.FieldViolations() {
			got = append(got, v.Field)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s：欄位 %v，要 %v", name, got, tt.want)
		}
	}
}

func TestValidateRejectsMalformedInput(t *testing.T) {
	for name, mutate := range map[string]func(*model.SubmitInput){
		"沒有檢舉人":   func(in *model.SubmitInput) { in.ReporterBuyerID = 0 },
		"負的檢舉人":   func(in *model.SubmitInput) { in.ReporterBuyerID = -1 },
		"買家與店家同時": func(in *model.SubmitInput) { in.ReporterStoreID = 3 },
		"店家檢舉評價": func(in *model.SubmitInput) {
			in.ReporterBuyerID, in.ReporterStoreID, in.Target.Kind = 0, 3, model.TargetReview
		},
		"店家檢舉社群留言": func(in *model.SubmitInput) { in.ReporterBuyerID, in.ReporterStoreID = 0, 3 },
		"店家檢舉評價回覆": func(in *model.SubmitInput) {
			in.ReporterBuyerID, in.ReporterStoreID, in.Target.Kind = 0, 3, model.TargetReviewReply
		},
		"負的店家檢舉人": func(in *model.SubmitInput) { in.ReporterBuyerID, in.ReporterStoreID = 0, -3 },
		"對象形狀錯":   func(in *model.SubmitInput) { in.Target.BuyerID = 5 },
		"說明太長":    func(in *model.SubmitInput) { in.Description = strings.Repeat("字", model.MaxDescriptionRunes+1) },
		"沒有對象種類":  func(in *model.SubmitInput) { in.Target.Kind = "" },
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

func TestValidateAcceptsAStoreReporterOnlyOnChatMessages(t *testing.T) {
	in := input()
	in.ReporterBuyerID, in.ReporterStoreID, in.Target.Kind = 0, 3, model.TargetChatMessage
	if _, err := Validate(context.Background(), in); err != nil {
		t.Fatalf("店家檢舉聊天訊息：%v", err)
	}
}
