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

func ToROCDateCompact(t time.Time) string {
	return fmt.Sprintf("%d%02d%02d", t.Year()-1911, t.Month(), t.Day())
}

func ParseROCDateCompact(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) != 7 {
		return time.Time{}, fmt.Errorf("民國年緊湊格式長度應為 7: %q", s)
	}
	year, err := strconv.Atoi(s[0:3])
	if err != nil || year < 1 {
		return time.Time{}, fmt.Errorf("民國年年份無效: %q", s)
	}
	month, err := strconv.Atoi(s[3:5])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("民國年月份無效: %q", s)
	}
	day, err := strconv.Atoi(s[5:7])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("民國年日期無效: %q", s)
	}
	t := time.Date(year+1911, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year+1911 || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, fmt.Errorf("民國年日期在行事曆上不存在: %q", s)
	}
	return t, nil
}

type PorkRecord struct {
	TransDate  string  `json:"TransDate"`
	MarketName string  `json:"MarketName"`
	SpecPigNum float64 `json:"SpecPig_Num"`
	SpecPigWgt float64 `json:"SpecPig_AvgWgt"`
	SpecPigAvg float64 `json:"SpecPig_AvgPrice"`
}

func (r PorkRecord) HasTrade() bool { return r.SpecPigNum > 0 && r.SpecPigAvg > 0 }

func (c *Client) FetchPork(ctx context.Context, date time.Time) ([]PorkRecord, error) {
	want := ToROCDateCompact(date)
	params := url.Values{}
	params.Set("TransDate", want)

	reqURL := fmt.Sprintf("%s/PorkTransType/?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("農業部毛豬行情呼叫失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("農業部毛豬行情回傳 %d", resp.StatusCode)
	}

	var payload struct {
		Data []PorkRecord `json:"Data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析農業部毛豬行情回應失敗: %w", err)
	}

	for i, r := range payload.Data {
		got := strings.TrimSpace(r.TransDate)
		if got != want {
			return nil, fmt.Errorf(
				"農業部毛豬行情回傳日期不符：請求 %s，第 %d 列為 %q（上游對錯誤參數靜默失敗，不可採信）",
				want, i, got)
		}
	}
	return payload.Data, nil
}
