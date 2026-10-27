package arch

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Package 是一個套件與它 import 的套件（含測試檔的 import）。
type Package struct {
	ImportPath string
	Imports    []string
}

// Violation 是一條違反依賴規則的紀錄。Import 為空代表違規的是套件本身的位置。
type Violation struct {
	Package string
	Import  string
	Rule    string
}

func (v Violation) String() string {
	if v.Import == "" {
		return fmt.Sprintf("%s：%s", v.Package, v.Rule)
	}
	return fmt.Sprintf("%s → %s：%s", v.Package, v.Import, v.Rule)
}

type layer int

const (
	layerExternal   layer = iota // 單體 module 以外：標準庫與第三方
	layerCmd                     // cmd/...：組裝點，可以接任何東西
	layerPlatform                // internal/platform/...
	layerModuleAPI               // internal/modules/<名稱>：模組的公開 API
	layerModuleImpl              // internal/modules/<名稱>/...：模組的實作
	layerLegacy                  // internal/legacy/...：遷移期的暫時轉接
	layerHTTPAPI                 // internal/httpapi/...：對外 HTTP 入口
	layerArch                    // internal/arch：本檢查
	layerUnknown
)

type location struct {
	layer  layer
	module string
}

func locate(importPath string) location {
	if importPath != ModulePath && !strings.HasPrefix(importPath, ModulePath+"/") {
		return location{layer: layerExternal}
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(importPath, ModulePath), "/"), "/")
	if parts[0] == "cmd" {
		return location{layer: layerCmd}
	}
	if len(parts) < 2 || parts[0] != "internal" {
		return location{layer: layerUnknown}
	}
	switch parts[1] {
	case "platform":
		return location{layer: layerPlatform}
	case "legacy":
		return location{layer: layerLegacy}
	case "httpapi":
		return location{layer: layerHTTPAPI}
	case "arch":
		return location{layer: layerArch}
	case "modules":
		switch len(parts) {
		case 2:
			return location{layer: layerUnknown}
		case 3:
			return location{layer: layerModuleAPI, module: parts[2]}
		default:
			return location{layer: layerModuleImpl, module: parts[2]}
		}
	}
	return location{layer: layerUnknown}
}

