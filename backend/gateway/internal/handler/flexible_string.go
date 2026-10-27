package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexibleString 可以接受 JSON 中的字串或數字，並統一轉為字串
// 用於向後兼容：支援前端傳遞數字格式或字串格式的金額
type FlexibleString string

// UnmarshalJSON 實現自定義 JSON 解析
// 接受以下格式：
//   - 字串: "1000" -> "1000" ✅ (符合規範)
//   - 數字: 1000 -> "1000" ✅ (向後兼容)
//   - 浮點數: 1000.0 -> "1000" ✅ (自動轉整數)
func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	// 嘗試解析為字串
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*fs = FlexibleString(s)
		return nil
	}

	// 嘗試解析為浮點數（可以涵蓋整數）
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		// 轉為整數字串（金額不應有小數）
		*fs = FlexibleString(strconv.FormatInt(int64(f), 10))
		return nil
	}

	return fmt.Errorf("amount must be a string or number, got: %s", string(data))
}

// String 轉為 Go 字串
func (fs FlexibleString) String() string {
	return string(fs)
}

// MarshalJSON 實現 JSON 序列化，輸出為字串格式
func (fs FlexibleString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(fs))
}
