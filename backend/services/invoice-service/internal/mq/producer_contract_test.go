package mq

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)


type javaComponent struct {
	Type string
	Name string
	Wire string
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("找不到 repo root（往上 12 層都沒有 go.work）")
	return ""
}

var (
	recordHeaderRe = regexp.MustCompile(`(?s)public record\s+\w+\s*\((.*?)\)\s*\{`)
	componentRe    = regexp.MustCompile(`^([\w.<>\[\]]+)\s+(\w+)$`)
)

func parseJavaRecord(t *testing.T, relPath string) []javaComponent {
	t.Helper()
	full := filepath.Join(repoRoot(t), relPath)
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("讀不到 producer 的 Java record %s：%v\n"+
			"（檔案搬家或改名時本測試必須 fail-loud —— 契約的 producer 端消失了，consumer 端不該繼續綠）", full, err)
	}
	src := string(raw)
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(b.String(), "")

	m := recordHeaderRe.FindStringSubmatch(stripped)
	if m == nil {
		t.Fatalf("%s：找不到 `public record X(...)` header", relPath)
	}
	var out []javaComponent
	for _, part := range strings.Split(m[1], ",") {
		part = strings.TrimSpace(strings.Join(strings.Fields(part), " "))
		if part == "" {
			continue
		}
		cm := componentRe.FindStringSubmatch(part)
		if cm == nil {
			t.Fatalf("%s：component %q 解不出「型別 欄名」", relPath, part)
		}
		out = append(out, javaComponent{Type: cm[1], Name: cm[2], Wire: snakeCase(cm[2])})
	}
	if len(out) == 0 {
		t.Fatalf("%s：record header 解出 0 個 component —— 解析器壞了，不是契約對了", relPath)
	}
	return out
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func goWireFields(v any) map[string]string {
	rt := reflect.TypeOf(v)
	out := map[string]string{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		out[tag] = f.Type.String()
	}
	return out
}

var javaTypeToGo = map[string]string{
	"String":  "string",
	"Long":    "int64",
	"long":    "int64",
	"Integer": "int32",
	"Boolean": "bool",
	"boolean": "bool",
}

func assertWireContract(t *testing.T, javaPath string, goStruct any, deliberatelyIgnored map[string]string) {
	t.Helper()
	comps := parseJavaRecord(t, javaPath)
	byWire := map[string]javaComponent{}
	for _, c := range comps {
		byWire[c.Wire] = c
	}
	goFields := goWireFields(goStruct)

	for tag, goType := range goFields {
		c, ok := byWire[tag]
		if !ok {
			t.Errorf("Go 宣告了 json:%q，但 producer(%s) 沒有這個欄位 —— 這個欄位永遠是零值",
				tag, filepath.Base(javaPath))
			continue
		}
		want, known := javaTypeToGo[c.Type]
		if !known {
			t.Errorf("producer 欄位 %s 的 Java 型別 %q 不在 javaTypeToGo 對照表內 —— 請先決定它該對到什麼 Go 型別，不要猜",
				c.Name, c.Type)
			continue
		}
		if goType != want {
			t.Errorf("型別不符：wire 欄位 %q 在 producer 是 %s（⇒ JSON %s），Go 宣告成 %s。\n"+
				"json.Unmarshal 會對**每一則**訊息回錯，而錯誤的歸宿一旦是 ACK 丟棄，"+
				"整條線就是靜默 0 消費（dev 實測 5.4 秒內 40 則）。",
				tag, c.Type, jsonShapeOf(c.Type), goType)
		}
	}

	for _, c := range comps {
		if _, declared := goFields[c.Wire]; declared {
			continue
		}
		reason, registered := deliberatelyIgnored[c.Wire]
		if !registered {
			t.Errorf("producer(%s) 會發 %q，Go 既沒有宣告、也沒有登記為「刻意不讀」。\n"+
				"二選一：把欄位加進 wire struct，或在 deliberatelyIgnored 寫下為什麼不需要它。\n"+
				"（原缺陷就是這一格：producer 發 16 欄、consumer 只宣告 9 欄，"+
				"漏掉的 refund_outcome / cumulative_refund_amount 正是判作廢或折讓的輸入。）",
				filepath.Base(javaPath), c.Wire)
			continue
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%q 登記為刻意不讀，但沒有寫理由", c.Wire)
		}
	}
}

