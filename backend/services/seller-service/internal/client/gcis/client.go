// Package gcis is an anti-corruption layer over the 經濟部商業司 GCIS open-data
// registry. It translates GCIS's foreign model (民國 dates, embedded control
// chars, odata $filter, raw field names) into the seller domain's
// BusinessRegistration, isolating GCIS quirks behind one boundary.
//
// GCIS splits company registration (公司登記, filed nationally) and business
// registration (商業登記, i.e. 行號/獨資/合夥, filed with local governments)
// into separate datasets. Lookup queries the company dataset first and falls
// back to the business dataset on a definitive not-found, so both registration
// types resolve through the same ban (see issue #1336).
package gcis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// gcisBaseURL is overridable in tests to point at a mock server.
var gcisBaseURL = "https://data.gcis.nat.gov.tw"

// gcisCompanyOid is 公司登記基本資料-應用一 (company registration only).
// gcisBusinessOid is 商業登記基本資料-應用三 (行號/獨資/合夥); unlike -應用一/二
// it filters by ban alone, no registering agency code required.
const (
	gcisCompanyOid  = "5F64D864-61CB-4D0D-8AD9-492047CC1EA6"
	gcisBusinessOid = "426D5542-5F05-43EB-83F9-F1300F14E1F1"
)

// gcisControlCharRe strips the control characters GCIS embeds in its JSON.
var gcisControlCharRe = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`)

// gcisCompanyTypes known company-type suffixes (order matters: longest first).
var gcisCompanyTypes = []string{
	"股份有限公司",
	"有限合夥",
	"無限公司",
	"兩合公司",
	"有限公司",
}

// gcisCompanyRaw mirrors GCIS's raw 公司登記 response fields.
type gcisCompanyRaw struct {
	BusinessAccountingNO string `json:"Business_Accounting_NO"`
	CompanyName          string `json:"Company_Name"`
	CompanyLocation      string `json:"Company_Location"`
	CompanySetupDate     string `json:"Company_Setup_Date"` // 民國 YYYMMDD
	CapitalStockAmount   int64  `json:"Capital_Stock_Amount"`
}

// gcisBusinessRaw mirrors GCIS's raw 商業登記（行號/獨資/合夥）response fields.
// This dataset has no capital-amount field.
type gcisBusinessRaw struct {
	PresidentNo              string `json:"President_No"`
	BusinessName             string `json:"Business_Name"`
	BusinessAddress          string `json:"Business_Address"`
	BusinessSetupApproveDate string `json:"Business_Setup_Approve_Date"` // 民國 YYYMMDD
	BusinessOrganizationType string `json:"Business_Organization_Type_Desc"`
}

// Client calls the GCIS registry. It implements usecase.GcisClient.
type Client struct {
	httpClient *http.Client
}

// NewClient returns a GCIS client with a bounded timeout. The upstream is an
// external government API; a missing timeout would hang callers indefinitely.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 5 * time.Second}}
}

// Lookup returns the registration for ban, (nil, nil) when GCIS is reachable
// but ban exists in neither the company nor the business dataset (definitive
// not-found), or (nil, err) on transient upstream failure so the caller can
// fail-soft.
func (c *Client) Lookup(ctx context.Context, ban string) (*usecase.BusinessRegistration, error) {
	company, err := c.fetchCompany(ctx, ban)
	if err != nil {
		return nil, err
	}
	if company != nil {
		return &usecase.BusinessRegistration{
			Ban:               company.BusinessAccountingNO,
			BusinessNm:        company.CompanyName,
			BusinessAddress:   company.CompanyLocation,
			BusinessSetupDate: parseSetupDate(company.CompanySetupDate),
			BusinessType:      extractBusinessType(company.CompanyName),
			CapitalAmount:     company.CapitalStockAmount,
		}, nil
	}

	// Not a company registration — try 商業登記 (行號/獨資/合夥) before
	// declaring not-found (issue #1336).
	business, err := c.fetchBusiness(ctx, ban)
	if err != nil {
		return nil, err
	}
	if business == nil {
		return nil, nil
	}
	return &usecase.BusinessRegistration{
		Ban:               business.PresidentNo,
		BusinessNm:        business.BusinessName,
		BusinessAddress:   business.BusinessAddress,
		BusinessSetupDate: parseSetupDate(business.BusinessSetupApproveDate),
		BusinessType:      business.BusinessOrganizationType,
	}, nil
}

func (c *Client) fetchCompany(ctx context.Context, ban string) (*gcisCompanyRaw, error) {
	raw, err := c.fetchOne(ctx, gcisCompanyOid, "Business_Accounting_NO", ban)
	if err != nil || raw == nil {
		return nil, err
	}
	var company gcisCompanyRaw
	if err := json.Unmarshal(raw, &company); err != nil {
		return nil, fmt.Errorf("GCIS parse error: %w", err)
	}
	return &company, nil
}

func (c *Client) fetchBusiness(ctx context.Context, ban string) (*gcisBusinessRaw, error) {
	raw, err := c.fetchOne(ctx, gcisBusinessOid, "President_No", ban)
	if err != nil || raw == nil {
		return nil, err
	}
	var business gcisBusinessRaw
	if err := json.Unmarshal(raw, &business); err != nil {
		return nil, fmt.Errorf("GCIS parse error: %w", err)
	}
	return &business, nil
}

// fetchOne runs one GCIS $filter=<field> eq <ban> query and returns the raw
// JSON of the first matching record, or (nil, nil) on a definitive
// not-found (empty body or empty array — GCIS uses both for "no match").
func (c *Client) fetchOne(ctx context.Context, oid, field, ban string) (json.RawMessage, error) {
	apiURL := fmt.Sprintf(
		"%s/od/data/api/%s?$format=json&$filter=%s%%20eq%%20%s&$skip=0&$top=1",
		gcisBaseURL, oid, field, url.QueryEscape(ban),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GCIS returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	cleaned := gcisControlCharRe.ReplaceAll(body, nil)
	if len(cleaned) == 0 {
		return nil, nil
	}
	var records []json.RawMessage
	if err := json.Unmarshal(cleaned, &records); err != nil {
		return nil, fmt.Errorf("GCIS parse error: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// parseSetupDate converts 民國 YYYMMDD → 西元 YYYY-MM-DD; returns "" if malformed.
func parseSetupDate(s string) string {
	if len(s) != 7 {
		return ""
	}
	rocYear, err := strconv.Atoi(s[:3])
	if err != nil || rocYear < 0 {
		return ""
	}
	month, err := strconv.Atoi(s[3:5])
	if err != nil || month < 1 || month > 12 {
		return ""
	}
	day, err := strconv.Atoi(s[5:7])
	if err != nil || day < 1 || day > 31 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", rocYear+1911, month, day)
}

// extractBusinessType pulls the company-type suffix from the name; "" if none.
func extractBusinessType(name string) string {
	for _, t := range gcisCompanyTypes {
		if strings.HasSuffix(name, t) {
			return t
		}
	}
	return ""
}
