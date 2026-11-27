package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)


var delegationUpstreamBranchExceptions = map[string]string{
	"EndCurrentDelegatedOperation": "失敗情境表只有 9280 一列；分流業務碼會永久封鎖結束動作",
}

var delegationLegacyGlobs = []string{"admin_delegation*.go", "admin_delegated_*.go"}

func delegationPackageSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("掃描範圍為空——目錄讀取失效時本閘門會靜默全綠")
	}
	return out
}

func delegationGrpcClientFields(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "holder.go", nil, 0)
	if err != nil {
		t.Fatalf("parse holder.go: %v", err)
	}
	out := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "Holder" {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range st.Fields.List {
			star, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			sel, ok := star.X.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "grpcclient" {
				continue
			}
			for _, name := range field.Names {
				out[name.Name] = true
			}
		}
		return false
	})
	if !out["Account"] || !out["Seller"] {
		t.Fatalf("從 holder.go 推導出的 gRPC client 欄位不含 Account／Seller（推到 %v）——解析八成失準了", delegationSortedKeys(out))
	}
	return out
}

func delegationUpstreamCallee(e ast.Expr, clients map[string]bool) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	client, ok := method.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	holder, ok := client.X.(*ast.Ident)
	if !ok || holder.Name != "h" {
		return ""
	}
	if !clients[client.Sel.Name] {
		return ""
	}
	return client.Sel.Name + "." + method.Sel.Name
}

func delegationMarkedFunc(fn *ast.FuncDecl, clients map[string]bool) bool {
	marked := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if marked {
			return false
		}
		switch node := n.(type) {
		case *ast.SelectorExpr:
			pkg, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch pkg.Name {
			case "bizcode":
				if strings.HasPrefix(node.Sel.Name, "Delegation") {
					marked = true
				}
			case "middleware":
				if node.Sel.Name == "DelegationSession" {
					marked = true
				}
			case "h":
				if node.Sel.Name == "orchestrator" {
					marked = true
				}
			}
		case *ast.Ident:
			switch node.Name {
			case "resolveDelegatedRequest", "denyDelegated", "bindDelegatedBody",
				"denialFromGrpcErr", "delegationGrpcFailure":
				marked = true
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if callee := delegationUpstreamCallee(call, clients); strings.Contains(callee, "Delegation") {
				marked = true
			}
		}
		return true
	})
	return marked
}

func delegationAssignsErr(a *ast.AssignStmt) bool {
	for _, lhs := range a.Lhs {
		if id, ok := lhs.(*ast.Ident); ok && id.Name == "err" {
			return true
		}
	}
	return false
}

func delegationIsErrNotNil(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	x, ok := bin.X.(*ast.Ident)
	if !ok || x.Name != "err" {
		return false
	}
	y, ok := bin.Y.(*ast.Ident)
	return ok && y.Name == "nil"
}

func delegationBranchFacts(body *ast.BlockStmt) (calls map[string]bool, flat9280 bool) {
	calls = map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			switch fn := node.Fun.(type) {
			case *ast.Ident:
				calls[fn.Name] = true
			case *ast.SelectorExpr:
				if pkg, ok := fn.X.(*ast.Ident); ok {
					calls[pkg.Name+"."+fn.Sel.Name] = true
				} else {
					calls[fn.Sel.Name] = true
				}
			}
		case *ast.SelectorExpr:
			if pkg, ok := node.X.(*ast.Ident); ok &&
				pkg.Name == "bizcode" && node.Sel.Name == "DelegationUpstreamUnavailable" {
				flat9280 = true
			}
		}
		return true
	})
	return calls, flat9280
}

type delegationUpstreamSite struct {
	file   string
	line   int
	fn     string
	callee string
	guard  *ast.BlockStmt
}

