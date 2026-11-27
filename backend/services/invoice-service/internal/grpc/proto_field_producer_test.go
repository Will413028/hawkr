package grpc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var knownEmptyFields = map[string]string{
	"InvoiceAuditLogItem.actor_display_name": "設計上恆為空：accounts 表沒有任何名字欄，GetAccountInfoBatch 的 nickname 在批次路徑上結構性恆空；不得用 email 頂替",
	"InvoiceAuditLogItem.result":             "設計上恆為空：invoice_admin_audit_logs 無此欄；填 SUCCESS 會讓失敗的操作看起來都成功",
	"InvoiceAuditLogItem.terms_version":      "設計上恆為空：invoice_admin_audit_logs 無此欄；只對條款相關 action 有意義，猜當期版本會讓每一列都像簽過同一版",
	"SelfIssueAttestationInfo.terms_key":     "設計上恆為空：條款全文尚未發布成 media 資產；猜一個 key 會產生 404 的下載連結",
	"SellerInvoiceDocumentDetail.created_by": "設計上恆為空：來源是 coupon 的 created_by，本服務無 coupon client；同 message 的 funded_by 已用 UNDETERMINED 承擔 fail-closed（§3.7 D9a）",

	"TaxDocumentInfo.kind": "設計上恆為空：上傳端只收 objectKey 純字串陣列，DB 與 media metadata 皆無分類欄；填 OTHER 會讓 ADM 以為賣家分過類",
}

