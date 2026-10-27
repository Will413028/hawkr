import {
	ACTIVE_ROLE_STORAGE_KEY,
	canUsePerspective,
	parseRole,
	type RoleSlot,
} from "./active-role";

export function reduceStorageEvent(
	event: Pick<StorageEvent, "key" | "newValue">,
	account: { isSeller: boolean } | null,
): RoleSlot | null {
	if (event.key !== ACTIVE_ROLE_STORAGE_KEY || account == null) return null;
	const next = parseRole(event.newValue);
	return next && canUsePerspective(account, next) ? next : null;
}