// Check 回傳 pkgs 違反依賴規則之處。modulesOnDisk 是 internal/modules 底下實際存在的目錄名。
func Check(pkgs []Package, deps map[string][]string, modulesOnDisk []string) []Violation {
	var out []Violation
	for _, m := range modulesOnDisk {
		if _, ok := deps[m]; !ok {
			out = append(out, Violation{Package: ModulePath + "/internal/modules/" + m, Rule: "模組目錄不在依賴表（internal/arch/modules.go）內"})
		}
	}
	for _, p := range pkgs {
		from := locate(p.ImportPath)
		if from.layer == layerUnknown {
			out = append(out, Violation{Package: p.ImportPath, Rule: "套件不在任何已知的層：cmd、internal/{platform,modules/<名稱>,legacy,httpapi,arch}"})
			continue
		}
		for _, imp := range p.Imports {
			to := locate(imp)
			if to.layer == layerExternal {
				continue
			}
			if rule := forbidden(from, to, deps); rule != "" {
				out = append(out, Violation{Package: p.ImportPath, Import: imp, Rule: rule})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// CheckInternal 檢查根套件與模組內部 package 的依賴圖。
func CheckInternal(pkgs []Package, graphs map[string]InternalGraph, modulesOnDisk []string) []Violation {
	var out []Violation
	packagesOnDisk := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		packagesOnDisk[p.ImportPath] = true
	}
	for _, m := range modulesOnDisk {
		if _, ok := graphs[m]; !ok {
			out = append(out, Violation{Package: ModulePath + "/internal/modules/" + m, Rule: "模組沒有宣告內部依賴圖"})
		}
	}
	for m, graph := range graphs {
		modulePath := ModulePath + "/internal/modules/" + m
		if !slices.Contains(modulesOnDisk, m) {
			out = append(out, Violation{Package: modulePath, Rule: "依賴圖宣告的模組目錄不存在"})
			continue
		}
		for node := range graph.Packages {
			path := modulePath + "/internal/" + node
			if !packagesOnDisk[path] {
				out = append(out, Violation{Package: path, Rule: "依賴圖宣告的內部 package 不存在"})
			}
		}
	}
	for _, p := range pkgs {
		from := locate(p.ImportPath)
		if from.layer != layerModuleAPI && from.layer != layerModuleImpl {
			continue
		}
		prefix := ModulePath + "/internal/modules/" + from.module + "/internal/"
		if from.layer == layerModuleImpl && !strings.HasPrefix(p.ImportPath, prefix) {
			out = append(out, Violation{Package: p.ImportPath, Rule: "模組實作 package 必須放在 internal/"})
			continue
		}
		graph, ok := graphs[from.module]
		if !ok {
			continue
		}
		fromNode := strings.TrimPrefix(p.ImportPath, prefix)
		if from.layer == layerModuleImpl {
			if _, ok := graph.Packages[fromNode]; !ok {
				out = append(out, Violation{Package: p.ImportPath, Rule: "內部 package 沒有宣告在依賴圖"})
				continue
			}
		}
		for _, imp := range p.Imports {
			if imp == p.ImportPath {
				continue
			}
			if from.layer == layerModuleImpl && imp == ModulePath+"/internal/modules/"+from.module {
				out = append(out, Violation{Package: p.ImportPath, Import: imp, Rule: "內部 package 不得反向依賴根套件"})
				continue
			}
			if !strings.HasPrefix(imp, prefix) {
				continue
			}
			toNode := strings.TrimPrefix(imp, prefix)
			if from.layer == layerModuleAPI {
				if toNode == "store" {
					out = append(out, Violation{Package: p.ImportPath, Import: imp, Rule: "模組根套件不得 import internal/store"})
				}
				continue
			}
			if !slices.Contains(graph.Packages[fromNode], toNode) {
				out = append(out, Violation{Package: p.ImportPath, Import: imp, Rule: "內部依賴圖不允許此方向"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// ValidateInternalGraphs 檢查圖的節點、邊與環。
func ValidateInternalGraphs(graphs map[string]InternalGraph) []string {
	var problems []string
	for m, graph := range graphs {
		for _, problem := range ValidateTable(graph.Packages) {
			problems = append(problems, m+"："+problem)
		}
	}
	sort.Strings(problems)
	return problems
}

func forbidden(from, to location, deps map[string][]string) string {
	switch from.layer {
	case layerCmd, layerArch:
		return ""
	case layerPlatform:
		if to.layer != layerPlatform {
			return "platform 只能依賴 platform"
		}
	case layerModuleAPI, layerModuleImpl:
		switch to.layer {
		case layerPlatform:
			return ""
		case layerModuleAPI, layerModuleImpl:
			if to.module == from.module {
				return ""
			}
			if to.layer == layerModuleImpl {
				return fmt.Sprintf("只能 import 模組 %s 的根套件（公開 API）", to.module)
			}
			if !slices.Contains(deps[from.module], to.module) {
				return fmt.Sprintf("依賴表不允許模組 %s 依賴 %s", from.module, to.module)
			}
			return ""
		case layerExternal, layerCmd, layerLegacy, layerHTTPAPI, layerArch, layerUnknown:
			return "模組只能依賴 platform 與依賴表允許的模組"
		default:
			return "模組只能依賴 platform 與依賴表允許的模組"
		}
	case layerLegacy:
		if to.layer != layerPlatform && to.layer != layerModuleAPI && to.layer != layerLegacy {
			return "legacy 只能依賴 platform、模組的公開 API 與 legacy"
		}
	case layerHTTPAPI:
		if to.layer != layerPlatform && to.layer != layerModuleAPI && to.layer != layerLegacy && to.layer != layerHTTPAPI {
			return "httpapi 只能依賴 platform、模組的公開 API、legacy 與 httpapi"
		}
	case layerExternal, layerUnknown:
	}
	return ""
}

// ValidateTable 檢查依賴表本身：依賴的對象必須存在、不得依賴自己、不得重複、不得有環。
func ValidateTable(deps map[string][]string) []string {
	var problems []string
	names := make([]string, 0, len(deps))
	for m := range deps {
		names = append(names, m)
	}
	sort.Strings(names)
	for _, m := range names {
		seen := map[string]bool{}
		for _, d := range deps[m] {
			switch {
			case d == m:
				problems = append(problems, fmt.Sprintf("%s 依賴自己", m))
			case seen[d]:
				problems = append(problems, fmt.Sprintf("%s 重複列出 %s", m, d))
			case !hasKey(deps, d):
				problems = append(problems, fmt.Sprintf("%s 依賴不存在的模組 %s", m, d))
			}
			seen[d] = true
		}
	}
	if cycle := findCycle(names, deps); cycle != nil {
		problems = append(problems, "依賴表有環："+strings.Join(cycle, " → "))
	}
	return problems
}

func hasKey(deps map[string][]string, k string) bool {
	_, ok := deps[k]
	return ok
}

func findCycle(names []string, deps map[string][]string) []string {
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var stack []string
	var visit func(string) []string
	visit = func(m string) []string {
		state[m] = visiting
		stack = append(stack, m)
		for _, d := range deps[m] {
			switch state[d] {
			case visiting:
				start := slices.Index(stack, d)
				return append(slices.Clone(stack[start:]), d)
			case unvisited:
				if !hasKey(deps, d) {
					continue
				}
				if c := visit(d); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[m] = done
		return nil
	}
	for _, m := range names {
		if state[m] == unvisited {
			if c := visit(m); c != nil {
				return c
			}
		}
	}
	return nil
}
