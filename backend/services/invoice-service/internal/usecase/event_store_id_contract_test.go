package usecase

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)


type wireAxis struct {
	Tag      string
	Type     string
	Ctors    []string
	GoNames  []string
	Required bool
	Impact   string
}

var wireAxes = []wireAxis{
	{
		Tag: "store_id", Type: "eventwire.StoreID",
		Ctors:    []string{"eventwire.StoreIDOf"},
		GoNames:  []string{"StoreID", "StoreId"},
		Required: true,
		Impact: "invoice_issuers.id 是 TSID、issuer_ref_id 才是 sellers.store_id。" +
			"填錯或不填不會報錯，只會讓 outbox dispatcher 的 withNotifyRouting 解不出賣家 ⇒ " +
			"notify_seller_account_id 恆為 \"0\" ⇒ 賣家向通知靜默 0 投遞（零錯誤零 DLQ 零 metric）",
	},
	{
		Tag: "buyer_id", Type: "eventwire.BuyerID",
		Ctors:    []string{"eventwire.BuyerIDOfDocument", "eventwire.BuyerIDOfComplaint"},
		GoNames:  []string{"BuyerID", "BuyerId"},
		Required: false,
		Impact: "invoice_documents.recipient_ref_id 是多義欄位：BUYER ⇒ buyers.buyer_id、" +
			"SELLER ⇒ sellers.store_id、EXTERNAL ⇒ NULL。不判 recipient_type 就填 ⇒ " +
			"方向二／四的事件把一個 store_id 裝進 buyer_id ⇒ §6.3.9 模板的 target_user_field " +
			"正是它 ⇒ 通知投遞到錯的人或無人",
	},
}

var wireCtorParamTypes = map[string]string{
	"eventwire.StoreIDOf":          "domain.Issuer",
	"eventwire.BuyerIDOfDocument":  "domain.Document",
	"eventwire.BuyerIDOfComplaint": "domain.Complaint",
}

var snakeCaseRe = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

type payloadViolation struct {
	Pos    string
	Struct string
	Field  string
	Rule   string
	Detail string
}

func (v payloadViolation) String() string {
	return v.Pos + " " + v.Struct + "." + v.Field + " [" + v.Rule + "] " + v.Detail
}

type gateReport struct {
	Violations   []payloadViolation
	PayloadTypes []string
	OutboxTypes  []string
	WritePoints  map[string]int
}

