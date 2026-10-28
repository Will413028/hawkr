import {
	DELEGATED_AUDIT_RISK_LEVELS,
	type DelegatedAuditRiskLevel,
	isOneOf,
	isRecord,
	parseCountString,
	parseNullableText,
	parseNullableTimestamp,
	parseText,
	parseTimestamp,
} from "./governance";
import { isSafeOpaqueId } from "./policy";

/**
 * 代行**授權列**的 DTO、parser 與能力連動規則（backend#2444，七支
 * `/admin/delegated-operations/…` 治理端點）。
 *
 * 與 `governance.ts` 分開的理由：那一份描述「某一次代行做過什麼」，這一份描述
 * 「這家店可不可以被代行、範圍多大」——授權的開通與撤回發生在任何工作階段之前。
 * 解析原語共用 `governance.ts`，不另抄一份（兩套「什麼叫合法時間」必然漂移）。
 *
 * ⚠️ **本檔不得引入任何本地的能力依賴表。** 依賴閉包在 DB 沒有落點，唯一權威是
 * gateway `internal/authz` 經 `GET …/capabilities` 下發的 `dependsOn`。
 * `policy.ts` 的 `CAPABILITY_DEPENDENCIES` 是 preflight 那條路徑的私有副本
 * （它解析的是**已核發**的工作階段範圍，允許比預期窄的組合），本檔一律不引用它：
 * 引用等於讓「授權設定畫面允許送出什麼」由一份沒有人在維護的鏡像決定，而漂移的
 * 症狀是畫面擋掉合法組合、或放行後被後端以 9287 拒絕——兩者都不會有東西轉紅。
 */

/** `chk_sda_source` 的三值（gateway `delegationAuthorizationSources`）。 */
export const DELEGATION_AUTHORIZATION_SOURCES = [
	"contract",
	"ticket",
	"platform_policy",
] as const;
export type DelegationAuthorizationSource =
	(typeof DELEGATION_AUTHORIZATION_SOURCES)[number];

/** `GrantDelegationAuthorization` 實際走到的 CAS 分支，由伺服端回報。 */
export const DELEGATION_AUTHORIZATION_OUTCOMES = [
	"created",
	"reinstated",
] as const;
export type DelegationAuthorizationOutcome =
	(typeof DELEGATION_AUTHORIZATION_OUTCOMES)[number];

/** gateway `delegatedPaging` 的上界（越界回 9260，不靜默夾值）。 */
export const DELEGATION_AUTHORIZATION_MAX_PAGE_SIZE = 50;

/** `chk_sda_source_reference`：4–200 字且至少含一個數字。 */
export const DELEGATION_SOURCE_REFERENCE_MIN_LENGTH = 4;
export const DELEGATION_SOURCE_REFERENCE_MAX_LENGTH = 200;
/** `operationReason` / `revokeReason` 同為 1–200 字。 */
export const DELEGATION_REASON_MAX_LENGTH = 200;

export interface DelegationCapabilityDescriptor {
	value: string;
	risk: DelegatedAuditRiskLevel;
	/** **直接**依賴。遞移閉包由本檔的純函式現算，不由後端展開。 */
	dependsOn: string[];
}

export interface DelegationCapabilityCatalog {
	capabilities: DelegationCapabilityDescriptor[];
}

export interface DelegationAuthorization {
	id: string;
	storeId: string;
	source: DelegationAuthorizationSource;
	sourceReference: string;
	grantedCapabilities: string[];
	effectiveFrom: string;
	/** null ＝ 無期限。 */
	effectiveUntil: string | null;
	approvedByAdminId: string;
	approvedByAdminName: string;
	approvedAt: string;
	revokedAt: string | null;
	revokedByAdminId: string | null;
	revokedByAdminName: string | null;
	revokeReason: string | null;
	/**
	 * 伺服端以 now() 逐列評估的結果。**前端不得從三個時間欄自行推導**——客戶端重算
	 * 一次就會有兩套答案，而其中一套會在邊界時刻說謊。
	 */
	effective: boolean;
	createdAt: string;
	updatedAt: string;
}

