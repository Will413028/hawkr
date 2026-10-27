package gcis

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fake 以資料集的 oid 回不同的本文，記下每個請求。
type fake struct {
	company, business         string
	companyStatus, bizStatus  int
	companyDelay              time.Duration
	companyCalls, bizCalls    atomic.Int32
	lastCompanyQuery, lastBiz atomic.Value
}

func (f *fake) handler(w http.ResponseWriter, r *http.Request) {
	status, body := f.companyStatus, f.company
	switch {
	case strings.Contains(r.URL.Path, businessOid):
		f.bizCalls.Add(1)
		f.lastBiz.Store(r.URL.RawQuery)
		status, body = f.bizStatus, f.business
	default:
		f.companyCalls.Add(1)
		f.lastCompanyQuery.Store(r.URL.RawQuery)
		if f.companyDelay > 0 {
			select {
			case <-time.After(f.companyDelay):
			case <-r.Context().Done():
				return
			}
		}
	}
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func newClient(t *testing.T, f *fake, timeout time.Duration) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL, Timeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const companyBody = `[{
	"Business_Accounting_NO": "33330333",
	"Company_Name": "台灣積體電路製造股份有限公司",
	"Company_Location": "新竹科學園區新竹市力行六路8號",
	"Company_Setup_Date": "0760221",
	"Company_Status_Desc": "核准設立",
	"Sus_Beg_Date": "",
	"Sus_End_Date": "",
	"Capital_Stock_Amount": 280500000000
}]`

const businessBody = `[{
	"President_No": "71777777",
	"Business_Name": "範例食品企業社",
	"Business_Address": "高雄市彌陀區中正路1號1樓",
	"Business_Setup_Approve_Date": "1040713",
	"Business_Organization_Type_Desc": "獨資",
	"Business_Current_Status": "01",
	"Business_Current_Status_Desc": "核准設立"
}]`

func TestLookupCompany(t *testing.T) {
	f := &fake{company: companyBody}
	reg, found, err := newClient(t, f, 0).Lookup(context.Background(), "33330333")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	want := Registration{
		TaxID: "33330333", Name: "台灣積體電路製造股份有限公司", Address: "新竹科學園區新竹市力行六路8號",
		SetupDate: time.Date(1987, 2, 21, 0, 0, 0, 0, time.UTC), OrganizationType: "股份有限公司",
		Source: SourceCompany, StatusText: "核准設立",
	}
	if reg != want {
		t.Fatalf("reg=%+v", reg)
	}
	if f.bizCalls.Load() != 0 {
		t.Fatal("公司登記查得到時不該再查商業登記")
	}
	if q, _ := f.lastCompanyQuery.Load().(string); !strings.Contains(q, "Business_Accounting_NO%20eq%2033330333") {
		t.Fatalf("query=%q", q)
	}
}

// 公司登記查無時改查商業登記（行號、獨資、合夥不在公司登記的資料集）。
func TestLookupFallsBackToBusinessRegistration(t *testing.T) {
	f := &fake{company: `[]`, business: businessBody}
	reg, found, err := newClient(t, f, 0).Lookup(context.Background(), "71777777")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if reg.Name != "範例食品企業社" || reg.OrganizationType != "獨資" || reg.SetupDate != time.Date(2015, 7, 13, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("reg=%+v", reg)
	}
}

// 兩個資料集都查無：found 為 false、沒有錯誤；空本文與空陣列都是查無。
func TestLookupNotFoundIsNotAnError(t *testing.T) {
	for name, f := range map[string]*fake{
		"空陣列": {company: `[]`, business: `[]`},
		"空本文": {company: ``, business: ``},
	} {
		t.Run(name, func(t *testing.T) {
			reg, found, err := newClient(t, f, 0).Lookup(context.Background(), "00000000")
			if err != nil || found || reg != (Registration{}) {
				t.Fatalf("reg=%+v found=%v err=%v", reg, found, err)
			}
		})
	}
}

// 不可用與查無是不同的結果：任何一個資料集沒有給出可用的答案都是 ErrUnavailable，即使另一個查無。
func TestLookupUnavailableIsNeverNotFound(t *testing.T) {
	for name, f := range map[string]*fake{
		"公司登記 500":    {companyStatus: http.StatusInternalServerError, company: `oops`, business: businessBody},
		"商業登記 503":    {company: `[]`, bizStatus: http.StatusServiceUnavailable},
		"公司登記不是 JSON": {company: `<html>maintenance</html>`},
		"商業登記不是陣列":    {company: `[]`, business: `{"error":"x"}`},
		"回應過大":        {company: strings.Repeat(" ", maxResponse+10) + `[]`},
	} {
		t.Run(name, func(t *testing.T) {
			_, found, err := newClient(t, f, 0).Lookup(context.Background(), "71777777")
			if !errors.Is(err, ErrUnavailable) || found {
				t.Fatalf("found=%v err=%v", found, err)
			}
		})
	}
}

// 逾時是不可用；總上限涵蓋兩個請求。
func TestLookupTimeoutIsUnavailable(t *testing.T) {
	f := &fake{company: companyBody, companyDelay: 2 * time.Second}
	start := time.Now()
	_, found, err := newClient(t, f, 50*time.Millisecond).Lookup(context.Background(), "33330333")
	if !errors.Is(err, ErrUnavailable) || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("沒有在總上限內返回：%v", time.Since(start))
	}
}

