package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	yaml "go.yaml.in/yaml/v2"
)

const maxBodyLogSize = 2000

const redactedValue = `"[REDACTED]"`

var sensitiveKeys = map[string]struct{}{
	"password":             {},
	"token":                {},
	"accesstoken":          {},
	"refreshtoken":         {},
	"refreshtokenenc":      {},
	"idtoken":              {},
	"firebasetoken":        {},
	"firebaseidtoken":      {},
	"firebasephoneidtoken": {},
	"identitytoken":        {},
	"tempbindtoken":        {},
	"authtoken":            {},
	"authorization":        {},
	"secret":               {},
	"apikey":               {},
	"otpcode":              {},
	"authorizationcode":    {},
	"applepaytoken":        {},
	"googlepaytoken":       {},
	"idcardno":             {},
	"idcardfronturl":       {},
	"idcardbackurl":        {},
	"taxid":                {},
	"bankaccount":          {},
	"bankbookurl":          {},
	"realname":             {},
	"birthdate":            {},
	"guestid":              {},
	"operationcredential":  {},
	"handoffcode":          {},
	"uploadurl":            {},
	"appkey":               {},
	"carrierid":            {},
}

var sensitiveKeySuffixes = []string{"stagedtoken", "stagedtokens"}

func isSensitiveKey(normalized string) bool {
	if _, ok := sensitiveKeys[normalized]; ok {
		return true
	}
	for _, suffix := range sensitiveKeySuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

var allowedKeys = map[string]struct{}{
	"code": {}, "errors": {}, "errormessage": {},
	"results": {}, "providermessage": {}, "providercontext": {},
	"providerrefundno": {}, "redirecturl": {}, "admincontext": {},

	"accountid": {}, "buyerid": {}, "assetid": {}, "appealid": {},
	"caseno": {}, "checkoutid": {}, "coupontemplateid": {}, "eshipid": {},
	"id": {}, "orderno": {}, "ordernos": {}, "orderitemid": {},
	"partnerid": {}, "productid": {}, "productids": {}, "shipmentno": {},
	"shiptradeno": {}, "skuid": {}, "storeid": {}, "storecode": {},
	"tagids": {}, "uid": {}, "sessionid": {}, "groupindex": {},
	"onboardingid": {}, "decidedbyadminid": {}, "traceid": {},
	"validationno": {},

	"state": {}, "status": {}, "statuslabel": {}, "eventtype": {},
	"evidencetype": {}, "appealstate": {}, "decision": {}, "outcome": {},
	"resolutiontype": {}, "goodstype": {}, "temperature": {},
	"temperaturecode": {}, "shippingtype": {}, "shippingmethod": {},
	"shippingroute": {}, "paymenttype": {}, "providersource": {},
	"providerstatus": {}, "returnproviderstatus": {}, "refundtaskstatus": {},
	"payunisubmitstatus": {}, "clienttype": {}, "customertype": {},
	"usertype": {}, "platform": {}, "deliverytemp": {},
	"deliverytempintersection": {}, "onboardingstatus": {}, "errorcode": {},
	"valid": {}, "eligible": {}, "enabled": {}, "available": {},
	"canappeal": {}, "autoescalated": {}, "isdefault": {}, "iscontact": {},
	"istrolley": {}, "onboardingrequired": {},
	"freeshippingeligible": {}, "tentativefreeshippingeligible": {},
	"webhookreceived": {}, "success": {}, "tags": {},

	"actualpayamount": {}, "actualrefundamount": {}, "availableqty": {},
	"couponamount": {}, "couponcount": {}, "couponminspend": {},
	"discountamount": {}, "estimatedtotal": {}, "fee": {},
	"freeshippingthreshold": {}, "importedcount": {}, "listprice": {},
	"newdiscount": {}, "newshippingfee": {}, "newsubtotal": {},
	"newtotalamount": {}, "oldtotalamount": {}, "originalprice": {},
	"price": {}, "quantity": {}, "refundableamount": {}, "refundamount": {},
	"requestedqty": {}, "saleprice": {}, "sharediscount": {},
	"shippingfee": {}, "stockqty": {}, "subtotal": {},
	"subtotaloriginalamount": {}, "total": {}, "totalactualamount": {},
	"totalcount": {}, "totaldiscount": {}, "totaloriginalamount": {},
	"totalpayamount": {}, "totalshippingfee": {}, "totalspent": {},
	"deletedcount": {}, "successcount": {}, "count": {},
	"completiontokens": {}, "prompttokens": {}, "maxtokens": {},

	"appealsubmittedat": {}, "appealdeadline": {}, "autocompleteatstamp": {},
	"autoconfirmat": {}, "businessdays": {}, "calltime": {},
	"closedat": {}, "coveragedays": {}, "createdat": {}, "date": {},
	"dayofweek": {}, "decidedat": {}, "deliverydate": {},
	"lastrejectedat": {}, "maxholidaydate": {},
	"processdeadline": {}, "processdeadlinehours": {}, "sellerpickedupat": {},
	"shipbackdeadlineat": {}, "shipdate": {}, "statustime": {},
	"storearrivedat": {}, "storepickupdeadlineat": {}, "timeline": {},
	"timestamp": {}, "updatedat": {}, "year": {}, "years": {},
	"returndeliveredat": {},

	"storename": {}, "skuname": {}, "sellername": {}, "returnstorename": {},
	"labelcompanyname": {},

	"fcmtoken": {}, "phototokens": {}, "evidencephototokens": {},
	"inspectionphototokens": {},

	"page": {}, "size": {},

	"bankcode": {}, "bankname": {},

	"accounttype": {},
}

func isAllowedKey(normalized string) bool {
	_, ok := allowedKeys[normalized]
	return ok
}

func shouldRedact(normalized string, extraAllowed ...map[string]struct{}) bool {
	if isSensitiveKey(normalized) {
		return true
	}
	if isAllowedKey(normalized) {
		return false
	}
	if len(extraAllowed) > 0 && extraAllowed[0] != nil {
		if _, ok := extraAllowed[0][normalized]; ok {
			return false
		}
	}
	return true
}

const scanLimit = 16 << 10

var kvPattern = regexp.MustCompile(`"([A-Za-z0-9_-]+)"\s*:\s*("(?:[^"\\]|\\.)*"|\[[^\[\]{}]*\]|true|false|null|-?\d+(?:\.\d+)?|[^\s,:{}\[\]"]+)`)

func normalizeKey(k string) string {
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, "-", "")
	return k
}

