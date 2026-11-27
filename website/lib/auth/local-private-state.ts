/**
 * Browser-local private state, bound to the account that owns it
 * (hawkr-website#382).
 *
 * Two buyer features persist to Web Storage with no backend behind them, so
 * neither is scoped by the session the way every API-backed view is:
 *
 *   - save-for-later (`lib/cart/saved.ts`, localStorage)
 *   - the checkout draft (`lib/checkout/draft.ts`, localStorage + sessionStorage)
 *
 * Both used a fixed, account-free key and neither was cleared on any logout
 * path, so a shared device handed the next person who signed in the previous
 * buyer's parked products, and — where the two carts happened to share a SKU —
 * their recipient name, phone and address as a pre-filled checkout card.
 *
 * The binding is one stored owner id rather than an account-suffixed key per
 * feature: a suffixed key would leave one orphaned record per account on the
 * device forever (the leak becomes unreachable, but the data stays), whereas a
 * single owner marker lets a change of owner delete the data outright. It also
 * keeps every store's own key stable, so the cross-tab `storage` contracts both
 * features already rely on keep working untouched.
 *
 * `syncLocalPrivateState` is the only writer of the marker and must be called
 * on EVERY identity transition — not just sign-out. Sign-out cleanup alone
 * cannot cover the device whose browser was simply closed mid-session: nothing
 * runs on the way out, the data is still there on the next launch, and the next
 * person's sign-in is the first moment anything can notice. Comparing the
 * marker at that moment is what closes it.
 *
 * 2026-08-07：清除清單改由 lib/auth/local-private-keys.ts 的 registry 驅動。
 * 原本這裡手寫 clearSavedItems() / clearCheckoutDraft() 兩行呼叫，於是新增的
 * 本地持久層只要沒有人記得回來加一行就會漏——cartUiStore 與賣家中心 5 個
 * store 都是這樣漏掉的。改成走訪 registry 之後，「登記」是新增 key 時唯一
 * 需要做的事，而 tests/local-private-registry.test.ts 會強制它被做到。
 */

import {
	LOCAL_PRIVATE_KEY_SPECS,
	type LocalPrivateKeySpec,
} from "./local-private-keys";
import { runLocalPrivateReset } from "./local-private-resets";

/**
 * Account id whose browser-local private state is currently on this device.
 * Absent = nobody has been recorded yet (a fresh device, or data written before
 * this binding existed — treated as unowned, see below).
 */
export const LOCAL_STATE_OWNER_KEY = "hawkr:local:owner";

function isBrowser(): boolean {
	return typeof window !== "undefined" && typeof localStorage !== "undefined";
}

function readOwner(): string | null {
	if (!isBrowser()) return null;
	return localStorage.getItem(LOCAL_STATE_OWNER_KEY);
}

function writeOwner(ownerId: string | null): void {
	if (!isBrowser()) return;
	if (ownerId === null) localStorage.removeItem(LOCAL_STATE_OWNER_KEY);
	else localStorage.setItem(LOCAL_STATE_OWNER_KEY, String(ownerId));
}

/**
 * Tell the already-mounted readers that the store under `key` is gone.
 *
 * Clearing storage is not enough on its own: `CartClient` reads
 * `getSavedItems()` once on mount and `CheckoutClient` reads
 * `readCheckoutDraft()` once on mount, both BEFORE the account probe can answer
 * who the viewer is. On the cold load where the previous buyer's data is still
 * present, that read has already put it into React state by the time the new
 * identity lands — purging storage underneath it would leave the stale copy on
 * screen. Both components already listen for `storage` (the cross-tab channel)
 * and re-read from it, so re-using that channel drops the stale copy without
 * either component needing to know this module exists.
 */
function notifyCleared(key: string): void {
	if (!isBrowser() || typeof StorageEvent !== "function") return;
	try {
		window.dispatchEvent(
			new StorageEvent("storage", { key, oldValue: null, newValue: null }),
		);
	} catch {
	}
}

function storageFor(area: "local" | "session"): Storage | null {
	try {
		return area === "local" ? window.localStorage : window.sessionStorage;
	} catch {
		return null;
	}
}

/**
 * 清掉一把已登記的 key：兩種 storage 都照宣告清，然後沿用既有的合成 storage
 * 事件通知已掛載的畫面（見 notifyCleared 的說明）。
 */
function clearRegisteredKey(spec: LocalPrivateKeySpec): void {
	try {
		runLocalPrivateReset(spec.key);
	} catch {
	}
	for (const area of spec.in) {
		try {
			storageFor(area)?.removeItem(spec.key);
		} catch {
		}
	}
	notifyCleared(spec.key);
}

/**
 * Reconcile the device's browser-local private state with `ownerId`, the
 * account the viewer has just become (`null` = signed out / guest).
 *
 * Same owner as last recorded → keep everything, so a page refresh or the
 * same-account identity backfill never costs a buyer their parked items or an
 * in-progress checkout draft. Any other transition (sign-out, session expiry,
 * a different account signing in on this device) drops both stores.
 *
 * Data written before this binding existed has no marker. It is left alone for
 * a guest — nothing renders it while signed out — and dropped the moment any
 * account signs in, because there is no evidence it belongs to that account.
 */
export function syncLocalPrivateState(ownerId: string | null): void {
	if (!isBrowser()) return;
	const previousOwner = readOwner();
	if (previousOwner === ownerId) return;
	writeOwner(ownerId);
	for (const spec of LOCAL_PRIVATE_KEY_SPECS) {
		if (
			spec.clearOn === "leaving-a-signed-in-owner" &&
			previousOwner === null
		) {
			continue;
		}
		clearRegisteredKey(spec);
	}
}
