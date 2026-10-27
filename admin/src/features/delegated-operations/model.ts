import {
	hasAsciiControlCharacters,
	isSafeOpaqueId,
	isSafeReference,
	parseDelegatedOperationCapabilities,
} from "./policy";

export type { DelegatedOperationCapability } from "./policy";

export interface DelegatedOperationTarget {
	storeId: string;
	sellerAccountId: string;
	storeName: string;
}

export interface DelegatedOperationActor {
	adminId: string;
	displayName: string;
}

export interface DelegatedOperationReasonOption {
	code: string;
	label: string;
	caseReferenceRequired?: boolean;
}

export interface DelegatedOperationPreflight {
	allowed: true;
	target: DelegatedOperationTarget;
	actor: DelegatedOperationActor;
	reasonOptions: DelegatedOperationReasonOption[];
	capabilities: import("./policy").DelegatedOperationCapability[];
	ttlSeconds: number;
	/** Idle window the server will enforce; the dialog must not restate it. */
	idleSeconds: number;
	caseReferenceRequired: boolean;
}

export type DelegatedOperationSessionStatus = "active" | "expired" | "revoked";

/**
 * Browser-safe session context. It intentionally has no operation credential,
 * seller credential, bearer token, or server-owned operation-session ID.
 */
export interface DelegatedOperationSessionSummary {
	target: DelegatedOperationTarget;
	actor: DelegatedOperationActor;
	reason: DelegatedOperationReasonOption;
	caseReference: string | null;
	capabilities: import("./policy").DelegatedOperationCapability[];
	issuedAt: string;
	expiresAt: string;
	status: DelegatedOperationSessionStatus;
	revision: number;
}

export interface StartDelegatedOperationRequest {
	storeId: string;
	reasonCode: string;
	caseReference: string | null;
	/**
	 * Optional shorter window chosen in the dialog. Absent means "use whatever
	 * preflight issued"; the server still clamps to the preflight value, so this
	 * can only narrow the audited window, never widen it.
	 */
	ttlSeconds?: number;
}

export type DelegatedOperationState =
	| "active"
	| "expiring"
	| "expired"
	| "revoked"
	| "end_unknown"
	| "target_mismatch"
	| "invalid";

export const DELEGATED_OPERATION_MAX_TTL_SECONDS = 30 * 60;
export const DELEGATED_OPERATION_MIN_IDLE_SECONDS = 60;
export const DELEGATED_OPERATION_EXPIRING_SECONDS = 5 * 60;
export const DELEGATED_OPERATION_CLOCK_SKEW_SECONDS = 60;

type UnknownRecord = Record<string, unknown>;

function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseLabel(value: unknown, maxLength = 160): string | null {
	if (
		typeof value !== "string" ||
		value.length === 0 ||
		value.length > maxLength ||
		value !== value.trim() ||
		hasAsciiControlCharacters(value)
	) {
		return null;
	}
	return value;
}

function parseTarget(value: unknown): DelegatedOperationTarget | null {
	if (!isRecord(value)) return null;
	const storeName = parseLabel(value.storeName);
	if (
		!isSafeOpaqueId(value.storeId) ||
		!isSafeOpaqueId(value.sellerAccountId) ||
		!storeName
	) {
		return null;
	}
	return {
		storeId: value.storeId,
		sellerAccountId: value.sellerAccountId,
		storeName,
	};
}

function parseActor(value: unknown): DelegatedOperationActor | null {
	if (!isRecord(value)) return null;
	const displayName = parseLabel(value.displayName);
	if (!isSafeOpaqueId(value.adminId) || !displayName) return null;
	return { adminId: value.adminId, displayName };
}

function parseReason(value: unknown): DelegatedOperationReasonOption | null {
	if (!isRecord(value)) return null;
	const label = parseLabel(value.label);
	if (!isSafeOpaqueId(value.code) || !label) return null;
	if (
		value.caseReferenceRequired !== undefined &&
		typeof value.caseReferenceRequired !== "boolean"
	) {
		return null;
	}
	return {
		code: value.code,
		label,
		...(value.caseReferenceRequired === undefined
			? {}
			: { caseReferenceRequired: value.caseReferenceRequired }),
	};
}

/** Only canonical UTC RFC 3339 timestamps cross this trust boundary. */
export function parseDelegatedOperationExpiry(value: unknown): string | null {
	if (
		typeof value !== "string" ||
		!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/u.test(value)
	) {
		return null;
	}
	const epoch = Date.parse(value);
	if (!Number.isFinite(epoch)) return null;
	const canonicalInput = value.includes(".")
		? value.replace(
				/\.(\d{1,3})Z$/u,
				(_match, fraction: string) => `.${fraction.padEnd(3, "0")}Z`,
			)
		: value.replace(/Z$/u, ".000Z");
	return new Date(epoch).toISOString() === canonicalInput ? value : null;
}