func jsonShapeOf(javaType string) string {
	if javaType == "String" {
		return `"..." 字串`
	}
	return "數字"
}

var wireContractByRoutingKey = map[string]string{
	pkgmq.RoutingKeyOrderPaid:            "services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderPaidPayload.java",
	pkgmq.RoutingKeyOrderRefundFinalized: "services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderRefundFinalizedPayload.java",
	pkgmq.RoutingKeyOrderCancelled:       "services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderCancelledPayload.java",
}

// TestEverySubscribedRoutingKeyHasAWireContract 對帳的是 Subscriptions() 的**本尊**，
// 不是抄本：新增一條訂閱而沒有對應的 producer 對拍時，這裡會紅在「還沒有人驗過
// producer 到底發什麼」，而不是等它上線後靜默吃掉訊息。
func TestEverySubscribedRoutingKeyHasAWireContract(t *testing.T) {
	subs := Subscriptions(nil, nil, nil)
	if len(subs) == 0 {
		t.Fatal("Subscriptions() 回傳空清單 —— 本測試會變成永遠通過的空殼")
	}
	seen := map[string]bool{}
	for _, sub := range subs {
		for _, rk := range sub.RoutingKeys {
			seen[rk] = true
			path, ok := wireContractByRoutingKey[rk]
			if !ok {
				t.Errorf("訂閱了 routing key %q（queue %s）卻沒有 wire 契約："+
					"沒有任何測試從 producer 原始碼讀出它的欄位名與型別。"+
					"手寫 fixture 不算——四個 P0 能活下來正是因為兩端都由同一份想像產生",
					rk, sub.Queue)
				continue
			}
			if _, err := os.Stat(filepath.Join(repoRoot(t), path)); err != nil {
				t.Errorf("routing key %q 的 producer 原始碼 %s 讀不到：%v —— "+
					"契約的另一端被搬走或改名了，這一端不該繼續綠", rk, path, err)
			}
		}
	}
	for rk := range wireContractByRoutingKey {
		if !seen[rk] {
			t.Errorf("wireContractByRoutingKey 列了 %q，但 Subscriptions() 沒有訂閱它 —— "+
				"多餘的條目會讓覆蓋率看起來比實際好", rk)
		}
	}
}

// TestOrderRefundFinalizedPayloadMatchesJavaProducer 是本檔的主體。
func TestOrderRefundFinalizedPayloadMatchesJavaProducer(t *testing.T) {
	assertWireContract(t,
		wireContractByRoutingKey[pkgmq.RoutingKeyOrderRefundFinalized],
		orderRefundFinalizedPayload{},
		map[string]string{})
}

// TestOrderPaidPayloadMatchesJavaProducer 對 C1 做同一件事。
//
// order.paid 的既有債是「金額與 TSID 用 Long」（設計書 §6.3.2.3 明列），所以這一支
// 期待的是 int64 —— 兩條 queue 的答案不同，正是必須各自對拍而不能推論的理由。
func TestOrderPaidPayloadMatchesJavaProducer(t *testing.T) {
	assertWireContract(t,
		wireContractByRoutingKey[pkgmq.RoutingKeyOrderPaid],
		orderPaidPayload{},
		map[string]string{
			"seller_name":         "賣家顯示名，發票線不用（銷售人名稱一律取自 invoice_issuers.display_name，那是法定名稱不是店名）",
			"seller_avatar_key":   "頭像 objectKey，只給 notification 顯示用",
			"buyer_nickname":      "買家顯示名，發票的買受人名稱取自 order 快照與載具，不用暱稱",
			"buyer_avatar_key":    "同上",
			"invoice_snapshot_id": "只是 correlation 指標；真正的快照走 ORD.GetOrderInvoiceSnapshotInternal（§6.3.2.6：品項與金額刻意不上 MQ）",
		})
}

