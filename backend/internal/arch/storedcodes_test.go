package arch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// storedCodeExceptions 是 CHECK 值清單允許小寫的例外，每一筆都要有理由；沒有被任何 schema.sql 用到的筆數會讓測試失敗（過期的例外不留）。
var storedCodeExceptions = []StoredCodeException{
	{"delegation", "delegated_operation_sessions_capabilities_check", "能力是「資源.動作」的權限識別字（storefront.read），是 gateway 與 admin 的 allow-list 成員，不是列舉代碼；對外契約的形式"},
	{"delegation", "admin_operation_audit_logs_capability_check", "同上：能力的權限識別字"},
	{"delegation", "seller_delegation_authorizations_capabilities_check", "同上：能力的權限識別字"},
	{"seller", "kyc_registration_checks_fields_check", "值是欄位路徑（business.company_name），對應 proto 的欄位名，不是列舉代碼"},
	{"seller", "kyc_review_issues_field_check", "同上：欄位路徑，審核退件指向的表單欄位"},
	{"seller", "kyc_review_issues_asset_check", "同上：欄位路徑（id_card_front 等），限制哪些欄位可以掛證件圖片"},
	{"content", "banners_action_check", "待改（另案）：action_type 的 popup、url、product、post 是代碼，要連同 Go 型別、proto 與 admin／client 的對應一起改成 UPPER_SNAKE；本次只收 delegation"},
}

func TestStoredCodesInChecksAreUpperSnake(t *testing.T) {
	root := moduleRoot(t)
	var files []string
	for _, pattern := range []string{"db/schema.sql", "db/objects.sql"} {
		m, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 8 {
		t.Fatalf("只找到 %d 份 SQL，檢查範圍不對", len(files))
	}
	used := map[StoredCodeException]bool{}
	checked := 0
	for _, path := range files {
		rel, _ := filepath.Rel(filepath.Join(root, "internal", "modules"), path)
		module, file, _ := strings.Cut(rel, string(filepath.Separator))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		checked += strings.Count(string(raw), "IN ('") + strings.Count(string(raw), "ARRAY['")
		for _, v := range CheckStoredCodeCase(module, file, string(raw), storedCodeExceptions, used) {
			t.Error(v)
		}
	}
	if checked < 10 {
		t.Fatalf("只找到 %d 個值清單，檢查範圍不對", checked)
	}
	for _, e := range storedCodeExceptions {
		if e.Reason == "" {
			t.Errorf("例外 %s／%s 沒有理由", e.Module, e.Constraint)
		}
		if !used[e] {
			t.Errorf("例外 %s／%s 沒有任何 CHECK 用到：已修掉或改名，刪掉這一筆", e.Module, e.Constraint)
		}
	}
}

func TestCheckStoredCodeCaseFlagsLowercaseValueLists(t *testing.T) {
	cases := map[string]struct {
		sql  string
		want string
	}{
		"IN 清單":      {"CREATE TABLE s.t (v TEXT, CONSTRAINT t_v_check CHECK (v::text IN ('ACTIVE', 'revoked')));", "t_v_check"},
		"ARRAY 清單":   {"CREATE TABLE s.t (v TEXT, CONSTRAINT t_v_check CHECK (v = ANY (ARRAY['low'::text, 'HIGH'::text])));", "low"},
		"欄位內嵌 CHECK": {"CREATE TABLE s.t (v TEXT CHECK (v IN ('a_b')));", "(未命名)"},
		"多行 IN":      {"CREATE TABLE s.t (v TEXT, CONSTRAINT c CHECK (\n v IN (\n 'OK',\n 'Mixed'\n )\n));", "Mixed"},
		"巢狀括號後的 IN":  {"CREATE TABLE s.t (v TEXT, w INT, CONSTRAINT c CHECK ((w > 0) AND v IN ('x')));", "x"},
	}
	for name, c := range cases {
		got := CheckStoredCodeCase("m", "schema.sql", c.sql, nil, map[StoredCodeException]bool{})
		if len(got) != 1 || !strings.Contains(got[0], c.want) {
			t.Errorf("%s：%v", name, got)
		}
	}
	ok := map[string]string{
		"全大寫":          "CREATE TABLE s.t (v TEXT, CONSTRAINT c CHECK (v::text IN ('ACTIVE', 'PLATFORM_POLICY')));",
		"註解裡的小寫":       "CREATE TABLE s.t (v TEXT, -- IN ('active')\n CONSTRAINT c CHECK (v IN ('A')));",
		"非值清單的小寫字串":    "CREATE TABLE s.t (v TEXT, CONSTRAINT c CHECK (v ~ '^[a-z]+$' AND v <> ''));",
		"CHECK 之外的 IN": "CREATE INDEX i ON s.t (v) WHERE v IN ('active');",
	}
	for name, sql := range ok {
		if got := CheckStoredCodeCase("m", "schema.sql", sql, nil, map[StoredCodeException]bool{}); len(got) != 0 {
			t.Errorf("%s 被誤判：%v", name, got)
		}
	}
	// 例外以（模組、constraint）為鍵並登記使用。
	ex := []StoredCodeException{{"m", "c", "理由"}}
	used := map[StoredCodeException]bool{}
	if got := CheckStoredCodeCase("m", "schema.sql", "CREATE TABLE s.t (v TEXT, CONSTRAINT c CHECK (v IN ('x')));", ex, used); len(got) != 0 || !used[ex[0]] {
		t.Errorf("例外沒有生效：%v %v", got, used)
	}
	if got := CheckStoredCodeCase("other", "schema.sql", "CREATE TABLE s.t (v TEXT, CONSTRAINT c CHECK (v IN ('x')));", ex, map[StoredCodeException]bool{}); len(got) != 1 {
		t.Errorf("別的模組的同名 constraint 不應豁免：%v", got)
	}
}
