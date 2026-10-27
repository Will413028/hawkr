"use client";

import * as React from "react";
import { ApiClientError, apiClient } from "@/lib/api/client";
import {
	type BuyerFacingSellerResponse,
	type BuyerQuoteAcceptanceCommand,
	type BuyerQuoteAcceptanceResponse,
	type BuyerQuoteFailure,
	type BuyerQuoteResult,
	buyerFacingSellerResponseListSchema,
	buyerQuoteAcceptanceBody,
	buyerQuoteAcceptanceResponseSchema,
	buyerQuoteFailure,
	type QuoteShortlistTarget,
} from "./buyer-quote-contract";
import { createExclusiveMutationGuard } from "./session-operation";

/**
 * 買家端報價比較與接受的資料層（issue #2538 A8 前半）。
 *
 * 與 `useProcurementSession` 分開的理由：那支綁的是一輪對話的 session 身分
 * （turn／revision／candidate set），而比較與接受綁的是**一張 RFQ 與一個
 * immutable 版本**，兩者的失效條件不同。塞進同一支 hook 會讓「換一輪對話」
 * 這件事誤傷已經送出的接受請求。
 */

const BASE = "/buyers/me/procurement/requests";

function requestPath(requestId: string): string {
	return `${BASE}/${encodeURIComponent(requestId)}`;
}

function shortlistPath(
	requestId: string,
	target: QuoteShortlistTarget,
): string {
	return `${requestPath(requestId)}/shortlist/${encodeURIComponent(
		target.sellerResponseId,
	)}/versions/${encodeURIComponent(target.sellerResponseVersion)}`;
}

/**
 * `Idempotency-Key`：8–128 字、`^[A-Za-z0-9._:-]+$`（Gateway §6.1.0）。
 *
 * ⚠️ 一次「送出動作」一把，重試沿用同一把——沿用是呼叫端的責任，這裡只負責
 * 產生。換一把重送同一個接受會在伺服器上多出第二筆。
 */
function newIdempotencyKey(): string {
	const random =
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID().replace(/-/g, "")
			: `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`;
	return `bqa-${random}`;
}

/**
 * 從錯誤 payload 機會性地讀出既有訂單編號。
 *
 * `RFQ_CHECKOUT_ALREADY_ACTIVE` 契約上「回既有 orderNo」，但沒有明訂它掛在
 * 哪一層，所以這裡只認最直接的兩種位置，讀不到就回 undefined——畫面會退回
 * 訂單列表而不是導去一個猜出來的單號。
 */
function readOrderNo(payload: unknown): string | undefined {
	if (!payload || typeof payload !== "object") return undefined;
	const record = payload as Record<string, unknown>;
	const direct = record.orderNo;
	if (typeof direct === "string" && direct) return direct;
	const nested = record.data;
	if (nested && typeof nested === "object") {
		const value = (nested as Record<string, unknown>).orderNo;
		if (typeof value === "string" && value) return value;
	}
	return undefined;
}

function failureFromError(error: unknown): BuyerQuoteFailure {
	if (error instanceof ApiClientError) {
		return buyerQuoteFailure(
			error.status,
			error.reason,
			readOrderNo(error.payload),
		);
	}
	return buyerQuoteFailure(0, "");
}

const INVALID_LIST_FAILURE: BuyerQuoteFailure = {
	code: "INVALID_RESPONSE",
	detail:
		"報價清單回傳不完整，無法確認收到哪些版本。請重新整理；這個畫面不會只顯示解析得出來的那幾筆。",
	retryable: true,
};

const INVALID_ACK_FAILURE: BuyerQuoteFailure = {
	code: "INVALID_RESPONSE",
	detail:
		"接受回執不完整，無法確認這次操作的結果。請重新整理確認狀態；這個畫面不會宣稱已接受。",
	retryable: true,
};

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
	newIdempotencyKey: () => string;
}

export function useBuyerQuoteComparison(
	requestId: string | null,
): BuyerQuoteComparison {
	const [list, setList] = React.useState<QuoteListState>({
		status: "loading",
	});
	// 同一列的加入／移除必須序列化：兩個 request 交錯時伺服器的處理順序可以與
	// 回應順序不同，畫面會停在伺服器已經不承認的狀態。不同列仍可並行。
	const shortlistGuard = React.useMemo(createExclusiveMutationGuard, []);
	const generationRef = React.useRef(0);

	const refresh = React.useCallback(async () => {
		if (!requestId) {
			setList({
				status: "error",
				error: {
					code: "STALE_CONTEXT",
					detail: "目前沒有可比較的詢價，請回到對話重新開始。",
					retryable: false,
				},
			});
			return;
		}
		generationRef.current += 1;
		const generation = generationRef.current;
		setList({ status: "loading" });
		try {
			const env = await apiClient.authed.get<unknown>(
				`${requestPath(requestId)}/seller-responses`,
			);
			if (generation !== generationRef.current) return;
			const parsed = buyerFacingSellerResponseListSchema.safeParse(env.data);
			if (!parsed.success) {
				setList({ status: "error", error: INVALID_LIST_FAILURE });
				return;
			}
			setList({ status: "success", responses: parsed.data });
		} catch (error) {
			if (generation !== generationRef.current) return;
			setList({ status: "error", error: failureFromError(error) });
		}
	}, [requestId]);

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
					error: {
						code: "STALE_CONTEXT",
						detail: "目前沒有可比較的詢價，請回到對話重新開始。",
						retryable: false,
					},
				};
			}
			const key = `${target.sellerResponseId}:${target.sellerResponseVersion}`;
			const lease = shortlistGuard.begin(key);
			// 同一列還有 request 在途：直接當成沒有按過，不排隊也不覆寫。
			if (!lease) return { ok: true, data: shortlisted };
			const path = shortlistPath(requestId, target);
			// 樂觀更新：這兩支端點冪等且零 side effect，失敗時把該列還原即可，
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
				// ⛔ 不帶 Idempotency-Key：契約明訂這兩支不需要，加了只會讓連續兩次
				// 同向操作撞 409。
				if (shortlisted) await apiClient.authed.put(path, {});
				else await apiClient.authed.delete(path, {});
				return { ok: true, data: shortlisted };
			} catch (error) {
				apply(!shortlisted);
				return { ok: false, error: failureFromError(error) };
			} finally {
				shortlistGuard.finish(lease);
			}
		},
		[requestId, shortlistGuard],
	);

	const acceptQuote = React.useCallback(
		async (
			command: BuyerQuoteAcceptanceCommand,
		): Promise<BuyerQuoteResult<BuyerQuoteAcceptanceResponse>> => {
			if (!requestId) {
				return {
					ok: false,
					error: {
						code: "STALE_CONTEXT",
						detail: "目前沒有可接受的詢價，請回到對話重新開始。",
						retryable: false,
					},
				};
			}
			try {
				const env = await apiClient.authed.post<unknown>(
					`${requestPath(requestId)}/buyer-quote-acceptances`,
					{
						json: buyerQuoteAcceptanceBody(command),
						headers: { "Idempotency-Key": command.idempotencyKey },
					},
				);
				const parsed = buyerQuoteAcceptanceResponseSchema.safeParse(env.data);
				if (!parsed.success) {
					return { ok: false, error: INVALID_ACK_FAILURE };
				}
				return { ok: true, data: parsed.data };
			} catch (error) {
				return { ok: false, error: failureFromError(error) };
			}
		},
		[requestId],
	);

	return {
		list,
		refresh,
		setShortlisted,
		acceptQuote,
		newIdempotencyKey,
	};
}
