package moa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PoultrySeriesSpec struct {
	Field       string
	Name        string
	PriceLayer  string
	CategoryIDs []string
	// SourceTerms 是行情來源對這個品項的短稱（白肉雞、雞蛋…），ingest 時登記成
	// priceable_surface_form 的 SOURCE_TERM，讓買家用這些字就能精確命中。
	SourceTerms []string
}

var PoultryChickenEggSpecs = []PoultrySeriesSpec{
	{Field: "TaijinPrice_2.0kgup", Name: "白肉雞（2.0 公斤以上）產地價", SourceTerms: []string{"白肉雞"}, PriceLayer: "farmgate", CategoryIDs: chickenL3},

	{Field: "TaijinPrice_1.75kg_1.95kg", Name: "白肉雞（1.75–1.95 公斤）產地價", SourceTerms: []string{"白肉雞"}, PriceLayer: "farmgate"},

	{Field: "Store_KP_TaijinPrice", Name: "白肉雞門市價（高屏）", SourceTerms: []string{"白肉雞"}, PriceLayer: "retail"},
	{Field: "egg_Producer_Price", Name: "雞蛋產地價", SourceTerms: []string{"雞蛋"}, PriceLayer: "farmgate", CategoryIDs: []string{"1999"}},

	{Field: "egg_Price", Name: "雞蛋大運輸價", SourceTerms: []string{"雞蛋"}, PriceLayer: "wholesale_bulk"},
}

var chickenL3 = []string{"2011", "2258"}

var PoultryGooseDuckSpecs = []PoultrySeriesSpec{
	{Field: "Goose_WR_TaijinPrice", Name: "肉鵝（白羅曼）產地價", SourceTerms: []string{"肉鵝", "白羅曼"}, PriceLayer: "farmgate", CategoryIDs: []string{"2013"}},

	{Field: "Duck_M_TaijinPrice", Name: "正番鴨（公）產地價", SourceTerms: []string{"正番鴨"}, PriceLayer: "farmgate"},
	{Field: "Duck_75D_TaijinPrice", Name: "土番鴨（75 天）產地價", SourceTerms: []string{"土番鴨"}, PriceLayer: "farmgate", CategoryIDs: []string{"2012"}},
	{Field: "Duckegg_TNN_TaijinPrice", Name: "鴨蛋（新蛋・台南）產地價", SourceTerms: []string{"鴨蛋"}, PriceLayer: "farmgate", CategoryIDs: []string{"2000"}},
}

type PoultryEndpoint struct {
	Path  string
	Specs []PoultrySeriesSpec
}

var (
	PoultryChickenEgg = PoultryEndpoint{Path: "PoultryTransType_BoiledChicken_Eggs", Specs: PoultryChickenEggSpecs}
	PoultryGooseDuck  = PoultryEndpoint{Path: "PoultryTransType_Goose_Duck_Duckegg", Specs: PoultryGooseDuckSpecs}
)

type PoultryRow struct {
	TradeDate time.Time
	Prices    map[string]float64
}

func ParsePoultryPrice(raw any) (float64, bool) {
	s, ok := raw.(string)
	if !ok {
		return 0, false
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "休市" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func (c *Client) FetchPoultry(ctx context.Context, ep PoultryEndpoint, start, end time.Time) ([]PoultryRow, error) {
	params := url.Values{}
	params.Set("Start_time", start.Format("2006/01/02"))
	params.Set("End_time", end.Format("2006/01/02"))

	reqURL := fmt.Sprintf("%s/%s/?%s", c.baseURL, ep.Path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("農業部家禽行情呼叫失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("農業部家禽行情回傳 %d", resp.StatusCode)
	}

	var payload struct {
		Data []map[string]any `json:"Data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析農業部家禽行情回應失敗: %w", err)
	}

	out := make([]PoultryRow, 0, len(payload.Data))
	for i, rec := range payload.Data {
		rawDate, _ := rec["TransDate"].(string)
		day, derr := time.Parse("2006/01/02", strings.TrimSpace(rawDate))
		if derr != nil {
			return nil, fmt.Errorf("家禽行情第 %d 列日期無法解析: %q", i, rawDate)
		}
		if day.Before(start) || day.After(end) {
			return nil, fmt.Errorf(
				"農業部家禽行情回傳日期超出請求區間：請求 %s~%s，第 %d 列為 %s（上游對錯誤參數靜默忽略 filter，不可採信）",
				start.Format("2006/01/02"), end.Format("2006/01/02"), i, day.Format("2006/01/02"))
		}
		row := PoultryRow{TradeDate: day, Prices: map[string]float64{}}
		for _, spec := range ep.Specs {
			if v, ok := ParsePoultryPrice(rec[spec.Field]); ok {
				row.Prices[spec.Field] = v
			}
		}
		out = append(out, row)
	}
	return out, nil
}
