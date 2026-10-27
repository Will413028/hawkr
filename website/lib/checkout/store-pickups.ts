/**
 * Store-pickup (超商取貨) repository.
 *
 * Endpoints (per OpenAPI):
 *   GET    /app/store_pickups        — buyer's saved store-pickup records
 *   POST   /app/store_pickups        — create new pickup record
 *   PUT    /app/store_pickups/{id}   — update
 *   DELETE /app/store_pickups/{id}   — delete
 *   POST   /app/store_map_sessions   — get PayUni form params for opening
 *                                       the 7-11 electronic-map flow
 *
 * Wire-shape note: backend `BuyerStorePickupResp` / `AddStorePickupReq` use
 * `storeCode` (not `storeId`). We keep `storeId` on the TS-side because
 * `/app/checkout/preview` + `/confirm` accept `storeId` for the same value
 * — the field is just inconsistently named across endpoints. Translate at
 * the wire boundary.
 */

import { authedFetch } from "../api/client";
import { safeRedirectUrl } from "../navigation/safe-url";

export interface StorePickup {
	id: string;
	receiverName: string;
	receiverPhone: string;
	storeId: string; // backend `storeCode` — kept as `storeId` here to match the
	// checkout API's field name (`/app/checkout/preview` uses `storeId`)
	storeName: string;
	storeAddress?: string;
	isDefault?: boolean;
}

interface StorePickupWire {
	id?: string | number;
	receiverName?: string;
	receiver_name?: string;
	receiverPhone?: string;
	receiver_phone?: string;
	storeCode?: string;
	store_code?: string;
	storeId?: string;
	store_id?: string;
	storeName?: string;
	store_name?: string;
	storeAddress?: string;
	store_address?: string;
	isDefault?: boolean | string;
	is_default?: boolean | string;
}

function normalize(input: StorePickupWire): StorePickup {
	return {
		id: String(input.id ?? ""),
		receiverName: String(input.receiverName ?? input.receiver_name ?? ""),
		receiverPhone: String(input.receiverPhone ?? input.receiver_phone ?? ""),
		storeId: String(
			input.storeCode ??
				input.store_code ??
				input.storeId ??
				input.store_id ??
				"",
		),
		storeName: String(input.storeName ?? input.store_name ?? ""),
		storeAddress: input.storeAddress ?? input.store_address,
		isDefault:
			input.isDefault === true ||
			input.isDefault === "true" ||
			input.is_default === true ||
			input.is_default === "true",
	};
}

export async function fetchStorePickups(): Promise<StorePickup[]> {
	const env = await authedFetch<
		StorePickupWire[] | { records?: StorePickupWire[] }
	>("/app/store_pickups");
	const records = Array.isArray(env.data)
		? env.data
		: (env.data?.records ?? []);
	return records.map(normalize);
}

export type StorePickupInput = Omit<StorePickup, "id">;

function toCreatePayload(body: StorePickupInput) {
	return {
		receiverName: body.receiverName,
		receiverPhone: body.receiverPhone,
		storeCode: body.storeId,
		storeName: body.storeName,
		storeAddress: body.storeAddress ?? "",
		isDefault: body.isDefault ?? false,
	};
}

export async function createStorePickup(body: StorePickupInput): Promise<void> {
	await authedFetch("/app/store_pickups", {
		method: "POST",
		json: toCreatePayload(body),
	});
}

export async function updateStorePickup(
	id: string,
	body: StorePickupInput,
): Promise<void> {
	await authedFetch(`/app/store_pickups/${encodeURIComponent(id)}`, {
		method: "PUT",
		json: toCreatePayload(body),
	});
}

export async function deleteStorePickup(id: string): Promise<void> {
	await authedFetch(`/app/store_pickups/${encodeURIComponent(id)}`, {
		method: "DELETE",
	});
}

/* =============================================================
 *           PayUni 7-11 電子地圖 (store map) flow
 * ============================================================= */

export interface StoreMapSession {
	redirectUrl?: string;
	formAction?: string;
	encryptInfo?: string;
	hashInfo?: string;
	merId?: string;
	version?: string;
	sessionId?: string;
	expiresAt?: number;
}

