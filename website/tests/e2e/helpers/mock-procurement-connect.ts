import type { Page, Route } from "@playwright/test";
import {
	connectRequestMessage,
	errorInfoDetail,
	fulfillConnect,
	fulfillConnectError,
	localizedMessageDetail,
} from "./mock-gateway";

/**
 * BuyerProcurementAppService 的 Connect mock（`/api/connect/hawkr.procurement.v1alpha1.BuyerProcurementAppService/{Method}`）。
 * 回應是 proto3 JSON：camelCase 欄位、enum 用 proto 的完整名稱（`RUN_STATUS_SUCCEEDED`）、int64 與 uint64 是字串、
 * 時間是 RFC3339。形狀以 `api/hawkr/procurement/v1alpha1/procurement.proto` 為準。
 *
 * server streaming（`WatchRun`）的回應是 Connect 的 streaming envelope：每個訊息一個 5 位元組標頭
 * （1 位元組旗標、4 位元組大端長度）加 JSON；最後一個旗標 0x02 的 end-stream 訊框。`route.fulfill` 一次送完整個
 * body，所以這裡的串流是「連線一開就整批收到、然後正常結束」；沒有 `finished` 就結束的串流，client 會依 cursor 重連。
 */

const SERVICE = "hawkr.procurement.v1alpha1.BuyerProcurementAppService";

function procurementMethod(method: string): RegExp {
	return new RegExp(`/api/connect/${SERVICE}/${method}(?:\\?|$)`);
}

function connectEnvelope(flags: number, json: unknown): Buffer {
	const payload = Buffer.from(JSON.stringify(json));
	const frame = Buffer.alloc(5 + payload.length);
	frame.writeUInt8(flags, 0);
	frame.writeUInt32BE(payload.length, 1);
	payload.copy(frame, 5);
	return frame;
}

/** 一個事件的 proto3 JSON：`WatchRunResponse`。 */
type WatchFrame = Record<string, unknown>;

/** 把一串事件以 streaming envelope 回給 `WatchRun`，並以正常的 end-stream 結束。 */
function fulfillWatchRun(
	route: Route,
	frames: readonly WatchFrame[],
): Promise<void> {
	return route.fulfill({
		status: 200,
		contentType: "application/connect+json",
		body: Buffer.concat([
			...frames.map((frame) => connectEnvelope(0x00, frame)),
			connectEnvelope(0x02, {}),
		]),
	});
}

/** `WatchRun` 請求（streaming envelope 的第一個訊息）。 */
function watchRunRequest(route: Route): {
	runId: string;
	cursor?: string;
} {
	const body = route.request().postDataBuffer();
	if (!body || body.length < 5) return { runId: "" };
	return JSON.parse(body.subarray(5).toString("utf8"));
}

export const watchFrame = {
	accepted(cursor: string, runId: string, conversationId: string): WatchFrame {
		return { cursor, runAccepted: { runId, conversationId } };
	},
	step(
		cursor: string,
		seq: number,
		code: string,
		status: "RUNNING" | "SUCCEEDED",
		text = "",
	): WatchFrame {
		return {
			cursor,
			step: { seq, step: { code, status: `STEP_STATUS_${status}`, text } },
		};
	},
	activity(
		cursor: string,
		row: {
			rowId: string;
			code: string;
			label: string;
			status: "PENDING" | "RUNNING" | "SUCCEEDED";
			parentRowId?: string;
			count?: number;
		},
	): WatchFrame {
		return {
			cursor,
			activity: {
				rowId: row.rowId,
				attempt: 1,
				maxAttempts: 1,
				code: row.code,
				label: row.label,
				text: "",
				status: `STEP_STATUS_${row.status}`,
				...(row.parentRowId ? { parentRowId: row.parentRowId } : {}),
				...(row.count === undefined ? {} : { count: row.count }),
			},
		};
	},
	turnResult(cursor: string, result: Record<string, unknown>): WatchFrame {
		return { cursor, turnResult: result };
	},
	finished(
		cursor: string,
		status: "SUCCEEDED" | "FAILED" | "CANCELLED" = "SUCCEEDED",
		failureReason?: string,
	): WatchFrame {
		return {
			cursor,
			finished: {
				status: `RUN_STATUS_${status}`,
				...(failureReason
					? { failureReason: `RUN_FAILURE_REASON_${failureReason}` }
					: {}),
			},
		};
	},
};

/** 對 Connect 一元方法的處理器：收到請求訊息，回應訊息（或丟 `ConnectFailure`）。 */
export class ConnectFailure extends Error {
	constructor(
		readonly code: Parameters<typeof fulfillConnectError>[1],
		message: string,
		readonly reason?: string,
	) {
		super(message);
	}
}

export async function mockProcurementUnary(
	page: Page,
	method: string,
	handler: (request: Record<string, unknown>) => unknown,
): Promise<void> {
	await page.route(procurementMethod(method), (route) => {
		try {
			void fulfillConnect(route, handler(connectRequestMessage(route)));
		} catch (error) {
			if (!(error instanceof ConnectFailure)) throw error;
			void fulfillConnectError(route, error.code, error.message, undefined, [
				...(error.reason ? [errorInfoDetail(error.reason)] : []),
				localizedMessageDetail(error.message),
			]);
		}
	});
}

export async function mockWatchRun(
	page: Page,
	handler: (request: {
		runId: string;
		cursor?: string;
	}) => readonly WatchFrame[],
): Promise<void> {
	await page.route(procurementMethod("WatchRun"), (route) => {
		void fulfillWatchRun(route, handler(watchRunRequest(route)));
	});
}

/** 這個 spec 共用的 `TurnResult`（`requestId`、`revisionId` 等必填 ID 與預設的回覆文字）。 */
export function turnResultJson(
	ids: { requestId: string; conversationId: string; revisionId: string },
	extra: Record<string, unknown> = {},
) {
	return {
		...ids,
		reply: "好的，請問數量？",
		missingRequired: [],
		recordedThisTurn: [],
		...extra,
	};
}


/** 一元方法的處理器回傳它，代表這個請求要在網路層中斷（沒有回應）。 */
export const ABORT_REQUEST = Symbol("abort-request");

/**
 * 同 `mockProcurementUnary`，另外允許處理器回傳 `ABORT_REQUEST` 讓請求在網路層失敗。
 * 後註冊的 route 優先，所以 spec 可以在 helper 的預設之後再覆寫某個方法。
 */
export async function mockProcurementUnaryAbortable(
	page: Page,
	method: string,
	handler: (request: Record<string, unknown>) => unknown | typeof ABORT_REQUEST,
): Promise<void> {
	await page.route(procurementMethod(method), (route) => {
		try {
			const out = handler(connectRequestMessage(route));
			if (out === ABORT_REQUEST) {
				void route.abort("failed");
				return;
			}
			void fulfillConnect(route, out);
		} catch (error) {
			if (!(error instanceof ConnectFailure)) throw error;
			void fulfillConnectError(route, error.code, error.message, undefined, [
				...(error.reason ? [errorInfoDetail(error.reason)] : []),
				localizedMessageDetail(error.message),
			]);
		}
	});
}