func TestEveryResponseFieldHasAProducer(t *testing.T) {
	protoPath := filepath.Join("..", "..", "..", "..", "api", "proto", "invoice", "invoice.proto")
	raw, err := os.ReadFile(protoPath) //nolint:gosec // 固定的 repo 內相對路徑
	if err != nil {
		t.Fatalf("讀不到 proto（本測試已經在守一個不存在的東西）：%v", err)
	}
	proto := string(raw)

	messages := parseMessages(proto)
	if len(messages) == 0 {
		t.Fatal("一個 message 都沒解到 —— proto 格式變了，本測試已失效")
	}
	reachable := reachableFromRPCOutputs(proto, messages)
	if len(reachable) < 50 {
		t.Fatalf("只走訪到 %d 個 response message，遠少於預期 —— 解析壞了，"+
			"而壞掉的症狀是「什麼都沒查到」＝ 全綠", len(reachable))
	}

	idx := buildProducerIndex(t)

	var offenders []string
	var undecidable []string
	used := map[string]bool{}
	for _, msg := range sortedKeys(reachable) {
		for _, f := range protoFields(messages[msg]) {
			key := msg + "." + f
			goField := goFieldName(f)
			produced := idx.has(msg, goField)
			_, registered := knownEmptyFields[key]
			undecided := !produced && !registered && idx.unresolvedAssign[goField]
			if undecided {
				undecidable = append(undecidable, key)
			}
			if registered {
				used[key] = true
				if produced {
					offenders = append(offenders,
						key+"（已登記為「沒有生產者」，但實際上有人填了 ⇒ 移除該登記）")
				}
				continue
			}
			if produced || undecided {
				continue
			}
			offenders = append(offenders, key)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("以下 response 欄位在 internal/ 內**沒有任何賦值**⇒ 前端一定拿到零值。\n"+
			"這是 #3017／#3019 的缺陷類：畫面上看起來是「這筆沒有資料」，實際是「這個功能沒做」。\n"+
			"修法二選一：把它填上，或登記進 knownEmptyFields 並寫下理由（設計上恆為空／KNOWN GAP #票號）。\n  %s",
			strings.Join(offenders, "\n  "))
	}

	if len(undecidable) > 0 {
		t.Errorf("以下 response 欄位**只**被「接收者型別無法靜態解析」的指派命中"+
			"（`a.b.Field =`／`out[i].Field =`／宣告在本函式之外）⇒ 本 gate 無法判定它有沒有生產者。\n"+
			"看不到就要報錯，不是看不到就放行。修法三選一：改成複合字面量、把接收者的宣告移進同一函式、"+
			"或確認它確實沒有生產者後登記進 knownEmptyFields。\n  %s",
			strings.Join(undecidable, "\n  "))
	}

	for key := range knownEmptyFields {
		if !used[key] {
			t.Errorf("knownEmptyFields 的 %q 已經不成立（欄位被刪掉、或已經有人填了）⇒ 移除該條目", key)
		}
	}
}

var (
	reMessage = regexp.MustCompile(`(?ms)^message\s+(\w+)\s*\{(.*?)^\}`)
	reRPCOut  = regexp.MustCompile(`rpc\s+\w+\s*\([^)]*\)\s*returns\s*\(\s*([\w.]+)\s*\)`)
	reField   = regexp.MustCompile(`^(?:repeated\s+|optional\s+)?(?:map<[^>]+>|[\w.]+)\s+(\w+)\s*=\s*\d+\s*;`)
	reFType   = regexp.MustCompile(`^(?:repeated\s+|optional\s+)?([\w.]+)\s+\w+\s*=\s*\d+\s*;`)
	reMapVal  = regexp.MustCompile(`^map<[^,]+,\s*([\w.]+)>\s+\w+\s*=`)
)

func parseMessages(proto string) map[string]string {
	out := map[string]string{}
	for _, m := range reMessage.FindAllStringSubmatch(proto, -1) {
		out[m[1]] = m[2]
	}
	return out
}

func stripComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

func protoFields(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if m := reField.FindStringSubmatch(stripComment(line)); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

func nestedTypes(body string, messages map[string]string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		l := stripComment(line)
		if m := reMapVal.FindStringSubmatch(l); m != nil {
			if _, ok := messages[m[1]]; ok {
				out = append(out, m[1])
			}
			continue
		}
		if m := reFType.FindStringSubmatch(l); m != nil {
			if _, ok := messages[m[1]]; ok {
				out = append(out, m[1])
			}
		}
	}
	return out
}

func reachableFromRPCOutputs(proto string, messages map[string]string) map[string]bool {
	seen := map[string]bool{}
	var stack []string
	for _, m := range reRPCOut.FindAllStringSubmatch(proto, -1) {
		stack = append(stack, m[1])
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		body, ok := messages[n]
		if !ok {
			continue
		}
		seen[n] = true
		stack = append(stack, nestedTypes(body, messages)...)
	}
	return seen
}

func goFieldName(snake string) string {
	parts := strings.Split(snake, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

type producerIndex struct {
	produced         map[string]bool
	unresolvedAssign map[string]bool
}

func (idx producerIndex) has(msgType, goField string) bool {
	return idx.produced[msgType+"."+goField]
}

func buildProducerIndex(t *testing.T) producerIndex {
	t.Helper()
	idx := producerIndex{produced: map[string]bool{}, unresolvedAssign: map[string]bool{}}
	fset := token.NewFileSet()
	var files []*ast.File
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("解析 %s 失敗（本 gate 對這個檔完全失明）：%v", path, perr)
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		t.Fatalf("走訪 internal/ 失敗：%v", err)
	}
	if len(files) < 30 {
		t.Fatalf("只讀到 %d 個 .go 檔，遠少於預期 —— 走訪範圍壞了，"+
			"而壞掉的症狀是「每個欄位都查不到賦值」＝ 全紅，或路徑錯掉時全綠", len(files))
	}

	returns := collectFuncReturnTypes(files)
	for _, f := range files {
		collectCompositeLiterals(f, idx.produced)
		collectAssignments(f, returns, idx.produced, idx.unresolvedAssign)
	}
	return idx
}

func collectCompositeLiterals(f *ast.File, produced map[string]bool) {
	ast.Inspect(f, func(node ast.Node) bool {
		cl, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		walkCompositeLit(cl, typeNameOf(cl.Type), produced)
		return true
	})
}

func walkCompositeLit(cl *ast.CompositeLit, typeHint string, produced map[string]bool) {
	elemHint := ""
	switch tt := cl.Type.(type) {
	case *ast.ArrayType:
		elemHint = typeNameOf(tt.Elt)
	case *ast.MapType:
		elemHint = typeNameOf(tt.Value)
	}
	self := typeNameOf(cl.Type)
	if self == "" {
		self = typeHint
	}
	for _, e := range cl.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && self != "" {
				produced[self+"."+id.Name] = true
			}
			if inner, ok := kv.Value.(*ast.CompositeLit); ok && inner.Type == nil && elemHint != "" {
				walkCompositeLit(inner, elemHint, produced)
			}
			continue
		}
		if inner, ok := e.(*ast.CompositeLit); ok && inner.Type == nil && elemHint != "" {
			walkCompositeLit(inner, elemHint, produced)
		}
	}
}

