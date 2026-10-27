/** 空字串（常見品項）不用 debounce——那是固定的 27 筆，query 本身沒有變化
 *  空間，delay 只會拖慢「打開就看到常見品項」的第一印象。非空字串沿用
 *  HomeCompactSearch 既有的 250ms。 */
export function computeDebounceDelay(query: string): number {
	return query === "" ? 0 : 250;
}

export type CropCatalogKeyAction =
	| { kind: "move"; index: number }
	| { kind: "select"; index: number }
	| { kind: "close" }
	| { kind: "none" };

/**
 * Pure combobox keyboard state transition. Focus stays on the input while the
 * active option wraps through the list; Enter selects only a valid active
 * option, and Escape delegates closing to the caller.
 */
export function resolveCropCatalogKeyAction(
	key: string,
	activeIndex: number | null,
	itemCount: number,
): CropCatalogKeyAction {
	if (key === "Escape") return { kind: "close" };

	const hasActiveOption =
		activeIndex != null && activeIndex >= 0 && activeIndex < itemCount;
	if (key === "Enter") {
		return hasActiveOption
			? { kind: "select", index: activeIndex }
			: { kind: "none" };
	}

	if (key !== "ArrowDown" && key !== "ArrowUp") return { kind: "none" };
	if (itemCount <= 0) return { kind: "none" };

	if (!hasActiveOption) {
		return { kind: "move", index: key === "ArrowDown" ? 0 : itemCount - 1 };
	}

	const delta = key === "ArrowDown" ? 1 : -1;
	return {
		kind: "move",
		index: (activeIndex + delta + itemCount) % itemCount,
	};
}
