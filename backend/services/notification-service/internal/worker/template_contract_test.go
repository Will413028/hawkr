//go:build integration

// template_contract_test.go — issue #1176 契約 guardrail（13 號設計書 §14.4）。
//
// 對「重放全部 Atlas migrations 後的真實 notification_templates rows」逐 row 驗證：
// 每個 template 讀的 payload key（target_user_field / 無 default 的 #{placeholder} /
// data_fields.field 值 / payload_filter key）都必須在該 event 對應 publisher 的
// golden emitted_keys 內。任一 read-key 對不上 = publisher↔template 契約漂移 =
// 本 issue 要根治的 bug class（靜默漏一處通知）→ 測試紅、擋 merge。
//
// golden 來源：tests/contract/golden/event_payload_keys.json（各 event 的 publisher
// 實發 key 快照）。已知未修復的 template 列在 known_pending.json（後續 PR 修，此處 skip 但保留追蹤）。
package worker_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/worker"
)

type goldenEntry struct {
	Publisher   string   `json:"publisher"`
	Lang        string   `json:"lang"`
	RoutingKey  string   `json:"routing_key"`
	EmittedKeys []string `json:"emitted_keys"`
	// IdentitySpaces 宣告「這個 payload key 裡的數字屬於哪一個身分空間」。
	// 只需要列身分欄位（被某張 template 當 target_user_field 讀的、以及
	// actorFieldByEventType 登記的 actor 欄位），不是每個 emitted key 都要列
	// —— case_no / order_no 這種沒有身分語意的欄位列了只會腐爛。
	IdentitySpaces map[string]identitySpace `json:"identity_spaces,omitempty"`
}

// identitySpace 是一個 payload key 的身分空間宣告，JSON 上有兩種寫法：
//
//	"seller_account_id": "account_id"                       // 固定空間
//	"receiver_id": {"seller": "account_id", "buyer": "buyer_id"}  // 依收件人角色而異
//
// 需要後者是因為**同一個 event_type 的同一個 key 在兩張 template 上是不同空間**：
// chat.message.sent 只有一個 receiver_id，靠 payload_filter receiver_type 分流給
// 買賣兩張 template，publisher 依角色各自填對應空間的值（im-service
// internal/usecase/message.go:556-568）。純「key → 空間」的 map 表達不了這件事，
// 硬要表達就只能挑一邊寫死，而寫死的那一邊會讓另一邊的錯置永遠檢查不到。
type identitySpace struct {
	Fixed  string            // 非空 = 固定空間
	ByRole map[string]string // 非 nil = 依 target_user_type 取值
}

func (s *identitySpace) UnmarshalJSON(b []byte) error {
	var fixed string
	if err := json.Unmarshal(b, &fixed); err == nil {
		s.Fixed = fixed
		return nil
	}
	return json.Unmarshal(b, &s.ByRole)
}

// resolve 回傳這個 key 對某個 target_user_type 而言落在哪個身分空間。
// 第二個回傳值為 false 代表「這是 role-dependent 宣告，但沒有列到這個角色」——
// 那是宣告本身不完整，不能當成通過。
func (s identitySpace) resolve(userType string) (string, bool) {
	if s.Fixed != "" {
		return s.Fixed, true
	}
	v, ok := s.ByRole[userType]
	return v, ok
}

// expectedIdentitySpaceFor 是 13 號設計書 §12.1 那條最終契約的機械版：buyer 的
// user_id 必須是 buyer_id，seller 的 user_id 必須是 store_id。填錯完全無聲（通知照寫、
// HTTP 200、無 log、不進 DLQ，只是收件人永遠讀不到），而首任帳號 grandfather 恆等
// （store_id == account_id、buyer_id == account_id）會讓錯誤 fixture 假綠，所以這裡直接
// 由 template 的最終欄位／旗標與 golden 的 identity_spaces 互相驗證。
//
// `identity_migrated` 曾支援 #2707 的逐批遷移；本次收尾後所有 seller template 都必須
// 為 true，欄位保留作為 migration audit／contract guard，而不是 account_id 相容分流。
// buyer 側固定使用 buyer_id。
func expectedIdentitySpaceFor(tmpl templateRow) (string, bool) {
	switch tmpl.TargetUserType {
	case "buyer":
		return "buyer_id", true
	case "seller":
		return "store_id", true
	default:
		return "", false
	}
}

type templateRow struct {
	EventType           string
	TargetUserType      string
	TargetUserField     string
	TitleTemplate       string
	BodyTemplate        string
	ImageKeyTemplate    *string
	ReferenceIDTemplate *string
	SchemaURL           *string
	DataType            string
	Category            string
	Enabled             bool
	IdentityMigrated    bool // seller identity migration audit／final contract guard
	DataFields          map[string]domain.FieldSpec
	PayloadFilter       map[string]json.RawMessage // 契約測試只需要 key 集合，值可為字串或 {"$ne": ...} 物件
}

func loadGolden(t *testing.T) map[string]goldenEntry {
	t.Helper()
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "tests", "contract", "golden", "event_payload_keys.json"))
	require.NoError(t, err)
	var g map[string]goldenEntry
	require.NoError(t, json.Unmarshal(b, &g))
	return g
}

func loadKnownPending(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "tests", "contract", "golden", "known_pending.json"))
	require.NoError(t, err)
	var p map[string]string
	require.NoError(t, json.Unmarshal(b, &p))
	return p
}

func loadIdentityPending(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "tests", "contract", "golden", "known_pending_identity.json"))
	require.NoError(t, err)
	var p map[string]string
	require.NoError(t, json.Unmarshal(b, &p))
	return p
}

