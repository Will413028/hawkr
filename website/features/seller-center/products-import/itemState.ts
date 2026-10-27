import { IMPORT_ITEM_STATUS, type ImportItemBrief } from "./types";

const count = (value: string | number | undefined): number =>
	Number(value ?? "0") || 0;

export function isImportItemSelectable(item: ImportItemBrief): boolean {
	return (
		item.status !== IMPORT_ITEM_STATUS.COMMITTED &&
		item.status !== IMPORT_ITEM_STATUS.COMMITTING
	);
}

export function isImportItemBatchEditable(item: ImportItemBrief): boolean {
	return (
		isImportItemSelectable(item) && item.status !== IMPORT_ITEM_STATUS.REMOVED
	);
}

export function isImportItemRemovable(item: ImportItemBrief): boolean {
	return isImportItemBatchEditable(item);
}

export function isImportItemRestorable(item: ImportItemBrief): boolean {
	return item.status === IMPORT_ITEM_STATUS.REMOVED;
}

export function isImportItemCommitEligible(item: ImportItemBrief): boolean {
	return (
		(item.status === IMPORT_ITEM_STATUS.READY ||
			item.status === IMPORT_ITEM_STATUS.COMMIT_FAILED) &&
		count(item.blockingErrorCount) === 0
	);
}

export function isImportItemPreviewable(item: ImportItemBrief): boolean {
	return (
		item.status === IMPORT_ITEM_STATUS.READY &&
		count(item.blockingErrorCount) === 0
	);
}

export function isImportItemEditable(item: ImportItemBrief): boolean {
	return item.status !== IMPORT_ITEM_STATUS.COMMITTED;
}

/**
 * 正式商品是否已被移除（軟刪）。
 *
 * backend #2819：`formalProductPublicationState` 是 ProductPublicationState，
 * `"4014"` = REMOVED。此時 `/products/{id}` 回 404「商品已永久移除」，
 * 因此**不可**再顯示「前往正式商品」連結。
 *
 * 🔴 與 `formalProductStatusUnavailable` 是兩件事：本函式答「確定已移除」，
 * 那個欄位答「後端不知道」。修復前兩者都表現為欄位為空，賣家無從分辨。
 */
export const PRODUCT_PUBLICATION_STATE_REMOVED = "4014";

export function isFormalProductRemoved(item: ImportItemBrief): boolean {
	return (
		item.formalProductPublicationState === PRODUCT_PUBLICATION_STATE_REMOVED
	);
}
