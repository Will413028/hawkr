import { sellerPublicClient } from "@/lib/connect/seller-public";
import { defaultLocale } from "@/lib/intl/locales";
import { isStoreAbsentError, isStoreCodeShape } from "./public-store";

/**
 * 店面網址的 request 判斷，給 `middleware.ts`：`/sellers/{store_code}` 底下的頁面是靜態預渲染，
 * 預渲染的回應沒有地方放狀態碼，頁面裡的 `notFound()` 會回 404 內容卻是 200，所以真正的 404 只能由
 * middleware 決定。只有 `GetStore` 回 not_found（店面不存在或不公開，兩者回應相同）
 * 才是 404，頁面不透露原因；invalid_argument 與其餘錯誤（5xx、逾時、連線失敗）都不是「沒有這家店」，
 * 屬於不明，放行讓頁面自己渲染。結果不快取：店面可能隨時停權或下架，快取會讓剛變成不公開的店面
 * 在快取期間通過 middleware，頁面以 200 渲染 not-found 內容。
 */

const STORE_PATH = /^\/sellers\/([^/]+)(\/.*)?$/;

/** 在等著文件的訪客前面做的查詢，預算遠低於 `apiTimeoutMs`：放行是可接受的退路，卡住的 middleware 不是。 */
const LOOKUP_TIMEOUT_MS = 1_500;

export type StoreLookup =
	| { kind: "present"; storeId: string }
	| { kind: "absent" }
	/** 沒有可用的答案：5xx、逾時、連線失敗。 */
	| { kind: "unknown" };

export type StorefrontOutcome = { kind: "pass" } | { kind: "not-found" };

export function parseStorefrontPath(
	pathname: string,
): { storeCode: string; tail: string } | null {
	const match = STORE_PATH.exec(pathname);
	return match ? { storeCode: match[1], tail: match[2] ?? "" } : null;
}

/** 以 `GetStore` 問網址上的 store_code。 */
export async function lookupStore(
	storeCode: string,
	visitorIp?: string | null,
): Promise<StoreLookup> {
	try {
		const res = await sellerPublicClient({
			locale: defaultLocale,
			visitorIp: visitorIp ?? undefined,
		}).getStore({ storeCode }, { timeoutMs: LOOKUP_TIMEOUT_MS });
		const storeId = res.store?.id ?? "";
		if (!storeId) return { kind: "unknown" };
		return { kind: "present", storeId };
	} catch (err) {
		return isStoreAbsentError(err) ? { kind: "absent" } : { kind: "unknown" };
	}
}

/** 決定店面請求：形狀不對或沒有這家店回 404，其餘放行。 */
export async function resolveStorefrontRequest(
	pathname: string,
	lookup: (storeCode: string) => Promise<StoreLookup> = lookupStore,
): Promise<StorefrontOutcome> {
	const parsed = parseStorefrontPath(pathname);
	if (!parsed) return { kind: "pass" };
	if (!isStoreCodeShape(parsed.storeCode)) return { kind: "not-found" };
	const result = await lookup(parsed.storeCode);
	return result.kind === "absent" ? { kind: "not-found" } : { kind: "pass" };
}
