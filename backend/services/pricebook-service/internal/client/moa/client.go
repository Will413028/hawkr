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
)

const (
	closedMarketCropName = "休市"
	closedMarketCropCode = "-"
)

type TradeRecord struct {
	TransDate     string  `json:"TransDate"`
	TcType        string  `json:"TcType"`
	CropCode      string  `json:"CropCode"`
	CropName      string  `json:"CropName"`
	MarketCode    string  `json:"MarketCode"`
	MarketName    string  `json:"MarketName"`
	UpperPrice    float64 `json:"Upper_Price"`
	MiddlePrice   float64 `json:"Middle_Price"`
	LowerPrice    float64 `json:"Lower_Price"`
	AvgPrice      float64 `json:"Avg_Price"`
	TransQuantity float64 `json:"Trans_Quantity"`
}

const maxRowsPerRequest = 1000

const maxResponseBodyBytes = 8 << 20

var ErrResponseTruncated = fmt.Errorf("MOA 回應觸及 %d 列上限，查詢範圍需再切小", maxRowsPerRequest)

type Client struct {
	baseURL         string
	openDataBaseURL string
	http            *http.Client
}

func NewClient(baseURL, openDataBaseURL string) *Client {
	return &Client{
		baseURL:         strings.TrimSuffix(baseURL, "/"),
		openDataBaseURL: strings.TrimSuffix(openDataBaseURL, "/"),
		http:            &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) FetchRange(ctx context.Context, start, end time.Time, cropCode, marketCode string) ([]TradeRecord, error) {
	params := url.Values{}
	params.Set("Start_time", ToROCDate(start))
	params.Set("End_time", ToROCDate(end))
	if cropCode != "" {
		params.Set("CropCode", cropCode)
	}
	if marketCode != "" {
		params.Set("MarketCode", marketCode)
	}

	reqURL := fmt.Sprintf("%s/AgriProductsTransType/?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("農業部 API 呼叫失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("農業部 API 回傳 %d", resp.StatusCode)
	}

	var payload struct {
		Data []TradeRecord `json:"Data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析農業部 API 回應失敗: %w", err)
	}
	if len(payload.Data) >= maxRowsPerRequest {
		return nil, ErrResponseTruncated
	}
	return payload.Data, nil
}
