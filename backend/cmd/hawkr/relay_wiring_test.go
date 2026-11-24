package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// callsIn 回傳 file 裡函式 fn 本體內實際呼叫的名稱（`Name(...)` 與 `x.Name(...)`），已排序去重；只被參照而沒呼叫的不算。
func callsIn(t *testing.T, file, fn string) []string {
	t.Helper()
	return namesIn(t, file, fn, true)
}

// refsIn 另外包含 `x.Name` 形式的參照（當成值傳出的方法，例如 `events.Subscribe(bus, m.OnX)`）。
func refsIn(t *testing.T, file, fn string) []string {
	t.Helper()
	return namesIn(t, file, fn, false)
}

func namesIn(t *testing.T, file, fn string, callsOnly bool) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn || fd.Body == nil {
			return true
		}
		found = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				switch fun := x.Fun.(type) {
				case *ast.Ident:
					got = append(got, fun.Name)
				case *ast.SelectorExpr:
					if id, ok := fun.X.(*ast.Ident); ok {
						got = append(got, id.Name+"."+fun.Sel.Name)
					}
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && !callsOnly {
					got = append(got, id.Name+"."+x.Sel.Name)
				}
			}
			return true
		})
		return false
	})
	if !found {
		t.Fatalf("%s 沒有 %s", file, fn)
	}
	slices.Sort(got)
	return slices.Compact(got)
}

func requireCalls(t *testing.T, file, fn string, want ...string) {
	t.Helper()
	requireNames(t, callsIn(t, file, fn), file, fn, want)
}

func requireRefs(t *testing.T, file, fn string, want ...string) {
	t.Helper()
	requireNames(t, refsIn(t, file, fn), file, fn, want)
}

func requireNames(t *testing.T, got []string, file, fn string, want []string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s 的 %s 沒有呼叫 %s（有：%v）", file, fn, w, got)
		}
	}
}

// 並行期單一生效者（遷移計畫 A6 不變量）：舊 review 的 REST 路由刪除後，單體是 review.created、review.hidden 唯一的 producer，
// relay 的訂閱在 serve 與 worker 共用的 newReview 內接上。
func TestReviewRelayIsSubscribedByNewReview(t *testing.T) {
	requireCalls(t, "review.go", "newReview", "subscribeReviewRelay")
	requireRefs(t, "review.go", "subscribeReviewRelay", "reviewrelay.OnReviewCreated", "reviewrelay.OnReviewAggregateChanged")
}

func TestCRMModuleSubscribesTheLifecycleEvents(t *testing.T) {
	requireRefs(t, "crm.go", "newCRM", "m.OnBuyerDeleted", "m.OnStoreRetired")
}

// 兩個角色都建立三個模組（serve 的刪帳與退場、worker 的 CompletePending 都會發事件，訂閱要在兩邊）。
func TestBothRolesBuildReviewIMAndCRM(t *testing.T) {
	for _, role := range []string{"runServe", "runWorker"} {
		requireCalls(t, "main.go", role, "newReview", "newIM", "newCRM")
	}
}

// worker 登記 relay 與 consumer 的 job worker，並跑 crm 的 RabbitMQ consumer；serve 不跑它們（job 與 consumer 只在 worker）。
func TestWorkerRunsTheRelaysAndTheCRMConsumer(t *testing.T) {
	requireCalls(t, "main.go", "runWorker",
		"reviewrelay.NewWorker", "crmModule.AddWorkers", "crmConsumer.AddWorkers", "crmConsumer.Run")
	for _, name := range []string{"reviewrelay.NewWorker", "crmconsumer.Consumer", "crmConsumer.Run"} {
		if slices.Contains(callsIn(t, "main.go", "runServe"), name) {
			t.Errorf("runServe 不該呼叫 %s", name)
		}
	}
}
