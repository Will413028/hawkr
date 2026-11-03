//go:build integration

package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (f *fixture) selfEntered(t *testing.T, accountID int64) *string {
	t.Helper()
	var v *string
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT self_entered_email FROM identity.accounts WHERE id = $1`, accountID).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// 聯絡信箱：已驗證的 email 優先，沒有才用自填的，都沒有時 ok 是 false；帳號不存在回 NotFound。
// mutation：contactEmail 改成自填優先 → 第一段失敗；只回已驗證的 → 第二段失敗；不正規化已驗證的值 → 第一段失敗。
func TestContactEmailPrefersTheVerifiedEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.emails["both"] = "Verified@Example.com"
	both := f.account(t, "both")
	if _, err := f.db.Pool().Exec(ctx, `UPDATE identity.accounts SET self_entered_email = 'self@example.com' WHERE id = $1`, both); err != nil {
		t.Fatal(err)
	}
	got, ok, err := f.m.ContactEmail(ctx, both)
	if err != nil || !ok || got != (ContactEmail{Address: "verified@example.com", Source: ContactEmailVerified}) {
		t.Fatalf("兩者都有：%+v %v %v", got, ok, err)
	}
	onlySelf := f.account(t, "only_self")
	if _, err := f.db.Pool().Exec(ctx, `UPDATE identity.accounts SET self_entered_email = 'self@example.com' WHERE id = $1`, onlySelf); err != nil {
		t.Fatal(err)
	}
	got, ok, err = f.m.ContactEmail(ctx, onlySelf)
	if err != nil || !ok || got != (ContactEmail{Address: "self@example.com", Source: ContactEmailSelfEntered}) {
		t.Fatalf("只有自填：%+v %v %v", got, ok, err)
	}
	if got, ok, err = f.m.ContactEmail(ctx, f.account(t, "neither")); err != nil || ok || got != (ContactEmail{}) {
		t.Fatalf("都沒有：%+v %v %v", got, ok, err)
	}
	_, _, err = f.m.ContactEmail(ctx, 999999)
	wantKind(t, err, apperr.KindNotFound)
}

// 設定自填信箱：正規化後存；只在沒有可用的已驗證 email 時可設；格式不合回 InvalidInput 指向 Email；不存在或已刪除回 NotFound；不寫進 account_identities。
// mutation：拿掉「有已驗證 email 就拒絕」→ 第二段失敗；SetSelfEnteredEmail 不正規化 → 第一段失敗；同時把信箱寫進 account_identities → 最後一段失敗。
func TestSetSelfEnteredEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "no_email")
	got, err := f.m.SetSelfEnteredEmail(ctx, id, "  Me@Example.COM ")
	if err != nil || got != (ContactEmail{Address: "me@example.com", Source: ContactEmailSelfEntered}) {
		t.Fatalf("設定：%+v %v", got, err)
	}
	if v := f.selfEntered(t, id); v == nil || *v != "me@example.com" {
		t.Fatalf("存的值=%v", v)
	}
	if again, err := f.m.SetSelfEnteredEmail(ctx, id, "me@example.com"); err != nil || again != got {
		t.Fatalf("以同一個值重送：%+v %v", again, err)
	}
	if next, err := f.m.SetSelfEnteredEmail(ctx, id, "other@example.com"); err != nil || next.Address != "other@example.com" {
		t.Fatalf("改成另一個：%+v %v", next, err)
	}

	f.emails["verified"] = "v@example.com"
	verified := f.account(t, "verified")
	_, err = f.m.SetSelfEnteredEmail(ctx, verified, "me@example.com")
	wantKind(t, err, apperr.KindFailedPrecondition)
	if v := f.selfEntered(t, verified); v != nil {
		t.Fatalf("被拒絕之後不該寫入：%v", *v)
	}

	for _, bad := range []string{"", "   ", "no-at.example.com", "user@localhost", "Name <a@b.co>", strings.Repeat("a", 250) + "@b.co"} {
		_, err := f.m.SetSelfEnteredEmail(ctx, id, bad)
		wantKind(t, err, apperr.KindInvalidInput)
		var ae *apperr.Error
		if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "Email" {
			t.Fatalf("%q：欄位違規=%v", bad, err)
		}
	}
	if v := f.selfEntered(t, id); v == nil || *v != "other@example.com" {
		t.Fatalf("無效輸入之後值被改了：%v", v)
	}

	_, err = f.m.SetSelfEnteredEmail(ctx, 999999, "me@example.com")
	wantKind(t, err, apperr.KindNotFound)
	gone := f.account(t, "gone")
	if err := f.m.DeleteAccount(ctx, gone); err != nil {
		t.Fatal(err)
	}
	_, err = f.m.SetSelfEnteredEmail(ctx, gone, "me@example.com")
	wantKind(t, err, apperr.KindNotFound)

	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE email IS NOT NULL AND account_id = $1`, id); n != 0 {
		t.Fatalf("自填信箱不該寫進 account_identities：%d 列", n)
	}
}

