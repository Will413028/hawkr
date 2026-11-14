package rpc

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"golang.org/x/text/language"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 支援的語系與對應的 BCP 47 標籤；第一個是 Matcher 沒有把握時的預設（zh-TW，ADR 多語系 D1）。
var (
	localeTags    = []language.Tag{language.MustParse(string(uitext.ZhTW)), language.MustParse(string(uitext.EN))}
	localeValues  = []uitext.Locale{uitext.ZhTW, uitext.EN}
	localeMatcher = language.NewMatcher(localeTags)
)

// negotiateLocale 以 Accept-Language 協商語系：沒有 header、解析失敗或信心值為 No 都是 zh-TW。
func negotiateLocale(acceptLanguage string) uitext.Locale {
	if acceptLanguage == "" {
		return uitext.DefaultLocale
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return uitext.DefaultLocale
	}
	_, i, conf := localeMatcher.Match(tags...)
	if conf == language.No || i < 0 || i >= len(localeValues) {
		return uitext.DefaultLocale
	}
	return localeValues[i]
}

// requestLocale 決定這個請求的語系。admin 的 service 固定 zh-TW、不看 header（admin 只有 zh-TW），
// 所以它的回應也不依 header 變化，不需要 Vary；其餘的回應依 Accept-Language 變化，要帶 Vary。
func requestLocale(procedure string, header http.Header) (loc uitext.Locale, varies bool) {
	if aud, ok := AudienceOf(procedure); ok && aud == AudienceAdmin {
		return uitext.ZhTW, false
	}
	return negotiateLocale(strings.Join(header.Values("Accept-Language"), ",")), true
}

// withRequestLocale 把協商結果寫進 ctx，並回傳是否要在回應加 Vary: Accept-Language。
func withRequestLocale(ctx context.Context, procedure string, header http.Header) (context.Context, bool) {
	loc, varies := requestLocale(procedure, header)
	return uitext.WithLocale(ctx, loc), varies
}

// NegotiateRequest 給 interceptor 之外、自己寫使用者文字的入口中介層（限流、session 驗證、轉發閘）用：
// 規則與 interceptor 相同（依 procedure 與 Accept-Language 協商，admin 的 service 固定 zh-TW），回傳帶語系的 ctx，
// 並在回應會依 header 變化時加 Vary: Accept-Language。要在寫任何回應之前呼叫。
// 不是 Connect 的路徑（轉給 gateway 的 REST）同樣協商。
func NegotiateRequest(w http.ResponseWriter, r *http.Request) context.Context {
	ctx, varies := withRequestLocale(r.Context(), r.URL.Path, r.Header)
	if varies {
		markVary(w.Header())
	}
	return ctx
}

// markVary 在成功回應的 header 與錯誤回應的 metadata 上加 Vary: Accept-Language。
func markVary(h http.Header) { h.Add("Vary", "Accept-Language") }

func markErrVary(err error) {
	var ce *connect.Error
	if errors.As(err, &ce) {
		markVary(ce.Meta())
	}
}
