package write

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/review/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/textfilter"
)


type orders struct {
	order model.OrderForReview
	err   error
	calls int
}

func (o *orders) OrderForReview(context.Context, string) (model.OrderForReview, error) {
	o.calls++
	return o.order, o.err
}

var now = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func service(o *orders) *Service {
	return New(Deps{Orders: o, Now: func() time.Time { return now }})
}

func valid() model.SubmitInput {
	return model.SubmitInput{BuyerID: 11, OrderNo: "N1", OrderItemID: 5, Rating: 5, Content: "很好喝"}
}

// RD2：長度以字元（rune）計。1000 個中文字（3000 位元組）要能通過驗證走到訂單查詢，1001 個要在查訂單之前被擋。
func TestSubmitCountsRunesNotBytes(t *testing.T) {
	o := &orders{err: model.ErrOrderNotFound}
	in := valid()
	in.Content = strings.Repeat("字", model.MaxContentRunes)
	_, err := service(o).Submit(context.Background(), in)
	if o.calls != 1 || model.ReasonOf(err) != model.ReasonOrderNotFound {
		t.Fatalf("1000 個字要通過驗證並走到訂單查詢：calls=%d err=%v", o.calls, err)
	}

	o = &orders{err: model.ErrOrderNotFound}
	in.Content = strings.Repeat("字", model.MaxContentRunes+1)
	_, err = service(o).Submit(context.Background(), in)
	if o.calls != 0 || apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("1001 個字要在查訂單之前被擋：calls=%d err=%v", o.calls, err)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "Content" {
		t.Fatalf("要指到 Content：%v", err)
	}
}

// 內容先去頭尾空白再計長度：頭尾空白不佔額度。
func TestSubmitTrimsBeforeCounting(t *testing.T) {
	o := &orders{err: model.ErrOrderNotFound}
	in := valid()
	in.Content = "  " + strings.Repeat("字", model.MaxContentRunes) + "\n"
	if _, err := service(o).Submit(context.Background(), in); o.calls != 1 {
		t.Fatalf("去掉頭尾空白後剛好 1000 個字：calls=%d err=%v", o.calls, err)
	}
}

// RD9：只有訂單明確不存在是 NotFound；暫時性故障（任何其他錯誤）是 Unavailable，不得誤報成訂單不存在。
func TestSubmitSeparatesOrderNotFoundFromUnavailable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		kind apperr.Kind
	}{
		{"not found", model.ErrOrderNotFound, apperr.KindNotFound},
		{"wrapped not found", fmt.Errorf("包住：%w", model.ErrOrderNotFound), apperr.KindNotFound},
		{"transient", errors.New("connection refused"), apperr.KindUnavailable},
		{"deadline", context.DeadlineExceeded, apperr.KindUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service(&orders{err: tc.err}).Submit(context.Background(), valid())
			if apperr.KindOf(err) != tc.kind {
				t.Fatalf("kind = %v, want %v（err = %v）", apperr.KindOf(err), tc.kind, err)
			}
			if tc.kind == apperr.KindNotFound && model.ReasonOf(err) != model.ReasonOrderNotFound {
				t.Fatalf("reason = %q", model.ReasonOf(err))
			}
			if tc.kind == apperr.KindUnavailable && model.ReasonOf(err) != "" {
				t.Fatalf("暫時性故障不帶 ORDER_NOT_FOUND：%q", model.ReasonOf(err))
			}
		})
	}
}

// A6-D10：擁有者先於狀態檢查，別人的訂單（不論狀態）與不存在的訂單回應一致。
func TestSubmitForeignOrderIsIndistinguishableFromAbsent(t *testing.T) {
	completed := now.Add(-time.Hour)
	order := model.OrderForReview{OrderNo: "N1", BuyerID: 99, StoreID: 22, Status: "50", CompletedAt: completed, Items: []model.OrderItem{{OrderItemID: 5, ProductID: uuid.UUID{15: 7}, SkuName: "紅茶", Price: 100}}}

	_, absent := service(&orders{err: model.ErrOrderNotFound}).Submit(context.Background(), valid())

	notDone := order
	notDone.Status = "40"
	_, err := service(&orders{order: notDone}).Submit(context.Background(), valid())
	if apperr.KindOf(err) != apperr.KindNotFound || model.ReasonOf(err) != model.ReasonOrderNotFound {
		t.Fatalf("別人且未完成的訂單要是 NotFound：%v", err)
	}

	_, err = service(&orders{order: order}).Submit(context.Background(), valid())
	if apperr.KindOf(err) != apperr.KindNotFound || model.ReasonOf(err) != model.ReasonOrderNotFound {
		t.Fatalf("別人且已完成的訂單要是 NotFound：%v", err)
	}
	if err.Error() != absent.Error() || apperr.KindOf(err) != apperr.KindOf(absent) || model.ReasonOf(err) != model.ReasonOf(absent) {
		t.Fatalf("要與不存在的訂單一致：%v vs %v", err, absent)
	}

	own := order
	own.BuyerID = 11
	own.Status = "40"
	_, err = service(&orders{order: own}).Submit(context.Background(), valid())
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || model.ReasonOf(err) != model.ReasonOrderNotReviewable {
		t.Fatalf("自己的未完成訂單：%v", err)
	}
}