// loadActorFields 直接從 template_handler.go 的原始碼解析 actorFieldByEventType。
//
// 為什麼不在測試裡另抄一份：抄一份等於把這條 guardrail 關掉。要擋的正是「有人把
// actor 欄位改回未解析的 commenterId / replierId / userId」，而那個改動只會動
// template_handler.go；測試若讀的是自己的副本，那次改動不會讓任何東西變紅，
// guardrail 就從此只擋得住願意同步兩份的人（02-judgment §2.1.1）。
//
// 用 go/ast 而不是 regex：map literal 的排版（單行 / 多行 / 加註解）隨時會變，
// 語法樹不會。解析失敗或抓不到 map 一律 fail，不回空 map —— 回空 map 的話下面
// 整段 actor 檢查會靜默不執行，正是本檔到處在防的那種失效。
func loadActorFields(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "template_handler.go", nil, 0)
	require.NoError(t, err, "parse template_handler.go")

	out := map[string]string{}
	found := false
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "actorFieldByEventType" {
				continue
			}
			found = true
			require.Len(t, vs.Values, 1)
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			require.Truef(t, ok, "actorFieldByEventType 不是 composite literal，本解析需要跟著改")
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				require.Truef(t, ok, "actorFieldByEventType 的元素不是 key: value")
				k, kerr := strconv.Unquote(kv.Key.(*ast.BasicLit).Value)
				require.NoError(t, kerr)
				v, verr := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
				require.NoError(t, verr)
				out[k] = v
			}
		}
	}
	require.True(t, found, "template_handler.go 找不到 actorFieldByEventType")
	require.NotEmpty(t, out, "actorFieldByEventType 解析出空 map —— 這會讓下面的 actor 檢查靜默不執行")
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// services/notification-service/internal/worker → repo root = ../../../..
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	return root
}

// 這個 package 只有 TestTemplateContract 需要真的 Postgres，所以它一次執行就只開
// 一個容器——已經是 #2070 曝險的下限，沒有「每測一容器」可收。姊妹 package 的
// dbtest_test.go 用共用容器 + CREATE DATABASE ... TEMPLATE 把 N 個容器收成 1 個，
// 那套機制在 N=1 時全是空轉（一個 clone、一個 drop、一組 mutex/seq），所以這裡
// 刻意不套。
//
// 但範本裡有一件事在 N=1 一樣成立：容器起不來就整包紅。#2070 的 RWLayer 失敗是
// 機率性的 daemon 問題，不是測試問題，重試一次比讓人重跑整個 job 便宜。任何啟動
// 錯誤都重試，不只比對 RWLayer 字串——比對 daemon 錯誤訊息很脆，而真正壞掉的環境
// 第二次一樣會失敗。
func runPostgres(ctx context.Context, scripts []string) (*tcpostgres.PostgresContainer, error) {
	return tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("notif_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.WithInitScripts(scripts...),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(90*time.Second),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	)
}

func setupTemplateDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	migDir, err := filepath.Abs(filepath.Join("..", "..", "db", "migrations"))
	require.NoError(t, err)
	entries, err := os.ReadDir(migDir)
	require.NoError(t, err)
	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			scripts = append(scripts, filepath.Join(migDir, e.Name()))
		}
	}
	sort.Strings(scripts)
	require.NotEmpty(t, scripts)

	var container *tcpostgres.PostgresContainer
	var startErr error
	for attempt := 1; attempt <= 2; attempt++ {
		container, startErr = runPostgres(ctx, scripts)
		if startErr == nil {
			break
		}
		t.Logf("postgres container start attempt %d/2 failed: %v", attempt, startErr)
		if container != nil {
			_ = container.Terminate(ctx)
			container = nil
		}
	}
	require.NoError(t, startErr, "start postgres container")

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() { pool.Close(); _ = container.Terminate(ctx) }
}

