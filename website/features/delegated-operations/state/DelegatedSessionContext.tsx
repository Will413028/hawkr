"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { enterDelegatedRuntime } from "@/lib/api/delegated-runtime";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import {
	clearEndPendingSessionId,
	isEndLatched,
	readEndPendingSessionId,
	writeEndPendingSessionId,
} from "./end-pending-latch";
import {
	canRenderDelegatedWorkspace,
	type DelegatedCapability,
	type DelegatedSessionState,
	type DelegatedSessionSummary,
	delegatedAuditUrl,
	delegatedIdentityKey,
	delegatedIdentityOf,
	resolveDelegatedSessionState,
} from "./session";
import {
	DELEGATED_QUERY_ROOT,
	DELEGATED_SESSION_QUERY_KEY,
	DelegatedSessionError,
	endDelegatedSession,
	fetchDelegatedSessionCurrent,
} from "./session-client";

/**
 * 代行工作階段的執行期狀態（56 號 §4.5 L1033／定案第 4 條）。
 *
 * 由 `hawkr-admin` 的 `OperationSessionBoundary` 搬移**生命週期**那一半：
 * 30 秒輪詢、終態偵測、身分變動即清快取、結束時中止 in-flight 寫入。
 */

interface DelegatedSessionContextValue {
	summary: DelegatedSessionSummary | null;
	state: DelegatedSessionState;
	/** 首抓未回：還不知道有沒有工作階段，不能當成「沒有」。 */
	loading: boolean;
	/** 每秒更新一次，供倒數用；工作階段非 active 時停住。 */
	now: number;
	capabilities: DelegatedCapability[];
	hasCapability: (capability: DelegatedCapability) => boolean;
	writesAllowed: boolean;
	/** 註冊寫入用的 AbortController，終態／結束時一併中止。回傳解除註冊函式。 */
	registerWriteController: (controller: AbortController) => () => void;
	ending: boolean;
	endError: DelegatedSessionError | Error | null;
	endSession: () => void;
	/** ADM 商家詳情頁（返回入口與結束後的落點）。 */
	returnUrl: string;
	/**
	 * ADM 的代行紀錄頁（左欄「操作紀錄」的目的地）。
	 *
	 * 拿得到 `sessionId` 時直接指向本次工作階段的稽核主控台；還沒拿到 summary
	 * 時退到查詢頁而不是留一個到不了的網址 —— 這個入口存在的理由就是「讓操作者
	 * 查得到自己剛留下的稽核」（§3871），指到一個沒有東西的地方等於它不存在。
	 */
	auditUrl: string;
	/**
	 * ADM 的 origin。訂單抽屜的「平台異常處置」要以 orderNo 現組跨 host 連結，
	 * 而那個值只有渲染當下才知道，組不進 provider 層的常數。
	 */
	admOrigin: string;
}

/**
 * ⚠️ 匯出**只為了測試**能給一份 stub 工作階段。
 *
 * 產品碼一律走 `useDelegatedSession()`／`useOptionalDelegatedSession()` —— 直接消費
 * context 會繞過「只能在 /adm 之下使用」那道保護。之所以需要這個接縫：閘門的全部價值
 * 在 `granted={hasCapability(capability)}` 那一個運算式上，而它只有在**真的渲染整支
 * 閘門**時才驗得到；`DelegatedSessionProvider` 本體會發 30 秒輪詢的網路請求，測不了。
 */
export const DelegatedSessionContext =
	React.createContext<DelegatedSessionContextValue | null>(null);

export function useDelegatedSession(): DelegatedSessionContextValue {
	const value = React.useContext(DelegatedSessionContext);
	if (!value) {
		throw new Error("useDelegatedSession 只能在代行工作區（/adm）之下使用");
	}
	return value;
}

/** 沒有工作階段時回 null，供 chrome 之外的元件做寬鬆判斷。 */
export function useOptionalDelegatedSession(): DelegatedSessionContextValue | null {
	return React.useContext(DelegatedSessionContext);
}

