package handler

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)


const (
	scopeTestStoreID   = "2685914682"
	scopeTestProductID = "7494590641851072512"
)


type scopeAuditStub struct {
	accountpb.DelegationServiceClient
	intents  []*accountpb.RecordAuditIntentRequest
	confirms []*accountpb.ConfirmAuditResultRequest
}

func (s *scopeAuditStub) RecordAuditIntent(ctx context.Context, in *accountpb.RecordAuditIntentRequest, opts ...grpc.CallOption) (*accountpb.RecordAuditIntentResponse, error) {
	s.intents = append(s.intents, in)
	return &accountpb.RecordAuditIntentResponse{
		Data: &accountpb.RecordAuditIntentData{AuditLogId: "2701445588392017"},
	}, nil
}

func (s *scopeAuditStub) ConfirmAuditResult(ctx context.Context, in *accountpb.ConfirmAuditResultRequest, opts ...grpc.CallOption) (*accountpb.ConfirmAuditResultResponse, error) {
	s.confirms = append(s.confirms, in)
	return &accountpb.ConfirmAuditResultResponse{}, nil
}

func (s *scopeAuditStub) RecordAuditIntentBatch(ctx context.Context, in *accountpb.RecordAuditIntentBatchRequest, opts ...grpc.CallOption) (*accountpb.RecordAuditIntentBatchResponse, error) {
	resp := &accountpb.RecordAuditIntentBatchResponse{}
	for i, item := range in.GetItems() {
		s.intents = append(s.intents, item)
		resp.Results = append(resp.Results, &accountpb.RecordAuditIntentItemResult{
			ResourceId: item.GetResourceId(),
			Ok:         true,
			Data:       &accountpb.RecordAuditIntentData{AuditLogId: "27014455883920" + strconv.Itoa(20+i)},
		})
	}
	return resp, nil
}

func (s *scopeAuditStub) ConfirmAuditResultBatch(ctx context.Context, in *accountpb.ConfirmAuditResultBatchRequest, opts ...grpc.CallOption) (*accountpb.ConfirmAuditResultBatchResponse, error) {
	s.confirms = append(s.confirms, in.GetItems()...)
	return &accountpb.ConfirmAuditResultBatchResponse{}, nil
}

type scopeProductStub struct {
	productpb.ProductServiceClient
}

func fullProductDetail(name, status, version string) *productpb.ProductDetail {
	return &productpb.ProductDetail{
		Id:             scopeTestProductID,
		Version:        version,
		Name:           name,
		Status:         status,
		CategoryLv3Id:  "3001",
		CategoryName:   "水果",
		Origin:         "屏東",
		PriceMin:       "120",
		PriceMax:       "180",
		StockQty:       "36",
		Gallery:        []*productpb.GalleryItem{{AssetId: "9001", ObjectKey: "p1.jpg"}},
		Skus:           []*productpb.ProductSku{{Id: "8001", SkuName: "一箱"}},
		Hashtags:       []*productpb.HashtagItem{{Name: "tgap產銷履歷無籽檸檬"}},
		Certifications: []*productpb.Certification{{Name: "產銷履歷", CertNo: "TAP-001"}},
	}
}

func statusOnlyProductDetail(status, version string) *productpb.ProductDetail {
	d := fullProductDetail("無籽檸檬", status, version)
	d.Hashtags = nil
	d.Certifications = nil
	return d
}

func (s *scopeProductStub) GetProduct(ctx context.Context, in *productpb.GetProductRequest, opts ...grpc.CallOption) (*productpb.GetProductResponse, error) {
	return &productpb.GetProductResponse{
		Data: fullProductDetail("無籽檸檬", strconv.Itoa(4003), "12"),
	}, nil
}

func (s *scopeProductStub) CreateProduct(ctx context.Context, in *productpb.CreateProductRequest, opts ...grpc.CallOption) (*productpb.CreateProductResponse, error) {
	return &productpb.CreateProductResponse{
		Data: fullProductDetail(in.GetName(), strconv.Itoa(4001), "1"),
	}, nil
}

