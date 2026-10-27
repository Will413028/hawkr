import { DELEGATED_OPERATION_MAX_TTL_SECONDS } from "../state/session";

/**
 * 代行 credential cookie（56 號 §4.5 L1001–L1009，由 `hawkr-admin` 的
 * `server/session.ts` 搬移；**Max-Age 算式一字不改**）。
 *
 * 兩處刻意與 ADM 那份不同：
 * - **名稱不同名**：同名會讓「哪一個 host 的哪一份」在除錯時分不出來。
 * - **`Path=/adm`，因此不用 `__Host-`**：`__Host-` 強制 `Path=/`，會讓憑證跟著 www
 *   的**每一個**請求（公開商品頁、賣家中心、靜態路由）一起送出。「單獨持有即可
 *   操作」的識別碼必須把暴露面壓到最小，`Path=/adm` 做得到而 `__Host-` 做不到。
 *
 * 影子 cookie 風險（`__Secure-` 不禁 `Domain`，子網域可寫同名值）是**接受**的：
 * credential 沒有任何本地授權力，每一次使用都由 Gateway 重新驗證 ⇒ 影子值的後果
 * 是 9261／9270（fail-closed），不是越權。
 *
 * ⚠️ 全 repo 只有 `/adm/enter` 與 `/adm/api/session/end` 能寫這個 cookie 名
 * （`tests/delegated-operation-session.test.ts` 守著）。
 */

export const PRODUCTION_DELEGATED_COOKIE_NAME =
	"__Secure-hk_delegated_operation" as const;
export const LOCAL_DELEGATED_COOKIE_NAME = "hk_delegated_operation" as const;
export const DELEGATED_COOKIE_NAMES = [
	PRODUCTION_DELEGATED_COOKIE_NAME,
	LOCAL_DELEGATED_COOKIE_NAME,
] as const;

export const DELEGATED_COOKIE_PATH = "/adm" as const;

type CookieGetter = { get(name: string): { value: string } | undefined };

export interface DelegatedCookieOptions {
	httpOnly: true;
	secure: boolean;
	sameSite: "lax";
	path: typeof DELEGATED_COOKIE_PATH;
	maxAge: number;
}

export function getDelegatedCookieName(
	nodeEnv: string | undefined = process.env.NODE_ENV,
): (typeof DELEGATED_COOKIE_NAMES)[number] {
	return nodeEnv === "production"
		? PRODUCTION_DELEGATED_COOKIE_NAME
		: LOCAL_DELEGATED_COOKIE_NAME;
}

export function readDelegatedCookie(
	store: CookieGetter,
	nodeEnv: string | undefined = process.env.NODE_ENV,
): { name: (typeof DELEGATED_COOKIE_NAMES)[number]; value: string } | null {
	const name = getDelegatedCookieName(nodeEnv);
	const value = store.get(name)?.value;
	return isSafeOperationCredential(value) ? { name, value } : null;
}

function hasAsciiControlCharacters(value: string): boolean {
	for (let index = 0; index < value.length; index += 1) {
		const code = value.charCodeAt(index);
		if (code < 0x20 || code === 0x7f) return true;
	}
	return false;
}

export function isSafeOperationCredential(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.length >= 16 &&
		value.length <= 8192 &&
		!hasAsciiControlCharacters(value) &&
		!value.includes(" ") &&
		!value.includes(";") &&
		!value.includes(",")
	);
}

/** 只有 canonical 的 UTC RFC 3339 時間戳能跨過這道信任邊界。 */
export function parseDelegatedExpiry(value: unknown): string | null {
	if (
		typeof value !== "string" ||
		!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/u.test(value)
	) {
		return null;
	}
	const epoch = Date.parse(value);
	if (!Number.isFinite(epoch)) return null;
	const canonicalInput = value.includes(".")
		? value.replace(
				/\.(\d{1,3})Z$/u,
				(_match, fraction: string) => `.${fraction.padEnd(3, "0")}Z`,
			)
		: value.replace(/Z$/u, ".000Z");
	return new Date(epoch).toISOString() === canonicalInput ? value : null;
}

