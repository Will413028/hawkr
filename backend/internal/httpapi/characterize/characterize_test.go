package characterize

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
)

// Probe 要記下 Errorf，也要在 Fatalf 時停下情境：突變測試靠它判斷情境抓到了錯的資料。
func TestProbeRecordsFailuresAndStopsAtFatal(t *testing.T) {
	reached := false
	s := Scenario[int]{Name: "x", Run: func(_ context.Context, t T, _ int) {
		t.Errorf("第 %d 個", 1)
		t.Fatalf("第 %d 個", 2)
		reached = true
	}}
	if got := Probe(0, s); len(got) != 2 || got[0] != "第 1 個" || got[1] != "第 2 個" {
		t.Fatalf("failures = %q", got)
	}
	if reached {
		t.Fatal("Fatalf 之後情境應該停下")
	}
	if got := Probe(0, Scenario[int]{Name: "ok", Run: func(context.Context, T, int) {}}); len(got) != 0 {
		t.Fatalf("通過的情境記下了 %q", got)
	}
}

func TestErrorsMapToTheSameKinds(t *testing.T) {
	cases := []struct {
		connect connect.Code
		status  int
		kind    error
	}{
		{connect.CodeUnauthenticated, http.StatusUnauthorized, ErrUnauthenticated},
		{connect.CodePermissionDenied, http.StatusForbidden, ErrPermission},
		{connect.CodeNotFound, http.StatusNotFound, ErrNotFound},
		{connect.CodeInvalidArgument, http.StatusBadRequest, ErrInvalid},
		{connect.CodeAborted, http.StatusConflict, ErrConflict},
	}
	for _, c := range cases {
		if err := ConnectKind(connect.NewError(c.connect, errors.New("x"))); !errors.Is(err, c.kind) {
			t.Errorf("Connect %v → %v，預期 %v", c.connect, err, c.kind)
		}
		if err := RESTKind(c.status, 0); !errors.Is(err, c.kind) {
			t.Errorf("HTTP %d → %v，預期 %v", c.status, err, c.kind)
		}
	}
	// envelope 的 code 優先於 HTTP status（gateway 的業務錯誤常回 200 帶非 0 的 code）。
	if err := RESTKind(http.StatusOK, http.StatusNotFound); !errors.Is(err, ErrNotFound) {
		t.Errorf("code 404 → %v", err)
	}
	if err := ConnectKind(connect.NewError(connect.CodeInternal, errors.New("x"))); errors.Is(err, ErrInvalid) {
		t.Errorf("internal 不該被分類成輸入錯誤")
	}
}

func TestBearerCarriesTheCallersToken(t *testing.T) {
	h := http.Header{}
	Bearer(map[Caller]string{Admin: "tok"})(Admin, h)
	if h.Get("Authorization") != "Bearer tok" {
		t.Fatalf("header = %v", h)
	}
	h = http.Header{}
	Bearer(map[Caller]string{Admin: "tok"})(Anonymous, h)
	if h.Get("Authorization") != "" {
		t.Fatal("訪客不帶 token")
	}
}