func (s *scopeProductStub) UpdateProduct(ctx context.Context, in *productpb.UpdateProductRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
	return &productpb.UpdateProductResponse{
		Data: fullProductDetail(in.GetName(), strconv.Itoa(4003), "13"),
	}, nil
}

func (s *scopeProductStub) UpdateSellerProductStatus(ctx context.Context, in *productpb.UpdateSellerProductStatusRequest, opts ...grpc.CallOption) (*productpb.UpdateProductResponse, error) {
	return &productpb.UpdateProductResponse{Data: statusOnlyProductDetail(in.GetStatus(), "13")}, nil
}

func (s *scopeProductStub) BatchGetSellerProductsForDelegation(ctx context.Context, in *productpb.BatchGetSellerProductsForDelegationRequest, opts ...grpc.CallOption) (*productpb.BatchGetSellerProductsForDelegationResponse, error) {
	resp := &productpb.BatchGetSellerProductsForDelegationResponse{}
	for _, id := range in.GetProductIds() {
		detail := fullProductDetail("無籽檸檬", strconv.Itoa(4004), "12")
		detail.Id = id
		resp.Results = append(resp.Results, &productpb.BatchProductSnapshot{
			ProductId: id, Found: true, Version: "12", Status: strconv.Itoa(4004),
			Name: "無籽檸檬", Detail: detail,
		})
	}
	return resp, nil
}

func (s *scopeProductStub) BatchUpdateSellerProductStatus(ctx context.Context, in *productpb.BatchUpdateSellerProductStatusRequest, opts ...grpc.CallOption) (*productpb.BatchUpdateSellerProductStatusResponse, error) {
	resp := &productpb.BatchUpdateSellerProductStatusResponse{}
	for _, item := range in.GetItems() {
		resp.Results = append(resp.Results, &productpb.BatchProductStatusResult{
			ProductId: item.GetProductId(), Ok: true, Version: "13",
		})
	}
	return resp, nil
}


func newDelegatedScopeRouter(t *testing.T) (*gin.Engine, *scopeAuditStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	audit := &scopeAuditStub{}
	h := &Holder{
		Account: grpcclient.NewAccountDelegationClientForTest(audit),
		Product: grpcclient.NewProductClientForTest(&scopeProductStub{}),
	}

	r := gin.New()
	g := r.Group("/api/v1/admin")
	g.Use(func(c *gin.Context) {
		middleware.SetDelegationSession(c, &accountpb.OperationSessionSummary{
			SessionId: "2701445580000001",
			Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
			Target:    &accountpb.DelegationTarget{StoreId: scopeTestStoreID},
			Status:    "active",
		})
		c.Next()
	})
	sellers := "/sellers/:store_id"
	g.POST(sellers+"/products", h.CreateDelegatedProduct)
	g.PATCH(sellers+"/products/:product_id", h.UpdateDelegatedProduct)
	g.POST(sellers+"/products/:product_id/publish", h.PublishDelegatedProduct)
	g.POST(sellers+"/products/:product_id/unpublish", h.UnpublishDelegatedProduct)
	g.POST(sellers+"/products/batch-unpublish", h.BatchUnpublishDelegatedProducts)
	return r, audit
}


