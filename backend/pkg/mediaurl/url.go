// Package mediaurl 是後端 Go 服務組裝媒體資產顯示 URL 的唯一入口（49 號設計書 §3/§4.3）。
// 域名唯一來源為 MEDIA_BASE_URL；DB/MQ/快取只存 objectKey，顯示時才由此組裝，禁止 ad-hoc 拼接。
package mediaurl

import (
	"fmt"
	"strings"
)

// BuildURL 將 objectKey 組裝為原圖基底 URL（Image Gateway 契約 baseURL + "/i/original/" + objectKey，
// 49 號 §6.1/§6.5：imageUrl＝原圖基底 URL，走 Worker `/i/original/{objectKey}` 路徑）。
// 規則：objectKey 為空回傳空字串；已是 http/https 絕對 URL（如 Google 頭像）直接回傳；
// baseURL 未設（空字串）時回傳裸 objectKey。
//
// 媒體域名不做新舊相容（backend#2283）：dev 一律 test-media.example.com、prod 一律
// media.example.com，各服務 config 不填任何預設域名，Go 側缺值即 fail-fast
// （Config.Validate()）。因此正常執行期 baseURL 恆為非空，「回裸 objectKey」只在單元測試
// 等未注入環境變數的情境成立。Java 側 MediaUrlProperties.buildUrl 同語意。
func BuildURL(baseURL, objectKey string) string {
	return BuildImageURL(baseURL, "original", objectKey)
}

// BuildImageURL 組裝 Image Gateway preset URL（baseURL + "/i/" + preset + "/" + objectKey，
// 對應 49 號 §4.4 preset 白名單）。切換至 Image Gateway（MEDIA_BASE_URL 指向 Worker 域名）後生效。
// 空值與 http 前綴處理同 BuildURL。
func BuildImageURL(baseURL, preset, objectKey string) string {
	if objectKey == "" {
		return ""
	}
	if strings.HasPrefix(objectKey, "http") {
		return objectKey
	}
	if baseURL == "" {
		return objectKey
	}
	return strings.TrimRight(baseURL, "/") + "/i/" + preset + "/" + objectKey
}

// ValidateObjectKey 驗證前端可控輸入的 objectKey（49 號設計書 §8.1/§25）。
// 只允許存入純 objectKey，拒絕：空字串、完整 URL（http/https 開頭）、含 ".." 的路徑穿越。
// 傳入 allowedPrefixes 時，key 必須符合其中任一前綴（前綴白名單）；不傳則只做前三項基本檢查。
// 用途：寫入路徑（如 ai_styles.thumbnail_key）在存 DB 前呼叫，堵住「前端直送 URL 回存」的洞。
func ValidateObjectKey(key string, allowedPrefixes ...string) error {
	if key == "" {
		return fmt.Errorf("objectKey 不可為空")
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return fmt.Errorf("objectKey 不可為完整 URL: %s", key)
	}
	if strings.Contains(key, "..") {
		return fmt.Errorf("objectKey 不可含 .. : %s", key)
	}
	if len(allowedPrefixes) > 0 {
		for _, p := range allowedPrefixes {
			if strings.HasPrefix(key, p) {
				return nil
			}
		}
		return fmt.Errorf("objectKey 前綴不在白名單: %s", key)
	}
	return nil
}

// NormalizeToObjectKey 把「前端可控輸入」的單一圖片欄位值收斂為裸 objectKey（49 號設計書 §25）。
// 用於輸入面（寫入路徑）容錯：前端在遷移期可能回送 GET 回應的顯示 URL（完整 URL），
// 後端必須先剝殼還原成 objectKey 再落庫，禁止把完整 URL 存進 *_key 欄位。
//
// 規則：
//   - 空字串 → 回傳空字串（代表「未提供」，交由呼叫端的必填/staged token 邏輯處理）；
//   - http/https 開頭：逐一比對 knownBases，命中則剝除該域名前綴還原 key，再走 ValidateObjectKey；
//     若不屬任何已知媒體域名 → 視為外來域名，回錯（禁止回存）；
//   - 其餘（裸 objectKey）→ 直接走 ValidateObjectKey。
//
// allowedPrefixes 沿用 ValidateObjectKey 語意（非空時 key 必須符合其中一個前綴）。
func NormalizeToObjectKey(value string, knownBases []string, allowedPrefixes []string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		for _, base := range knownBases {
			if base == "" {
				continue
			}
			prefix := strings.TrimRight(base, "/") + "/"
			if strings.HasPrefix(value, prefix) {
				// 剝除域名前綴後，還原可能含 Image Gateway 路徑段（/i/{preset}/），一併剝掉還原裸 key
				key := stripImageGatewayPath(strings.TrimPrefix(value, prefix))
				if err := ValidateObjectKey(key, allowedPrefixes...); err != nil {
					return "", err
				}
				return key, nil
			}
		}
		return "", fmt.Errorf("圖片 URL 非本平台媒體域名，禁止回存: %s", value)
	}
	if err := ValidateObjectKey(value, allowedPrefixes...); err != nil {
		return "", err
	}
	return value, nil
}

// StripPrefix 移除 baseURL 前綴（含 Image Gateway 路徑段 /i/{preset}/），確保只保留 objectKey。
// 寫入 DB 前應呼叫，避免存入完整 URL；顯示 URL 切換為 Worker `/i/{preset}/{key}` 契約後，
// 剝殼必須連 preset 段一併還原成裸 key。
func StripPrefix(baseURL, value string) string {
	if value == "" {
		return ""
	}
	prefix := strings.TrimRight(baseURL, "/") + "/"
	if !strings.HasPrefix(value, prefix) {
		return value
	}
	return stripImageGatewayPath(strings.TrimPrefix(value, prefix))
}

// stripImageGatewayPath 把「剝除域名後」的殘值還原成裸 objectKey：
// 若殘值是 Image Gateway 路徑（i/{preset}/{objectKey}），剝掉前兩段（i 與 preset）回傳 objectKey；
// 否則（已是裸 key，如舊 CDN 直拼格式）原樣回傳。
// objectKey 前綴恆為 public/、secure/、tmp/、defaults/ 等，絕不以 "i/" 開頭，故判斷無歧義。
func stripImageGatewayPath(rest string) string {
	const gwPrefix = "i/"
	if !strings.HasPrefix(rest, gwPrefix) {
		return rest
	}
	afterI := rest[len(gwPrefix):] // {preset}/{objectKey}
	if idx := strings.IndexByte(afterI, '/'); idx >= 0 {
		return afterI[idx+1:]
	}
	return rest
}
