package arch

import (
	"slices"
	"sort"
	"strings"
)

// RestrictedImports 規定一組外部套件（含子套件）只能被哪些套件 import。
type RestrictedImports struct {
	// Paths 是受限的 import 路徑；路徑本身與它的子套件都算。
	Paths []string
	// AllowedPackages 是可以 import 它們的套件（含子套件），以 import 路徑表示。
	AllowedPackages []string
	// Why 說明規則，放進違規訊息。
	Why string
}

// LLMClientImports 守住單一 LLM client 建構點：ADK 的 model 實作與 openai SDK 只有 internal/platform/llm 能 import（genai 的型別如 Content、Part
// 是 ADK agent、runner、tool 的介面，各處可以 import，建構 client 的 genai.NewClient 由 LLMClientCalls 守），
// 其他套件從那裡拿已決定好開通與否的 Model，不自己建 client，也就不會漏掉 provider 的 request quirk 或讓缺金鑰擋住啟動。
var LLMClientImports = RestrictedImports{
	Paths: []string{
		"google.golang.org/adk/v2/model/gemini",
		"google.golang.org/adk/v2/model/openaimodel",
		"github.com/openai/openai-go/v3",
	},
	AllowedPackages: []string{ModulePath + "/internal/platform/llm"},
	Why:             "LLM client 只能由 internal/platform/llm 建構（A9-D10、AD5）",
}

func underPath(importPath, root string) bool {
	return importPath == root || strings.HasPrefix(importPath, root+"/")
}

// CheckRestrictedImports 回傳不在允許名單內卻 import 受限路徑的套件，並回傳在允許名單內看到的受限 import 的次數，
// 供測試確認掃描不是空轉。
func CheckRestrictedImports(pkgs []Package, rule RestrictedImports) (violations []Violation, allowed int) {
	for _, p := range pkgs {
		ok := slices.ContainsFunc(rule.AllowedPackages, func(a string) bool { return underPath(p.ImportPath, a) })
		for _, imp := range p.Imports {
			if !slices.ContainsFunc(rule.Paths, func(r string) bool { return underPath(imp, r) }) {
				continue
			}
			if ok {
				allowed++
				continue
			}
			violations = append(violations, Violation{Package: p.ImportPath, Import: imp, Rule: rule.Why})
		}
	}
	sort.Slice(violations, func(i, j int) bool { return violations[i].String() < violations[j].String() })
	return violations, allowed
}
