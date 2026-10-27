package handler

import (
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/mediaurl"
)

// resolveInputObjectKey 把前端送來的一對 *_key / *_url 圖片欄位收斂為單一裸 objectKey。
//
// 背景：媒體 URL 切片後，GET 回應的 *_image_url 由裸 key 變成完整顯示 URL；admin 前端
// 尚未改送 key（Phase 6），仍存在「GET 值原樣回打 PUT」的流量。後端必須自保，
// 讓落庫的一律是裸 objectKey。
//
// 語意：
//   - keyVal 優先（新欄位，語意即裸 objectKey）；urlVal 為舊欄位，值可能是裸 key 或完整媒體域名 URL；
//   - 兩者都給且正規化後不一致 → 回錯（防歧義）；
//   - 兩者皆空 → 回空字串（交由下游 staged token / 必填邏輯處理）；
//   - 外來 http(s) 域名 → 回錯（禁止回存）。
//
// allowedPrefixes 沿用 mediaurl.ValidateObjectKey 語意（如 banner 用 "public/"）。
//
// 已知媒體來源**只有本環境設定的 MEDIA_BASE_URL 一個**（backend#2283）：媒體域名不做
// 新舊相容，dev 只認 test-media.example.com、prod 只認 media.example.com。
// 舊實作額外把退役中的 cdn.example.com 列為永久已知來源，等於在輸入面保留一條
// 沒有任何真實資料需要的相容路徑（dev sellers 124 筆的 logo/cover_key 實查：0 筆 URL
// 形式、0 筆含 cdn），現已移除——送 cdn 網址進來一律視為外來域名而拒絕回存。
func resolveInputObjectKey(mediaBase, keyVal, urlVal string, allowedPrefixes ...string) (string, error) {
	bases := []string{}
	if mediaBase != "" {
		bases = append(bases, mediaBase)
	}

	normalizedKey, err := mediaurl.NormalizeToObjectKey(keyVal, bases, allowedPrefixes)
	if err != nil {
		return "", err
	}
	normalizedURL, err := mediaurl.NormalizeToObjectKey(urlVal, bases, allowedPrefixes)
	if err != nil {
		return "", err
	}

	if normalizedKey != "" && normalizedURL != "" && normalizedKey != normalizedURL {
		return "", fmt.Errorf("圖片 key 與 url 值不一致: %q / %q", normalizedKey, normalizedURL)
	}
	if normalizedKey != "" {
		return normalizedKey, nil
	}
	return normalizedURL, nil
}