func TestDelegatedHandlers_UpstreamErrorBranchesRestoreBizCode(t *testing.T) {
	clients := delegationGrpcClientFields(t)
	fset := token.NewFileSet()
	var sites []delegationUpstreamSite
	markedFiles := map[string]bool{}

	for _, path := range delegationPackageSources(t) {
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !delegationMarkedFunc(fn, clients) {
				continue
			}
			markedFiles[path] = true
			collectDelegationUpstreamSites(fset, path, fn, clients, &sites)
		}
	}

	for _, pattern := range delegationLegacyGlobs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, p := range matches {
			if strings.HasSuffix(p, "_test.go") || markedFiles[p] {
				continue
			}
			t.Errorf("%s 命中代行檔名樣式，卻沒有任何函式被認定為代行 —— "+
				"delegationMarkedFunc 的標記判定八成失準了，此時的綠燈是假的", p)
		}
	}

	const minSites = 23
	if len(sites) < minSites {
		t.Fatalf("只掃到 %d 個上游呼叫點（< %d）——比對器八成失準了，此時的綠燈是假的", len(sites), minSites)
	}

	usedExceptions := map[string]bool{}
	for _, s := range sites {
		if s.guard == nil {
			t.Errorf("%s:%d %s：%s 的 err 沒有任何 `if err != nil` 檢查", s.file, s.line, s.fn, s.callee)
			continue
		}
		calls, flat9280 := delegationBranchFacts(s.guard)
		if _, exempt := delegationUpstreamBranchExceptions[s.fn]; exempt {
			usedExceptions[s.fn] = true
			continue
		}
		if flat9280 {
			t.Errorf("%s:%d %s：%s 的 err 分支直接寫 bizcode.DelegationUpstreamUnavailable —— "+
				"上游的業務碼是夾在 gRPC status message 的 \"[NNNN] \" 前綴裡回來的，"+
				"壓成 9280 會把「工作階段不存在」講成「服務暫時不可用，請稍後再試」",
				s.file, s.line, s.fn, s.callee)
			continue
		}
		if !calls["delegationGrpcFailure"] && !calls["denialFromGrpcErr"] {
			t.Errorf("%s:%d %s：%s 的 err 分支沒有經過 delegationGrpcFailure 或 denialFromGrpcErr —— "+
				"這兩支是把 \"[NNNN] \" 前綴還原成 envelope code 的唯二出口（實際呼叫到：%v）",
				s.file, s.line, s.fn, s.callee, sortedCallNames(calls))
		}
	}

	for fn := range delegationUpstreamBranchExceptions {
		if !usedExceptions[fn] {
			t.Errorf("例外 %q 已不對應任何上游呼叫點，請一併移除", fn)
		}
	}

	t.Logf("掃過 %d 個上游呼叫點（%d 個檔），具名例外 %d 個；納管的 gRPC client %d 個",
		len(sites), len(markedFiles), len(usedExceptions), len(clients))
}

func collectDelegationUpstreamSites(fset *token.FileSet, path string, fn *ast.FuncDecl, clients map[string]bool, out *[]delegationUpstreamSite) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			if ifStmt, ok := stmt.(*ast.IfStmt); ok && ifStmt.Init != nil {
				if assign, ok := ifStmt.Init.(*ast.AssignStmt); ok && delegationAssignsErr(assign) {
					if callee := delegationUpstreamCalleeOf(assign, clients); callee != "" {
						site := delegationUpstreamSite{
							file: path, line: fset.Position(assign.Pos()).Line,
							fn: fn.Name.Name, callee: callee,
						}
						if delegationIsErrNotNil(ifStmt.Cond) {
							site.guard = ifStmt.Body
						}
						*out = append(*out, site)
					}
				}
				continue
			}
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok || !delegationAssignsErr(assign) {
				continue
			}
			callee := delegationUpstreamCalleeOf(assign, clients)
			if callee == "" {
				continue
			}
			site := delegationUpstreamSite{
				file: path, line: fset.Position(assign.Pos()).Line,
				fn: fn.Name.Name, callee: callee,
			}
			if i+1 < len(block.List) {
				if ifStmt, ok := block.List[i+1].(*ast.IfStmt); ok &&
					ifStmt.Init == nil && delegationIsErrNotNil(ifStmt.Cond) {
					site.guard = ifStmt.Body
				}
			}
			*out = append(*out, site)
		}
		return true
	})
}

func delegationUpstreamCalleeOf(a *ast.AssignStmt, clients map[string]bool) string {
	for _, rhs := range a.Rhs {
		if callee := delegationUpstreamCallee(rhs, clients); callee != "" {
			return callee
		}
	}
	return ""
}

func sortedCallNames(calls map[string]bool) []string {
	return delegationSortedKeys(calls)
}

func delegationSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