/**
 * 「授權變成什麼」——開通與調整範圍送出的同一組欄位（gateway 也以一支
 * `validateDelegationAuthorizationScope` 共用）。全量語意：未送的欄位不沿用舊值。
 */
export interface DelegationAuthorizationScope {
	source: DelegationAuthorizationSource;
	sourceReference: string;
	grantedCapabilities: string[];
	/** null ＝ 無期限。復權時舊列的期限一律被本欄取代（含取代成無期限）。 */
	effectiveUntil: string | null;
	operationReason: string;
}

export interface DelegationAuthorizationLookup {
	exists: boolean;
	authorization: DelegationAuthorization | null;
	/**
	 * 該店此刻進行中的代行工作階段數。撤回二次確認的**唯一**資料來源：另打工作階段
	 * 列表去數，看到的不會是撤回所看到的那個瞬間。與 `exists` 正交。
	 */
	activeSessionCount: number;
}

export interface DelegationAuthorizationGrant {
	authorization: DelegationAuthorization;
	outcome: DelegationAuthorizationOutcome;
}

export interface DelegationAuthorizationUpdate {
	authorization: DelegationAuthorization;
}

export interface DelegationAuthorizationRevocation {
	authorization: DelegationAuthorization;
	/** true ＝ 抵達時已是撤回狀態：沒有欄位被改、沒有稽核列、沒有工作階段被中止。 */
	alreadyRevoked: boolean;
	/** 恆為陣列。以本欄回報「已同時中止 N 個」，不是撤回前查到的 activeSessionCount。 */
	revokedSessionIds: string[];
}

export interface DelegationAuthorizationHistoryEntry {
	id: string;
	authorizationId: string;
	storeId: string;
	action: string;
	/** 後端逐列下發。前端不建 action → 中文對照表（那會是 CodeMeta 的第二份副本）。 */
	actionLabel: string;
	actorAdminId: string;
	/** 寫入當下的快照，不是現況 join。 */
	actorDisplayName: string;
	operationReason: string;
	source: string | null;
	sourceReference: string | null;
	capabilitiesBefore: string[];
	capabilitiesAfter: string[];
	effectiveUntilBefore: string | null;
	effectiveUntilAfter: string | null;
	createdAt: string;
}

export interface DelegationAuthorizationHistoryPage {
	records: DelegationAuthorizationHistoryEntry[];
	total: number;
	page: number;
	pageSize: number;
}

// ==================== parsers ====================

/**
 * 能力機器名。**刻意不比對 `policy.ts` 的封閉九值**：那份清單是為 preflight 寫的，
 * 拿它當授權面的白名單，等於後端新增一個能力時 ADM 這一側直接把整份目錄判成契約錯，
 * 而目錄正是要用來告訴前端有哪些能力的東西。
 */
function parseCapabilityValue(value: unknown): string | null {
	const parsed = parseText(value, 64);
	return parsed === null || parsed.length === 0 ? null : parsed;
}

function parseCapabilityList(value: unknown): string[] | null {
	if (!Array.isArray(value)) return null;
	const parsed: string[] = [];
	const seen = new Set<string>();
	for (const raw of value) {
		const capability = parseCapabilityValue(raw);
		// 重複值會讓「勾了幾項」與「送出幾項」對不上，是契約錯不是可容忍的雜訊。
		if (capability === null || seen.has(capability)) return null;
		seen.add(capability);
		parsed.push(capability);
	}
	return parsed;
}

export function parseDelegationCapabilityCatalog(
	value: unknown,
): DelegationCapabilityCatalog | null {
	if (!isRecord(value) || !Array.isArray(value.capabilities)) return null;
	const capabilities: DelegationCapabilityDescriptor[] = [];
	const seen = new Set<string>();
	for (const raw of value.capabilities) {
		if (!isRecord(raw)) return null;
		const descriptorValue = parseCapabilityValue(raw.value);
		const dependsOn = parseCapabilityList(raw.dependsOn);
		if (
			descriptorValue === null ||
			seen.has(descriptorValue) ||
			dependsOn === null ||
			!isOneOf(raw.risk, DELEGATED_AUDIT_RISK_LEVELS)
		) {
			return null;
		}
		seen.add(descriptorValue);
		capabilities.push({ value: descriptorValue, risk: raw.risk, dependsOn });
	}
	// 空目錄不是「沒有能力可選」，是「這一版後端沒回東西」。放行會畫出一個永遠
	// 送不出去的表單（至少要勾一項），而畫面上看起來像是設定被清空了。
	if (capabilities.length === 0) return null;
	// 依賴指向目錄外的值 ⇒ 連動規則算不完整，勾選介面會給出一個後端必然以 9287
	// 拒絕的組合。fail-closed 比畫一個會被拒絕的表單誠實。
	for (const descriptor of capabilities) {
		for (const dependency of descriptor.dependsOn) {
			if (!seen.has(dependency)) return null;
		}
	}
	return { capabilities };
}

