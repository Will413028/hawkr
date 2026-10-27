package cache

import "maps"

// ManualAliasKeys 回傳人工覆寫層的複本，供 cmd/category-alias-gen 在生成時
// 避開已被人工擁有的別名。回複本而非本尊：這張表是服務執行期在讀的資料，
// 交出可寫的參照等於把它變成可變全域狀態。
func ManualAliasKeys() map[string]string {
	return maps.Clone(synonyms)
}
