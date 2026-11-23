package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
)

type fakeRatings struct {
	calls int
	out   map[int64]review.StoreRating
	err   error
}

func (f *fakeRatings) StoreRatingStats(_ context.Context, ids []int64) (map[int64]review.StoreRating, error) {
	f.calls++
	return f.out, f.err
}

func TestRatingStatsConvertsToTheConsumersOwnType(t *testing.T) {
	src := &fakeRatings{out: map[int64]review.StoreRating{7: {Count: 3, Average: 4.5}}}
	got, err := ratingStats[discovery.RatingStats](t.Context(), src, []int64{7, 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[7] != (discovery.RatingStats{Count: 3, Average: 4.5}) {
		t.Fatalf("轉換結果 %v", got)
	}
	seller, err := ratingStats[httpseller.RatingStats](t.Context(), src, []int64{7})
	if err != nil || seller[7] != (httpseller.RatingStats{Count: 3, Average: 4.5}) {
		t.Fatalf("店面頁的轉換 %v %v", seller, err)
	}
}

// review 不接受空清單（InvalidInput），舊 client 回空 map；呼叫端（空頁的店家卡片）依賴後者，所以空清單不呼叫來源。
func TestRatingStatsSkipsTheSourceForAnEmptyList(t *testing.T) {
	src := &fakeRatings{err: errors.New("不該被呼叫")}
	got, err := ratingStats[discovery.RatingStats](t.Context(), src, nil)
	if err != nil || len(got) != 0 || src.calls != 0 {
		t.Fatalf("got=%v err=%v calls=%d", got, err, src.calls)
	}
}

func TestRatingStatsPropagatesSourceErrors(t *testing.T) {
	boom := errors.New("讀取失敗")
	if _, err := ratingStats[discovery.RatingStats](t.Context(), &fakeRatings{err: boom}, []int64{1}); !errors.Is(err, boom) {
		t.Fatalf("錯誤要原樣回傳，不推定為沒有評價：%v", err)
	}
}

// 評分統計的來源是 review 模組（A6-D11）：三個 port 與 newDiscovery 在 serve 與 worker 都收到 reviewModule，不是 nil 或別的值。
func TestRatingPortsAreBackedByTheReviewModule(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	isReviewModule := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "reviewModule"
	}
	seen := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			if id, ok := x.Type.(*ast.Ident); ok && (id.Name == "sellerRatings" || id.Name == "legacySellerRatings") {
				if len(x.Elts) != 1 || !isReviewModule(x.Elts[0]) {
					t.Errorf("%s 的來源必須是 reviewModule", id.Name)
				}
				seen[id.Name]++
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "newDiscovery" {
				if len(x.Args) != 6 || !isReviewModule(x.Args[3]) {
					t.Errorf("newDiscovery 的第四個參數必須是 reviewModule")
				}
				seen["newDiscovery"]++
			}
		}
		return true
	})
	if seen["sellerRatings"] != 1 || seen["legacySellerRatings"] != 1 || seen["newDiscovery"] != 2 {
		t.Fatalf("接線點數量不對：%v", seen)
	}
}

// 買家會員卡的 VIP 狀態來自 crm 模組（A6-D18）：httpbuyer.Ports.CRM 收到 crmModule。
func TestBuyerPortsVIPComesFromTheCRMModule(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "CRM" {
			if v, ok := kv.Value.(*ast.Ident); ok && v.Name == "crmModule" {
				found++
			}
		}
		return true
	})
	// httpapi.Deps.CRM（對外入口）、httpbuyer.Ports.CRM 與 worker 的 crmconsumer.Consumer.CRM。
	if found != 3 {
		t.Fatalf("main.go 的 CRM 欄位收到 crmModule 的次數是 %d，預期 3（Deps.CRM、Ports.CRM、crmconsumer.Consumer.CRM）", found)
	}
}
