/**
 * 目前登入的帳號與登出：hawkr.identity.v1alpha1（ADR 2026-10-30-identity-self-hosted-federated-login）。
 *
 * 登入是單體的 `GET /api/auth/{provider}/start`（整頁導向 Google 或 LINE，callback 設 `__Host-hk_session`
 * 之後 302 回來）；憑證是 HttpOnly 的 session cookie，JS 讀不到，所以「有沒有登入」只能問單體：GetSession。
 * 每個帳號都是買家（契約的 `roles` 只列另外申請的 ROLE_SELLER），所以 client 只記「是不是賣家」。
 * 帳號 ID 與是否賣家由 GetSession 讀取；買家或賣家工作區是 client 的狀態（lib/auth/active-role.ts）。
 */

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
import {
	authedTransport,
	SUPPRESS_AUTH_EXPIRED,
} from "@/lib/connect/transport";
import type { Identity } from "../auth/storage";

/**
 * fetchSession could not determine the identity because the backend was
 * temporarily unavailable (5xx / network), NOT because the session is invalid.
 * Callers MUST keep the existing login state on this result (B-199) — a
 * momentary outage must never look like a logout.
 */
export const SESSION_UNAVAILABLE = Symbol("session-unavailable");
export type SessionResult = Identity | null | typeof SESSION_UNAVAILABLE;

/** 登入方式：單體 `/api/auth/{provider}/start` 的 provider 段。 */
export type LoginProvider = "google" | "line";

function identityApp() {
	return createClient(IdentityAppService, authedTransport());
}

function toIdentity(account: Account): Identity {
	return {
		accountId: account.id,
		isSeller: account.roles.includes(Role.SELLER),
	};
}

/** 讀取登入的帳號時帶上：「沒有登入」由呼叫端自己處理，不廣播 AUTH_EXPIRED_EVENT。 */
function suppressAuthExpired() {
	return createContextValues().set(SUPPRESS_AUTH_EXPIRED, true);
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
 * The session was NOT ended. Callers MUST keep the login state: the server
 * still holds the session, so anything that clears the local identity would produce
 * a user who believes they are signed out while the session keeps resolving
 * everywhere else.
 *
 * `reason` only selects the wording shown to the user — both are the same
 * failure, and neither may clear a single byte of local state:
 *  - "server": the monolith answered and refused.
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

/**
 * 登出：呼叫 SignOut RPC，單體撤銷目前的 session，並以 Set-Cookie 清掉 `__Host-hk_session`。session 已經
 * 不存在時伺服器也回成功。成功後由 AuthProvider 清除帳號與私有快取。
 *
 * 單體有回應（任何 ConnectError）是 "server"；沒收到回應（離線、逾時）是 "network"，兩者都沒有結束 session。
 */
export async function logout(): Promise<void> {
	try {
		await identityApp().signOut({});
	} catch (error) {
		const err = ConnectError.from(error);
		if (!NO_RESPONSE_CODES.has(err.code)) {
			throw new LogoutRevokeFailedError("server");
		}
		// Network-layer failure (offline / flaky mobile). Reported as a warning
		// rather than an error because it is a connectivity fact, not a defect —
		// it is the one signal for how often people try to sign out with no line.
		// Lazy import（比照 lib/api/client.ts reportApiFailure）：build/test 情境
		// Sentry 可能缺席或只部分 mock；.catch 吞掉 import 失敗，觀測絕不能反過
		// 來讓登出中斷。訊息只取網路層錯誤字串（"Load failed" 類）並截斷。
		const message = err.rawMessage;
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
 * 刪除之後 session 不再有效，所以不自動重送：沒收到回應（unavailable、deadline_exceeded、網路錯誤）時以
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
