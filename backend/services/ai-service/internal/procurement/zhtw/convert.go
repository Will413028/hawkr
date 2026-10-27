// Package zhtw 把中文文字正規化成繁體（臺灣正體）。
//
// 存在理由：procurement 模組的 LLM 輸出（slot 抽取、對話回覆／推理過程）語域是
// 機率性的，即使 prompt 全程要求繁體中文，底層模型（尤其中國廠商模型，例如
// EXTRACTOR_MODEL=deepseek-flash）仍有機率吐出簡體字。2026-08-15 買家實測踩到：
// 說「高麗菜」，LLM 抽取結果變成「高丽菜」，跟 pricebook／product 目錄的繁體字
// 比對失敗，候選搜尋直接 0 家——這不是候選資料的問題，是文字正規化缺口。
// 用程式做確定性轉換，不依賴 prompt 工程去賭模型這次會不會聽話。
package zhtw

import "github.com/longbridgeapp/opencc"

// converter 用 s2twp（簡體→臺灣正體，含常用詞彙轉換，不只是字形）。字典透過
// Go 的 embed 特性打包進二進位檔，New() 不吃網路／檔案系統，正常情況下不會
// 失敗——失敗代表函式庫本身或內嵌字典壞了，這是建置期就該炸出來的錯誤，不是可以在
// 執行期優雅降級的情境，所以用 init()+panic 讓它在服務啟動當下就顯性失敗。
var converter = mustNewConverter()

func mustNewConverter() *opencc.OpenCC {
	c, err := opencc.New("s2twp")
	if err != nil {
		panic("zhtw: failed to initialize s2twp converter: " + err.Error())
	}
	return c
}

// ToTraditional 把 s 轉成繁體中文；非中文字元或已經是繁體的部分原樣保留。
// 轉換失敗（目前 opencc 的字典查表實作不會失敗，但介面保留 error）時回傳原文，
// 不讓正規化這一步反過來讓抽取結果消失。
func ToTraditional(s string) string {
	out, err := converter.Convert(s)
	if err != nil {
		return s
	}
	return out
}
