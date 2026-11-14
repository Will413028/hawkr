package account

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
)

func ptr(s string) *string { return &s }

// 聯絡信箱的規則：通過 emailaddr.Normalize 的已驗證 email（回正規化值）優先；格式不合視為沒有，改用自填的；已刪除的帳號沒有。
// mutation：validEmail 改成只檢查非空 → 「已驗證格式不合」「已驗證要正規化」失敗；自填優先 → 「兩者都有效」失敗；contactEmail 不檢查 DeletedAt → 「已刪除」失敗。
func TestContactEmailRule(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		row  store.IdentityAccount
		want model.ContactEmail
		ok   bool
	}{
		"兩者都有效":       {store.IdentityAccount{Email: ptr("v@example.com"), SelfEnteredEmail: ptr("s@example.com")}, model.ContactEmail{Address: "v@example.com", Source: model.ContactEmailVerified}, true},
		"已驗證要正規化":     {store.IdentityAccount{Email: ptr(" Verified@Example.COM ")}, model.ContactEmail{Address: "verified@example.com", Source: model.ContactEmailVerified}, true},
		"已驗證格式不合，用自填": {store.IdentityAccount{Email: ptr("not-an-email"), SelfEnteredEmail: ptr("Self@Example.com")}, model.ContactEmail{Address: "self@example.com", Source: model.ContactEmailSelfEntered}, true},
		"已驗證格式不合又沒自填": {store.IdentityAccount{Email: ptr("user@localhost")}, model.ContactEmail{}, false},
		"只有自填":        {store.IdentityAccount{SelfEnteredEmail: ptr("s@example.com")}, model.ContactEmail{Address: "s@example.com", Source: model.ContactEmailSelfEntered}, true},
		"空字串":         {store.IdentityAccount{Email: ptr("")}, model.ContactEmail{}, false},
		"都沒有":         {store.IdentityAccount{}, model.ContactEmail{}, false},
		"已刪除":         {store.IdentityAccount{Email: ptr("v@example.com"), DeletedAt: &now}, model.ContactEmail{}, false},
	} {
		got, ok := contactEmail(tc.row)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s：%+v %v，預期 %+v %v", name, got, ok, tc.want, tc.ok)
		}
	}
}
