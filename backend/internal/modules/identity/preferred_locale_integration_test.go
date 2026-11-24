//go:build integration

package identity_test

import (
	"context"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func (f *fixture) updatedAt(t *testing.T, id int64) time.Time {
	t.Helper()
	var at time.Time
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT updated_at FROM identity.accounts WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// 新帳號的語系是預設 zh-TW；語系不同才寫（回 true），相同是 no-op（不動 updated_at）。
func TestPreferredLocaleDefaultsAndUpdatesOnlyWhenItDiffers(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if got, err := f.m.PreferredLocale(ctx, id); err != nil || got != uitext.ZhTW {
		t.Fatalf("預設 = %q %v", got, err)
	}
	created := f.updatedAt(t, id)
	f.now = f.now.Add(time.Hour)
	if changed, err := f.m.SetPreferredLocale(ctx, id, uitext.ZhTW); err != nil || changed {
		t.Fatalf("相同語系：changed=%v err=%v", changed, err)
	}
	if !f.updatedAt(t, id).Equal(created) {
		t.Fatal("相同語系不該動 updated_at")
	}
	if changed, err := f.m.SetPreferredLocale(ctx, id, uitext.EN); err != nil || !changed {
		t.Fatalf("不同語系：changed=%v err=%v", changed, err)
	}
	if got, err := f.m.PreferredLocale(ctx, id); err != nil || got != uitext.EN {
		t.Fatalf("更新後 = %q %v", got, err)
	}
	if changed, err := f.m.SetPreferredLocale(ctx, id, uitext.EN); err != nil || changed {
		t.Fatalf("再次相同語系：changed=%v err=%v", changed, err)
	}
}

func TestPreferredLocaleRejectsUnsupportedAndMissingAccounts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	for _, l := range []uitext.Locale{"", "fr", "zh-CN"} {
		if _, err := f.m.SetPreferredLocale(ctx, id, l); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%q：err = %v", l, err)
		}
	}
	if got, _ := f.m.PreferredLocale(ctx, id); got != uitext.ZhTW {
		t.Fatalf("被拒的語系不得寫入：%q", got)
	}
	_, err := f.m.PreferredLocale(ctx, id+1000)
	wantKind(t, err, apperr.KindNotFound)
	if changed, err := f.m.SetPreferredLocale(ctx, id+1000, uitext.EN); err != nil || changed {
		t.Fatalf("不存在的帳號：changed=%v err=%v", changed, err)
	}
}

// 已刪除的帳號不更新語系（墓碑保持刪除當時的值）。
func TestPreferredLocaleIsNotUpdatedOnATombstone(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if changed, err := f.m.SetPreferredLocale(ctx, id, uitext.EN); err != nil || changed {
		t.Fatalf("墓碑：changed=%v err=%v", changed, err)
	}
	if got, err := f.m.PreferredLocale(ctx, id); err != nil || got != uitext.ZhTW {
		t.Fatalf("墓碑的語系 = %q %v", got, err)
	}
}

// 登入帶語系：新帳號與既有帳號都記成請求的語系；沒有語系資訊（空字串）不動。
func TestLoginRecordsTheRequestLocale(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	login := func(l uitext.Locale) LoginResult {
		res, err := f.m.Login(ctx, LoginInput{Identity: googleAlice.identity(), Client: ClientApp, Locale: l})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := login(uitext.EN)
	if !res.Created {
		t.Fatal("預期建立新帳號")
	}
	locale := func() uitext.Locale {
		got, err := f.m.PreferredLocale(ctx, res.AccountID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if locale() != uitext.EN {
		t.Fatalf("新帳號登入語系 = %q", locale())
	}
	login("")
	if locale() != uitext.EN {
		t.Fatalf("沒有語系資訊不該改值：%q", locale())
	}
	login(uitext.ZhTW)
	if locale() != uitext.ZhTW {
		t.Fatalf("既有帳號登入語系 = %q", locale())
	}
	if _, err := f.m.Login(ctx, LoginInput{Identity: googleAlice.identity(), Client: ClientApp, Locale: "fr"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("不支援的語系：%v", err)
	}
	if locale() != uitext.ZhTW {
		t.Fatalf("被拒的登入不得改語系：%q", locale())
	}
}
