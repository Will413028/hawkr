package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 並行期單一生效者（遷移計畫 A6 不變量）：舊服務在 A6-6e 前仍是評價唯一的寫入者與唯一的 review.created、review.hidden 發送者，
// reviewrelay 只在測試中啟用；A6-6e 刪路由的同一個變更才在這裡接上。cmd/hawkr 的非測試檔 import 它就失敗。
func TestReviewRelayIsNotWiredIntoTheBinary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("掃不到 cmd/hawkr 的原始檔：%v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			if strings.HasSuffix(strings.Trim(imp.Path.Value, `"`), "/internal/legacy/reviewrelay") {
				t.Errorf("%s import 了 reviewrelay：A6-6e 之前 review 的 relay 不接進 cmd/hawkr", f)
			}
		}
	}
	if _, err := os.Stat("review.go"); err != nil {
		t.Fatal(err)
	}
}
