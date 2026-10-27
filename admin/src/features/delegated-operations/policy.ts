/**
 * The closed enumeration the backend can issue. Values outside it are never
 * granted, so listing one here only hides a real affordance behind a flag that
 * is permanently false.
 */
export const KNOWN_DELEGATED_OPERATION_CAPABILITIES = [
	"storefront.read",
	"storefront.write",
	"products.read",
	"products.write",
	"products.publish",
	"posts.read",
	"posts.write",
	"posts.publish",
	"orders.read",
] as const;

export type DelegatedOperationCapability =
	(typeof KNOWN_DELEGATED_OPERATION_CAPABILITIES)[number];

/**
 * These areas can never be reached through an ADM delegated-operation session.
 * The backend remains authoritative; this list is also used by the Admin UI to
 * avoid presenting a dangerous affordance before a request is made.
 */
export const DELEGATED_OPERATION_PROHIBITED_AREAS = [
	"fulfillment",
	"shipment",
	"cancel",
	"refund",
	"payment",
	"finance",
	"identity",
	"certification",
	"account-security",
] as const;

const CAPABILITY_SET = new Set<string>(KNOWN_DELEGATED_OPERATION_CAPABILITIES);

/**
 * ⚠️ 這份依賴表**只服務 `parseDelegatedOperationCapabilities`**（解析一份**已核發**
 * 的工作階段範圍，允許比預期窄的組合）。
 *
 * 代行**授權設定**畫面（backend#2444）一律不得引用它：那一側的權威是 gateway
 * `GET /admin/delegated-operations/capabilities` 逐次下發的 `dependsOn`，兩份表漂移
 * 時的症狀是「畫面允許送出的組合被後端擋掉」或更糟的「畫面擋掉了合法組合」，
 * 兩者都不會有任何東西轉紅。規則與純函式在 `./authorization.ts`，
 * `tests/delegation-authorization-contract.test.ts` 斷言那一側不 import 本常數。
 */
const CAPABILITY_DEPENDENCIES: Partial<
	Record<DelegatedOperationCapability, DelegatedOperationCapability[]>
> = {
	"storefront.write": ["storefront.read"],
	"products.write": ["products.read"],
	"products.publish": ["products.read", "products.write"],
	"posts.write": ["posts.read"],
	"posts.publish": ["posts.read", "posts.write"],
};

export function hasAsciiControlCharacters(value: string): boolean {
	for (const character of value) {
		const code = character.charCodeAt(0);
		if (code <= 31 || code === 127) return true;
	}
	return false;
}

export function isDelegatedOperationCapability(
	value: unknown,
): value is DelegatedOperationCapability {
	return typeof value === "string" && CAPABILITY_SET.has(value);
}

/**
 * Unknown values, duplicates, and entries whose dependencies are absent are
 * dropped one at a time; only an entirely empty result fails closed.
 *
 * Voiding the whole grant would turn one hand-written authority row into a
 * blanket preflight failure: the backend CHECK closes the value domain but
 * does not enforce the dependency closure, so a narrower-than-expected grant
 * is a legitimate thing for this parser to receive.
 */
export function parseDelegatedOperationCapabilities(
	value: unknown,
): DelegatedOperationCapability[] | null {
	if (!Array.isArray(value) || value.length === 0) return null;

	const granted = new Set<DelegatedOperationCapability>();
	const deduplicated: DelegatedOperationCapability[] = [];
	for (const candidate of value) {
		if (!isDelegatedOperationCapability(candidate) || granted.has(candidate)) {
			continue;
		}
		granted.add(candidate);
		deduplicated.push(candidate);
	}

	const parsed = deduplicated.filter((capability) =>
		(CAPABILITY_DEPENDENCIES[capability] ?? []).every((dependency) =>
			granted.has(dependency),
		),
	);
	return parsed.length > 0 ? parsed : null;
}

export function isSafeOpaqueId(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.length > 0 &&
		value.length <= 128 &&
		value === value.trim() &&
		!hasAsciiControlCharacters(value) &&
		!value.includes("/") &&
		!value.includes("\\")
	);
}

export function isSafeReference(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.length > 0 &&
		value.length <= 200 &&
		value === value.trim() &&
		!hasAsciiControlCharacters(value)
	);
}
