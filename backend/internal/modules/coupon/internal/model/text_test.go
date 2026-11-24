package model

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// constsOf 讀 model.go 宣告的、型別為 typeName 的常數值；用原始碼而不是手寫清單，新增常數忘了加句子時測試會紅。
func constsOf(t *testing.T, typeName string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "model.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, sp := range gd.Specs {
			vs := sp.(*ast.ValueSpec)
			id, ok := vs.Type.(*ast.Ident)
			if !ok || id.Name != typeName {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok {
					t.Fatalf("%s 常數必須是字串字面值", typeName)
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, s)
			}
		}
	}
	return out
}

func checkBothLocales(t *testing.T, name string, f func(uitext.Locale) uitext.Message) {
	t.Helper()
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		m := f(l)
		if m.Text == "" {
			t.Errorf("%s：語系 %v 的句子是空的", name, l)
		}
		if m.Locale != l {
			t.Errorf("%s：語系 %v 缺字，回的是 %v", name, l, m.Locale)
		}
	}
}

func TestEveryReasonHasTextInBothLocales(t *testing.T) {
	reasons := constsOf(t, "Reason")
	if len(reasons) == 0 {
		t.Fatal("沒有讀到任何 Reason 常數")
	}
	for _, r := range reasons {
		f, ok := reasonText[Reason(r)]
		if !ok {
			t.Errorf("Reason %s 沒有句子對照", r)
			continue
		}
		checkBothLocales(t, r, f)
	}
	if len(reasonText) != len(reasons) {
		t.Errorf("reasonText 有 %d 筆，Reason 常數有 %d 個", len(reasonText), len(reasons))
	}
}

func TestEveryInvalidKeyHasTextInBothLocales(t *testing.T) {
	keys := constsOf(t, "InvalidKey")
	if len(keys) == 0 {
		t.Fatal("沒有讀到任何 InvalidKey 常數")
	}
	for _, k := range keys {
		f, ok := invalidText[InvalidKey(k)]
		if !ok {
			t.Errorf("InvalidKey %s 沒有句子對照", k)
			continue
		}
		checkBothLocales(t, k, f)
	}
	if len(invalidText) != len(keys) {
		t.Errorf("invalidText 有 %d 筆，InvalidKey 常數有 %d 個", len(invalidText), len(keys))
	}
}

// 模組的句子依 ctx 的請求語系產生，錯誤帶的語系是實際使用的語系（傳輸層的 LocalizedMessage.locale 取它，不寫死 zh-TW）。
func TestErrorsCarryTheLocaleTheSentenceWasProducedIn(t *testing.T) {
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		ctx := uitext.WithLocale(context.Background(), l)
		for name, err := range map[string]error{
			"precondition": Precondition(ctx, ReasonIssueLimitReached),
			"not found":    NotFound(ctx, ReasonCampaignNotFound),
			"invalid":      Invalid(ctx, "Title", InvalidTitleLength),
		} {
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Locale() != l {
				t.Errorf("%s（%s）：錯誤的語系 = %v", name, l, err)
			}
		}
	}
	zh := Precondition(uitext.WithLocale(context.Background(), uitext.ZhTW), ReasonIssueLimitReached).Error()
	en := Precondition(uitext.WithLocale(context.Background(), uitext.EN), ReasonIssueLimitReached).Error()
	if zh == en {
		t.Errorf("en 的請求應拿到不同於 zh-TW 的句子：%q", en)
	}
}
