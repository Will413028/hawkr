// Package gcis 是經濟部商業司公開資料（data.gcis.nat.gov.tw）的統一編號查詢 client：公司登記與商業登記
// （行號、獨資、合夥）兩個資料集，以統編查一筆登記資料。它只負責傳輸與把對方的形狀（民國日期、夾在 JSON 裡的控制
// 字元、OData 的 $filter、原始欄位名）轉成 Registration，不含任何業務判斷（現況的分類在使用的模組）。
//
// 規則（backend AGENTS.md 規則 5 與「設定與 secret」）：
//   - 每次 Lookup 有一個總 deadline（Config.Timeout，再受 ctx 限制），兩個資料集的請求共用；client 本身不重試：互動查詢
//     （人在填表）查不到或不可用時畫面退回手動填寫，重試只會加長等待並增加對公開 API 的負擔；KYC 送件時存登記資料快照的
//     查詢是背景 job，由 River 重試，重試與退避在呼叫端，不在這個 client。
//   - 「查無」與「不可用」是不同的結果：兩個資料集都連得上而且都沒有這個統編，回 found 為 false、沒有錯誤；連線、逾時、
//     非 200、回應解不開或過大一律回 ErrUnavailable，不轉成查無。
//   - 資料是公開的政府資料，沒有金鑰；統編本身不是祕密，但這裡不記 log 回應內容。
package gcis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL 是公開資料平台的網址，Config.BaseURL 沒有設定時使用。
const DefaultBaseURL = "https://data.gcis.nat.gov.tw"

// DefaultTimeout 是 Config.Timeout 的預設值：一次 Lookup（最多兩個請求）的總上限。
const DefaultTimeout = 5 * time.Second

// maxResponse 是單一回應本文的讀取上限（一筆登記資料遠小於它）。
const maxResponse = 1 << 20

// 兩個資料集的 oid。公司登記基本資料（應用一）以 Business_Accounting_NO 篩選；商業登記基本資料（應用三）只用統編
// 篩選，不需要登記機關代碼。
const (
	companyOid  = "5F64D864-61CB-4D0D-8AD9-492047CC1EA6"
	businessOid = "426D5542-5F05-43EB-83F9-F1300F14E1F1"
)

// ErrUnavailable 表示資料來源這一次沒有給出可用的答案（連線、逾時、非 200、回應解不開或過大）；不代表查無。
var ErrUnavailable = errors.New("gcis：商業登記資料來源暫時無法使用")

// ErrInvalidTaxID 表示傳入的統編不是 8 碼數字；統編的規則（格式加檢查碼）在 seller 模組的 NormalizeTaxID，呼叫方先驗證，這裡只是
// 不讓不合格式的值進入查詢條件的保底。
var ErrInvalidTaxID = errors.New("gcis：統一編號必須是 8 碼數字")

// Config 是 client 的設定。
type Config struct {
	// BaseURL 是 API 的根網址（含 scheme）；空字串用 DefaultBaseURL。
	BaseURL string
	// Timeout 是一次 Lookup 的總上限，0 用 DefaultTimeout。
	Timeout time.Duration
	// HTTPClient 給測試注入；nil 時建一個不跟隨轉址的 client。
	HTTPClient *http.Client
}

// Source 是這筆資料來自哪個資料集：兩者的現況寫法不同（公司只有文字，商業登記是代碼加文字）。
type Source string

const (
	// SourceCompany 是公司登記。
	SourceCompany Source = "COMPANY"
	// SourceBusiness 是商業登記（行號、獨資、合夥）。
	SourceBusiness Source = "BUSINESS"
)