func checkEventPayloads(fset *token.FileSet, files []*ast.File) gateReport {
	rep := gateReport{WritePoints: map[string]int{}}
	add := func(pos token.Pos, structName, field, rule, detail string) {
		rep.Violations = append(rep.Violations, payloadViolation{
			Pos: fset.Position(pos).String(), Struct: structName, Field: field, Rule: rule, Detail: detail,
		})
	}

	structDecls := map[string]*ast.StructType{}
	funcDecls := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				funcDecls[decl.Name.Name] = decl
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if st, ok := ts.Type.(*ast.StructType); ok {
						structDecls[ts.Name.Name] = st
					}
				}
			}
		}
	}

	payloadTypes := map[string]*ast.StructType{}
	markType := func(name string) {
		if st, ok := structDecls[name]; ok {
			payloadTypes[name] = st
		}
	}
	for name := range structDecls {
		if strings.HasSuffix(name, "Event") {
			markType(name)
		}
	}
	for name, st := range structDecls {
		for _, fld := range st.Fields.List {
			tag := jsonTagOf(fld)
			for _, ax := range wireAxes {
				if tag == ax.Tag {
					markType(name)
				}
				for _, gn := range ax.GoNames {
					if len(fld.Names) > 0 && fld.Names[0].Name == gn && fld.Tag != nil {
						markType(name)
					}
				}
			}
		}
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isJSONMarshal(call.Fun) || len(call.Args) == 0 {
				return true
			}
			if cl, ok := call.Args[0].(*ast.CompositeLit); ok {
				if id, ok := cl.Type.(*ast.Ident); ok {
					markType(id.Name)
				}
			}
			return true
		})
	}
	for n := range payloadTypes {
		rep.PayloadTypes = append(rep.PayloadTypes, n)
	}
	sort.Strings(rep.PayloadTypes)

	trackedFields := map[string]map[string]wireAxis{}

	for _, name := range rep.PayloadTypes {
		st := payloadTypes[name]
		for _, fld := range st.Fields.List {
			goName := "?"
			if len(fld.Names) > 0 {
				goName = fld.Names[0].Name
			}
			tag := jsonTagOf(fld)
			declared := exprText(fld.Type)

			if tag != "" && tag != "-" && !snakeCaseRe.MatchString(tag) {
				add(fld.Pos(), name, goName, "R1",
					"json tag "+strconv.Quote(tag)+" 不是 snake_case。§6.3.4 業務欄位一律 snake_case；"+
						"camelCase 的直接後果是下游拿不到 payload[\"store_id\"] ⇒ 賣家向通知靜默 0 投遞")
			}

			for _, ax := range wireAxes {
				isAxisTag := tag == ax.Tag
				isAxisGoName := false
				for _, gn := range ax.GoNames {
					if goName == gn {
						isAxisGoName = true
					}
				}
				switch {
				case isAxisTag:
					if declared != ax.Type {
						add(fld.Pos(), name, goName, "R2",
							"wire 欄位 "+strconv.Quote(ax.Tag)+" 的型別是 "+strconv.Quote(declared)+
								"，必須是 "+ax.Type+"。\n"+
								"宣告成 string 的話任何字串都填得進去，整道保證退回「靠檢查」——"+
								"而檢查漏過「事後賦值」整整一輪。後果："+ax.Impact)
						continue
					}
					if trackedFields[name] == nil {
						trackedFields[name] = map[string]wireAxis{}
					}
					trackedFields[name][goName] = ax
				case isAxisGoName && fld.Tag != nil:
					add(fld.Pos(), name, goName, "R2",
						"欄位叫 "+goName+" 卻掛 json tag "+strconv.Quote(tag)+
							"，必須是 "+strconv.Quote(ax.Tag)+"。後果："+ax.Impact)
				case declared == ax.Type:
					add(fld.Pos(), name, goName, "R2",
						"欄位型別是 "+ax.Type+" 卻掛 json tag "+strconv.Quote(tag)+
							"，必須是 "+strconv.Quote(ax.Tag))
				}
			}
		}
	}

	litCount := map[string]int{}
	for _, f := range files {
		for _, topDecl := range f.Decls {
			enclosing, _ := topDecl.(*ast.FuncDecl)
			ast.Inspect(topDecl, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				id, ok := cl.Type.(*ast.Ident)
				if !ok {
					return true
				}
				fields, tracked := trackedFields[id.Name]
				if !tracked {
					return true
				}
				litCount[id.Name]++

				seen := map[string]bool{}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						add(elt.Pos(), id.Name, "?", "R3",
							"事件 payload 不得使用位置參數複合字面量（無法驗證哪個值進了哪個欄位）")
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					ax, isTracked := fields[key.Name]
					if !isTracked {
						continue
					}
					seen[key.Name] = true
					rep.WritePoints[id.Name+"."+key.Name]++
					ctor, approved := approvedCtorOf(kv.Value, ax.Ctors)
					if !approved {
						add(kv.Pos(), id.Name, key.Name, "R3",
							strconv.Quote(ax.Tag)+" 的值必須是 "+strings.Join(ax.Ctors, " / ")+
								" 的回傳，實際是 `"+exprText(kv.Value)+"`。後果："+ax.Impact)
						continue
					}
					if detail, bad := ctorArgProvenance(enclosing, kv.Value.(*ast.CallExpr), ctor); bad {
						add(kv.Pos(), id.Name, key.Name, "R8", detail+"後果："+ax.Impact)
					}
				}
				for goName, ax := range fields {
					if !seen[goName] {
						add(cl.Pos(), id.Name, goName, "R4",
							"複合字面量沒有顯式設定 "+goName+"（wire 欄 "+strconv.Quote(ax.Tag)+"）。"+
								"零值不是合法值：eventwire 的 MarshalJSON 會回 error 讓整筆交易失敗。後果："+ax.Impact)
					}
				}
				return true
			})
		}
	}
	for typeName, fields := range trackedFields {
		if litCount[typeName] == 0 {
			for goName := range fields {
				add(payloadTypes[typeName].Pos(), typeName, goName, "R3",
					"型別宣告了 wire 路由欄位，但本 package 內找不到任何複合字面量 ⇒ "+
						"本檢查對它零鑑別力。它若真的沒有生產者就該刪除，不是留著讓 gate 沉默")
			}
		}
	}

	for _, f := range files {
		for _, topDecl := range f.Decls {
			enclosing, _ := topDecl.(*ast.FuncDecl)
			ast.Inspect(topDecl, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, lhs := range assign.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					for typeName, fields := range trackedFields {
						ax, isTracked := fields[sel.Sel.Name]
						if !isTracked || !identHasType(f, sel.X, typeName) {
							continue
						}
						rep.WritePoints[typeName+"."+sel.Sel.Name]++
						if i >= len(assign.Rhs) {
							continue
						}
						ctor, approved := approvedCtorOf(assign.Rhs[i], ax.Ctors)
						if !approved {
							add(assign.Pos(), typeName, sel.Sel.Name, "R3",
								"事後賦值的值必須是 "+strings.Join(ax.Ctors, " / ")+" 的回傳。後果："+ax.Impact)
							continue
						}
						if detail, bad := ctorArgProvenance(enclosing, assign.Rhs[i].(*ast.CallExpr), ctor); bad {
							add(assign.Pos(), typeName, sel.Sel.Name, "R8", detail+"後果："+ax.Impact)
						}
					}
				}
				return true
			})
		}
	}

	outboxTypes := map[string]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok || exprText(cl.Type) != "domain.NewOutboxEvent" {
					return true
				}
				var payloadExpr ast.Expr
				for _, elt := range cl.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Payload" {
							payloadExpr = kv.Value
						}
					}
				}
				if payloadExpr == nil {
					add(cl.Pos(), "domain.NewOutboxEvent", "Payload", "R5",
						"outbox 落點沒有 Payload 欄位 —— 無法判定它送出去的是什麼")
					return true
				}
				resolved, ok := resolvePayloadTypes(payloadExpr, fn, funcDecls)
				if !ok || len(resolved) == 0 {
					add(payloadExpr.Pos(), "domain.NewOutboxEvent", "Payload", "R5",
						"追不回這個 payload 是由哪個 wire 型別 marshal 出來的（實際是 `"+
							exprText(payloadExpr)+"`）。\n"+
							"出向事件的 payload 必須是 json.Marshal(<事件型別>{...}) 的結果，"+
							"不得用 map、不得用外部組好的 []byte —— 那會讓 R1–R4 全部繞過")
					return true
				}
				for _, tn := range resolved {
					if _, known := payloadTypes[tn]; !known {
						add(payloadExpr.Pos(), tn, "Payload", "R5",
							"outbox payload 的型別 "+tn+" 沒有被辨識為事件 payload 型別 —— 辨識器有漏")
						continue
					}
					outboxTypes[tn] = true
				}
				return true
			})
		}
	}
	for tn := range outboxTypes {
		rep.OutboxTypes = append(rep.OutboxTypes, tn)
	}
	sort.Strings(rep.OutboxTypes)

	aggregateTypes := map[string]bool{}
	for _, t := range wireCtorParamTypes {
		aggregateTypes[t] = true
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok || len(cl.Elts) == 0 {
				return true
			}
			tn := exprText(cl.Type)
			if !aggregateTypes[tn] {
				return true
			}
			add(cl.Pos(), tn, "<composite literal>", "R9",
				"不得在本 package 內就地組出 "+tn+"（帶 "+strconv.Itoa(len(cl.Elts))+
					" 個欄位）。取得它的正常路徑是 repository —— 也就是定義 "+
					"id ↔ ref_id 映射的那張表本身。\n"+
					"就地捏一個之後 `eventwire.StoreIDOf(...)` 會編譯過、AST 的 R3 也會過"+
					"（被呼叫者的名字完全正確），而填進 store_id 的可以是任何東西 —— "+
					"這正是 2026-08-31 對抗覆核抓到的第六種繞道。\n"+
					"零欄位的 "+tn+"{} 是允許的（錯誤回傳路徑；eventwire 對零值回不可 marshal 的值）。")
			return true
		})
	}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isJSONMarshal(call.Fun) || len(call.Args) == 0 {
				return true
			}
			cl, ok := call.Args[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			if _, isMap := cl.Type.(*ast.MapType); !isMap {
				return true
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				lit, ok := kv.Key.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				key, _ := strconv.Unquote(lit.Value)
				for _, ax := range wireAxes {
					if key == ax.Tag {
						add(kv.Pos(), "map[string]any", ax.Tag, "R6",
							"不得用 map 直接組出帶 "+strconv.Quote(ax.Tag)+" 的 payload —— "+
								"那會繞過型別與 R1–R4 全部四道。後果："+ax.Impact)
					}
				}
			}
			return true
		})
	}

	for _, tn := range rep.OutboxTypes {
		st := payloadTypes[tn]
		for _, ax := range wireAxes {
			if !ax.Required {
				continue
			}
			has := false
			for _, fld := range st.Fields.List {
				if jsonTagOf(fld) == ax.Tag {
					has = true
				}
			}
			if !has {
				add(st.Pos(), tn, ax.Tag, "R7",
					"這個型別會被送進 outbox，卻沒有 "+strconv.Quote(ax.Tag)+" 欄位。後果："+ax.Impact)
			}
		}
	}

	return rep
}

