"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { userMessage } from "@hawkr/api/connect/errors";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import * as Sentry from "@sentry/nextjs";
import { skipToken, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "@/lib/feedback/toast";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { stripLocale } from "@/lib/intl/paths";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { logout as apiLogout, LogoutRevokeFailedError } from "../api/auth";
import { AUTH_EXPIRED_EVENT } from "../api/client";
import { fetchProfile } from "../profile/repo";
import {
	canUsePerspective,
	effectiveActiveRole,
	getStoredActiveRole,
	type RoleSlot,
	setStoredActiveRole,
} from "./active-role";
import { reduceStorageEvent } from "./cross-tab-sync";
import { syncLocalPrivateState } from "./local-private-state";
import type { SessionState } from "./session-state";
import type { Identity } from "./storage";
import {
	shouldTeardownPrivateCache,
	teardownPrivateCache as teardownPrivateCacheOp,
} from "./teardown";

const VOLUNTARY_LOGOUT_WINDOW_MS = 15_000;
let lastVoluntaryLogoutAt = 0;
function markVoluntaryLogout() {
	lastVoluntaryLogoutAt = Date.now();
}
function withinVoluntaryLogout() {
	return Date.now() - lastVoluntaryLogoutAt <= VOLUNTARY_LOGOUT_WINDOW_MS;
}

/** 目前登入的帳號在 React Query 的快取 key（見 AuthProvider）。 */
const SESSION_KEY = ["session"] as const;

function templatePath(path: string | undefined): string {
	if (!path) return "unknown";
	return (path.split("?")[0] ?? "")
		.split("/")
		.map((segment) => (/\d/.test(segment) ? ":id" : segment))
		.join("/");
}

interface AuthContextValue {
	identity: Identity | null;
	isLoggedIn: boolean;
	/** 帳號有 ROLE_SELLER。每個登入的帳號都是買家，所以沒有 hasBuyer：看 isLoggedIn。 */
	hasSeller: boolean;
	activeRole: RoleSlot | null;
	/** 伺服器已經把登入狀態帶進第一個畫面，恆為 true；等 client 狀態整併後移除。 */
	hydrated: boolean;
	/** 單體暫時答不出來（B-199）：不是登出，`identity` 為 null 但不能當成訪客。 */
	sessionUnavailable: boolean;
	/** 同 `hydrated`，恆為 true。 */
	sessionSettled: boolean;
	commitIdentity: (identity: Identity) => void;
	switchRole: (next: RoleSlot) => boolean;
	startSellerOnboarding: (redirect?: string) => void;
	signOut: () => Promise<void>;
	rejectSession: (error?: ConnectError) => Promise<void>;
}

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

/** 伺服器的 GetSession 答案（DAL）換成 React Query 快取的初值：`undefined` 是「沒有答案」（暫時不可用）。 */
function identityFromSession(
	session: SessionState,
): Identity | null | undefined {
	if (session.status === "signedIn") return session.identity;
	if (session.status === "guest") return null;
	return undefined;
}

/**
 * 登入狀態的唯一來源是伺服器：根 layout 不 await `verifySession()`，把 promise 交給這裡，
 * 在 `<Suspense>` 內以 `use()` 讀（shell 仍可靜態，身分在同一個 response 串流）。
 * 憑證是 HttpOnly cookie，JS 讀不到，所以 client 不再自己問單體；事件（登入、登出、換帳號）
 * 之後才更新 React Query 快取裡的同一個 key。
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
	const initialSession = React.use(session);
	// 快取裡 undefined 是「沒有答案」，null 是「確定沒有登入」。登入、換帳號都是整頁導覽，所以整份快取只有一個 key。
	const sessionQuery = useQuery<Identity | null>({
		queryKey: SESSION_KEY,
		queryFn: skipToken,
		enabled: false,
		staleTime: Number.POSITIVE_INFINITY,
		initialData: identityFromSession(initialSession),
	});
	const identity = sessionQuery.data ?? null;
	// 第一次渲染的工作區必須與伺服器一致：只有 localStorage 知道賣家上次停在哪個工作區，effect 之後才讀它。
	const [active, setActive] = React.useState<RoleSlot | null>(() =>
		identity ? effectiveActiveRole(identity, null) : null,
	);
	const previousAccountId = React.useRef<string | undefined>(undefined);
	const profileStartedFor = React.useRef<string | null>(null);
	const endingSession = React.useRef(false);
	const sessionUnavailable =
		initialSession.status === "unavailable" && sessionQuery.data === undefined;

	const teardownPrivateCache = React.useCallback(
		() => teardownPrivateCacheOp(queryClient),
		[queryClient],
	);

	const defaultErrorText = useDefaultErrorText();
	const rejectSession = React.useMemo(() => {
		let rejection: Promise<void> | undefined;
		let notified = false;
		return (error?: ConnectError): Promise<void> => {
			if (rejection) return rejection;
			markVoluntaryLogout();
			if (error && !notified) {
				notified = true;
				const text = userMessage(error, defaultErrorText);
				if (text) toast.error(text);
			}
			queryClient.setQueryData(SESSION_KEY, null);
			// 單體拒絕這個帳號（刪除或停權）：盡力撤銷 session 並清掉 cookie；失敗不影響本機已經當成訪客。
			rejection = apiLogout().catch(() => {
				rejection = undefined;
			});
			return rejection;
		};
	}, [defaultErrorText, queryClient]);

	const refusal =
		initialSession.status === "guest" ? initialSession.refusal : undefined;
	React.useEffect(() => {
		if (!refusal) return;
		// 單體拒絕這個帳號（刪除或停權）：伺服器端唯讀，由這裡撤銷 session、清 cookie 並顯示原因。
		void rejectSession(
			new ConnectError(
				refusal.message ?? "session refused",
				Code.PermissionDenied,
				undefined,
				refusal.message
					? [
							{
								desc: LocalizedMessageSchema,
								value: { message: refusal.message },
							},
						]
					: undefined,
			),
		);
	}, [refusal, rejectSession]);

	React.useEffect(() => {
		if (sessionQuery.data === undefined) return;
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
			!withinVoluntaryLogout() &&
			stripLocale(window.location.pathname) !== "/"
		) {
			commitDocumentNavigation("/", true);
		}
	}, [sessionQuery.data, identity, teardownPrivateCache]);

	React.useEffect(() => {
		if (!identity) {
			profileStartedFor.current = null;
			return;
		}
		if (
			effectiveActiveRole(identity) !== "buyer" ||
			identity.nickname ||
			profileStartedFor.current === identity.accountId
		) {
			return;
		}
		profileStartedFor.current = identity.accountId;
		let canceled = false;
		void fetchProfile({ suppressAuthExpired: true })
			.then((profile) => {
				if (canceled) return;
				queryClient.setQueryData<Identity | null>(SESSION_KEY, (current) =>
					current?.accountId === identity.accountId
						? {
								...current,
								nickname: profile.nickname,
								avatarKey: profile.avatarKey,
							}
						: current,
				);
			})
			.catch(() => {});
		return () => {
			canceled = true;
		};
	}, [identity, queryClient]);

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
				// 單體說 unauthenticated：session 已經不在（過期、撤銷、帳號停權或刪除），沒有東西要結束。
				queryClient.setQueryData(SESSION_KEY, null);
				setActive(null);
				await teardownPrivateCache();
				if (stripLocale(window.location.pathname) !== "/") {
					commitDocumentNavigation("/", true);
				}
			} catch {
				return;
			} finally {
				endingSession.current = false;
			}
		};
		const listener = (event: Event) => void onExpired(event);
		window.addEventListener(AUTH_EXPIRED_EVENT, listener);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, listener);
	}, [queryClient, teardownPrivateCache]);

	React.useEffect(() => {
		const onStorage = (event: StorageEvent) => {
			const role = reduceStorageEvent(event, identity);
			if (role) setActive(role);
		};
		window.addEventListener("storage", onStorage);
		return () => window.removeEventListener("storage", onStorage);
	}, [identity]);

	const commitIdentity = React.useCallback(
		(next: Identity) => {
			queryClient.setQueryData(SESSION_KEY, next);
			setActive(resolveActiveRole(next));
		},
		[queryClient],
	);

	const switchRole = React.useCallback(
		(next: RoleSlot): boolean => {
			const account =
				queryClient.getQueryData<Identity | null>(SESSION_KEY) ?? identity;
			if (!account || !canUsePerspective(account, next)) return false;
			setStoredActiveRole(next);
			setActive(next);
			return true;
		},
		[identity, queryClient],
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

	const signOut = React.useCallback(async () => {
		markVoluntaryLogout();
		try {
			await apiLogout();
		} catch (error) {
			toast.error(
				error instanceof LogoutRevokeFailedError && error.reason === "network"
					? "登出失敗，請確認網路後再試一次"
					: "登出失敗，請再試一次",
			);
			throw error;
		}
		queryClient.setQueryData(SESSION_KEY, null);
		setActive(null);
		await teardownPrivateCache();
	}, [queryClient, teardownPrivateCache]);

	const value = React.useMemo<AuthContextValue>(
		() => ({
			identity,
			isLoggedIn: !!identity,
			hasSeller: !!identity && identity.isSeller,
			activeRole: active,
			hydrated: true,
			sessionUnavailable,
			sessionSettled: true,
			commitIdentity,
			switchRole,
			startSellerOnboarding,
			signOut,
			rejectSession,
		}),
		[
			identity,
			active,
			sessionUnavailable,
			commitIdentity,
			switchRole,
			startSellerOnboarding,
			signOut,
			rejectSession,
		],
	);

	return (
		<AuthContext.Provider value={value}>
			<span data-auth-hydrated="true" style={{ display: "none" }} />
			{children}
		</AuthContext.Provider>
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