// 呼叫方取消不是「不可用」。
func TestLookupCallerCancellationIsNotUnavailable(t *testing.T) {
	f := &fake{company: companyBody, companyDelay: 2 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	_, _, err := newClient(t, f, 5*time.Second).Lookup(ctx, "33330333")
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

// 統編不是 8 碼數字時不發出任何請求（不讓不合格式的值進入查詢條件）。
func TestLookupRejectsMalformedTaxIDsWithoutACall(t *testing.T) {
	f := &fake{company: companyBody}
	c := newClient(t, f, 0)
	for _, id := range []string{"", "1234567", "123456789", "1234567a", "1 or 1", "12345678'", "１２３４５６７８"} {
		if _, found, err := c.Lookup(context.Background(), id); !errors.Is(err, ErrInvalidTaxID) || found {
			t.Errorf("%q：found=%v err=%v", id, found, err)
		}
	}
	if f.companyCalls.Load() != 0 {
		t.Fatal("格式不合的統編發出了請求")
	}
}

// 對方夾在 JSON 裡的控制字元先去掉再解析。
func TestLookupStripsControlCharacters(t *testing.T) {
	body := "[{\x01\"Business_Accounting_NO\":\"12345678\",\"Company_Name\":\"測試有限公司\",\"Company_Location\":\"台北市\",\"Company_Setup_Date\":\"1120101\"}]"
	reg, found, err := newClient(t, &fake{company: body}, 0).Lookup(context.Background(), "12345678")
	if err != nil || !found || reg.Name != "測試有限公司" || reg.OrganizationType != "有限公司" {
		t.Fatalf("reg=%+v found=%v err=%v", reg, found, err)
	}
}

// 民國日期只接受存在的日期；不合法時其他欄位照樣回傳，設立日期是零值。
func TestParseRepublicDate(t *testing.T) {
	for in, want := range map[string]time.Time{
		"0760221": time.Date(1987, 2, 21, 0, 0, 0, 0, time.UTC),
		"1120101": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		"1120229": {}, // 民國 112 年不是閏年
		"1090229": time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC),
		"0761301": {}, "0760232": {}, "0760000": {}, "0000101": {}, "": {}, "076022": {}, "07602210": {}, "abc0221": {},
	} {
		if got := parseRepublicDate(in); !got.Equal(want) {
			t.Errorf("parseRepublicDate(%q) = %v，預期 %v", in, got, want)
		}
	}
}

func TestCompanyType(t *testing.T) {
	for in, want := range map[string]string{
		"台灣積體電路製造股份有限公司": "股份有限公司", "某某有限公司": "有限公司", "某某無限公司": "無限公司",
		"某某兩合公司": "兩合公司", "某某有限合夥": "有限合夥", "個人工作室": "", "": "",
	} {
		if got := companyType(in); got != want {
			t.Errorf("companyType(%q) = %q，預期 %q", in, got, want)
		}
	}
}

func TestNewValidatesTheBaseURL(t *testing.T) {
	for _, bad := range []string{"data.gcis.nat.gov.tw", "ftp://x", "https://"} {
		if _, err := New(Config{BaseURL: bad}); err == nil {
			t.Errorf("%q 通過了檢查", bad)
		}
	}
	c, err := New(Config{})
	if err != nil || c.base.String() != DefaultBaseURL || c.timeout != DefaultTimeout {
		t.Fatalf("預設值：%+v err=%v", c, err)
	}
}

// 不跟隨轉址：轉址是來源沒有給出可用的答案。
func TestLookupDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(companyBody))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	c, err := New(Config{BaseURL: redirect.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.Lookup(context.Background(), "33330333"); !errors.Is(err, ErrUnavailable) || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

// 回來的那一筆不是查的統編（對方忽略篩選條件）或沒有名稱（[null]、[{}]）：不是查到，也不是查無，是沒有給出可用的答案。
func TestLookupRejectsRecordsThatAreNotTheOneAskedFor(t *testing.T) {
	for name, f := range map[string]*fake{
		"公司登記回別的統編":  {company: companyBody},
		"公司登記回 null": {company: `[null]`},
		"公司登記回空物件":   {company: `[{}]`},
		"商業登記回別的統編":  {company: `[]`, business: businessBody},
		"商業登記回 null": {company: `[]`, business: `[null]`},
		"商業登記回空物件":   {company: `[]`, business: `[{}]`},
	} {
		t.Run(name, func(t *testing.T) {
			reg, found, err := newClient(t, f, 0).Lookup(context.Background(), "00000000")
			if !errors.Is(err, ErrUnavailable) || found || reg != (Registration{}) {
				t.Fatalf("reg=%+v found=%v err=%v", reg, found, err)
			}
		})
	}
}

// 現況與停業期間只回來源的值，不分類、不判斷：公司的現況是原文，商業登記是代碼加原文，顛倒的停業期間原樣回傳
// （分類與合理性檢查在使用的模組，model.NewRegistration）。
func TestLookupReturnsTheRawStatusAndSuspensionPeriod(t *testing.T) {
	ctx := context.Background()
	for _, text := range []string{"核准設立", "核准設立，但已命令解散", "廢止已清算完結", "", "未來新增的現況"} {
		body := `[{"Business_Accounting_NO":"33330333","Company_Name":"某某實業股份有限公司","Company_Status_Desc":"` + text + `"}]`
		reg, found, err := newClient(t, &fake{company: body}, 0).Lookup(ctx, "33330333")
		if err != nil || !found {
			t.Fatalf("%q：found=%v err=%v", text, found, err)
		}
		if reg.Source != SourceCompany || reg.StatusText != text || reg.StatusCode != "" {
			t.Errorf("%q：reg=%+v", text, reg)
		}
	}
	// 迄日早於起日：兩個日期原樣回傳。
	inverted := `[{"Business_Accounting_NO":"33330333","Company_Name":"某某有限公司","Sus_Beg_Date":"0830331","Sus_End_Date":"0821101"}]`
	reg, _, err := newClient(t, &fake{company: inverted}, 0).Lookup(ctx, "33330333")
	if err != nil || !reg.SuspendedFrom.Equal(time.Date(1994, 3, 31, 0, 0, 0, 0, time.UTC)) || !reg.SuspendedUntil.Equal(time.Date(1993, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("reg=%+v err=%v", reg, err)
	}
	// 缺漏或不合法的日期是零值。
	broken := `[{"Business_Accounting_NO":"33330333","Company_Name":"某某有限公司","Sus_Beg_Date":"","Sus_End_Date":"abc"}]`
	reg, _, err = newClient(t, &fake{company: broken}, 0).Lookup(ctx, "33330333")
	if err != nil || !reg.SuspendedFrom.IsZero() || !reg.SuspendedUntil.IsZero() {
		t.Fatalf("reg=%+v err=%v", reg, err)
	}
	// 商業登記：代碼與原文各自回傳，沒有停業期間。
	biz := `[{"President_No":"71777777","Business_Name":"某某企業社","Business_Current_Status":"02","Business_Current_Status_Desc":"某個現況"}]`
	reg, _, err = newClient(t, &fake{company: `[]`, business: biz}, 0).Lookup(ctx, "71777777")
	if err != nil || reg.Source != SourceBusiness || reg.StatusCode != "02" || reg.StatusText != "某個現況" || !reg.SuspendedFrom.IsZero() {
		t.Fatalf("reg=%+v err=%v", reg, err)
	}
}
