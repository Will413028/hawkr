package moa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const restDayFixture = `{"RS":"OK","Next":false,"Data":[
 {"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
   {"MarketType":"F","YearList":[
     {"Year":115,"MonthList":[
       {"Month":1,"Rest":"05、08、12"},
       {"Month":2,"Rest":"02、09"}
     ]}
   ]}
 ]}
]}`

func TestFetchRestDaysExpandsNestedStructure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(restDayFixture))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	if len(days) != 5 {
		t.Fatalf("展開後 = %d 筆，預期 5 筆", len(days))
	}
	first := days[0]
	if first.MarketCode != "104" {
		t.Errorf("MarketCode = %q，預期 104", first.MarketCode)
	}
	if first.MarketName != "台北二" {
		t.Errorf("MarketName = %q，預期 台北二", first.MarketName)
	}
	if got := first.Date.Format("2006-01-02"); got != "2026-01-05" {
		t.Errorf("第一筆日期 = %s，預期 2026-01-05（民國 115 年 1 月 5 日）", got)
	}
}

// TestFetchRestDaysSkipsMalformedDays 防的是 Rest 欄位可能有空字串或非數字 token，
// 不可讓整批解析失敗——單一壞 token 只需略過那個 token，其餘照常展開。
func TestFetchRestDaysSkipsMalformedDays(t *testing.T) {
	body := `{"RS":"OK","Data":[{"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
	  {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":2,"Rest":"30、、abc、09"}]}]}]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	// 2 月 30 日不存在（2026 年非閏年）、空字串與 abc 都要略過，只剩 2/09。
	if len(days) != 1 || days[0].Date.Format("2006-01-02") != "2026-02-09" {
		t.Errorf("= %+v，預期只留下 2026-02-09", days)
	}
}

// TestFetchRestDaysKeepsSameDayPerMarketType 同一市場在多個 MarketType 下同一天
// 都列為休市時，兩筆都要保留——跨 type 的同一天是兩筆獨立的休市事實，不是重複
// 資料。這是本測試原本斷言的反面（原名
// TestFetchRestDaysDedupesAcrossMarketTypes、原斷言 len(days)==1）：2026-07-30
// 實打 MOA API 量測到 21 個市場中有 3 個（台中市 400 / 高雄市 800 / 溪湖鎮 514）
// 的類別間休市排程每年差 71-73 天，若把 MarketType 併掉當重複資料去重，會讓
// 只在花卉類休市的那些天，蔬菜／水果也被假標成休市。同一個 type 內的重複 token
// 仍然要去重，見 TestFetchRestDaysDedupesRepeatedTokenWithinSameMarketType。
func TestFetchRestDaysKeepsSameDayPerMarketType(t *testing.T) {
	body := `{"RS":"OK","Data":[{"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
	  {"MarketType":"F","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]},
	  {"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}
	]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("同一天在兩個 MarketType 下應各保留一筆，得到 %d 筆: %+v", len(days), days)
	}
	types := map[string]bool{}
	for _, d := range days {
		if d.Date.Format("2006-01-02") != "2026-01-05" {
			t.Errorf("非預期日期 %s", d.Date.Format("2006-01-02"))
		}
		types[d.MarketType] = true
	}
	if !types["F"] || !types["V"] {
		t.Errorf("預期 F 與 V 各一筆，實得 %v", types)
	}
}

// TestFetchRestDaysSkipsYearListMissingYear 防的是 YearList 缺 Year 欄位（JSON 缺鍵
// 或顯式 null）時，int 零值解出來是 0，yr.Year+1911 = 1911——day/month 的 round-trip
// 檢查完全擋不住這個情境（1911 年 1 月 5 日是行事曆上真實存在的日期），這種壞資料
// 若不在年份這一層另外擋，會安靜寫出一筆看起來正常、年份卻是亂數的休市日。
// 這組測試混入一個正常的 YearList 條目，確認壞條目被整條略過、好條目不受影響。
func TestFetchRestDaysSkipsYearListMissingYear(t *testing.T) {
	body := `{"RS":"OK","Data":[{"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
	  {"MarketType":"F","YearList":[
	    {"MonthList":[{"Month":1,"Rest":"05"}]},
	    {"Year":115,"MonthList":[{"Month":1,"Rest":"12"}]}
	  ]}
	]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	if len(days) != 1 || days[0].Date.Format("2006-01-02") != "2026-01-12" {
		t.Fatalf("= %+v，預期只留下民國 115 年那筆 2026-01-12，缺 Year 的條目應整條被略過", days)
	}
	for _, d := range days {
		if d.Date.Year() == 1911 {
			t.Errorf("缺 Year 的 YearList 條目不應被解讀成民國 0 年（西元 1911）: %+v", d)
		}
	}
}

// 同一市場、同一年、兩個 MarketType 休市日不同——這是 2026-07-30 實測到的
// 真實形狀（台中市 400 / 高雄市 800 / 溪湖鎮 514 三個市場都是這樣）。
// 舊實作的去重 key 是 MarkerNo+date，會把兩個 type 的休市日併成聯集。
const twoMarketTypeRestDayFixture = `{"RS":"OK","Data":[{
  "MarkerNo":"400","MarkerName":"台中市",
  "MarketTypeList":[
    {"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":7,"Rest":"05、12"}]}]},
    {"MarketType":"L","YearList":[{"Year":115,"MonthList":[{"Month":7,"Rest":"05、20"}]}]}
  ]}],"Next":false}`

func TestFetchRestDaysKeepsMarketTypeSeparate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(twoMarketTypeRestDayFixture))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}

	type key struct {
		marketType string
		date       string
	}
	got := map[key]bool{}
	for _, d := range days {
		if d.MarketCode != "400" {
			t.Fatalf("非預期的市場代碼 %q", d.MarketCode)
		}
		got[key{d.MarketType, d.Date.Format("2006-01-02")}] = true
	}
	want := []key{
		{"V", "2026-07-05"}, {"V", "2026-07-12"},
		{"L", "2026-07-05"}, {"L", "2026-07-20"},
	}
	if len(got) != len(want) {
		t.Fatalf("休市列數 = %d，預期 %d（舊實作跨 MarketType 去重會得到 3 筆聯集）: %v", len(got), len(want), got)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("缺少 %s %s", w.marketType, w.date)
		}
	}
}

// TestFetchRestDaysSkipsMarketTypeListEntryMissingMarketType 防的是
// MarketTypeList 條目缺 MarketType 欄位（JSON 缺鍵或空字串）——與
// TestFetchRestDaysSkipsYearListMissingYear 防的 yr.Year 缺鍵同一類缺陷，
// 只是巢狀層次不同。這道 guard 不只是資料品質問題：usecase 端把空字串
// MarketType 存成 market_type IS NULL，而 RestDayIngest 每輪結束會執行
// DELETE ... WHERE market_type IS NULL 清理 Task 3 之前的舊列——沒有這道
// guard，殘缺條目展開出的休市日會在同一輪 ingest 剛寫入就被判定成舊列，
// 當場被刪掉，written 計數與實際列數不一致，且不會報錯（2026-07-30 review
// 用隔離 testcontainer 實測過：written=2、market_rest_day 實際 0 列）。
func TestFetchRestDaysSkipsMarketTypeListEntryMissingMarketType(t *testing.T) {
	body := `{"RS":"OK","Data":[{"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
	  {"YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"08"}]}]},
	  {"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}
	]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	if len(days) != 1 || days[0].MarketType != "V" || days[0].Date.Format("2006-01-02") != "2026-01-05" {
		t.Fatalf("= %+v，預期只留下 V 的 2026-01-05，缺 MarketType 的條目應整條被略過", days)
	}
}

// TestFetchRestDaysSkipsDataEntryMissingMarkerNo 防的是 Data[] 條目缺 MarkerNo
// 欄位（JSON 缺鍵或空字串）——與 TestFetchRestDaysSkipsMarketTypeListEntryMissingMarketType
// 防的 MarketType 缺鍵同一類缺陷，只是外層一階。這道 guard 不只是資料品質問題：
// listMarketCodes（price_ingest.go）直接拿 FetchRestDays 的輸出當市場清單，沒有
// 這道 guard，空字串會混進市場清單，讓 FetchRange 收到 marketCode==""——等同
// 查全部市場全部作物，實測單日就觸頂 1000 列上限，最終讓 Handle 的市場迴圈整批
// 中止，波及當天所有市場的 ingest（不只是壞資料那筆）。
func TestFetchRestDaysSkipsDataEntryMissingMarkerNo(t *testing.T) {
	body := `{"RS":"OK","Data":[
	  {"MarketTypeList":[{"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"08"}]}]}]},
	  {"MarkerNo":"104","MarketTypeList":[{"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05"}]}]}]}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	if len(days) != 1 || days[0].MarketCode != "104" || days[0].Date.Format("2006-01-02") != "2026-01-05" {
		t.Fatalf("= %+v，預期只留下 MarkerNo=104 的 2026-01-05，缺 MarkerNo 的條目應整條被略過", days)
	}
}

// TestFetchRestDaysDedupesRepeatedTokenWithinSameMarketType 防的是同一個
// MarketType 內 Rest 欄位重複列出同一天（例如 "05、05"）——這條路徑與跨
// MarketType 的去重（TestFetchRestDaysKeepsMarketTypeSeparate）是不同的
// 斷言：那條測試用的 fixture 各 MarketType 內都沒有重複 token，砍掉 seen
// 去重整段也會全綠，不覆蓋這裡。
func TestFetchRestDaysDedupesRepeatedTokenWithinSameMarketType(t *testing.T) {
	body := `{"RS":"OK","Data":[{"MarkerNo":"104","MarkerName":"台北二","MarketTypeList":[
	  {"MarketType":"V","YearList":[{"Year":115,"MonthList":[{"Month":1,"Rest":"05、05"}]}]}
	]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	days, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("同一 MarketType 內重複的 token 應被去重成 1 筆，得到 %d 筆: %+v", len(days), days)
	}
}

func TestFetchRestDaysErrorsOnNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	if _, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background()); err == nil {
		t.Error("預期非 200 狀態碼回錯誤")
	}
}

// TestFetchRestDaysCapsResponseBodySize 建一個遠超過 maxResponseBodyBytes 的
// 合法 JSON body，驗證這支端點跟 FetchRange 共用同一個 io.LimitReader 上限，
// 不會無界緩衝整個 body（P4 review followup）。
func TestFetchRestDaysCapsResponseBodySize(t *testing.T) {
	huge := strings.Repeat("x", maxResponseBodyBytes+1024)
	fixture := `{"RS":"OK","Next":false,"Data":[{"MarkerNo":"104","MarkerName":"` + huge + `","MarketTypeList":[]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(srv.Close)

	if _, err := NewClient(srv.URL, srv.URL).FetchRestDays(context.Background()); err == nil {
		t.Error("超過 body size 上限的回應應該回錯誤，不能被無界解析成功")
	}
}
