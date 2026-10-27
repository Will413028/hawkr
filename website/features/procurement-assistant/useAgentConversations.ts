"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/context";
import { toast } from "@/lib/feedback/toast";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { queryKeys } from "@/lib/query/keys";
import {
	type AgentConversationPage,
	type AgentConversationSummary,
	type AgentTranscript,
	deleteAgentConversation,
	fetchAgentConversations,
	fetchAgentTranscript,
	renameAgentConversation,
} from "./conversations";

const EMPTY_PAGE: AgentConversationPage = {
	records: [],
	hasMore: false,
	nextCursorId: null,
	available: false,
};

/**
 * Hawkr Agent 對話列表（B-244 Ask 1）。照 useConversations 的慣例：
 * identity-scoped key、placeholderData（非 initialData）、30s 全域
 * staleTime。端點尚未部署時 fetch 層回 available:false 的空頁——
 * 側欄不長出 agent 區段，不當錯誤呈現。
 */
export function useAgentConversations(enabled = true) {
	const { hydrated, isLoggedIn } = useAuth();
	const queryIdentity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.assistant.conversations(queryIdentity);
	const shouldFetch = enabled && hydrated && isLoggedIn;
	const query = useQuery({
		queryKey,
		enabled: shouldFetch,
		placeholderData: EMPTY_PAGE,
		queryFn: () => fetchAgentConversations({ pageSize: 50 }),
	});
	const page = shouldFetch ? (query.data ?? EMPTY_PAGE) : EMPTY_PAGE;

	/**
	 * hover／focus 某一列時先把該對話的紀錄與快照抓進快取。
	 *
	 * 冷快取切換實測會閃約 370ms 的骨架（右側 workspace 因路由參數變更而
	 * remount，掛載時快取沒東西可讀）。滑過去的那段時間剛好夠抓完，點下去
	 * 就直接有內容。prefetchQuery 會遵守 staleTime，所以同一列反覆 hover
	 * 不會重打；snapshot 這裡刻意給 15s staleTime（避免 hover 洗 API），
	 * 還原路徑仍用 staleTime 0 在背景校正 stage。
	 */
	const prefetch = async (conversationId: string) => {
		if (!shouldFetch) return;
		const transcript = await queryClient
			.fetchQuery({
				queryKey: queryKeys.assistant.transcript(queryIdentity, conversationId),
				queryFn: () => fetchAgentTranscript(conversationId),
				staleTime: 60_000,
			})
			.catch(() => null as AgentTranscript | null);
		if (!transcript?.requestId) return;
		await queryClient
			.prefetchQuery({
				queryKey: queryKeys.assistant.snapshot(
					queryIdentity,
					transcript.requestId,
				),
				queryFn: async () => {
					const env = await apiClient.authed.get<unknown>(
						`/buyers/me/procurement/requests/${transcript.requestId}/snapshot`,
					);
					return (env as { data: unknown }).data;
				},
				staleTime: 15_000,
			})
			.catch(() => {});
	};

	const invalidate = () =>
		queryClient.invalidateQueries({
			queryKey: queryKeys.assistant.conversations(queryIdentity),
		});

	return {
		prefetch,
		conversations: page.records,
		available: page.available,
		/** 後端還有更多沒抓下來——側欄用它讓「找不到對話」的文案誠實。 */
		hasMore: page.hasMore,
		loading:
			!hydrated ||
			query.isLoading ||
			(query.isFetching && query.isPlaceholderData),
		async rename(conversationId: string, title: string) {
			// 樂觀改名＋失敗還原；成功後仍 invalidate 讓伺服器版（可能截斷／
			// 正規化過）覆寫本地樂觀值。先 cancelQueries：in-flight refetch
			// 落地會蓋掉樂觀值，previous 也要是 post-settlement 快照。
			await queryClient.cancelQueries({ queryKey });
			const previous =
				queryClient.getQueryData<AgentConversationPage>(queryKey);
			queryClient.setQueryData<AgentConversationPage>(queryKey, (old) =>
				old
					? {
							...old,
							records: old.records.map(
								(row): AgentConversationSummary =>
									row.conversationId === conversationId
										? { ...row, title }
										: row,
							),
						}
					: old,
			);
			try {
				await renameAgentConversation(conversationId, title);
			} catch {
				queryClient.setQueryData(queryKey, previous);
				toast.error("改名失敗，請重試");
				return false;
			}
			void invalidate();
			return true;
		},
		/** 刪除＝樂觀移除＋sonner 復原（規劃 Wave 4 規格）。契約沒有還原
		 * 端點，所以「復原」是取消尚未送出的 DELETE，不是 un-delete——
		 * toast 存活期間不打後端，逾時或關閉才真正送出。 */
		async remove(conversationId: string) {
			await queryClient.cancelQueries({ queryKey });
			const previous =
				queryClient.getQueryData<AgentConversationPage>(queryKey);
			queryClient.setQueryData<AgentConversationPage>(queryKey, (old) =>
				old
					? {
							...old,
							records: old.records.filter(
								(row) => row.conversationId !== conversationId,
							),
						}
					: old,
			);
			let settled = false;
			const commit = () => {
				if (settled) return;
				settled = true;
				void (async () => {
					try {
						await deleteAgentConversation(conversationId);
					} catch {
						queryClient.setQueryData(queryKey, previous);
						toast.error("刪除失敗，請重試");
						return;
					}
					void invalidate();
				})();
			};
			const undo = () => {
				if (settled) return;
				settled = true;
				queryClient.setQueryData(queryKey, previous);
				void invalidate();
			};
			toast.success("已刪除對話", {
				duration: 6000,
				action: { label: "復原", onClick: undo },
				onAutoClose: commit,
				onDismiss: commit,
			});
			return true;
		},
		refresh: invalidate,
	};
}