// loadAllTemplates 讀**全表**，包含 enabled = false。
//
// ⚠️ 刻意不在 SQL 過濾 enabled：本檔的檢查分兩類——
//   - **需要 golden 的**（第 1~4、L）只對 enabled row 有意義；仍在規劃中的停用
//     row 沒有 golden 條目可對，逐 row 在迴圈內 skip。
//   - **不需要 golden 的**（refType 值域、K 成對、M url 渲染、N 反向清單）對**全表**
//     生效。理由是這個 class 的失效點正好在「啟用的那一天」：停用 row 不受任何斷言
//     約束，一旦被 enable 就直接流到 GET /app/notifications，而 enable 通常只是一行
//     UPDATE 或一次 admin API 呼叫，不會有人回來重看契約。
//     實例：procurement quote-invite rows 在啟用前也必須先通過這些規則。
func loadAllTemplates(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []templateRow {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT event_type, target_user_type, target_user_field,
		       title_template, body_template, image_key_template,
		       reference_id_template, schema_url, data_type, category,
		       data_fields, payload_filter, enabled, identity_migrated
		FROM notification_templates
		ORDER BY event_type, target_user_type`)
	require.NoError(t, err)
	defer rows.Close()

	var out []templateRow
	for rows.Next() {
		var r templateRow
		var dataFields, payloadFilter []byte
		require.NoError(t, rows.Scan(
			&r.EventType, &r.TargetUserType, &r.TargetUserField,
			&r.TitleTemplate, &r.BodyTemplate, &r.ImageKeyTemplate,
			&r.ReferenceIDTemplate, &r.SchemaURL, &r.DataType, &r.Category,
			&dataFields, &payloadFilter, &r.Enabled, &r.IdentityMigrated))
		if len(dataFields) > 0 {
			require.NoError(t, json.Unmarshal(dataFields, &r.DataFields))
		}
		if len(payloadFilter) > 0 {
			require.NoError(t, json.Unmarshal(payloadFilter, &r.PayloadFilter))
		}
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func assertInvoiceSellerTemplatesUseStoreID(t *testing.T, templates []templateRow) {
	t.Helper()

	expected := map[string]struct{}{
		"invoice.issued.seller":                            {},
		"invoice.issue_failed.seller":                      {},
		"invoice.operation.review_required.seller":         {},
		"invoice.complaint.created.seller":                 {},
		"invoice.complaint.overdue.seller":                 {},
		"invoice.seller_tax_profile.status_changed.seller": {},
	}

	found := make(map[string]templateRow, len(expected))
	for _, tmpl := range templates {
		if tmpl.TargetUserType != "seller" {
			continue
		}
		if _, ok := expected[tmpl.EventType]; !ok {
			continue
		}
		require.NotContains(t, found, tmpl.EventType)
		found[tmpl.EventType] = tmpl
	}

	require.Len(t, found, len(expected))
	for eventType := range expected {
		tmpl, ok := found[eventType]
		require.Truef(t, ok, "missing seller invoice template %q", eventType)
		require.Equal(t, "store_id", tmpl.TargetUserField, eventType)
		require.True(t, tmpl.IdentityMigrated, eventType)
	}
}

func assertFinalRecipientIdentityContract(t *testing.T, templates []templateRow) {
	t.Helper()

	for _, tmpl := range templates {
		if tmpl.TargetUserType == "seller" {
			require.Truef(t, tmpl.IdentityMigrated,
				"seller template %q must use store_id and identity_migrated=true", tmpl.EventType)
		}
	}

	wantBuyerFields := map[string]string{
		"payment.rescue.refund_completed/buyer": "buyer_id",
		"product.restock.target/buyer":          "buyer_id",
	}
	foundBuyer := make(map[string]templateRow, len(wantBuyerFields))
	for _, tmpl := range templates {
		key := tmpl.EventType + "/" + tmpl.TargetUserType
		if want, ok := wantBuyerFields[key]; ok {
			require.NotContains(t, foundBuyer, key)
			foundBuyer[key] = tmpl
			require.Equalf(t, want, tmpl.TargetUserField,
				"buyer template %q must use the canonical buyer_id payload field", tmpl.EventType)
		}
	}
	require.Len(t, foundBuyer, len(wantBuyerFields), "final buyer cleanup templates must exist")
}

// TestTemplateContract 是本 issue guardrail 主體。
func TestTemplateContract(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTemplateDB(ctx, t)
	defer cleanup()

	golden := loadGolden(t)
	pending := loadKnownPending(t)
	identityPending := loadIdentityPending(t)
	actorFields := loadActorFields(t)
	templates := loadAllTemplates(ctx, t, pool)
	require.NotEmpty(t, templates)
	assertInvoiceSellerTemplatesUseStoreID(t, templates)
	assertFinalRecipientIdentityContract(t, templates)

	// 反向：golden 的每個條目都必須有對應的 template（全表，含 disabled）。
	//
	// golden 是**模板↔publisher 契約的測試 fixture**，不是事件目錄（那是 36 號）。
	// 沒有 template 消費的條目對這份契約零貢獻，卻會安靜地腐爛——沒有人會回頭重驗
	// 它，而讀的人會以為它受本測試保護。實例：`return.shipped` /
	// `order.shipped.merchant` / `payment.rescue.refund_completed.seller` 三支的
	// template 早在 20260703130001 與 20260703140000 就因語意被取代／0 delivery 而
	// 刪除，golden 條目卻留了下來（2026-08-16 清掉）。
	//
	// 退役事件的 payload 記錄歸 36 號設計書，不歸這裡。
	{
		have := map[string]bool{}
		for _, tmpl := range templates {
			have[tmpl.EventType] = true
		}
		for eventType := range golden {
			require.Truef(t, have[eventType],
				"golden 有 %q 但 notification_templates 全表（含 disabled）沒有任何 template 消費它——"+
					"template 被刪時要同步刪 golden 條目；事件本身的 payload 記錄歸 36 號設計書", eventType)
		}
	}

	for _, tmpl := range templates {
		key := tmpl.EventType + "/" + tmpl.TargetUserType
		name := key
		if !tmpl.Enabled {
			name += " (disabled)"
		}
		t.Run(name, func(t *testing.T) {
			// 6.5) 【全表，含 disabled】refType 的 const 值必須落在 13 號 §5.2 宣告的
			//      封閉值域內。
			//
			//      為什麼是「全表」而不是「enabled」：這個 class 的失效點正好在**啟用的
			//      那一天**。停用 row 不受 golden 相關規則約束（它們本來就沒有 golden），
			//      如果值域也只查 enabled，一支帶著值域外 refType 的模板可以安安靜靜躺在
			//      表裡，直到某人跑一行 `UPDATE ... SET enabled = true` 或按下 admin 的
			//      開關，它就直接流到 GET /app/notifications —— 而 K/L/M/N 沒有一條會紅。
			//      實例：procurement quote-invite row 曾經是這個規則加入時的
			//      未啟用 future contract，現在仍受同一條全表規則保護。
			//
			//      §5.2 之所以是「封閉」值域：新增一個值不是只改後端，還要同步 gateway
			//      §6.4 的 fallback 表、兩支 App 的 inbox switch 與 FCM router、website
			//      `lib/notifications/resolve.ts` 的三個集合。沒有這條斷言，「不新創值」
			//      就只是 migration 註解裡的一句話（02-judgment §2.1.1：訂新規範時要同時
			//      訂出「違反它的東西怎麼認出來」）。
			if spec, ok := tmpl.DataFields["refType"]; ok && spec.Const != nil {
				require.Containsf(t, refTypeDomain, *spec.Const,
					"refType const %q 不在 13 號 §5.2 宣告的封閉值域內。"+
						"要新增值域成員，先改 §5.2 的值域清單＋對照表、評估 gateway fallback 表與"+
						"三個前端消費者，再把它加進本檔的 refTypeDomain —— 順序不可顛倒",
					*spec.Const)
			}

			// 7) 【全表，先於所有 skip】schema_url 渲染後不得殘留 `{`。
			//
			// RenderBraces 找不到 key 時**原樣保留** `{key}`（template_handler.go:376-380），
			// 於是送出去的 data.url 是一個沒渲染的模板字串。這個失效沒有任何訊號：
			// 通知照送、內文照對、outbox done，只有 url 是壞的。
			//
			// 兩種成因都擋得住：① data_fields 整組是 NULL / 少了那個鍵（3 支售後模板即此）
			// ② brace 鍵名與 data_fields 的鍵名不一致（訂單全系列的 {order_id} vs data 的
			// orderId、貼文三支的 {post_id} vs postId、`#{deepLink}` 這種 schema_url 根本
			// 不會經過的語法）。修復前實查 57 支有 schema_url 的 enabled template，23 支中招。
			//
			// 這條**刻意放在廣播與 known_pending 的 skip 之前**：broadcastTemplate 同樣會
			// RenderBraces（template_handler.go:226-231），廣播模板的 url 壞掉一樣送得出去。
			// payload 用「template 自己讀的欄位」而非 golden 造，讓本條與 golden 無耦合——
			// 「這些欄位 publisher 到底發不發得出來」由下面第 1~4 條各自負責。
			assertSchemaURLRenders(t, tmpl)

			// 5) 【全表，含 disabled】refType 與 refId 必須成對。gateway 只在兩者皆
			//    存在且皆非空時才回傳（gateway/internal/handler/notification_inbox.go:41-46），
			//    只給其中一個 = 兩個都沒給，而模板這邊看起來是有寫的。
			//    這條不需要 golden，所以與值域／M／N 同組對全表生效。
			_, hasRefType := tmpl.DataFields["refType"]
			_, hasRefID := tmpl.DataFields["refId"]
			require.Equalf(t, hasRefType, hasRefID,
				"data_fields 的 refType/refId 只給了一個（refType=%v refId=%v）——"+
					"gateway 兩者皆非空才回傳，缺一即整組降級成 null", hasRefType, hasRefID)

			// 8) 【全表，先於所有 skip】刻意不給 refType/refId 的模板必須維持沒有。
			//    第 5、6 條擋的是「該給而沒給」，這條擋反方向：**不該給而被硬塞**。
			//    沒有這條，下一個人做同類 sweep 時很容易替公告 / 追蹤這種沒有對應資源的
			//    事件挑一個「看起來最接近」的 refType，前端就會拿它去開不存在的頁面。
			if reason, ok := deliberatelyNoRefContract[key]; ok {
				_, hasRefType := tmpl.DataFields["refType"]
				_, hasRefID := tmpl.DataFields["refId"]
				require.Falsef(t, hasRefType || hasRefID,
					"%s 被列為「刻意不給 refType/refId」但 data_fields 裡出現了——"+
						"理由：%s。若確定要給，先改 13 號 §5.2 的值域與對照表，再從本清單移除", key, reason)
			}

			// 9) 【全表，含 disabled】target_user_field 與 actor 欄位裝的值必須落在
			//    target_user_type 對應的身分空間（13 號 §12.1）。
			//
			//    這是 #1626 那個 bug class 的**預防面**：既有的第 1 條只驗
			//    「target_user_field 這個 key 在不在 emitted_keys 內」，驗不出那個 key
			//    裝的是哪一個身分空間 —— 而填錯身分空間正是整個 class 的形狀。
			//    scripts/audit-notification-identity.py 抓的是已經寫進 DB 的漂移（偵測），
			//    這一條在 PR 階段擋住新的填錯（預防），兩者不可互相取代。
			//
			//    刻意對**全表**生效，理由與值域／K／M／N 同組：停用 row 的錯置在
			//    `UPDATE ... SET enabled = true` 的那一刻才生效，而那時沒有人會回來重看契約。
			assertIdentitySpace(t, tmpl, key, golden, actorFields, pending, identityPending)

			// ── 以下全部需要 golden，只對 enabled row 有意義 ──────────────────
			// 停用 row 的 publisher 可能還沒落地，沒有 emitted_keys 可對。它們已經
			// 過上面四條與 golden 無關的規則（值域 / K 成對 / M url / N 反向清單）。
			if !tmpl.Enabled {
				t.Skip("disabled template：golden 相關檢查略過（值域 / 成對 / url / 反向清單已驗）")
			}

			if reason, ok := pending[key]; ok {
				t.Skipf("KNOWN_PENDING（後續 PR 修）: %s", reason)
			}

			g, ok := golden[tmpl.EventType]
			require.Truef(t, ok, "event_type %q 無 golden 條目——新增 template 必須同步 tests/contract/golden/event_payload_keys.json", tmpl.EventType)
			emitted := map[string]bool{}
			for _, k := range g.EmittedKeys {
				emitted[k] = true
			}

			// 1) target_user_field 必須被 publisher 實發（否則無收件人 → 通知靜默遺失）
			require.NotEmpty(t, tmpl.TargetUserField)
			require.Truef(t, emitted[tmpl.TargetUserField],
				"target_user_field %q 不在 %s(%s) golden emitted_keys 內", tmpl.TargetUserField, g.Publisher, g.RoutingKey)

			// 2) 無 default 的 #{placeholder}（title/body/image/reference）必須被實發
			//    復用 handler.RenderTemplate 的解析（正確處理 #{x|default} 視為可選）
			payload := map[string]interface{}{}
			for k := range emitted {
				payload[k] = "x" // 非空值，讓「有實發的 key」不被列 missing
			}
			for field, tpl := range map[string]string{
				"title_template":        tmpl.TitleTemplate,
				"body_template":         tmpl.BodyTemplate,
				"image_key_template":    deref(tmpl.ImageKeyTemplate),
				"reference_id_template": deref(tmpl.ReferenceIDTemplate),
			} {
				_, missing := worker.RenderTemplate(tpl, payload)
				require.Emptyf(t, missing,
					"%s 的必填 #{placeholder} %v 不在 %s(%s) golden emitted_keys 內",
					field, missing, g.Publisher, g.RoutingKey)
			}

			// 3) data_fields.field 值必須被實發（否則該 FCM data key 缺漏）
			for fcmKey, spec := range tmpl.DataFields {
				if spec.Field == nil {
					continue // const 不讀 payload
				}
				require.Truef(t, emitted[*spec.Field],
					"data_fields[%q].field=%q 不在 %s(%s) golden emitted_keys 內", fcmKey, *spec.Field, g.Publisher, g.RoutingKey)
			}

			// 4) payload_filter key 必須被實發（否則 filter 永不命中 → 通知永不發）
			for fk := range tmpl.PayloadFilter {
				require.Truef(t, emitted[fk],
					"payload_filter key %q 不在 %s(%s) golden emitted_keys 內", fk, g.Publisher, g.RoutingKey)
			}

			// 6) 白名單內的事件必須真的交得出跳轉目標。第 5 條擋的是「寫了一半」，
			//    這一條擋的是「整組沒寫」——那種情況第 1~5 條全綠，通知照送、內文
			//    照對，只有收件人點不進去（#2419 代行完成通知即此）。
			if want, ok := requiredRefContract[key]; ok {
				spec, ok := tmpl.DataFields["refType"]
				require.Truef(t, ok, "data_fields 缺 refType —— 收件人點開通知後沒有跳轉目標")
				require.NotNilf(t, spec.Const, "refType 必須是 const，不能取自 payload 欄位")
				require.Equal(t, want.RefTypeConst, *spec.Const, "refType 值與契約不符")

				idSpec, ok := tmpl.DataFields["refId"]
				require.Truef(t, ok, "data_fields 缺 refId —— 收件人點開通知後沒有跳轉目標")
				require.NotNilf(t, idSpec.Field, "refId 必須取自 payload 欄位，不能是 const")
				require.Equal(t, want.RefIDField, *idSpec.Field, "refId 的來源欄位與契約不符")
				require.Truef(t, emitted[*idSpec.Field],
					"refId 來源欄位 %q 不在 %s(%s) golden emitted_keys 內", *idSpec.Field, g.Publisher, g.RoutingKey)

				// 宣告對了不等於引擎吐得出來——真的跑一次 BuildDataMap（就是
				// handleTemplate 寫 inbox 前用的那一支），因為 gateway 讀的是它的
				// 輸出，不是 data_fields 本身。payload 用 golden 的 emitted keys 造，
				// 所以這一段同時證明「publisher 發的欄位餵得動這個模板」。
				dataMap := worker.BuildDataMap(tmpl.DataFields, payload)
				require.NotEmptyf(t, dataMap["refType"],
					"BuildDataMap 產出的 data 缺 refType —— gateway 會回 refType=null")
				require.NotEmptyf(t, dataMap["refId"],
					"BuildDataMap 產出的 data 缺 refId —— gateway 會回 refId=null")
			}
		})
	}
}

