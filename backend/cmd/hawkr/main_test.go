package main

import (
	"context"
	"errors"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}} {
		err := run(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "子命令") {
			t.Errorf("run(%q) = %v，預期子命令錯誤", args, err)
		}
	}
}

func TestEveryRoleRequiresADatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	for _, role := range []string{"serve", "worker", "migrate", "delegation-eligibility"} {
		args := []string{role}
		if role == "delegation-eligibility" {
			args = append(args, "disable", "-admin-id", "1", "-operator-admin-id", "2")
		}
		err := run(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
			t.Errorf("%s 缺少 DATABASE_URL 時回傳 %v，預期指出 DATABASE_URL", role, err)
		}
	}
}

func TestServeAnswersUntilContextEnds(t *testing.T) {
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := func(context.Context) error { return nil }
	h, err := httpapi.NewHandler(httpapi.Deps{Social: &social.Module{}, Ready: ready, Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}}), Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}, Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{}})
	if err != nil {
		t.Fatal(err)
	}
	go func() { done <- serve(ctx, ln, h) }()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("/healthz = %d，預期 204", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve 回傳 %v，預期 nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 結束後 serve 沒有停止")
	}
}

// noDashboardOrders 是這個測試不讀儀表板的訂單時的替身：NewHandler 要求提供儀表板訂單的來源。
type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

func TestServeAcceptsUnencryptedHTTP2(t *testing.T) {
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(r.Proto)) })
	go func() { done <- serve(ctx, ln, h) }()

	tr := &http.Transport{Protocols: new(http.Protocols)}
	tr.Protocols.SetUnencryptedHTTP2(true)
	defer tr.CloseIdleConnections()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.ProtoMajor != 2 || string(body) != "HTTP/2.0" {
		t.Fatalf("proto = %s，server 看到 %q，預期 HTTP/2", resp.Proto, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve 回傳 %v，預期 nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 結束後 serve 沒有停止")
	}
}
