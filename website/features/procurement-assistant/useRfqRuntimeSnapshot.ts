"use client";

import * as React from "react";
import { buyerProcurementClient } from "@/lib/connect/procurement";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import type { BuyerQuoteFailure } from "./buyer-quote-contract";
import { procurementFailureFrom } from "./errors";
import {
	type RuntimeSnapshot,
	runtimeSnapshotFromProto,
} from "./runtime-snapshot-contract";
import type { PublishedRfqPublication } from "./types";

/**
 * 稿 R-02 三區版面的資料層。
 *
 * 兩個讀取、一支 hook：
 *   1. `GetRuntimeSnapshot`：發布結果與進度事實。
 *   2. `ListSellerResponses`：「目前交易真相」卡的筆數。沒有第二個讀取就數不出
 *      「已收到 N 筆」，而猜一個 0 出來正是這張卡存在的反面。
 *
 * 兩個讀取獨立成敗：發布結果讀到、回價清單讀不到時，畫面照樣呈現發布事實，只把
 * 筆數留成「尚未取得」。綁成同一個 all-or-nothing 會讓一次逾時把已知的事實一起藏掉。
 *
 * 與 `useBuyerQuoteComparison` 分開的理由：那支是**可操作**的比較清單（含
 * shortlist 與接受的樂觀更新、互斥鎖），這支是**唯讀**的狀態摘要。
 * 兩者的失效條件與重試語意不同，合併會讓摘要的一次背景重讀動到比較表上
 * 使用者正在操作的那一列。
 */

/** 一頁最多 100 筆（契約上限）。 */
const RESPONSES_PAGE_SIZE = 100;

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
	/** 目前的發布結果；快照還沒讀到或還沒有發布時為 null。 */
	publication: PublishedRfqPublication | null;
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
	const defaults = useDefaultErrorText();
	const [state, setState] = React.useState<RuntimeSnapshotState>({
		status: "idle",
	});
	const [counts, setCounts] =
		React.useState<RfqDealTruthCounts>(UNKNOWN_COUNTS);
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
		const client = buyerProcurementClient();

		let snapshot: RuntimeSnapshot;
		try {
			snapshot = runtimeSnapshotFromProto(
				await client.getRuntimeSnapshot({ conversationId }),
			);
		} catch (error) {
			if (!isCurrent()) return;
			setState({
				status: "error",
				error: procurementFailureFrom(error, defaults),
			});
			setCounts(UNKNOWN_COUNTS);
			return;
		}
		if (!isCurrent()) return;
		setState({ status: "success", snapshot });

		const publicationId = snapshot.publication?.publicationId;
		if (!requestId || !publicationId) {
			setCounts(UNKNOWN_COUNTS);
			return;
		}
		try {
			let responseCount = 0;
			let shortlistCount = 0;
			let pageToken = "";
			do {
				const res = await client.listSellerResponses({
					requestId,
					publicationId,
					pageSize: RESPONSES_PAGE_SIZE,
					pageToken,
				});
				if (!isCurrent()) return;
				responseCount += res.sellerResponses.length;
				shortlistCount += res.sellerResponses.filter(
					(r) => r.shortlisted,
				).length;
				if (res.nextPageToken !== "" && res.nextPageToken === pageToken) {
					throw new Error(
						"ListSellerResponses next_page_token did not advance",
					);
				}
				pageToken = res.nextPageToken;
			} while (pageToken !== "");
			setCounts({ responseCount, shortlistCount, error: null });
		} catch (error) {
			if (!isCurrent()) return;
			setCounts({
				responseCount: null,
				shortlistCount: null,
				error: procurementFailureFrom(error, defaults),
			});
		}
	}, [conversationId, requestId, defaults]);

	React.useEffect(() => {
		void refresh();
		return () => {
			generationRef.current += 1;
		};
	}, [refresh]);

	const publication = React.useMemo(
		() => (state.status === "success" ? state.snapshot.publication : null),
		[state],
	);

	return { state, publication, counts, refresh };
}
