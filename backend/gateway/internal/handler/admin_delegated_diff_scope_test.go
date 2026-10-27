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
	"time"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	productpb "github.com/Will413028/hawkr/go-pkg/proto/product"
	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// 稽核列的 changed_fields 是爭議時的舉證。它記到一筆從未發生的變更，比少記一筆更糟：
// 少記只是舉證不足，多記是**證據本身在說謊**——賣家看到「你把我的店名清空了」，而店名
// 一個字都沒動。
//
// dev 實測（2026-08-16）：
//   - 代行換一次店家 logo ⇒ changed_fields 記了 10 列，只有 logoKey 是真的，其餘 9 列
//     （店名／地址／電話／封面…）全被記成「被清空」
//   - 上／下架每次虛報一列「主題標籤被清空」，而 DB 的 product_hashtag 一列未動
//
// 成因是同一個：diff 的前提是「before 與 after 是同一個投影函式對同一資源的兩次完整
// 取樣」，而狀態切換與媒體替換這類端點的寫入回應**本來就不是資源全貌**。它有兩種外殼，
// 而且只有第一種會被「鍵集合是否相同」抓到：
//
//	① 鍵**缺席**（媒體端點手寫單鍵 after）⇒ 缺席側 render 成「—」
//	② 鍵**在、值是零值**（上游回的 DTO 沒回填衍生集合）⇒ 看起來就是被清空
//
// ⇒ 本檔的不變式不是「兩側鍵集合相同」（那抓不到 ②），而是**每一個 action 的
// changed_fields 必須恰好落在該 action 動得到的欄位集合內**。這個集合就是
// delegation.Spec.SnapshotFields，由端點自己宣告。
//
// ⚠️ 下面的 stub 刻意讓寫入回應**殘缺**（狀態端點不回 hashtags、貼文狀態端點不回
// content）。那正是 dev 上發生過的形狀，也是本測試的鑑別力來源：把 SnapshotFields
// 拿掉，這幾支就會轉紅。Gateway 不得依賴領域服務回一份完整資源——十三支端點跨三個
// 服務，「大家都會記得回全欄」不是一個守得住的前提。

const (
	scopeTestStoreID   = "2685914682"
	scopeTestProductID = "7494590641851072512"
	scopeTestPostID    = "7494590641851072999"
)

// ==================== stubs ====================

// scopeAuditStub 收下三段序的四支 RPC 並保留送出的內容。稽核正確與否只看「Gateway 送
// 了什麼」，回應長什麼樣在這個 bug 上完全正常，那正是它無聲的原因。
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

// statusOnlyProductDetail 是**殘缺**的寫入回應：狀態變了，衍生集合（hashtags／
// certifications）沒回填。這正是 dev 上 UpdateSellerProductStatus 回過的形狀。
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
		// detail 是 before_snapshot 的素材，與單筆路徑的 GetProduct 回同一份形狀。
		// product-service 這一跳刻意不填 seller／reviews（投影用不到，且填 seller 要
		// 跨服務呼叫），所以這裡也不填 —— stub 要像真的那樣殘缺，不能比真的還完整。
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

type scopePostStub struct {
	socialpb.SocialServiceClient
}

func fullPostDetail(content, publishStatus, version, publishedAt, scheduledAt string) *socialpb.PostDetailInfo {
	return &socialpb.PostDetailInfo{
		Id:                 scopeTestPostID,
		Version:            version,
		Content:            content,
		ImageKeys:          []string{"post1.jpg"},
		Tags:               []string{"檸檬"},
		Products:           []*socialpb.PostProduct{{Id: scopeTestProductID}},
		PublishStatus:      publishStatus,
		PublishedAt:        publishedAt,
		ScheduledPublishAt: scheduledAt,
	}
}

// publishStateOnlyPostDetail 是**殘缺**的寫入回應：發布狀態變了，內容欄沒回填。
func publishStateOnlyPostDetail(publishStatus, version, publishedAt, scheduledAt string) *socialpb.PostDetailInfo {
	d := fullPostDetail("", publishStatus, version, publishedAt, scheduledAt)
	d.Tags = nil
	d.ImageKeys = nil
	d.Products = nil
	return d
}

func (s *scopePostStub) GetPostDetail(ctx context.Context, in *socialpb.GetPostDetailRequest, opts ...grpc.CallOption) (*socialpb.PostDetailResponse, error) {
	return &socialpb.PostDetailResponse{
		Data: fullPostDetail("今天的檸檬很甜", strconv.Itoa(4401), "3", "", ""),
	}, nil
}

func (s *scopePostStub) CreatePost(ctx context.Context, in *socialpb.CreatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	return &socialpb.PostResponse{Data: fullPostDetail(in.GetContent(), strconv.Itoa(4401), "1", "", "")}, nil
}