/**
 * cookie 必須活得**比工作階段久一點**，而不是短一點（餘裕放在後面）。
 *
 * 早死一秒的後果不是「早一秒被踢出去」，而是**換一種錯誤**：
 * credential cookie 不在 ⇒ BFF 的 `GET current` 回 `401 CREDENTIAL_MISSING`（一個
 * **字串**碼，對不上 `DELEGATED_TERMINAL_BIZ_CODES` 的數字表）⇒ 工作區判成 `invalid`
 * 「目前無法確認代行工作階段」，而不是 `expired`「代行工作階段已到期」。dev 實測
 * cookie 比 `expiresAt` 早死 3.3 秒，於是 `labels.ts` 的 expired 那段文案**從來沒有
 * 出現過**，它是死碼。
 *
 * 反過來多活一分鐘沒有任何授權風險：credential 在本地不具任何授權力，每一次使用都由
 * Gateway 重新驗證，過期後一律 9270（fail-closed）—— 那正是我們要的那個碼。
 *
 * 30 秒不夠：工作階段狀態靠 30 秒一次的輪詢確認，餘裕短於一輪就可能整輪都沒問到。
 * 取 60 秒＝與 `DELEGATED_OPERATION_CLOCK_SKEW_SECONDS` 同一份時鐘誤差預算（cookie
 * 的 `Max-Age` 由**瀏覽器**的鐘計時，`expiresAt` 由**後端**的鐘決定，兩者本來就是跨鐘
 * 比較），同時容納傳輸延遲與 `Math.ceil` 之外的零頭。
 */
export const DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS = 60;

export function createDelegatedCookieOptions(
	expiresAt: string,
	now: number = Date.now(),
	nodeEnv: string | undefined = process.env.NODE_ENV,
	maxAgeSeconds: number = DELEGATED_OPERATION_MAX_TTL_SECONDS,
): DelegatedCookieOptions | null {
	if (
		!parseDelegatedExpiry(expiresAt) ||
		!Number.isFinite(now) ||
		!Number.isSafeInteger(maxAgeSeconds) ||
		maxAgeSeconds <= 0 ||
		maxAgeSeconds > DELEGATED_OPERATION_MAX_TTL_SECONDS
	) {
		return null;
	}
	// `ceil` 而不是 `floor`：截掉的那不到一秒全部落在「cookie 比工作階段早死」那一側。
	const remaining = Math.ceil((Date.parse(expiresAt) - now) / 1000);
	if (remaining <= 0) return null;
	// 上界仍然是工作階段本身的壽命（`maxAgeSeconds`／30 分鐘硬頂），grace 加在最後：
	// 它是「不要早死」的餘裕，不是把 credential 的可用期拉長 —— 那一段時間裡每一次
	// 呼叫都會被 Gateway 以 9270 擋掉。
	return {
		httpOnly: true,
		secure: nodeEnv === "production",
		sameSite: "lax",
		path: DELEGATED_COOKIE_PATH,
		maxAge:
			Math.min(remaining, maxAgeSeconds, DELEGATED_OPERATION_MAX_TTL_SECONDS) +
			DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS,
	};
}

export function clearDelegatedCookieOptions(
	nodeEnv: string | undefined = process.env.NODE_ENV,
): DelegatedCookieOptions {
	return {
		httpOnly: true,
		secure: nodeEnv === "production",
		sameSite: "lax",
		path: DELEGATED_COOKIE_PATH,
		maxAge: 0,
	};
}

/**
 * 只有伺服器**明確證明**工作階段已終止，才可以清掉 credential cookie。
 *
 * 狀態碼本身不是證明：一個籠統的 401 可能只是上游拒絕，一個籠統的 404 可能是
 * 端點沒部署。要嘛是成功 body（`{"ended": true}`），要嘛是窄白名單內的終態業務碼。
 *
 * ⚠️ gateway 的 `DELETE current` 在「已 ended／expired／revoked」與「credential
 * 無法解析」時**都回 200 `{"ended": true}`**（冪等，L2803／L2805）——回 401／404
 * 會讓前端判定「結果不明」，把工作區寫入永久 latch 封鎖。所以下面的終態碼分支
 * 是防禦性的第二條路，不是主要路徑。
 */
const TERMINAL_BIZ_CODES = new Set([9270, 9271, 9272, 9277]);

export function isDelegatedEndProven(
	status: number,
	payload: unknown,
): boolean {
	if (status === 204) return true;
	if (typeof payload !== "object" || payload === null) return false;
	const envelope = payload as Record<string, unknown>;
	if (status === 200 && envelope.code === 0) {
		const data = envelope.data;
		return (
			typeof data === "object" &&
			data !== null &&
			(data as Record<string, unknown>).ended === true
		);
	}
	return (
		typeof envelope.code === "number" && TERMINAL_BIZ_CODES.has(envelope.code)
	);
}

/** 序列化成 `Set-Cookie`。route handler 直接寫 header，不經 `cookies()`。 */
export function serializeDelegatedCookie(
	name: (typeof DELEGATED_COOKIE_NAMES)[number],
	value: string,
	options: DelegatedCookieOptions,
): string {
	const parts = [
		`${name}=${value}`,
		`Path=${options.path}`,
		`Max-Age=${options.maxAge}`,
		"HttpOnly",
		"SameSite=Lax",
	];
	if (options.secure) parts.push("Secure");
	return parts.join("; ");
}
