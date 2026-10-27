/**
 * 目前登入的帳號與登出：hawkr.identity.v1alpha1（ADR 2026-09-27-identity-clerk）。
 *
 * 登入、登出與 session 由 Clerk 處理；單體以 Clerk 的 session token 認人，帳號在第一個認證過的請求時
 * 建立。每個帳號都是買家（契約的 `roles` 只列另外申請的 ROLE_SELLER），所以 client 只記「是不是賣家」。
 * 帳號 ID 與是否賣家由 GetSession 讀取；買家或賣家工作區是 client 的狀態（lib/auth/active-role.ts）。
 */

import { isClerkAPIResponseError } from "@clerk/nextjs/errors";
import {
	Code,
	ConnectError,
	createClient,
	createContextValues,
} from "@connectrpc/connect";
import {
	type Account,
	DeleteAccountResponse_DeletionState,
	IdentityAppService,
	Role,
} from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { trackLogin } from "@/lib/analytics/events";
import {
	authedTransport,
	SUPPRESS_AUTH_EXPIRED,
} from "@/lib/connect/transport";
import type { Identity } from "../auth/storage";
import { fetchMyStore } from "../seller/repo";
import { isStoreMissing, needsOnboarding } from "../seller/store";

/**
 * fetchSession could not determine the identity because the backend was
 * temporarily unavailable (5xx / network), NOT because the session is invalid.
 * Callers MUST keep the existing login state on this result (B-199) — a
 * momentary outage must never look like a logout.
 */
export const SESSION_UNAVAILABLE = Symbol("session-unavailable");
export type SessionResult = Identity | null | typeof SESSION_UNAVAILABLE;

/** 登入方式：Clerk 的 OAuth strategy 去掉 `oauth_` 前綴。 */
export type LoginProvider = "google" | "apple" | "line";

export type OAuthLoginResult = Identity & {
	/**
	 * 賣家是否還沒完成開店：由 GetMyStore 判斷。沒有賣家角色是 true；查不到時是 undefined，
	 * 呼叫端 fail-closed 送進 onboarding。
	 */
	needsOnboarding: boolean | undefined;
};

function identityApp() {
	return createClient(IdentityAppService, authedTransport());
}

function toIdentity(account: Account): Identity {
	return {
		accountId: account.id,
		isSeller: account.roles.includes(Role.SELLER),
	};
}

/** 帳號有賣家角色時向 GetMyStore 確認開店是否完成（同開店精靈的入口判斷）。 */
async function withOnboarding(id: Identity): Promise<OAuthLoginResult> {
	if (!id.isSeller) return { ...id, needsOnboarding: true };
	try {
		return { ...id, needsOnboarding: needsOnboarding(await fetchMyStore()) };
	} catch (e) {
		// 只有確認沒有店家才是 true；暫時失敗是「不知道」，呼叫端 fail-closed。
		return {
			...id,
			needsOnboarding: isStoreMissing(e) ? true : undefined,
		};
	}
}

/** 讀取登入的帳號時帶上：「沒有登入」由呼叫端自己處理，不廣播 AUTH_EXPIRED_EVENT。 */
function suppressAuthExpired() {
	return createContextValues().set(SUPPRESS_AUTH_EXPIRED, true);
}

/**
 * Clerk 完成 OAuth 之後的第一步：以 GetSession 讀取帳號（第一次登入時伺服器同時建立帳號），
 * 再向 GetMyStore 確認開店。
 *
 * 伺服器不認這個 session（unauthenticated：帳號已停權或刪除）或答不出來時 throw 那個 ConnectError，
 * 由呼叫端決定是否結束 Clerk 的 session。
 */
export async function completeSignIn(
	provider: LoginProvider | null,
): Promise<OAuthLoginResult> {
	let account: Account | undefined;
	try {
		const res = await identityApp().getSession(
			{},
			{ contextValues: suppressAuthExpired() },
		);
		account = res.account;
	} catch (e) {
		throw ConnectError.from(e);
	}
	if (!account?.id) {
		throw new ConnectError("GetSession 的回應沒有帳號", Code.Internal);
	}
	const id = toIdentity(account);
	if (provider) trackLogin(provider);
	return withOnboarding(id);
}