// 刪除帳號清掉自填信箱（accounts_deleted_check 也要求），墓碑沒有聯絡信箱。
// mutation：DeleteAccount 不清 self_entered_email → UPDATE 違反 accounts_deleted_check，刪除失敗，測試失敗。
func TestDeletionClearsTheSelfEnteredEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "leaving")
	if _, err := f.m.SetSelfEnteredEmail(ctx, id, "me@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if v := f.selfEntered(t, id); v != nil {
		t.Fatalf("刪除之後仍有自填信箱：%v", *v)
	}
	if _, ok, err := f.m.ContactEmail(ctx, id); err != nil || ok {
		t.Fatalf("墓碑的聯絡信箱：ok=%v err=%v", ok, err)
	}
	if _, err := f.db.Pool().Exec(ctx, `UPDATE identity.accounts SET self_entered_email = 'x@example.com' WHERE id = $1`, id); err == nil {
		t.Fatal("accounts_deleted_check 應該擋下墓碑上的自填信箱")
	}
}

// 自填信箱不是自動連結的依據，也不影響登入時 email 的更新：LINE 帳號自填 shared@gmail.com 之後，Google 以同一個已驗證的 gmail 登入建新帳，不連結；
// 之後 LINE 帳號登入拿到已驗證的 email，聯絡信箱改回已驗證的，自填的值保留。
// mutation：自動連結的查詢改讀 accounts 的信箱（email 或 self_entered_email）→ 第一段變成連結，失敗；登入更新 email 時清掉自填 → 最後一段失敗。
func TestSelfEnteredEmailIsNotAnAutoLinkAnchor(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	line := f.loginAs(t, who{provider: ProviderLine, subject: "line-s", email: "", verified: false}, ClientApp)
	if _, err := f.m.SetSelfEnteredEmail(ctx, line.AccountID, "shared@gmail.com"); err != nil {
		t.Fatal(err)
	}
	g := f.loginAs(t, who{provider: ProviderGoogle, subject: "google-s", email: "shared@gmail.com", verified: true}, ClientWeb)
	if !g.Created || g.Linked || g.AccountID == line.AccountID {
		t.Fatalf("Google 登入 = %+v，預期建新帳而不是連到自填信箱的帳號 %d", g, line.AccountID)
	}
	f.loginAs(t, who{provider: ProviderLine, subject: "line-s", email: "line@example.com", verified: true}, ClientApp)
	got, ok, err := f.m.ContactEmail(ctx, line.AccountID)
	if err != nil || !ok || got != (ContactEmail{Address: "line@example.com", Source: ContactEmailVerified}) {
		t.Fatalf("取得已驗證 email 之後：%+v %v %v", got, ok, err)
	}
	if v := f.selfEntered(t, line.AccountID); v == nil || *v != "shared@gmail.com" {
		t.Fatalf("自填的值應保留：%v", v)
	}
}

// 已驗證的 email 格式不合（accounts.email 沒有格式 CHECK）視為沒有：聯絡信箱改用自填的，沒有自填的時 ok 是 false；
// 設定自填信箱不被已驗證的壞值擋住，設定之後讀到自填的；已驗證的有效 email 仍擋設定。
// mutation：contactEmail 不檢查已驗證 email 的格式 → 第一、二段失敗；SetSelfEnteredEmail 改回只看非空 → 第三段失敗；
// 拿掉「有效的已驗證 email 就拒絕」→ 最後一段失敗。
func TestInvalidVerifiedEmailIsTreatedAsAbsent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.emails["bad"] = "not-an-email"
	bad := f.account(t, "bad")
	if got, ok, err := f.m.ContactEmail(ctx, bad); err != nil || ok || got != (ContactEmail{}) {
		t.Fatalf("壞的已驗證信箱且沒有自填：%+v %v %v", got, ok, err)
	}
	if _, err := f.db.Pool().Exec(ctx, `UPDATE identity.accounts SET self_entered_email = 'Self@Example.com' WHERE id = $1`, bad); err != nil {
		t.Fatal(err)
	}
	got, ok, err := f.m.ContactEmail(ctx, bad)
	if err != nil || !ok || got != (ContactEmail{Address: "self@example.com", Source: ContactEmailSelfEntered}) {
		t.Fatalf("壞的已驗證信箱加自填：%+v %v %v", got, ok, err)
	}

	f.emails["bad2"] = "user@localhost"
	bad2 := f.account(t, "bad2")
	set, err := f.m.SetSelfEnteredEmail(ctx, bad2, "me@example.com")
	if err != nil || set != (ContactEmail{Address: "me@example.com", Source: ContactEmailSelfEntered}) {
		t.Fatalf("壞的已驗證信箱時設定：%+v %v", set, err)
	}
	if got, ok, err := f.m.ContactEmail(ctx, bad2); err != nil || !ok || got != set {
		t.Fatalf("設定之後讀到：%+v %v %v", got, ok, err)
	}

	f.emails["good"] = "good@example.com"
	good := f.account(t, "good")
	_, err = f.m.SetSelfEnteredEmail(ctx, good, "me@example.com")
	wantKind(t, err, apperr.KindFailedPrecondition)
	if v := f.selfEntered(t, good); v != nil {
		t.Fatalf("被拒絕之後不該寫入：%v", *v)
	}
}
