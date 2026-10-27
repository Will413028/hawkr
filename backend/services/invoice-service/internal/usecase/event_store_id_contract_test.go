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

// ---------------------------------------------------------------------------
// 出向事件 payload 的機械保證（第二道線；第一道是型別，見 internal/eventwire）
//
// 🔴 為什麼有兩道線，以及各自擋什麼——這一段是本檔存在的理由，改本檔前先讀：
//
//	第一道（編譯期，internal/eventwire）
//	  StoreID／BuyerID 的底層是帶未匯出欄位的 struct ⇒ 本 package 內
//	  **組不出**一個錯的值。`ev.StoreID = strconv.FormatInt(...)`、
//	  `s := ...; ev.StoreID = s`、`ev.StoreID = 自己寫的 helper(...)`
//	  三種寫法全部編不過。
//
//	第二道（本檔，AST）
//	  編譯期擋不住的還有五種，全部長得像正例、全部編得過：
//	    ① 新的 payload 型別把欄位宣告成 `StoreID string` ⇒ P0-3 原樣復活
//	    ② 欄位整個不填（零值）⇒ wire 上是空字串或 marshal 失敗
//	    ③ 繞開型別：`json.Marshal(map[string]any{"store_id": ...})`
//	    ④ 新增第 7 個事件型別而**沒有** store_id 欄位 ⇒ 賣家向通知恆 skip
//	    ⑤ 建構函式呼叫對了，**引數是就地捏出來的**：
//	         eventwire.StoreIDOf(domain.Issuer{ID: doc.IssuerID, IssuerRefID: doc.IssuerID})
//	       這一種是 2026-08-31 的對抗覆核抓到的，而它在補上 R8／R9 之前
//	       **編譯期與 AST 兩道都放行**（`go build` exit 0、本檔連跑 8 次全 PASS）。
//	       成因：舊的 isApprovedCtorCall 只比對**被呼叫者的名字文字**，從不看引數哪來的。
//
// 🔴 本檔的形狀是 **fail-closed**：不是「列出已知的壞寫法」而是
// 「除非證明安全否則報錯」——
//   - 認得出的 payload 型別，其 store_id／buyer_id 欄位的**每一個寫入點**都必須是
//     核可的建構函式呼叫；認不出寫入點（位置參數字面量、無法解析的運算式）⇒ 報錯
//   - 那個呼叫的**引數本身**必須追得回一個真的從 repository 取得的 domain 物件
//     （R8）：函式參數且型別正確，或區域變數且來自某個依賴的方法呼叫。
//     追不回來源 ⇒ **報錯，不是放行**
//   - 本 package 內**不得就地組出**任何被追蹤的 domain 聚合（R9）。這條關的是
//     「先在別處捏一個、再當成參數傳進來」那條路：R8 只看得到當前函式，
//     R9 讓「捏」這個動作在整個 package 裡都無處可做
//   - 型別宣告了 tracked 欄位卻在本 package 內**找不到任何複合字面量** ⇒ 報錯
//     （目標不存在一律 FAIL 不 SKIP）
//   - outbox 落點的 Payload 追不回一個已納管的型別 ⇒ 報錯
//
// 判定材料是**原始碼 AST**，不是任何人的自述 ⇒ 可以強制而不只是提醒。
// 鑑別力由 TestEventPayloadGateCatchesEveryKnownBypass 證明：它把四種繞道逐一餵進去，
// 任何一種沒被抓到就 fail。
// ---------------------------------------------------------------------------

// wireAxis 是一個「路由識別碼軸」的規格。
//
// 新增第三個軸（例如 recipient_store_id）時，在 wireAxes 加一條就好——所有規則都
// 以本表驅動，沒有任何一條規則寫死 "store_id"。
type wireAxis struct {
	Tag      string   // wire 欄名
	Type     string   // 欄位唯一合法的 Go 型別
	Ctors    []string // 唯一合法的值來源（限定選擇器全名）
	GoNames  []string // 對應的 Go 欄位識別字（用來抓「改了 tag 卻留著欄名」）
	Required bool     // 是否每個出向事件都必須有這一欄
	Impact   string   // 違反時的實際後果（寫進錯誤訊息，讓下一個人不用重新考古）
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
		Required: false, // taxStatusChangedEvent 是純賣家向事件，沒有買家
		Impact: "invoice_documents.recipient_ref_id 是多義欄位：BUYER ⇒ buyers.buyer_id、" +
			"SELLER ⇒ sellers.store_id、EXTERNAL ⇒ NULL。不判 recipient_type 就填 ⇒ " +
			"方向二／四的事件把一個 store_id 裝進 buyer_id ⇒ §6.3.9 模板的 target_user_field " +
			"正是它 ⇒ 通知投遞到錯的人或無人",
	},
}

