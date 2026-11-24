package reach

import (
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/deliver"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var clock = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

func newService() *Service {
	return New(Deps{Registry: registry.Registry{}, Now: func() time.Time { return clock }})
}

func validInput() ResendInput {
	return ResendInput{FailStart: clock.Add(-24 * time.Hour), FailEnd: clock.Add(-time.Hour)}
}

func TestResendFilterRejectsBadInput(t *testing.T) {
	for name, mutate := range map[string]func(*ResendInput){
		"沒有起點":    func(in *ResendInput) { in.FailStart = time.Time{} },
		"沒有終點":    func(in *ResendInput) { in.FailEnd = time.Time{} },
		"起點不早於終點": func(in *ResendInput) { in.FailStart = in.FailEnd },
		"終點在未來":   func(in *ResendInput) { in.FailEnd = clock.Add(time.Second) },
		"窗太長":     func(in *ResendInput) { in.FailStart = in.FailEnd.Add(-MaxResendWindow - time.Second) },
		"筆數為負":    func(in *ResendInput) { in.MaxCount = -1 },
		"帳號為負":    func(in *ResendInput) { in.AccountID = -1 },
		"渠道是收件匣":  func(in *ResendInput) { in.Channel = model.ChannelInbox },
		"不可重送的原因": func(in *ResendInput) { in.Reasons = []string{deliver.ReasonNoLongerValid} },
		"停寄名單":    func(in *ResendInput) { in.Reasons = []string{deliver.ReasonSuppressed} },
		"收件人已刪除":  func(in *ResendInput) { in.Reasons = []string{deliver.ReasonRecipientGone} },
		"不認得的分類":  func(in *ResendInput) { in.Category = "NOPE" },
	} {
		in := validInput()
		mutate(&in)
		if _, _, err := newService().resendFilter(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：想要 invalid_input，得到 %v", name, err)
		}
	}
}

func TestResendFilterDefaultsAndBounds(t *testing.T) {
	f, n, err := newService().resendFilter(validInput())
	if err != nil || n != DefaultMaxCount {
		t.Fatalf("預設批次：%d %v", n, err)
	}
	if !slices.Equal(f.reasons, resendable) || f.channel != nil || f.accountID != nil || f.types != nil {
		t.Fatalf("預設條件不該限縮：%+v", f)
	}
	in := validInput()
	in.MaxCount = 100000
	if _, n, _ := newService().resendFilter(in); n != MaxMaxCount {
		t.Fatalf("批次上限：%d", n)
	}
}

// 不可重送的原因永遠不在白名單裡，新增的原因預設不可重送。
func TestNeverResendableReasons(t *testing.T) {
	for _, r := range []string{
		deliver.ReasonNoLongerValid, deliver.ReasonSuppressed, deliver.ReasonRecipientGone, deliver.ReasonIntentMissing,
		deliver.ReasonTypeUnregistered, deliver.ReasonRenderFailed, deliver.ReasonChannelUnsupported,
	} {
		if slices.Contains(Resendable(), r) {
			t.Errorf("%q 不可重送", r)
		}
	}
	for _, r := range Resendable() {
		if !slices.Contains(deliver.Reasons(), r) {
			t.Errorf("%q 不是已知的失敗原因", r)
		}
	}
}

// 交易必要的型別由註冊表的分類推導：全是交易類，行銷、互動與公告都不在內。
func TestTransactionalTypesComeFromTheRegistry(t *testing.T) {
	s := newService()
	got := s.TransactionalTypes()
	if len(got) == 0 {
		t.Fatal("註冊表沒有交易必要的型別")
	}
	for _, e := range registry.All() {
		sp := e.Spec()
		class, _ := model.ClassOf(sp.Category)
		if (class == model.ClassTransactional) != slices.Contains(got, sp.Name) {
			t.Errorf("型別 %s（分類 %s）的歸類與註冊表不一致", sp.Name, sp.Category)
		}
	}
}

func TestResendCategoryFilterSelectsThatCategoryTypes(t *testing.T) {
	in := validInput()
	in.Category = model.CategoryAccount
	f, _, err := newService().resendFilter(in)
	if err != nil || f.types == nil {
		t.Fatalf("%+v %v", f, err)
	}
	for _, name := range f.types {
		e, ok := registry.Lookup(name)
		if !ok || e.Spec().Category != model.CategoryAccount {
			t.Errorf("%s 不在 ACCOUNT 分類", name)
		}
	}
}

func TestUnreachedRejectsNegativeInput(t *testing.T) {
	if _, err := newService().Unreached(t.Context(), UnreachedInput{Days: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("天數為負：%v", err)
	}
}