export function DelegatedSessionProvider({
	storeId,
	returnUrl,
	admOrigin,
	children,
}: {
	storeId: string;
	returnUrl: string;
	/** ADM 的 origin。稽核頁的網址由 sessionId 決定，只能在拿到 summary 之後組。 */
	admOrigin: string;
	children: React.ReactNode;
}) {
	const queryClient = useQueryClient();
	const [now, setNow] = React.useState(() => Date.now());
	const [writesBlockedByEnd, setWritesBlockedByEnd] = React.useState(false);
	const [endPendingSessionId, setEndPendingSessionId] = React.useState<
		string | null
	>(null);
	const writeControllersRef = React.useRef(new Set<AbortController>());
	const writableRef = React.useRef(false);
	const previousIdentityRef = React.useRef<string | null>(null);
	/** onMutate 要記下「是哪一個工作階段的結束沒被證明」，取當下這一次 render 的值。 */
	const sessionIdRef = React.useRef<string | null>(null);

	React.useEffect(() => {
		setEndPendingSessionId(readEndPendingSessionId());
	}, []);

	React.useEffect(() => enterDelegatedRuntime(), []);

	const currentQuery = useQuery({
		queryKey: DELEGATED_SESSION_QUERY_KEY,
		queryFn: ({ signal }) => fetchDelegatedSessionCurrent(signal),
		retry: false,
		staleTime: 0,
		refetchInterval: 30_000,
		refetchOnWindowFocus: true,
		refetchOnReconnect: true,
	});

	const summary = currentQuery.data ?? null;
	sessionIdRef.current = summary?.sessionId ?? null;

	const state = React.useMemo<DelegatedSessionState>(() => {
		const error = currentQuery.error;
		return resolveDelegatedSessionState({
			summary,
			failed: currentQuery.isError,
			failureCode:
				error instanceof DelegatedSessionError ? (error.code ?? null) : null,
			expectedStoreId: storeId,
			now,
		});
	}, [currentQuery.error, currentQuery.isError, now, storeId, summary]);

	const endLatched = isEndLatched({
		blockedInMemory: writesBlockedByEnd,
		pendingSessionId: endPendingSessionId,
		currentSessionId: summary?.sessionId ?? null,
	});

	const effectiveState: DelegatedSessionState =
		endLatched && canRenderDelegatedWorkspace(state) ? "end_unknown" : state;

	const abortRegisteredWrites = React.useCallback(() => {
		for (const controller of writeControllersRef.current) controller.abort();
		writeControllersRef.current.clear();
	}, []);

	const purgeDelegatedCache = React.useCallback(async () => {
		await queryClient.cancelQueries({ queryKey: DELEGATED_QUERY_ROOT });
		queryClient.removeQueries({ queryKey: DELEGATED_QUERY_ROOT });
	}, [queryClient]);

	const registerWriteController = React.useCallback(
		(controller: AbortController) => {
			if (!writableRef.current) {
				controller.abort();
				return () => undefined;
			}
			writeControllersRef.current.add(controller);
			return () => {
				writeControllersRef.current.delete(controller);
			};
		},
		[],
	);

	const endMutation = useMutation({
		mutationFn: () => endDelegatedSession(storeId),
		onMutate: async () => {
			setWritesBlockedByEnd(true);
			const sessionId = sessionIdRef.current;
			if (sessionId) {
				writeEndPendingSessionId(sessionId);
				setEndPendingSessionId(sessionId);
			}
			writableRef.current = false;
			abortRegisteredWrites();
			await purgeDelegatedCache();
		},
		onSuccess: async () => {
			clearEndPendingSessionId();
			setEndPendingSessionId(null);
			await purgeDelegatedCache();
			commitDocumentNavigation(returnUrl);
		},
	});

	const writesAllowed =
		canRenderDelegatedWorkspace(effectiveState) &&
		!endLatched &&
		!endMutation.isPending;
	writableRef.current = writesAllowed;

	React.useEffect(() => {
		if (!summary || summary.status !== "active") return;
		const timer = window.setInterval(() => setNow(Date.now()), 1000);
		return () => window.clearInterval(timer);
	}, [summary]);

	const identity = summary
		? delegatedIdentityKey(delegatedIdentityOf(summary))
		: null;
	React.useEffect(() => {
		const previous = previousIdentityRef.current;
		if (previous !== null && identity !== previous) {
			abortRegisteredWrites();
			void purgeDelegatedCache();
		}
		previousIdentityRef.current = identity;
	}, [abortRegisteredWrites, identity, purgeDelegatedCache]);

	React.useEffect(() => {
		if (canRenderDelegatedWorkspace(effectiveState)) return;
		writableRef.current = false;
		abortRegisteredWrites();
		void purgeDelegatedCache();
	}, [abortRegisteredWrites, effectiveState, purgeDelegatedCache]);

	const currentSessionId = summary?.sessionId ?? null;
	React.useEffect(() => {
		if (endPendingSessionId === null || currentSessionId === null) return;
		if (currentSessionId === endPendingSessionId) return;
		clearEndPendingSessionId();
		setEndPendingSessionId(null);
	}, [currentSessionId, endPendingSessionId]);

	const capabilities = summary?.capabilities ?? [];
	const loading = currentQuery.isLoading && !summary;
	const auditUrl = delegatedAuditUrl(admOrigin, summary?.sessionId);
	const value = React.useMemo<DelegatedSessionContextValue>(
		() => ({
			summary,
			state: effectiveState,
			loading,
			now,
			capabilities,
			hasCapability: (capability) => capabilities.includes(capability),
			writesAllowed,
			registerWriteController,
			ending: endMutation.isPending,
			endError: endMutation.error,
			endSession: () => endMutation.mutate(),
			returnUrl,
			auditUrl,
			admOrigin,
		}),
		[
			admOrigin,
			auditUrl,
			capabilities,
			effectiveState,
			endMutation.error,
			endMutation.isPending,
			endMutation.mutate,
			loading,
			now,
			registerWriteController,
			returnUrl,
			summary,
			writesAllowed,
		],
	);

	return (
		<DelegatedSessionContext.Provider value={value}>
			{children}
		</DelegatedSessionContext.Provider>
	);
}

export { canRenderDelegatedWorkspace };
