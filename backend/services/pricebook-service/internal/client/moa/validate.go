package moa

import "strings"

type RejectReason string

const (
	RejectClosedMarket RejectReason = "closed_market"

	RejectNonPositivePrice RejectReason = "non_positive_price"

	RejectMissingCropCode   RejectReason = "missing_crop_code"
	RejectMissingCropName   RejectReason = "missing_crop_name"
	RejectMissingMarketCode RejectReason = "missing_market_code"
	RejectMissingMarketName RejectReason = "missing_market_name"
)

func (r RejectReason) IsDataDefect() bool {
	switch r {
	case RejectMissingCropCode, RejectMissingCropName,
		RejectMissingMarketCode, RejectMissingMarketName:
		return true
	default:
		return false
	}
}

type ValidTradeRecord struct{ rec TradeRecord }

func (v ValidTradeRecord) Record() TradeRecord { return v.rec }

func (r TradeRecord) Validate() (ValidTradeRecord, RejectReason) {
	r.TransDate = strings.TrimSpace(r.TransDate)
	r.CropCode = strings.TrimSpace(r.CropCode)
	r.CropName = strings.TrimSpace(r.CropName)
	r.MarketCode = strings.TrimSpace(r.MarketCode)
	r.MarketName = strings.TrimSpace(r.MarketName)

	switch {
	case r.CropName == closedMarketCropName || r.CropCode == closedMarketCropCode:
		return ValidTradeRecord{}, RejectClosedMarket
	case r.AvgPrice <= 0:
		return ValidTradeRecord{}, RejectNonPositivePrice
	case r.CropCode == "":
		return ValidTradeRecord{}, RejectMissingCropCode
	case r.CropName == "":
		return ValidTradeRecord{}, RejectMissingCropName
	case r.MarketCode == "":
		return ValidTradeRecord{}, RejectMissingMarketCode
	case r.MarketName == "":
		return ValidTradeRecord{}, RejectMissingMarketName
	}
	return ValidTradeRecord{rec: r}, ""
}