/**
 * GetSession resolves the signed-in account: 401 returns null, 403 throws the
 * server rejection, and every other failure returns SESSION_UNAVAILABLE.
 */
export async function fetchSession(): Promise<SessionResult> {
	let account: Account | undefined;
	try {
		const res = await identityApp().getSession(
			{},
			{ contextValues: suppressAuthExpired() },
		);
		account = res.account;
	} catch (e) {
		// 代行工作區的護欄拋的不是 ConnectError：同樣是「答不出來」。
		const err = ConnectError.from(e);
		if (err.code === Code.Unauthenticated) return null;
		if (err.code === Code.PermissionDenied) throw err;
		return SESSION_UNAVAILABLE;
	}
	if (!account?.id) {
		return SESSION_UNAVAILABLE;
	}
	return toIdentity(account);
}

/**
 * The session was NOT ended. Callers MUST keep the login state: Clerk still
 * holds the session, so anything that clears the local identity would produce
 * a user who believes they are signed out while the session keeps resolving
 * everywhere else.
 *
 * `reason` only selects the wording shown to the user — both are the same
 * failure, and neither may clear a single byte of local state:
 *  - "server": Clerk answered and refused.
 *  - "network": the request never got an answer.
 */
export class LogoutRevokeFailedError extends Error {
	public reason: "server" | "network";
	constructor(reason: "server" | "network", message = "登出失敗") {
		super(message);
		this.name = "LogoutRevokeFailedError";
		this.reason = reason;
	}
}

/** 結束 Clerk session；成功後由 AuthProvider 清除帳號與私有快取。 */
export async function logout(
	endClerkSession: () => Promise<void>,
): Promise<void> {
	try {
		await endClerkSession();
	} catch (error) {
		if (isClerkAPIResponseError(error)) {
			throw new LogoutRevokeFailedError("server");
		}
		// Network-layer failure (offline / flaky mobile). Reported as a warning
		// rather than an error because it is a connectivity fact, not a defect —
		// it is the one signal for how often people try to sign out with no line.
		// Lazy import（比照 lib/api/client.ts reportApiFailure）：build/test 情境
		// Sentry 可能缺席或只部分 mock；.catch 吞掉 import 失敗，觀測絕不能反過
		// 來讓登出中斷。訊息只取網路層錯誤字串（"Load failed" 類）並截斷。
		const message = error instanceof Error ? error.message : String(error);
		void import("@sentry/nextjs")
			.then((Sentry) => {
				Sentry.captureMessage?.("logout revoke failed (network)", {
					level: "warning",
					fingerprint: ["logout-revoke-network-failure"],
					extra: { message: message.slice(0, 200) },
				});
			})
			.catch(() => {});
		throw new LogoutRevokeFailedError("network");
	}
}

/**
 * 刪除登入的帳號。沒有未完成的訂單時立即刪除（DELETED，呼叫端登出）；有的話進入刪除流程，最後一筆
 * 訂單完成後自動刪除（PENDING_ORDERS）。帳號同時是賣家時回 failed_precondition：要先關閉店家。
 *
 * 刪除之後 token 不再有效，所以不自動重送：沒收到回應（unavailable、deadline_exceeded、網路錯誤）時以
 * GetSession 確認，unauthenticated 表示已刪除，回 DELETED；仍在登入就把原本的錯誤丟出去。
 */
export async function deleteAccount(): Promise<DeleteAccountResponse_DeletionState> {
	try {
		const res = await identityApp().deleteAccount({});
		return res.state;
	} catch (e) {
		if (!NO_RESPONSE_CODES.has(ConnectError.from(e).code)) throw e;
		if ((await fetchSession()) === null) {
			return DeleteAccountResponse_DeletionState.DELETED;
		}
		throw e;
	}
}

const NO_RESPONSE_CODES: ReadonlySet<Code> = new Set([
	Code.Unavailable,
	Code.DeadlineExceeded,
	Code.Unknown,
]);
