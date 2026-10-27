import type { InlineRun } from "@/types/product";

export interface TableCell {
	runs?: InlineRun[];
	textAlign?: "center" | "right";
}

export interface TableModel {
	withHeadings: boolean;
	rows: TableCell[][];
}

export interface ListItemModel {
	runs?: InlineRun[];
}

export interface ListModel {
	ordered: boolean;
	items: ListItemModel[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

export function parseTableJson(
	tableJson: string | undefined,
): TableModel | null {
	if (!tableJson) return null;
	let parsed: unknown;
	try {
		parsed = JSON.parse(tableJson);
	} catch {
		return null;
	}
	if (
		!isRecord(parsed) ||
		!Array.isArray(parsed.rows) ||
		parsed.rows.length === 0
	) {
		return null;
	}
	return {
		withHeadings: parsed.withHeadings === true,
		rows: parsed.rows as TableCell[][],
	};
}

export function parseListJson(listJson: string | undefined): ListModel | null {
	if (!listJson) return null;
	let parsed: unknown;
	try {
		parsed = JSON.parse(listJson);
	} catch {
		return null;
	}
	if (
		!isRecord(parsed) ||
		!Array.isArray(parsed.items) ||
		parsed.items.length === 0
	) {
		return null;
	}
	return {
		ordered: parsed.ordered === true,
		items: parsed.items as ListItemModel[],
	};
}

export function formatCategoryBreadcrumb(
	names: Array<string | null | undefined>,
): string {
	const filtered = names.filter(
		(name): name is string =>
			typeof name === "string" && name.trim().length > 0,
	);
	return filtered.length > 0 ? filtered.join(" > ") : "-";
}
