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

// encodeOnce 把 payload 序列化成**唯一一份** JSON 字串。
//
// 🔴 這個回傳值必須同時用於 (a) 算 sign 與 (b) 放進 form 的 data 欄位，
// 呼叫之後**絕對不可再 marshal 一次 payload**。
//
// AMEGO 不做任何 canonicalization——它拿「你實際送來的 data 字串」原樣算 md5
// （官方 Python 與 Java 兩份範例排版完全不同、兩個 sign 都被官方認可，這就是證明）。
// 任何「先 marshal 算 sign、再 marshal 一次送出」的寫法都會 100% 回 code 16，
// 而錯誤訊息完全不提示原因。
//
// SetEscapeHTML(false)：encoding/json 預設把 < > & 轉成 < 這類轉義，
// 而 json.Marshal 與 Encoder 的預設不同 ⇒ 兩條路徑產出的位元組會在含這些字元的
// 業務資料上分歧。關掉它讓行為單一且可預測。
func encodeOnce(payload any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return "", fmt.Errorf("amego: encode data: %w", err)
	}
	// Encoder.Encode 固定在尾端補一個換行，那個換行不屬於 data。
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// signature 是 md5(data + time + AppKey)，三段直接串接、無分隔符、UTF-8、小寫 hex。
// 大寫 hex 會被拒（code 16）。
func signature(dataJSON string, unixSec int64, appKey string) string {
	h := md5.New() //nolint:gosec // 同上：演算法由 AMEGO 規定
	h.Write([]byte(dataJSON))
	h.Write([]byte(strconv.FormatInt(unixSec, 10)))
	h.Write([]byte(appKey))
	return hex.EncodeToString(h.Sum(nil))
}

// buildForm 組出四個欄位。
//
// 用 url.Values.Encode 做**標準一次**編碼，不要自己拼字串：
// 自己拼會把 `+` 直接塞進 body 而被伺服器解成空白（碰到載具 /TRM+O+P 就壞）。
// 「請進行 url encode 動作」那句話實測就是標準的一次 form 編碼，不是編兩次。
//
// dataJSON 必須是 encodeOnce 的回傳值本身，sign 必須由同一個字串算出。
func buildForm(invoice, dataJSON string, unixSec int64, sign string) url.Values {
	return url.Values{
		"invoice": {invoice},
		"data":    {dataJSON},
		"time":    {strconv.FormatInt(unixSec, 10)},
		"sign":    {sign},
	}
}

// maskSign 只留 sign 的前 8 碼給稽核。
//
// 給定已落庫的 data 與 time，全長 sign 就是離線暴力破解 AppKey 的 oracle
// ⇒ 8 碼足以與客服對單，不足以當破解目標（§4.5.4）。
func maskSign(sign string) string {
	if len(sign) <= 8 {
		return sign
	}
	return sign[:8]
}
