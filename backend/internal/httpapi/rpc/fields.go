package rpc

import (
	"strconv"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// FormatID 是對外的 ID：帳號、管理員、買家與店家的主鍵過渡期是整數（ADR 2026-09-25-row-identity-by-data-kind），
// 契約上一律是不透明的字串。
func FormatID(id int64) string { return strconv.FormatInt(id, 10) }

// RequiredField 是缺少必填請求欄位 path 的錯誤（INVALID_ARGUMENT，指向該欄位）。
func RequiredField(path, message string) error {
	return WithFieldPaths(apperr.InvalidInputOn([]string{path}, "%s", message), FieldPaths{path: path})
}

// ParseID 解析請求裡的整數 ID（過渡期的店家、帳號、買家與商品 ID 是十進位整數字串）；path 是請求的欄位路徑。格式不對
// （不是整數、不是正數）是呼叫方的錯：INVALID_ARGUMENT，指向該欄位。對外的契約仍把 ID 當不透明的字串，轉換只在這裡。
func ParseID(path, raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, RequiredField(path, path+" 必須是有效的 ID")
	}
	return id, nil
}

// ParseUUID 解析請求裡的 UUID 字串 ID；path 是請求的欄位路徑，格式不對是 INVALID_ARGUMENT，指向該欄位。
func ParseUUID(path, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, RequiredField(path, path+" 必須是有效的 ID")
	}
	return id, nil
}