/**
 * POST /app/store_map_sessions — ask backend to mint a one-shot
 * PayUni 7-11 map session.
 *
 * The web flow sends `appType: "web"` so the gateway routes PayUni's
 * `MapReturnURL` to our Next.js `/store-map/return` page instead of a
 * `hawkrbuyer://` deep link. Until the gateway lands the `"web"`
 * branch the call will fail with HTTP 400 (`appType must be 'buyer' or
 * 'seller'`); we deliberately keep the wire shape ready so the cutover
 * is zero-frontend-change once the backend deploys.
 * `mobileTag: "N"` is the PayUni hint for "not a mobile webview".
 */
export async function createStoreMapSession(
	appType: "web" | "buyer" | "seller" = "web",
): Promise<StoreMapSession> {
	const env = await authedFetch<StoreMapSession>("/app/store_map_sessions", {
		method: "POST",
		json: { appType, mobileTag: "N" },
	});
	return env.data ?? {};
}

export interface StoreMapResult {
	storeId: string;
	storeName: string;
	storeAddress: string;
}

/**
 * Browser-side helper: open the PayUni map in a popup, listen for the
 * `postMessage` from our `/store-map/return` callback page, resolve with
 * the picked store. Rejects if the popup is blocked, the user closes it,
 * or the message origin doesn't match our app.
 *
 * The matching counter-part is `app/store-map/return/route.ts`, which
 * receives PayUni's callback and posts the parsed store back here.
 */
export function openStoreMapPopup(
	redirectUrl: string,
): Promise<StoreMapResult> {
	return new Promise((resolve, reject) => {
		if (typeof window === "undefined") {
			reject(new Error("openStoreMapPopup requires a browser"));
			return;
		}
		// Size the popup as ~80% of the opener's viewport, clamped to a band
		// where PayUni's 7-11 map stays usable: too narrow squashes the store
		// list, too tall on 4K screens leaves the controls floating.
		const vw = window.innerWidth || window.screen.width || 1024;
		const vh = window.innerHeight || window.screen.height || 768;
		const width = Math.round(Math.min(1100, Math.max(480, vw * 0.8)));
		const height = Math.round(Math.min(900, Math.max(640, vh * 0.8)));
		const left = Math.max(0, Math.round((window.screen.width - width) / 2));
		const top = Math.max(0, Math.round((window.screen.height - height) / 2));
		const features = [
			"popup=yes",
			`width=${width}`,
			`height=${height}`,
			`left=${left}`,
			`top=${top}`,
			"scrollbars=yes",
			"resizable=yes",
		].join(",");
		const target = safeRedirectUrl(redirectUrl);
		if (!target) {
			reject(new Error("門市地圖連結格式不正確，請稍後再試"));
			return;
		}
		const popup = window.open(target, "hk-store-map", features);
		if (!popup) {
			reject(new Error("無法開啟門市地圖，請允許彈出視窗"));
			return;
		}

		const expectedOrigin = window.location.origin;
		let settled = false;

		const cleanup = () => {
			settled = true;
			window.removeEventListener("message", onMessage);
			clearInterval(closedTimer);
		};

		const onMessage = (event: MessageEvent) => {
			if (event.origin !== expectedOrigin) return;
			const data = event.data;
			if (!data || typeof data !== "object") return;
			if ((data as { type?: string }).type !== "hk-store-map-result") return;
			const payload = data as {
				storeId?: string;
				storeName?: string;
				storeAddress?: string;
				error?: string;
			};
			if (payload.error) {
				cleanup();
				reject(new Error(payload.error));
				return;
			}
			if (!payload.storeId || !payload.storeName) {
				cleanup();
				reject(new Error("門市資料缺漏"));
				return;
			}
			cleanup();
			try {
				popup.close();
			} catch {
				/* popup may be cross-origin at close-time; ignore */
			}
			resolve({
				storeId: payload.storeId,
				storeName: payload.storeName,
				storeAddress: payload.storeAddress ?? "",
			});
		};

		const closedTimer = window.setInterval(() => {
			if (settled) return;
			if (popup.closed) {
				cleanup();
				reject(new Error("已取消選擇門市"));
			}
		}, 500);

		window.addEventListener("message", onMessage);
	});
}
