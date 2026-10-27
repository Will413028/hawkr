import { Code, ConnectError } from "@connectrpc/connect";
import { classifyError, factsFromError } from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	type Store,
	type StoreOpeningUnavailability,
	StoreOpeningUnavailableReason,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { ApplyStoreInput, StoreUpdate } from "./repo";

/**
 * 帳號不能開店的原因（伺服器的答案，client 只用來分支）：`GetMyStoreOpening` 的 unavailabilities 的 reason，
 * 以及它失效時 ApplyStore 回的 ErrorReason（STORE_OPENING_TERMINATED／STORE_OPENING_SUSPENDED）。顯示的文字是伺服器給的
 * `message`（與 ApplyStore 被擋下時錯誤的 LocalizedMessage 同一份），不在 client 對照。
 */
export type StoreOpeningRefusal =
	| "terminated"
	| "suspended"
	| "accountDeleting"
	| "storeExists";

/** 開店被擋下時給賣家看的：`refusal` 只用來決定要不要附措施與申覆的連結，`message` 是伺服器的句子。 */
export interface StoreOpeningNotice {
	refusal: StoreOpeningRefusal | null;
	message: string | undefined;
}

/** 終止與停權導向措施與申覆；帳號刪除沒有申覆可走；不認得的原因（null）不給連結。 */
export function storeOpeningRefusalHasAppealLink(
	refusal: StoreOpeningRefusal | null,
): boolean {
	return refusal === "terminated" || refusal === "suspended";
}

function refusalOf(
	reason: StoreOpeningUnavailableReason,
): StoreOpeningRefusal | null {
	switch (reason) {
		case StoreOpeningUnavailableReason.TERMINATED:
			return "terminated";
		case StoreOpeningUnavailableReason.SUSPENDED:
			return "suspended";
		case StoreOpeningUnavailableReason.ACCOUNT_DELETING:
			return "accountDeleting";
		case StoreOpeningUnavailableReason.STORE_EXISTS:
			return "storeExists";
		default:
			return null;
	}
}

/**
 * 開店入口依 `GetMyStoreOpening` 的答案決定要不要擋送出：`can_apply_store` 為 false 一律擋；顯示的是伺服器已依優先序排好的
 * 第一筆（不重排）：`message` 是它的句子，`refusal` 是它的 reason（UNSPECIFIED 與不認得的值是 null，不給申覆連結，但有句子仍顯示）。
 * 只在帳號沒有目前店家時呼叫（有店家走更新流程，不讀開店可用性）。
 */
export function storeOpeningGate(answer: {
	canApplyStore: boolean;
	unavailabilities: readonly StoreOpeningUnavailability[];
}): { blocked: boolean } & StoreOpeningNotice {
	if (answer.canApplyStore) {
		return { blocked: false, refusal: null, message: undefined };
	}
	const first = answer.unavailabilities[0];
	return {
		blocked: true,
		refusal: first ? refusalOf(first.reason) : null,
		message: first?.message?.message || undefined,
	};
}

/**
 * ApplyStore 的 failed_precondition 加 STORE_OPENING_* reason：`refusal` 決定附不附連結，`message` 取錯誤的 LocalizedMessage；
 * 其他錯誤回 null。
 */
export function storeOpeningRefusal(error: unknown): StoreOpeningNotice | null {
	const e = ConnectError.from(error);
	if (e.code !== Code.FailedPrecondition) return null;
	const facts = factsFromError(e);
	const d = classifyError(facts);
	if (d.action !== "reason") return null;
	const refusal = (() => {
		if (d.reason === ErrorReason.STORE_OPENING_TERMINATED) return "terminated";
		if (d.reason === ErrorReason.STORE_OPENING_SUSPENDED) return "suspended";
		return null;
	})();
	if (refusal === null) return null;
	return { refusal, message: facts.localizedMessage || undefined };
}

export interface PersistSellerOnboardingDeps {
	applyStore: (input: ApplyStoreInput) => Promise<Store>;
	/** 讀目前的店家；帳號還沒有店家時 throw not_found。 */
	fetchStore: () => Promise<{ store?: Store }>;
	updateStore: (update: StoreUpdate) => Promise<{ store?: Store }>;
}

export type SellerOnboardingPersistenceResult = "created" | "updated";

export interface FinalizeSellerOnboardingDeps
	extends PersistSellerOnboardingDeps {
	/** 以最新的 etag 標記新手引導完成。 */
	complete: (etag: string) => Promise<unknown>;
	onCompleted: (result: SellerOnboardingPersistenceResult) => void;
}

function isCode(error: unknown, code: Code): boolean {
	return error != null && ConnectError.from(error).code === code;
}

async function fetchExistingStore(
	fetchStore: PersistSellerOnboardingDeps["fetchStore"],
): Promise<Store | null> {
	try {
		return (await fetchStore()).store ?? null;
	} catch (e) {
		// 只有確認沒有店家才是 null；暫時失敗不能當成「沒有店家」。
		if (isCode(e, Code.NotFound)) return null;
		throw e;
	}
}

/** 開店表單對既有店家的修改：只列表單有填的欄位，沒填的不動。 */
function onboardingUpdate(
	existing: Store,
	input: ApplyStoreInput,
): StoreUpdate {
	return {
		etag: existing.etag,
		storeName: input.storeName,
		...(input.description === undefined
			? {}
			: { description: input.description }),
		...(input.contactPhone === undefined
			? {}
			: { contactPhone: input.contactPhone }),
		...(input.address === undefined ? {} : { address: input.address }),
	};
}

async function updateExisting(
	existing: Store,
	input: ApplyStoreInput,
	deps: PersistSellerOnboardingDeps,
): Promise<Store> {
	const res = await deps.updateStore(onboardingUpdate(existing, input));
	return res.store ?? existing;
}

/**
 * Persist the store form for both first-time onboarding and an account whose
 * store was created earlier (by registration or the App) but never completed
 * onboarding. 回傳最後的店家版本，標記完成要用它的 etag。
 *
 * 讀到沒有店家之後 ApplyStore 回 already_exists，表示店家在這之間被別處建立：不拿新讀到的 etag
 * 覆寫，錯誤原樣丟給呼叫端走衝突流程（保留輸入、顯示最新版本）。
 */
export async function persistSellerOnboarding(
	input: ApplyStoreInput,
	deps: PersistSellerOnboardingDeps,
): Promise<{ result: SellerOnboardingPersistenceResult; store: Store }> {
	const existing = await fetchExistingStore(deps.fetchStore);
	if (existing) {
		return {
			result: "updated",
			store: await updateExisting(existing, input, deps),
		};
	}
	return { result: "created", store: await deps.applyStore(input) };
}

/**
 * Persist the store, mark onboarding complete, then notify the caller.
 * `onCompleted` is deliberately last so analytics never records completion for
 * a partial or rejected onboarding transaction.
 */
export async function finalizeSellerOnboarding(
	input: ApplyStoreInput,
	deps: FinalizeSellerOnboardingDeps,
): Promise<SellerOnboardingPersistenceResult> {
	const { result, store } = await persistSellerOnboarding(input, deps);
	await deps.complete(store.etag);
	try {
		deps.onCompleted(result);
	} catch {
		// Onboarding is already authoritative and complete. Analytics is best-effort
		// and must never reverse the product result shown to the seller.
	}
	return result;
}
