package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"
)

// funcBody 回傳 main.go 裡名為 name 的函式本體。
func funcBody(t *testing.T, name string) *ast.BlockStmt {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name && fd.Body != nil {
			return fd.Body
		}
	}
	t.Fatalf("main.go 沒有 %s", name)
	return nil
}

// runsIMThrough 回報函式本體是否把 imModule.Run 交給 serveWithRun（直接，或經 joinRuns）。
func runsIMThrough(body *ast.BlockStmt) (servesWithRun, referencesRun bool) {
	return runsModuleThrough(body, "imModule")
}

// runsModuleThrough 回報函式本體是否把 <module>.Run 交給 serveWithRun（直接，或在 joinRuns 的參數裡），以及是否引用了它。
func runsModuleThrough(body *ast.BlockStmt, module string) (servesWithRun, referencesRun bool) {
	isRun := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == module
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if isRun(x) {
				referencesRun = true
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "serveWithRun" && len(x.Args) >= 2 {
				if isRun(x.Args[1]) {
					servesWithRun = true
				}
				if call, ok := x.Args[1].(*ast.CallExpr); ok {
					if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "joinRuns" && slices.ContainsFunc(call.Args, isRun) {
						servesWithRun = true
					}
				}
			}
		}
		return true
	})
	return servesWithRun, referencesRun
}

// notification 的串流 hub 在 serve 與 server 同一個生命週期（拿掉就沒有 LISTEN 連線，worker 發的通知永遠到不了訂閱者）；worker 沒有串流 handler，不跑 Run。
func TestServeRunsTheNotificationHubAndWorkerDoesNot(t *testing.T) {
	if served, _ := runsModuleThrough(funcBody(t, "runServe"), "notificationModule"); !served {
		t.Error("runServe 沒有以 serveWithRun 跑 notificationModule.Run")
	}
	if served, referenced := runsModuleThrough(funcBody(t, "runWorker"), "notificationModule"); served || referenced {
		t.Error("runWorker 不得跑 notificationModule.Run：worker 沒有串流 handler")
	}
}

func TestJoinRunsCancelsTheRestWhenOneFails(t *testing.T) {
	boom := errors.New("listen 失敗")
	stopped := make(chan struct{})
	waiting := func(ctx context.Context) error { <-ctx.Done(); close(stopped); return ctx.Err() }
	err := joinRuns(waiting, func(context.Context) error { return boom })(t.Context())
	if !errors.Is(err, boom) {
		t.Fatalf("失敗要回傳：%v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("另一個元件沒有被取消")
	}
}

func TestJoinRunsReturnsNilWhenCtxEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	waitingA := func(ctx context.Context) error { <-ctx.Done(); return nil }
	waitingB := func(ctx context.Context) error { <-ctx.Done(); return nil }
	done := make(chan error, 1)
	go func() { done <- joinRuns(waitingA, waitingB)(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("ctx 結束應回 nil：%v", err)
	}
}

// serve 的 im.Module.Run 與 server 同一個生命週期（拿掉就沒有 LISTEN 連線，串流的事件永遠到不了）；worker 沒有串流 handler，不跑 Run。
func TestServeRunsTheIMLiveChannelAndWorkerDoesNot(t *testing.T) {
	if served, _ := runsIMThrough(funcBody(t, "runServe")); !served {
		t.Error("runServe 沒有以 serveWithRun 跑 imModule.Run")
	}
	if served, referenced := runsIMThrough(funcBody(t, "runWorker")); served || referenced {
		t.Error("runWorker 不得跑 imModule.Run：worker 沒有串流 handler")
	}
}

func listenOnce(t *testing.T) listening {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listening{ln, newServer(http.NewServeMux())}
}

func TestServeWithRunStopsTheServersAndFailsWhenRunFails(t *testing.T) {
	boom := errors.New("listen 連線失敗")
	err := serveWithRun(t.Context(), func(context.Context) error { return boom }, listenOnce(t))
	if !errors.Is(err, boom) {
		t.Fatalf("Run 的錯誤要讓 serve 失敗：%v", err)
	}
}

func TestServeWithRunTreatsAnEarlyNilReturnAsFailure(t *testing.T) {
	if err := serveWithRun(t.Context(), func(context.Context) error { return nil }, listenOnce(t)); err == nil {
		t.Fatal("Run 在 ctx 結束前返回要算失敗")
	}
}

func TestServeWithRunEndsRunWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	ended := make(chan struct{})
	run := func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(ended)
		return ctx.Err()
	}
	done := make(chan error, 1)
	go func() { done <- serveWithRun(ctx, run, listenOnce(t)) }()
	<-started
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("ctx 取消的正常結束不該有錯誤：%v", err)
	}
	<-ended
}

// failingListener 在 fail 關閉後讓 Accept 回錯誤，模擬 server 自己失敗。
type failingListener struct {
	net.Listener
	fail chan struct{}
}

func (l failingListener) Accept() (net.Conn, error) {
	<-l.fail
	return nil, errors.New("accept 失敗")
}

// server 自己失敗（不是 ctx 結束）時，要先取消 run 的 ctx 再 Shutdown：另一個 server 上有一個只等 run 的 ctx 結束才返回的進行中請求，
// 順序正確時 Shutdown 立刻收尾；順序顛倒時 Shutdown 等不到請求結束，serve 在安全上限內回不來。
func TestServeWithRunCancelsRunBeforeShutdownWhenAServerFails(t *testing.T) {
	base := listenOnce(t)
	failing := listening{failingListener{base.ln, make(chan struct{})}, base.srv}
	fail := failing.ln.(failingListener).fail

	var runCtx context.Context
	started := make(chan struct{})
	run := func(ctx context.Context) error {
		runCtx = ctx
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	inFlight := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		close(inFlight)
		select {
		case <-runCtx.Done():
		case <-t.Context().Done():
		}
	})
	healthy := listenOnce(t)
	healthy.srv.Handler = mux

	done := make(chan error, 1)
	go func() { done <- serveWithRun(t.Context(), run, failing, healthy) }()
	<-started
	go func() {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+healthy.ln.Addr().String()+"/", nil)
		if err != nil {
			return
		}
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-inFlight:
	case <-time.After(5 * time.Second):
		t.Fatal("進行中的請求沒有抵達 handler")
	}
	close(fail)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("server 失敗要讓 serve 回錯誤")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve 沒有在安全上限內返回：Shutdown 開始前 run 的 ctx 沒被取消，進行中的請求等不到結束")
	}
}
