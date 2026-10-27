package importraw

import (
	"testing"
)

func TestToTradeRecordConvertsLegacyISODateToROC(t *testing.T) {
	r := rawRecord{
		TransDate: "2022-08-12", TcType: "N05", CropCode: "11", CropName: "椰子",
		MarketCode: "104", MarketName: "台北二",
		UpperPrice: 48.0, MiddlePrice: 23.1, LowerPrice: 17.4, AvgPrice: 27.0, TransQuantity: 1230.0,
	}
	tr, err := r.toTradeRecord()
	if err != nil {
		t.Fatalf("toTradeRecord 失敗: %v", err)
	}
	if tr.TransDate != "111.08.12" {
		t.Errorf("TransDate = %q，預期民國格式 %q", tr.TransDate, "111.08.12")
	}
	if tr.CropCode != "11" || tr.MarketCode != "104" || tr.AvgPrice != 27.0 {
		t.Errorf("其餘欄位應原樣帶過，實得 %+v", tr)
	}
}

func TestToTradeRecordPassesThroughBronzeROCDate(t *testing.T) {
	r := rawRecord{
		TransDate: "114.11.20", TcType: "N05", CropCode: "11", CropName: "椰子",
		MarketCode: "104", MarketName: "台北二",
		UpperPrice: 19.0, MiddlePrice: 14.2, LowerPrice: 13.0, AvgPrice: 14.9, TransQuantity: 940.0,
	}
	tr, err := r.toTradeRecord()
	if err != nil {
		t.Fatalf("toTradeRecord 失敗: %v", err)
	}
	if tr.TransDate != "114.11.20" {
		t.Errorf("bronze 來源的民國日期應原樣帶過，實得 %q", tr.TransDate)
	}
}

func TestToTradeRecordReturnsErrorOnInvalidLegacyDate(t *testing.T) {
	r := rawRecord{TransDate: "not-a-date"}
	if _, err := r.toTradeRecord(); err == nil {
		t.Fatal("非法的 legacy 日期應回傳錯誤")
	}
}

// TestToTradeRecordAcceptsROCDateRegardlessOfSource 驗證本次修復的 Critical
// bug：legacy 封存並非全部都是西元格式，實測 date=2022-09-30 起「交易日期」
// 已經是民國格式（例如 "111.09.30"）——格式判斷必須看內容本身，不能靠呼叫端
// 是哪個來源目錄傳來的。
func TestToTradeRecordAcceptsROCDateRegardlessOfSource(t *testing.T) {
	r := rawRecord{
		TransDate: "111.09.30", TcType: "N05", CropCode: "11", CropName: "椰子",
		MarketCode: "104", MarketName: "台北二",
		UpperPrice: 20.0, MiddlePrice: 15.0, LowerPrice: 10.0, AvgPrice: 15.0, TransQuantity: 100.0,
	}
	tr, err := r.toTradeRecord()
	if err != nil {
		t.Fatalf("toTradeRecord 失敗: %v", err)
	}
	if tr.TransDate != "111.09.30" {
		t.Errorf("民國格式應原樣帶過，實得 %q", tr.TransDate)
	}
}