// TestDelegatedAudit_ChangedFieldsStayInsideTheMutationScope 是本案的機械判準：
// 對每一支寫入端點跑一次完整的 orchestrate，斷言 changed_fields 的欄位集合恰好等於
// 該 action 動得到的欄位。
//
// 「動得到的欄位」在測試裡是**期望值**（人寫的），在程式裡是 Spec.SnapshotFields
// （宣告的）—— 兩者由這支測試對帳。第 14 支端點若沿用「寫入回應就是資源全貌」的錯誤
// 假設，這裡會立刻看到多出來的欄位。
func TestDelegatedAudit_ChangedFieldsStayInsideTheMutationScope(t *testing.T) {
	cases := []struct {
		action string
		method string
		path   string
		body   string
		want []string
	}{
		{
			action: "PRODUCT_PUBLISH",
			method: http.MethodPost, path: "/products/" + scopeTestProductID + "/publish",
			body: `{"expectedVersion":"12","operationReason":"協助上架"}`,
			want: []string{"status"},
		},
		{
			action: "PRODUCT_UNPUBLISH",
			method: http.MethodPost, path: "/products/" + scopeTestProductID + "/unpublish",
			body: `{"expectedVersion":"12","operationReason":"協助下架"}`,
			want: nil,
		},
		{
			action: "PRODUCT_BATCH_UNPUBLISH",
			method: http.MethodPost, path: "/products/batch-unpublish",
			body: `{"operationReason":"批次下架","items":[{"productId":"` + scopeTestProductID + `","expectedVersion":"12"}]}`,
			want: []string{"status"},
		},
		{
			action: "PRODUCT_UPDATE",
			method: http.MethodPatch, path: "/products/" + scopeTestProductID,
			body: `{"expectedVersion":"12","operationReason":"改名","name":"有機無籽檸檬"}`,
			want: []string{"name"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			r, audit := newDelegatedScopeRouter(t)
			w := doDelegatedScopeRequest(r, tc.method, tc.path, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
			}
			if len(audit.confirms) != 1 {
				t.Fatalf("ConfirmAuditResult 呼叫次數 = %d, want 1", len(audit.confirms))
			}
			got := changedFieldNames(audit.confirms[0])
			if !equalStringSets(got, tc.want) {
				t.Errorf("%s 的 changed_fields = %v, want %v\n"+
					"多出來的每一欄都是一筆從未發生的變更 —— changed_fields 是爭議時的舉證，"+
					"記到假的比少記更糟。修法是在該端點的 delegation.Spec 宣告 SnapshotFields。",
					tc.action, got, tc.want)
			}
		})
	}
}

// 建立類單獨一組：沒有 before，所以每一個「這次真的設了值」的欄位都是「— → 值」一列，
// 不受 SnapshotFields 收窄影響（建立本來就該記下它設了什麼）。
//
// ⚠️ 但**不是整張規則表**。建立時沒有給值的欄位，缺席的 before 與空值的 after 會渲染
// 成同一個字串（plain／truncate 的空形是「—」、qty 是「0 張」），那一列描述的是「從沒有
// 變成沒有」——它不是變更。changed_fields 是爭議時的舉證，記到假的比少記更糟：dev 三案
// 獨立重現「代行建立一則無配圖、無關聯商品的貼文」，稽核固定多出 imageKeys「0 張 →
// 0 張」與 productIds「0 項 → 0 項」兩列從未發生的變更，而稽核列不可竄改。
//
// 「建立當下這件東西長什麼樣」不靠 changed_fields 舉證，靠 after_snapshot —— Project()
// 仍存完整白名單投影，本區別不會讓任何資訊消失。
func TestDelegatedAudit_CreateActionsRecordTheWholeProjection(t *testing.T) {
	cases := []struct {
		action  string
		path    string
		body    string
		want    []string
		omitted string
	}{
		{
			action: "PRODUCT_CREATE", path: "/products",
			body: `{"operationReason":"代客建立","name":"無籽檸檬","categoryId":"3001",` +
				`"skus":[{"spec":"一箱","price":"120","stockQty":"36","unit":"3301"}]}`,
			want: []string{
				"name", "categoryName", "origin", "status", "priceMin", "priceMax",
				"stockQty", "skus", "images", "hashtags", "certifications",
			},
			omitted: "description（未填，兩側皆為「—」）",
		},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			r, audit := newDelegatedScopeRouter(t)
			w := doDelegatedScopeRequest(r, http.MethodPost, tc.path, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
			}
			got := changedFieldNames(audit.confirms[0])
			if !equalStringSets(got, tc.want) {
				t.Errorf("%s 的 changed_fields = %v, want %v（刻意不含 %s）",
					tc.action, got, tc.want, tc.omitted)
			}
		})
	}
}


