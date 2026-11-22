package upload

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const validMD5 = "1B2M2Y8AsgTpgAmY7PhCfg==" // 空內容的 MD5

func TestValidate(t *testing.T) {
	ok := BeginInput{Visibility: model.VisibilityPublic, Size: 10, ContentType: "image/png", MD5: validMD5}
	if err := validate(ok); err != nil {
		t.Fatalf("合法的宣告被拒：%v", err)
	}
	cases := []struct {
		name string
		mut  func(*BeginInput)
	}{
		{"未知的可見性", func(in *BeginInput) { in.Visibility = "PRIVATE" }},
		{"零大小", func(in *BeginInput) { in.Size = 0 }},
		{"負大小", func(in *BeginInput) { in.Size = -1 }},
		{"空型別", func(in *BeginInput) { in.ContentType = "" }},
		{"型別帶換行", func(in *BeginInput) { in.ContentType = "image/png\r\nX: y" }},
		{"型別過長", func(in *BeginInput) { in.ContentType = strings.Repeat("a", maxContentTypeLen+1) }},
		{"MD5 不是 base64", func(in *BeginInput) { in.MD5 = "***" }},
		{"MD5 長度不對", func(in *BeginInput) { in.MD5 = base64.StdEncoding.EncodeToString([]byte("short")) }},
	}
	for _, c := range cases {
		in := ok
		c.mut(&in)
		if got := apperr.KindOf(validate(in)); got != apperr.KindInvalidInput {
			t.Errorf("%s：分類 %v，預期 invalid_input", c.name, got)
		}
	}
}

func TestTokenIsOpaqueRandomAndHashedToFixedWidth(t *testing.T) {
	a, b := newToken(), newToken()
	if a == b {
		t.Fatal("兩個 token 相同")
	}
	for _, tok := range []string{a, b} {
		if !wellFormed(tok) {
			t.Errorf("token %q 形狀不對", tok)
		}
	}
	if len(hashToken(a)) != 32 || string(hashToken(a)) == a {
		t.Error("雜湊應該是 32 位元組，而且不是明文")
	}
	for _, bad := range []string{"", "short", a + "x", strings.Repeat("=", len(a)), strings.Repeat("+", len(a))} {
		if wellFormed(bad) {
			t.Errorf("%q 不該被當成 token", bad)
		}
	}
}

func TestValidateRejectsSizeAboveModuleLimit(t *testing.T) {
	for _, v := range []model.Visibility{model.VisibilityPublic, model.VisibilitySecure} {
		limit := model.MaxUploadSize(v)
		in := BeginInput{Visibility: v, Size: limit, ContentType: "image/png", MD5: validMD5}
		if err := validate(in); err != nil {
			t.Fatalf("%s：剛好等於上限的宣告被拒：%v", v, err)
		}
		in.Size = limit + 1
		if got := apperr.KindOf(validate(in)); got != apperr.KindInvalidInput {
			t.Errorf("%s：超過上限的分類 %v，預期 invalid_input", v, got)
		}
	}
}
