package moa

import "testing"

func TestValidateRejectsRecordsWithoutUsableIdentity(t *testing.T) {
	base := TradeRecord{
		TransDate: "115.09.15", TcType: "N04",
		CropCode: "LA1", CropName: "甘藍-初秋",
		MarketCode: "104", MarketName: "台北一",
		AvgPrice: 46.2,
	}
	mutate := func(f func(*TradeRecord)) TradeRecord {
		r := base
		f(&r)
		return r
	}

	cases := []struct {
		name   string
		record TradeRecord
		want   RejectReason
	}{
		{"缺品名", mutate(func(r *TradeRecord) { r.CropName = "" }), RejectMissingCropName},
		{"品名只有空白", mutate(func(r *TradeRecord) { r.CropName = "   " }), RejectMissingCropName},
		{"缺品項代碼", mutate(func(r *TradeRecord) { r.CropCode = "" }), RejectMissingCropCode},
		{"缺市場代碼", mutate(func(r *TradeRecord) { r.MarketCode = " " }), RejectMissingMarketCode},
		{"缺市場名稱", mutate(func(r *TradeRecord) { r.MarketName = "" }), RejectMissingMarketName},
		{"休市哨兵", mutate(func(r *TradeRecord) {
			r.CropCode, r.CropName, r.AvgPrice = "-", "休市", 0
		}), RejectClosedMarket},
		{"均價為零", mutate(func(r *TradeRecord) { r.AvgPrice = 0 }), RejectNonPositivePrice},
		{"均價為負", mutate(func(r *TradeRecord) { r.AvgPrice = -1 }), RejectNonPositivePrice},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, reason := tc.record.Validate(); reason != tc.want {
				t.Errorf("Validate() reason = %q, want %q", reason, tc.want)
			}
		})
	}
}

func TestValidateReportsClosedMarketBeforeMissingName(t *testing.T) {
	sentinel := TradeRecord{
		TransDate: "115.09.15", CropCode: "-", CropName: "休市",
		MarketCode: "104", MarketName: "台北一", AvgPrice: 0,
	}
	if _, reason := sentinel.Validate(); reason != RejectClosedMarket {
		t.Errorf("Validate() reason = %q, want %q", reason, RejectClosedMarket)
	}
}

func TestValidateAcceptsNormalRecord(t *testing.T) {
	r := TradeRecord{
		TransDate: "115.09.15", TcType: "N04",
		CropCode: "LA1", CropName: "甘藍-初秋",
		MarketCode: "104", MarketName: "台北一",
		AvgPrice: 46.2, UpperPrice: 60, MiddlePrice: 46, LowerPrice: 30,
		TransQuantity: 1234.5,
	}
	valid, reason := r.Validate()
	if reason != "" {
		t.Fatalf("Validate() reason = %q, want accepted", reason)
	}
	if valid.Record() != r {
		t.Errorf("Record() = %+v, want %+v", valid.Record(), r)
	}
}

func TestValidateTrimsIdentityFields(t *testing.T) {
	r := TradeRecord{
		TransDate: " 115.09.15 ", TcType: "N04",
		CropCode: " LA1 ", CropName: " 甘藍-初秋 ",
		MarketCode: " 104 ", MarketName: " 台北一 ",
		AvgPrice: 46.2,
	}
	valid, reason := r.Validate()
	if reason != "" {
		t.Fatalf("Validate() reason = %q, want accepted", reason)
	}
	got := valid.Record()
	for _, c := range []struct{ field, want, have string }{
		{"CropCode", "LA1", got.CropCode},
		{"CropName", "甘藍-初秋", got.CropName},
		{"MarketCode", "104", got.MarketCode},
		{"MarketName", "台北一", got.MarketName},
		{"TransDate", "115.09.15", got.TransDate},
	} {
		if c.have != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.have, c.want)
		}
	}
}

func TestZeroValidTradeRecordCarriesNoIdentity(t *testing.T) {
	var zero ValidTradeRecord
	if zero.Record().CropCode != "" || zero.Record().CropName != "" {
		t.Error("ValidTradeRecord 零值不應帶有任何身分")
	}
}

func TestOnlyIdentityDefectsAreQuarantined(t *testing.T) {
	defects := []RejectReason{
		RejectMissingCropCode, RejectMissingCropName,
		RejectMissingMarketCode, RejectMissingMarketName,
	}
	routine := []RejectReason{RejectClosedMarket, RejectNonPositivePrice}

	for _, r := range defects {
		if !r.IsDataDefect() {
			t.Errorf("%q 應被視為資料品質異常", r)
		}
	}
	for _, r := range routine {
		if r.IsDataDefect() {
			t.Errorf("%q 是正常過濾，不該進隔離區", r)
		}
	}
}