func redactBody(b []byte, extraAllowed ...map[string]struct{}) string {
	if len(b) == 0 {
		return ""
	}
	if !utf8.Valid(b) {
		return "[binary]"
	}
	s := string(b)
	if len(s) > scanLimit {
		s = s[:scanLimit]
	}
	matches := kvPattern.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return truncateBody([]byte(s))
	}
	var sb strings.Builder
	sb.Grow(len(s))
	last := 0
	for _, m := range matches {
		key := s[m[2]:m[3]]
		if !shouldRedact(normalizeKey(key), extraAllowed...) {
			continue
		}
		valStart, valEnd := m[4], m[5]
		sb.WriteString(s[last:valStart])
		sb.WriteString(redactedValue)
		last = valEnd
	}
	sb.WriteString(s[last:])
	return truncateBody([]byte(sb.String()))
}

func redactQuery(raw string, extraAllowed ...map[string]struct{}) string {
	var sb strings.Builder
	sb.Grow(len(raw))
	for rest := raw; rest != ""; {
		pair := rest
		sep := byte(0)
		if n := strings.IndexAny(rest, "&;"); n >= 0 {
			pair, sep, rest = rest[:n], rest[n], rest[n+1:]
		} else {
			rest = ""
		}
		if eq := strings.IndexByte(pair, '='); eq < 0 {
			sb.WriteString(pair)
		} else {
			key := pair[:eq]
			decoded := key
			if u, err := url.QueryUnescape(key); err == nil {
				decoded = u
			}
			sb.WriteString(key)
			sb.WriteByte('=')
			if shouldRedact(normalizeKey(decoded), extraAllowed...) {
				sb.WriteString(redactedValue)
			} else {
				sb.WriteString(pair[eq+1:])
			}
		}
		if sep != 0 {
			sb.WriteByte(sep)
		}
	}
	return sb.String()
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body               *bytes.Buffer
	contentTypeChecked bool
	streaming          bool
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	if !w.contentTypeChecked {
		w.contentTypeChecked = true
		w.streaming = isStreamingContentType(w.Header().Get("Content-Type"))
	}
	if !w.streaming && w.body.Len() < scanLimit+1 {
		remaining := scanLimit + 1 - w.body.Len()
		chunk := b
		if len(chunk) > remaining {
			chunk = chunk[:remaining]
		}
		w.body.Write(chunk)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying connection.
// gin.ResponseWriter (the embedded interface field's type) does not declare
// Unwrap() even though gin's concrete implementation has it — embedding an
// interface only promotes methods the interface itself declares. Without
// this, SetReadDeadline/SetWriteDeadline calls against a Logger-wrapped
// c.Writer silently no-op instead of reaching the real connection.
func (w *responseBodyWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func isStreamingContentType(ct string) bool {
	switch {
	case strings.HasPrefix(ct, "text/event-stream"),
		strings.HasPrefix(ct, "application/connect+"),
		strings.HasPrefix(ct, "application/octet-stream"),
		strings.HasPrefix(ct, "image/"),
		strings.HasPrefix(ct, "video/"),
		strings.HasPrefix(ct, "audio/"):
		return true
	default:
		return false
	}
}

func appendTraceFields(fields []zap.Field, ctx context.Context) []zap.Field {
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.HasTraceID() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return fields
}

// Logger logs request and response details for every API call. Prod takes a
// metadata-only path that never reads request/response bodies (architectural
// elimination, not a config toggle that can be left on by mistake — see
// #2506's design spec §2 for why). Dev/staging keep body capture but redact
// by allowlist (Task 2). diagnosticOverrideActive lets a specific prod route
// borrow the dev path for a config-scoped, expiring window (Task 4).
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if pkgconfig.IsProduction() && !diagnosticOverrideActive(c) {
			loggerMetadataOnly(c)
			return
		}
		loggerWithBody(c)
	}
}

func loggerMetadataOnly(c *gin.Context) {
	start := time.Now()
	method := c.Request.Method

	c.Next()

	latency := time.Since(start)
	status := c.Writer.Status()

	fields := []zap.Field{
		zap.String("method", method),
		zap.String("route", c.FullPath()),
		zap.Int("status", status),
		zap.Duration("latency", latency),
		zap.String("ip", c.ClientIP()),
		zap.String("xff", truncateHeaderValue(c.GetHeader("X-Forwarded-For"))),
		zap.String("app_version", truncateHeaderValue(c.GetHeader("X-App-Version"))),
		zap.String("app_type", truncateHeaderValue(c.GetHeader("X-App-Type"))),
	}

	if actorID, ok := c.Get("accountId"); ok {
		if id, ok := actorID.(int64); ok {
			fields = append(fields, zap.Int64("actor_id", id))
		}
	}

	if status >= http.StatusBadRequest {
		fields = append(fields, zap.String("error", truncateHeaderValue(c.Errors.String())))
	}

	fields = appendTraceFields(fields, c.Request.Context())

	zap.L().Info("gateway request", fields...)
}

func loggerWithBody(c *gin.Context) {
	extra := diagnosticExtraAllowed(c)
	start := time.Now()
	path := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		path += "?" + redactQuery(c.Request.URL.RawQuery, extra)
	}
	method := c.Request.Method

	var reqBody string
	if c.Request.Body != nil {
		limited := io.LimitReader(c.Request.Body, scanLimit+1)
		bodyBytes, err := io.ReadAll(limited)
		c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), c.Request.Body))
		if err == nil {
			reqBody = redactBody(bodyBytes, extra)
		}
	}

	auth := ""
	if c.GetHeader("Authorization") != "" {
		auth = "[REDACTED]"
	}

	rbw := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
	c.Writer = rbw

	c.Next()

	latency := time.Since(start)
	status := c.Writer.Status()
	respBody := redactBody(rbw.body.Bytes(), extra)

	fields := []zap.Field{
		zap.Int("status", status),
		zap.Duration("latency", latency),
		zap.String("ip", c.ClientIP()),
		zap.String("xff", truncateHeaderValue(c.GetHeader("X-Forwarded-For"))),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("auth", auth),
		zap.String("req_body", reqBody),
		zap.String("resp_body", respBody),
		zap.String("app_version", truncateHeaderValue(c.GetHeader("X-App-Version"))),
		zap.String("app_type", truncateHeaderValue(c.GetHeader("X-App-Type"))),
	}

	fields = appendTraceFields(fields, c.Request.Context())

	zap.L().Info("gateway request", fields...)
}