func resolvePayloadTypes(e ast.Expr, enclosing *ast.FuncDecl, funcs map[string]*ast.FuncDecl) ([]string, bool) {
	switch v := e.(type) {
	case *ast.CallExpr:
		if isJSONMarshal(v.Fun) && len(v.Args) > 0 {
			if cl, ok := v.Args[0].(*ast.CompositeLit); ok {
				if id, ok := cl.Type.(*ast.Ident); ok {
					return []string{id.Name}, true
				}
			}
			return nil, false
		}
		name := calleeName(v.Fun)
		fn, ok := funcs[name]
		if !ok || fn.Body == nil {
			return nil, false
		}
		return marshalledTypesIn(fn.Body), true
	case *ast.Ident:
		if enclosing == nil || enclosing.Body == nil {
			return nil, false
		}
		var out []string
		found := false
		ok := true
		ast.Inspect(enclosing.Body, func(n ast.Node) bool {
			assign, isAssign := n.(*ast.AssignStmt)
			if !isAssign {
				return true
			}
			for i, lhs := range assign.Lhs {
				id, isIdent := lhs.(*ast.Ident)
				if !isIdent || id.Name != v.Name {
					continue
				}
				rhs := assign.Rhs[0]
				if i < len(assign.Rhs) {
					rhs = assign.Rhs[i]
				}
				found = true
				types, sub := resolvePayloadTypes(rhs, enclosing, funcs)
				if !sub {
					ok = false
					continue
				}
				out = append(out, types...)
			}
			return true
		})
		return out, found && ok
	}
	return nil, false
}

