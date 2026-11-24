import { RejectionReason } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import {
	mapSlots,
	type NormalizedProcurementTurnResult,
} from "./session-logic";
import type {
	ClarifyQuestion,
	ProcurementTurnResult,
	RecordedField,
} from "./types";

/**
 * 舊 SSE 串流終態（`turn-stream.ts`）與同步 `/turns` 回應的欄位轉成畫面用的 `ProcurementTurnResult`。
 * 只服務還沒換成 SubmitTurn＋WatchRun 的那條舊路徑；換掉之後整支刪除。
 */

function normalizeOptions(
	raw: unknown,
): Array<{ optionId: string; label: string }> {
	if (!Array.isArray(raw)) return [];
	return raw.flatMap((o) => {
		if (!o || typeof o !== "object") return [];
		const opt = o as Record<string, unknown>;
		if (typeof opt.optionId !== "string" || typeof opt.label !== "string")
			return [];
		return [{ optionId: opt.optionId, label: opt.label }];
	});
}

function normalizeQuestion(raw: unknown): ClarifyQuestion | null {
	if (!raw || typeof raw !== "object") return null;
	const r = raw as Record<string, unknown>;
	if (typeof r.slotKey !== "string" || typeof r.prompt !== "string")
		return null;
	const unitOptions = normalizeOptions(r.unitOptions);
	return {
		slotKey: r.slotKey,
		prompt: r.prompt,
		options: normalizeOptions(r.options),
		unitOptions: unitOptions.length > 0 ? unitOptions : undefined,
		allowFreeText: r.allowFreeText !== false,
		canSkip: r.canSkip === true,
	};
}

function normalizeRecorded(raw: unknown): RecordedField[] {
	if (!Array.isArray(raw)) return [];
	return raw.flatMap((item) => {
		if (!item || typeof item !== "object") return [];
		const r = item as Record<string, unknown>;
		if (typeof r.key !== "string" || typeof r.value !== "string") return [];
		return [
			{
				key: r.key,
				label: typeof r.label === "string" && r.label ? r.label : r.key,
				value: r.value,
			},
		];
	});
}

export function turnResultFromLegacy(
	data: NormalizedProcurementTurnResult,
): ProcurementTurnResult {
	return {
		requestId: data.requestId,
		revisionId: data.revisionId,
		conversationId: data.conversationId,
		slots: mapSlots(data.slots),
		rejected: data.rejected.map((r) => ({
			key: r.key,
			reason: RejectionReason.UNSPECIFIED,
		})),
		missingRequired: data.missingRequired,
		reply: data.reply,
		blocks: data.blocks,
		textFallback: data.textFallback,
		question: normalizeQuestion(data.question),
		recordedThisTurn: normalizeRecorded(data.recordedThisTurn),
		interrupt: data.interruptId
			? { interruptId: data.interruptId, reason: data.interruptKind ?? "" }
			: null,
	};
}
