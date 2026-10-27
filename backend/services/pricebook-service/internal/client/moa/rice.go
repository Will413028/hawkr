package moa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

type RiceSeriesSpec struct {
	Field        string
	Name         string
	PriceLayer   string
	Unit         string
	CategoryIDs  []string
	SourceTerms  []string
	BuyerAliases []string
}

var riceL3 = []string{"2022"}

var RiceSpecs = []RiceSeriesSpec{
	{Field: "pt_1japt", Name: "白米零售價（稉種）", SourceTerms: []string{"稉種"}, BuyerAliases: []string{"糯米"}, PriceLayer: string(domain.PriceLayerRetail), Unit: domain.UnitKilogram, CategoryIDs: riceL3},
	{Field: "pt_1tsait", Name: "白米零售價（硬秈）", SourceTerms: []string{"硬秈"}, PriceLayer: string(domain.PriceLayerRetail), Unit: domain.UnitKilogram},
	{Field: "pt_1sangt", Name: "白米零售價（軟秈）", SourceTerms: []string{"軟秈"}, PriceLayer: string(domain.PriceLayerRetail), Unit: domain.UnitKilogram},
	{Field: "pt_1glutrt", Name: "白米零售價（圓糯）", SourceTerms: []string{"圓糯"}, PriceLayer: string(domain.PriceLayerRetail), Unit: domain.UnitKilogram},
	{Field: "pt_1glutlt", Name: "白米零售價（長糯）", SourceTerms: []string{"長糯"}, PriceLayer: string(domain.PriceLayerRetail), Unit: domain.UnitKilogram},

	{Field: "pt_2japt", Name: "白米躉售價（稉種）", SourceTerms: []string{"稉種"}, PriceLayer: string(domain.PriceLayerWholesaleBulk), Unit: domain.UnitHectokilogram},
	{Field: "pt_2tsait", Name: "白米躉售價（硬秈）", SourceTerms: []string{"硬秈"}, PriceLayer: string(domain.PriceLayerWholesaleBulk), Unit: domain.UnitHectokilogram},
	{Field: "pt_2sangt", Name: "白米躉售價（軟秈）", SourceTerms: []string{"軟秈"}, PriceLayer: string(domain.PriceLayerWholesaleBulk), Unit: domain.UnitHectokilogram},
	{Field: "pt_2glutrt", Name: "白米躉售價（圓糯）", SourceTerms: []string{"圓糯"}, PriceLayer: string(domain.PriceLayerWholesaleBulk), Unit: domain.UnitHectokilogram},
	{Field: "pt_2glutlt", Name: "白米躉售價（長糯）", SourceTerms: []string{"長糯"}, PriceLayer: string(domain.PriceLayerWholesaleBulk), Unit: domain.UnitHectokilogram},
}

type RiceRow struct {
	TradeDate time.Time
	CityName  string
	Prices    map[string]float64
}

const riceMaxRows = 9999

func (c *Client) FetchRice(ctx context.Context, top, skip int) ([]RiceRow, error) {
	if top <= 0 || top > riceMaxRows {
		return nil, fmt.Errorf("糧價 $top 需介於 1~%d，得到 %d", riceMaxRows, top)
	}
	if skip < 0 {
		return nil, fmt.Errorf("糧價 $skip 不可為負，得到 %d", skip)
	}
	params := url.Values{}
	params.Set("$top", fmt.Sprintf("%d", top))
	if skip > 0 {
		params.Set("$skip", fmt.Sprintf("%d", skip))
	}

	reqURL := fmt.Sprintf("%s/RicepriceData.aspx?%s", c.openDataBaseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("農業部糧價呼叫失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("農業部糧價回傳 %d", resp.StatusCode)
	}

	var records []map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&records); err != nil {
		return nil, fmt.Errorf("解析農業部糧價回應失敗（上游對錯誤路徑回的是 HTML 錯誤頁而非 404）: %w", err)
	}

	out := make([]RiceRow, 0, len(records))
	for i, rec := range records {
		rawDate, _ := rec["pt_date_day"].(string)
		day, derr := ParseROCDate(strings.TrimSpace(rawDate))
		if derr != nil {
			return nil, fmt.Errorf("糧價第 %d 列日期無法解析: %q: %w", i, rawDate, derr)
		}
		rawCity, _ := rec["name"].(string)
		city := strings.TrimSpace(rawCity)
		if city == "" {
			return nil, fmt.Errorf("糧價第 %d 列沒有縣市名（%s）", i, day.Format("2006-01-02"))
		}
		row := RiceRow{TradeDate: day, CityName: city, Prices: map[string]float64{}}
		for _, spec := range RiceSpecs {
			if v, ok := ParseRicePrice(rec[spec.Field]); ok {
				row.Prices[spec.Field] = v
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func ParseRicePrice(raw any) (float64, bool) {
	v, ok := raw.(float64)
	if !ok || v <= 0 {
		return 0, false
	}
	return v, true
}
