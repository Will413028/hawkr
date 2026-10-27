import { defineRouting } from "next-intl/routing";
import { allRoutableLocales, defaultLocale } from "@/lib/intl/locales";

/**
 * 語系路由（ADR 2026-10-17 D1）：zh-TW 在 `/`、其他語系在 `/<locale>`（`as-needed`），URL 是唯一的語系來源。
 *
 *   locales          只放這個部署會路由的語系（lib/intl/locales.ts）：prod 的 `/en` 因此回 404，dev 兩個都有。
 *   localeDetection  關：任何頁都不依 Accept-Language 導向。
 *   localeCookie     關：偵測關掉後 cookie 不會被讀，留著只會在可快取的 HTML 上多一個 Set-Cookie
 *                    （Cloudflare 的 Workers Cache 略過帶 Set-Cookie 的回應）。
 *   alternateLinks   關：middleware 的 `Link` header 會列出所有設定過的語系（含未發佈的），而且用請求的
 *                    origin；hreflang 改由 metadata 以 canonical origin 與已發佈清單產生（lib/seo/urls.ts）。
 */
export const routing = defineRouting({
	locales: allRoutableLocales(),
	defaultLocale,
	localePrefix: "as-needed",
	localeDetection: false,
	localeCookie: false,
	alternateLinks: false,
});
