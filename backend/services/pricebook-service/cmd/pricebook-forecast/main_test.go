package main

import (
	"context"
	"strings"
	"testing"
)

// TestRunRejectsGroupModeWithMarket 驗證 P4 review followup 補的驗證：
// package doc／flag usage 文字原本就宣稱「--group 模式不接受 --market」，
// 但實際上從未真的檢查過（task-11-report.md §6）。這條檢查在載入 model.json
// 或連線資料庫之前就會觸發，不需要真的檔案或 DB。
func TestRunRejectsGroupModeWithMarket(t *testing.T) {
	err := run(context.Background(), []string{
		"--model", "does-not-need-to-exist.json",
		"--group", "GRP-A",
		"--market", "109",
	})
	if err == nil {
		t.Fatal("--group 與 --market 同時給值應該回錯誤")
	}
	if !strings.Contains(err.Error(), "--group 模式不接受 --market") {
		t.Errorf("錯誤訊息 = %q，未包含預期的說明", err.Error())
	}
}

// TestRunAllowsGroupModeWithoutMarket 確認正常的 --group（不帶 --market）
// 不會被新加的檢查誤擋——會在載入 model.json 這一步失敗（檔案不存在），
// 但那是驗證通過之後的下一步，證明新檢查沒有連帶擋掉合法輸入。
func TestRunAllowsGroupModeWithoutMarket(t *testing.T) {
	err := run(context.Background(), []string{
		"--model", "does-not-exist.json",
		"--group", "GRP-A",
	})
	if err == nil {
		t.Fatal("model.json 不存在應該回錯誤（驗證這個測試真的往下跑到載入那一步）")
	}
	if strings.Contains(err.Error(), "--group 模式不接受 --market") {
		t.Error("沒有帶 --market 卻被新加的檢查擋下，誤判")
	}
}
