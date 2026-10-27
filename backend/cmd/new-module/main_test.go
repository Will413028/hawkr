package main

import (
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/arch"
)

// scaffold 在暫存目錄裡模擬 backend/：放一份真的 cmd/hawkr/modules.go，產生 payment（依賴表裡有、還沒
// 搬進來的模組）的模組與特性測試骨架。
func scaffold(t *testing.T, legacy string) string {
	t.Helper()
	root := t.TempDir()
	modules, err := os.ReadFile("../hawkr/modules.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd", "hawkr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "hawkr", "modules.go"), modules, 0o644); err != nil {
		t.Fatal(err)
	}
	archSource, err := os.ReadFile(filepath.Join("..", "..", "internal", "arch", "modules.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "arch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "arch", "modules.go"), archSource, 0o644); err != nil {
		t.Fatal(err)
	}
	d := data{Name: "payment", Legacy: legacy, Module: arch.ModulePath}
	if err := newModule(root, d); err != nil {
		t.Fatal(err)
	}
	if err := newCharacterization(root, d); err != nil {
		t.Fatal(err)
	}
	return root
}

// templateAction 是沒有被渲染掉的模板動作；Go 的 composite literal（{{Name: …}}）不算。
var templateAction = regexp.MustCompile(`\{\{(\.|if |end|else|")`)