// Registration 是一筆登記資料，只有來源給的值（原文與解析後的日期），沒有任何業務判斷：現況的分類、停業期間是否合理、地址拆成
// 鄉鎮市區，都是呼叫方（seller 模組的 model.NewRegistration，唯一一份）的事。沒有的欄位是零值。
type Registration struct {
	Source Source
	// TaxID 是統編。
	TaxID string
	// Name 是公司或商號的名稱。
	Name string
	// Address 是登記的地址，原樣的文字。
	Address string
	// SetupDate 是設立（核准設立）日期，只有日曆日有意義（UTC 午夜）；來源缺漏或不合法時是零值。
	SetupDate time.Time
	// OrganizationType 是組織型態：公司登記取自名稱的結尾（股份有限公司、有限公司……），商業登記取自登記的組織型態
	// （獨資、合夥……）；判斷不出時是空字串。
	OrganizationType string
	// StatusCode 是商業登記的現況代碼（Business_Current_Status）；公司登記沒有代碼，是空字串。StatusText 是現況的原文
	// （公司：Company_Status_Desc；商業登記：Business_Current_Status_Desc）。
	StatusCode, StatusText string
	// SuspendedFrom、SuspendedUntil 是公司登記的停業起訖（Sus_Beg_Date、Sus_End_Date，日曆日，UTC 午夜）；來源缺漏或不合法時
	// 是零值。原樣回傳，包括迄日早於起日的情形。
	SuspendedFrom, SuspendedUntil time.Time
}

// Client 查詢統編。
type Client struct {
	base    *url.URL
	timeout time.Duration
	http    *http.Client
}

// New 建立 client；網址格式不合時回錯誤。
func New(cfg Config) (*Client, error) {
	raw := cfg.BaseURL
	if raw == "" {
		raw = DefaultBaseURL
	}
	base, err := url.Parse(raw)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
		return nil, errors.New("gcis：BaseURL 必須是含 scheme 與主機的網址")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Client{base: base, timeout: timeout, http: hc}, nil
}

var taxIDRe = regexp.MustCompile(`^[0-9]{8}$`)

