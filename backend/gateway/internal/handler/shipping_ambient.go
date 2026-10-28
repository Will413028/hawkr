package handler

// ambientShippingTemperature 是 CodeMeta ShippingTemperature 的 AMBIENT 值；
// Java 的配送方式聚合與超商建店仍要求溫層，閘道一律補常溫，計畫 B4 刪除。
const ambientShippingTemperature = "3201"

// isAmbientOrUntyped 為真表示該列不帶溫層（宅配）或溫層是常溫；賣家端只看得到這些列。
func isAmbientOrUntyped(temperature string) bool {
	return temperature == "" || temperature == ambientShippingTemperature
}
