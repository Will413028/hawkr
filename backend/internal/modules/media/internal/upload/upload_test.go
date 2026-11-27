package upload

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var ctx0 = context.Background()

func validateErr(in BeginInput) error {
	_, err := validate(ctx0, in)
	return err
}

const validMD5 = "1B2M2Y8AsgTpgAmY7PhCfg=="

func TestValidate(t *testing.T) {
	ok := BeginInput{Size: 10, ContentType: "image/png", MD5: validMD5}
	if _, err := validate(ctx0, ok); err != nil {
		t.Fatalf("合法的宣告被拒：%v", err)
	}
	cases := []struct {
		name string
		mut  func(*BeginInput)
	}{
		{"零大小", func(in *BeginInput) { in.Size = 0 }},
		{"負大小", func(in *BeginInput) { in.Size = -1 }},
		{"空型別", func(in *BeginInput) { in.ContentType = "" }},
		{"型別帶換行", func(in *BeginInput) { in.ContentType = "image/png\r\nX: y" }},
		{"MD5 不是 base64", func(in *BeginInput) { in.MD5 = "***" }},
		{"MD5 長度不對", func(in *BeginInput) { in.MD5 = base64.StdEncoding.EncodeToString([]byte("short")) }},
	}
	for _, c := range cases {
		in := ok
		c.mut(&in)
		if got := apperr.KindOf(validateErr(in)); got != apperr.KindInvalidInput {
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
	limit := model.MaxPublicUploadSize
	in := BeginInput{Size: limit, ContentType: "image/png", MD5: validMD5}
	if _, err := validate(ctx0, in); err != nil {
		t.Fatalf("剛好等於上限的宣告被拒：%v", err)
	}
	in.Size = limit + 1
	if got := apperr.KindOf(validateErr(in)); got != apperr.KindInvalidInput {
		t.Errorf("超過上限的分類 %v，預期 invalid_input", got)
	}
}

func TestCreateUpload_AcceptsAllowedImage(t *testing.T) {
	for _, ct := range []string{"image/jpeg", "image/png", "image/webp", "image/gif"} {
		if _, err := validate(ctx0, BeginInput{Size: 10, ContentType: ct, MD5: validMD5}); err != nil {
			t.Errorf("%s 被拒：%v", ct, err)
		}
	}
}

func TestCreateUpload_RejectsUnknownMime(t *testing.T) {
	for _, ct := range []string{"image/heic", "application/pdf", "text/html", "image/svg+xml", "image/png;;"} {
		_, err := validate(ctx0, BeginInput{Size: 10, ContentType: ct, MD5: validMD5})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v，預期 invalid_input", ct, err)
		}
	}
}

func TestCreateUpload_NormalizesContentType(t *testing.T) {
	for in, want := range map[string]string{
		"IMAGE/PNG":                  "image/png",
		"image/jpeg; charset=binary": "image/jpeg",
		"image/jpg":                  "image/jpeg",
		" image/webp":                "image/webp",
	} {
		got, err := validate(ctx0, BeginInput{Size: 10, ContentType: in, MD5: validMD5})
		if err != nil {
			t.Errorf("%q 被拒：%v", in, err)
			continue
		}
		if got.ContentType != want {
			t.Errorf("%q 正規化成 %q，預期 %q", in, got.ContentType, want)
		}
	}
}

func TestValidate_MessageFollowsLocale(t *testing.T) {
	en := uitext.WithLocale(ctx0, uitext.EN)
	cases := []struct {
		name string
		in   BeginInput
		want string
		zh   string
	}{
		{"over limit", BeginInput{Size: model.MaxPublicUploadSize + 1, ContentType: "image/png", MD5: validMD5}, "The upload exceeds the limit of", "上傳的大小超過上限"},
		{"bad type", BeginInput{Size: 10, ContentType: "image/heic", MD5: validMD5}, "Unsupported image type", "不支援的圖片型別"},
	}
	for _, c := range cases {
		_, err := validate(en, c.in)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s：en 訊息 %v，預期含 %q", c.name, err, c.want)
		}
		_, err = validate(uitext.WithLocale(ctx0, uitext.ZhTW), c.in)
		if err == nil || !strings.Contains(err.Error(), c.zh) {
			t.Errorf("%s：zh-TW 訊息 %v，預期含 %q", c.name, err, c.zh)
		}
	}
}