func marshalledTypesIn(body *ast.BlockStmt) []string {
	var out []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isJSONMarshal(call.Fun) || len(call.Args) == 0 {
			return true
		}
		if cl, ok := call.Args[0].(*ast.CompositeLit); ok {
			if id, ok := cl.Type.(*ast.Ident); ok {
				out = append(out, id.Name)
			}
		}
		return true
	})
	return out
}

func calleeName(fun ast.Expr) string {
	switch v := fun.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}

func identHasType(f *ast.File, x ast.Expr, typeName string) bool {
	id, ok := x.(*ast.Ident)
	if !ok {
		return false
	}
	hit := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.ValueSpec:
			if exprText(v.Type) != typeName {
				return true
			}
			for _, name := range v.Names {
				if name.Name == id.Name {
					hit = true
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				lid, isIdent := lhs.(*ast.Ident)
				if !isIdent || lid.Name != id.Name || i >= len(v.Rhs) {
					continue
				}
				rhs := v.Rhs[i]
				if u, isUnary := rhs.(*ast.UnaryExpr); isUnary {
					rhs = u.X
				}
				if cl, isLit := rhs.(*ast.CompositeLit); isLit && exprText(cl.Type) == typeName {
					hit = true
				}
			}
		}
		return true
	})
	return hit
}

func isJSONMarshal(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Marshal" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "json"
}

func jsonTagOf(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return ""
	}
	m := regexp.MustCompile(`json:"([^"]*)"`).FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	return strings.Split(m[1], ",")[0]
}

func approvedCtorOf(e ast.Expr, ctors []string) (string, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	name := exprText(call.Fun)
	for _, c := range ctors {
		if name == c {
			return name, true
		}
	}
	return "", false
}

func ctorArgProvenance(fn *ast.FuncDecl, call *ast.CallExpr, ctor string) (string, bool) {
	want, known := wireCtorParamTypes[ctor]
	if !known {
		return "建構函式 " + ctor + " 沒有登記期望的 domain 參數型別（wireCtorParamTypes）—— " +
			"新增建構函式必須同時登記，否則引數溯源對它形同不存在。", true
	}
	if len(call.Args) != 1 {
		return ctor + " 收到 " + strconv.Itoa(len(call.Args)) + " 個引數，期望 1 個。", true
	}

	arg := call.Args[0]
	for {
		star, ok := arg.(*ast.StarExpr)
		if !ok {
			break
		}
		arg = star.X
	}

	switch v := arg.(type) {
	case *ast.CompositeLit:
		return ctor + " 的引數是**就地組出來的** `" + exprText(arg) + "`。\n" +
			"取得 " + want + " 的正常路徑是 repository；就地捏一個之後，填進去的 " +
			"id ↔ ref_id 映射沒有任何東西在守，而這一行會編譯過、R3 也會過" +
			"（被呼叫者的名字完全正確）。\n", true
	case *ast.UnaryExpr:
		if _, isLit := v.X.(*ast.CompositeLit); isLit {
			return ctor + " 的引數是就地組出來的 `" + exprText(arg) + "`（取址不改變這件事）。\n", true
		}
		return ctor + " 的引數 `" + exprText(arg) + "` 追不回來源。\n", true
	case *ast.Ident:
		if detail, bad := identProvenance(fn, v.Name, want); bad {
			return ctor + " 的引數 " + v.Name + " " + detail, true
		}
		return "", false
	default:
		return ctor + " 的引數 `" + exprText(arg) + "` 不是一個可溯源的識別字。\n" +
			"fail-closed：追不回來源一律報錯，不是放行 —— 放行的話「先在別處捏、再包一層」" +
			"就是一條敞開的路。\n", true
	}
}

func identProvenance(fn *ast.FuncDecl, name, want string) (string, bool) {
	if fn == nil || fn.Body == nil {
		return "出現在任何函式之外，無從溯源（fail-closed）。\n", true
	}

	for _, fld := range fn.Type.Params.List {
		for _, n := range fld.Names {
			if n.Name != name {
				continue
			}
			if got := exprText(fld.Type); got != want {
				return "是參數，但宣告型別是 " + got + "，期望 " + want + "。\n" +
					"型別對不上就不是同一個東西 —— 而本檢查的整個力量來自「取得 " + want +
					" 的正常路徑是 repository」。\n", true
			}
			return "", false
		}
	}

	found, ok := false, true
	var detail string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, isAssign := n.(*ast.AssignStmt)
		if !isAssign {
			return true
		}
		for i, lhs := range assign.Lhs {
			id, isIdent := lhs.(*ast.Ident)
			if !isIdent || id.Name != name {
				continue
			}
			found = true
			rhs := assign.Rhs[0]
			if i < len(assign.Rhs) {
				rhs = assign.Rhs[i]
			}
			if d, bad := rhsProvenance(rhs, want); bad {
				ok = false
				detail = d
			}
		}
		return true
	})
	if !found {
		return "在外圍函式裡既不是參數也找不到指派點 —— 追不回來源（fail-closed：" +
			"這裡回報錯而不是放行，因為「追不到」與「安全」是兩件事）。\n", true
	}
	if !ok {
		return detail, true
	}
	return "", false
}

