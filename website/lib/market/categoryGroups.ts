/**
 * Top-level category groups, derived from `/public/categories/tree`.
 *
 * A product list row (`handler.ProductItem`) carries only the leaf it was filed
 * under — `categoryId` is the lv2 id on today's list wire, and detail rows may
 * carry an lv3 — with no lv1 ancestor. The home shelves are themed by lv1
 * ("海鮮", "水果" …), so deciding whether a product belongs on a shelf needs
 * the ancestor the wire omits. This index is every lv1 node with the ids of all
 * its descendants: a few KB shipped inside the home bootstrap, instead of the
 * ~200KB tree the market filters fetch on demand.
 *
 * Pure and client-safe: the server page and the browser refresh both run it
 * through `buildHomeSnapshot`.
 */
export interface CategoryGroup {
	id: string;
	name: string;
	/** Ids of every descendant node at any depth, excluding `id` itself. */
	descendantIds: string[];
}

interface TreeNodeLike {
	id?: unknown;
	name?: unknown;
	children?: unknown;
}

function asNode(value: unknown): TreeNodeLike | null {
	return value && typeof value === "object" ? (value as TreeNodeLike) : null;
}

function idOf(node: TreeNodeLike): string {
	if (typeof node.id === "number" && Number.isFinite(node.id)) {
		return String(node.id);
	}
	return typeof node.id === "string" ? node.id.trim() : "";
}

function nameOf(node: TreeNodeLike): string {
	return typeof node.name === "string" ? node.name.trim() : "";
}

function collectDescendantIds(
	children: unknown,
	into: string[],
	seen: Set<string>,
): void {
	if (!Array.isArray(children)) return;
	for (const raw of children) {
		const node = asNode(raw);
		if (!node) continue;
		const id = idOf(node);
		if (id && !seen.has(id)) {
			seen.add(id);
			into.push(id);
		}
		collectDescendantIds(node.children, into, seen);
	}
}

/**
 * The public tree endpoint answers with the root array directly; the market
 * filter code also passes `{ list: [...] }` envelopes around. Accept both, and
 * treat anything else as "no tree" rather than guessing.
 */
function rootsOf(tree: unknown): unknown[] {
	if (Array.isArray(tree)) return tree;
	const list = (tree as { list?: unknown } | null)?.list;
	return Array.isArray(list) ? list : [];
}

export function categoryGroupsFromTree(tree: unknown): CategoryGroup[] {
	const groups: CategoryGroup[] = [];
	for (const raw of rootsOf(tree)) {
		const node = asNode(raw);
		if (!node) continue;
		const id = idOf(node);
		const name = nameOf(node);
		if (!id || !name) continue;
		const descendantIds: string[] = [];
		collectDescendantIds(node.children, descendantIds, new Set([id]));
		groups.push({ id, name, descendantIds });
	}
	return groups;
}

/** Every id that counts as "inside" a group: the group's own plus its subtree. */
export function categoryGroupMemberIds(group: CategoryGroup): Set<string> {
	return new Set([group.id, ...group.descendantIds]);
}

/** The group a product's leaf `categoryId` rolls up to, if any. */
export function findCategoryGroupByName(
	groups: readonly CategoryGroup[],
	name: string,
): CategoryGroup | undefined {
	const wanted = name.trim();
	return groups.find((group) => group.name === wanted);
}

/** Leaf category id/name as strings, or nothing at all when the wire carries
 *  none — the keys are omitted (not `undefined`) so snapshot equality checks
 *  keep treating "no category" and "field absent" as the same shape. */
export function readCategory(product: {
	categoryId?: string | number;
	categoryLv2Id?: string | number;
	categoryName?: string;
}): { categoryId?: string; categoryName?: string } {
	const rawId = product.categoryId ?? product.categoryLv2Id;
	const categoryId =
		rawId === undefined || rawId === null ? "" : String(rawId).trim();
	const categoryName = product.categoryName?.trim() ?? "";
	return {
		...(categoryId ? { categoryId } : {}),
		...(categoryName ? { categoryName } : {}),
	};
}
