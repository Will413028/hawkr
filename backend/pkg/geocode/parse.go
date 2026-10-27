// pkg/geocode/parse.go
package geocode

import "strings"

// extractDetail 從完整地址去掉縣市和區，取得路名門牌
func extractDetail(fullAddress, county, district string) string {
	detail := fullAddress
	if county != "" {
		if idx := strings.Index(detail, county); idx != -1 {
			detail = detail[idx+len(county):]
		}
	}
	if district != "" {
		if idx := strings.Index(detail, district); idx != -1 {
			detail = detail[idx+len(district):]
		}
	}
	return strings.TrimSpace(detail)
}
