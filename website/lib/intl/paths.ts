/**
 * 路徑與語系前綴的純函式：middleware、next.config、SEO 與驗證邏輯共用。
 *
 * URL 是網站唯一的語系來源（ADR 2026-10-17 D1）：預設語系 zh-TW 在 `/…`，其他語系在 `/<locale>/…`
 * （next-intl 的 `as-needed`）。這裡的 pathname 都不含 query；回傳的路徑一律以 `/` 開頭。
 *
 * 只能用相對路徑 import（`next.config.ts` 會載入它）。
 */
import {
	type AppLocale,
	allRoutableLocales,
	defaultLocale,
	routableLocales,
	type Surface,
	supportedLocales,
} from "./locales";

/**
 * 不本地化的路徑：沒有語系前綴，也不交給 next-intl（交過去會被改寫成 `/zh-TW/…` 而 404）。
 * 每一項都有理由，新增前先確認它真的不屬於 `app/[locale]`。
 *
 * 子樹（底下真的有整片路由，連同子路徑一起放行）：
 *   /api                 route handlers（含第一方反代 /api/connect、/api/v1、/api/auth）
 *   /adm                 代行工作區：staff 介面，只有 zh-TW，有自己的根 layout（app/(delegated)）
 *   /.well-known         平台驗證檔（next.config 的 rewrite 轉到 /api/apple-pay/…）
 *
 * 單一路徑（底下沒有東西；`/sentinel/account/orders` 這種子路徑不是它，要照一般路徑處理，
 * 否則會被放行到 `[locale]=sentinel` 底下去渲染頁面）：
 *   /sentinel            快取金絲雀的探針（基礎設施，沒有讀者）
 *   /store-map/return    PayUni 超商地圖的回呼 route handler，網址由第三方設定
 *   /apple-icon          根層的圖示 metadata route，沒有文字
 *   /opengraph-image     根層的分享圖 metadata route，網址由 SOCIAL_IMAGE_PATH 明寫
 *   /apple-app-site-association  public/ 的 iOS universal links 檔，沒有副檔名（上線時由 assets 直接回應，
 *                        next start 與 dev 要靠這一項才找得到它）
 *
 * robots.txt、sitemap.xml、manifest.webmanifest、icon.svg 有副檔名，middleware 的 matcher 明列排除。
 */
export const NON_LOCALIZED_PREFIXES = ["/api", "/adm", "/.well-known"] as const;

export const NON_LOCALIZED_EXACT = [
	"/sentinel",
	"/store-map/return",
	"/apple-icon",
	"/opengraph-image",
	"/apple-app-site-association",
] as const;

/**
 * 賣家介面的路徑前綴；其餘一律算買家頁。以路徑段落比對（`/seller` 不含 `/sellers`、`/seller-guide`）。
 * `/seller`（含 `/seller/pricing`、`/seller/onboarding`）是公開的賣家招募頁，但受眾是想開店的賣家，所以歸賣家介面：
 * 它們的英文版跟賣家介面一起發佈；買家看的賣家資訊在 `/sellers/*`，歸買家頁。
 */
const SELLER_SURFACE_PREFIXES = [
	"/seller",
	"/seller-center",
	"/seller-product-import-preview",
] as const;