// TestJavaRecordParserHasDiscriminatingPower 驗解析器本身**抓得到**錯誤。
//
// 🔴 沒有這一支的話，`parseJavaRecord` 只要解出空清單或解錯型別，上面兩支就會靜默轉綠
// —— 那是把 gate 關掉，而它長得跟修好一模一樣。
func TestJavaRecordParserHasDiscriminatingPower(t *testing.T) {
	comps := parseJavaRecord(t,
		"services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderRefundFinalizedPayload.java")
	if len(comps) != 16 {
		t.Fatalf("解出 %d 個 component，want 16（producer record 的實際欄數）", len(comps))
	}
	for _, c := range comps {
		if c.Type != "String" {
			t.Errorf("%s 的型別解成 %q，而該 record 每一個 component 都是 String —— 解析器讀錯了",
				c.Name, c.Type)
		}
	}
	got := map[string]bool{}
	for _, c := range comps {
		got[c.Wire] = true
	}
	for _, want := range []string{"refund_ref", "case_no", "cancel_id", "order_total_actual_amount", "cumulative_refund_amount", "refund_outcome"} {
		if !got[want] {
			t.Errorf("wire 欄名 %q 沒被解出來 —— SnakeCaseStrategy 的模擬壞了", want)
		}
	}

	type looksRightButWrong struct {
		EventID      string `json:"event_id"`
		OrderNo      string `json:"order_no"`
		StoreID      int64  `json:"store_id"`
		BuyerID      int64  `json:"buyer_id"`
		RefundAmount int64  `json:"refund_amount"`
	}
	sub := &testing.T{}
	assertWireContract(sub, // nolint:thelper
		"services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderRefundFinalizedPayload.java",
		looksRightButWrong{},
		map[string]string{})
	if !sub.Failed() {
		t.Fatal("反例沒有被抓到：把 store_id/buyer_id/refund_amount 宣告成 int64（＝原缺陷的形狀）竟然通過了")
	}
}

// TestRefundFixtureCoversEveryProducerField 守住**測試 fixture 自己**。
//
// 缺陷能活下來的原因是「自己跟自己對」。所以 fixture 的欄位集合也要對 Java 原始碼，
// 不能只對 Go struct —— 兩者一起錯的時候，只驗其中一邊照樣全綠。
func TestRefundFixtureCoversEveryProducerField(t *testing.T) {
	comps := parseJavaRecord(t,
		"services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderRefundFinalizedPayload.java")
	var fixture map[string]any
	if err := json.Unmarshal(producerShapedRefundBody(t, nil), &fixture); err != nil {
		t.Fatalf("fixture 不是合法 JSON：%v", err)
	}
	for _, c := range comps {
		if _, ok := fixture[c.Wire]; !ok {
			t.Errorf("fixture 少了 producer 會發的 %q", c.Wire)
		}
	}
	for k := range fixture {
		found := false
		for _, c := range comps {
			if c.Wire == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("fixture 多了 producer 不會發的 %q —— 這種欄位會讓測試證明一件不存在的事", k)
		}
	}
	for k, v := range fixture {
		if v == nil {
			continue
		}
		if _, ok := v.(string); !ok {
			t.Errorf("fixture 的 %q 是 %T，但 producer 送的是字串 —— fixture 沒有重現真實 wire 形狀", k, v)
		}
	}
}

// TestNoIntegerWireFieldsOnRefundPayload 是上面的獨立第二道，用 AST 而不是 reflect。
//
// 兩道用不同材料（原始碼 vs 執行期型別）刻意重疊：reflect 那道若因為 struct 被改名而
// 對到別的型別，這一道還在。
func TestNoIntegerWireFieldsOnRefundPayload(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "payload.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse payload.go: %v", err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "orderRefundFinalizedPayload" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		found = true
		for _, fld := range st.Fields.List {
			id, ok := fld.Type.(*ast.Ident)
			if !ok || id.Name != "string" {
				t.Errorf("orderRefundFinalizedPayload.%v 不是 string —— producer 每一欄都是 String，"+
					"宣告成別的型別會讓 json.Unmarshal 對每一則訊息回錯", fld.Names)
			}
		}
		return false
	})
	if !found {
		t.Fatal("在 payload.go 找不到 orderRefundFinalizedPayload —— 檢查本身失效了")
	}
}

