package rpc

import (
	"strconv"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Etag 是資源版本的對外形式；client 將它視為不透明字串。
func Etag(version int64) string { return strconv.FormatInt(version, 10) }

// VersionFromEtag 解析選填的 etag；格式不符時回版本衝突。
func VersionFromEtag(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, apperr.VersionConflict("etag 與目前的版本不符，請重新讀取後再送出")
	}
	return v, nil
}

// RequiredVersion 解析修改操作必填的 etag。
func RequiredVersion(s string) (int64, error) {
	if s == "" {
		return 0, Error(connect.CodeInvalidArgument, "%s", "修改必須帶讀取時拿到的 etag")
	}
	return VersionFromEtag(s)
}

// RequiredVersionField 將缺少 etag 的錯誤指向指定的請求欄位。
func RequiredVersionField(s, path string) (int64, error) {
	if s == "" {
		return 0, WithFieldPaths(apperr.InvalidInputOn([]string{path}, "必須提供 etag"), FieldPaths{path: path})
	}
	return VersionFromEtag(s)
}