// TestGoldenEntriesHaveAttributedPublisher 要求 golden 的每個條目都追得到一個真的
// publisher。
//
// `publisher` 欄位不是註解，它是 §14 這整套 guardrail 的**可追溯性前提**：TestTemplateContract
// 的每一條錯誤訊息都用它告訴人「去哪個服務改」，而 §12.1 的第 2 步（追值的來源）也是從它
// 起跳。填 "unknown" 的條目等於一份沒有人負責、也沒有人會回頭重驗的 emitted_keys 快照
// —— 讀的人卻會以為它受本測試保護（同一個病的另一面：README「刪除 template」那條，
// 2026-08-16 清掉三支沒有 template 消費的幽靈條目）。
//
// 不需要 Postgres：本斷言只讀 golden，刻意獨立於 TestTemplateContract，這樣它在沒有
// Docker 的環境也跑得起來、紅的時候也不必先等一個容器。
func TestGoldenEntriesHaveAttributedPublisher(t *testing.T) {
	golden := loadGolden(t)
	require.NotEmpty(t, golden)

	var unattributed []string
	for eventType, entry := range golden {
		if entry.Publisher == "" || entry.Publisher == "unknown" {
			unattributed = append(unattributed, eventType+" (publisher="+entry.Publisher+")")
		}
	}
	sort.Strings(unattributed)

	require.Emptyf(t, unattributed,
		"golden 條目的 publisher 追不到任何服務：%v。\n"+
			"每個條目都必須指名發出它的服務——查不到 publisher 就表示這個 event 沒有生產者，"+
			"正解是連同它的 template 一起刪掉（README「刪除 template」節），不是把 publisher 填成 unknown。",
		unattributed)
}

