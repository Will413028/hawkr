package arch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 模組不得自己寫 Gemini 的端點：手寫 HTTP 會繞過 internal/platform/llm 這個唯一的建構點（LLMClientImports 與 LLMClientCalls 只擋 SDK 的 import 與
// genai.NewClient，抓不到字串寫死的 URL）。
func TestModulesDoNotHardcodeTheGeminiEndpoint(t *testing.T) {
	const endpoint = "generativelanguage.googleapis.com"
	modules := filepath.Join(moduleRoot(t), "internal", "modules")
	scanned := 0
	err := filepath.WalkDir(modules, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		if strings.Contains(string(raw), endpoint) {
			rel, _ := filepath.Rel(modules, path)
			t.Errorf("internal/modules/%s 含 %s：LLM 呼叫一律經 internal/platform/llm", rel, endpoint)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 100 {
		t.Fatalf("只掃到 %d 個檔案，檢查範圍不對", scanned)
	}
}
