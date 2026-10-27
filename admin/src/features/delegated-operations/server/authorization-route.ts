import type { NextRequest, NextResponse } from "next/server";
import {
	DELEGATION_AUTHORIZATION_SOURCES,
	DELEGATION_REASON_MAX_LENGTH,
	DELEGATION_SOURCE_REFERENCE_MAX_LENGTH,
	DELEGATION_SOURCE_REFERENCE_MIN_LENGTH,
} from "../authorization";
import { hasAsciiControlCharacters, isSafeOpaqueId } from "../policy";
import { invalidRequest } from "./governance-route";

/**
 * 授權列五支寫入／查詢路由共用的請求側檢查（backend#2444）。
 *
 * 抽出來的理由與 `governance-route.ts` 同一條：開通與調整範圍收的是**同一組**
 * 「授權變成什麼」欄位（gateway 的 `validateDelegationAuthorizationScope` 也是一支
 * 共用函式）。兩支各寫一份，遲早有一支忘了某個界線，而寬的那一支會把一個必然被
 * 上游拒絕的請求送出去，操作者收到的是一句與欄位無關的通用錯誤。
 *
 * ⚠️ **這裡刻意不驗能力依賴閉包。** 閉包規則的唯一權威是 gateway 的 authz package
 * （經 `GET …/capabilities` 下發給瀏覽器）；在 BFF 再寫一份就是第三份副本，而它
 * 沒有任何東西會在後端改規則時提醒它更新。瀏覽器以下發的目錄擋在前、gateway 以
 * 9287 擋在後，中間這一層只負責形狀。
 */

export const MAX_AUTHORIZATION_REQUEST_BYTES = 8 * 1024;

/** gateway `delegationEffectiveUntilRE`：ISO 8601 UTC 秒精度，帶毫秒會被拒。 */
const EFFECTIVE_UNTIL_RE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/u;

const IDEMPOTENCY_KEY_RE = /^[A-Za-z0-9._:-]{8,128}$/u;

export interface DelegationAuthorizationScopeBody {
	source: string;
	sourceReference: string;
	grantedCapabilities: string[];
	effectiveUntil: string | null;
	operationReason: string;
}

export function isValidAuthorizationIdempotencyKey(
	value: unknown,
): value is string {
	return typeof value === "string" && IDEMPOTENCY_KEY_RE.test(value);
}

export function isRequestTooLarge(request: NextRequest): boolean {
	const declared = request.headers.get("content-length");
	return (
		!!declared &&
		/^\d+$/u.test(declared) &&
		Number(declared) > MAX_AUTHORIZATION_REQUEST_BYTES
	);
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseCapabilities(value: unknown): string[] | null {
	if (!Array.isArray(value) || value.length === 0) return null;
	const seen = new Set<string>();
	for (const raw of value) {
		if (
			typeof raw !== "string" ||
			raw.length === 0 ||
			raw.length > 64 ||
			hasAsciiControlCharacters(raw) ||
			seen.has(raw)
		) {
			return null;
		}
		seen.add(raw);
	}
	return [...seen];
}

/**
 * 逐項回不同的 code：全部壓成一個 `INVALID_REQUEST`，操作者唯一能做的就是把整張
 * 表單重填一次。這幾個界線都是他改得動的東西。
 */
export function parseAuthorizationScopeBody(
	body: unknown,
):
	| { ok: true; value: DelegationAuthorizationScopeBody }
	| { ok: false; response: NextResponse } {
	if (!isRecord(body)) {
		return {
			ok: false,
			response: invalidRequest("INVALID_REQUEST", "授權設定內容不完整"),
		};
	}
	const source = body.source;
	if (
		typeof source !== "string" ||
		!(DELEGATION_AUTHORIZATION_SOURCES as readonly string[]).includes(source)
	) {
		return {
			ok: false,
			response: invalidRequest(
				"INVALID_AUTHORIZATION_SOURCE",
				"授權依據必須是合作合約、工單或平台政策",
			),
		};
	}
	const sourceReference =
		typeof body.sourceReference === "string" ? body.sourceReference.trim() : "";
	const referenceLength = [...sourceReference].length;
	if (
		referenceLength < DELEGATION_SOURCE_REFERENCE_MIN_LENGTH ||
		referenceLength > DELEGATION_SOURCE_REFERENCE_MAX_LENGTH ||
		hasAsciiControlCharacters(sourceReference) ||
		!/\d/u.test(sourceReference)
	) {
		// 「至少含一個數字」是 `chk_sda_source_reference`：它要的是依據**編號**，
		// 不是「賣家說要開通」這種說明文字。
		return {
			ok: false,
			response: invalidRequest(
				"INVALID_SOURCE_REFERENCE",
				"依據編號必須為 4–200 字且至少含一個數字",
			),
		};
	}
	const grantedCapabilities = parseCapabilities(body.grantedCapabilities);
	if (!grantedCapabilities) {
		return {
			ok: false,
			response: invalidRequest(
				"INVALID_CAPABILITIES",
				"授權範圍必須至少包含一項能力且不得重複",
			),
		};
	}
	const rawUntil = body.effectiveUntil;
	if (
		rawUntil !== null &&
		rawUntil !== undefined &&
		(typeof rawUntil !== "string" || !EFFECTIVE_UNTIL_RE.test(rawUntil))
	) {
		return {
			ok: false,
			response: invalidRequest(
				"INVALID_EFFECTIVE_UNTIL",
				"到期時間必須為 ISO 8601 UTC 秒精度，無期限請送 null",
			),
		};
	}
	const operationReason =
		typeof body.operationReason === "string" ? body.operationReason.trim() : "";
	const reasonLength = [...operationReason].length;
	if (
		reasonLength < 1 ||
		reasonLength > DELEGATION_REASON_MAX_LENGTH ||
		hasAsciiControlCharacters(operationReason)
	) {
		return {
			ok: false,
			response: invalidRequest(
				"OPERATION_REASON_REQUIRED",
				"請填寫 1–200 字的操作理由",
			),
		};
	}
	return {
		ok: true,
		value: {
			source,
			sourceReference,
			grantedCapabilities,
			// 明確送 null 而不是省略：復權時舊列的期限一律被本欄取代（含取代成無期限）。
			effectiveUntil: typeof rawUntil === "string" ? rawUntil : null,
			operationReason,
		},
	};
}

export function isAuthorizationStoreId(value: unknown): value is string {
	return isSafeOpaqueId(value);
}
