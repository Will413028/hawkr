// Package uitext 是共用文案目錄（契約值到使用者面文字）的 Go 執行期。
//
// 目錄的唯一來源是 catalog/{config,zh-TW,en}.json，catalog_gen.go 由 backend/cmd/uitext-gen 生成，不得手改。
// 決策：ADR backend/docs/adr/2026-10-17-multi-locale-architecture.md；設計：
// backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-Lprime-design.md。
package uitext

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// Locale 是 UI 語系，值是 BCP 47 標籤。
type Locale string

const (
	// ZhTW 是來源語系。
	ZhTW Locale = "zh-TW"
	// EN 是英文。
	EN Locale = "en"
)

// DefaultLocale 是沒有協商結果時的語系（ADR D1）。
const DefaultLocale = ZhTW

type localeKey struct{}

// WithLocale 與 LocaleFrom 是請求語系的縫：L′ 沒有人呼叫 WithLocale，LocaleFrom 一律回 DefaultLocale；
// I-server 的 interceptor 依 Accept-Language 協商後寫入。模組只能經這裡取語系，不讀 header。
func WithLocale(ctx context.Context, l Locale) context.Context {
	return context.WithValue(ctx, localeKey{}, l)
}

// LocaleFrom 回 ctx 裡的請求語系，沒有就是 DefaultLocale。
func LocaleFrom(ctx context.Context) Locale {
	if l, ok := ctx.Value(localeKey{}).(Locale); ok && l != "" {
		return l
	}
	return DefaultLocale
}

// Text 是某個語系與受眾的目錄查詢。zero value 等於 For(DefaultLocale)。
type Text struct {
	locale Locale
	staff  bool
}

// For 回預設受眾（賣家、買家面）在語系 l 的目錄。
func For(l Locale) Text { return Text{locale: l} }

// Staff 回後台員工受眾的目錄：admin RPC 固定 zh-TW 加 staff（ADR D1）。
func Staff() Text { return Text{locale: ZhTW, staff: true} }

// Locale 是這個目錄查詢使用的語系。
func (t Text) Locale() Locale {
	if t.locale == "" {
		return DefaultLocale
	}
	return t.locale
}

// Message 是一句給使用者看的文字與它實際使用的語系：語系缺字時退回來源語系，Locale 就是來源語系，不是請求的語系
// （ADR D3：LocalizedMessage.locale 是實際使用的語系）。
type Message struct {
	Text   string
	Locale Locale
}

// String 回 Text，讓 %s 與 %v 印出的是文字而不是整個結構。
func (m Message) String() string { return m.Text }

// dateTimeLayouts 是各語系的日期時間版面；沒有列出的語系用 DefaultLocale 的。
var dateTimeLayouts = map[Locale]string{ZhTW: "2006/01/02 15:04"}

// FormatDateTime 是伺服器文字裡的日期時間：台北時間、到分鐘。伺服器文字的時間版面只有這一處；
// en 的版面核可後在 dateTimeLayouts 加一列。
func FormatDateTime(l Locale, t time.Time) string {
	layout, ok := dateTimeLayouts[l]
	if !ok {
		layout = dateTimeLayouts[DefaultLocale]
	}
	return t.In(twtime.Taipei).Format(layout)
}