export function parseDelegationAuthorization(
	value: unknown,
): DelegationAuthorization | null {
	if (!isRecord(value)) return null;
	const sourceReference = parseText(
		value.sourceReference,
		DELEGATION_SOURCE_REFERENCE_MAX_LENGTH,
	);
	const grantedCapabilities = parseCapabilityList(value.grantedCapabilities);
	const effectiveFrom = parseTimestamp(value.effectiveFrom);
	const effectiveUntil = parseNullableTimestamp(value.effectiveUntil);
	const approvedByAdminName = parseText(value.approvedByAdminName, 160);
	const approvedAt = parseTimestamp(value.approvedAt);
	const revokedAt = parseNullableTimestamp(value.revokedAt);
	const revokedByAdminId = parseNullableText(value.revokedByAdminId, 64);
	const revokedByAdminName = parseNullableText(value.revokedByAdminName, 160);
	const revokeReason = parseNullableText(
		value.revokeReason,
		DELEGATION_REASON_MAX_LENGTH,
	);
	const createdAt = parseTimestamp(value.createdAt);
	const updatedAt = parseTimestamp(value.updatedAt);
	if (
		!isSafeOpaqueId(value.id) ||
		!isSafeOpaqueId(value.storeId) ||
		!isOneOf(value.source, DELEGATION_AUTHORIZATION_SOURCES) ||
		sourceReference === null ||
		sourceReference.length === 0 ||
		grantedCapabilities === null ||
		// 授權列的 cardinality 下界是 1：空集合＝一列不授權任何東西的授權，
		// 畫面會顯示「已開通」卻沒有任何範圍。
		grantedCapabilities.length === 0 ||
		effectiveFrom === null ||
		!effectiveUntil.ok ||
		!isSafeOpaqueId(value.approvedByAdminId) ||
		approvedByAdminName === null ||
		approvedAt === null ||
		!revokedAt.ok ||
		!revokedByAdminId.ok ||
		!revokedByAdminName.ok ||
		!revokeReason.ok ||
		typeof value.effective !== "boolean" ||
		createdAt === null ||
		updatedAt === null
	) {
		return null;
	}
	return {
		id: value.id,
		storeId: value.storeId,
		source: value.source,
		sourceReference,
		grantedCapabilities,
		effectiveFrom,
		effectiveUntil: effectiveUntil.value,
		approvedByAdminId: value.approvedByAdminId,
		approvedByAdminName,
		approvedAt,
		revokedAt: revokedAt.value,
		revokedByAdminId: revokedByAdminId.value,
		revokedByAdminName: revokedByAdminName.value,
		revokeReason: revokeReason.value,
		effective: value.effective,
		createdAt,
		updatedAt,
	};
}

export function parseDelegationAuthorizationLookup(
	value: unknown,
): DelegationAuthorizationLookup | null {
	if (!isRecord(value) || typeof value.exists !== "boolean") return null;
	const activeSessionCount = parseCountString(value.activeSessionCount);
	if (activeSessionCount === null) return null;
	if (!value.exists) {
		// exists=false 是正常狀態（尚未開通），不是錯誤；但同時帶著一列授權就是一份
		// 自相矛盾的回應，照著畫會出現「尚未開通」配上一份範圍。
		if (value.authorization !== null && value.authorization !== undefined) {
			return null;
		}
		return { exists: false, authorization: null, activeSessionCount };
	}
	const authorization = parseDelegationAuthorization(value.authorization);
	if (!authorization) return null;
	return { exists: true, authorization, activeSessionCount };
}

