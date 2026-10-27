// Package pagination 統一正規化清單每頁筆數。
package pagination

import "github.com/Will413028/hawkr/backend/internal/platform/apperr"

// PageSize 套用 Google AIP-158：零使用預設值，超過上限時降至上限，負數是無效輸入。
func PageSize(size, def, max int32) (int32, error) {
	switch {
	case size < 0:
		return 0, apperr.InvalidInput("筆數不可為負數")
	case size == 0:
		return def, nil
	case size > max:
		return max, nil
	}
	return size, nil
}
