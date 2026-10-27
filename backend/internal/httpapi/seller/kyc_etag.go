package seller

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 後台 KYC 檢視的 etag：申請的版本加顯示的登記資料查詢的序號（沒有查詢是 0），格式是 "<版本>.<序號>"，對 client 仍是不透明的字串。
// 顯示的查詢變了（新的已完成結果）而 etag 是舊的，核准與退回回 aborted；賣家端的 etag 仍是單純的版本（Etag）。

// kycAdminEtag 組後台 KYC 檢視的 etag。
func kycAdminEtag(version, checkSeq int64) string {
	return strconv.FormatInt(version, 10) + "." + strconv.FormatInt(checkSeq, 10)
}

// parseKYCAdminEtag 解析 kycAdminEtag；格式不符（不是兩個以 . 分隔的整數、版本不是正數、序號是負數）回 invalid_argument，
// 指向 path 欄位（與版本不符的 aborted 不同：格式錯是 client 的錯，不是資料變了）。
func parseKYCAdminEtag(s, path string) (version, checkSeq int64, err error) {
	if s == "" {
		return 0, 0, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{path}, "必須提供 etag"), rpc.FieldPaths{path: path})
	}
	v, c, ok := strings.Cut(s, ".")
	if ok {
		version, err = strconv.ParseInt(v, 10, 64)
		if err == nil {
			checkSeq, err = strconv.ParseInt(c, 10, 64)
		}
	}
	if !ok || err != nil || version <= 0 || checkSeq < 0 || v != strconv.FormatInt(version, 10) || c != strconv.FormatInt(checkSeq, 10) {
		return 0, 0, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{path}, "etag 的格式不正確，請重新讀取後再送出"), rpc.FieldPaths{path: path})
	}
	return version, checkSeq, nil
}