export function parseDelegationAuthorizationGrant(
	value: unknown,
): DelegationAuthorizationGrant | null {
	if (!isRecord(value)) return null;
	const authorization = parseDelegationAuthorization(value.authorization);
	if (
		!authorization ||
		!isOneOf(value.outcome, DELEGATION_AUTHORIZATION_OUTCOMES)
	) {
		return null;
	}
	return { authorization, outcome: value.outcome };
}

export function parseDelegationAuthorizationUpdate(
	value: unknown,
): DelegationAuthorizationUpdate | null {
	if (!isRecord(value)) return null;
	const authorization = parseDelegationAuthorization(value.authorization);
	return authorization ? { authorization } : null;
}

export function parseDelegationAuthorizationRevocation(
	value: unknown,
): DelegationAuthorizationRevocation | null {
	if (!isRecord(value) || typeof value.alreadyRevoked !== "boolean")
		return null;
	const authorization = parseDelegationAuthorization(value.authorization);
	if (!authorization) return null;
	const ids =
		value.revokedSessionIds === null || value.revokedSessionIds === undefined
			? []
			: value.revokedSessionIds;
	if (!Array.isArray(ids)) return null;
	const revokedSessionIds: string[] = [];
	for (const raw of ids) {
		if (!isSafeOpaqueId(raw)) return null;
		revokedSessionIds.push(raw);
	}
	// 宣稱「已撤回過、什麼都沒動」卻同時回報中止了工作階段＝一份假的撤回證明。
	if (value.alreadyRevoked && revokedSessionIds.length > 0) return null;
	return {
		authorization,
		alreadyRevoked: value.alreadyRevoked,
		revokedSessionIds,
	};
}

export function parseDelegationAuthorizationHistoryEntry(
	value: unknown,
): DelegationAuthorizationHistoryEntry | null {
	if (!isRecord(value)) return null;
	const action = parseText(value.action, 60);
	const actionLabel = parseText(value.actionLabel, 60);
	const actorDisplayName = parseText(value.actorDisplayName, 160);
	const operationReason = parseText(
		value.operationReason,
		DELEGATION_REASON_MAX_LENGTH,
	);
	const source = parseNullableText(value.source, 32);
	const sourceReference = parseNullableText(
		value.sourceReference,
		DELEGATION_SOURCE_REFERENCE_MAX_LENGTH,
	);
	const capabilitiesBefore = parseCapabilityList(
		value.capabilitiesBefore ?? [],
	);
	const capabilitiesAfter = parseCapabilityList(value.capabilitiesAfter ?? []);
	const effectiveUntilBefore = parseNullableTimestamp(
		value.effectiveUntilBefore,
	);
	const effectiveUntilAfter = parseNullableTimestamp(value.effectiveUntilAfter);
	const createdAt = parseTimestamp(value.createdAt);
	if (
		!isSafeOpaqueId(value.id) ||
		!isSafeOpaqueId(value.authorizationId) ||
		!isSafeOpaqueId(value.storeId) ||
		action === null ||
		action.length === 0 ||
		actionLabel === null ||
		!isSafeOpaqueId(value.actorAdminId) ||
		actorDisplayName === null ||
		operationReason === null ||
		!source.ok ||
		!sourceReference.ok ||
		capabilitiesBefore === null ||
		capabilitiesAfter === null ||
		!effectiveUntilBefore.ok ||
		!effectiveUntilAfter.ok ||
		createdAt === null
	) {
		return null;
	}
	return {
		id: value.id,
		authorizationId: value.authorizationId,
		storeId: value.storeId,
		action,
		actionLabel,
		actorAdminId: value.actorAdminId,
		actorDisplayName,
		operationReason,
		source: source.value,
		sourceReference: sourceReference.value,
		capabilitiesBefore,
		capabilitiesAfter,
		effectiveUntilBefore: effectiveUntilBefore.value,
		effectiveUntilAfter: effectiveUntilAfter.value,
		createdAt,
	};
}

