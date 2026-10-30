package main

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// devSeedRebuiltAccounts 要與 01-account.sql 刪除重建的帳號雙向一致：01 多一個帳號，區間內的店家不會被前提檢查擋下，重建帳號後失去 SELLER 角色；
// 這裡多一個，前提檢查會無端拒絕。兩邊都解析成 (From, To) 的清單（單一 ID 是 (n, n)）後逐項比對，條件裡有認不得的寫法也失敗。
func TestDevSeedRebuiltAccountsMatchTheAccountSeed(t *testing.T) {
	b, err := os.ReadFile("../../infra/seed-data/01-account.sql")
	if err != nil {
		t.Fatal(err)
	}
	const marker = "SELECT id FROM identity.accounts WHERE "
	_, rest, ok := strings.Cut(string(b), marker)
	if !ok {
		t.Fatalf("01-account.sql 找不到 %q", marker)
	}
	where, _, _ := strings.Cut(rest, ";")

	type span struct{ from, to int64 }
	var got []span
	num := func(s string) int64 {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			t.Fatalf("01-account.sql 的清除條件有不是數字的值 %q：%s", s, where)
		}
		return n
	}
	left := where
	between := regexp.MustCompile(`id BETWEEN (\d+) AND (\d+)`)
	for _, m := range between.FindAllStringSubmatch(left, -1) {
		got = append(got, span{num(m[1]), num(m[2])})
	}
	left = between.ReplaceAllString(left, "")
	in := regexp.MustCompile(`id IN \(([^)]*)\)`)
	for _, m := range in.FindAllStringSubmatch(left, -1) {
		for _, v := range strings.Split(m[1], ",") {
			got = append(got, span{num(v), num(v)})
		}
	}
	left = in.ReplaceAllString(left, "")
	if rest := strings.TrimSpace(strings.ReplaceAll(left, "OR", "")); rest != "" {
		t.Fatalf("01-account.sql 的清除條件有這個測試認不得的寫法 %q（支援 id IN (…) 與 id BETWEEN a AND b，以 OR 連接）：%s", rest, where)
	}

	var want []span
	for _, r := range devSeedRebuiltAccounts {
		want = append(want, span{r.From, r.To})
	}
	cmp := func(a, b span) int {
		if a.from != b.from {
			return int(a.from - b.from)
		}
		return int(a.to - b.to)
	}
	slices.SortFunc(got, cmp)
	slices.SortFunc(want, cmp)
	if !slices.Equal(got, want) {
		t.Errorf("01-account.sql 刪除重建的帳號 %v 與 devSeedRebuiltAccounts %v 不一致：%s", got, want, where)
	}
}

// dev 使用者以帳號 ID 識別，seed 不建任何假的登入身分（account_identities）；重置段落仍刪它，真人連結過的 seed 帳號才能重置。
func TestAccountSeedCreatesNoLoginIdentities(t *testing.T) {
	b, err := os.ReadFile("../../infra/seed-data/01-account.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	if regexp.MustCompile(`(?i)INSERT\s+INTO\s+identity\.account_identities`).MatchString(sql) {
		t.Error("01-account.sql 不該插入 identity.account_identities")
	}
	if !strings.Contains(sql, "DELETE FROM identity.account_identities WHERE account_id IN (SELECT id FROM seed_accounts)") {
		t.Error("01-account.sql 的重置段落要刪 seed 帳號的 account_identities")
	}
}