// 產生的 Go 檔都要是合法而且已經 gofmt 過的：模板改壞時在這裡失敗，不必等下一個模組搬遷才發現。
func TestGeneratedFilesAreValid(t *testing.T) {
	for _, legacy := range []string{"", "account-service"} {
		root := scaffold(t, legacy)
		var goFiles []string
		err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
			if err == nil && strings.HasSuffix(path, ".go") {
				goFiles = append(goFiles, path)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range goFiles {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			formatted, err := format.Source(src)
			if err != nil {
				t.Errorf("%s：%v", f, err)
			} else if string(formatted) != string(src) {
				t.Errorf("%s 沒有 gofmt", f)
			}
			if templateAction.Match(src) {
				t.Errorf("%s 留著沒有渲染的模板語法", f)
			}
		}
		chDir := filepath.Join(root, "internal", "httpapi", "payment", "characterization")
		_, restErr := os.Stat(filepath.Join(chDir, "rest_test.go"))
		if (restErr == nil) != (legacy != "") {
			t.Errorf("legacy = %q：rest_test.go 存在 = %v", legacy, restErr == nil)
		}
		if legacy != "" {
			if out, err := exec.CommandContext(t.Context(), "bash", "-n", filepath.Join(chDir, "legacy.sh")).CombinedOutput(); err != nil {
				t.Errorf("legacy.sh 語法錯誤：%v\n%s", err, out)
			}
		}
	}
}

func TestGeneratedSkeletonCompiles(t *testing.T) {
	root := scaffold(t, "")
	archCheck, err := os.ReadFile(filepath.Join("..", "..", "internal", "arch", "check.go"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":                     "module " + arch.ModulePath + "\n\ngo 1.27.1\n",
		"internal/platform/db/db.go": "package db\ntype DB struct{}\n",
		"internal/modules/payment/db/migrations/001.sql":   "-- migration\n",
		"internal/modules/payment/db/migrations/atlas.sum": "h1:test\n",
		"internal/arch/check.go":                           string(archCheck),
		"internal/arch/generated_test.go": `package arch

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGeneratedGraph(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}{{range .Imports}} {{.}}{{end}}{{range .TestImports}} {{.}}{{end}}{{range .XTestImports}} {{.}}{{end}}", "./internal/modules/payment/...")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	var pkgs []Package
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		pkgs = append(pkgs, Package{ImportPath: fields[0], Imports: fields[1:]})
	}
	graph := map[string]InternalGraph{"payment": InternalDependencies["payment"]}
	for _, v := range CheckInternal(pkgs, graph, []string{"payment"}) {
		t.Error(v)
	}
	for _, problem := range ValidateInternalGraphs(graph) {
		t.Error(problem)
	}
}

`,
	}
	for path, body := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.CommandContext(t.Context(), "go", "test", "./internal/arch", "./internal/modules/payment/...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOCACHE="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("產生的模組無法編譯：%v\n%s", err, out)
	}
}

// buildGeneratedCharacterization 產生 content 的特性測試骨架並編譯成 integration 的測試執行檔，回傳它的路徑與測試的工作目錄。
func buildGeneratedCharacterization(t *testing.T) (bin, dir string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "modules", "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := newCharacterization(root, data{Name: "content", Module: arch.ModulePath}); err != nil {
		t.Fatal(err)
	}
	chDir := filepath.Join(root, "internal", "httpapi", "content", "characterization")
	backend, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mod := "module " + arch.ModulePath + "/internal/httpapi/content/characterization\n\ngo 1.27.1\n\nrequire " + arch.ModulePath + " v0.0.0\n\nreplace " + arch.ModulePath + " => " + backend + "\n"
	if err := os.WriteFile(filepath.Join(chDir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	bin = filepath.Join(root, "characterization.test")
	cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-mod=mod", "-tags", "integration", "-o", bin, ".")
	cmd.Dir = chDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("產生的特性測試無法編譯：%v\n%s", err, out)
	}
	return bin, chDir
}

func TestGeneratedCharacterizationCompiles(t *testing.T) { buildGeneratedCharacterization(t) }

func TestNewModulePreparesRegistrationsBeforeWriting(t *testing.T) {
	root := t.TempDir()
	graphPath := filepath.Join(root, "internal", "arch", "modules.go")
	graph, err := os.ReadFile(filepath.Join("..", "..", "internal", "arch", "modules.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(graphPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(graphPath, graph, 0o644); err != nil {
		t.Fatal(err)
	}
	modulesPath := filepath.Join(root, "cmd", "hawkr", "modules.go")
	if err := os.MkdirAll(filepath.Dir(modulesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modulesPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := newModule(root, data{Name: "payment", Module: arch.ModulePath}); err == nil {
		t.Fatal("缺少登記清單時仍建立了模組")
	}
	got, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(graph) {
		t.Error("登記失敗卻已改寫內部依賴圖")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "modules", "payment")); !os.IsNotExist(err) {
		t.Errorf("登記失敗卻已產生模組：%v", err)
	}
}

// CARRYOVER.md 要有兩張盤點表，而且留著空列：填好之前 TestEveryMovedModuleHasACarryOverAudit 失敗。
// AGENTS.md 只留連結，盤點不進每次都載入的指令檔。
func TestGeneratedAgentsLeavesTheAuditToFill(t *testing.T) {
	dir := filepath.Join(scaffold(t, "account-service"), "internal", "modules", "payment")
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	carry, agents := read("CARRYOVER.md"), read("AGENTS.md")
	for _, want := range []string{
		"## 沿用盤點", "## 傳輸層的沿用盤點", "## 與 account-service 的差異",
		"| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |\n|---|---|---|---|---|\n|  |  |  |  |  |",
	} {
		if !strings.Contains(carry, want) {
			t.Errorf("CARRYOVER.md 缺少 %q", want)
		}
	}
	for _, want := range []string{
		"## 模組內部結構", "依賴圖的唯一來源", "自己套件宣告最小的 `Deps`", "`CARRYOVER.md`",
	} {
		if !strings.Contains(agents, want) {
			t.Errorf("AGENTS.md 缺少 %q", want)
		}
	}
	if strings.Contains(agents, "## 沿用盤點") {
		t.Error("AGENTS.md 不該有沿用盤點：它放在 CARRYOVER.md")
	}
}

// 模組 AGENTS.md 只有 ADR 2026-10-28-agents-md-layering 的固定七節，依序排列；
// 行為規格與已知限制在 docs/modules/<模組>.md，不在每次都載入的指令檔。
func TestGeneratedAgentsHasOnlyTheFixedSections(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(scaffold(t, ""), "internal", "modules", "payment", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	agents := string(b)
	var got []string
	for _, line := range strings.Split(agents, "\n") {
		if strings.HasPrefix(line, "## ") {
			got = append(got, strings.TrimPrefix(line, "## "))
		}
	}
	want := []string{"職責", "擁有的資料", "發布的讀取介面", "模組內部結構", "不變量", "不負責", "決策紀錄"}
	if strings.Join(got, "｜") != strings.Join(want, "｜") {
		t.Errorf("AGENTS.md 的節是 %q，要是 %q", got, want)
	}
	if !strings.Contains(agents, "`docs/modules/payment.md`") {
		t.Error("AGENTS.md 要指向模組規格 docs/modules/payment.md")
	}
}

func TestRegisterAppendsTheModule(t *testing.T) {
	root := scaffold(t, "")
	src, err := os.ReadFile(filepath.Join(root, "cmd", "hawkr", "modules.go"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, `"`+arch.ModulePath+`/internal/modules/payment"`) {
		t.Error("沒有 import 新模組")
	}
	content := strings.Index(s, `{Name: "content"`)
	payment := strings.Index(s, `{Name: "payment", Dir: payment.Migrations()}`)
	if content < 0 || payment < content {
		t.Errorf("新模組要加在清單最後（migrate 依序套用）：\n%s", s)
	}
	graph, err := os.ReadFile(filepath.Join(root, "internal", "arch", "modules.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(graph), `"payment": {Packages: map[string][]string{"feature": {}, "model": {}, "store": {}}`) {
		t.Error("新模組沒有登記內部依賴圖")
	}
	deps, err := os.ReadFile(filepath.Join(root, "internal", "modules", "payment", "payment.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type Deps struct", "feature.New(feature.Deps{})"} {
		if !strings.Contains(string(deps), want) {
			t.Errorf("產生的 Deps 缺少 %q", want)
		}
	}
	if !strings.Contains(string(deps), "type Deps struct{}") || strings.Contains(string(deps), "internal/platform/db") || strings.Contains(string(deps), "uuid") {
		t.Error("空骨架不應預設資料庫、ID 或其他尚未使用的依賴")
	}
	model, err := os.ReadFile(filepath.Join(root, "internal", "modules", "payment", "internal", "model", "model.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(model), "type Deps") || strings.Contains(string(model), "internal/platform/db") {
		t.Error("model 不得擁有組裝依賴")
	}
	feature, err := os.ReadFile(filepath.Join(root, "internal", "modules", "payment", "internal", "feature", "feature.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(feature), "type Deps struct{}") || strings.Contains(string(feature), "model.Deps") {
		t.Error("子領域要宣告自己的最小 Deps")
	}
}

func TestRefusesUnknownOrExistingModules(t *testing.T) {
	root := scaffold(t, "")
	if err := newModule(root, data{Name: "payment", Module: arch.ModulePath}); err == nil {
		t.Error("已經存在的模組被覆寫了")
	}
	if err := newModule(root, data{Name: "nosuchmodule", Module: arch.ModulePath}); err == nil {
		t.Error("依賴表裡沒有的模組也產生了")
	}
	if err := newCharacterization(root, data{Name: "cart", Module: arch.ModulePath}); err == nil {
		t.Error("模組還沒建立就產生了特性測試")
	}
	if err := run([]string{"module", "Bad-Name"}); err == nil {
		t.Error("不合法的名稱被接受了")
	}
	if _, err := registerInternalGraph(filepath.Join(root, "internal", "arch", "modules.go"), data{Name: "payment"}); err == nil {
		t.Error("內部依賴圖接受了重複的模組 key")
	}
}
