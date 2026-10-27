package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func TestHealthAndReadiness(t *testing.T) {
	ready := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("db down") }
	cases := []struct {
		name   string
		ready  func(context.Context) error
		method string
		path   string
		want   int
	}{
		{"liveness 不看依賴", down, http.MethodGet, "/healthz", http.StatusNoContent},
		{"liveness 只接受 GET", ready, http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
		{"依賴可用時 ready", ready, http.MethodGet, "/readyz", http.StatusNoContent},
		{"依賴不可用時 not ready", down, http.MethodGet, "/readyz", http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h, err := assemble(Deps{Ready: c.ready}, nil)
			if err != nil {
				t.Fatal(err)
			}
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), c.method, c.path, nil))
			if rec.Code != c.want {
				t.Errorf("%s %s = %d，預期 %d", c.method, c.path, rec.Code, c.want)
			}
		})
	}
}

func TestNewHandlerRequiresDelegation(t *testing.T) {
	if _, err := NewHandler(Deps{Ready: func(context.Context) error { return nil }}); err == nil {
		t.Fatal("缺少 Delegation 時應回傳組裝錯誤")
	}
}

func TestNewHandlerRequiresSeller(t *testing.T) {
	if _, err := NewHandler(Deps{Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}}), Deletion: &accountdeletion.Module{}}); err == nil {
		t.Fatal("缺少 Seller 時應回傳組裝錯誤")
	}
}

func TestNewHandlerRequiresDeletion(t *testing.T) {
	if _, err := NewHandler(Deps{Delegation: delegation.New(delegation.Deps{Seller: &seller.Module{}})}); err == nil {
		t.Fatal("缺少 Deletion 時應回傳組裝錯誤")
	}
}
