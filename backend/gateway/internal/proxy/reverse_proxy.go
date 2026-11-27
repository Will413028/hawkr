package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpstreamFailureStatus 宣告「這條代理路由的上游不可達時，要回什麼 HTTP status」。
//
// ⚠️ 判準是**這條路由的消費者是誰**，不是錯誤的技術性質——同一個「上游連不上」的
// 事實，對兩類消費者的正確答覆剛好相反，所以每個代理路由都必須自己講清楚。
// 這是必填參數而不是預設值：漏答會編譯不過，不會靜默選到錯的那一邊。
type UpstreamFailureStatus int

const (
	// FailClientFacing：消費者是**我們自己的前端**（web / admin / App）。
	//
	// 回 424 Failed Dependency 而不是 502：Cloudflare 站在 Gateway 前面，origin 回
	// 5xx 時它會把整個 body 換成自己的錯誤頁（實測：叢集內直打回 application/json
	// 的 envelope，同一支端點經 CF 變成 `error code: 502` ＋ text/plain）⇒ 前端的
	// response.json() 直接拋錯，**這個 envelope 永遠送不到**。9280 就是這個形狀，
	// 已於 b723abc4c 改 424；語意也對得上——「依賴的上游失敗」正是這裡的意思，
	// 而且 4xx 說的是「這一次請求出了事」，不是「這個服務不存在」（後者才是
	// 56 §0.3 禁 404／503 的顧慮，對 424 不成立）。同理見 response/bizhttp.go。
	FailClientFacing UpstreamFailureStatus = http.StatusFailedDependency // 424
)

const upstreamFailureMessage = "服務暫時無法使用，請稍後再試"

func writeUpstreamFailureEnvelope(w http.ResponseWriter, r *http.Request, status UpstreamFailureStatus, err error) {
	traceID := proxyTraceID(w, r)

	ctx := context.Background()
	method, path := "", ""
	if r != nil {
		ctx = r.Context()
		method, path = r.Method, r.URL.Path
	}
	pkglogger.Error(ctx, "[Proxy] upstream unreachable",
		zap.Int("status", int(status)),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("traceId", traceID),
		zap.Error(err))

	code := int32(status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(status))
	body, mErr := json.Marshal(response.Envelope{
		Code:    code,
		Message: upstreamFailureMessage,
		TraceID: traceID,
	})
	if mErr != nil {
		body = []byte(`{"code":` + strconv.Itoa(int(status)) + `,"message":"` + upstreamFailureMessage + `"}`)
	}
	_, _ = w.Write(body)
}

func proxyTraceID(w http.ResponseWriter, r *http.Request) string {
	if tid := w.Header().Get("x-trace-id"); tid != "" {
		return tid
	}
	if r == nil {
		return ""
	}
	if tid := r.Header.Get("x-trace-id"); tid != "" {
		return tid
	}
	return r.Header.Get("x-request-id")
}

// ServiceConfig holds backend service configuration
type ServiceConfig struct {
	Name   string
	Host   string
	Port   string
	Prefix string
}

var services = map[string]ServiceConfig{
	"product": {
		Name:   "product-service",
		Host:   getEnvOrDefault("PRODUCT_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("PRODUCT_SERVICE_PORT", "9912"),
		Prefix: "",
	},
	"order": {
		Name:   "order-service",
		Host:   getEnvOrDefault("ORDER_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("ORDER_SERVICE_PORT", "9914"),
		Prefix: "",
	},
	"aftersales": {
		Name:   "aftersales-service",
		Host:   getEnvOrDefault("AFTERSALES_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("AFTERSALES_SERVICE_PORT", "9922"),
		Prefix: "",
	},
	"shipping": {
		Name:   "shipping-service",
		Host:   getEnvOrDefault("SHIPPING_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("SHIPPING_SERVICE_PORT", "9930"),
		Prefix: "",
	},
	"invoice": {
		Name:   "invoice-service",
		Host:   getEnvOrDefault("INVOICE_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("INVOICE_SERVICE_FILE_PORT", "10042"),
		Prefix: "",
	},
}

func getEnvOrDefault(key, defaultVal string) string {
	return pkgconfig.GetEnvOrDefault(key, defaultVal)
}

func mustParseTarget(serviceName string, config ServiceConfig) *url.URL {
	raw := "http://" + config.Host + ":" + config.Port
	targetURL, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf(
			"proxy: service %q 的上游位址無法解析: %q (host=%q port=%q): %v",
			serviceName, raw, config.Host, config.Port, err))
	}
	if targetURL == nil {
		panic(fmt.Sprintf(
			"proxy: service %q 的上游位址解析後為 nil: %q (host=%q port=%q)",
			serviceName, raw, config.Host, config.Port))
	}
	return targetURL
}

// CreateProxy creates a reverse proxy handler for a service.
//
// onUpstreamFailure 是必填的：呼叫端必須宣告這條路由上游不可達時的失敗模式。
// 判準與理由見 UpstreamFailureStatus。
func CreateProxy(serviceName string, onUpstreamFailure UpstreamFailureStatus) gin.HandlerFunc {
	config, ok := services[serviceName]
	if !ok {
		return func(c *gin.Context) {
			response.NotFound(c, "service not found")
		}
	}

	targetURL := mustParseTarget(serviceName, config)

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(targetURL)
			r.SetXForwarded()

			path := r.Out.URL.Path
			if strings.HasPrefix(path, "/api/v1/") {
				path = strings.TrimPrefix(path, "/api/v1")
			} else if strings.HasPrefix(path, "/api/") {
				path = strings.TrimPrefix(path, "/api")
			}
			r.Out.URL.Path = path

			r.Out.Host = targetURL.Host
		},
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeUpstreamFailureEnvelope(w, r, onUpstreamFailure, err)
	}

	return func(c *gin.Context) {
		if accountId, exists := c.Get("accountId"); exists {
			c.Request.Header.Set("x-account-id", fmt.Sprintf("%v", accountId))
		}
		if accountType, exists := c.Get("accountType"); exists {
			c.Request.Header.Set("x-account-type", fmt.Sprintf("%v", accountType))
			pkglogger.Info(c.Request.Context(), "[Proxy] injecting account type",
				zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path), zap.Any("accountType", accountType))
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// GetServiceConfig returns configuration for a service
func GetServiceConfig(name string) (ServiceConfig, bool) {
	config, ok := services[name]
	return config, ok
}

// PathOverride defines a path pattern and its override handler
type PathOverride struct {
	Handler    gin.HandlerFunc
	PathPrefix string
}
