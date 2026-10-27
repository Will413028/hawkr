package config

import (
	"strings"
	"testing"
	"time"
)

func TestEnvReadsValuesAndDefaults(t *testing.T) {
	e := FromMap(map[string]string{"URL": "postgres://x", "N": "7", "D": "3s", "B": "true"})
	if got := e.String("URL"); got != "postgres://x" {
		t.Errorf("String = %q", got)
	}
	if got := e.Int("N", 1); got != 7 {
		t.Errorf("Int = %d", got)
	}
	if got := e.Int32("N", 1); got != 7 {
		t.Errorf("Int32 = %d", got)
	}
	if got := e.Duration("D", time.Second); got != 3*time.Second {
		t.Errorf("Duration = %v", got)
	}
	if got := e.Bool("B", false); !got {
		t.Errorf("Bool = %v", got)
	}
	if got := e.Int("MISSING", 42); got != 42 {
		t.Errorf("預設值 = %d", got)
	}
	if got := e.StringOr("MISSING", "def"); got != "def" {
		t.Errorf("StringOr 預設值 = %q", got)
	}
	if err := e.Err(); err != nil {
		t.Fatalf("Err = %v，預期 nil", err)
	}
}

func TestEnvRejectsInt32Overflow(t *testing.T) {
	e := FromMap(map[string]string{"DB_MAX_CONNS": "2147483648"})
	if got := e.Int32("DB_MAX_CONNS", 10); got != 10 {
		t.Errorf("Int32 = %d，預期預設值 10", got)
	}
	if err := e.Err(); err == nil || !strings.Contains(err.Error(), "DB_MAX_CONNS") {
		t.Errorf("Err = %v，預期回報超出範圍", err)
	}
}

func TestEnvReportsEveryProblemAtOnce(t *testing.T) {
	// 空字串視同沒設定：部署設定裡留空的必填值，不能被當成有效值。
	e := FromMap(map[string]string{"EMPTY": "", "N": "abc", "D": "5"})
	e.String("MISSING")
	e.String("EMPTY")
	e.Int("N", 1)
	e.Duration("D", time.Second)

	err := e.Err()
	if err == nil {
		t.Fatal("Err = nil，預期回報問題")
	}
	for _, want := range []string{"MISSING", "EMPTY", "N", "D"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("錯誤訊息沒有提到 %s：%v", want, err)
		}
	}
}

func TestListRequiredBoolAndFail(t *testing.T) {
	e := FromMap(map[string]string{"L": " a, ,b ,", "B": "false", "BAD": "nope"})
	if got := e.List("L"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("List = %q", got)
	}
	if e.List("MISSING") != nil {
		t.Fatal("沒設定的 List 應該是 nil")
	}
	if e.RequiredBool("B") || e.Err() != nil {
		t.Fatalf("RequiredBool = true 或有錯：%v", e.Err())
	}
	e.RequiredBool("BAD")
	e.RequiredBool("MISSING_BOOL")
	e.Fail("整組 %s 只設了一部分", "X")
	if err := e.Err(); err == nil || !strings.Contains(err.Error(), "BAD") || !strings.Contains(err.Error(), "MISSING_BOOL") || !strings.Contains(err.Error(), "整組 X") {
		t.Fatalf("Err = %v", err)
	}
}