func (s *scopePostStub) UpdatePost(ctx context.Context, in *socialpb.UpdatePostRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	return &socialpb.PostResponse{Data: fullPostDetail(in.GetContent(), strconv.Itoa(4401), "4", "", "")}, nil
}

func (s *scopePostStub) UpdateSellerPostStatus(ctx context.Context, in *socialpb.UpdateSellerPostStatusRequest, opts ...grpc.CallOption) (*socialpb.PostResponse, error) {
	publishedAt := ""
	if in.GetTargetStatus() == strconv.Itoa(4402) && in.GetScheduledPublishAt() == "" {
		publishedAt = "2026-08-16T04:00:00Z"
	}
	return &socialpb.PostResponse{
		Data: publishStateOnlyPostDetail(in.GetTargetStatus(), "4", publishedAt, in.GetScheduledPublishAt()),
	}, nil
}

// ==================== router ====================

func newDelegatedScopeRouter(t *testing.T) (*gin.Engine, *scopeAuditStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	audit := &scopeAuditStub{}
	h := &Holder{
		Account: grpcclient.NewAccountDelegationClientForTest(audit),
		Product: grpcclient.NewProductClientForTest(&scopeProductStub{}),
		Social:  grpcclient.NewSocialClientForTest(&scopePostStub{}),
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
	g.POST(sellers+"/posts", h.CreateDelegatedPost)
	g.PATCH(sellers+"/posts/:post_id", h.UpdateDelegatedPost)
	g.POST(sellers+"/posts/:post_id/publish", h.PublishDelegatedPost)
	g.POST(sellers+"/posts/:post_id/unpublish", h.UnpublishDelegatedPost)
	return r, audit
}

// ==================== 逐 action 的比較範圍 ====================

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
		// want 是這一次操作**應該**留下的 changed_fields 欄位集合。
		want []string
	}{
		{
			// 本案回歸點②：上架只能記 status 一列（56 號 §6.1.11 白紙黑字）。
			// 修復前是兩列，多出來的那列是「主題標籤被清空」。
			action: "PRODUCT_PUBLISH",
			method: http.MethodPost, path: "/products/" + scopeTestProductID + "/publish",
			body: `{"expectedVersion":"12","operationReason":"協助上架"}`,
			want: []string{"status"},
		},
		{
			action: "PRODUCT_UNPUBLISH",
			method: http.MethodPost, path: "/products/" + scopeTestProductID + "/unpublish",
			body: `{"expectedVersion":"12","operationReason":"協助下架"}`,
			// before 已經是 4003（未上架），下架成同一個值 ⇒ 沒有任何欄位變更。
			// 「什麼都沒變就一列都不記」與「只記 status」是同一條規則的兩端。
			want: nil,
		},
		{
			action: "PRODUCT_BATCH_UNPUBLISH",
			method: http.MethodPost, path: "/products/batch-unpublish",
			body: `{"operationReason":"批次下架","items":[{"productId":"` + scopeTestProductID + `","expectedVersion":"12"}]}`,
			want: []string{"status"},
		},
		{
			// 本案反向鑑別力：正常 PATCH 路徑不得被一起收窄掉。改 name 就要記 name。
			action: "PRODUCT_UPDATE",
			method: http.MethodPatch, path: "/products/" + scopeTestProductID,
			body: `{"expectedVersion":"12","operationReason":"改名","name":"有機無籽檸檬"}`,
			want: []string{"name"},
		},
		{
			action: "POST_UPDATE",
			method: http.MethodPatch, path: "/posts/" + scopeTestPostID,
			body: `{"expectedVersion":"3","operationReason":"改內容","content":"今天的檸檬更甜"}`,
			want: []string{"content"},
		},
		{
			action: "POST_PUBLISH",
			method: http.MethodPost, path: "/posts/" + scopeTestPostID + "/publish",
			body: `{"expectedVersion":"3","operationReason":"協助發布","publishMode":"immediate"}`,
			want: []string{"publishStatus", "publishedAt"},
		},
		{
			action: "POST_UNPUBLISH",
			method: http.MethodPost, path: "/posts/" + scopeTestPostID + "/unpublish",
			body: `{"expectedVersion":"3","operationReason":"協助下架"}`,
			want: []string{"publishStatus"},
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
			// 規則表 12 欄扣掉 description：建立回應沒有商品介紹，兩側都是「—」。
			want: []string{
				"name", "categoryName", "origin", "status", "priceMin", "priceMax",
				"stockQty", "skus", "images", "hashtags", "certifications",
			},
			omitted: "description（未填，兩側皆為「—」）",
		},
		{
			action: "POST_CREATE", path: "/posts",
			body: `{"operationReason":"代客發文","content":"今天的檸檬很甜"}`,
			// 規則表 7 欄扣掉兩個時間欄：立即發布以外的貼文兩者皆空，兩側都是「—」。
			want:    []string{"content", "tags", "productIds", "imageKeys", "publishStatus"},
			omitted: "scheduledPublishAt／publishedAt（未排程未發布，兩側皆為「—」）",
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

// POST_SCHEDULE 與 POST_PUBLISH 共用同一個 Orchestrate 呼叫點，所以比較範圍相同；
// 但它寫的是 scheduledPublishAt 而不是 publishedAt —— 範圍少列一欄的話，一次真實的
// 排程設定就完全不會出現在稽核上。排程時間必須在執行當下計算（下界 now+5min）。
func TestDelegatedAudit_PostScheduleRecordsTheScheduledTime(t *testing.T) {
	r, audit := newDelegatedScopeRouter(t)
	scheduledAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second).Format("2006-01-02T15:04:05Z")
	w := doDelegatedScopeRequest(r, http.MethodPost, "/posts/"+scopeTestPostID+"/publish",
		`{"expectedVersion":"3","operationReason":"排程發布","publishMode":"scheduled",`+
			`"scheduledPublishAt":"`+scheduledAt+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if got, want := audit.intents[0].GetAction(), "POST_SCHEDULE"; got != want {
		t.Fatalf("action = %q, want %q —— 成功建立的排程是 POST_SCHEDULE 不是 POST_PUBLISH", got, want)
	}
	got := changedFieldNames(audit.confirms[0])
	if !equalStringSets(got, []string{"publishStatus", "scheduledPublishAt"}) {
		t.Errorf("POST_SCHEDULE 的 changed_fields = %v, want [publishStatus scheduledPublishAt]", got)
	}
}

// ==================== 機械閘門：新端點必須自己決定比較範圍 ====================

// SnapshotFields 漏填 = 選了全表 = 稽核可能虛報，而它不編譯失敗、不轉紅、回應完全
// 正常。上面那張表守的是「已知的 12 個 action」，這一支守的是**第 13 個之後**：任何
// 新的 Orchestrate／OrchestrateBatch 呼叫點都必須明寫 SnapshotFields（要全表就寫
// nil），把「想過這件事」留在版控裡。
//
// 以 AST 而不是 grep 為錨：註解與字串裡出現 SnapshotFields 不算數，只有真的填在
// composite literal 的 key 才算（§5「命中的是程式碼還是註解」）。
func TestDelegatedOrchestrateCallSites_AllDeclareSnapshotFields(t *testing.T) {
	// 掃整個 package 而不是 admin_delegated_*.go：第 14 支端點若落在不合那個命名的
	// 檔案（例如既有的 admin_delegation.go），glob 版會靜默漏掉它，而 checked 門檻
	// 只擋得住「全部錨點失效」，擋不住「新增一個沒被掃到的」。
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
	// 反向鑑別力：掃不到呼叫點時上面的迴圈一次都不會執行，整支測試會假綠。9 是
	// 當下的實際數量（product 4、post 4、order 1；商品上下架兩支各自共用一個呼叫點）。新增端點時這個數字要跟著往上調——那正是提醒
	// 你「有一支新的東西要決定比較範圍」的地方。
	if checked < 9 {
		t.Errorf("只掃到 %d 個 Orchestrate 呼叫點，少於已知的 9 個 —— AST 錨點可能失效了", checked)
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

// ==================== helpers ====================

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

// ==================== before_snapshot 的舉證素材 ====================

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

	// ① 先確認單筆這一側真的是完整白名單投影 —— 少了這一步，兩側一起退化成 {status}
	//    時本測試仍會通過。
	want := make([]string, 0, len(delegation.DelegationDiffRules["product"]))
	for f := range delegation.DelegationDiffRules["product"] {
		want = append(want, f)
	}
	if !equalStringSets(single, want) {
		t.Fatalf("PRODUCT_UNPUBLISH 的 before_snapshot 鍵集合 = %v, want 完整規則表 %v", single, want)
	}

	// ② 批次必須與單筆逐鍵相同。
	if !equalStringSets(batch, single) {
		t.Errorf("PRODUCT_BATCH_UNPUBLISH 的 before_snapshot 鍵集合 = %v，單筆是 %v\n"+
			"before_snapshot 是爭議時的舉證素材，批次正是「一次動很多件」的高風險操作，"+
			"不得比單筆少。修法是讓 BatchGetSellerProductsForDelegation 回完整 detail，"+
			"由 productResource() 投影（不是在 Gateway 手寫第二份欄位清單）。", batch, single)
	}

	// ③ 反向鑑別力：before 補齊了，changed_fields 仍必須只有 status。收窄的是 Diff 的
	//    比較範圍（SnapshotFields），不是 Project —— 兩者混為一談會讓這次修復把 §6.1.11
	//    「PRODUCT_BATCH_UNPUBLISH 只記 status」一起改壞。
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
		Status: strconv.Itoa(4004), Name: "無籽檸檬", // Detail 刻意缺席
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