// Lookup 以統編查登記資料：先查公司登記，沒有再查商業登記（行號、獨資、合夥不在公司登記的資料集裡）。found 為 false
// 且沒有錯誤是兩個資料集都查無；err 是 ErrUnavailable 表示來源這一次沒有給出可用的答案。ctx 被呼叫方取消時回 ctx 的
// 錯誤，不當成不可用。外部呼叫，不可在資料庫 transaction 內呼叫。
func (c *Client) Lookup(ctx context.Context, taxID string) (reg Registration, found bool, err error) {
	if !taxIDRe.MatchString(taxID) {
		return Registration{}, false, ErrInvalidTaxID
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	defer func() {
		if err != nil && !errors.Is(err, ErrInvalidTaxID) {
			reg, found = Registration{}, false
			if parent := ctx.Err(); errors.Is(parent, context.Canceled) {
				err = parent
			} else if !errors.Is(err, ErrUnavailable) {
				err = fmt.Errorf("%w：%w", ErrUnavailable, err)
			}
		}
	}()
	raw, err := c.fetchOne(ctx, companyOid, "Business_Accounting_NO", taxID)
	if err != nil {
		return Registration{}, false, err
	}
	if raw != nil {
		var company struct {
			TaxID    string `json:"Business_Accounting_NO"`
			Name     string `json:"Company_Name"`
			Location string `json:"Company_Location"`
			Setup    string `json:"Company_Setup_Date"`
			Status   string `json:"Company_Status_Desc"`
			SusBeg   string `json:"Sus_Beg_Date"`
			SusEnd   string `json:"Sus_End_Date"`
		}
		if err := json.Unmarshal(raw, &company); err != nil {
			return Registration{}, false, fmt.Errorf("解析公司登記資料：%w", err)
		}
		if err := checkRecord(taxID, company.TaxID, company.Name); err != nil {
			return Registration{}, false, err
		}
		return Registration{
			Source: SourceCompany, TaxID: company.TaxID, Name: company.Name, Address: company.Location,
			SetupDate: parseRepublicDate(company.Setup), OrganizationType: companyType(company.Name), StatusText: company.Status,
			SuspendedFrom: parseRepublicDate(company.SusBeg), SuspendedUntil: parseRepublicDate(company.SusEnd),
		}, true, nil
	}
	raw, err = c.fetchOne(ctx, businessOid, "President_No", taxID)
	if err != nil {
		return Registration{}, false, err
	}
	if raw == nil {
		return Registration{}, false, nil
	}
	var business struct {
		TaxID    string `json:"President_No"`
		Name     string `json:"Business_Name"`
		Address  string `json:"Business_Address"`
		Approved string `json:"Business_Setup_Approve_Date"`
		Type     string `json:"Business_Organization_Type_Desc"`
		Status   string `json:"Business_Current_Status"`
		Desc     string `json:"Business_Current_Status_Desc"`
	}
	if err := json.Unmarshal(raw, &business); err != nil {
		return Registration{}, false, fmt.Errorf("解析商業登記資料：%w", err)
	}
	if err := checkRecord(taxID, business.TaxID, business.Name); err != nil {
		return Registration{}, false, err
	}
	return Registration{
		Source: SourceBusiness, TaxID: business.TaxID, Name: business.Name, Address: business.Address,
		SetupDate: parseRepublicDate(business.Approved), OrganizationType: business.Type,
		StatusCode: business.Status, StatusText: business.Desc,
	}, true, nil
}

// checkRecord 確認回來的那一筆就是查的統編、而且有名稱：對方忽略篩選條件（回資料集的第一筆）或回 [null]、[{}] 時，
// 不能當成查到，那是沒有給出可用的答案。
func checkRecord(asked, got, name string) error {
	if got != asked || strings.TrimSpace(name) == "" {
		return errors.New("資料來源回的不是查詢的統編，或沒有名稱")
	}
	return nil
}

// controlChars 是對方夾在 JSON 裡的控制字元（會讓標準的 JSON 解析失敗）。
var controlChars = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`)

// fetchOne 以 $filter=<field> eq <統編> 查一個資料集，回第一筆的原始 JSON；nil 是這個資料集查無（空本文與空陣列都是
// 對方表示查無的寫法）。非 200 與無法解讀的回應是錯誤。
func (c *Client) fetchOne(ctx context.Context, oid, field, taxID string) (json.RawMessage, error) {
	u := *c.base
	u.Path = strings.TrimRight(u.Path, "/") + "/od/data/api/" + oid
	u.RawQuery = "$format=json&$filter=" + url.PathEscape(field+" eq "+taxID) + "&$skip=0&$top=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("資料來源回應 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponse {
		return nil, errors.New("資料來源的回應過大")
	}
	body = controlChars.ReplaceAll(body, nil)
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	var records []json.RawMessage
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("資料來源的回應不是預期的格式：%w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// parseRepublicDate 把民國 YYYMMDD 轉成日曆日（UTC 午夜）；格式不合或不是存在的日期時回零值。
func parseRepublicDate(s string) time.Time {
	if len(s) != 7 {
		return time.Time{}
	}
	year, err := strconv.Atoi(s[:3])
	if err != nil || year < 1 {
		return time.Time{}
	}
	month, err := strconv.Atoi(s[3:5])
	if err != nil {
		return time.Time{}
	}
	day, err := strconv.Atoi(s[5:7])
	if err != nil {
		return time.Time{}
	}
	d := time.Date(year+1911, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if d.Year() != year+1911 || int(d.Month()) != month || d.Day() != day {
		return time.Time{}
	}
	return d
}

// companyTypes 是公司型態的結尾；「股份有限公司」以「有限公司」為結尾，所以先判斷它。
var companyTypes = []string{"股份有限公司", "有限合夥", "無限公司", "兩合公司", "有限公司"}

func companyType(name string) string {
	for _, t := range companyTypes {
		if strings.HasSuffix(name, t) {
			return t
		}
	}
	return ""
}