var diagnosticBodyLoggingConfigPath = "/etc/gateway/diagnostic-body-logging.yaml"

type diagnosticBodyLoggingRule struct {
	Route     string    `yaml:"route"`
	Fields    []string  `yaml:"fields"`
	Owner     string    `yaml:"owner"`
	ExpiresAt time.Time `yaml:"expires_at"`
}

type diagnosticBodyLoggingConfig struct {
	Rules []diagnosticBodyLoggingRule `yaml:"diagnostic_body_logging"`
}

func loadDiagnosticBodyLoggingRules() []diagnosticBodyLoggingRule {
	data, err := os.ReadFile(diagnosticBodyLoggingConfigPath)
	if err != nil {
		return nil
	}
	var cfg diagnosticBodyLoggingConfig
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		zap.L().Warn("diagnostic body logging config malformed, ignoring", zap.Error(err))
		return nil
	}
	return cfg.Rules
}

func diagnosticOverrideActive(c *gin.Context) bool {
	return diagnosticExtraAllowed(c) != nil
}

func diagnosticExtraAllowed(c *gin.Context) map[string]struct{} {
	rules := loadDiagnosticBodyLoggingRules()
	routeKey := c.Request.Method + " " + c.FullPath()
	now := time.Now()
	for _, r := range rules {
		if r.Route == routeKey && now.Before(r.ExpiresAt) && len(r.Fields) > 0 {
			extra := make(map[string]struct{}, len(r.Fields))
			for _, f := range r.Fields {
				extra[normalizeKey(f)] = struct{}{}
			}
			return extra
		}
	}
	return nil
}

const headerValueLogLimit = 32

func truncateHeaderValue(v string) string {
	if len(v) <= headerValueLogLimit {
		return v
	}
	r := []rune(v)
	if len(r) <= headerValueLogLimit {
		return v
	}
	return string(r[:headerValueLogLimit])
}

func truncateBody(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if !utf8.Valid(b) {
		return "[binary]"
	}
	s := strings.TrimSpace(string(b))
	if len(s) > maxBodyLogSize {
		return s[:maxBodyLogSize] + "...(truncated)"
	}
	return s
}