// assertIdentitySpace 驗證這張 template 讀的身分欄位落在正確的身分空間。
//
// 規則是「宣告過且相符，否則 FAIL」而不是「宣告過才檢查」：後者會讓沒宣告的欄位
// 靜默通過，於是新加的 template 只要不碰 golden 就永遠不受這條約束 —— 那正是
// 這個 class 得以反覆發生的結構。所以未查證的必須明確登記在
// known_pending_identity.json 裡，**零筆未驗證的斷言**。
//
// known_pending.json 列的一併放行：那份清單的語意是「這張 template 的 publisher
// 契約整體還無從對照」，身分空間自然也追不到，不需要在兩個檔案各登記一次。
func assertIdentitySpace(
	t *testing.T,
	tmpl templateRow,
	key string,
	golden map[string]goldenEntry,
	actorFields map[string]string,
	pending, identityPending map[string]string,
) {
	t.Helper()
	if tmpl.TargetUserField == "" {
		return // 廣播 template：無 1:1 收件人語意，沒有身分欄位可驗
	}
	if reason, ok := identityPending[key]; ok {
		t.Logf("身分空間檢查 KNOWN_PENDING：%s", reason)
		return
	}
	if reason, ok := pending[key]; ok {
		t.Logf("身分空間檢查隨 known_pending.json 一併略過：%s", reason)
		return
	}

	want, ok := expectedIdentitySpaceFor(tmpl)
	require.Truef(t, ok, "未知的 target_user_type %q —— 13 號 §12.1 只定義了 seller / buyer", tmpl.TargetUserType)

	g, ok := golden[tmpl.EventType]
	require.Truef(t, ok,
		"event_type %q 無 golden 條目，無從宣告身分空間 —— 新增 template 必須同步 "+
			"tests/contract/golden/event_payload_keys.json，publisher 還沒落地就登記進 known_pending_identity.json",
		tmpl.EventType)

	assertKeyIdentitySpace(t, g, tmpl, want, tmpl.TargetUserField, "target_user_field")

	// actor 欄位（self-notification skip 的比對來源）必須與 target 落在同一空間，
	// 否則比對永遠不成立、self-skip 靜默失效：沒有錯誤、沒有 metric，只有收件人
	// 收到自己觸發的通知。template_handler.go 的 actorFieldByEventType 上方有同一
	// 條規則的散文版，這裡是它的機械版。
	if actorField, ok := actorFields[tmpl.EventType]; ok {
		assertKeyIdentitySpace(t, g, tmpl, want, actorField, "actorFieldByEventType")
	}
}

