package arch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// social 與組裝它的 cmd/hawkr/social.go 不在 apperr.* 的參數裡寫字串字面值：使用者面的句子在 social/internal/text 的目錄（zh-TW、en），
// 語系取自 context。sqlc、jet 與文案目錄的生成檔、測試檔不看。
func TestSocialApperrCallsCarryNoSentenceLiterals(t *testing.T) {
	root := moduleRoot(t)
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "internal", "modules", "social"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join(root, "cmd", "hawkr", "social.go"))
	apperrCalls, scanned := 0, 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if generatedFile(src) {
			continue
		}
		scanned++
		apperrCalls += strings.Count(string(src), "apperr.")
		got, err := CheckApperrLiterals(path, src)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range got {
			t.Error(v)
		}
	}
	if scanned < 20 || apperrCalls < 30 {
		t.Fatalf("只掃到 %d 個檔、%d 處 apperr：檢查範圍不對", scanned, apperrCalls)
	}
}

func TestCheckApperrLiteralsFlagsSentencesOnly(t *testing.T) {
	const src = `package x

func f(ctx context.Context, err error) error {
	_ = apperr.InvalidInputOn([]string{"ProductID"}, "%s", text.X(loc))
	_ = apperr.NotFound("%s", text.Y(loc))
	_ = apperr.WithCause(apperr.Unavailable("暫時無法確認商品"), err)
	_ = apperr.PermissionDenied("需要" + "登入")
	_ = apperr.InvalidInput(fmt.Sprintf("bad %d", 1))
	_ = other.Thing("字串")
	return nil
}
`
	got, err := CheckApperrLiterals("x.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("要抓到 4 處（Unavailable 的句子、兩段相加各一、Sprintf 的格式字串），實際 %d：%v", len(got), got)
	}
	for _, v := range got {
		if strings.Contains(v, "other") || strings.Contains(v, `"%s"`) || strings.Contains(v, "ProductID") {
			t.Errorf("不該被抓：%s", v)
		}
	}
}
