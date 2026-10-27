/**
 * 網站的語系清單：有哪些語系、哪些已發佈、這個部署會路由哪些。
 *
 * 決策：backend/docs/adr/2026-10-17-multi-locale-architecture.md D1。
 *
 * 「已發佈」和「會路由」是兩件事：
 *   - 已發佈（`publishedLocales`）：對外承諾存在的語系。hreflang、sitemap、`/en` 的索引規則只看這份。
 *     某個介面在該語系下看不到任何未翻譯的文字，才能把語系加進它的清單（ADR 的發佈門檻）。
 *   - 會路由（`routableLocales`）：這個部署的 router 認得的語系。prod 只路由已發佈的（`/en` 回 404），
 *     dev flavor 路由全部支援的語系，讓還沒發佈的語系能在開發與 preview 測試。
 *
 * 已發佈清單每個介面一份，初始都是 `["zh-TW"]`：
 *   buyer   買家頁：除了賣家介面以外的所有頁面
 *   seller  賣家介面：`/seller`（含公開的招募頁 `/seller/pricing`，受眾是想開店的賣家）、`/seller-center`、
 *           `/seller-product-import-preview` 底下（見 paths.ts 的 surfaceOfPath）
 * 兩支 App 各有自己的清單（buyer-app、seller-app，不在這裡）；共用文案目錄 `catalog/config.json` 的
 * `locales` 必須包含這裡所有已發佈的語系（I-CI 檢查）。
 *
 * 這個檔案只能用相對路徑 import：`next.config.ts` 在 Next 的別名生效之前就會載入它。
 */
import { currentFlavor, type Flavor } from "../config/flavor";

export const supportedLocales = ["zh-TW", "en"] as const;
export type AppLocale = (typeof supportedLocales)[number];

/** 預設語系：URL 沒有前綴時的語系（`as-needed`），也是 hreflang 的 `x-default`。 */
export const defaultLocale = "zh-TW" satisfies AppLocale;

/** 網站有兩個會各自發佈語系的介面。 */
export type Surface = "buyer" | "seller";

/** 每個介面已發佈的語系。預設語系永遠在清單裡（`locales.unit.test.ts` 鎖住）。 */
export const publishedLocales = {
	buyer: ["zh-TW"],
	seller: ["zh-TW"],
} as const satisfies Record<Surface, readonly AppLocale[]>;

/**
 * 發佈任何預設語系以外的語系之前，還沒做完的事。清單非空時，`publishedLocales` 不能有預設語系以外的
 * 語系（`tests/locale-launch-gate.test.ts` 擋）；做完一項刪一項，說明與先後在計畫
 * （backend/docs/plans/2026-09-29-a3-buyer-seller.md，I-web-0 的「發佈 en 之前」）。
 */
export const LAUNCH_BLOCKERS: readonly string[] = [
	"語言切換器（設計要 Will 同意；網址不依瀏覽器語言導向，切換器是讀者進 /en 的唯一入口）",
	"/en 之下不存在的網址回語系正確的 404（目前回預設語系的 global-not-found）",
	"該介面在該語系下沒有未翻譯的文字，含 lib/seo/metadata.ts 的 EN_IDENTITY.description（ADR 發佈門檻）",
];

let publishedOverride: Record<Surface, readonly AppLocale[]> | null = null;

/** 這個介面已發佈的語系。讀已發佈清單一律經這裡，測試才換得掉。 */
export function publishedFor(surface: Surface): readonly AppLocale[] {
	return (publishedOverride ?? publishedLocales)[surface];
}

/** Test-only override of the published lists. Avoid in app code. */
export function __setPublishedLocalesForTest(
	next: Record<Surface, readonly AppLocale[]> | null,
): void {
	publishedOverride = next;
}

export function isAppLocale(locale: unknown): locale is AppLocale {
	return (
		typeof locale === "string" &&
		(supportedLocales as readonly string[]).includes(locale)
	);
}

/**
 * 這個部署在某個介面上會路由的語系：dev flavor 路由全部支援的語系（未發佈的也要能測），
 * 其餘 flavor 只路由已發佈的。`NEXT_PUBLIC_FLAVOR` 在建置時寫死，edge middleware 與 server 看到同一個值。
 */
export function routableLocales(
	surface: Surface,
	flavor: Flavor = currentFlavor(),
): readonly AppLocale[] {
	return flavor === "dev" ? supportedLocales : publishedFor(surface);
}

/** router 要認得的語系：各介面會路由的語系的聯集（維持 `supportedLocales` 的順序）。 */
export function allRoutableLocales(
	flavor: Flavor = currentFlavor(),
): readonly AppLocale[] {
	const surfaces = Object.keys(publishedLocales) as Surface[];
	return supportedLocales.filter((locale) =>
		surfaces.some((surface) =>
			routableLocales(surface, flavor).includes(locale),
		),
	);
}

/**
 * URL 上的語系段落（`[locale]`）若是這個部署會路由的語系就回它，否則 null（呼叫端回 404）。
 * 比 `isAppLocale` 嚴：prod 的 `en` 是支援的語系，但沒有發佈，不能被渲染。
 */
export function resolveRoutableLocale(
	raw: unknown,
	flavor: Flavor = currentFlavor(),
): AppLocale | null {
	return isAppLocale(raw) && allRoutableLocales(flavor).includes(raw)
		? raw
		: null;
}
