package seller

import (
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)


func kycAdminEtag(version, checkSeq int64) string {
	return strconv.FormatInt(version, 10) + "." + strconv.FormatInt(checkSeq, 10)
}

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
