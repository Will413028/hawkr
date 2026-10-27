import type { CategoryNode } from "@/lib/products/seller-repo";

export interface PartialCategorySelection {
	level: 1 | 2;
	categoryId: string;
	label: string;
}

/** Given the raw `[l1Id, l2Id, l3Id]` path `CategorySelector` reports and the
 *  category tree, decide which id (if any) should drive the simplified
 *  `GetCategoryPriceRollup` summary — the deepest level picked short of L3.
 *  Returns `null` once L3 is also picked: the full crop/group resolution
 *  flow (`useCategoryResolution`) takes over entirely and this summary never
 *  coexists with it. */
export function resolvePartialCategorySelection(
	tree: CategoryNode[],
	[l1Id, l2Id, l3Id]: [string, string, string],
): PartialCategorySelection | null {
	if (l3Id) return null;
	const l1 = tree.find((node) => String(node.id) === l1Id);
	if (!l1) return null;
	if (l2Id) {
		const l2 = l1.children?.find((node) => String(node.id) === l2Id);
		if (!l2) return null;
		return { level: 2, categoryId: l2Id, label: l2.name };
	}
	return { level: 1, categoryId: l1Id, label: l1.name };
}
