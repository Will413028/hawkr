package login

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestAutoLinkEligible(t *testing.T) {
	t.Parallel()
	google := func(email string, verified bool, hd string) model.ProviderIdentity {
		return model.ProviderIdentity{Provider: model.ProviderGoogle, Subject: "s", Email: email, EmailVerified: verified, HostedDomain: hd}
	}
	cases := map[string]struct {
		id   model.ProviderIdentity
		want bool
	}{
		"gmail 已驗證":           {google("a@gmail.com", true, ""), true},
		"gmail 網域大小寫不分":       {google("a@GMail.com", true, ""), true},
		"Workspace 已驗證（有 hd）": {google("a@corp.example", true, "corp.example"), true},
		"hd 與 email 網域大小寫不同":  {google("a@Corp.Example", true, "corp.example"), true},
		"hd 與 email 網域不同":     {google("a@other.example", true, "corp.example"), false},
		"gmail 網域但帶了別的 hd":    {google("a@gmail.com", true, "corp.example"), false},
		"gmail 未驗證":           {google("a@gmail.com", false, ""), false},
		"非 gmail 且沒有 hd":      {google("a@corp.example", true, ""), false},
		"網域只是以 gmail.com 結尾":  {google("a@notgmail.com", true, ""), false},
		"沒有 email":            {google("", true, "corp.example"), false},
		"email 沒有網域":          {google("agmail.com", true, ""), false},
		"LINE 即使 email 標為已驗證": {model.ProviderIdentity{Provider: model.ProviderLine, Subject: "s", Email: "a@gmail.com", EmailVerified: true}, false},
		"LINE 帶 hd 也不行":       {model.ProviderIdentity{Provider: model.ProviderLine, Subject: "s", Email: "a@x.example", EmailVerified: true, HostedDomain: "x.example"}, false},
	}
	for name, c := range cases {
		if got := AutoLinkEligible(c.id); got != c.want {
			t.Errorf("%s：AutoLinkEligible = %v，預期 %v", name, got, c.want)
		}
	}
}

func TestNormalizeRejectsUnknownValuesAndDropsVerifiedWithoutEmail(t *testing.T) {
	t.Parallel()
	for name, id := range map[string]model.ProviderIdentity{
		"空的 provider":  {Subject: "s"},
		"未知的 provider": {Provider: "FACEBOOK", Subject: "s"},
		"UNKNOWN":      {Provider: "UNKNOWN", Subject: "s"},
		"沒有 subject":   {Provider: model.ProviderGoogle},
		"小寫的 provider": {Provider: "google", Subject: "s"},
	} {
		_, err := normalize(id)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 InvalidInput", name, err)
		}
	}
	got, err := normalize(model.ProviderIdentity{Provider: model.ProviderGoogle, Subject: "s", Email: "  ", EmailVerified: true})
	if err != nil || got.Email != "" || got.EmailVerified {
		t.Fatalf("沒有 email 時不能是已驗證：%+v, %v", got, err)
	}
	if verifiedEmail(got) != nil {
		t.Fatal("沒有 email 不能產生寄件地址")
	}
	got, err = normalize(model.ProviderIdentity{Provider: model.ProviderGoogle, Subject: "s", Email: " a@gmail.com ", EmailVerified: true})
	if err != nil || got.Email != "a@gmail.com" {
		t.Fatalf("email 要去前後空白：%+v, %v", got, err)
	}
	if e := verifiedEmail(model.ProviderIdentity{Email: "a@x.example"}); e != nil {
		t.Fatal("未驗證的 email 不能當寄件地址")
	}
}