/**
 * ⚠️ 單列不合契約時整頁失敗，不是跳過那一列（同 `parseDelegatedAuditLogPage`）：
 * 授權變更歷程是「這家店曾經被撤回過」的唯一痕跡（每店終生一列，復權會就地覆寫），
 * 靜默丟掉一列之後剩下的每一列看起來都正常。
 */
export function parseDelegationAuthorizationHistoryPage(
	value: unknown,
): DelegationAuthorizationHistoryPage | null {
	if (!isRecord(value) || !Array.isArray(value.records)) return null;
	const total = parseCountString(value.total);
	const page = parseCountString(value.page);
	const pageSize = parseCountString(value.pageSize);
	if (
		total === null ||
		page === null ||
		pageSize === null ||
		page < 1 ||
		pageSize < 1 ||
		pageSize > DELEGATION_AUTHORIZATION_MAX_PAGE_SIZE
	) {
		return null;
	}
	const records: DelegationAuthorizationHistoryEntry[] = [];
	for (const raw of value.records) {
		const entry = parseDelegationAuthorizationHistoryEntry(raw);
		if (!entry) return null;
		records.push(entry);
	}
	return { records, total, page, pageSize };
}

// ==================== 能力連動（目錄驅動，無本地依賴表） ====================

function directDependencies(
	catalog: DelegationCapabilityCatalog,
): Map<string, string[]> {
	return new Map(
		catalog.capabilities.map((descriptor) => [
			descriptor.value,
			descriptor.dependsOn,
		]),
	);
}

/** 目錄順序是唯一的排序依據：任何一組能力在畫面上與送出時都以同一順序出現。 */
function orderByCatalog(
	catalog: DelegationCapabilityCatalog,
	values: Iterable<string>,
): string[] {
	const wanted = new Set(values);
	const ordered = catalog.capabilities
		.map((descriptor) => descriptor.value)
		.filter((value) => wanted.has(value));
	// 目錄外的值不丟掉（丟掉＝在送出時靜默縮小授權範圍），排在後面且保持穩定。
	const extras = [...wanted].filter((value) => !ordered.includes(value)).sort();
	return [...ordered, ...extras];
}

/**
 * `value` 的**遞移**依賴（不含自己）。目錄若含環，`visited` 讓它停在有限步，
 * 而不是在勾選的當下把分頁凍住。
 */
export function delegationCapabilityDependencyClosure(
	catalog: DelegationCapabilityCatalog,
	value: string,
): string[] {
	const deps = directDependencies(catalog);
	const visited = new Set<string>();
	const stack = [...(deps.get(value) ?? [])];
	while (stack.length > 0) {
		const current = stack.pop() as string;
		if (current === value || visited.has(current)) continue;
		visited.add(current);
		stack.push(...(deps.get(current) ?? []));
	}
	return orderByCatalog(catalog, visited);
}

/** 勾選一項 ⇒ 它的遞移依賴一併勾上。 */
export function selectDelegationCapability(
	catalog: DelegationCapabilityCatalog,
	selected: readonly string[],
	value: string,
): string[] {
	return orderByCatalog(catalog, [
		...selected,
		value,
		...delegationCapabilityDependencyClosure(catalog, value),
	]);
}

/**
 * 取消一項 ⇒ 所有（遞移）依賴它的項一併取消。
 *
 * 只移除自己會留下一組閉包不完整的選擇，而它要到送出被 9287 拒絕才第一次現形。
 */
export function deselectDelegationCapability(
	catalog: DelegationCapabilityCatalog,
	selected: readonly string[],
	value: string,
): string[] {
	const remaining = new Set(selected);
	remaining.delete(value);
	for (const candidate of [...remaining]) {
		if (
			delegationCapabilityDependencyClosure(catalog, candidate).includes(value)
		) {
			remaining.delete(candidate);
		}
	}
	return orderByCatalog(catalog, remaining);
}

/**
 * 每個已勾選的項是被哪些**其他已勾選的項**連帶要求的。
 *
 * 非空 ⇒ 該項是連帶勾選，勾選框停用並說明「要取消它，請先取消『X』」。直接讓人
 * 取消它，畫面會在下一刻自己把它勾回來（`selectDelegationCapability` 的閉包），
 * 看起來像壞掉。
 */