// wireCtorParamTypes 是「每支建構函式收哪個 domain 型別」的唯一來源。
//
// 🔴 兩處消費者刻意共用同一份：
//   - R8（引數溯源）用它判定「這個識別字必須是哪個型別的參數」
//   - TestEventWireTypesCannotBeBuiltOutsideTheirPackage 用它驗 eventwire 那邊的
//     實際簽章
//
// 各寫一份的話，改了 eventwire 的簽章而忘記改 R8，R8 會開始拿舊型別去比對 ⇒
// 所有寫入點一起紅（吵）或一起過（危險），而兩種都不會指向真正的原因。
//
// 值同時也是 R9 的「不得就地組出的聚合型別」清單 —— 那不是巧合：
// 一個型別之所以能當路由識別碼的來源，正是因為取得它的正常路徑是 repository。
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

// gateReport 是 checkEventPayloads 的完整輸出。
//
// 除了違規清單，它還回三個**鑑別力自檢用**的集合：辨識器若什麼都認不出來，違規清單
// 當然是空的，而那看起來跟「全部合規」一模一樣。呼叫端必須斷言它們非空。
type gateReport struct {
	Violations   []payloadViolation
	PayloadTypes []string       // 被認定為出向事件 payload 的型別名
	OutboxTypes  []string       // 追得回 outbox 落點的 payload 型別名
	WritePoints  map[string]int // "型別.欄位" → 找到幾個寫入點
}

