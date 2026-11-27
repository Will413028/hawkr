package arch

import (
	"slices"
	"testing"
)

func TestOnlyPlatformLLMImportsLLMClients(t *testing.T) {
	pkgs := listPackages(t, moduleRoot(t))
	violations, allowed := CheckRestrictedImports(pkgs, LLMClientImports)
	for _, v := range violations {
		t.Error(v)
	}
	seen := map[string]bool{}
	for _, p := range pkgs {
		if p.ImportPath != ModulePath+"/internal/platform/llm" {
			continue
		}
		for _, imp := range p.Imports {
			for _, r := range LLMClientImports.Paths {
				if underPath(imp, r) {
					seen[r] = true
				}
			}
		}
	}
	if allowed == 0 || len(seen) != len(LLMClientImports.Paths) {
		t.Fatalf("internal/platform/llm 只用到 %d/%d 條受限路徑（共 %d 處）：檢查範圍不對", len(seen), len(LLMClientImports.Paths), allowed)
	}
}

func TestCheckRestrictedImportsFlagsEveryImporterOutsideTheAllowList(t *testing.T) {
	rule := RestrictedImports{
		Paths:           []string{"example.com/sdk", "example.com/other/model"},
		AllowedPackages: []string{ModulePath + "/internal/platform/llm"},
		Why:             "test",
	}
	pkgs := []Package{
		{ImportPath: ModulePath + "/internal/platform/llm", Imports: []string{"example.com/sdk", "example.com/other/model/sub"}},
		{ImportPath: ModulePath + "/internal/platform/llm/fake", Imports: []string{"example.com/sdk/option"}},
		{ImportPath: ModulePath + "/internal/modules/assistant", Imports: []string{"example.com/sdk/option", "fmt"}},
		{ImportPath: ModulePath + "/internal/platform/llmx", Imports: []string{"example.com/other/model"}},
		{ImportPath: ModulePath + "/cmd/hawkr", Imports: []string{"example.com/sdkx", "example.com/other"}},
	}
	violations, allowed := CheckRestrictedImports(pkgs, rule)
	var got []string
	for _, v := range violations {
		got = append(got, v.Package+" "+v.Import)
	}
	want := []string{
		ModulePath + "/internal/modules/assistant example.com/sdk/option",
		ModulePath + "/internal/platform/llmx example.com/other/model",
	}
	if !slices.Equal(got, want) || allowed != 3 {
		t.Fatalf("violations = %q allowed = %d", got, allowed)
	}
}

func TestLLMClientImportsCoverModelSubpackagesButNotTheInterfacePackage(t *testing.T) {
	pkgs := []Package{
		{ImportPath: ModulePath + "/internal/modules/assistant", Imports: []string{
			"google.golang.org/adk/v2/model",
			"google.golang.org/adk/v2/model/apigee",
			"google.golang.org/adk/v2/model/anthropicmodel",
			"google.golang.org/genai",
		}},
	}
	violations, _ := CheckRestrictedImports(pkgs, LLMClientImports)
	var got []string
	for _, v := range violations {
		got = append(got, v.Import)
	}
	want := []string{"google.golang.org/adk/v2/model/anthropicmodel", "google.golang.org/adk/v2/model/apigee"}
	if !slices.Equal(got, want) {
		t.Fatalf("violations = %q", got)
	}
}
