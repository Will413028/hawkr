package rpc

import (
	"fmt"
	"net/url"
	"strings"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Images 把模組的 object key 組成對外的 Image（ADR 2026-09-25-api-image-uri-template）：
// {MEDIA_ORIGIN}/i/{preset}/{key}，client 只把 {preset} 換成 preset 名稱。對外的圖片 URL 只在這裡組，
// 模組只處理 key。
type Images struct {
	origin string
}

// NewImages 檢查媒體網域：只能是 origin（scheme 加 host，不帶路徑、query、fragment），scheme 是 https；
// 本機開發的 image gateway（wrangler dev）另外接受 http://localhost 與 http://127.0.0.1。
func NewImages(origin string) (Images, error) {
	o, err := parseOrigin("MEDIA_ORIGIN", origin, "https://media.example.com")
	if err != nil {
		return Images{}, err
	}
	return Images{origin: o}, nil
}

func parseOrigin(name, origin, example string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%s %q 必須只有 scheme 與 host，例如 %s", name, origin, example)
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	if u.Scheme != "https" && (u.Scheme != "http" || !local) {
		return "", fmt.Errorf("%s %q 必須是 https（本機的 localhost、127.0.0.1 可以用 http）", name, origin)
	}
	return u.Scheme + "://" + u.Host, nil
}

// Image 回傳 key 的圖；key 是空字串（沒有圖）時回 nil，欄位就不設。
func (i Images) Image(key string) *commonpb.Image {
	if key == "" {
		return nil
	}
	return &commonpb.Image{UriTemplate: i.origin + "/i/{preset}/" + key}
}

// ObjectKey 檢查模組給的圖片欄位是 object key：舊服務的資料可能存了完整網址，那不能組進 URI template，回 Unavailable。
func ObjectKey(key string) error {
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return apperr.Unavailable("圖片來源沒有提供 object key")
	}
	return nil
}