func assertKeyIdentitySpace(t *testing.T, g goldenEntry, tmpl templateRow, want, payloadKey, role string) {
	t.Helper()
	space, ok := g.IdentitySpaces[payloadKey]
	require.Truef(t, ok,
		"%s %q 沒有在 golden 的 identity_spaces 宣告身分空間。"+
			"先追到 %s(%s) 真正填進這個欄位的值是 account_id 還是 buyer_id 再宣告；"+
			"追不到就登記進 known_pending_identity.json，不要猜",
		role, payloadKey, g.Publisher, g.RoutingKey)

	got, ok := space.resolve(tmpl.TargetUserType)
	require.Truef(t, ok,
		"%s %q 宣告成 role-dependent，但沒有列到 target_user_type=%q 這個角色",
		role, payloadKey, tmpl.TargetUserType)

	require.Equalf(t, want, got,
		"%s %q 的身分空間是 %q，但 target_user_type=%q 的收件匣要的是 %q（13 號 §12.1）。"+
			"填錯是完全無聲的失效：通知照寫、HTTP 200、不進 DLQ，只有收件人永遠讀不到，"+
			"而首任帳號 grandfather 恆等會讓所有 fixture 照樣全綠",
		role, payloadKey, got, tmpl.TargetUserType, want)
}

