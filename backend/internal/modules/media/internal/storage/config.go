package storage

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

// Backend 是 MEDIA_STORAGE 的值。
type Backend string

const (
	// BackendR2 連 Cloudflare R2（`https://<account>.r2.cloudflarestorage.com`）。
	BackendR2 Backend = "r2"
	// BackendS3Compat 連自訂 endpoint 的 S3 相容服務（CI 與本機的 MinIO），用 path-style。
	BackendS3Compat Backend = "s3compat"
)

// Config 是 S3 adapter 的設定。
type Config struct {
	Backend         Backend
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PathStyle       bool
}

// ConfigFromEnv 讀取設定。MEDIA_STORAGE 必填且只接受 r2 或 s3compat；選定實作缺任何設定都記為問題，
// 由 e.Err 與其他問題一起回報。回傳的 Config 只有在 e.Err 為 nil 時有效。
//
//	r2:       MEDIA_R2_ACCOUNT_ID、MEDIA_R2_BUCKET、MEDIA_R2_ACCESS_KEY_ID、MEDIA_R2_SECRET_ACCESS_KEY
//	s3compat: MEDIA_S3_ENDPOINT、MEDIA_S3_REGION、MEDIA_S3_BUCKET、MEDIA_S3_ACCESS_KEY_ID、MEDIA_S3_SECRET_ACCESS_KEY
func ConfigFromEnv(e *config.Env) Config {
	choice := e.String("MEDIA_STORAGE")
	switch Backend(choice) {
	case "":
		return Config{}
	case BackendR2:
		account := e.String("MEDIA_R2_ACCOUNT_ID")
		cfg := Config{
			Backend:         BackendR2,
			Region:          "auto",
			Bucket:          e.String("MEDIA_R2_BUCKET"),
			AccessKeyID:     e.String("MEDIA_R2_ACCESS_KEY_ID"),
			SecretAccessKey: e.String("MEDIA_R2_SECRET_ACCESS_KEY"),
		}
		if account != "" {
			if strings.ContainsAny(account, "/:@?#. ") {
				e.Fail("環境變數 MEDIA_R2_ACCOUNT_ID 只能是帳號 ID，不含網域或路徑")
			} else {
				cfg.Endpoint = "https://" + account + ".r2.cloudflarestorage.com"
			}
		}
		return cfg
	case BackendS3Compat:
		cfg := Config{
			Backend:         BackendS3Compat,
			Endpoint:        e.String("MEDIA_S3_ENDPOINT"),
			Region:          e.String("MEDIA_S3_REGION"),
			Bucket:          e.String("MEDIA_S3_BUCKET"),
			AccessKeyID:     e.String("MEDIA_S3_ACCESS_KEY_ID"),
			SecretAccessKey: e.String("MEDIA_S3_SECRET_ACCESS_KEY"),
			PathStyle:       true,
		}
		if cfg.Endpoint != "" {
			if u, err := url.Parse(cfg.Endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				e.Fail("環境變數 MEDIA_S3_ENDPOINT 的值 %q 不是有效的 http(s) 網址", cfg.Endpoint)
			}
		}
		return cfg
	default:
		e.Fail("環境變數 MEDIA_STORAGE 的值 %q 無效，只接受 %q 或 %q", choice, BackendR2, BackendS3Compat)
		return Config{}
	}
}

func (c Config) validate() error {
	if c.Backend != BackendR2 && c.Backend != BackendS3Compat {
		return fmt.Errorf("storage: unknown backend %q", c.Backend)
	}
	var missing []string
	for _, f := range []struct{ name, v string }{
		{"endpoint", c.Endpoint}, {"region", c.Region}, {"bucket", c.Bucket},
		{"access key id", c.AccessKeyID}, {"secret access key", c.SecretAccessKey},
	} {
		if f.v == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("storage: incomplete %s config, missing %s", c.Backend, strings.Join(missing, ", "))
	}
	return nil
}
