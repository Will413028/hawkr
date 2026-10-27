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

	// FailThirdPartyWebhook：消費者是**第三方的 webhook 送信端**（目前唯一一條是
	// PayUni 的 /webhook/payuni/notify）。
	//
	// ⚠️ 這一條**刻意維持 502，不要跟著 424 改**。第三方對 HTTP status 家族的解讀
	// 與瀏覽器相反：5xx ＝「你那邊壞了，我等一下重送」；4xx ＝「你說我這個請求有
	// 問題，那我不重送了」。而本 repo 的 webhook 失敗回收設計**整個建立在重送上**：
	// 10 號設計書 §4.5.3／§4.5.5.D 明載「T2 失敗一律 rethrow → controller 回 500 →
	// PayUni 重送 → 下次 T1 dedup 命中 FAILED → replay 路徑接手」，且方案 F 被選中的
	// 理由就是「復用既有重送機制」。把這條改成 4xx ＝ 讓 PayUni 停止重送 ＝ 那條
	// replay 路徑永遠等不到觸發，付款狀態永久不同步，而且**沒有任何錯誤訊號**。
	// CF 會把 body 換掉在這裡不是問題：PayUni 不讀我們的 envelope，它只看 status。
	//
	// ⚠️ 另一個不對稱：這裡的錯誤是「payuni-webhook-service 連不上」——正是最需要
	// 對方重送的時刻（服務回來之後那筆通知還在）。回 4xx 會把一個**可恢復**的
	// 中斷變成**永久遺失**。
	//
	// 誠實標註：PayUni 對 5xx 的實際重送次數與窗口**未經實測坐實**，10 號 §4.5.5.D
	// 自己就標了「實測 PayUni 的重送策略需確認」。但那影響的是「重送夠不夠」，
	// 不影響本判斷的方向——改成 4xx 是把重送機率從「未知但非零」直接歸零。
	FailThirdPartyWebhook UpstreamFailureStatus = http.StatusBadGateway // 502
)

// upstreamFailureMessage 是**唯一**會出現在使用者畫面上的那句話。
//
// ⚠️ 這裡絕對不能串接 err.Error()。原本是 `"service unavailable: " + err.Error()`，
// 在 502 時代之所以沒事，是因為 Cloudflare 把整個 body 換掉了 —— 使用者看不到。
// 改成 424 之後 envelope 是真的送達的，而 seller App 的 `AppException`
// （`lib/core/exceptions/app_exception.dart:26-44`）直接把 `responseData['message']`
// 當文案顯示 ⇒ 使用者會在畫面上看到 `dial tcp 10.x.x.x:9936: connect: connection refused`，
// 內部 host:port 就這樣從「被 CF 藏著」變成「印在使用者手機上」。
//
// **PayUni 那條（維持 502）也吃同一份文案**：它現在被 CF 擋著是巧合不是設計，
// 而且 PayUni 只看 status 不讀 body，換成固定文案零成本。
//
// 真正的錯誤只進 log；排障靠 envelope 裡的 traceId 對回那筆 log。
const upstreamFailureMessage = "服務暫時無法使用，請稍後再試"

