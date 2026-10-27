import { getEnv } from "@/lib/config/env";

/**
 * 代行 BFF 的入站防護（56 號 §4.5 L1045）。
 *
 * 兩件事，判準不同：
 * 1. **CSRF**：非 GET 一律要求 `X-Requested-With: XMLHttpRequest` **且** `Origin`
 *    等於自身 web origin。cookie 是 `SameSite=Lax` —— 它擋得住跨站 POST，但擋不住
 *    跨站 top-level GET 導航；反過來說**所有會改變狀態的代行端點一律非 GET**，
 *    `/adm/enter` 是唯一的 GET 例外而它是單次的。
 * 2. **保留 header**：瀏覽器送來即視為攻擊嘗試，**拒絕而不是靜默丟棄** —— 靜默
 *    丟棄會讓「有人在試」這件事沒有任何一層看得見。
 *
 * ⚠️ **不得用 `Referer` 兜底**：它可被 `Referrer-Policy` 完全抑制（§4.5 的 handoff
 * 流程自己就在用 `Referrer-Policy: no-referrer`）⇒ 兜底要嘛在正常流程下把合法請求
 * 擋掉，要嘛被迫放行「Referer 缺席」而使檢查形同虛設。
 *
 * ⚠️ **不得用 `new URL(request.url).origin` 判自身 origin**（本 repo 唯一的既有樣本
 * `app/api/public/seller-leads/route.ts` 就是這樣寫的）：這支 BFF 跑在 Cloudflare
 * Workers 上，`request.url` 的 host 未必等於使用者看到的那個 origin。權威來源是
 * `getEnv().webOrigin`（取自 `NEXT_PUBLIC_WEB_ORIGIN`，prod 必填）。
 */

/**
 * 保留 header：這些名字的值一律由伺服器自己決定。
 *
 * `x-hk-origin-auth` 由本進程自簽（限流主體的宣告），瀏覽器送來的那一份是「被計費者
 * 自己宣稱要計給誰」；`x-hk-operation-session` 由 cookie 轉出，讓瀏覽器直接指定等於
 * 把 httpOnly 這層拆掉；其餘是 Gateway 的身分與代行語境欄位。
 */
const RESERVED_EXACT = new Set([
	"x-hk-operation-session",
	"x-hk-origin-auth",
	"x-account-id",
	"x-account-type",
	"x-admin-actor-id",
	"x-delegated-store-id",
	"x-impersonated-user-id",
	"x-delegation-session-id",
	"x-delegation-capability",
	"x-delegation-audit-id",
	"x-acting-store-id",
]);

const RESERVED_PREFIXES = ["x-hk-operation-", "x-delegated-operation-"];

/** 瀏覽器唯一被允許影響上游 header 的一項（§6.1.0 共通四欄）。 */
export const IDEMPOTENCY_KEY_HEADER = "Idempotency-Key";

/** 與 Gateway `delegatedIdempotencyKeyRE` 同一份值域，不另立第二套。 */
const IDEMPOTENCY_KEY_PATTERN = /^[A-Za-z0-9._:-]{8,128}$/;

export type DelegatedRejectionCode =
	| "CSRF_REJECTED"
	| "RESERVED_HEADER"
	| "CREDENTIAL_MISSING"
	| "INVALID_PATH_PARAM"
	| "INVALID_QUERY"
	| "INVALID_BODY"
	| "INVALID_IDEMPOTENCY_KEY"
	| "TARGET_MISMATCH";

/**
 * BFF 自己產生的拒絕。碼刻意是**字串**：Gateway 的業務碼是 int32，兩者永遠不會
 * 被前端誤認為同一族（前端對 9270／9271／9272 有終態處置，那三個碼只能來自後端）。
 */
export class DelegatedRequestRejectedError extends Error {
	readonly code: DelegatedRejectionCode;
	readonly status: number;

	constructor(code: DelegatedRejectionCode, status: number) {
		super(`delegated request rejected (${code})`);
		this.name = "DelegatedRequestRejectedError";
		this.code = code;
		this.status = status;
	}
}

function isReservedHeader(name: string): boolean {
	const lower = name.toLowerCase();
	return (
		RESERVED_EXACT.has(lower) ||
		RESERVED_PREFIXES.some((prefix) => lower.startsWith(prefix))
	);
}

/**
 * 本機開發用的 origin。
 *
 * `pnpm dev` 在 `http://localhost:3000` 提供服務，但本機也可能用別的
 * loopback 名稱或把 `NEXT_PUBLIC_WEB_ORIGIN` 設成部署用的值 —— 不放行就等於整個
 * 代行工作區在本機無法測試。
 *
 * **只在非 production 生效**：production 的判定材料只有 `webOrigin` 一個。
 */
function isLocalDevelopmentOrigin(origin: string): boolean {
	if (process.env.NODE_ENV === "production") return false;
	let hostname: string;
	let protocol: string;
	try {
		const url = new URL(origin);
		hostname = url.hostname.toLowerCase();
		protocol = url.protocol;
	} catch {
		return false;
	}
	if (protocol !== "http:" && protocol !== "https:") return false;
	return (
		hostname === "localhost" || hostname === "127.0.0.1" || hostname === "[::1]"
	);
}

/** 非 GET 的同源檢查。GET／HEAD 不適用（它們不改變狀態）。 */
export function assertSameOriginMutation(request: Request): void {
	if (request.method === "GET" || request.method === "HEAD") return;
	if (request.headers.get("x-requested-with") !== "XMLHttpRequest") {
		throw new DelegatedRequestRejectedError("CSRF_REJECTED", 403);
	}
	const origin = request.headers.get("origin");
	if (!origin) throw new DelegatedRequestRejectedError("CSRF_REJECTED", 403);
	if (origin !== getEnv().webOrigin && !isLocalDevelopmentOrigin(origin)) {
		throw new DelegatedRequestRejectedError("CSRF_REJECTED", 403);
	}
}

/** 保留 header 出現即拒絕（含 GET —— 偽造身分不分方法）。 */
export function assertNoReservedHeaders(request: Request): void {
	for (const [name] of request.headers) {
		if (isReservedHeader(name)) {
			throw new DelegatedRequestRejectedError("RESERVED_HEADER", 400);
		}
	}
}

/**
 * 讀出瀏覽器的 `Idempotency-Key`。
 *
 * 值域與 Gateway 同一份，但**必填與否由 Gateway 決定**：BFF 不自己判斷哪支端點需要
 * 它，更不得代為生成一把 —— 生成的那把每次重送都不一樣，冪等就只剩名字。
 */
export function readIdempotencyKey(request: Request): string | null {
	const key = request.headers.get(IDEMPOTENCY_KEY_HEADER);
	if (key === null) return null;
	if (!IDEMPOTENCY_KEY_PATTERN.test(key)) {
		throw new DelegatedRequestRejectedError("INVALID_IDEMPOTENCY_KEY", 400);
	}
	return key;
}

/** 每一支代行 BFF route 的第一道動作。 */
export function assertDelegatedBrowserRequest(request: Request): void {
	assertNoReservedHeaders(request);
	assertSameOriginMutation(request);
}
