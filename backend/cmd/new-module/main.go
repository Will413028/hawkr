// Command new-module 產生單體模組的骨架（遷移計畫 0.5）：模板從第一個搬進來的 content 抽出。開發時由
// make new-module、make new-characterization 呼叫，不部署（唯一部署的 binary 是 cmd/hawkr）。
//
//	go run ./cmd/new-module module <模組> [-from <舊服務>]            # internal/modules/<模組>，並登記到 cmd/hawkr
//	go run ./cmd/new-module characterization <模組> [-legacy <舊服務>] # internal/httpapi/<模組>/characterization
//
// 在 backend/ 執行。產生的檔案刻意留著會讓測試失敗的空位（沿用盤點的空列、沒有情境），填好才算完成。
package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Will413028/hawkr/backend/internal/arch"
)

//go:embed templates
var templates embed.FS

// data 是模板的參數。
type data struct {
	Name   string // 模組名稱，也是 Go package 與 DB schema 的名稱
	Legacy string // 舊服務的名稱（例如 content-service）；沒有時是空字串
	Module string // 單體的 Go module path
}

var validName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "new-module：", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return errors.New("用法：new-module module <模組> [-from <舊服務>]｜new-module characterization <模組> [-legacy <舊服務>]")
	}
	kind, name := args[0], args[1]
	fl := flag.NewFlagSet(kind, flag.ContinueOnError)
	legacy := fl.String("legacy", "", "舊服務的名稱")
	from := fl.String("from", "", "舊服務的名稱")
	if err := fl.Parse(args[2:]); err != nil {
		return err
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("模組名稱 %q 必須是小寫英文字母開頭、只有小寫英文字母與數字（它也是 Go package 與 DB schema 的名稱）", name)
	}
	switch kind {
	case "module":
		if err := newModule(".", data{Name: name, Legacy: *from, Module: arch.ModulePath}); err != nil {
			return err
		}
		fmt.Printf(`已產生 internal/modules/%s，並登記到 cmd/hawkr/modules.go 與 internal/arch/modules.go。接下來（遷移計畫「模組完成的定義」）：
  1. 先做 CARRYOVER.md 的沿用盤點（TestEveryMovedModuleHasACarryOverAudit 在表還是空列時失敗），再填 AGENTS.md。
  2. 在 db/schema.sql 寫表，在 db/ 跑 atlas migrate diff <名稱> --env dev；寫第一支 query 後 make sqlc。
  3. 根套件只做公開 API；把 internal/feature 改成子領域名稱，在 cmd/hawkr 組裝，並更新 internal/arch 的依賴圖。
  4. 對外 API 先改根目錄 api/ 的 proto，再 make connect；特性測試用 make new-characterization 產生骨架。
`, name)
	case "characterization":
		if err := newCharacterization(".", data{Name: name, Legacy: *legacy, Module: arch.ModulePath}); err != nil {
			return err
		}
		fmt.Printf(`已產生 internal/httpapi/%s/characterization。接下來（ADR 2026-09-26-external-contract-protobuf-connect D4）：
  1. 在 driver_test.go 定義 Driver，情境寫在 scenarios_test.go：ch.Run 在沒有情境時失敗。
  2. Connect driver 用生成的 client 打單體；monolith_integration_test.go 在 httpapi.Deps 接上模組。
  3. 有舊服務時實作 REST driver，並在 legacy.sh 起舊服務；scripts/characterize.sh %s 經 gateway 跑兩邊。
`, name, name)
	default:
		return fmt.Errorf("不認得的種類 %q：module 或 characterization", kind)
	}
	return nil
}

func newModule(root string, d data) error {
	if _, ok := arch.Dependencies[d.Name]; !ok {
		return fmt.Errorf("%s 不在依賴表（internal/arch/modules.go）：新模組要先寫進依賴表與 ADR D2 的表", d.Name)
	}
	dir := filepath.Join(root, "internal", "modules", d.Name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s 已經存在，不覆寫", dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := renderFiles("templates/module", dir, d, map[string]string{
		"module.go.tmpl":           d.Name + ".go",
		"integration_test.go.tmpl": d.Name + "_integration_test.go",
	}, nil)
	if err != nil {
		return err
	}
	graphPath := filepath.Join(root, "internal", "arch", "modules.go")
	graph, err := registerInternalGraph(graphPath, d)
	if err != nil {
		return err
	}
	modulesPath := filepath.Join(root, "cmd", "hawkr", "modules.go")
	modules, err := register(modulesPath, d)
	if err != nil {
		return err
	}
	files = append(files, generatedFile{path: graphPath, body: graph}, generatedFile{path: modulesPath, body: modules})
	return writeFiles(files)
}

