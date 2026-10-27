package moa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestToROCDateCompact(t *testing.T) {
	got := ToROCDateCompact(time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC))
	if got != "1150912" {
		t.Errorf("ToROCDateCompact = %q，預期 1150912", got)
	}
}

func TestToROCDateCompact_IsNotDottedFormat(t *testing.T) {
	d := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	if ToROCDateCompact(d) == ToROCDate(d) {
		t.Fatal("緊湊格式不該等於 YYY.MM.DD")
	}
}

func TestParseROCDateCompact(t *testing.T) {
	got, err := ParseROCDateCompact("1150912")
	if err != nil {
		t.Fatalf("ParseROCDateCompact 失敗: %v", err)
	}
	want := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("= %v，預期 %v", got, want)
	}
}

func TestParseROCDateCompactRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "115.09.12", "2026-09-12", "11509123", "115091", "abcdefg", "0000912"} {
		if _, err := ParseROCDateCompact(s); err == nil {
			t.Errorf("ParseROCDateCompact(%q) 應回錯誤", s)
		}
	}
}

func TestParseROCDateCompactRejectsNonexistentCalendarDate(t *testing.T) {
	for _, s := range []string{"1150230", "1150431", "1110230"} {
		if got, err := ParseROCDateCompact(s); err == nil {
			t.Errorf("ParseROCDateCompact(%q) 應回錯誤，卻靜默正規化成 %v", s, got)
		}
	}
}

func TestPorkRecord_HasTrade_ZeroIsSentinelNotPrice(t *testing.T) {
	cases := []struct {
		name string
		rec  PorkRecord
		want bool
	}{
		{"正常成交", PorkRecord{SpecPigNum: 397, SpecPigAvg: 108.4}, true},
		{"無成交：頭數與價格皆 0", PorkRecord{SpecPigNum: 0, SpecPigAvg: 0}, false},
		{"頭數 0 但價格非 0（上游髒資料）", PorkRecord{SpecPigNum: 0, SpecPigAvg: 100}, false},
		{"有頭數但價格 0（上游髒資料）", PorkRecord{SpecPigNum: 10, SpecPigAvg: 0}, false},
	}
	for _, c := range cases {
		if got := c.rec.HasTrade(); got != c.want {
			t.Errorf("%s: HasTrade = %v，預期 %v", c.name, got, c.want)
		}
	}
}

func porkServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/PorkTransType/") {
			t.Errorf("非預期路徑: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchPork_ParsesSpecPigFields(t *testing.T) {
	srv := porkServer(t, `{"Data":[{"TransDate":"1150912","MarketName":"高雄岡山","SpecPig_Num":397,"SpecPig_AvgWgt":128.38,"SpecPig_AvgPrice":108.4}]}`)
	defer srv.Close()

	got, err := NewClient(srv.URL, srv.URL).FetchPork(context.Background(), time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchPork 失敗: %v", err)
	}
	if len(got) != 1 || got[0].MarketName != "高雄岡山" || got[0].SpecPigAvg != 108.4 {
		t.Fatalf("解析結果不符: %+v", got)
	}
}

func TestFetchPork_RejectsMismatchedDate(t *testing.T) {
	srv := porkServer(t, `{"Data":[{"TransDate":"1150901","MarketName":"雲林縣","SpecPig_Num":10,"SpecPig_AvgPrice":105}]}`)
	defer srv.Close()

	_, err := NewClient(srv.URL, srv.URL).FetchPork(context.Background(), time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("回傳日期與請求不符時必須報錯，不可靜默採用")
	}
	if !strings.Contains(err.Error(), "1150912") {
		t.Errorf("錯誤訊息應指出請求的日期，got %v", err)
	}
}

func TestFetchPork_EmptyDataIsNotAnError(t *testing.T) {
	srv := porkServer(t, `{"Data":[]}`)
	defer srv.Close()

	got, err := NewClient(srv.URL, srv.URL).FetchPork(context.Background(), time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("空陣列不該報錯: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("預期 0 列，got %d", len(got))
	}
}