func rhsProvenance(rhs ast.Expr, want string) (string, bool) {
	for {
		star, isStar := rhs.(*ast.StarExpr)
		if !isStar {
			break
		}
		rhs = star.X
	}
	switch v := rhs.(type) {
	case *ast.CompositeLit:
		return "由就地組出來的 `" + exprText(rhs) + "` 指派 —— 那不是從 repository 取得的 " +
			want + "。\n", true
	case *ast.UnaryExpr:
		if _, isLit := v.X.(*ast.CompositeLit); isLit {
			return "由就地組出來的 `" + exprText(rhs) + "` 指派。\n", true
		}
	case *ast.CallExpr:
		sel, isSel := v.Fun.(*ast.SelectorExpr)
		if !isSel {
			return "由本 package 的函式 `" + exprText(v.Fun) + "` 指派 —— " +
				"依賴的方法呼叫才算來源（`u.issuers.GetByID(...)` 這種形狀）。\n", true
		}
		if root := rootIdent(sel.X); root == "domain" {
			return "由 `" + exprText(v.Fun) + "` 指派 —— 那是從基本型別重建一個聚合，" +
				"不是從 repository 取得。\n", true
		}
		return "", false
	}
	return "由 `" + exprText(rhs) + "` 指派，追不回一個依賴的方法呼叫。\n", true
}

func rootIdent(e ast.Expr) string {
	for {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			break
		}
		e = sel.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case nil:
		return ""
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprText(v.X) + "." + v.Sel.Name
	case *ast.StarExpr:
		return "*" + exprText(v.X)
	case *ast.CallExpr:
		args := make([]string, 0, len(v.Args))
		for _, a := range v.Args {
			args = append(args, exprText(a))
		}
		return exprText(v.Fun) + "(" + strings.Join(args, ", ") + ")"
	case *ast.BasicLit:
		return v.Value
	case *ast.CompositeLit:
		return exprText(v.Type) + "{…}"
	case *ast.UnaryExpr:
		return v.Op.String() + exprText(v.X)
	case *ast.ArrayType:
		return "[]" + exprText(v.Elt)
	case *ast.MapType:
		return "map[" + exprText(v.Key) + "]" + exprText(v.Value)
	default:
		return "<expr>"
	}
}

func parsePackageFiles(t *testing.T, dir string) (*token.FileSet, []*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatalf("%s 下沒有解到任何非測試 .go 檔 —— 檢查本身失效了", dir)
	}
	return fset, files
}

var knownEventPayloadTypes = []string{
	"issuedEvent", "issueFailedEvent", "complaintCreatedEvent",
	"complaintRespondedEvent", "resolvedExternallyEvent", "taxStatusChangedEvent",
}

// TestEventPayloadsObeyWireRoutingRules 是本 package 的實際 gate。
func TestEventPayloadsObeyWireRoutingRules(t *testing.T) {
	fset, files := parsePackageFiles(t, ".")
	rep := checkEventPayloads(fset, files)

	for _, v := range rep.Violations {
		t.Errorf("%s", v)
	}
	if len(rep.Violations) > 0 {
		t.Logf("辨識到的事件 payload 型別：%v；追到 outbox 的：%v", rep.PayloadTypes, rep.OutboxTypes)
	}

	have := map[string]bool{}
	for _, n := range rep.PayloadTypes {
		have[n] = true
	}
	for _, want := range knownEventPayloadTypes {
		if !have[want] {
			t.Fatalf("辨識器沒認出 %s（實際認出 %v）—— 這不是「全部合規」，是檢查壞了",
				want, rep.PayloadTypes)
		}
	}
	inOutbox := map[string]bool{}
	for _, n := range rep.OutboxTypes {
		inOutbox[n] = true
	}
	for _, want := range knownEventPayloadTypes {
		if !inOutbox[want] {
			t.Fatalf("%s 追不回任何 outbox 落點（實際追到 %v）—— R5/R7 對它形同不存在",
				want, rep.OutboxTypes)
		}
	}
	for _, tn := range knownEventPayloadTypes {
		key := tn + ".StoreID"
		if rep.WritePoints[key] == 0 {
			t.Fatalf("%s 找不到任何 store_id 寫入點 —— 「零違規」在這個型別上沒有意義", key)
		}
	}
}

