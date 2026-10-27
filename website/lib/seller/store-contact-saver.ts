/**
 * KYC 頁的店家資料（聯絡人姓名、聯絡人手機、客服電子信箱與地址）自動儲存：和申請草稿一樣先排隊、一次只有一個寫入在途，
 * 寫入帶最近一次的 etag 與只含改過欄位的 update_mask。哪些欄位還缺由伺服器的 `missing_store_fields` 決定，這裡不判斷。
 */

import type { DefaultErrorText } from "@hawkr/api/connect/errors";
import { type SubmitError, submitError } from "@hawkr/api/connect/submit";
import type { Store } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { SellerAppService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { countyCodeOfTownCode } from "@hawkr/twaddr/lookup";
import type { StorefrontWriteResult, StoreUpdate } from "./repo";
import type { StoreContact } from "./store-contact";
import { type StoreChanges, storeChangesFromKyc } from "./store-form";

export interface StoreFormDraft {
	contact: StoreContact;
	address: { countyCode: string; townCode: string; detail: string };
}

/** 伺服器上的店家在 KYC 頁的輸入形狀（聯絡資料與地址）。 */
export function storeDraftOf(store: Store | undefined): StoreFormDraft {
	const townCode = store?.address?.townCode ?? "";
	return {
		contact: {
			contactName: store?.contactName ?? "",
			contactMobile: store?.contactMobile ?? "",
			supportEmail: store?.supportEmail ?? "",
		},
		address: {
			countyCode: countyCodeOfTownCode(townCode),
			townCode,
			detail: (store?.address?.addressDetail ?? "").trim(),
		},
	};
}

export type StoreSaveResult =
	/** 沒有要寫的欄位。 */
	| { kind: "idle" }
	| { kind: "saved"; store: Store }
	/** 版本衝突：呼叫端讀最新版本、`adopt` 之後再寫一次（輸入保留）。 */
	| { kind: "conflict" }
	| { kind: "failed"; failure: Exclude<SubmitError, { kind: "conflict" }> };

export interface StoreContactSaverDeps {
	save(update: StoreUpdate): Promise<StorefrontWriteResult>;
	/** 沒有伺服器文字時的 code 預設文字（見 `submitError`）。 */
	defaults: DefaultErrorText;
}

export interface StoreContactSaver {
	/** 採用伺服器上的店家（換 baseline 與 etag）。 */
	adopt(store: Store | undefined): void;
	/**
	 * 版本衝突之後採用最新的店家：換 baseline 與 etag，並回傳要放進輸入的值。使用者自上次儲存以來沒有改過的欄位取伺服器的值
	 * （別處改過的不會被當成使用者的修改寫回去），使用者自己改過、還沒寫進去的欄位保留。
	 */
	rebase(draft: StoreFormDraft, latest: Store | undefined): StoreFormDraft;
	/** 目前的輸入相對於 baseline 要寫的欄位；沒有時 null。 */
	pending(draft: StoreFormDraft): StoreChanges | null;
	/** 排在前一個寫入之後寫入目前的輸入；永遠回傳結果、不丟錯。 */
	flush(draft: StoreFormDraft): Promise<StoreSaveResult>;
}

export function createStoreContactSaver(
	deps: StoreContactSaverDeps,
): StoreContactSaver {
	let baseline: Store | undefined;
	let etag = "";
	let chain: Promise<unknown> = Promise.resolve();

	const adopt = (store: Store | undefined) => {
		baseline = store;
		etag = store?.etag ?? "";
	};

	const rebase = (
		draft: StoreFormDraft,
		latest: Store | undefined,
	): StoreFormDraft => {
		const was = storeDraftOf(baseline);
		const server = storeDraftOf(latest);
		const same = (a: string, b: string) => a.trim() === b.trim();
		const pick = (mine: string, base: string, theirs: string) =>
			same(mine, base) ? theirs : mine;
		const addressEdited =
			draft.address.countyCode !== was.address.countyCode ||
			!same(draft.address.townCode, was.address.townCode) ||
			!same(draft.address.detail, was.address.detail);
		adopt(latest);
		return {
			contact: {
				contactName: pick(
					draft.contact.contactName,
					was.contact.contactName,
					server.contact.contactName,
				),
				contactMobile: pick(
					draft.contact.contactMobile,
					was.contact.contactMobile,
					server.contact.contactMobile,
				),
				supportEmail: pick(
					draft.contact.supportEmail,
					was.contact.supportEmail,
					server.contact.supportEmail,
				),
			},
			address: addressEdited ? draft.address : server.address,
		};
	};

	const pending = (draft: StoreFormDraft): StoreChanges | null => {
		if (!baseline || !etag) return null;
		const changes = storeChangesFromKyc(baseline, draft);
		return Object.keys(changes).length > 0 ? changes : null;
	};

	const run = async (draft: StoreFormDraft): Promise<StoreSaveResult> => {
		const changes = pending(draft);
		if (!changes) return { kind: "idle" };
		try {
			const written = await deps.save({ etag, ...changes });
			if (written.store) adopt(written.store);
			return written.store
				? { kind: "saved", store: written.store }
				: { kind: "idle" };
		} catch (e) {
			const failure = submitError(
				e,
				deps.defaults,
				SellerAppService.method.updateMyStore,
			);
			if (failure.kind === "conflict") return { kind: "conflict" };
			return { kind: "failed", failure };
		}
	};

	const flush = (draft: StoreFormDraft): Promise<StoreSaveResult> => {
		const next = chain.then(
			() => run(draft),
			() => run(draft),
		);
		chain = next;
		return next;
	};

	return { adopt, rebase, pending, flush };
}
