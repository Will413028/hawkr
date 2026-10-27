package moa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const riceBody = `[
  {"name":"基隆市  ","pt_date_day":"115.9.13","pt_1japt":0.0,"pt_1tsait":0.0,"pt_1sangt":0.0,"pt_1glutrt":0.0,"pt_1glutlt":0.0,"pt_2japt":0.0,"pt_2tsait":0.0,"pt_2sangt":0.0,"pt_2glutrt":0.0,"pt_2glutlt":0.0,"pt_4japt":0.0,"pt_4tsait":0.0,"pt_4sangt":0.0,"pt_4glutrt":0.0,"pt_4glutlt":0.0},
  {"name":"基隆市  ","pt_date_day":"115.9.12","pt_1japt":51.33,"pt_1tsait":52.78,"pt_1sangt":0.0,"pt_1glutrt":62.0,"pt_1glutlt":61.67,"pt_2japt":4633.0,"pt_2tsait":4667.0,"pt_2sangt":0.0,"pt_2glutrt":5267.0,"pt_2glutlt":5333.0,"pt_4japt":0.0,"pt_4tsait":0.0,"pt_4sangt":0.0,"pt_4glutrt":0.0,"pt_4glutlt":0.0}
]`

func riceServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/RicepriceData.aspx" {
			t.Errorf("路徑 = %q，預期 /RicepriceData.aspx", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchRice_ParsesBareArrayAndDropsZeroSentinels(t *testing.T) {
	srv := riceServer(t, riceBody)
	defer srv.Close()

	rows, err := NewClient(srv.URL, srv.URL).FetchRice(context.Background(), 40, 0)
	if err != nil {
		t.Fatalf("FetchRice: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("列數 = %d，預期 2", len(rows))
	}

	if rows[0].CityName != "基隆市" {
		t.Errorf("CityName = %q，預期 %q（上游帶兩個尾隨空白）", rows[0].CityName, "基隆市")
	}

	if got := rows[0].TradeDate.Format("2006-01-02"); got != "2026-09-13" {
		t.Errorf("TradeDate = %s，預期 2026-09-13", got)
	}

	if len(rows[0].Prices) != 0 {
		t.Errorf("整列全零的日子 Prices 應為空，得到 %v", rows[0].Prices)
	}

	p := rows[1].Prices
	if got, ok := p["pt_1japt"]; !ok || got != 51.33 {
		t.Errorf("pt_1japt = %v（ok=%v），預期 51.33", got, ok)
	}
	if got, ok := p["pt_2japt"]; !ok || got != 4633.0 {
		t.Errorf("pt_2japt = %v（ok=%v），預期 4633", got, ok)
	}
	if _, ok := p["pt_1sangt"]; ok {
		t.Error("pt_1sangt 該縣市無報價（0.0），不得出現在 Prices 中")
	}

	if _, ok := p["pt_4japt"]; ok {
		t.Error("pt_4japt 本波不接（layer 未驗證），不應被解析")
	}
}

func TestFetchRice_HTMLErrorPageIsHardFailure(t *testing.T) {
	srv := riceServer(t, "<!DOCTYPE html><html><head><title>error</title></head></html>")
	defer srv.Close()

	if _, err := NewClient(srv.URL, srv.URL).FetchRice(context.Background(), 40, 0); err == nil {
		t.Fatal("HTML 錯誤頁應回錯誤，不得靜默回零列")
	}
}

func TestFetchRice_RejectsOutOfRangePaging(t *testing.T) {
	srv := riceServer(t, riceBody)
	defer srv.Close()
	c := NewClient(srv.URL, srv.URL)

	for _, tc := range []struct {
		name      string
		top, skip int
	}{
		{"top 為零", 0, 0},
		{"top 超過上游上限", riceMaxRows + 1, 0},
		{"skip 為負", 40, -1},
	} {
		if _, err := c.FetchRice(context.Background(), tc.top, tc.skip); err == nil {
			t.Errorf("%s：應回錯誤", tc.name)
		}
	}
}

func TestFetchRice_UsesOpenDataBaseNotAPIv1(t *testing.T) {
	apiV1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("FetchRice 不應打 api/v1 base")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiV1.Close()
	openData := riceServer(t, riceBody)
	defer openData.Close()

	if _, err := NewClient(apiV1.URL, openData.URL).FetchRice(context.Background(), 40, 0); err != nil {
		t.Fatalf("FetchRice: %v", err)
	}
}

func TestParseRicePrice_ZeroIsMissingNotAPrice(t *testing.T) {
	for _, raw := range []any{0.0, -1.0, nil, "51.33", "休市"} {
		if _, ok := ParseRicePrice(raw); ok {
			t.Errorf("ParseRicePrice(%v) 應判為缺值", raw)
		}
	}
	if v, ok := ParseRicePrice(51.33); !ok || v != 51.33 {
		t.Errorf("ParseRicePrice(51.33) = %v, %v", v, ok)
	}
}

func TestRiceSpecs_Invariants(t *testing.T) {
	seen := map[string]bool{}
	withCategory := 0
	for _, s := range RiceSpecs {
		if seen[s.Field] {
			t.Errorf("欄名 %q 重複——commodity_code 會撞 unique key", s.Field)
		}
		seen[s.Field] = true

		switch s.PriceLayer {
		case "retail":
			if s.Unit != "kg" {
				t.Errorf("%s 零售應為元/公斤，得到 %q", s.Field, s.Unit)
			}
		case "wholesale_bulk":
			if s.Unit != "100kg" {
				t.Errorf("%s 躉售應為元/百公斤，得到 %q", s.Field, s.Unit)
			}
		default:
			t.Errorf("%s 的層級 %q 不在本波接入範圍", s.Field, s.PriceLayer)
		}
		withCategory += len(s.CategoryIDs)
	}
	if len(RiceSpecs) != 10 {
		t.Errorf("序列數 = %d，預期 10（零售 5 + 躉售 5，稻穀不接）", len(RiceSpecs))
	}

	if withCategory != 1 {
		t.Errorf("宣告分類的序列共 %d 條，預期恰好 1（稉種白米零售代表「米」2022）", withCategory)
	}
}
