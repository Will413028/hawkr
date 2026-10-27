package moa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RestDay struct {
	MarketCode string
	MarketName string
	MarketType string
	Date       time.Time
}

type restDayPayload struct {
	Data []struct {
		MarkerNo       string `json:"MarkerNo"`
		MarkerName     string `json:"MarkerName"`
		MarketTypeList []struct {
			MarketType string `json:"MarketType"`
			YearList   []struct {
				Year      int `json:"Year"`
				MonthList []struct {
					Month int    `json:"Month"`
					Rest  string `json:"Rest"`
				} `json:"MonthList"`
			} `json:"YearList"`
		} `json:"MarketTypeList"`
	} `json:"Data"`
}

func (c *Client) FetchRestDays(ctx context.Context) ([]RestDay, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/MarketRestDayFarmWCF/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("休市日曆 API 呼叫失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("休市日曆 API 回傳 %d", resp.StatusCode)
	}

	var payload restDayPayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析休市日曆失敗: %w", err)
	}

	var days []RestDay
	seen := make(map[string]struct{})
	for _, market := range payload.Data {
		marketCode := strings.TrimSpace(market.MarkerNo)
		if marketCode == "" {

			continue
		}
		for _, mt := range market.MarketTypeList {
			marketType := strings.TrimSpace(mt.MarketType)
			if marketType == "" {

				continue
			}
			for _, yr := range mt.YearList {
				if yr.Year < 1 {

					continue
				}
				for _, mo := range yr.MonthList {
					for _, token := range strings.Split(mo.Rest, "、") {
						token = strings.TrimSpace(token)
						if token == "" {
							continue
						}
						day, err := strconv.Atoi(token)
						if err != nil {

							continue
						}
						d := time.Date(yr.Year+1911, time.Month(mo.Month), day, 0, 0, 0, 0, time.UTC)

						if d.Day() != day || int(d.Month()) != mo.Month {
							continue
						}

						key := marketCode + "\x00" + marketType + "\x00" + d.Format("2006-01-02")
						if _, dup := seen[key]; dup {
							continue
						}
						seen[key] = struct{}{}
						days = append(days, RestDay{
							MarketCode: marketCode,
							MarketName: market.MarkerName,
							MarketType: marketType,
							Date:       d,
						})
					}
				}
			}
		}
	}
	return days, nil
}