// assertSchemaURLRenders 用 template 自己的 data_fields 造一份「publisher 什麼都發得出來」
// 的 payload，跑一次 handleTemplate 真正用的那條路徑（BuildDataMap → 補 type/category →
// RenderBraces），斷言渲染後的 url 沒有殘留的 `{`。
//
// 之所以不用 golden 造 payload：本條要問的是「brace 鍵名對不對得上 data map 的鍵名」，
// 那是 template 內部的一致性；「data_fields 讀的欄位 publisher 發不發得出來」是另一個
// 問題，由第 3 條負責。兩件事分開，任一條紅的時候不用猜是哪個原因。
func assertSchemaURLRenders(t *testing.T, tmpl templateRow) {
	t.Helper()
	if tmpl.SchemaURL == nil || *tmpl.SchemaURL == "" {
		return
	}
	payload := map[string]interface{}{}
	for _, spec := range tmpl.DataFields {
		if spec.Field != nil {
			payload[*spec.Field] = "x" // 非空值：BuildDataMap 對空字串仍會寫鍵，但這裡要模擬正常情況
		}
	}
	dataMap := worker.BuildDataMap(tmpl.DataFields, payload)
	dataMap["type"] = tmpl.DataType
	dataMap["category"] = tmpl.Category

	rendered := worker.RenderBraces(*tmpl.SchemaURL, dataMap)
	require.NotContainsf(t, rendered, "{",
		"schema_url %q 渲染後仍含未替換的佔位符（得到 %q）——送到裝置上的 data.url 會是一段"+
			"模板字串。修法是把 brace 的鍵名改成 data_fields 已經有的那個鍵（或補上缺的鍵），"+
			"不要新增 snake_case 別名鍵（那會進每一則 FCM payload 並改變 App 端查找優先序）。"+
			"目前 data map 的鍵：%v",
		*tmpl.SchemaURL, rendered, sortedKeys(dataMap))
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// refTypeDomain 是 `data_fields.refType.const` 的**封閉值域**，SSOT 是
// 13 號設計書 §5.2（本清單是它的機械化副本，兩邊必須一起改）。
//
// 為什麼需要它：migration 註解寫「一律取自既有值域，不新創值」只是一句話，
// 而 requiredRefContract + deliberatelyNoRefContract 加起來只覆蓋 20/78 支，
// 其餘 58 支的 refType 值原本不受任何斷言約束。02-judgment §2.1.1：**訂新規範時
// 要同時訂出「違反它的東西怎麼認出來」**，否則規範只擋得住願意讀它的人。
//
// 新增值域成員的順序（不可顛倒）：
//  1. 13 號 §5.2 的值域清單與對照表加該值，寫明它指向什麼資源
//  2. 評估四個消費者：gateway §6.4 fallback 表、seller App inbox switch＋FCM router、
//     buyer App `destination.dart`、website `lib/notifications/resolve.ts` 的三個集合。
//     **沒有消費者不是阻擋條件**（DELEGATION 與 PROCUREMENT_QUOTE_INVITE 現況即是），
//     但必須在 §5.2 明寫「本期無落地頁」，免得下一個人以為是漏接
//  3. 才把它加進這裡
//
// ⚠️ PROCUREMENT_QUOTE_INVITE（55 號採購助手）目前三個前端都沒有任何分支，
// 但 backend seller notification contract 已固定使用 inviteId／batchId 作為
// reference；這兩個欄位是既有值域裡沒有任何值描述得了的資源，硬塞 ORDER／POST
// 才是這個 class 真正要防的錯誤。
var refTypeDomain = []string{
	"ORDER",
	"AFTERSALE",
	"CHAT",
	"POST",
	"PRODUCT",
	"COUPON",
	"SHIPMENT_ANOMALY",
	"SELLER_SUSPENSION",
	"MERCHANT_VERIFICATION",
	"DELEGATION",
	"PROCUREMENT_QUOTE_INVITE",
	// INVOICE（電子發票，58 號 §D.4）：文件類八列的 refId ＝ invoice_document_id。
	// 舊版 58 曾主張沿用 ORDER，那個論證只涵蓋「賣家→買家（有訂單）」一個方向；
	// §2.3 矩陣的另外三個方向（平台→賣家服務費／平台→買家訂閱／平台→非會員）
	// source_ref 恆為 NULL ⇒ 沿用 ORDER 的後果不是多跳一次，是 refId **沒有值可填**。
	// 四消費者評估已在 58 §D.4 逐項寫明：WEB 通知中心本波實作
	// （/account/invoices/{id}、/seller-center/invoices/{id}）；兩支 App 因本波凍結
	// 而落進 switch 的 default（**本期無落地頁**，與沿用 ORDER 時等價——App 的訂單
	// 詳情本來就沒有發票區塊）；ADM 不消費 refType。
	"INVOICE",
}

// requiredRefContract 是「這則通知必須讓收件人點得進去」的白名單，
// key = event_type + "/" + target_user_type。
//
// 只列**曾經漏過、且漏了不會有任何其他訊號**的事件，不是全表清單：其餘模板
// 是否該有 refType/refId 由第 5 條的成對檢查與各自的設計書決定，把整表塞進來
// 會讓這份清單變成第二份 SSOT。
//
// delegation：refId 用 audit_log_id 而非 resource_id，因為訂單面的 resource_id
// 恆為空字串（訂單號存不進 bigint 欄位），而 gateway 要求 refId 非空才回傳；
// audit_log_id 恆非空且與本列 reference_id_template 同值。完整理由見
// db/migrations/20260816093000_delegation_template_ref_fields.sql。
//
// 其餘 15 支是同一個 root cause 的 class 級修復（§17.7「相同 root cause 一次全修」）：
// 修復前實查全表 77 支 enabled template，20 支交不出 refType/refId；代行那支由上一個
// migration 修掉，剩 19 支在 20260816120000_template_ref_fields_and_schema_url_keys.sql
// 處置——15 支補齊列在這裡，4 支刻意不給列在 deliberatelyNoRefContract。
// 每一支的 refId 來源欄位都對過 publisher 原始碼（不是只對 golden），逐支理由見該 migration。
var requiredRefContract = map[string]struct {
	RefTypeConst string
	RefIDField   string
}{
	"delegation.operation.completed.seller/seller": {RefTypeConst: "DELEGATION", RefIDField: "audit_log_id"},

	// 售後：5 支漏網的與既有 33 支同型（refType=AFTERSALE、refId=case_no）。
	// 其中 3 支的 data_fields 原本整個是 NULL，連 schema_url 的 {caseNo} 都渲染不出來。
	"aftersales.case.auto_completed_return_pending_timeout.buyer/buyer":   {RefTypeConst: "AFTERSALE", RefIDField: "case_no"},
	"aftersales.case.auto_completed_return_pending_timeout.seller/seller": {RefTypeConst: "AFTERSALE", RefIDField: "case_no"},
	"aftersales.case.auto_escalated.buyer/buyer":                          {RefTypeConst: "AFTERSALE", RefIDField: "case_no"},
	"aftersales.case.auto_escalated.seller/seller":                        {RefTypeConst: "AFTERSALE", RefIDField: "case_no"},
	"aftersales.case.return_pending_24h.buyer/buyer":                      {RefTypeConst: "AFTERSALE", RefIDField: "case_no"},

	// 貼文/留言：與買家側對應模板對稱（comment.created.buyer / comment.replied.buyer
	// 早就是 POST + postId）。wire 大小寫依 publisher 而異：social-service 的
	// comment.* 系列是 camelCase、post.liked 是 snake_case。
	"comment.created/seller":        {RefTypeConst: "POST", RefIDField: "postId"},
	"comment.like_changed/seller":   {RefTypeConst: "POST", RefIDField: "postId"},
	"comment.replied.seller/seller": {RefTypeConst: "POST", RefIDField: "postId"},
	"post.liked/seller":             {RefTypeConst: "POST", RefIDField: "post_id"},

	// 商品互動：PRODUCT + product_id，與既有 product.restock.target 一致。
	// ⚠️ product.favorited 綁的是 buyer.favorite.added（BuyerFavoriteEvent），不是同名
	// routing key——refId 欄位要對那個 payload，不要看 event_type 猜。
	"product.favorited/seller": {RefTypeConst: "PRODUCT", RefIDField: "product_id"},
	"product.viewed/seller":    {RefTypeConst: "PRODUCT", RefIDField: "product_id"},

	// 實名認證：MERCHANT_VERIFICATION 是 gateway §6.4 fallback 表既有的值，但那條
	// fallback 讀的 certification_id 從來沒有 publisher 發過；payload 裡唯一的 id 是
	// merchant_id（= 收件人自己的 account_id）。認證資料與商家 1:1，故以它為資源鍵；
	// 目的地 /seller/certification 是登入者自己的頁面，不吃路徑參數。
	"merchant.verification.approved/seller":     {RefTypeConst: "MERCHANT_VERIFICATION", RefIDField: "merchant_id"},
	"merchant.verification.forwarded/seller":    {RefTypeConst: "MERCHANT_VERIFICATION", RefIDField: "merchant_id"},
	"merchant.verification.pending_info/seller": {RefTypeConst: "MERCHANT_VERIFICATION", RefIDField: "merchant_id"},
	"merchant.verification.rejected/seller":     {RefTypeConst: "MERCHANT_VERIFICATION", RefIDField: "merchant_id"},
	"merchant.verification.under_review/seller": {RefTypeConst: "MERCHANT_VERIFICATION", RefIDField: "merchant_id"},
}

// deliberatelyNoRefContract 是 requiredRefContract 的反向清單：**確定不該有**
// refType/refId 的事件，key 格式相同，值是理由。
//
// 為什麼需要它：只有「該給的清單」時，審查只能看出「漏了什麼」，看不出「多了什麼」。
// 而這個 class 的 sweep 最容易犯的錯，就是替一個沒有對應資源的事件挑一個「看起來
// 最接近」的 refType——那會讓前端拿著一個不是資源 id 的值去開頁面，比不跳更糟
// （13 號 §5.2 對 refType 值域的規定就是為此存在）。
//
// 要把某一支從這裡移到 requiredRefContract，前置條件是先在 13 號 §5.2 的值域與對照表
// 立好那個 refType，而不是先改 migration。
//
// 2026-08-22 移除三筆（`notification.system.maintenance/buyer`、
// `notification.terms.update/buyer`、`notification.announcement/seller`）：那三張
// template 本身已刪除（20260822130000，三者都沒有 publisher），清單只釘活著的 row。
var deliberatelyNoRefContract = map[string]string{
	// 電子發票四支（58 號 §D.4 指名登記）。三種不同的理由，不要合併成一句：
	"invoice.issue_failed.seller/seller": "開票**失敗**時 invoice_documents 還沒有那一列 ⇒ " +
		"沒有 invoice_document_id 可指。schema_url 指向 jobs 列表（S08），異常詳情由列表點進去。",
	"invoice.complaint.created.seller/seller": "申訴的主體是**申訴**不是文件，而 refTypeDomain 裡沒有描述得了申訴的值；" +
		"complaint_id 雖然在 payload 內，硬挑一個「看起來最接近」的 refType（例如 INVOICE）會讓前端拿申訴 id 去開發票詳情。" +
		"schema_url 指向申訴列表（S09），該前綴對停權賣家也走得到。",
	"invoice.complaint.overdue.seller/seller": "同 invoice.complaint.created.seller：申訴不是 refTypeDomain 描述得了的資源，指向申訴列表（S09）。",
	"invoice.seller_tax_profile.status_changed.seller/seller": "賣家稅務身分不對應任何 refTypeDomain 資源，且 P11 是唯一沒有 order_no 與買家身分欄的事件" +
		"（58 §6.3.3.3）；data_fields 整組為 NULL ⇒ schema_url 只能是靜態網址。先例：delegation.operation.completed.seller。",

	"procurement.rfq_publication_invite.created.seller/seller": "RFQ 發布 invite（64 號設計書 §4.2）不是 refTypeDomain 描述得了的資源：" +
		"`PROCUREMENT_QUOTE_INVITE` 是 §3.6l 報價單邀請那條線的另一張表，塞它會讓前端拿 publication invite id 去開報價單頁——" +
		"正是本條與值域規則要防的錯誤。賣家回覆面（§4.4 的 sellerResponseVersion）本波尚未實作，也沒有落地頁可指，" +
		"故 schema_url 一併維持 NULL。⚠️ 本期無落地頁**不是**阻擋條件（13 號 §5.2 逐字寫明），不擴充值域是產品決定未定案；" +
		"本列已同步寫進 13 號 §5.2 對照表，解除順序見 db/migrations/20260910120000_procurement_rfq_publication_invite_template.sql 檔頭。",

	"follow.added/seller": "「被誰追蹤」的資源是那個買家，值域裡沒有 BUYER/USER/FOLLOWER；" +
		"publisher 只發 follower_id / follower_name / merchant_id，其中 merchant_id 是收件人自己。" +
		"（seller App _iconConfig 有 'USER'/'FOLLOWER' 兩個 case，但那是前端預留的圖示分支，" +
		"後端從未產出過這兩個值，不能當值域依據）",
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
