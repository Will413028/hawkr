import { sellerPublicClient } from "@/lib/connect/seller-public";
import { defaultLocale } from "@/lib/intl/locales";
import { isStoreAbsentError, isStoreCodeShape } from "./public-store";

/**
 * 店面網址的 request 判斷，給 `middleware.ts`：`/sellers/{store_code}` 底下的頁面是靜態預渲染，
 * 預渲染的回應沒有地方放狀態碼，頁面裡的 `notFound()` 會回 404 內容卻是 200，所以真正的 404 只能由
 * middleware 決定。只有 `GetStore` 回 not_found（店面不存在或不公開，兩者回應相同）
 * 才是 404，頁面不透露原因；invalid_argument 與其餘錯誤（5xx、逾時、連線失敗）都不是「沒有這家店」，
 * 屬於不明，放行讓頁面自己渲染。只記「找到了」的答案，TTL 見 `PRESENT_TTL_MS`：每個店面請求都擋一次
 * GetStore 太貴，代價是剛變成不公開的店面最多 60 秒內仍通過 middleware，頁面以 200 渲染 not-found 內容。
 * not_found 與錯誤都不記。
 */

const STORE_PATH = /^\/sellers\/([^/]+)(\/.*)?$/;

/** 在等著文件的訪客前面做的查詢，預算遠低於 `apiTimeoutMs`：放行是可接受的退路，卡住的 middleware 不是。 */
const LOOKUP_TIMEOUT_MS = 1_500;

/**
 * Bounded staleness: a store or thread that turns non-public may still pass as present for up
 * to this long (served as a 200 not-found body), traded against one blocking backend call per request.
 */
export const PRESENT_TTL_MS = 60_000;
const MEMO_MAX_ENTRIES = 500;

/** Remembers only backend-confirmed "present" answers, per isolate, each for `PRESENT_TTL_MS`. */
export function createPresentMemo<V>() {
	const entries = new Map<string, { value: V; at: number }>();
	return {
		get(key: string): V | undefined {
			const hit = entries.get(key);
			if (!hit) return undefined;
			if (Date.now() - hit.at < PRESENT_TTL_MS) return hit.value;
			entries.delete(key);
			return undefined;
		},
		set(key: string, value: V): void {
			if (entries.size >= MEMO_MAX_ENTRIES) entries.clear();
			entries.set(key, { value, at: Date.now() });
		},
		delete(key: string): void {
			entries.delete(key);
		},
		clear(): void {
			entries.clear();
		},
	};
}

const presentStoreMemo = createPresentMemo<string>();

export function __resetStorefrontGuardMemoForTest(): void {
	presentStoreMemo.clear();
}

export type StoreLookup =
	| { kind: "present"; storeId: string }
	| { kind: "absent" }
	/** 沒有可用的答案：5xx、逾時、連線失敗。 */
	| { kind: "unknown" };

export type StorefrontOutcome =
	/** `storeId` is set when the lookup confirmed the store, so later checks need no second lookup. */
	{ kind: "pass"; storeId?: string } | { kind: "not-found" };

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
	const memoised = presentStoreMemo.get(storeCode);
	if (memoised) return { kind: "present", storeId: memoised };
	try {
		const res = await sellerPublicClient({
			locale: defaultLocale,
			visitorIp: visitorIp ?? undefined,
		}).getStore({ storeCode }, { timeoutMs: LOOKUP_TIMEOUT_MS });
		const storeId = res.store?.id ?? "";
		if (!storeId) return { kind: "unknown" };
		presentStoreMemo.set(storeCode, storeId);
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
	if (result.kind === "absent") return { kind: "not-found" };
	return result.kind === "present"
		? { kind: "pass", storeId: result.storeId }
		: { kind: "pass" };
}