// SnapshotFields 漏填 = 選了全表 = 稽核可能虛報，而它不編譯失敗、不轉紅、回應完全
// 正常。上面那張表守的是「已知的 12 個 action」，這一支守的是**第 13 個之後**：任何
// 新的 Orchestrate／OrchestrateBatch 呼叫點都必須明寫 SnapshotFields（要全表就寫
// nil），把「想過這件事」留在版控裡。
//
// 以 AST 而不是 grep 為錨：註解與字串裡出現 SnapshotFields 不算數，只有真的填在
// composite literal 的 key 才算（§5「命中的是程式碼還是註解」）。
func TestDelegatedOrchestrateCallSites_AllDeclareSnapshotFields(t *testing.T) {
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob 失敗: %v", err)
	}
	files := make([]string, 0, len(all))
	for _, f := range all {
		if !strings.HasSuffix(f, "_test.go") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatal("掃不到任何 .go —— 這支測試的錨點失效了，等於沒有閘門")
	}

	checked := 0
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("解析 %s 失敗: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Orchestrate" && sel.Sel.Name != "OrchestrateBatch") {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.CompositeLit)
				if !ok {
					continue
				}
				checked++
				if !compositeLitHasKey(lit, "SnapshotFields") {
					t.Errorf("%s:%d 的 %s 呼叫沒有宣告 SnapshotFields —— 漏填等於選了全表比較，"+
						"而狀態切換／媒體替換類端點在全表比較下會把寫入回應沒帶到的每一欄"+
						"記成「被清空」。要全表請明寫 SnapshotFields: nil。",
						path, fset.Position(lit.Pos()).Line, sel.Sel.Name)
				}
			}
			return true
		})
	}
	if checked < 5 {
		t.Errorf("只掃到 %d 個 Orchestrate 呼叫點，少於已知的 5 個 —— AST 錨點可能失效了", checked)
	}
}

func compositeLitHasKey(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == key {
			return true
		}
	}
	return false
}


func doDelegatedScopeRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/admin/sellers/"+scopeTestStoreID+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "ik-scope-00000001")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func changedFieldNames(confirm *accountpb.ConfirmAuditResultRequest) []string {
	out := make([]string, 0, len(confirm.GetChangedFields()))
	for _, cf := range confirm.GetChangedFields() {
		out = append(out, cf.GetField())
	}
	sort.Strings(out)
	return out
}

func equalStringSets(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	w := append([]string(nil), want...)
	sort.Strings(w)
	for i := range got {
		if got[i] != w[i] {
			return false
		}
	}
	return true
}


