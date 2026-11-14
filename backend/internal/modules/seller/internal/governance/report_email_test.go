package governance

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func TestValidateSubmitAnonymousEmailRule(t *testing.T) {
	submit := func(email string) (*string, error) {
		params, err := validateSubmit(&model.SubmitReportInput{
			StoreID: 1, Category: policy.ReporterSelectable()[0], Description: "內容", Email: email,
		})
		return params.ReporterEmail, err
	}
	accepted := []struct{ in, want string }{
		{"a@b.co", "a@b.co"},
		{" Owner@Example.COM ", "owner@example.com"},
		{`"ab"@c.com`, "ab@c.com"},
		{"<a@b.co>", "a@b.co"},
		{"a+b@c.co", "a+b@c.co"},
		{"用戶@例子.台灣", "用戶@例子.台灣"},
	}
	for _, c := range accepted {
		got, err := submit(c.in)
		if err != nil || got == nil || *got != c.want {
			t.Errorf("email %q：存 %v err=%v，預期存 %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"user@localhost", "a@b", "Name <a@b.co>", "a@b.", "a@@b.co", strings.Repeat("a", 250) + "@b.co"} {
		_, err := submit(in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("email %q 應該回 invalid_input，得到 %v", in, err)
		}
	}
}
