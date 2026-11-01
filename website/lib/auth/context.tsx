"use client";

import * as Sentry from "@sentry/nextjs";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "@/lib/feedback/toast";
import { stripLocale } from "@/lib/intl/paths";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	logout as apiLogout,
	fetchSession,
	LogoutRevokeFailedError,
	SESSION_UNAVAILABLE,
} from "../api/auth";
import { AUTH_EXPIRED_EVENT } from "../api/client";
import {
	canUsePerspective,
	effectiveActiveRole,
	getStoredActiveRole,
	type RoleSlot,
	setStoredActiveRole,
} from "./active-role";
import {
	openSessionChannel,
	peerAction,
	reduceStorageEvent,
	type SessionChannel,
} from "./cross-tab-sync";
import { syncLocalPrivateState } from "./local-private-state";
import {
	type AuthStatus,
	SESSION_RETRY_BACKOFF_MS,
	type SessionState,
} from "./session-state";
import type { Identity } from "./storage";
import {
	shouldTeardownPrivateCache,
	teardownPrivateCache as teardownPrivateCacheOp,
} from "./teardown";
import { ZoneMountRegistryContext } from "./zone-context";

const VOLUNTARY_LOGOUT_WINDOW_MS = 15_000;
let lastVoluntaryLogoutAt = 0;
function markVoluntaryLogout() {
	lastVoluntaryLogoutAt = Date.now();
}
function withinVoluntaryLogout() {
	return Date.now() - lastVoluntaryLogoutAt <= VOLUNTARY_LOGOUT_WINDOW_MS;
}

function templatePath(path: string | undefined): string {
	if (!path) return "unknown";
	return (path.split("?")[0] ?? "")
		.split("/")
		.map((segment) => (/\d/.test(segment) ? ":id" : segment))
		.join("/");
}

type AuthIdentityState =
	| { status: "signedIn"; identity: Identity }
	| { status: Exclude<AuthStatus, "signedIn">; identity: null };

/**
 * `status` 是 `unknown`（伺服器的答案還沒到：靜態外殼與第一次 client render 都是它）、
 * `guest | signedIn | unavailable`。`unavailable` 是單體暫時答不出來（B-199）：不是登出，
 * `identity` 為 null 但不能當成訪客。沒有「是否登入」的布林：`unknown` 與 `guest` 都會讓它是 false，
 * 把還沒回答當成訪客；要分支就明寫 `status`。
 */
type AuthContextValue = AuthIdentityState & {
	/** 帳號有 ROLE_SELLER。每個登入的帳號都是買家，所以沒有 hasBuyer：看 `status === "signedIn"`。 */
	hasSeller: boolean;
	activeRole: RoleSlot | null;
	/** `confirmed`：剛由伺服器確認的身分（開店完成後的 GetSession），用它判斷能不能切，不看載入頁面時的身分。 */
	switchRole: (next: RoleSlot, confirmed?: Identity) => boolean;
	startSellerOnboarding: (redirect?: string) => void;
	/**
	 * 撤銷 session、清本機私有資料，再整頁導向 `to`（預設首頁；`replace` 不留歷史）。身分改變的事件都以
	 * 換文件收尾，所以進行中的舊答案不會讓舊身分回來。同時的呼叫合併成一次；撤銷失敗時 reject（已跳 toast），
	 * 什麼都不改，可以再呼叫。
	 */
	signOut: (to?: string, replace?: boolean) => Promise<void>;
};

const AuthContext = React.createContext<AuthContextValue | null>(null);

function resolveActiveRole(identity: Identity | null): RoleSlot | null {
	if (!identity) {
		setStoredActiveRole(null);
		return null;
	}
	const storedRole = getStoredActiveRole();
	const active = effectiveActiveRole(identity, storedRole);
	if (active !== storedRole) setStoredActiveRole(active);
	return active;
}

/**
 * 登入狀態的唯一來源是伺服器：根 layout 不 await `verifySession()`，把 promise 交給這裡。
 * Provider 不 suspend：promise 在 effect 內接收，結果出來前 `status` 是 `unknown`，所以 `{children}`
 * 留在靜態外殼內，server HTML 與第一次 client render 一致（都是 `unknown`）。
 * 憑證是 HttpOnly cookie，JS 讀不到，所以 client 不再自己問單體；顯示用的暱稱與頭像也在同一個答案裡。
 * client 不持有自己的身分：改變身分的事件（登出、401、結束 session、完成開店）都整頁導向新文件，
 * 新文件的一次 server render 讀事件之後的 cookie；舊文件卸載會中止進行中的請求。
 * 其餘 `router.refresh()` 的答案由最新的 promise 取代舊答案（同帳號或訪客變登入）。
 * 暫時不可用的答案不取代已有的答案。
 */
