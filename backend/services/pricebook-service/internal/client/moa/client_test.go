package moa

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestToROCDate(t *testing.T) {
	got := ToROCDate(time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC))
	if got != "115.07.24" {
		t.Errorf("ToROCDate = %q，預期 115.07.24", got)
	}
}

func TestParseROCDate(t *testing.T) {
	got, err := ParseROCDate("115.07.24")
	if err != nil {
		t.Fatalf("ParseROCDate 失敗: %v", err)
	}
	want := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseROCDate = %v，預期 %v", got, want)
	}
}

func TestParseROCDateRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "2026-07-24", "115/07/24", "abc.de.fg"} {
		if _, err := ParseROCDate(s); err == nil {
			t.Errorf("ParseROCDate(%q) 應回錯誤", s)
		}
	}
}

func TestParseROCDateRejectsNonexistentCalendarDate(t *testing.T) {
	cases := []string{
		"115.02.30",
		"115.04.31",
		"111.02.30",
	}
	for _, s := range cases {
		got, err := ParseROCDate(s)
		if err == nil {
			t.Errorf("ParseROCDate(%q) 應回錯誤（行事曆上不存在），卻靜默正規化成 %v", s, got)
		}
	}
}

func TestParseROCDateRejectsNonPositiveYear(t *testing.T) {
	cases := []string{"0.07.24", "-1.07.24"}
	for _, s := range cases {
		got, err := ParseROCDate(s)
		if err == nil {
			t.Errorf("ParseROCDate(%q) 應回錯誤（民國年非正整數），卻靜默解析成 %v", s, got)
		}
	}
}

func TestParseROCDateAcceptsLeapDay(t *testing.T) {
	got, err := ParseROCDate("113.02.29")
	if err != nil {
		t.Fatalf("ParseROCDate(\"113.02.29\") 不應報錯: %v", err)
	}
	want := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseROCDate(\"113.02.29\") = %v，預期 %v", got, want)
	}
}

func newTestServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchRangeDetectsTruncation(t *testing.T) {
	rows := make([]TradeRecord, maxRowsPerRequest)
	body, _ := json.Marshal(map[string]any{"RS": "OK", "Data": rows})
	srv := newTestServer(t, http.StatusOK, body)

	c := NewClient(srv.URL, srv.URL)
	_, err := c.FetchRange(context.Background(), time.Now(), time.Now(), "LA1", "")
	if !errors.Is(err, ErrResponseTruncated) {
		t.Errorf("預期 ErrResponseTruncated，得到 %v", err)
	}
}

func TestFetchRangeCapsResponseBodySize(t *testing.T) {
	huge := strings.Repeat("x", maxResponseBodyBytes+1024)
	body, _ := json.Marshal(map[string]any{"RS": "OK", "Data": []TradeRecord{{CropName: huge}}})
	srv := newTestServer(t, http.StatusOK, body)

	c := NewClient(srv.URL, srv.URL)
	_, err := c.FetchRange(context.Background(), time.Now(), time.Now(), "", "")
	if err == nil {
		t.Fatal("超過 body size 上限的回應應該回錯誤，不能被無界解析成功")
	}
}

func TestFetchRangeReturnsRecords(t *testing.T) {
	want := []TradeRecord{
		{TransDate: "115.07.24", CropCode: "LA1", CropName: "甘藍-初秋", MarketCode: "104", MarketName: "台北二", AvgPrice: 46.2},
		{TransDate: "115.07.24", CropCode: "LA1", CropName: "甘藍-初秋", MarketCode: "108", MarketName: "台中", AvgPrice: 38.5},
	}
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		body, _ := json.Marshal(map[string]any{"RS": "OK", "Data": want})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, srv.URL)
	start := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	got, err := c.FetchRange(context.Background(), start, end, "LA1", "104")
	if err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("FetchRange 回傳 %d 筆，預期 %d 筆", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FetchRange[%d] = %+v，預期 %+v", i, got[i], want[i])
		}
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("解析請求 query 失敗: %v", err)
	}
	if q.Get("Start_time") != "115.07.24" || q.Get("End_time") != "115.07.24" {
		t.Errorf("請求 query 未帶正確民國年日期: %v", q)
	}
	if q.Get("CropCode") != "LA1" {
		t.Errorf("請求 query 未帶 CropCode: %v", q)
	}
	if q.Get("MarketCode") != "104" {
		t.Errorf("請求 query 未帶 MarketCode: %v", q)
	}
}

func TestFetchRangeOmitsCropCodeAndMarketCodeWhenEmpty(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		body, _ := json.Marshal(map[string]any{"RS": "OK", "Data": []TradeRecord{}})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, srv.URL)
	if _, err := c.FetchRange(context.Background(), time.Now(), time.Now(), "", ""); err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("解析請求 query 失敗: %v", err)
	}
	if _, ok := q["CropCode"]; ok {
		t.Errorf("cropCode 為空時不該帶 CropCode 參數，實際 query: %v", q)
	}
	if _, ok := q["MarketCode"]; ok {
		t.Errorf("marketCode 為空時不該帶 MarketCode 參數，實際 query: %v", q)
	}
}

func TestFetchRangeIncludesMarketCodeIndependentlyOfCropCode(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		body, _ := json.Marshal(map[string]any{"RS": "OK", "Data": []TradeRecord{}})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, srv.URL)
	if _, err := c.FetchRange(context.Background(), time.Now(), time.Now(), "", "104"); err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("解析請求 query 失敗: %v", err)
	}
	if q.Get("MarketCode") != "104" {
		t.Errorf("請求 query 未帶 MarketCode: %v", q)
	}
	if _, ok := q["CropCode"]; ok {
		t.Errorf("cropCode 為空時不該帶 CropCode 參數，實際 query: %v", q)
	}
}

func TestFetchRangeErrorsOnNonJSONBody(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, []byte("<html><body>系統維護中，請稍後再試</body></html>"))

	c := NewClient(srv.URL, srv.URL)
	got, err := c.FetchRange(context.Background(), time.Now(), time.Now(), "LA1", "")
	if err == nil {
		t.Fatalf("預期解析失敗回錯誤，卻拿到 %d 筆資料且 error 為 nil", len(got))
	}
	if errors.Is(err, ErrResponseTruncated) {
		t.Error("非 JSON body 不應被誤判成截斷")
	}
}

func TestFetchRangeErrorsOnNonOKStatus(t *testing.T) {
	srv := newTestServer(t, http.StatusInternalServerError, []byte(`{"RS":"Error"}`))

	c := NewClient(srv.URL, srv.URL)
	if _, err := c.FetchRange(context.Background(), time.Now(), time.Now(), "LA1", ""); err == nil {
		t.Error("預期非 200 狀態碼回錯誤")
	}
}
