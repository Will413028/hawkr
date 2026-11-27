package rpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/text/language"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var (
	localeTags    = []language.Tag{language.MustParse(string(uitext.ZhTW)), language.MustParse(string(uitext.EN))}
	localeValues  = []uitext.Locale{uitext.ZhTW, uitext.EN}
	localeMatcher = language.NewMatcher(localeTags)
)

func negotiateRequested(acceptLanguage string) (uitext.Locale, bool) {
	if acceptLanguage == "" {
		return uitext.DefaultLocale, false
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return uitext.DefaultLocale, false
	}
	_, i, conf := localeMatcher.Match(tags...)
	if conf == language.No || i < 0 || i >= len(localeValues) {
		return uitext.DefaultLocale, false
	}
	return localeValues[i], true
}

func requestLocale(procedure string, header http.Header) (loc uitext.Locale, requested, varies bool) {
	if aud, ok := AudienceOf(procedure); ok && aud == AudienceAdmin {
		return uitext.ZhTW, false, false
	}
	loc, requested = negotiateRequested(strings.Join(header.Values("Accept-Language"), ","))
	return loc, requested, true
}

func withRequestLocale(ctx context.Context, procedure string, header http.Header) (context.Context, bool) {
	loc, requested, varies := requestLocale(procedure, header)
	if requested {
		return uitext.WithRequestedLocale(ctx, loc), varies
	}
	return uitext.WithLocale(ctx, loc), varies
}

// Negotiated 給 interceptor 之外、自己寫使用者文字的入口中介層（限流、session 驗證）用：規則與 interceptor 相同
// （依 procedure 與 Accept-Language 協商，admin 的 service 固定 zh-TW），在回應會依 header 變化時加
// Vary: Accept-Language，並把協商出的 ctx 只交給 build 組文字或錯誤，呼叫端拿不到也就接不錯 ctx。
// 要在寫任何回應之前呼叫。不是 Connect 的路徑（轉給 gateway 的 REST）此時無法判斷呼叫者是不是管理員，一律協商。
func Negotiated[T any](w http.ResponseWriter, r *http.Request, build func(ctx context.Context) T) T {
	ctx, varies := withRequestLocale(r.Context(), r.URL.Path, r.Header)
	if varies {
		markVary(w.Header())
	}
	return build(ctx)
}

// NegotiatedCommonText 是 Negotiated 加上 CommonText。
func NegotiatedCommonText(w http.ResponseWriter, r *http.Request, code connect.Code) uitext.Message {
	return Negotiated(w, r, func(ctx context.Context) uitext.Message { return CommonText(ctx, code) })
}

// NegotiatedCommonError 是 Negotiated 加上 CommonError。
func NegotiatedCommonError(w http.ResponseWriter, r *http.Request, code connect.Code) *connect.Error {
	return Negotiated(w, r, func(ctx context.Context) *connect.Error { return CommonError(ctx, code) })
}

// NegotiatedRetryLater 是 Negotiated 加上 RetryLater。
func NegotiatedRetryLater(w http.ResponseWriter, r *http.Request, wait time.Duration) *connect.Error {
	return Negotiated(w, r, func(ctx context.Context) *connect.Error { return RetryLater(ctx, wait) })
}

func markVary(h http.Header) { h.Add("Vary", "Accept-Language") }

func markErrVary(err error) {
	var ce *connect.Error
	if errors.As(err, &ce) {
		markVary(ce.Meta())
	}
}