export function delegationCapabilityRequiredBy(
	catalog: DelegationCapabilityCatalog,
	selected: readonly string[],
): Map<string, string[]> {
	const chosen = orderByCatalog(catalog, selected);
	const required = new Map<string, string[]>();
	for (const candidate of chosen) {
		const dependents = chosen.filter(
			(other) =>
				other !== candidate &&
				delegationCapabilityDependencyClosure(catalog, other).includes(
					candidate,
				),
		);
		if (dependents.length > 0) required.set(candidate, dependents);
	}
	return required;
}

/**
 * 閉包缺口。送出前的本地校驗：不齊就擋下，不要換一次 9287 往返。
 * 後端仍是權威（同一份規則的最後一道），這裡擋的是可預見的失敗。
 */
export function missingDelegationCapabilityDependencies(
	catalog: DelegationCapabilityCatalog,
	selected: readonly string[],
): string[] {
	const chosen = new Set(selected);
	const missing = new Set<string>();
	for (const capability of chosen) {
		for (const dependency of delegationCapabilityDependencyClosure(
			catalog,
			capability,
		)) {
			if (!chosen.has(dependency)) missing.add(dependency);
		}
	}
	return orderByCatalog(catalog, missing);
}

/**
 * 已選項裡目錄認不得的值。來源是版本落差（既有授權列帶著本版目錄沒有的能力）。
 *
 * 不靜默丟掉：丟掉會讓「調整授權範圍」在送出時把那些能力一併撤掉，而畫面從未
 * 顯示過它們，操作者不會知道自己縮小了範圍。改為擋下並明說。
 */
export function unknownDelegationCapabilities(
	catalog: DelegationCapabilityCatalog,
	selected: readonly string[],
): string[] {
	const known = new Set(catalog.capabilities.map((d) => d.value));
	return [...new Set(selected)].filter((value) => !known.has(value)).sort();
}

// ==================== 表單值轉換 ====================

/**
 * `<input type="datetime-local">` 給的是本地時間、無時區；gateway 只收
 * `YYYY-MM-DDTHH:mm:ssZ`（秒精度，帶毫秒會被 `delegationEffectiveUntilRE` 擋掉）。
 * 空字串 ＝ 無期限 ⇒ `null`（送 null 而不是省略欄位：復權時舊值一律被本欄取代）。
 */
export function toDelegationEffectiveUntil(local: string): string | null {
	if (!local.trim()) return null;
	const parsed = new Date(local);
	if (Number.isNaN(parsed.getTime())) return null;
	return parsed.toISOString().replace(/\.\d{3}Z$/u, "Z");
}

/** 反向：把授權列的 UTC 值填回 `datetime-local`（本地時區，分鐘精度）。 */
export function toDelegationLocalDateTime(iso: string | null): string {
	if (!iso) return "";
	const parsed = new Date(iso);
	if (Number.isNaN(parsed.getTime())) return "";
	const offset = parsed.getTime() - parsed.getTimezoneOffset() * 60_000;
	return new Date(offset).toISOString().slice(0, 16);
}

/** `chk_sda_source_reference`：4–200 字且至少含一個數字。 */
export function isDelegationSourceReference(value: string): boolean {
	const trimmed = value.trim();
	const length = [...trimmed].length;
	return (
		length >= DELEGATION_SOURCE_REFERENCE_MIN_LENGTH &&
		length <= DELEGATION_SOURCE_REFERENCE_MAX_LENGTH &&
		/\d/u.test(trimmed)
	);
}

export function isDelegationOperationReason(value: string): boolean {
	const length = [...value.trim()].length;
	return length >= 1 && length <= DELEGATION_REASON_MAX_LENGTH;
}

// ==================== 錯誤分派（業務碼，不看 HTTP status） ====================

/**
 * 業務碼 → 下一步。**一律以碼分派，不看 HTTP status**：9286（找不到授權設定）就是
 * 404，而 404 同時也是「這個環境沒有這支端點」的形狀——拿 status 分派會把「這家店
 * 還沒開通」講成「代行營運未啟用」，而後者的下一步是找維運，前者是按下開通。
 *
 * BFF 的 `safeUpstreamError` 不透傳上游訊息（可能夾帶憑證或內部拓撲），所以文案在
 * 這裡。碼本身仍一併顯示，客服／工單據它追。
 */
