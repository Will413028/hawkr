//go:build integration

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 產生的特性測試的組裝（monolith()：migration、identity、delegation、httpapi）要真的跑得起來，只編譯抓不到 panic（例如 delegation 缺依賴）。
// 只跑 TestMonolith：monolith() 在自己的空 database 起單體 HTTP 層（testcontainers）；骨架的 scenarios 是空的，ch.Run 會以「沒有任何情境」
// 失敗，那是骨架的預期（寫情境是使用者的事），其他任何輸出（panic、組裝失敗）都算失敗。
func TestGeneratedCharacterizationSetupRuns(t *testing.T) {
	bin, dir := buildGeneratedCharacterization(t)
	cmd := exec.CommandContext(t.Context(), bin, "-test.run", "^TestMonolith$", "-test.v")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "沒有任何情境") || strings.Contains(string(out), "panic") {
		t.Fatalf("產生的特性測試的組裝沒有跑到 ch.Run（或 panic）：\n%s", out)
	}
}
