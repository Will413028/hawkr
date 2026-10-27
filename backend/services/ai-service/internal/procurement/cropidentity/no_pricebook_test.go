package cropidentity

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 品項解析只靠分類樹與 LLM：解析路徑（本套件與 recall、fuse）不得依賴 pricebook 的
// 任何東西。pricebook 的 client 與型別住在 internal/client/pricebook 與
// internal/usecase；把任一個 import 回來，就等於把行情查詢接回召回。
//
// 這裡看 import 而不是塞一個「一呼叫就失敗的 pricebook 替身」，因為解析路徑現在
// 沒有任何接 pricebook 的參數可以塞——建構子只收分類樹與模型。
func TestResolutionPathDoesNotDependOnPricebook(t *testing.T) {
	forbidden := []string{
		"/internal/client/pricebook",
		"/internal/usecase",
		"pkg/proto/pricebook",
	}
	for _, dir := range []string{".", "recall", "fuse"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			require.NoError(t, err)
			parsed, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
			require.NoError(t, err)
			for _, imp := range parsed.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, bad := range forbidden {
					require.NotContainsf(t, path+"/", bad+"/", "%s 依賴了 pricebook：%s", file, path)
				}
			}
		}
	}
}