export const DELEGATION_AUTHORIZATION_ERROR_MESSAGES: Record<number, string> = {
	// 401 是 envelope 的 `code`，不是 HTTP status —— 兩個來源都送得出它：
	//   ① BFF 本地攔截（server/governance-route.ts 的 unauthorized()，`{code:401}`）
	//   ② gateway 的 JWT 中介層（middleware/auth.go 的 `response.Err(…, 401, …)`），
	//      經 `safeUpstreamError` 保留數值碼後過線。
	// 漏了它 ⇒ ① 顯示英文 "Unauthorized"、② 顯示「代行營運服務暫時無法使用」，而兩者
	// 的正解都是回去重新登入 —— 後者尤其糟：它把「該重新登入」講成「該等一等」。
	// 與 9261 是兩個不同的碼，不可互相取代（9261 是 requireAdmin 過了但取不到
	// accountID 的窄路徑，gateway admin_delegation.go 的 delegationAdminID）。
	401: "管理員登入已失效，請重新登入後再試。",
	9260: "送出的內容不符合授權設定的格式要求，請檢查後重試。",
	9261: "管理員登入狀態已失效，請重新登入後再試。",
	// 「或已到期」不是贅字：治理端只在 revoked_at 有值時送 9264，但 preflight／
	// 建立工作階段那條路徑對「有效期間已過」也送同一個碼（delegation 模組的
	// `authorizationVerdict`），而兩者的下一步一樣是重新開通。少了它，一個因到期
	// 而失效的授權會被說成「被撤回」，管理員會去查一件從未發生的治理動作。
	9264: "這家店的代行授權已撤回或已到期，請改用「重新開通」。",
	// 9277 與 9280 的下一步相反，而 BFF 的 safeUpstreamError 對兩者送的 message
	// 一字不差（它刻意不透傳上游訊息）⇒ 不在這裡按碼分派的話，畫面對兩者都說
	// 「服務暫時無法使用，請稍後再試」。9277 的正解是「這個 ID 不對，回列表重挑」，
	// 而「稍後再試」會讓人去找維運等一個永遠不會好的服務——過期的工作階段 ID
	// 再等一萬年也還是不存在。
	9277: "找不到這個代行工作階段：它可能已經結束或被撤銷，也可能 ID 有誤。請回代行紀錄列表重新選取。",
	9280: "代行營運服務暫時無法使用，請稍後再試。",
	9285: "這家店已有有效的代行授權，請改用「調整授權範圍」。",
	// 收下這個碼的有兩個畫面（授權面板與代行入口），措辭因此不綁任一邊的動作：
	// 「請先開通後再調整」在入口那邊會叫人去調整一件他沒有要調整的東西。
	9286: "這家店尚未開通代行授權，請先於「代行授權」分頁開通。",
	9287: "所選能力的相依關係不完整：選擇編輯或發布時必須同時包含對應的檢視權限。",
	9288: "你的管理員帳號不在代行營運的允許名單內。",
	// #2744 一碼多義拆分。三者的共同點是「操作者的下一步」各不相同 —— 這正是分碼的判準。
	// 9289 不寫「請稍後再試」是刻意的：它與 9280 的差別就在重試永遠不會好。
	9289: "代行營運服務的設定有誤，重試不會改善。請聯絡技術人員並附上這個畫面的時間。",
	9290: "這個資源已經是你要求的狀態，不需要再改一次。請重新整理確認。",
	9291: "這個環境尚未啟用代行營運功能。這與店家和你的權限無關，請洽維運確認設定。",
};

export function delegationAuthorizationErrorMessage(
	code: number | string | undefined,
	fallback: string,
): string {
	if (
		typeof code === "number" &&
		DELEGATION_AUTHORIZATION_ERROR_MESSAGES[code]
	) {
		return DELEGATION_AUTHORIZATION_ERROR_MESSAGES[code];
	}
	return fallback;
}