// TestEventPayloadGateCatchesEveryKnownBypass 證明上面那道 gate **抓得到什麼**。
//
// 🔴 沒有這一支，把 checkEventPayloads 改成永遠回空也會全綠 —— 而那長得跟修好
// 一模一樣。每個反例都刻意「長得像正例」：命名對、能編譯、下游讀得到那個 key。
//
// 🔴 注意這裡**只放編譯得過的繞道**。`ev.StoreID = strconv.FormatInt(...)`、
// `s := ...; ev.StoreID = s`、`ev.StoreID = 自寫 helper(...)` 三種已經由
// internal/eventwire 的不透明型別在編譯期擋掉（證據見
// TestEventWireTypesCannotBeBuiltOutsideTheirPackage 與 scripts 側的變異實驗），
// 不需要也不應該由 AST 層重複守。
func TestEventPayloadGateCatchesEveryKnownBypass(t *testing.T) {
	const preamble = "package usecase\nimport \"encoding/json\"\n"
	bt := func(s string) string { return "`" + s + "`" }

	cases := []struct {
		name     string
		src      string
		wantRule string
		wantHint string
	}{
		{
			name:     "繞道①：新型別把 store_id 宣告成 string，再填 invoice_issuers 的 PK",
			wantRule: "R2",
			wantHint: "eventwire.StoreID",
			src: preamble + `
type issuedEvent struct {
	OrderNo string ` + bt(`json:"order_no"`) + `
	StoreID string ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document) ([]byte, error) {
	return json.Marshal(issuedEvent{OrderNo: doc.SourceRef, StoreID: strconv.FormatInt(doc.IssuerID, 10)})
}
`,
		},
		{
			name:     "繞道①b：型別對了，值卻換個 helper 包一層",
			wantRule: "R3",
			wantHint: "formatID",
			src: preamble + `
type complaintCreatedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(c domain.Complaint) ([]byte, error) {
	return json.Marshal(complaintCreatedEvent{StoreID: formatID(c.IssuerID)})
}
`,
		},
		{
			name:     "繞道②：欄位整個不填（零值）",
			wantRule: "R4",
			wantHint: "StoreID",
			src: preamble + `
type issuedEvent struct {
	OrderNo string            ` + bt(`json:"order_no"`) + `
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document) ([]byte, error) {
	return json.Marshal(issuedEvent{OrderNo: doc.SourceRef})
}
`,
		},
		{
			name:     "繞道③：改用 map 直接組 payload",
			wantRule: "R6",
			wantHint: "store_id",
			src: preamble + `
func emit(doc domain.Document) ([]byte, error) {
	return json.Marshal(map[string]any{"order_no": doc.SourceRef, "store_id": doc.IssuerID})
}
`,
		},
		{
			name:     "繞道④：新增第 7 個事件型別，忘了帶 store_id",
			wantRule: "R7",
			wantHint: "store_id",
			src: preamble + `
type adjustmentAppliedEvent struct {
	OrderNo string ` + bt(`json:"order_no"`) + `
}
func emit(doc domain.Document) error {
	payload, _ := json.Marshal(adjustmentAppliedEvent{OrderNo: doc.SourceRef})
	_, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{Payload: payload})
	return err
}
`,
		},
		{
			name:     "繞道④b：payload 從別處拿來，追不回任何 wire 型別",
			wantRule: "R5",
			wantHint: "追不回",
			src: preamble + `
func emit(raw []byte) error {
	_, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{Payload: raw})
	return err
}
`,
		},
		{
			name:     "繞道⑤：buyer_id 換回不判 recipient_type 的寫法（P0-3 的孿生）",
			wantRule: "R3",
			wantHint: "formatOptionalID",
			src: preamble + `
type issuedEvent struct {
	BuyerID eventwire.BuyerID ` + bt(`json:"buyer_id"`) + `
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document, i domain.Issuer) ([]byte, error) {
	return json.Marshal(issuedEvent{
		BuyerID: formatOptionalID(doc.RecipientRefID),
		StoreID: eventwire.StoreIDOf(i),
	})
}
`,
		},
		{
			name:     "繞道⑥：buyer_id 的欄位型別被改回 string",
			wantRule: "R2",
			wantHint: "eventwire.BuyerID",
			src: preamble + `
type issuedEvent struct {
	BuyerID string            ` + bt(`json:"buyer_id"`) + `
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document, i domain.Issuer) ([]byte, error) {
	return json.Marshal(issuedEvent{BuyerID: "1", StoreID: eventwire.StoreIDOf(i)})
}
`,
		},
		{
			name:     "繞道⑦：欄名留著、tag 改成下游不會讀的鍵",
			wantRule: "R2",
			wantHint: "seller_id",
			src: preamble + `
type issuedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"seller_id"`) + `
}
func emit(i domain.Issuer) ([]byte, error) { return json.Marshal(issuedEvent{StoreID: eventwire.StoreIDOf(i)}) }
`,
		},
		{
			name:     "繞道⑧：camelCase wire 欄名",
			wantRule: "R1",
			wantHint: "storeId",
			src: preamble + `
type taxStatusChangedEvent struct {
	StoreID    eventwire.StoreID ` + bt(`json:"storeId"`) + `
	FromStatus string            ` + bt(`json:"fromStatus"`) + `
}
func emit(i domain.Issuer) ([]byte, error) { return json.Marshal(taxStatusChangedEvent{StoreID: eventwire.StoreIDOf(i)}) }
`,
		},
		{
			name:     "繞道⑨：事後賦值（本輪對抗覆核抓到的那一種）",
			wantRule: "R3",
			wantHint: "事後賦值",
			src: preamble + `
type complaintCreatedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(c domain.Complaint, i domain.Issuer) ([]byte, error) {
	ev := complaintCreatedEvent{StoreID: eventwire.StoreIDOf(i)}
	ev.StoreID = wrongWire(c.IssuerID)
	return json.Marshal(ev)
}
`,
		},
		{
			name:     "繞道⑪：建構函式對了，引數是**就地捏出來的**（2026-08-31 對抗覆核）",
			wantRule: "R8",
			wantHint: "就地組出來的",
			src: preamble + `
type issuedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document) ([]byte, error) {
	return json.Marshal(issuedEvent{
		StoreID: eventwire.StoreIDOf(domain.Issuer{ID: doc.IssuerID, IssuerRefID: doc.IssuerID}),
	})
}
`,
		},
		{
			name:     "繞道⑪b：先在別處捏好，再當成一般變數傳進去",
			wantRule: "R8",
			wantHint: "就地組出來的",
			src: preamble + `
type issuedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document) ([]byte, error) {
	issuer := domain.Issuer{ID: doc.IssuerID, IssuerRefID: doc.IssuerID}
	return json.Marshal(issuedEvent{StoreID: eventwire.StoreIDOf(issuer)})
}
`,
		},
		{
			name:     "繞道⑪c：捏一個 RecipientType=BUYER 的 document，抹掉建構函式內的守衛",
			wantRule: "R8",
			wantHint: "就地組出來的",
			src: preamble + `
type issuedEvent struct {
	BuyerID eventwire.BuyerID ` + bt(`json:"buyer_id"`) + `
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document, i domain.Issuer) ([]byte, error) {
	return json.Marshal(issuedEvent{
		BuyerID: eventwire.BuyerIDOfDocument(domain.Document{
			ID: doc.ID, RecipientType: "BUYER", RecipientRefID: doc.RecipientRefID,
		}),
		StoreID: eventwire.StoreIDOf(i),
	})
}
`,
		},
		{
			name:     "繞道⑪d：引數來自本 package 自己寫的重建函式（fail-closed：追不到不是安全）",
			wantRule: "R8",
			wantHint: "本 package 的函式",
			src: preamble + `
type issuedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(doc domain.Document) ([]byte, error) {
	issuer := rebuildIssuer(doc.IssuerID)
	return json.Marshal(issuedEvent{StoreID: eventwire.StoreIDOf(issuer)})
}
`,
		},
		{
			name:     "繞道⑫：不碰建構函式，只是在本 package 內就地組出一個聚合（R8 的來源端）",
			wantRule: "R9",
			wantHint: "domain.Issuer",
			src: preamble + `
type issuedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func build(doc domain.Document) domain.Issuer {
	return domain.Issuer{ID: doc.IssuerID, IssuerRefID: doc.IssuerID}
}
func emit(i domain.Issuer) ([]byte, error) {
	return json.Marshal(issuedEvent{StoreID: eventwire.StoreIDOf(i)})
}
`,
		},
		{
			name:     "繞道⑩：位置參數字面量（欄位對應關係無從驗證）",
			wantRule: "R3",
			wantHint: "位置參數",
			src: preamble + `
type issuedEvent struct {
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(i domain.Issuer) ([]byte, error) { return json.Marshal(issuedEvent{eventwire.StoreIDOf(i)}) }
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "synthetic.go", c.src, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse synthetic: %v", err)
			}
			rep := checkEventPayloads(fset, []*ast.File{f})
			for _, v := range rep.Violations {
				if v.Rule == c.wantRule && strings.Contains(v.String(), c.wantHint) {
					return
				}
			}
			t.Fatalf("反例沒有被 %s（含 %q）抓到。實際違規：%v", c.wantRule, c.wantHint, rep.Violations)
		})
	}

	ok := preamble + `
type issuedEvent struct {
	OrderNo string            ` + bt(`json:"order_no"`) + `
	BuyerID eventwire.BuyerID ` + bt(`json:"buyer_id"`) + `
	StoreID eventwire.StoreID ` + bt(`json:"store_id"`) + `
}
func emit(i domain.Issuer, doc domain.Document) error {
	payload, err := json.Marshal(issuedEvent{
		OrderNo: doc.SourceRef,
		BuyerID: eventwire.BuyerIDOfDocument(doc),
		StoreID: eventwire.StoreIDOf(i),
	})
	if err != nil {
		return err
	}
	_, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{Payload: payload})
	return err
}
`
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "ok.go", ok, parser.ParseComments)
	if rep := checkEventPayloads(fset, []*ast.File{f}); len(rep.Violations) != 0 {
		t.Fatalf("正例被誤報：%v", rep.Violations)
	}
}

// TestEventWireTypesCannotBeBuiltOutsideTheirPackage 守住**第一道線本身**。
//
// R2 把「欄位型別必須是 eventwire.StoreID」寫成規則，而那條規則的全部力量來自
// 「eventwire.StoreID 在 package 外組不出來」。這一支就是在驗那個前提：
//
//	① 底層必須是 struct 且欄位全部未匯出 —— 換成 `type StoreID string` 的話
//	   `StoreID("123")` 立刻編得過，整道保證當場失效且**不會有任何測試變紅**
//	② 建構函式的參數型別必須是 domain 型別 —— 收 int64 的話
//	   `StoreIDOf(doc.IssuerID)` 會編譯過，而那正是 P0-3 的那一行
//	③ 建構函式集合必須等於 gate 的白名單 —— 新增一支而沒登記，R3 會擋；
//	   登記了而 eventwire 沒有，這裡會擋。兩邊互相釘住。
func TestEventWireTypesCannotBeBuiltOutsideTheirPackage(t *testing.T) {
	_, files := parsePackageFiles(t, "../eventwire")

	structs := map[string]*ast.StructType{}
	ctorsOf := map[string][]string{}
	paramsOf := map[string][]string{}
	for _, f := range files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							structs[ts.Name.Name] = st
						} else {
							structs[ts.Name.Name] = nil
						}
					}
				}
			case *ast.FuncDecl:
				if decl.Recv != nil || decl.Type.Results.NumFields() != 1 {
					continue
				}
				ret := exprText(decl.Type.Results.List[0].Type)
				ctorsOf[ret] = append(ctorsOf[ret], decl.Name.Name)
				var ps []string
				for _, p := range decl.Type.Params.List {
					for range max(1, len(p.Names)) {
						ps = append(ps, exprText(p.Type))
					}
				}
				paramsOf[decl.Name.Name] = ps
			}
		}
	}

	wantCtorParams := map[string][]string{}
	for full, domainType := range wireCtorParamTypes {
		wantCtorParams[strings.TrimPrefix(full, "eventwire.")] = []string{domainType}
	}

	for _, ax := range wireAxes {
		typeName := strings.TrimPrefix(ax.Type, "eventwire.")

		st, declared := structs[typeName]
		if !declared {
			t.Fatalf("internal/eventwire 沒有宣告 %s —— R2 指向一個不存在的型別", typeName)
		}
		if st == nil {
			t.Fatalf("%s 的底層不是 struct。named string 的話 %s(\"123\") 就編得過，"+
				"「package 外組不出來」這個前提當場失效", typeName, typeName)
		}
		if st.Fields.NumFields() == 0 {
			t.Fatalf("%s 沒有任何欄位 —— 空 struct 可由 %s{} 直接組出，不具不透明性", typeName, typeName)
		}
		for _, fld := range st.Fields.List {
			for _, n := range fld.Names {
				if ast.IsExported(n.Name) {
					t.Fatalf("%s.%s 是匯出欄位 ⇒ 外部可用 %s{%s: ...} 直接組出一個值，"+
						"整道編譯期保證失效", typeName, n.Name, typeName, n.Name)
				}
			}
		}
		if _, hasMarshal := findMethod(files, typeName, "MarshalJSON"); !hasMarshal {
			t.Fatalf("%s 沒有 MarshalJSON ⇒ 零值會被 encoding/json 靜默序列化成 `{}`，"+
				"「漏填會失敗」這個保證不成立", typeName)
		}

		var got []string
		for _, c := range ctorsOf[typeName] {
			got = append(got, "eventwire."+c)
		}
		sort.Strings(got)
		want := append([]string(nil), ax.Ctors...)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s 的建構函式集合 = %v，gate 白名單 = %v。\n"+
				"兩者必須逐字相等：eventwire 多一支而白名單沒登記 ⇒ 那支的產物過不了 R3；"+
				"白名單多一項而 eventwire 沒有 ⇒ R3 放行一個不存在的東西。", typeName, got, want)
		}

		for _, c := range ax.Ctors {
			if _, ok := wireCtorParamTypes[c]; !ok {
				t.Fatalf("%s 在 gate 白名單裡，卻沒有登記 wireCtorParamTypes —— "+
					"R8 的引數溯源對它形同不存在", c)
			}
		}

		for _, c := range ctorsOf[typeName] {
			wantParams, known := wantCtorParams[c]
			if !known {
				t.Fatalf("建構函式 %s 沒有登記期望參數型別 —— 新增建構函式必須同時登記，"+
					"否則「參數是 domain 型別」這條就沒人守", c)
			}
			if strings.Join(paramsOf[c], ",") != strings.Join(wantParams, ",") {
				t.Fatalf("%s 的參數 = %v，必須是 %v。\n"+
					"收 int64 的話 %s(doc.IssuerID) 會編譯過，而那正是本規則要防的那一行 —— "+
					"取得 domain 型別的唯一路徑是 repository，也就是定義該映射的那張表。",
					c, paramsOf[c], wantParams, c)
			}
		}
	}
}

func findMethod(files []*ast.File, recvType, name string) (*ast.FuncDecl, bool) {
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Name.Name != name {
				continue
			}
			if exprText(fd.Recv.List[0].Type) == recvType ||
				exprText(fd.Recv.List[0].Type) == "*"+recvType {
				return fd, true
			}
		}
	}
	return nil, false
}
