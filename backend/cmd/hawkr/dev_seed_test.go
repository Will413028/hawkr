package main

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func accountSeedSQL(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../infra/seed-data/01-account.sql")
	if err != nil {
		t.Fatal(err)
	}
	var code []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			code = append(code, line)
		}
	}
	return strings.Join(code, "\n")
}

// seedTable 回傳 01 的 INSERT INTO <table> (…) VALUES 之後每一列的欄位文字（以逗號切開、去空白），列以 "(" 開頭、"),"或");" 結尾。
func seedTable(t *testing.T, sql, table string) [][]string {
	t.Helper()
	_, rest, ok := strings.Cut(sql, "INSERT INTO "+table+" ")
	if !ok {
		t.Fatalf("01-account.sql 找不到 INSERT INTO %s", table)
	}
	_, rest, ok = strings.Cut(rest, "VALUES")
	if !ok {
		t.Fatalf("01-account.sql 的 %s 沒有 VALUES", table)
	}
	values, _, _ := strings.Cut(rest, ";")
	var rows [][]string
	for _, m := range regexp.MustCompile(`(?m)^\s*\(([^\n]*)\)[,]?\s*$`).FindAllStringSubmatch(values, -1) {
		var cols []string
		for _, c := range strings.Split(m[1], ",") {
			cols = append(cols, strings.Trim(strings.TrimSpace(c), "'"))
		}
		rows = append(rows, cols)
	}
	if len(rows) == 0 {
		t.Fatalf("01-account.sql 的 %s 解析不到任何列", table)
	}
	return rows
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("不是數字：%q", s)
	}
	return n
}

// devSeedAccountIDs 要與 01-account.sql 的 seed_accounts 雙向一致：01 多一個帳號，它的刪帳申請不會被前提檢查擋下；這裡多一個，前提檢查會無端拒絕。
// 帶賣家店家的帳號要與 devSeedStores 的店家與 store_id 逐一對上（SELLER 角色的 source_id 是店家）。
func TestDevSeedAccountIDsMatchTheAccountSeed(t *testing.T) {
	rows := seedTable(t, accountSeedSQL(t), "seed_accounts (id, email, seller_store_id)")
	var got []int64
	roleStore := map[int64]int64{}
	for _, r := range rows {
		id := mustInt(t, r[0])
		got = append(got, id)
		if r[2] != "NULL" {
			roleStore[id] = mustInt(t, r[2])
		}
	}
	want := slices.Clone(devSeedAccountIDs)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("01-account.sql 的種子帳號 %v 與 devSeedAccountIDs %v 不一致", got, want)
	}
	stores := map[int64]int64{}
	for _, st := range devSeedStores() {
		stores[st.AccountID] = st.StoreID
	}
	if len(stores) != len(roleStore) {
		t.Fatalf("01 授予 SELLER 的帳號 %v 與 devSeedStores 的店家 %v 不一致", roleStore, stores)
	}
	for account, store := range stores {
		if roleStore[account] != store {
			t.Errorf("帳號 %d：01 的 SELLER source_id = %d，devSeedStores 的 store_id = %d", account, roleStore[account], store)
		}
	}
}

// 買家的 buyer_id 一律是 account_id + 50000，帳號與買家的集合一一對應。
func TestAccountSeedBuyersFollowTheIDRule(t *testing.T) {
	sql := accountSeedSQL(t)
	accounts := map[int64]bool{}
	for _, r := range seedTable(t, sql, "seed_accounts (id, email, seller_store_id)") {
		accounts[mustInt(t, r[0])] = true
	}
	buyers := seedTable(t, sql, "seed_buyers (buyer_id, account_id, display_name, phone, gender, city)")
	if len(buyers) != len(accounts) {
		t.Errorf("買家 %d 列、帳號 %d 列，要一一對應", len(buyers), len(accounts))
	}
	for _, r := range buyers {
		buyerID, accountID := mustInt(t, r[0]), mustInt(t, r[1])
		if buyerID != accountID+50000 || !accounts[accountID] {
			t.Errorf("買家 %d 的帳號 %d 不符 buyer_id = account_id + 50000 或不在種子帳號內", buyerID, accountID)
		}
	}
}

// 01 是 upsert：不刪任何列；dev 使用者以帳號 ID 識別，seed 不建任何假的登入身分（account_identities）。
func TestAccountSeedNeverDeletesAndCreatesNoLoginIdentities(t *testing.T) {
	sql := accountSeedSQL(t)
	if m := regexp.MustCompile(`(?i)\b(DELETE\s+FROM|TRUNCATE)`).FindString(sql); m != "" {
		t.Errorf("01-account.sql 不該刪除資料（%q）：種子是冪等 upsert", m)
	}
	if regexp.MustCompile(`(?i)INSERT\s+INTO\s+identity\.account_identities`).MatchString(sql) {
		t.Error("01-account.sql 不該插入 identity.account_identities")
	}
}

// 01 的地址上限是買家模組 MaxAddresses 的複本；種子地址的固定 address_id 是 buyer_id * 100 加序號，且屬於種子買家。
func TestAccountSeedAddressesMatchTheBuyerModule(t *testing.T) {
	src, err := os.ReadFile("../../internal/modules/buyer/internal/model/model.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`const MaxAddresses = (\d+)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("買家模組找不到 MaxAddresses")
	}
	sql := accountSeedSQL(t)
	n := regexp.MustCompile(`max_addresses CONSTANT INT := (\d+);`).FindStringSubmatch(sql)
	if n == nil || n[1] != string(m[1]) {
		t.Errorf("01-account.sql 的 max_addresses %v 與買家模組的 MaxAddresses %s 不一致", n, m[1])
	}
	buyers := map[int64]bool{}
	for _, r := range seedTable(t, sql, "seed_buyers (buyer_id, account_id, display_name, phone, gender, city)") {
		buyers[mustInt(t, r[0])] = true
	}
	for _, r := range seedTable(t, sql, "seed_addresses (address_id, buyer_id, receiver_name, receiver_phone, town_code, address_detail, is_default)") {
		id, buyer := mustInt(t, r[0]), mustInt(t, r[1])
		if !buyers[buyer] || id/100 != buyer {
			t.Errorf("種子地址 %d 的買家 %d 不在種子買家內或不符 address_id = buyer_id * 100 + 序號", id, buyer)
		}
	}
}
