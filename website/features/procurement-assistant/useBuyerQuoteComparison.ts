"use client";

import { commonStoreCardClosedStore } from "@hawkr/ui-text/keys";
import * as React from "react";
import { buyerProcurementClient } from "@/lib/connect/procurement";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import {
	acceptanceFromProto,
	type BuyerFacingSellerResponse,
	type BuyerQuoteAcceptanceCommand,
	type BuyerQuoteAcceptanceResponse,
	type BuyerQuoteFailure,
	type BuyerQuoteResult,
	type QuoteShortlistTarget,
	sellerResponseFromProto,
} from "./buyer-quote-contract";
import { procurementFailureFrom } from "./errors";
import { createExclusiveMutationGuard } from "./session-operation";

/**
 * 買家端報價比較與接受的資料層。
 *
 * 與 `useProcurementSession` 分開的理由：那支綁的是一輪對話的 session 身分
 * （turn、revision、candidate set），而比較與接受綁的是**一張詢價與一個
 * immutable 版本**，兩者的失效條件不同。塞進同一支 hook 會讓「換一輪對話」
 * 這件事誤傷已經送出的接受請求。
 */

/** 一頁最多 100 筆（契約上限）。 */
const RESPONSES_PAGE_SIZE = 100;

/** 每次送出動作產生一個 client_request_id；沿用同一個值重送是呼叫端的責任。 */
function newClientRequestId(): string {
	return crypto.randomUUID();
}

export type QuoteListState =
	| { status: "loading" }
	| { status: "success"; responses: BuyerFacingSellerResponse[] }
	| { status: "error"; error: BuyerQuoteFailure };

export interface BuyerQuoteComparison {
	list: QuoteListState;
	refresh: () => Promise<void>;
	setShortlisted: (
		target: QuoteShortlistTarget,
		shortlisted: boolean,
	) => Promise<BuyerQuoteResult<boolean>>;
	acceptQuote: (
		command: BuyerQuoteAcceptanceCommand,
	) => Promise<BuyerQuoteResult<BuyerQuoteAcceptanceResponse>>;
	newClientRequestId: () => string;
}

const STALE_CONTEXT = (detail: string): BuyerQuoteFailure => ({
	code: "STALE_CONTEXT",
	detail,
	retryable: false,
});

export function useBuyerQuoteComparison(
	requestId: string | null,
	publicationId: string | null,
): BuyerQuoteComparison {
	const defaults = useDefaultErrorText();
	const ut = useUiText();
	const closedStoreName = ut(commonStoreCardClosedStore());
	const [list, setList] = React.useState<QuoteListState>({
		status: "loading",
	});
	// 同一列的加入／移除必須序列化：兩個 request 交錯時伺服器的處理順序可以與
	// 回應順序不同，畫面會停在伺服器已經不承認的狀態。不同列仍可並行。
	const shortlistGuard = React.useMemo(createExclusiveMutationGuard, []);
	const generationRef = React.useRef(0);

	const refresh = React.useCallback(async () => {
		if (!requestId || !publicationId) {
			setList({
				status: "error",
				error: STALE_CONTEXT("目前沒有可比較的詢價，請回到對話重新開始。"),
			});
			return;
		}
		generationRef.current += 1;
		const generation = generationRef.current;
		setList({ status: "loading" });
		try {
			const client = buyerProcurementClient();
			const responses: BuyerFacingSellerResponse[] = [];
			let pageToken = "";
			do {
				const res = await client.listSellerResponses({
					requestId,
					publicationId,
					pageSize: RESPONSES_PAGE_SIZE,
					pageToken,
				});
				if (generation !== generationRef.current) return;
				for (const item of res.sellerResponses) {
					const view = sellerResponseFromProto(item, closedStoreName);
					if (view) responses.push(view);
				}
				if (res.nextPageToken !== "" && res.nextPageToken === pageToken) {
					throw new Error(
						"ListSellerResponses next_page_token did not advance",
					);
				}
				pageToken = res.nextPageToken;
			} while (pageToken !== "");
			setList({ status: "success", responses });
		} catch (error) {
			if (generation !== generationRef.current) return;
			setList({
				status: "error",
				error: procurementFailureFrom(error, defaults),
			});
		}
	}, [requestId, publicationId, closedStoreName, defaults]);

	React.useEffect(() => {
		void refresh();
		return () => {
			generationRef.current += 1;
			shortlistGuard.invalidate();
		};
	}, [refresh, shortlistGuard]);

	const setShortlisted = React.useCallback(
		async (
			target: QuoteShortlistTarget,
			shortlisted: boolean,
		): Promise<BuyerQuoteResult<boolean>> => {
			if (!requestId) {
				return {
					ok: false,
					error: STALE_CONTEXT("目前沒有可比較的詢價，請回到對話重新開始。"),
				};
			}
			const key = `${target.sellerResponseId}:${target.sellerResponseVersion}`;
			const lease = shortlistGuard.begin(key);
			// 同一列還有 request 在途：直接當成沒有按過，不排隊也不覆寫。
			if (!lease) return { ok: true, data: shortlisted };
			// 樂觀更新：兩支方法都是目標狀態的方法（重送結果相同），失敗時把該列還原即可，
			// 不必整張表重讀。
			const apply = (value: boolean) =>
				setList((current) =>
					current.status === "success"
						? {
								status: "success",
								responses: current.responses.map((response) =>
									response.sellerResponseId === target.sellerResponseId &&
									response.sellerResponseVersion ===
										target.sellerResponseVersion
										? { ...response, shortlisted: value }
										: response,
								),
							}
						: current,
				);
			apply(shortlisted);
			try {
				const client = buyerProcurementClient();
				const request = {
					requestId,
					sellerResponseId: target.sellerResponseId,
					sellerResponseVersion: target.sellerResponseVersion,
				};
				if (shortlisted) await client.putShortlistEntry(request);
				else await client.deleteShortlistEntry(request);
				return { ok: true, data: shortlisted };
			} catch (error) {
				apply(!shortlisted);
				return { ok: false, error: procurementFailureFrom(error, defaults) };
			} finally {
				shortlistGuard.finish(lease);
			}
		},
		[requestId, shortlistGuard, defaults],
	);

	const acceptQuote = React.useCallback(
		async (
			command: BuyerQuoteAcceptanceCommand,
		): Promise<BuyerQuoteResult<BuyerQuoteAcceptanceResponse>> => {
			if (!requestId) {
				return {
					ok: false,
					error: STALE_CONTEXT("目前沒有可接受的詢價，請回到對話重新開始。"),
				};
			}
			try {
				const res = await buyerProcurementClient().acceptSellerQuote({
					requestId,
					clientRequestId: command.clientRequestId,
					sellerResponseId: command.sellerResponseId,
					expectedSellerResponseVersion: command.expectedSellerResponseVersion,
					expectedSellerFacingHash: command.expectedSellerFacingHash,
				});
				return { ok: true, data: acceptanceFromProto(res) };
			} catch (error) {
				return { ok: false, error: procurementFailureFrom(error, defaults) };
			}
		},
		[requestId, defaults],
	);

	return {
		list,
		refresh,
		setShortlisted,
		acceptQuote,
		newClientRequestId,
	};
}
