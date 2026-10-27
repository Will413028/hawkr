package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEveryModuleIsRegistered(t *testing.T) {
	var onDisk []string
	entries, err := os.ReadDir("../../internal/modules")
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			onDisk = append(onDisk, e.Name())
		}
	}

	var registered []string
	for _, m := range modules() {
		if slices.Contains(registered, m.Name) {
			t.Errorf("模組 %s 登記了兩次", m.Name)
		}
		registered = append(registered, m.Name)
	}

	for _, m := range apiOnly {
		if slices.Contains(registered, m) {
			t.Errorf("模組 %s 同時在 modules() 與 apiOnly：實作搬進來後要從 apiOnly 移除", m)
		}
		registered = append(registered, m)
	}

	for _, m := range withoutData {
		if slices.Contains(registered, m) {
			t.Errorf("模組 %s 在 withoutData 之外又登記了一次：有 migration 的模組不屬於 withoutData", m)
		}
		if _, err := os.Stat(filepath.Join("..", "..", "internal", "modules", m, "db", "schema.sql")); err == nil {
			t.Errorf("模組 %s 在 withoutData 裡卻有 db/schema.sql：有自己的資料就移到 modules() 並登記 migration", m)
		}
		registered = append(registered, m)
	}

	slices.Sort(onDisk)
	slices.Sort(registered)
	if !slices.Equal(onDisk, registered) {
		t.Fatalf("internal/modules 底下的模組 %v 與 modules()、apiOnly 登記的 %v 不一致", onDisk, registered)
	}
}

// 搬進單體的模組，CARRYOVER.md 都要有「沿用盤點」表（遷移計畫「模組完成的定義」）。
// 這裡只檢查表存在而且每格都有填；內容是否敷衍，靠模組完成前的獨立設計審查。
// 盤點是決策紀錄，不放進每次都載入的 AGENTS.md，所以 AGENTS.md 不得有這一節。
func TestEveryMovedModuleHasACarryOverAudit(t *testing.T) {
	names := slices.Clone(withoutData)
	for _, m := range modules() {
		names = append(names, m.Name)
	}
	for _, name := range names {
		dir := filepath.Join("..", "..", "internal", "modules", name)
		md, err := os.ReadFile(filepath.Join(dir, "CARRYOVER.md"))
		if err != nil {
			t.Errorf("模組 %s：%v", name, err)
			continue
		}
		if err := checkCarryOverAudit(string(md)); err != nil {
			t.Errorf("模組 %s 的 CARRYOVER.md：%v", name, err)
		}
		agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Errorf("模組 %s：%v", name, err)
			continue
		}
		if strings.Contains(string(agents), "\n## 沿用盤點\n") {
			t.Errorf("模組 %s 的 AGENTS.md 有「沿用盤點」一節：盤點放在 CARRYOVER.md", name)
		}
	}
}

func TestCarryOverAuditCheckRejectsIncompleteTables(t *testing.T) {
	const header = "| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |\n|---|---|---|---|---|\n"
	cases := []struct {
		name string
		md   string
		ok   bool
	}{
		{"完整", "## 沿用盤點\n\n" + header + "| TSID | node 編號 | 不成立 | UUIDv7 | 改 UUIDv7 |\n", true},
		{"儲存格裡跳脫的直線", "## 沿用盤點\n" + header + "| a \\| b | c | d | e | f |\n", true},
		{"沒有這一節", "## 不變量\n" + header + "| a | b | c | d | e |\n", false},
		{"這一節沒有表", "## 沿用盤點\n\n沒有照搬任何東西。\n", false},
		{"表在下一節", "## 沿用盤點\n\n## 讀取模型\n" + header + "| a | b | c | d | e |\n", false},
		{"欄位不對", "## 沿用盤點\n| 機制 | 決定 |\n|---|---|\n| a | b |\n", false},
		{"只有表頭", "## 沿用盤點\n" + header, false},
		{"有空格", "## 沿用盤點\n" + header + "| TSID | node 編號 | 不成立 |  | 改 UUIDv7 |\n", false},
		{"少一欄", "## 沿用盤點\n" + header + "| TSID | node 編號 | 不成立 | 改 UUIDv7 |\n", false},
	}
	for _, c := range cases {
		err := checkCarryOverAudit(c.md)
		if (err == nil) != c.ok {
			t.Errorf("%s：err = %v，預期 ok = %v", c.name, err, c.ok)
		}
	}
}

// carryOverColumns 是「沿用盤點」表的欄位。
var carryOverColumns = []string{"沿用的機制", "必要條件", "今天是否成立", "業界在同樣條件下的做法", "決定"}

// checkCarryOverAudit 確認 md 有「## 沿用盤點」一節，其中的表欄位是 carryOverColumns，
// 至少有一列，而且每一格都有填。
func checkCarryOverAudit(md string) error {
	lines := strings.Split(md, "\n")
	start := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "## 沿用盤點" })
	if start < 0 {
		return errors.New("沒有「## 沿用盤點」一節")
	}
	var rows [][]string
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "## ") {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(l), "|") {
			rows = append(rows, tableCells(l))
		}
	}
	switch {
	case len(rows) == 0:
		return errors.New("「沿用盤點」一節沒有表")
	case !slices.Equal(rows[0], carryOverColumns):
		return fmt.Errorf("表的欄位是 %q，應該是 %q", rows[0], carryOverColumns)
	case len(rows) < 3:
		return errors.New("盤點表沒有任何一列")
	}
	for i, r := range rows[2:] {
		if len(r) != len(carryOverColumns) {
			return fmt.Errorf("第 %d 列有 %d 欄，應該是 %d 欄", i+1, len(r), len(carryOverColumns))
		}
		for j, c := range r {
			if c == "" {
				return fmt.Errorf("第 %d 列的「%s」沒有填", i+1, carryOverColumns[j])
			}
		}
	}
	return nil
}

// tableCells 切出 Markdown 表格一列的儲存格；儲存格裡的直線要寫成 \|。
func tableCells(line string) []string {
	line = strings.ReplaceAll(strings.TrimSpace(line), `\|`, "\x00")
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|"), "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(strings.ReplaceAll(p, "\x00", `\|`))
	}
	return parts
}