func collectAssignments(f *ast.File, returns map[string]string, produced, unresolved map[string]bool) {
	ast.Inspect(f, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		local := localVarTypes(fn, returns)
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			as, ok := m.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range as.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok {
					unresolved[sel.Sel.Name] = true
					continue
				}
				if typ, ok := local[id.Name]; ok {
					produced[typ+"."+sel.Sel.Name] = true
					continue
				}
				unresolved[sel.Sel.Name] = true
			}
			return true
		})
		return true
	})
}

func localVarTypes(fn *ast.FuncDecl, returns map[string]string) map[string]string {
	out := map[string]string{}
	add := func(name string, e ast.Expr) {
		if name == "" || name == "_" {
			return
		}
		if t := typeNameOf(e); t != "" {
			out[name] = t
		}
	}
	addRHS := func(name string, e ast.Expr) {
		if name == "" || name == "_" {
			return
		}
		if te := rhsTypeExpr(e); te != nil {
			add(name, te)
			return
		}
		if t := callReturnType(e, returns); t != "" {
			out[name] = t
		}
	}
	for _, fl := range []*ast.FieldList{fn.Recv, fn.Type.Params} {
		if fl == nil {
			continue
		}
		for _, fld := range fl.List {
			for _, nm := range fld.Names {
				add(nm.Name, fld.Type)
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.DeclStmt:
			gd, ok := v.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nm := range vs.Names {
					if vs.Type != nil {
						add(nm.Name, vs.Type)
					} else if i < len(vs.Values) {
						addRHS(nm.Name, vs.Values[i])
					}
				}
			}
		case *ast.AssignStmt:
			if v.Tok != token.DEFINE {
				return true
			}
			for i, l := range v.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok || i >= len(v.Rhs) {
					continue
				}
				addRHS(id.Name, v.Rhs[i])
			}
		}
		return true
	})
	return out
}

func collectFuncReturnTypes(files []*ast.File) map[string]string {
	out := map[string]string{}
	conflict := map[string]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
				continue
			}
			t := typeNameOf(fn.Type.Results.List[0].Type)
			if t == "" {
				continue
			}
			if prev, seen := out[fn.Name.Name]; seen && prev != t {
				conflict[fn.Name.Name] = true
				continue
			}
			out[fn.Name.Name] = t
		}
	}
	for name := range conflict {
		delete(out, name)
	}
	return out
}

func callReturnType(e ast.Expr, returns map[string]string) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	var name string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	default:
		return ""
	}
	return returns[name]
}

func rhsTypeExpr(e ast.Expr) ast.Expr {
	switch v := e.(type) {
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return rhsTypeExpr(v.X)
		}
	case *ast.CompositeLit:
		return v.Type
	case *ast.CallExpr:
		if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "new" && len(v.Args) == 1 {
			return v.Args[0]
		}
	}
	return nil
}

func typeNameOf(e ast.Expr) string {
	switch v := e.(type) {
	case nil:
		return ""
	case *ast.StarExpr:
		return typeNameOf(v.X)
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