// TestOrderCancelledPayloadMatchesJavaProducer 對 C3 做同一件事（#2904）。
//
// 🔴 這一支的存在理由比另外兩支更直接：`OrderCancelledPayload` 的 `buyerId` 是
// **`Long`**（⇒ JSON 數字），而正上方 `OrderRefundFinalizedPayload` 的 `buyerId` 是
// **`String`**（⇒ JSON 字串）。**同一個 producer 服務、同一個欄名、相反的答案。**
// 照著隔壁那支抄，consumer 就對每一則訊息 unmarshal 失敗。
func TestOrderCancelledPayloadMatchesJavaProducer(t *testing.T) {
	assertWireContract(t,
		wireContractByRoutingKey[pkgmq.RoutingKeyOrderCancelled],
		orderCancelledPayload{},
		map[string]string{
			"stock_tx_id":              "庫存交易 id，歸還庫存是 product-service 的事，發票線用不到",
			"items":                    "List<StockItemPayload>（sku_id/quantity）——庫存歸還的明細；發票的品項一律取自開票當下的訂單快照（§6.3.2.6），不從取消事件回推",
			"store_id":                 "本表以 order_no 為主鍵、以 buyer_id 為授權述詞；賣家維度不參與端點 8 的任何一格",
			"notify_buyer_account_id":  "登入 account_id，只給 notification 路由用。本服務一律用營運鍵 buyer_id（§6 身分分離），兩者不可互換",
			"notify_seller_account_id": "同上，賣家側",
			"seller_name":              "賣家顯示名，只給 notification 文案用",
			"seller_avatar_key":        "頭像 objectKey，只給 notification 顯示用",
		})
}

// TestOrderCancelledParserAndCounterexample 驗解析器對**這一份** record 有鑑別力，
// 並餵一個「長得像正例的反例」。
//
// 🔴 反例刻意選成「把 buyer_id 宣告成 string」——那正是「本 repo 的 payload 一律
// string」這條錯誤推論會產出的東西，而它在 refund 那份 payload 上是**對的**。
// 抓不到它，這組測試就只是在證明「我抄對了隔壁那支」。
func TestOrderCancelledParserAndCounterexample(t *testing.T) {
	path := wireContractByRoutingKey[pkgmq.RoutingKeyOrderCancelled]
	comps := parseJavaRecord(t, path)
	if len(comps) != 12 {
		t.Fatalf("解出 %d 個 component，want 12（OrderCancelledPayload 的實際欄數）\n"+
			"⚠️ 同檔還有一個巢狀的 `public record StockItemPayload(...)`；解析器若抓到它會得到 2，"+
			"若把便利建構子當成 header 會得到 11", len(comps))
	}
	byName := map[string]string{}
	for _, c := range comps {
		byName[c.Name] = c.Type
	}
	for name, want := range map[string]string{
		"orderNo":              "String",
		"buyerId":              "Long",
		"storeId":              "Long",
		"notifyBuyerAccountId": "Long",
		"cancelReason":         "String",
		"items":                "List<StockItemPayload>",
	} {
		if got := byName[name]; got != want {
			t.Errorf("component %s 的型別解成 %q，want %q —— 解析器讀錯了", name, got, want)
		}
	}
	gotWire := map[string]bool{}
	for _, c := range comps {
		gotWire[c.Wire] = true
	}
	for _, want := range []string{"stock_tx_id", "seller_avatar_key", "notify_seller_account_id", "cancel_reason"} {
		if !gotWire[want] {
			t.Errorf("wire 欄名 %q 沒被解出來 —— SnakeCaseStrategy 的模擬壞了", want)
		}
	}

	type looksRightButWrong struct {
		EventID string `json:"event_id"`
		OrderNo string `json:"order_no"`
		BuyerID string `json:"buyer_id"`
	}
	sub := &testing.T{}
	assertWireContract(sub, path, looksRightButWrong{}, map[string]string{
		"stock_tx_id": "x", "items": "x", "store_id": "x", "occurred_at": "x",
		"notify_buyer_account_id": "x", "notify_seller_account_id": "x",
		"seller_name": "x", "seller_avatar_key": "x", "cancel_reason": "x",
	})
	if !sub.Failed() {
		t.Fatal("反例①沒有被抓到：把 buyer_id 宣告成 string（＝ refund payload 的形狀）竟然通過了")
	}

	type ghostField struct {
		OrderNo     string `json:"order_no"`
		CancelledAt string `json:"cancelled_at"`
	}
	sub2 := &testing.T{}
	assertWireContract(sub2, path, ghostField{}, map[string]string{
		"event_id": "x", "stock_tx_id": "x", "items": "x", "store_id": "x", "occurred_at": "x",
		"buyer_id": "x", "notify_buyer_account_id": "x", "notify_seller_account_id": "x",
		"seller_name": "x", "seller_avatar_key": "x", "cancel_reason": "x",
	})
	if !sub2.Failed() {
		t.Fatal("反例②沒有被抓到：宣告一個 producer 不會發的 cancelled_at 竟然通過了")
	}
}