export function parseDelegatedOperationPreflight(
	value: unknown,
): DelegatedOperationPreflight | null {
	if (!isRecord(value)) return null;
	const target = parseTarget(value.target);
	const actor = parseActor(value.actor);
	const capabilities = parseDelegatedOperationCapabilities(value.capabilities);
	if (
		value.allowed !== true ||
		!target ||
		!actor ||
		!capabilities ||
		!Number.isSafeInteger(value.ttlSeconds) ||
		(value.ttlSeconds as number) <= 0 ||
		(value.ttlSeconds as number) > DELEGATED_OPERATION_MAX_TTL_SECONDS ||
		!Number.isSafeInteger(value.idleSeconds) ||
		(value.idleSeconds as number) < DELEGATED_OPERATION_MIN_IDLE_SECONDS ||
		(value.idleSeconds as number) > DELEGATED_OPERATION_MAX_TTL_SECONDS ||
		typeof value.caseReferenceRequired !== "boolean" ||
		!Array.isArray(value.reasonOptions) ||
		value.reasonOptions.length === 0
	) {
		return null;
	}

	const reasonOptions: DelegatedOperationReasonOption[] = [];
	const reasonCodes = new Set<string>();
	for (const option of value.reasonOptions) {
		const parsed = parseReason(option);
		if (!parsed || reasonCodes.has(parsed.code)) return null;
		reasonCodes.add(parsed.code);
		reasonOptions.push(parsed);
	}

	return {
		allowed: true,
		target,
		actor,
		reasonOptions,
		capabilities,
		ttlSeconds: value.ttlSeconds as number,
		idleSeconds: value.idleSeconds as number,
		caseReferenceRequired: value.caseReferenceRequired,
	};
}

export function isDelegatedOperationPreflightDenied(value: unknown): boolean {
	return isRecord(value) && value.allowed === false;
}

/**
 * 被拒絕時的原因。與 `isDelegatedOperationPreflightDenied` 分成兩支是刻意的：
 * 「是不是拒絕」必須在**任何**情況下都答得出來（fail-closed），「為什麼拒絕」
 * 則可以缺席退回通用句 —— 兩者合成一支，一份沒帶 `deniedLabel` 的回應就會從
 * 「拒絕」掉進「契約錯誤」，操作者看到的是 502 而不是原因。
 *
 * 前端**不建** `deniedCode → 中文` 對照表：拒絕原因的措辭由後端逐次下發
 * （`gateway/internal/handler/admin_delegation.go` 的 `deniedLabel`，值來自
 * 單體 `backend/internal/modules/identity/delegation.go` 的 `deniedLabel`），
 * 同 `actionLabel`／`changedFields[].label` 的理由。`deniedCode` 只作技術參考顯示。
 */
export interface DelegatedOperationPreflightDenial {
	allowed: false;
	/** 9264／9265／9268／9269／9272／9286…（`pkg/errors/bizcode/bizcode.go`）。 */
	deniedCode: number;
	deniedLabel: string;
}

export function parseDelegatedOperationPreflightDenial(
	value: unknown,
): DelegatedOperationPreflightDenial | null {
	if (!isRecord(value) || value.allowed !== false) return null;
	// 與 storeName／displayName 同一支校驗：長度上限＋禁控制字元＋禁前後空白。
	// 這是上游來的自由文字，會直接進畫面，不能只因為「它是我們自己的服務」就放行。
	const deniedLabel = parseLabel(value.deniedLabel, 120);
	if (
		!deniedLabel ||
		!Number.isSafeInteger(value.deniedCode) ||
		(value.deniedCode as number) <= 0
	) {
		return null;
	}
	return {
		allowed: false,
		deniedCode: value.deniedCode as number,
		deniedLabel,
	};
}

export function parseDelegatedOperationSessionSummary(
	value: unknown,
): DelegatedOperationSessionSummary | null {
	if (!isRecord(value)) return null;
	const target = parseTarget(value.target);
	const actor = parseActor(value.actor);
	const reason = parseReason(value.reason);
	const capabilities = parseDelegatedOperationCapabilities(value.capabilities);
	const issuedAt = parseDelegatedOperationExpiry(value.issuedAt);
	const expiresAt = parseDelegatedOperationExpiry(value.expiresAt);
	const status = value.status;
	const caseReference = value.caseReference;
	const issuedAtEpoch = issuedAt ? Date.parse(issuedAt) : Number.NaN;
	const expiresAtEpoch = expiresAt ? Date.parse(expiresAt) : Number.NaN;
	const lifetimeSeconds = (expiresAtEpoch - issuedAtEpoch) / 1000;
	if (
		!target ||
		!actor ||
		!reason ||
		!capabilities ||
		!issuedAt ||
		!expiresAt ||
		!Number.isFinite(lifetimeSeconds) ||
		lifetimeSeconds <= 0 ||
		lifetimeSeconds > DELEGATED_OPERATION_MAX_TTL_SECONDS ||
		(status !== "active" && status !== "expired" && status !== "revoked") ||
		!Number.isSafeInteger(value.revision) ||
		(value.revision as number) < 0 ||
		(caseReference !== null && !isSafeReference(caseReference))
	) {
		return null;
	}

	return {
		target,
		actor,
		reason,
		caseReference,
		capabilities,
		issuedAt,
		expiresAt,
		status,
		revision: value.revision as number,
	};
}

