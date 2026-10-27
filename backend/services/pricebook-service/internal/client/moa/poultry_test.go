package moa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParsePoultryPrice_SentinelForms(t *testing.T) {
	cases := []struct {
		raw  any
		want bool
	}{
		{"34.8", true},
		{" 34.8 ", true},
		{"休市", false},
		{"-", false},
		{"", false},
		{nil, false},
		{34.8, false},
		{"0", false},
		{"abc", false},
	}
	for _, c := range cases {
		_, ok := ParsePoultryPrice(c.raw)
		if ok != c.want {
			t.Errorf("ParsePoultryPrice(%#v) ok = %v，預期 %v", c.raw, ok, c.want)
		}
	}
}

func TestPoultryChickenEggSpecs_EggFieldNamesAreNotSwapped(t *testing.T) {
	byField := map[string]PoultrySeriesSpec{}
	for _, s := range PoultryChickenEggSpecs {
		byField[s.Field] = s
	}
	if got := byField["egg_Price"].PriceLayer; got != "wholesale_bulk" {
		t.Errorf("egg_Price 層級 = %q，預期 wholesale_bulk（它是大運輸價不是產地價）", got)
	}
	if got := byField["egg_Producer_Price"].PriceLayer; got != "farmgate" {
		t.Errorf("egg_Producer_Price 層級 = %q，預期 farmgate", got)
	}

	if got := byField["Store_KP_TaijinPrice"].PriceLayer; got != "retail" {
		t.Errorf("Store_KP_TaijinPrice 層級 = %q，預期 retail", got)
	}
}

func poultryServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchPoultry_SkipsSentinelFieldsKeepsRest(t *testing.T) {
	srv := poultryServer(t, `{"Data":[{"TransDate":"2026/09/10","TaijinPrice_2.0kgup":"34.8","TaijinPrice_1.75kg_1.95kg":"休市","Store_KP_TaijinPrice":"-","egg_Producer_Price":"39.5","egg_Price":null}]}`)
	defer srv.Close()

	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	got, err := NewClient(srv.URL, srv.URL).FetchPoultry(context.Background(), PoultryChickenEgg, start, start)
	if err != nil {
		t.Fatalf("FetchPoultry 失敗: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("列數 = %d，預期 1", len(got))
	}
	prices := got[0].Prices
	if len(prices) != 2 {
		t.Fatalf("有報價的欄位 = %v，預期只有 2 個（其餘三種缺值形式都要跳過）", prices)
	}
	if prices["TaijinPrice_2.0kgup"] != 34.8 || prices["egg_Producer_Price"] != 39.5 {
		t.Errorf("解析值不符: %v", prices)
	}
	for _, absent := range []string{"TaijinPrice_1.75kg_1.95kg", "Store_KP_TaijinPrice", "egg_Price"} {
		if _, ok := prices[absent]; ok {
			t.Errorf("%q 是缺值，不該出現在 Prices 裡（更不該是 0）", absent)
		}
	}
}

func TestFetchPoultry_RejectsDatesOutsideRequestedRange(t *testing.T) {
	srv := poultryServer(t, `{"Data":[{"TransDate":"2026/09/10","egg_Producer_Price":"39.5"},{"TransDate":"2020/01/01","egg_Producer_Price":"20"}]}`)
	defer srv.Close()

	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	_, err := NewClient(srv.URL, srv.URL).FetchPoultry(context.Background(), PoultryChickenEgg, start, start)
	if err == nil {
		t.Fatal("回傳日期超出請求區間時必須報錯")
	}
	if !strings.Contains(err.Error(), "2020/01/01") {
		t.Errorf("錯誤訊息應指出違規的那一列，got %v", err)
	}
}
