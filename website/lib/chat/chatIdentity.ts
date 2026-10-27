/**
 * Resolves "which id on the chat wire is me", extracted so the comparison is
 * unit-testable without mounting the hook or rendering a thread.
 *
 * Chat messages never carry the caller's raw `account_id`. The gateway's
 * `chatWireIdentity` (backend-payuni `resolve_seller.go`) rewrites the wire
 * identity to the caller's CURRENT-stint surrogate before it ever reaches
 * im-service — `store_id` when the active role is seller (accountType 2),
 * `buyer_id` otherwise — and that same resolved value is used both to register
 * the WebSocket (`/ws?userId=`) and as `FromUserId` on every send. So
 * `senderId`/`receiverId` are always surrogates, and the viewer's own id must
 * be resolved the same way or it matches nothing.
 *
 * Two properties this has to get right:
 *
 * 1. **Role branch mirrors the gateway.** The backend switches on
 *    `parseAccountType`, which for a dual-role account falls through to
 *    `loginRole` — the validated `X-Active-Role` header. `lib/api/client.ts`
 *    sends that header from `getStoredActiveRole()`, the same value
 *    `useAuth().activeRole` exposes, so keying off `activeRole` (rather than
 *    the pathname) guarantees the client and gateway agree on the branch.
 * 2. **Ids stay exact strings.** Since backend commit 2269417e3 (2026-07-18,
 *    "never grandfather buyer_id/store_id to account_id") these surrogates are
 *    freshly minted 64-bit TSIDs — 18-19 digits, past `Number.MAX_SAFE_INTEGER`.
 *    `senderId` is compared with `===` against an exact decimal string, so
 *    routing the viewer's id through `Number()` would silently corrupt the low
 *    digits and break the match (or, worse, collide with a nearby TSID).
 *
 * Accounts provisioned BEFORE 2026-07-18 were grandfathered to
 * `buyer_id == account_id` by the 2026-07-15 stint-reshape migration, which is
 * why comparing the raw `accountId` appeared to work for them and this stayed
 * invisible.
 */

export interface ResolveChatViewerIdParams {
	/** Active role is seller — the same branch the gateway takes for
	 *  `accountType`/`loginRole` 2. Read from `useAuth().activeRole`, the
	 *  source of the `X-Active-Role` header. */
	isSeller: boolean;
	/** Current store_id (`GetMyStore` -> `store.id`), already an
	 *  exact string on the wire. */
	storeId: string | number | null | undefined;
	/** Current buyer_id (`/app/buyers/me` -> `MemberProfile.id`), an exact
	 *  string. */
	buyerId: string | number | null | undefined;
}

/**
 * The id comparable against a chat message's `senderId`/`receiverId`.
 *
 * Returns `""` while the underlying profile fetch is still in flight (or when
 * the role has no surrogate yet) — callers treat empty as "not resolved", the
 * same convention the previous `myId` used, so no message is ever attributed
 * to a viewer whose identity hasn't loaded.
 */
export function resolveChatViewerId({
	isSeller,
	storeId,
	buyerId,
}: ResolveChatViewerIdParams): string {
	const raw = isSeller ? storeId : buyerId;
	if (raw == null) return "";
	return String(raw);
}
