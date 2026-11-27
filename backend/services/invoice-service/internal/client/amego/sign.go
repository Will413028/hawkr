package amego

import (
	"bytes"
	"crypto/md5" //nolint:gosec // AMEGO 的簽章演算法由對方規定，不是我方的安全選型
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func encodeOnce(payload any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return "", fmt.Errorf("amego: encode data: %w", err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func signature(dataJSON string, unixSec int64, appKey string) string {
	h := md5.New() //nolint:gosec // 同上：演算法由 AMEGO 規定
	h.Write([]byte(dataJSON))
	h.Write([]byte(strconv.FormatInt(unixSec, 10)))
	h.Write([]byte(appKey))
	return hex.EncodeToString(h.Sum(nil))
}

func buildForm(invoice, dataJSON string, unixSec int64, sign string) url.Values {
	return url.Values{
		"invoice": {invoice},
		"data":    {dataJSON},
		"time":    {strconv.FormatInt(unixSec, 10)},
		"sign":    {sign},
	}
}

func maskSign(sign string) string {
	if len(sign) <= 8 {
		return sign
	}
	return sign[:8]
}