func TestSubmitValidatesInputBeforeAnyCall(t *testing.T) {
	dirty := ""
	if len(textfilter.Words) > 0 {
		dirty = textfilter.Words[0]
	}
	cases := []struct {
		name   string
		mutate func(*model.SubmitInput)
		field  string
	}{
		{"rating 0", func(i *model.SubmitInput) { i.Rating = 0 }, "Rating"},
		{"rating 6", func(i *model.SubmitInput) { i.Rating = 6 }, "Rating"},
		{"five tokens", func(i *model.SubmitInput) { i.StagedTokens = []string{"a", "b", "c", "d", "e"} }, "StagedTokens"},
		{"no order", func(i *model.SubmitInput) { i.OrderNo = "" }, "OrderNo"},
		{"bad item", func(i *model.SubmitInput) { i.OrderItemID = 0 }, "OrderItemID"},
		{"invalid utf8", func(i *model.SubmitInput) { i.Content = "\xff" }, "Content"},
		{"dictionary", func(i *model.SubmitInput) { i.Content = dirty }, "Content"},
	}
	if dirty == "" {
		t.Fatal("詞典是空的，無法測詞典命中")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &orders{}
			in := valid()
			tc.mutate(&in)
			_, err := service(o).Submit(context.Background(), in)
			var ae *apperr.Error
			if o.calls != 0 || !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != tc.field {
				t.Fatalf("calls=%d err=%v", o.calls, err)
			}
			if tc.name == "dictionary" && model.ReasonOf(err) != model.ReasonModerationRejected {
				t.Fatalf("reason = %q", model.ReasonOf(err))
			}
		})
	}
}

func TestHideRejectsCategoriesThatDoNotApplyToBuyerContent(t *testing.T) {
	for _, c := range []string{"FOOD_SAFETY", "COUNTERFEIT", "OTHER", "", "spam"} {
		_, err := service(&orders{}).Hide(context.Background(), model.HideInput{Category: c, AdminID: 1})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("category %q 要被擋：%v", c, err)
		}
	}
	if _, err := service(&orders{}).Hide(context.Background(), model.HideInput{Category: "SPAM"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("沒有管理員要被擋：%v", err)
	}
}

func TestSubmitRejectsBlankAndDuplicateStagedTokensBeforeAnyCall(t *testing.T) {
	cases := []struct {
		name   string
		tokens []string
		field  string
	}{
		{"blank", []string{"a", "  \t"}, "StagedTokens[1]"},
		{"empty", []string{""}, "StagedTokens[0]"},
		{"duplicate", []string{"a", "b", "a"}, "StagedTokens[2]"},
		{"duplicate after trim", []string{"a", " a "}, "StagedTokens[1]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &orders{}
			in := valid()
			in.StagedTokens = tc.tokens
			_, err := service(o).Submit(context.Background(), in)
			var ae *apperr.Error
			if o.calls != 0 || apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != tc.field {
				t.Fatalf("calls=%d err=%v", o.calls, err)
			}
		})
	}
}

func TestHideAndRestoreRejectInvalidUTF8(t *testing.T) {
	bad := "a\xffb"
	cases := map[string]func() error{
		"hide text": func() error {
			_, err := service(&orders{}).Hide(context.Background(), model.HideInput{Category: "SPAM", AdminID: 1, Text: bad})
			return err
		},
		"hide source ref": func() error {
			_, err := service(&orders{}).Hide(context.Background(), model.HideInput{Category: "SPAM", AdminID: 1, SourceRef: bad})
			return err
		},
		"restore reason": func() error {
			_, err := service(&orders{}).Restore(context.Background(), model.RestoreInput{AdminID: 1, Reason: bad})
			return err
		},
	}
	for name, f := range cases {
		if err := f(); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s 要回 InvalidInput：%v", name, err)
		}
	}
}
