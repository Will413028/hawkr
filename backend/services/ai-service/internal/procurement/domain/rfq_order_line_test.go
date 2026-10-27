package domain

import (
	"errors"
	"testing"
)

// 這支守的是「品名不得退化成 sentinel」（doc 64 §6.4.3）：退款、售後、發票三條下游都吃
// order_items.sku_name，而 RFQ 明細沒有商品主檔可以反查。
//
// M1 變異：把缺值時的 error 改成回「RFQ 成交品項」之類的預設字串 ⇒ 下面三支
// rejects* 一起轉紅。
func TestComposeRfqOrderLineName(t *testing.T) {
	payload := []byte(`{"item":"文旦柚","itemResolution":{"kind":"CROP","code":"A123","label":"麻豆文旦"}}`)

	t.Run("品名取解析後的標籤，數量與單位取被接受的那一版報價", func(t *testing.T) {
		got, err := ComposeRfqOrderLineName(payload, "200", "公斤")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// ⚠️ 用 itemLabel 而不是買家原話 item：它與賣家報價畫面上看到的是同一個字串。
		if got != "麻豆文旦 200 公斤" {
			t.Errorf("got %q, want %q", got, "麻豆文旦 200 公斤")
		}
	})

	t.Run("沒有解析標籤時退回買家原話，仍是真品名", func(t *testing.T) {
		got, err := ComposeRfqOrderLineName([]byte(`{"item":"文旦柚"}`), "12.5", "箱")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 小數數量正是 order_items.quantity（INT）表達不出來、必須寫進品名的那一種。
		if got != "文旦柚 12.5 箱" {
			t.Errorf("got %q, want %q", got, "文旦柚 12.5 箱")
		}
	})

	t.Run("rejects 缺品名", func(t *testing.T) {
		if _, err := ComposeRfqOrderLineName([]byte(`{}`), "200", "公斤"); !errors.Is(err, ErrRfqOrderLineNameUnavailable) {
			t.Errorf("err = %v, want ErrRfqOrderLineNameUnavailable", err)
		}
	})

	t.Run("rejects 缺數量", func(t *testing.T) {
		if _, err := ComposeRfqOrderLineName(payload, "  ", "公斤"); !errors.Is(err, ErrRfqOrderLineNameUnavailable) {
			t.Errorf("err = %v, want ErrRfqOrderLineNameUnavailable", err)
		}
	})

	t.Run("rejects 缺單位", func(t *testing.T) {
		if _, err := ComposeRfqOrderLineName(payload, "200", ""); !errors.Is(err, ErrRfqOrderLineNameUnavailable) {
			t.Errorf("err = %v, want ErrRfqOrderLineNameUnavailable", err)
		}
	})

	t.Run("rejects 壞掉的 canonical payload", func(t *testing.T) {
		if _, err := ComposeRfqOrderLineName([]byte(`not json`), "200", "公斤"); !errors.Is(err, ErrRfqOrderLineNameUnavailable) {
			t.Errorf("err = %v, want ErrRfqOrderLineNameUnavailable", err)
		}
	})
}