// checkEventPayloads 掃一組 Go 原始檔，套用 R1–R7。
func checkEventPayloads(fset *token.FileSet, files []*ast.File) gateReport {
	rep := gateReport{WritePoints: map[string]int{}}
	add := func(pos token.Pos, structName, field, rule, detail string) {
		rep.Violations = append(rep.Violations, payloadViolation{
			Pos: fset.Position(pos).String(), Struct: structName, Field: field, Rule: rule, Detail: detail,
		})
	}

	// ---- 0) 收集全部 struct 宣告與 func 宣告 --------------------------------
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

	// ---- 1) 辨識「出向事件 payload 型別」-----------------------------------
	//
	// 四條並聯條件（union），任一成立即納入。用並聯而不是單一條件，是因為每一條
	// 單獨都有明確的繞道：
	//   (a) 名稱以 Event 結尾            → 改名叫 fooPayload 就繞過
	//   (b) 字面量直接在 json.Marshal 裡 → 先 marshal 到變數再傳就繞過
	//   (c) 帶 store_id / buyer_id tag   → 把 tag 改掉就繞過
	//   (d) Go 欄名叫 StoreID / BuyerID  → 把欄名改掉就繞過
	// 四條同時繞過 ＝ 那個值根本沒打算走 payload["store_id"]，會被 R5/R7 接住。
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

	// trackedFields[型別名][Go 欄名] = 對應的軸
	trackedFields := map[string]map[string]wireAxis{}

	// ---- 2) R1 欄名 snake_case ／ R2 欄位型別 -------------------------------
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
					// 欄名叫 StoreID 卻掛了別的 tag ＝ 把值送去一個下游不會讀的鍵。
					add(fld.Pos(), name, goName, "R2",
						"欄位叫 "+goName+" 卻掛 json tag "+strconv.Quote(tag)+
							"，必須是 "+strconv.Quote(ax.Tag)+"。後果："+ax.Impact)
				case declared == ax.Type:
					// 型別對、tag 不對：同樣送不到下游。
					add(fld.Pos(), name, goName, "R2",
						"欄位型別是 "+ax.Type+" 卻掛 json tag "+strconv.Quote(tag)+
							"，必須是 "+strconv.Quote(ax.Tag))
				}
			}
		}
	}

	// ---- 3) R3 寫入點 ------------------------------------------------------
	//
	// fail-closed：每一個寫入點都必須是核可建構函式的呼叫；型別宣告了 tracked 欄位
	// 卻找不到任何複合字面量，也是違規（目標不存在一律 FAIL 不 SKIP）。
	litCount := map[string]int{}
	for _, f := range files {
		for _, topDecl := range f.Decls {
			// 🔴 逐 decl 走而不是整檔走，是為了讓 R8 拿得到**外圍函式**：引數溯源要在
			// 那個函式的參數與區域指派裡找來源。整檔走的話這個資訊不存在，
			// R8 就只能退回「看名字」——那正是它要修的病。
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
						// 位置參數字面量：欄位對應關係只能靠宣告順序推，改一次欄位順序就
						// 全錯，而且本檢查無從驗證 ⇒ 一律當違規。
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
				// R4：漏填 ＝ 零值。編譯得過，wire 上不是空字串就是 marshal 失敗。
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
	// R3 的 fail-closed 半邊：宣告了卻沒有任何字面量 ⇒ 檢查對它完全沒有鑑別力。
	for typeName, fields := range trackedFields {
		if litCount[typeName] == 0 {
			for goName := range fields {
				add(payloadTypes[typeName].Pos(), typeName, goName, "R3",
					"型別宣告了 wire 路由欄位，但本 package 內找不到任何複合字面量 ⇒ "+
						"本檢查對它零鑑別力。它若真的沒有生產者就該刪除，不是留著讓 gate 沉默")
			}
		}
	}

	// R3 的補強：事後賦值。有了 eventwire 的不透明型別，`ev.StoreID = <字串>` 已經
	// 編不過；這一條守的是「有人把欄位型別改回 string 而 R2 又被繞過」的組合，
	// 以及本檢查被移植到還沒有型別保護的地方時。
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

	// ---- 4) R5 outbox payload 的來源必須追得回已納管的型別 ------------------
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

	// ---- 4.5) R9 不得就地組出被追蹤的 domain 聚合 -------------------------
	//
	// 🔴 這條關的是 R8 看不到的那半邊。R8 只看得到**當前函式**：引數若是一個型別正確的
	// 參數，它就放行——而那只是把問題推給呼叫端。呼叫端在同一個 package 裡
	// `complaintCreatedPayload(domain.Issuer{IssuerRefID: doc.IssuerID}, …)` 一樣成立。
	//
	// R9 讓「捏一個聚合」這個動作在整個 package 裡都無處可做。判準不是「長得可疑」而是
	// **結構**：這三個型別的正常來源是 repository（它們是 DB 列的投影），usecase 裡出現
	// 一個帶欄位值的字面量，就代表有人在用基本型別重建一個聚合，而重建的那一刻
	// id ↔ ref_id 的映射就沒有任何東西在守了。
	//
	// 零欄位字面量（`domain.Issuer{}`）放行：那是 `return domain.Issuer{}, err` 的錯誤
	// 回傳路徑，而 eventwire 的建構函式對零值本來就回不可 marshal 的結果。
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

	// ---- 5) R6 繞開型別：直接 marshal map --------------------------------
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

	// ---- 6) R7 每個真的送出去的事件都要帶必填軸 ----------------------------
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

// resolvePayloadTypes 把 `Payload:` 的值追回一組事件型別名。
//
// 支援兩層：直接 json.Marshal(T{...})，或指向一個區域變數而該變數由
// json.Marshal(T{...}) 或一支本 package 的函式（其函式體 marshal 了 T{...}）指派。
// 追不動就回 false —— **不猜、不放行**。
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
				// `a, b := f()` 這種單 RHS 多 LHS 的形狀取 RHS[0]。
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

// marshalledTypesIn 回一段函式體內所有 json.Marshal(T{...}) 的 T。
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

// identHasType 判斷 x 這個識別字在本檔內是否曾以 typeName 宣告或指派。
//
// 只認得同檔內的 `var x T`、`x := T{...}`、`x = T{...}` 三種——這是刻意的窄範圍：
// 主保證是編譯期型別，這一條只是補強，寧可窄也不要誤報。
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

// approvedCtorOf 只認「對核可建構函式的直接呼叫」，並回那支建構函式的全名。
//
// 🔴 白名單而不是黑名單：任何其他運算式（區域變數、自寫 helper、型別轉換、
// 三元式改寫）一律不放行。這就是 fail-closed 的具體形狀。
//
// ⚠️ 它**只回答「呼叫的是誰」**。「引數哪來的」是另一個問題，由 ctorArgProvenance 回答
// —— 把兩者混成一個 bool 正是舊版的病：`eventwire.StoreIDOf(domain.Issuer{…})` 的
// 被呼叫者完全正確，於是整道檢查放行一個就地捏出來的聚合。
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

// ctorArgProvenance 回答「這支建構函式收到的東西，是不是真的從 repository 來的」。
// 回 (說明, true) 代表**有問題**；(", false) 代表通過。
//
// 🔴 fail-closed：只有兩種來源算數，其餘一律報錯 —— 包含「追不回來源」。
//
//	(a) 外圍函式的參數，且宣告型別正好是這支建構函式要的 domain 型別
//	    （型別對不上就是另一件東西；型別對得上而是捏出來的，由 R9 在來源端擋）
//	(b) 外圍函式內的區域變數，且它由某個**依賴的方法呼叫**指派
//	    （`u.issuers.GetByID(ctx, doc.IssuerID)` 這種形狀）
//
// 為什麼 (b) 只要求「是個方法呼叫」而不追進那支方法：本 package 內任何從基本型別重建
// 聚合的動作都被 R9 擋住了，所以一個回 domain 聚合的方法呼叫，它的值只可能來自
// repository 或另一個同樣受管的來源。兩條規則是一組，拆開任一條都會漏。
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
		star, ok := arg.(*ast.StarExpr) // *ptr 解參照
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

// identProvenance 在 fn 內追一個識別字的來源。回 (說明, true) 代表有問題。
func identProvenance(fn *ast.FuncDecl, name, want string) (string, bool) {
	if fn == nil || fn.Body == nil {
		return "出現在任何函式之外，無從溯源（fail-closed）。\n", true
	}

	// (a) 函式參數（含 receiver 不算：receiver 是 usecase 自己）
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

	// (b) 函式內指派
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
			rhs := assign.Rhs[0] // `a, b := f()` 取單一 RHS
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

// rhsProvenance 判斷一個指派右值是不是可接受的來源。
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

// rootIdent 回一個選擇器鏈最左邊的識別字（`u.issuers.GetByID` → "u"）。
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

// exprText 把運算式還原成大致的原始碼文字。
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

// parsePackageFiles 讀一個目錄下的非測試 .go 檔。
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

// 本 package 已知的六個出向事件 payload 型別。
//
// 這份清單是**下界**不是白名單：多出來的型別一樣受全部規則管，但少一個就代表辨識器
// 壞了或某個事件被默默刪掉，兩者都必須當場紅。
var knownEventPayloadTypes = []string{
	"issuedEvent", "issueFailedEvent", "complaintCreatedEvent",
	"complaintRespondedEvent", "resolvedExternallyEvent", "taxStatusChangedEvent",
}

// TestEventPayloadsObeyWireRoutingRules 是本 package 的實際 gate。
func TestEventPayloadsObeyWireRoutingRules(t *testing.T) {
	fset, files := parsePackageFiles(t, ".")
	rep := checkEventPayloads(fset, files)

	// 先報違規再做鑑別力自檢：自檢是 Fatal，先跑會把真正的違規訊息蓋掉，
	// 而「gate 抓到了什麼」正是讀的人最需要看到的那一行。
	for _, v := range rep.Violations {
		t.Errorf("%s", v)
	}
	if len(rep.Violations) > 0 {
		t.Logf("辨識到的事件 payload 型別：%v；追到 outbox 的：%v", rep.PayloadTypes, rep.OutboxTypes)
	}

	// 鑑別力自檢 ①：六個已知型別一個都不能少。
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
	// 鑑別力自檢 ②：六個型別全都要追得回 outbox 落點，否則 R5/R7 對它們零作用。
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
	// 鑑別力自檢 ③：每個必填軸都要真的找到寫入點。
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

	// 正例必須是乾淨的：規則若對任何東西都報警，它就沒有資訊量。
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
	ctorsOf := map[string][]string{}  // 型別名 → 建構函式名
	paramsOf := map[string][]string{} // 建構函式名 → 參數型別
	for _, f := range files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							structs[ts.Name.Name] = st
						} else {
							structs[ts.Name.Name] = nil // 存在但不是 struct ⇒ 下面會紅
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

	// 🔴 期望參數型別**不在這裡另寫一份**：它與 R8 的引數溯源共用 wireCtorParamTypes。
	// 各寫一份的話，改了 eventwire 的簽章而只更新其中一處，另一處會安靜地拿舊型別去比對。
	wantCtorParams := map[string][]string{}
	for full, domainType := range wireCtorParamTypes {
		wantCtorParams[strings.TrimPrefix(full, "eventwire.")] = []string{domainType}
	}

	for _, ax := range wireAxes {
		typeName := strings.TrimPrefix(ax.Type, "eventwire.")

		// ① 不透明性
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

		// ③ 建構函式集合 == gate 白名單（也 == wireCtorParamTypes 的鍵集合，見下方 ②）
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

		// ②' 白名單裡的每一支都必須登記期望參數型別，否則 R8 會對它「沒有登記」而全數報錯，
		//     或更糟——有人為了讓它安靜而把 R8 的那條 fail-closed 拿掉。
		for _, c := range ax.Ctors {
			if _, ok := wireCtorParamTypes[c]; !ok {
				t.Fatalf("%s 在 gate 白名單裡，卻沒有登記 wireCtorParamTypes —— "+
					"R8 的引數溯源對它形同不存在", c)
			}
		}

		// ② 參數型別
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
