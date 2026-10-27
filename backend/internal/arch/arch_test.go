package arch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestRepositoryConformsToArchitecture(t *testing.T) {
	root := moduleRoot(t)
	pkgs := listPackages(t, root)
	onDisk := modulesOnDisk(t, root)
	for _, v := range Check(pkgs, Dependencies, onDisk) {
		t.Error(v)
	}
	for _, v := range CheckInternal(pkgs, InternalDependencies, onDisk) {
		t.Error(v)
	}
}

var (
	listPackagesOnce sync.Once
	listedRoot       string
	listedPackages   []Package
	listErr          error
	listOutput       []byte
)

// listPackages 列出單體的每個套件與它的 import（含測試的 import）。
func listPackages(t *testing.T, root string) []Package {
	t.Helper()
	listPackagesOnce.Do(func() {
		listedRoot = root
		cmd := exec.CommandContext(t.Context(), "go", "list", "-f",
			`{{.ImportPath}}{{range .Imports}} {{.}}{{end}}{{range .TestImports}} {{.}}{{end}}{{range .XTestImports}} {{.}}{{end}}`,
			"./...")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOWORK=off")
		listOutput, listErr = cmd.CombinedOutput()
		if listErr != nil {
			return
		}
		for _, line := range strings.Split(strings.TrimSpace(string(listOutput)), "\n") {
			fields := strings.Fields(line)
			listedPackages = append(listedPackages, Package{ImportPath: fields[0], Imports: fields[1:]})
		}
	})
	if root != listedRoot {
		t.Fatalf("go list 的根目錄從 %s 變成 %s", listedRoot, root)
	}
	if listErr != nil {
		t.Fatalf("go list 失敗：%v\n%s", listErr, listOutput)
	}
	// 掃到零個套件也會「全部通過」，所以先確認至少看得到這個套件自己。
	if !containsPackage(listedPackages, ModulePath+"/internal/arch") {
		t.Fatalf("go list 沒有列出 internal/arch，檢查範圍不對：\n%s", listOutput)
	}
	return listedPackages
}

// codemetaImporters 是還在 import internal/platform/codemeta 的套件。CodeMeta 凍結、隨模組搬遷退場
// （ADR 2026-09-25-codemeta-retirement 過渡第 1 步），名單只准變短：新的 import 會失敗，不再 import 的
// 也要從名單刪掉。遷移計畫 A2 收尾時名單清空（media 的上傳者改成模組自己的型別，舊的帳號類型由
// internal/legacy/mediaservice 換算）；生成物留到階段 C 隨 CodeMeta 一起刪除。
var codemetaImporters []string

func TestCodemetaImportsOnlyShrink(t *testing.T) {
	codemeta := ModulePath + "/internal/platform/codemeta"
	var got []string
	for _, p := range listPackages(t, moduleRoot(t)) {
		if p.ImportPath != codemeta && slices.Contains(p.Imports, codemeta) {
			got = append(got, p.ImportPath)
		}
	}
	for _, p := range got {
		if !slices.Contains(codemetaImporters, p) {
			t.Errorf("%s 新 import 了 internal/platform/codemeta：列舉改用 proto enum 或模組的 Go 型別，錯誤原因用 apperr.Reason", p)
		}
	}
	for _, p := range codemetaImporters {
		if !slices.Contains(got, p) {
			t.Errorf("%s 已經不再 import internal/platform/codemeta，把它從 codemetaImporters 刪掉", p)
		}
	}
}

func TestDependencyTableIsValid(t *testing.T) {
	for _, p := range ValidateTable(Dependencies) {
		t.Error(p)
	}
	for _, p := range ValidateInternalGraphs(InternalDependencies) {
		t.Error(p)
	}
}

func TestInternalGraphCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/internal-graph/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name        string
		Packages    []Package
		Graphs      map[string]InternalGraph
		Modules     []string
		MissingNode string
		Want        string
		Valid       bool
		ExactCount  int
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			pkgs := slices.Clone(c.Packages)
			for module, graph := range c.Graphs {
				for node := range graph.Packages {
					path := ModulePath + "/internal/modules/" + module + "/internal/" + node
					if node != c.MissingNode && !containsPackage(pkgs, path) {
						pkgs = append(pkgs, Package{ImportPath: path})
					}
				}
			}
			var got []string
			for _, v := range CheckInternal(pkgs, c.Graphs, c.Modules) {
				got = append(got, v.String())
			}
			got = append(got, ValidateInternalGraphs(c.Graphs)...)
			if c.Valid && len(got) != 0 {
				t.Fatalf("合法的依賴被判為違規：%v", got)
			}
			if c.ExactCount > 0 && len(got) != c.ExactCount {
				t.Fatalf("預期 %d 項違規，實際：%v", c.ExactCount, got)
			}
			if !c.Valid && !anyContains(got, c.Want) {
				t.Fatalf("預期出現「%s」，實際：%v", c.Want, got)
			}
		})
	}
}

