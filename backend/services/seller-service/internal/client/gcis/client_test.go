package gcis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSetupDate(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"0760221", "1987-02-21"}, // 民國 76 年（TSMC）
		{"0450825", "1956-08-25"},
		{"1120101", "2023-01-01"}, // 民國 112 年
		{"", ""},
		{"076022", ""},   // 太短
		{"07602210", ""}, // 太長
		{"abc0221", ""},  // 非數字
		{"0761301", ""},  // month 13
		{"0760232", ""},  // day 32
		{"0760000", ""},  // month/day 00
	}
	for _, tc := range cases {
		if got := parseSetupDate(tc.input); got != tc.want {
			t.Errorf("parseSetupDate(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractBusinessType(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"台灣積體電路製造股份有限公司", "股份有限公司"},
		{"某某有限公司", "有限公司"},
		{"某某無限公司", "無限公司"},
		{"某某兩合公司", "兩合公司"},
		{"某某有限合夥", "有限合夥"},
		{"個人工作室", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractBusinessType(tc.input); got != tc.want {
			t.Errorf("extractBusinessType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func mockGCIS(t *testing.T, body string, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := gcisBaseURL
	gcisBaseURL = srv.URL
	t.Cleanup(func() { gcisBaseURL = prev })
}

const successBody = `[{
	"Business_Accounting_NO": "33330333",
	"Company_Name": "台灣積體電路製造股份有限公司",
	"Company_Location": "新竹科學園區新竹市力行六路8號",
	"Company_Setup_Date": "0760221",
	"Capital_Stock_Amount": 280500000000
}]`

func TestLookup_Found(t *testing.T) {
	mockGCIS(t, successBody, http.StatusOK)
	br, err := NewClient().Lookup(context.Background(), "33330333")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br == nil {
		t.Fatal("expected company, got nil")
	}
	if br.BusinessNm != "台灣積體電路製造股份有限公司" {
		t.Errorf("BusinessNm = %q", br.BusinessNm)
	}
	if br.BusinessSetupDate != "1987-02-21" {
		t.Errorf("BusinessSetupDate = %q, want 1987-02-21", br.BusinessSetupDate)
	}
	if br.BusinessType != "股份有限公司" {
		t.Errorf("BusinessType = %q", br.BusinessType)
	}
	if br.CapitalAmount != 280500000000 {
		t.Errorf("CapitalAmount = %d", br.CapitalAmount)
	}
}

func TestLookup_NotFound(t *testing.T) {
	mockGCIS(t, `[]`, http.StatusOK)
	br, err := NewClient().Lookup(context.Background(), "00000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br != nil {
		t.Errorf("expected nil (definitive not-found), got %+v", br)
	}
}

func TestLookup_EmptyBody(t *testing.T) {
	// GCIS 對查無資料回空 body（Content-Length: 0），不是 []
	mockGCIS(t, ``, http.StatusOK)
	br, err := NewClient().Lookup(context.Background(), "00000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br != nil {
		t.Errorf("expected nil, got %+v", br)
	}
}

func TestLookup_UpstreamError(t *testing.T) {
	mockGCIS(t, `internal error`, http.StatusInternalServerError)
	br, err := NewClient().Lookup(context.Background(), "33330333")
	if err == nil {
		t.Error("expected transient error, got nil")
	}
	if br != nil {
		t.Errorf("expected nil on error, got %+v", br)
	}
}

func TestLookup_ControlCharStripped(t *testing.T) {
	body := "[{\x01\"Business_Accounting_NO\":\"12345678\",\"Company_Name\":\"測試有限公司\",\"Company_Location\":\"台北市\",\"Company_Setup_Date\":\"1120101\",\"Capital_Stock_Amount\":1000000}]"
	mockGCIS(t, body, http.StatusOK)
	br, err := NewClient().Lookup(context.Background(), "12345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br == nil || br.BusinessNm != "測試有限公司" {
		t.Errorf("control-char strip failed: %+v", br)
	}
}

// mockGCISByOid routes to companyBody or businessBody depending on which
// dataset oid the request path hits, so tests can exercise the
// company-not-found → business-fallback path (issue #1336: 行號/獨資/合夥
// aren't in the company dataset at all).
func mockGCISByOid(t *testing.T, companyBody, businessBody string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, gcisBusinessOid) {
			_, _ = w.Write([]byte(businessBody))
			return
		}
		_, _ = w.Write([]byte(companyBody))
	}))
	t.Cleanup(srv.Close)
	prev := gcisBaseURL
	gcisBaseURL = srv.URL
	t.Cleanup(func() { gcisBaseURL = prev })
}

const businessSuccessBody = `[{
	"President_No": "71777777",
	"Business_Name": "範例食品企業社",
	"Business_Current_Status_Desc": "核准設立",
	"Business_Setup_Approve_Date": "1040713",
	"Business_Organization_Type_Desc": "獨資",
	"Agency_Desc": "高雄市政府",
	"Business_Address": "高雄市彌陀區中正路1號1樓"
}]`

func TestLookup_CompanyNotFound_FallsBackToBusiness(t *testing.T) {
	mockGCISByOid(t, `[]`, businessSuccessBody)
	br, err := NewClient().Lookup(context.Background(), "71777777")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br == nil {
		t.Fatal("expected business registration, got nil")
	}
	if br.BusinessNm != "範例食品企業社" {
		t.Errorf("BusinessNm = %q", br.BusinessNm)
	}
	if br.BusinessType != "獨資" {
		t.Errorf("BusinessType = %q", br.BusinessType)
	}
	if br.BusinessSetupDate != "2015-07-13" {
		t.Errorf("BusinessSetupDate = %q, want 2015-07-13", br.BusinessSetupDate)
	}
	if br.BusinessAddress != "高雄市彌陀區中正路1號1樓" {
		t.Errorf("BusinessAddress = %q", br.BusinessAddress)
	}
}

func TestLookup_NeitherFound(t *testing.T) {
	mockGCISByOid(t, `[]`, `[]`)
	br, err := NewClient().Lookup(context.Background(), "00000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br != nil {
		t.Errorf("expected nil (not in either dataset), got %+v", br)
	}
}

func TestLookup_BusinessFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, gcisBusinessOid) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	prev := gcisBaseURL
	gcisBaseURL = srv.URL
	t.Cleanup(func() { gcisBaseURL = prev })

	br, err := NewClient().Lookup(context.Background(), "00000000")
	if err == nil {
		t.Error("expected transient error from business fetch, got nil")
	}
	if br != nil {
		t.Errorf("expected nil on error, got %+v", br)
	}
}
