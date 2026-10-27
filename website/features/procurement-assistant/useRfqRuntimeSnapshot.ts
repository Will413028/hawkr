"use client";

import * as React from "react";
import { ApiClientError, apiClient } from "@/lib/api/client";
import {
	type BuyerQuoteFailure,
	buyerFacingSellerResponseListSchema,
	buyerQuoteFailure,
} from "./buyer-quote-contract";
import {
	type PublicationResultView,
	type RuntimeSnapshot,
	runtimeSnapshotSchema,
	selectPublicationResult,
} from "./runtime-snapshot-contract";

/**
 * 稿 R-02 三區版面的資料層（issue #2538 A8 後半）。
 *
 * 兩個讀取、一支 hook：
 *   1. `GET .../conversations/{id}/runtime-snapshot` —— 發布結果與進度事實
 *      （doc 57 §5.6）。這是本 repo 對這支端點的第一個消費端。
 *   2. `GET .../requests/{id}/seller-responses` —— 「目前交易真相」卡的筆數。
 *      沒有第二支端點就數不出「已收到 N 筆」，而**猜一個 0 出來正是這張卡
 *      存在的反面**。
 *
 * 兩支獨立成敗：發布結果讀到、回價清單讀不到時，畫面照樣呈現發布事實，只把
 * 筆數留成「尚未取得」。把它們綁成同一個 all-or-nothing 會讓一次逾時把已知
 * 的事實一起藏掉。
 *
 * 與 `useBuyerQuoteComparison` 分開的理由：那支是**可操作**的比較清單（含
 * shortlist 與接受的樂觀更新、冪等鍵、互斥鎖），這支是**唯讀**的狀態摘要。
 * 兩者的失效條件與重試語意不同，合併會讓摘要的一次背景重讀動到比較表上
 * 使用者正在操作的那一列。
 */

const CONVERSATIONS_BASE = "/buyers/me/procurement/conversations";
const REQUESTS_BASE = "/buyers/me/procurement/requests";

const INVALID_SNAPSHOT_FAILURE: BuyerQuoteFailure = {
	code: "INVALID_RESPONSE",
	detail:
		"執行快照回傳不完整，無法確認這張詢價目前的狀態。請重新整理；這個畫面不會只顯示解析得出來的那幾項。",
	retryable: true,
};

function failureFromError(error: unknown): BuyerQuoteFailure {
	if (error instanceof ApiClientError)
		return buyerQuoteFailure(error.status, error.reason);
	return buyerQuoteFailure(0, "");
}

export type RuntimeSnapshotState =
	| { status: "idle" }
	| { status: "loading" }
	| { status: "success"; snapshot: RuntimeSnapshot }
	| { status: "error"; error: BuyerQuoteFailure };

/**
 * 「目前交易真相」卡的筆數。
 *
 * ⛔ 兩個欄位都是 `number | null`，而 `null` **不是 0**：`null` 代表這次沒有
 * 讀到清單，畫面要說「尚未取得」；0 只能來自一次成功的讀取。
 */
export interface RfqDealTruthCounts {
	responseCount: number | null;
	shortlistCount: number | null;
	/** 清單讀失敗的原因；成功或還沒讀時為 null。 */
	error: BuyerQuoteFailure | null;
}

const UNKNOWN_COUNTS: RfqDealTruthCounts = {
	responseCount: null,
	shortlistCount: null,
	error: null,
};

export interface RfqRuntimeSnapshot {
	state: RuntimeSnapshotState;
	/** 目前的發布結果；快照還沒讀到或裡面沒有這型 artifact 時為 null。 */
	publication: PublicationResultView | null;
	counts: RfqDealTruthCounts;
	refresh: () => Promise<void>;
}

/**
 * @param conversationId 對話身分；null＝沒有可讀的對話，整支 hook 靜止不發請求。
 * @param requestId      詢價身分；null＝不讀回價清單，筆數維持「尚未取得」。
 */
export function useRfqRuntimeSnapshot(
	conversationId: string | null,
	requestId: string | null,
): RfqRuntimeSnapshot {
	const [state, setState] = React.useState<RuntimeSnapshotState>({
		status: "idle",
	});
	const [counts, setCounts] =
		React.useState<RfqDealTruthCounts>(UNKNOWN_COUNTS);
	// 每次 refresh 遞增：晚回來的舊請求不得覆寫新請求的結果（同
	// useBuyerQuoteComparison 的 generation 手法）。
	const generationRef = React.useRef(0);

	const refresh = React.useCallback(async () => {
		if (!conversationId) {
			setState({ status: "idle" });
			setCounts(UNKNOWN_COUNTS);
			return;
		}
		generationRef.current += 1;
		const generation = generationRef.current;
		const isCurrent = () => generation === generationRef.current;
		setState({ status: "loading" });

		const snapshotRead = (async () => {
			try {
				const env = await apiClient.authed.get<unknown>(
					`${CONVERSATIONS_BASE}/${encodeURIComponent(conversationId)}/runtime-snapshot`,
				);
				if (!isCurrent()) return;
				const parsed = runtimeSnapshotSchema.safeParse(env.data);
				if (!parsed.success) {
					setState({ status: "error", error: INVALID_SNAPSHOT_FAILURE });
					return;
				}
				setState({ status: "success", snapshot: parsed.data });
			} catch (error) {
				if (!isCurrent()) return;
				setState({ status: "error", error: failureFromError(error) });
			}
		})();

		const countsRead = (async () => {
			if (!requestId) {
				if (isCurrent()) setCounts(UNKNOWN_COUNTS);
				return;
			}
			try {
				const env = await apiClient.authed.get<unknown>(
					`${REQUESTS_BASE}/${encodeURIComponent(requestId)}/seller-responses`,
				);
				if (!isCurrent()) return;
				const parsed = buyerFacingSellerResponseListSchema.safeParse(env.data);
				if (!parsed.success) {
					// 清單有一筆壞掉就整份不採用（該 schema 的既有立場）：只數得出
					// 解析成功的那幾筆，會把「已收到 N 筆」講成一個偏小的數字。
					setCounts({
						responseCount: null,
						shortlistCount: null,
						error: {
							code: "INVALID_RESPONSE",
							detail:
								"回價清單回傳不完整，無法確認收到哪些版本，因此這裡不顯示筆數。",
							retryable: true,
						},
					});
					return;
				}
				setCounts({
					responseCount: parsed.data.length,
					shortlistCount: parsed.data.filter((row) => row.shortlisted).length,
					error: null,
				});
			} catch (error) {
				if (!isCurrent()) return;
				setCounts({
					responseCount: null,
					shortlistCount: null,
					error: failureFromError(error),
				});
			}
		})();

		await Promise.all([snapshotRead, countsRead]);
	}, [conversationId, requestId]);

	React.useEffect(() => {
		void refresh();
		return () => {
			generationRef.current += 1;
		};
	}, [refresh]);

	const publication = React.useMemo(
		() =>
			state.status === "success"
				? selectPublicationResult(state.snapshot)
				: null,
		[state],
	);

	return { state, publication, counts, refresh };
}