func TestCheckFlagsViolations(t *testing.T) {
	const m = ModulePath + "/internal/modules/"
	deps := map[string][]string{"low": {}, "high": {"low"}}
	cases := []struct {
		name    string
		pkg     Package
		onDisk  []string
		wantHit string
	}{
		{"下層依賴上層", Package{m + "low", []string{m + "high"}}, nil, "依賴表不允許模組 low 依賴 high"},
		{"import 別的模組的子套件", Package{m + "high", []string{m + "low/store"}}, nil, "只能 import 模組 low 的根套件"},
		{"模組依賴 legacy", Package{m + "high/app", []string{ModulePath + "/internal/legacy/lowgrpc"}}, nil, "模組只能依賴 platform"},
		{"模組依賴 httpapi", Package{m + "high", []string{ModulePath + "/internal/httpapi"}}, nil, "模組只能依賴 platform"},
		{"platform 依賴模組", Package{ModulePath + "/internal/platform/db", []string{m + "low"}}, nil, "platform 只能依賴 platform"},
		{"legacy 依賴 httpapi", Package{ModulePath + "/internal/legacy/lowgrpc", []string{ModulePath + "/internal/httpapi"}}, nil, "legacy 只能依賴"},
		{"httpapi 依賴模組實作", Package{ModulePath + "/internal/httpapi", []string{m + "low/store"}}, nil, "httpapi 只能依賴"},
		{"套件不在已知的層", Package{ModulePath + "/internal/util", nil}, nil, "套件不在任何已知的層"},
		{"模組目錄不在依賴表", Package{ModulePath + "/cmd/hawkr", nil}, []string{"ghost"}, "模組目錄不在依賴表"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := violationStrings(Check([]Package{c.pkg}, deps, c.onDisk))
			if !anyContains(got, c.wantHit) {
				t.Fatalf("預期出現「%s」，實際：%v", c.wantHit, got)
			}
		})
	}

	allowed := []Package{
		{m + "high", []string{m + "low", m + "high/store", ModulePath + "/internal/platform/db", "context"}},
		{m + "high/store", []string{ModulePath + "/internal/platform/db"}},
		{ModulePath + "/internal/legacy/lowgrpc", []string{m + "low", ModulePath + "/internal/platform/db"}},
		{ModulePath + "/internal/httpapi", []string{m + "high", ModulePath + "/internal/legacy/lowgrpc"}},
		{ModulePath + "/cmd/hawkr", []string{m + "low/store", ModulePath + "/internal/legacy/lowgrpc"}},
	}
	if got := Check(allowed, deps, []string{"low", "high"}); len(got) != 0 {
		t.Fatalf("合法的依賴被判為違規：%v", got)
	}
}

func TestValidateTableFlagsProblems(t *testing.T) {
	cases := []struct {
		name    string
		deps    map[string][]string
		wantHit string
	}{
		{"環", map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}, "依賴表有環"},
		{"依賴自己", map[string][]string{"a": {"a"}}, "a 依賴自己"},
		{"不存在的模組", map[string][]string{"a": {"ghost"}}, "a 依賴不存在的模組 ghost"},
		{"重複", map[string][]string{"a": {"b", "b"}, "b": {}}, "a 重複列出 b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ValidateTable(c.deps)
			if !anyContains(got, c.wantHit) {
				t.Fatalf("預期出現「%s」，實際：%v", c.wantHit, got)
			}
		})
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("找不到 go.mod")
		}
		dir = parent
	}
}

func modulesOnDisk(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "internal", "modules"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func containsPackage(pkgs []Package, path string) bool {
	for _, p := range pkgs {
		if p.ImportPath == path {
			return true
		}
	}
	return false
}

func anyContains(items []string, sub string) bool {
	for _, s := range items {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func violationStrings(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.String()
	}
	return out
}