// writeUpstreamFailureEnvelope 在上游不可達時回統一 envelope，status 由呼叫端依
// 消費者類別宣告（見 UpstreamFailureStatus）。
//
// 這裡拿不到 *gin.Context（httputil.ReverseProxy 的 ErrorHandler 只給 ResponseWriter），
// 所以不能用 response 的 helper，但形狀必須一致：欄名是 message 不是 msg、錯誤回應要帶
// traceId（10 檔 §2；response.Err 系列一律帶）。
// 一律 json.Marshal，不用字串串接（歷史理由：err 裡只要有一個雙引號就會產出壞掉的
// JSON；現在 message 是常數，這條仍保留，因為 traceId 來自請求 header、同樣不可信）。
//
// envelope 的 code 鏡射 HTTP status（424／502），沿用這條路徑原本的慣例：它不是
// CodeMeta 登錄的業務碼，鏡射 status 至少是自我說明的，不會被誤認成某個業務語意。
func writeUpstreamFailureEnvelope(w http.ResponseWriter, r *http.Request, status UpstreamFailureStatus, err error) {
	traceID := proxyTraceID(w, r)

	// 錯誤細節只進 log。ctx 帶著 trace/span，log 裡自然對得回 envelope 的 traceId；
	// r 可能為 nil（測試路徑），此時退回 context.Background()。
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

// proxyTraceID 取這次請求的 trace id，順序與 pkg/envelope.ExtractTraceID 對齊。
// 首選是 TracingMiddleware 已寫進**回應** header 的 x-trace-id（全域 r.Use，代理 handler
// 之前就跑過，所以是 Gateway 本次請求真正的 traceId）；tracer 未啟用時退回上游 propagate
// 進來的請求 header。取不到就留空，Envelope 的 omitempty 會把欄位省略。
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
	"media": {
		Name:   "media-service",
		Host:   getEnvOrDefault("MEDIA_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("MEDIA_SERVICE_PORT", "9936"),
		Prefix: "",
	},
	"shipping": {
		Name:   "shipping-service",
		Host:   getEnvOrDefault("SHIPPING_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("SHIPPING_SERVICE_PORT", "9930"),
		Prefix: "",
	},
	"payment": {
		Name:   "payment-service",
		Host:   getEnvOrDefault("PAYMENT_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("PAYMENT_SERVICE_PORT", "9924"),
		Prefix: "",
	},
	"payuni-webhook": {
		Name:   "payuni-webhook-service",
		Host:   getEnvOrDefault("PAYUNI_WEBHOOK_SERVICE_HOST", "127.0.0.1"),
		Port:   getEnvOrDefault("PAYUNI_WEBHOOK_SERVICE_PORT", "9946"),
		Prefix: "",
	},
	// invoice-service 的**檔案串流面**（10042），不是 gRPC 面（10041）。發票 PDF 的
	// bytes 不該經 gRPC，而擁有者述詞（order_no × buyer_id × document_id）不在 media
	// ⇒ 這是「微服務不暴露 HTTP」的第二個明列例外（58 號 §6.1.4）。
	//
	// ⚠️ 這一列漏了**不會編譯錯**：CreateProxy 查不到 service name 就回一個永遠
	// response.NotFound 的 handler，症狀是「下載按鈕點了說找不到」。
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

// mustParseTarget 解析上游位址，失敗即 panic（開機就炸，不帶病上線）。
//
// 為什麼是 panic 而不是「回一個會回錯誤的 handler」：這裡的輸入全部來自部署期注入的
// env（`*_SERVICE_HOST` / `*_SERVICE_PORT`），解析失敗代表**這個環境的配置壞了**，
// 不是某個請求壞了。原本寫成 `targetURL, _ := url.Parse(...)` 把 error 丟掉，一旦
// 解析失敗 targetURL 就是 nil，而 nil 不會在啟動時被任何東西發現——它會在該路由的
// **每一個請求**上，於 httputil.ReverseProxy 內部（rewriteRequestURL 首句取
// target.RawQuery）nil dereference，被 gin Recovery() 接成 500 + 空 body。
// 那個形狀沒有 envelope、沒有 traceId、連 ErrorHandler 都不會進，日誌裡不會出現
// 任何指向配置的線索（#2813 為此花掉整輪才定位到這一行）。
//
// #2813 的實際觸發值是 `SELLER_SERVICE_PORT=tcp://10.43.144.51:10021`：k8s 對 Service
// `seller-service` 自動注入的 service-link 變數，與 gateway 自己要讀的同名 env 撞名。
// cm/gateway-config 沒有定義 SELLER_SERVICE_PORT，於是 service-link 的值生效，
// 拼出 `http://seller-service:tcp://10.43.144.51:10021` —— Go 1.26 的 URL host/port
// 驗證判它 `invalid port`，url.Parse 回 nil。同一個 pod 內 ACCOUNT_/BUYER_/SOCIAL_
// SERVICE_PORT 也是這個形狀，目前沒有代理路由在用；哪天有人加了，這裡會在開機
// 就炸並印出實際值，而不是再留一個沒有線索的 500。
//
// ⚠️ 要驗這類 parse 行為請在**本 module 內**寫測試：Go 1.26 收緊了 URL 驗證，而
// go.mod 的 go directive 會透過 GODEBUG 決定套用新舊哪套語意 —— 同一個字串在
// `go 1.24` 的 module 裡 parse 得過。另開小程式驗證會得到相反結論（#2813 踩過）。
//
// 額外檢查 targetURL == nil 而不是只看 err：成本是一個比較，換到的是「不明來源的
// nil 也擋在開機」。
//
// panic 作為配置錯誤的處置方式與本包既有慣例一致（見 router.assertRouteCoverage 的
// boot 斷言）：寧可部署即失敗，不要靜默帶病服務。
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
// onUpstreamFailure 是必填的：呼叫端必須宣告這條路由的消費者是我們自己的前端
// （FailClientFacing）還是第三方 webhook 送信端（FailThirdPartyWebhook），因為
// 上游不可達時兩者要的答覆剛好相反。判準與理由見 UpstreamFailureStatus。
func CreateProxy(serviceName string, onUpstreamFailure UpstreamFailureStatus) gin.HandlerFunc {
	config, ok := services[serviceName]
	if !ok {
		// 未登記的 service name = wiring 錯誤（該路由的每個請求都會落到這裡）。
		// 仍走統一 envelope，不自訂形狀（10 檔 §2）。
		return func(c *gin.Context) {
			response.NotFound(c, "service not found")
		}
	}

	targetURL := mustParseTarget(serviceName, config)

	// Rewrite（Go 1.26 起取代 deprecated 的 Director）。
	//
	// ⚠️ Rewrite 不像 Director 那樣自動補 X-Forwarded-*：ServeHTTP 在呼叫 Rewrite 之前
	//    會把 client 送來的 Forwarded / X-Forwarded-For / -Host / -Proto 全部刪掉，補不補
	//    由這個函式決定 ⇒ 必須顯式呼叫 SetXForwarded()。它的淨效果是**覆寫**而非附加
	//    （client 自稱的值已被刪光，只留真實 RemoteAddr），比 Director 時代的
	//    「client 值 + 真實 IP」更嚴格。2026-08-27 實查：全 services/ 無任何服務讀
	//    X-Forwarded-For / X-Real-IP，這個差異目前沒有消費者。
	//    **日後若有下游要看完整轉發鏈，這裡得改成手動接鏈**（自己從 r.In 讀出
	//    X-Forwarded-For 再串上 RemoteAddr），SetXForwarded() 一句不夠。
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(targetURL) // 等價於 NewSingleHostReverseProxy 的預設 director
			r.SetXForwarded()

			// Strip /api/v1 prefix - backend services expect paths like /auth/login, not /api/v1/auth/login
			path := r.Out.URL.Path
			if strings.HasPrefix(path, "/api/v1/") {
				path = strings.TrimPrefix(path, "/api/v1")
			} else if strings.HasPrefix(path, "/api/") {
				path = strings.TrimPrefix(path, "/api")
			}
			r.Out.URL.Path = path

			// Set host header（SetURL 會把 Out.Host 清空以沿用 URL.Host，這裡顯式釘住）
			r.Out.Host = targetURL.Host
		},
	}

	// Error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeUpstreamFailureEnvelope(w, r, onUpstreamFailure, err)
	}

	return func(c *gin.Context) {
		// 注入身分 header（小寫，符合 API 設計規範 §6.2）
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

// CreateProxyWithPathRewrite creates a proxy that rewrites the request path before
// forwarding. onUpstreamFailure 同 CreateProxy——必填，依消費者類別宣告。
func CreateProxyWithPathRewrite(serviceName, fromPath, toPath string, onUpstreamFailure UpstreamFailureStatus) gin.HandlerFunc {
	config, ok := services[serviceName]
	if !ok {
		return func(c *gin.Context) {
			response.NotFound(c, "service not found")
		}
	}

	targetURL := mustParseTarget(serviceName, config)
	// Rewrite 取代 deprecated 的 Director，X-Forwarded-* 的語意見 CreateProxy 的註解。
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(targetURL)
			r.SetXForwarded()
			// 路徑改寫
			if r.Out.URL.Path == fromPath {
				r.Out.URL.Path = toPath
			}
			r.Out.Host = targetURL.Host
		},
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeUpstreamFailureEnvelope(w, r, onUpstreamFailure, err)
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
