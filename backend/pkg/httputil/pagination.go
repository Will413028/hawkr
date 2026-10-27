package httputil

import "strconv"

// ParsePageParam 解析分頁參數字串為整數。
// 空字串、非法格式、小於 1 皆回傳 defaultVal。
func ParsePageParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}