function underPrefix(pathname: string, prefix: string): boolean {
	return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

export function isNonLocalizedPath(pathname: string): boolean {
	return (
		(NON_LOCALIZED_EXACT as readonly string[]).includes(pathname) ||
		NON_LOCALIZED_PREFIXES.some((prefix) => underPrefix(pathname, prefix))
	);
}

/**
 * `public/` 的頂層目錄：裡面是檔案，不是頁面。middleware 的 matcher 排除同一份清單
 * （`tests/middleware-matcher.test.ts` 比對 matcher 與 `public/` 的實際目錄）。
 */
export const PUBLIC_DIRECTORIES = [
	"brand",
	"placeholders",
	"templates",
] as const;

/** 站台根層由 Next 產生的檔案（`app/robots.ts`、`sitemap.ts`、`manifest.ts`、`icon.svg`）。 */
export const GENERATED_ROOT_FILES = [
	"/robots.txt",
	"/sitemap.xml",
	"/manifest.webmanifest",
	"/icon.svg",
] as const;

/** 這個路徑是某個檔案（`public/` 底下的、Next 產生的根層檔案），不是頁面？ */
function isStaticFilePath(pathname: string): boolean {
	return (
		(GENERATED_ROOT_FILES as readonly string[]).includes(pathname) ||
		PUBLIC_DIRECTORIES.some((dir) => pathname.startsWith(`/${dir}/`))
	);
}

/**
 * 這個站內路徑是 `app/[locale]` 的頁面嗎（該有語系前綴）？不是的：不本地化的子樹與單一路徑、`/_next`、
 * 明列的檔案（`public/` 的頂層目錄、Next 產生的根層檔案）。導覽層只替頁面路徑加前綴，`/en/api/…`、
 * `/en/templates/…` 這種網址不存在。
 *
 * 看明列的清單，不看副檔名：頁面路徑的最後一段也可能有點（`/products/1.5`），而
 * `/manifest.webmanifest` 的副檔名比任何合理的長度門檻都長。
 */
export function isLocalizablePagePath(pathname: string): boolean {
	return (
		!isNonLocalizedPath(pathname) &&
		!pathname.startsWith("/_next/") &&
		!isStaticFilePath(pathname)
	);
}

/** 站內的根路徑（`/x`、`/x?q#h`），不含 `//host` 與反斜線這種會被瀏覽器當成別的來源的寫法。 */
export function isRootRelativeHref(href: string): boolean {
	return href.startsWith("/") && !href.startsWith("//") && !href.includes("\\");
}

/**
 * `app/[locale]` 底下由程式產生圖片的 metadata route（網址沒有副檔名，所以 middleware 看得到）。Next 用 route
 * params 組它們的網址，所以語系值會被寫進 HTML，預設語系也不例外（賣家頁的 `/zh-TW/sellers/9/apple-icon-v673w8`；
 * 位在路由群組底下時最後一段有六碼雜湊）。這些網址原地服務，不為每次抓圖示多一次導向。
 *
 * 只放明確的路徑，不放「名字像圖示」的通則：`/zh-TW/account/orders/icon` 是訂單頁（`[orderNo]` 剛好叫 icon），
 * 通則會讓它繞過登入判斷。新增這類 route 時加在這裡（`tests/metadata-image-routes.test.ts` 會比對實際有哪些）。
 */
const METADATA_IMAGE_PATHS: readonly RegExp[] = [
	/^\/sellers\/\d{1,19}\/apple-icon-[0-9a-z]{6}$/,
];

export function isMetadataImagePath(pathname: string): boolean {
	return METADATA_IMAGE_PATHS.some((pattern) => pattern.test(pathname));
}

/**
 * next-intl 用來路由的路徑：`decodeURI`，再清掉反斜線、tab／換行與連續斜線（它的 `sanitizePathname`）。
 * `/%65n/products/1`、`//en/products/1` 都會被它當成 `/en/products/1` 改寫過去，所以站內的判斷（不本地化的
 * 清單、語系前綴、登入守衛、`/en` 的 noindex）也要看同一個字串，否則換個寫法就繞過去。網址壞掉（無法解碼）回
 * null，由呼叫端交給 Next 回 400。
 */
export function routingPathname(rawPathname: string): string | null {
	let decoded: string;
	try {
		decoded = decodeURI(rawPathname);
	} catch {
		return null;
	}
	return decoded
		.replace(/\\/g, "%5C")
		.replace(/[\t\n\r]/g, "")
		.replace(/\/+/g, "/");
}

/** 去掉語系前綴後的路徑屬於哪個介面。 */
export function surfaceOfPath(pathname: string): Surface {
	return SELLER_SURFACE_PREFIXES.some((prefix) => underPrefix(pathname, prefix))
		? "seller"
		: "buyer";
}

export interface SplitPath {
	/** URL 上明寫的語系；沒有前綴是 null（此時的語系是預設語系）。 */
	locale: AppLocale | null;
	/** 前綴本身，`/en` 或 `""`。 */
	prefix: string;
	/** 去掉前綴的路徑。 */
	pathname: string;
}

/**
 * `/en/account` → `{ locale: "en", prefix: "/en", pathname: "/account" }`。只認得這個部署會路由的語系
 * （prod 的 `/en/…` 不是語系前綴，會原樣當成路徑，最後回 404）。明寫的預設語系前綴（`/zh-TW/…`）也會被拆開，
 * next-intl 再把它導向無前綴的網址。
 */
export function splitLocale(
	rawPathname: string,
	locales: readonly AppLocale[] = allRoutableLocales(),
): SplitPath {
	for (const locale of locales) {
		if (rawPathname === `/${locale}`) {
			return { locale, prefix: `/${locale}`, pathname: "/" };
		}
		if (rawPathname.startsWith(`/${locale}/`)) {
			return {
				locale,
				prefix: `/${locale}`,
				pathname: rawPathname.slice(locale.length + 1),
			};
		}
	}
	return { locale: null, prefix: "", pathname: rawPathname };
}

/** 去掉語系前綴。 */
export function stripLocale(rawPathname: string): string {
	return splitLocale(rawPathname).pathname;
}

/**
 * 內嵌 script 用的 `stripLocale`：script 在還沒有 React 的時候跑，不能 import，所以是 JS 函式的原始碼
 * （`var strip = ${STRIP_LOCALE_SCRIPT}`）。語系清單取自 `supportedLocales`，建置時寫死。
 */
export const STRIP_LOCALE_SCRIPT = `function(p){var l=${JSON.stringify(supportedLocales)};for(var i=0;i<l.length;i++){var x="/"+l[i];if(p===x)return"/";if(p.indexOf(x+"/")===0)return p.slice(x.length)}return p}`;

/** 語系前綴：預設語系沒有，其他是 `/<locale>`。 */
export function localePrefix(locale: AppLocale): string {
	return locale === defaultLocale ? "" : `/${locale}`;
}

/** 把 `/about` 變成該語系的路徑（`/en/about`）；`/` 對非預設語系是 `/en`。 */
export function withLocale(path: string, locale: AppLocale): string {
	if (!path.startsWith("/")) {
		throw new Error(`withLocale: path must start with "/" (got "${path}")`);
	}
	const prefix = localePrefix(locale);
	if (prefix === "") return path;
	return path === "/" ? prefix : `${prefix}${path}`;
}

/** 這個部署在 `pathname`（已去前綴）所屬的介面上，有沒有路由 `locale`。 */
export function isLocaleRoutableFor(
	locale: AppLocale,
	pathname: string,
	routable: (surface: Surface) => readonly AppLocale[] = routableLocales,
): boolean {
	return routable(surfaceOfPath(pathname)).includes(locale);
}
