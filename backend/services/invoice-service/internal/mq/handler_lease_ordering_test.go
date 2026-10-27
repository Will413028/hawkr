//go:build !integration

package mq

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestHandlerLeaseOnlyHasToCoverWorkBeforeMarkProcessed 守住讓 handlerLease=60s
// 「夠用」的那個**排序**不變式。
//
// 背景：handlerLease 的註解把下界推導成「單筆最壞耗時 ≈ 6.6s（ORD 3s ＋ PAY 3s ＋
// 本地 ~0.6s）」。那個推導只涵蓋 prepareFromPayload，**不涵蓋 reclaimDeferred**——
// 而 reclaimDeferred 的成本是 O(N)（ListDeferredInboxEventsByAggregateRef **沒有
// LIMIT**），逐列還各打一次 ApplyRefund ＋ MarkProcessed。
//
// 那份推導之所以仍然成立，唯一的理由是**排序**：Handle 先 markInboxProcessed
// （該列離開 idx_invinbox_due 的候選集合）才 reclaimDeferred。於是那段無上界的工作
// 發生在租約**不再重要**之後。
//
// 🔴 把這兩行對調（或把 markInboxProcessed 移到 reclaim 之後），下界推導立刻失效：
// 一個有 N 筆 DEFERRED 退款的訂單會讓 handler 在候選集合內停留 O(N)，
// INV.dl 可以合法搶走一列正在被處理的事件，兩條路徑同時跑 §7.1.2 步驟 5–9。
// 那個改動不會有任何編譯錯、不會有任何既有測試轉紅——所以要有這一支。
//
// ⚠️ 對照：worker 那條路徑（ReprocessFromInbox）**沒有**這個保護，reclaimDeferred
// 就在 pkg/outbox.Worker 的 MarkDone 之前、租約仍然有效的窗口內。也就是說
// dlClaimLease(5min) 必須涵蓋 O(N) 的 reclaim，而 handlerLease(60s) 不必——
// 兩個租約的推導是同一句話抄兩次，但它們涵蓋的工作量並不相同。
// 今天 5 分鐘對現實的 N（一張訂單的退款筆數）綽綽有餘，這裡只記錄該差異。
func TestHandlerLeaseOnlyHasToCoverWorkBeforeMarkProcessed(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "order_paid_consumer.go", nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	body := methodBody(f, "OrderPaidConsumer", "Handle")
	if body == nil {
		t.Fatal("找不到 OrderPaidConsumer.Handle —— 本測試的錨已經漂掉，先修錨不要刪測試")
	}

	markAt, reclaimAt := -1, -1
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "markInboxProcessed":
			if markAt < 0 {
				markAt = int(sel.Pos())
			}
		case "reclaimDeferred":
			if reclaimAt < 0 {
				reclaimAt = int(sel.Pos())
			}
		}
		return true
	})

	if markAt < 0 || reclaimAt < 0 {
		t.Fatalf("Handle 內找不到 markInboxProcessed(%d) 或 reclaimDeferred(%d) 的呼叫 —— "+
			"本測試已失去鑑別力，不要因為它是綠的就相信它", markAt, reclaimAt)
	}
	if markAt > reclaimAt {
		t.Fatalf("Handle 先 reclaimDeferred 才 markInboxProcessed：那段 O(N) 的反向認領"+
			"（ListDeferredByAggregateRef 無 LIMIT）會發生在該列**仍在兜底候選集合內**的時候，"+
			"handlerLease=%v 的下界推導（只涵蓋 ~6.6s 的 prepareFromPayload）當場失效", handlerLease)
	}
}

// methodBody 找 (recv).name 的函式本體。
func methodBody(f *ast.File, recv, name string) *ast.BlockStmt {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		if id, ok := star.X.(*ast.Ident); ok && id.Name == recv {
			return fn.Body
		}
	}
	return nil
}