export function AuthProvider({
	session,
	children,
}: {
	session: Promise<SessionState>;
	children: React.ReactNode;
}) {
	const router = useObservedRouter("auth_context");
	const queryClient = useQueryClient();
	const routerRef = React.useRef(router);
	routerRef.current = router;
	const refreshFromServer = React.useCallback(() => {
		routerRef.current.refresh();
	}, []);
	const [serverSession, setServerSession] = React.useState<SessionState | null>(
		null,
	);
	const leaving = React.useRef(false);
	React.useEffect(() => {
		let live = true;
		const adopt = (next: SessionState) => {
			if (!live || leaving.current) return;
			setServerSession((current) =>
				next.status === "unavailable" && current ? current : next,
			);
		};
		session.then(adopt, () => adopt({ status: "unavailable" }));
		return () => {
			live = false;
		};
	}, [session]);
	const identity: Identity | null =
		serverSession?.status === "signedIn" ? serverSession.identity : null;
	const latestIdentity = React.useRef(identity);
	latestIdentity.current = identity;
	// 工作區：只有 localStorage 知道賣家上次停在哪個工作區，effect 之後才讀它；之前用伺服器答案的預設值。
	const [storedActive, setActive] = React.useState<RoleSlot | null>(null);
	const active = identity
		? (storedActive ?? effectiveActiveRole(identity, null))
		: null;
	const status: AuthStatus = serverSession?.status ?? "unknown";
	const identityState = React.useMemo<AuthIdentityState>(
		() =>
			serverSession?.status === "signedIn"
				? { status: "signedIn", identity: serverSession.identity }
				: {
						status: serverSession?.status ?? "unknown",
						identity: null,
					},
		[serverSession],
	);
	const previousAccountId = React.useRef<string | undefined>(undefined);
	const statusRef = React.useRef(status);
	statusRef.current = status;
	const mountedZones = React.useRef(0);
	const signingOut = React.useRef<Promise<void> | undefined>(undefined);
	const tAuth = useTranslations("auth");
	const endingSession = React.useRef(false);

	const teardownPrivateCache = React.useCallback(
		() => teardownPrivateCacheOp(queryClient),
		[queryClient],
	);

	const leaveDocument = React.useCallback(
		async (to: string | null, replace: boolean, onCancelled?: () => void) => {
			// 換文件前的空檔：舊答案不能再改這個文件的身分。
			leaving.current = true;
			// beforeunload 的提示被使用者取消時文件留下來，要讓這個文件繼續接收伺服器的答案。
			const onBeforeUnload = (event: BeforeUnloadEvent) => {
				if (!event.defaultPrevented) return;
				setTimeout(() => {
					leaving.current = false;
					onCancelled?.();
				}, 0);
			};
			window.addEventListener("beforeunload", onBeforeUnload, { once: true });
			try {
				await teardownPrivateCache();
			} catch {
				// 清不掉也要換文件：留著舊文件才是風險。
			}
			if (to === null) window.location.reload();
			else commitDocumentNavigation(to, replace);
		},
		[teardownPrivateCache],
	);

	React.useEffect(() => {
		if (status !== "unavailable") return;
		let cancelled = false;
		let inFlight = false;
		let timer: ReturnType<typeof setTimeout> | undefined;
		const ask = async (): Promise<boolean> => {
			if (inFlight) return true;
			inFlight = true;
			try {
				let result: Awaited<ReturnType<typeof fetchSession>>;
				try {
					result = await fetchSession();
				} catch {
					result = SESSION_UNAVAILABLE;
				}
				if (cancelled) return false;
				if (result !== SESSION_UNAVAILABLE) refreshFromServer();
				return true;
			} finally {
				inFlight = false;
			}
		};
		const schedule = (attempt: number) => {
			const delay = SESSION_RETRY_BACKOFF_MS[attempt];
			if (delay === undefined) return;
			timer = setTimeout(async () => {
				if (await ask()) schedule(attempt + 1);
			}, delay);
		};
		const onOnline = () => void ask();
		const onVisibility = () => {
			if (document.visibilityState === "visible") void ask();
		};
		window.addEventListener("online", onOnline);
		document.addEventListener("visibilitychange", onVisibility);
		schedule(0);
		return () => {
			cancelled = true;
			if (timer) clearTimeout(timer);
			window.removeEventListener("online", onOnline);
			document.removeEventListener("visibilitychange", onVisibility);
		};
	}, [status, refreshFromServer]);

	React.useEffect(() => {
		if (status !== "signedIn" && status !== "guest") return;
		const nextAccountId = identity?.accountId;
		const previous = previousAccountId.current;
		syncLocalPrivateState(nextAccountId ?? null);
		setActive(resolveActiveRole(identity));
		previousAccountId.current = nextAccountId;
		if (shouldTeardownPrivateCache(previous, nextAccountId)) {
			void teardownPrivateCache();
		}
		if (
			previous !== undefined &&
			nextAccountId === undefined &&
			stripLocale(window.location.pathname) !== "/"
		) {
			commitDocumentNavigation("/", true);
		}
	}, [status, identity, teardownPrivateCache]);

	React.useEffect(() => {
		const onExpired = async (event: Event) => {
			if (endingSession.current) return;
			endingSession.current = true;
			try {
				const detail = (event as CustomEvent<{ path?: string }>).detail;
				if (!withinVoluntaryLogout()) {
					Sentry.captureMessage("involuntary logout", {
						level: "warning",
						fingerprint: ["involuntary-logout"],
						tags: { trigger_path: templatePath(detail?.path) },
					});
				}
				// 單體說 unauthenticated：session 已經不在（過期、撤銷、帳號停權或刪除）。文件上沒有身分可清
				// （根層不是 signedIn，也沒有掛著由伺服器確認身分的受保護區）就不換文件，否則只會循環。
				if (statusRef.current !== "signedIn" && mountedZones.current === 0) {
					return;
				}
				await leaveDocument("/", true);
			} catch {
				return;
			} finally {
				endingSession.current = false;
			}
		};
		const listener = (event: Event) => void onExpired(event);
		window.addEventListener(AUTH_EXPIRED_EVENT, listener);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, listener);
	}, [leaveDocument]);

	const peerView = React.useRef({ status, accountId: identity?.accountId });
	peerView.current = { status, accountId: identity?.accountId };
	const channelRef = React.useRef<SessionChannel | null>(null);
	const announcedAccount = React.useRef<string | null | undefined>(undefined);
	React.useEffect(() => {
		const channel = openSessionChannel((announcement) => {
			const action = peerAction(announcement, peerView.current);
			if (action === "refresh") refreshFromServer();
			// 這個分頁已登入而別的分頁換了帳號或登出：整頁重載，不讓舊身分留在畫面上。
			if (action === "reload") void leaveDocument(null, true);
		});
		channelRef.current = channel;
		return () => {
			channel?.close();
			channelRef.current = null;
			announcedAccount.current = undefined;
		};
	}, [refreshFromServer, leaveDocument]);
	const announceableAccount =
		status === "signedIn" || status === "guest"
			? (identity?.accountId ?? null)
			: undefined;
	React.useEffect(() => {
		if (announceableAccount === undefined) return;
		if (announcedAccount.current === announceableAccount) return;
		announcedAccount.current = announceableAccount;
		channelRef.current?.announce(announceableAccount);
	}, [announceableAccount]);

	React.useEffect(() => {
		const onStorage = (event: StorageEvent) => {
			const role = reduceStorageEvent(event, identity);
			if (role) setActive(role);
		};
		window.addEventListener("storage", onStorage);
		return () => window.removeEventListener("storage", onStorage);
	}, [identity]);

	// 從 bfcache 還原的已登入頁面沒有重新問伺服器，可能還帶著登出前的身分；訪客頁沒有身分要清，留給 bfcache。
	React.useEffect(() => {
		const onPageShow = (event: PageTransitionEvent) => {
			if (event.persisted && statusRef.current === "signedIn") {
				window.location.reload();
			}
		};
		window.addEventListener("pageshow", onPageShow);
		return () => window.removeEventListener("pageshow", onPageShow);
	}, []);

	const switchRole = React.useCallback(
		(next: RoleSlot, confirmed?: Identity): boolean => {
			const account = confirmed ?? latestIdentity.current;
			if (!account || !canUsePerspective(account, next)) return false;
			setStoredActiveRole(next);
			setActive(next);
			return true;
		},
		[],
	);

	const startSellerOnboarding = React.useCallback(
		(redirect?: string) => {
			const params = new URLSearchParams();
			if (redirect?.startsWith("/") && !redirect.startsWith("//")) {
				params.set("redirect", redirect);
			}
			const qs = params.toString();
			router.push(`/seller/onboarding${qs ? `?${qs}` : ""}`);
		},
		[router],
	);

	const signOut = React.useCallback(
		(to = "/", replace = false): Promise<void> => {
			if (signingOut.current) return signingOut.current;
			markVoluntaryLogout();
			const run = apiLogout().then(
				() =>
					leaveDocument(to, replace, () => {
						// 工作階段已撤銷、文件被留下：放行下一次登出，並請伺服器重答（訪客）。
						signingOut.current = undefined;
						refreshFromServer();
					}),
				(error: unknown) => {
					signingOut.current = undefined;
					toast.error(
						error instanceof LogoutRevokeFailedError &&
							error.reason === "network"
							? tAuth("logoutFailedNetwork")
							: tAuth("logoutFailed"),
					);
					throw error;
				},
			);
			signingOut.current = run;
			return run;
		},
		[leaveDocument, refreshFromServer, tAuth],
	);

	const value = React.useMemo<AuthContextValue>(
		() => ({
			...identityState,
			hasSeller: !!identity && identity.isSeller,
			activeRole: active,
			switchRole,
			startSellerOnboarding,
			signOut,
		}),
		[
			identityState,
			identity,
			active,
			switchRole,
			startSellerOnboarding,
			signOut,
		],
	);

	return (
		<ZoneMountRegistryContext.Provider value={mountedZones}>
			<AuthContext.Provider value={value}>
				<span data-auth-status={status} style={{ display: "none" }} />
				{children}
			</AuthContext.Provider>
		</ZoneMountRegistryContext.Provider>
	);
}

export function useOptionalAuth(): AuthContextValue | null {
	return React.useContext(AuthContext);
}

export function useAuth(): AuthContextValue {
	const ctx = useOptionalAuth();
	if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
	return ctx;
}
