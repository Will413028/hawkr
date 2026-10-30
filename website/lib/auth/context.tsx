"use client";

import type { ConnectError } from "@connectrpc/connect";
import { userMessage } from "@hawkr/api/connect/errors";
import * as Sentry from "@sentry/nextjs";
import { skipToken, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "@/lib/feedback/toast";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { stripLocale } from "@/lib/intl/paths";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	logout as apiLogout,
	fetchSession,
	LogoutRevokeFailedError,
} from "../api/auth";
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
import { HYDRATION_GATE_MS, resolveSession } from "./session-resolver";
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
	hydrated: boolean;
	sessionUnavailable: boolean;
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

export function AuthProvider({ children }: { children: React.ReactNode }) {
	const router = useObservedRouter("auth_context");
	const queryClient = useQueryClient();
	// 憑證是 HttpOnly cookie，JS 讀不到：登入與否只能問單體（GetSession）。快取裡 undefined 是「還沒問到」，
	// null 是「問過了，沒有登入」。登入、換帳號都是整頁導覽，所以整份快取只有一個 key。
	const sessionQuery = useQuery<Identity | null>({
		queryKey: SESSION_KEY,
		queryFn: skipToken,
		enabled: false,
		staleTime: Number.POSITIVE_INFINITY,
	});
	const identity = sessionQuery.data ?? null;
	const [active, setActive] = React.useState<RoleSlot | null>(null);
	const [gateExpired, setGateExpired] = React.useState(false);
	const [probeUnavailable, setProbeUnavailable] = React.useState(false);
	const [probeGaveUp, setProbeGaveUp] = React.useState(false);
	const previousAccountId = React.useRef<string | undefined>(undefined);
	const profileStartedFor = React.useRef<string | null>(null);
	const endingSession = React.useRef(false);
	const sessionSettled = sessionQuery.data !== undefined || probeGaveUp;
	const hydrated = gateExpired || sessionSettled || probeUnavailable;

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
			setProbeUnavailable(false);
			setProbeGaveUp(true);
			// 單體拒絕這個帳號（刪除或停權）：盡力撤銷 session 並清掉 cookie；失敗不影響本機已經當成訪客。
			rejection = apiLogout().catch(() => {
				rejection = undefined;
			});
			return rejection;
		};
	}, [defaultErrorText, queryClient]);

	React.useEffect(() => {
		const gate = setTimeout(() => setGateExpired(true), HYDRATION_GATE_MS);
		return () => clearTimeout(gate);
	}, []);

	React.useEffect(() => {
		setProbeUnavailable(false);
		setProbeGaveUp(false);
		return resolveSession(fetchSession, {
			onUnavailable: () => setProbeUnavailable(true),
			onDenied: (error) => {
				void rejectSession(error);
			},
			onGaveUp: () => setProbeGaveUp(true),
			// null = unauthenticated：訪客。
			onResolved: (result) => {
				setProbeUnavailable(false);
				queryClient.setQueryData(SESSION_KEY, result);
			},
		});
	}, [queryClient, rejectSession]);

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
			hydrated,
			sessionUnavailable: probeUnavailable && !identity,
			sessionSettled,
			commitIdentity,
			switchRole,
			startSellerOnboarding,
			signOut,
			rejectSession,
		}),
		[
			identity,
			active,
			hydrated,
			probeUnavailable,
			sessionSettled,
			commitIdentity,
			switchRole,
			startSellerOnboarding,
			signOut,
			rejectSession,
		],
	);

	return (
		<AuthContext.Provider value={value}>
			<span data-auth-hydrated={hydrated} style={{ display: "none" }} />
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