func registerInternalGraph(path string, d data) ([]byte, error) {
	return spliceGo(path, func(src string) (string, error) {
		const list = "var InternalDependencies = map[string]InternalGraph{\n"
		pos := strings.Index(src, list)
		if pos < 0 {
			return "", fmt.Errorf("%s 裡找不到內部依賴圖", path)
		}
		pos += len(list)
		if strings.Contains(src[pos:], fmt.Sprintf("\t%q:", d.Name)) {
			return "", fmt.Errorf("%s 的內部依賴圖已有模組 %s", path, d.Name)
		}
		entry := fmt.Sprintf("\t%q: {Packages: map[string][]string{\"feature\": {}, \"model\": {}, \"store\": {}}},\n", d.Name)
		return src[:pos] + entry + src[pos:], nil
	})
}

func newCharacterization(root string, d data) error {
	if _, err := os.Stat(filepath.Join(root, "internal", "modules", d.Name)); err != nil {
		return fmt.Errorf("先建立模組（make new-module name=%s）：%w", d.Name, err)
	}
	dir := filepath.Join(root, "internal", "httpapi", d.Name, "characterization")
	skip := map[string]bool{}
	if d.Legacy == "" {
		skip["rest_test.go.tmpl"], skip["legacy.sh.tmpl"] = true, true
	}
	if err := render("templates/characterization", dir, d, nil, skip); err != nil {
		return err
	}
	if d.Legacy != "" {
		if err := os.Chmod(filepath.Join(dir, "legacy.sh"), 0o755); err != nil { //nolint:gosec // generated local script must be executable
			return err
		}
	}
	return nil
}

// render 把 src 底下的每個模板渲染到 dst：檔名去掉 .tmpl（rename 有列的改名），skip 列的不產生。
// 目的地已經存在時不覆寫任何東西。
func render(src, dst string, d data, rename map[string]string, skip map[string]bool) error {
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s 已經存在，不覆寫", dst)
	}
	files, err := renderFiles(src, dst, d, rename, skip)
	if err != nil {
		return err
	}
	return writeFiles(files)
}

type generatedFile struct {
	path string
	body []byte
}

func renderFiles(src, dst string, d data, rename map[string]string, skip map[string]bool) ([]generatedFile, error) {
	var files []generatedFile
	err := fs.WalkDir(templates, src, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if skip[rel] {
			return nil
		}
		out := strings.TrimSuffix(rel, ".tmpl")
		if name, ok := rename[rel]; ok {
			out = filepath.Join(filepath.Dir(rel), name)
		}
		body, err := execute(path, d)
		if err != nil {
			return err
		}
		files = append(files, generatedFile{path: filepath.Join(dst, out), body: body})
		return nil
	})
	return files, err
}

func writeFiles(files []generatedFile) error {
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil { //nolint:gosec // generated source directories need normal traversal permissions
			return err
		}
		if err := os.WriteFile(file.path, file.body, 0o644); err != nil { //nolint:gosec // generated source files are intentionally readable
			return err
		}
	}
	return nil
}

// execute 渲染一個模板；Go 檔再經過 gofmt，模板的縮排因此不必完全照 gofmt 寫。
func execute(path string, d data) ([]byte, error) {
	raw, err := templates.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(path)).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	if !strings.HasSuffix(path, ".go.tmpl") {
		return buf.Bytes(), nil
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%s 渲染出的 Go 不合法：%w", path, err)
	}
	return src, nil
}

func spliceGo(path string, edit func(string) (string, error)) ([]byte, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path is a local Go source file
	if err != nil {
		return nil, err
	}
	src, err := edit(string(raw))
	if err != nil {
		return nil, err
	}
	out, err := format.Source([]byte(src))
	if err != nil {
		return nil, fmt.Errorf("登記後的 %s 不合法：%w", path, err)
	}
	return out, nil
}

// register 把模組加到 cmd/hawkr/modules.go 的 modules() 最後面（順序就是 migrate 套用的順序）。
func register(path string, d data) ([]byte, error) {
	return spliceGo(path, func(src string) (string, error) {
		const list = "return []migrate.Module{\n"
		start := strings.Index(src, list)
		if start < 0 {
			return "", fmt.Errorf("%s 裡找不到 modules() 的清單", path)
		}
		end := strings.Index(src[start:], "\n\t}\n")
		if end < 0 {
			return "", fmt.Errorf("%s 的 modules() 清單沒有結尾", path)
		}
		end += start + 1
		entry := fmt.Sprintf("\t\t{Name: %q, Dir: %s.Migrations()},\n", d.Name, d.Name)
		src = src[:end] + entry + src[end:]
		imp := fmt.Sprintf("\t\"%s/internal/modules/%s\"\n", d.Module, d.Name)
		src = strings.Replace(src, "import (\n", "import (\n"+imp, 1)
		return src, nil
	})
}