// TestDelegatedAudit_BatchBeforeSnapshotMatchesSinglePath 鎖住批次與單筆的 before
// 快照對稱。
//
// changed_fields 與 before_snapshot 是兩件不同的事，很容易被混為一談：
//   - changed_fields 收窄到 {status} 是**對的**（狀態切換只動得到 status，多記一欄就是
//     虛報一筆從未發生的變更，見本檔開頭）
//   - before_snapshot **不跟著收窄**：它是爭議時的舉證素材，要回答的是「平台動手當下，
//     這件商品長什麼樣」
//
// 批次曾經只在 before_snapshot 存一個 status 鍵，因為它的 before 來自
// BatchGetSellerProductsForDelegation，而該 RPC 當時只回 id/found/version/status/name。
// 於是「一次動 50 件」這個最高風險的操作，留下的舉證素材反而最少。
//
// 這支測試的判準刻意是**與單筆逐鍵相同**，而不是「至少 N 個鍵」：規則表日後長出第 13 欄
// 時，只有前者會在批次沒跟上時轉紅。
func TestDelegatedAudit_BatchBeforeSnapshotMatchesSinglePath(t *testing.T) {
	snapshotKeys := func(t *testing.T, method, path, body string) []string {
		t.Helper()
		r, audit := newDelegatedScopeRouter(t)
		w := doDelegatedScopeRequest(r, method, path, body)
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if len(audit.intents) != 1 {
			t.Fatalf("RecordAuditIntent 呼叫次數 = %d, want 1", len(audit.intents))
		}
		var projection map[string]any
		raw := audit.intents[0].GetBeforeSnapshotJson()
		if err := json.Unmarshal([]byte(raw), &projection); err != nil {
			t.Fatalf("before_snapshot 不是合法 JSON（%v）：%s", err, raw)
		}
		keys := make([]string, 0, len(projection))
		for k := range projection {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	single := snapshotKeys(t, http.MethodPost, "/products/"+scopeTestProductID+"/unpublish",
		`{"expectedVersion":"12","operationReason":"協助下架"}`)
	batch := snapshotKeys(t, http.MethodPost, "/products/batch-unpublish",
		`{"operationReason":"批次下架","items":[{"productId":"`+scopeTestProductID+`","expectedVersion":"12"}]}`)

	want := make([]string, 0, len(delegation.DelegationDiffRules["product"]))
	for f := range delegation.DelegationDiffRules["product"] {
		want = append(want, f)
	}
	if !equalStringSets(single, want) {
		t.Fatalf("PRODUCT_UNPUBLISH 的 before_snapshot 鍵集合 = %v, want 完整規則表 %v", single, want)
	}

	if !equalStringSets(batch, single) {
		t.Errorf("PRODUCT_BATCH_UNPUBLISH 的 before_snapshot 鍵集合 = %v，單筆是 %v\n"+
			"before_snapshot 是爭議時的舉證素材，批次正是「一次動很多件」的高風險操作，"+
			"不得比單筆少。修法是讓 BatchGetSellerProductsForDelegation 回完整 detail，"+
			"由 productResource() 投影（不是在 Gateway 手寫第二份欄位清單）。", batch, single)
	}

	r, audit := newDelegatedScopeRouter(t)
	w := doDelegatedScopeRequest(r, http.MethodPost, "/products/batch-unpublish",
		`{"operationReason":"批次下架","items":[{"productId":"`+scopeTestProductID+`","expectedVersion":"12"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if got := changedFieldNames(audit.confirms[0]); !equalStringSets(got, []string{"status"}) {
		t.Errorf("PRODUCT_BATCH_UNPUBLISH 的 changed_fields = %v, want [status]\n"+
			"before_snapshot 補齊不得連帶放寬比較範圍。", got)
	}
}

// TestBatchProductResourceFallsBackWhenDetailMissing 守住滾動部署窗口。
//
// Gateway 先於 product-service 上線時，BatchProductSnapshot.detail 會是 nil。此時照樣
// 投影會產出一整組空字串，稽核列記下「名稱是空的、產地是空的」這種從未成立的事實 ——
// 比只記 status 更糟。正確行為是退回舊形狀。
func TestBatchProductResourceFallsBackWhenDetailMissing(t *testing.T) {
	h := &Holder{}
	res := batchProductResource(h, &productpb.BatchProductSnapshot{
		ProductId: scopeTestProductID, Found: true, Version: "12",
		Status: strconv.Itoa(4004), Name: "無籽檸檬",
	})
	if !equalStringSets(mapKeys(res.Fields), []string{"status"}) {
		t.Errorf("detail 缺席時的投影 = %v, want 只有 [status]；"+
			"照樣跑完整投影會把每一欄都記成空值", mapKeys(res.Fields))
	}
	if res.Name != "無籽檸檬" || res.Version != "12" || res.ID != scopeTestProductID {
		t.Errorf("退回形狀遺失了 id/name/version：%+v", res)
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