export function parseStartDelegatedOperationRequest(
	value: unknown,
): StartDelegatedOperationRequest | null {
	if (!isRecord(value)) return null;
	const caseReference = value.caseReference ?? null;
	const ttlSeconds = value.ttlSeconds;
	if (
		!isSafeOpaqueId(value.storeId) ||
		!isSafeOpaqueId(value.reasonCode) ||
		(caseReference !== null && !isSafeReference(caseReference)) ||
		(ttlSeconds !== undefined &&
			(!Number.isSafeInteger(ttlSeconds) ||
				(ttlSeconds as number) <= 0 ||
				(ttlSeconds as number) > DELEGATED_OPERATION_MAX_TTL_SECONDS))
	) {
		return null;
	}
	return {
		storeId: value.storeId,
		reasonCode: value.reasonCode,
		caseReference,
		...(ttlSeconds === undefined ? {} : { ttlSeconds: ttlSeconds as number }),
	};
}

export function deriveDelegatedOperationState(
	summary: DelegatedOperationSessionSummary,
	expectedStoreId: string,
	now: number | Date = Date.now(),
): DelegatedOperationState {
	if (
		!isSafeOpaqueId(expectedStoreId) ||
		!isSafeOpaqueId(summary.target?.storeId)
	) {
		return "invalid";
	}
	if (summary.target.storeId !== expectedStoreId) return "target_mismatch";

	const expiresAt = Date.parse(summary.expiresAt);
	const issuedAt = Date.parse(summary.issuedAt);
	const nowEpoch = now instanceof Date ? now.getTime() : now;
	const lifetimeMs = expiresAt - issuedAt;
	if (
		!Number.isFinite(expiresAt) ||
		!Number.isFinite(issuedAt) ||
		!Number.isFinite(nowEpoch) ||
		lifetimeMs <= 0 ||
		lifetimeMs > DELEGATED_OPERATION_MAX_TTL_SECONDS * 1000 ||
		issuedAt > nowEpoch + DELEGATED_OPERATION_CLOCK_SKEW_SECONDS * 1000
	)
		return "invalid";
	if (summary.status === "revoked") return "revoked";
	if (summary.status === "expired" || expiresAt <= nowEpoch) return "expired";
	if (summary.status !== "active") return "invalid";
	if (expiresAt - nowEpoch <= DELEGATED_OPERATION_EXPIRING_SECONDS * 1000) {
		return "expiring";
	}
	return "active";
}

/**
 * A successful create response and the authoritative current response must
 * describe the same exact operation context before the client may navigate.
 */
export function isSameDelegatedOperationSession(
	started: DelegatedOperationSessionSummary,
	current: DelegatedOperationSessionSummary,
): boolean {
	if (
		started.target.storeId !== current.target.storeId ||
		started.target.sellerAccountId !== current.target.sellerAccountId ||
		started.actor.adminId !== current.actor.adminId ||
		started.reason.code !== current.reason.code ||
		started.caseReference !== current.caseReference ||
		started.issuedAt !== current.issuedAt ||
		started.expiresAt !== current.expiresAt ||
		started.status !== current.status ||
		started.revision !== current.revision ||
		started.capabilities.length !== current.capabilities.length
	) {
		return false;
	}
	const currentCapabilities = new Set(current.capabilities);
	return started.capabilities.every((capability) =>
		currentCapabilities.has(capability),
	);
}

export function canRenderDelegatedWorkspace(
	state: DelegatedOperationState,
): boolean {
	return state === "active" || state === "expiring";
}

export function formatRemainingDuration(
	expiresAt: string,
	now: number | Date = Date.now(),
): string {
	const expiryEpoch = Date.parse(expiresAt);
	const nowEpoch = now instanceof Date ? now.getTime() : now;
	if (!Number.isFinite(expiryEpoch) || !Number.isFinite(nowEpoch))
		return "--:--";
	const remainingSeconds = Math.max(
		0,
		Math.ceil((expiryEpoch - nowEpoch) / 1000),
	);
	const minutes = Math.floor(remainingSeconds / 60);
	const seconds = remainingSeconds % 60;
	return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
