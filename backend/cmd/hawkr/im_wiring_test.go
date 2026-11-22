package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"testing"
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

// runsIMThrough 回報函式本體是否把 imModule.Run 交給 serveWithRun。
func runsIMThrough(body *ast.BlockStmt) (servesWithRun, referencesRun bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "imModule" && x.Sel.Name == "Run" {
				referencesRun = true
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "serveWithRun" && len(x.Args) >= 2 {
				if sel, ok := x.Args[1].(*ast.SelectorExpr); ok && sel.Sel.Name == "Run" {
					servesWithRun = true
				}
			}
		}
		return true
	})
	return servesWithRun, referencesRun
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

// server 自己失敗（不是 ctx 結束）時，Shutdown 開始前 run 的 ctx 已被取消。
func TestServeWithRunCancelsRunBeforeShutdownWhenAServerFails(t *testing.T) {
	failing := listenOnce(t)
	if err := failing.ln.Close(); err != nil {
		t.Fatal(err)
	}
	runCtx := make(chan context.Context, 1)
	started := make(chan struct{})
	run := func(ctx context.Context) error {
		runCtx <- ctx
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	cancelledAtShutdown := make(chan bool, 1)
	healthy := listenOnce(t)
	healthy.srv.RegisterOnShutdown(func() {
		select {
		case cancelledAtShutdown <- (<-runCtx).Err() != nil:
		default:
		}
	})
	done := make(chan error, 1)
	go func() { done <- serveWithRun(t.Context(), run, failing, healthy) }()
	if err := <-done; err == nil {
		t.Fatal("server 失敗要讓 serve 回錯誤")
	}
	if !<-cancelledAtShutdown {
		t.Fatal("Shutdown 開始時 run 的 ctx 還沒被取消")
	}
}
