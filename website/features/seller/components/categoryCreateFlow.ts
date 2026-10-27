import type { CategoryNode } from "@/lib/products/seller-repo";

export const CREATE_NEW_CATEGORY_VALUE = "__hawkr_create_new_category__";

export interface ExistingCategoryChoice {
	kind: "existing";
	id: string;
	name: string;
}

export interface NewCategoryChoice {
	kind: "new";
	name: string;
}

export type CategoryDraftChoice =
	| { kind: "empty" }
	| ExistingCategoryChoice
	| NewCategoryChoice;

export interface CategoryPathDraft {
	l1: ExistingCategoryChoice | null;
	l2: CategoryDraftChoice;
	l3: CategoryDraftChoice;
}

export type CategoryPathField = "l1" | "l2" | "l3";
export type CategoryPathErrors = Partial<Record<CategoryPathField, string>>;

export type CategoryPathPlan =
	| {
			kind: "commit";
			l1Id: string;
			l2Id: string;
			l3Id: string;
	  }
	| {
			kind: "create-l3";
			l1Id: string;
			l2Id: string;
			l3Name: string;
	  }
	| {
			kind: "create-l2-l3";
			l1Id: string;
			l2Name: string;
			l3Name: string;
	  };

const emptyChoice = (): CategoryDraftChoice => ({ kind: "empty" });

export function emptyCategoryPathDraft(): CategoryPathDraft {
	return { l1: null, l2: emptyChoice(), l3: emptyChoice() };
}

export function existingCategoryChoice(
	node: Pick<CategoryNode, "id" | "name">,
): ExistingCategoryChoice {
	return { kind: "existing", id: String(node.id), name: node.name };
}

export function categoryPathDraftFromIds(
	tree: CategoryNode[],
	path: readonly [string, string, string],
): CategoryPathDraft {
	const l1Node = tree.find((node) => String(node.id) === path[0]);
	if (!l1Node) return emptyCategoryPathDraft();

	const l2Node = (l1Node.children ?? []).find(
		(node) => String(node.id) === path[1],
	);
	if (!l2Node) {
		return {
			l1: existingCategoryChoice(l1Node),
			l2: emptyChoice(),
			l3: emptyChoice(),
		};
	}

	const l3Node = (l2Node.children ?? []).find(
		(node) => String(node.id) === path[2],
	);
	return {
		l1: existingCategoryChoice(l1Node),
		l2: existingCategoryChoice(l2Node),
		l3: l3Node ? existingCategoryChoice(l3Node) : emptyChoice(),
	};
}

export function selectDraftL1(
	draft: CategoryPathDraft,
	node: Pick<CategoryNode, "id" | "name">,
): CategoryPathDraft {
	const choice = existingCategoryChoice(node);
	if (draft.l1?.id === choice.id) {
		return draft.l1.name === choice.name ? draft : { ...draft, l1: choice };
	}
	return {
		...draft,
		l1: choice,
		l2: emptyChoice(),
		l3: emptyChoice(),
	};
}

export function selectDraftL2(
	draft: CategoryPathDraft,
	node: Pick<CategoryNode, "id" | "name">,
): CategoryPathDraft {
	const choice = existingCategoryChoice(node);
	if (draft.l2.kind === "existing" && draft.l2.id === choice.id) {
		return draft.l2.name === choice.name ? draft : { ...draft, l2: choice };
	}
	return {
		...draft,
		l2: choice,
		l3: emptyChoice(),
	};
}

export function startDraftL2(
	draft: CategoryPathDraft,
	name = "",
): CategoryPathDraft {
	return {
		...draft,
		l2: { kind: "new", name },
		// A not-yet-created L2 cannot have existing children. Starting a new L2
		// therefore makes L3 a required new value in the same flow.
		l3: { kind: "new", name: "" },
	};
}

export function markDraftL2Created(
	draft: CategoryPathDraft,
	node: Pick<CategoryNode, "id" | "name">,
): CategoryPathDraft {
	return {
		...draft,
		l2: existingCategoryChoice(node),
		// Preserve the typed L3 name. This is the recovery state when creating L2
		// succeeded but the subsequent L3 request failed.
		l3: draft.l3,
	};
}

export function selectDraftL3(
	draft: CategoryPathDraft,
	node: Pick<CategoryNode, "id" | "name">,
): CategoryPathDraft {
	if (draft.l2.kind === "new") return draft;
	return { ...draft, l3: existingCategoryChoice(node) };
}

export function startDraftL3(
	draft: CategoryPathDraft,
	name = "",
): CategoryPathDraft {
	return { ...draft, l3: { kind: "new", name } };
}

export function updateDraftCategoryName(
	draft: CategoryPathDraft,
	level: "l2" | "l3",
	name: string,
): CategoryPathDraft {
	const choice = draft[level];
	if (choice.kind !== "new") return draft;
	return { ...draft, [level]: { ...choice, name } };
}

export function clearDraftLevel(
	draft: CategoryPathDraft,
	level: "l2" | "l3",
): CategoryPathDraft {
	if (level === "l2") {
		return { ...draft, l2: emptyChoice(), l3: emptyChoice() };
	}
	return { ...draft, l3: emptyChoice() };
}

export function validateNewCategoryName(
	name: string,
	label: "子分類" | "細項",
): string | undefined {
	const trimmed = name.trim();
	if (!trimmed) return `請輸入${label}名稱。`;
	if (trimmed.length > 100) return `${label}名稱不可超過 100 字。`;
	if (/\p{Cc}/u.test(trimmed)) return `${label}名稱包含無法使用的控制字元。`;
	return undefined;
}

export function validateCategoryPathDraft(
	draft: CategoryPathDraft,
): CategoryPathErrors {
	const errors: CategoryPathErrors = {};
	if (!draft.l1) errors.l1 = "請選擇第一層主分類。";

	if (draft.l2.kind === "empty") {
		errors.l2 = "請選擇或新增子分類。";
	} else if (draft.l2.kind === "new") {
		const error = validateNewCategoryName(draft.l2.name, "子分類");
		if (error) errors.l2 = error;
	}

	if (draft.l3.kind === "empty") {
		errors.l3 = "請選擇或新增第三層細項。";
	} else if (draft.l3.kind === "new") {
		const error = validateNewCategoryName(draft.l3.name, "細項");
		if (error) errors.l3 = error;
	}

	return errors;
}

export function categoryPathPlan(
	draft: CategoryPathDraft,
): CategoryPathPlan | null {
	if (Object.keys(validateCategoryPathDraft(draft)).length > 0 || !draft.l1) {
		return null;
	}

	if (draft.l2.kind === "existing" && draft.l3.kind === "existing") {
		return {
			kind: "commit",
			l1Id: draft.l1.id,
			l2Id: draft.l2.id,
			l3Id: draft.l3.id,
		};
	}
	if (draft.l2.kind === "existing" && draft.l3.kind === "new") {
		return {
			kind: "create-l3",
			l1Id: draft.l1.id,
			l2Id: draft.l2.id,
			l3Name: draft.l3.name.trim(),
		};
	}
	if (draft.l2.kind === "new" && draft.l3.kind === "new") {
		return {
			kind: "create-l2-l3",
			l1Id: draft.l1.id,
			l2Name: draft.l2.name.trim(),
			l3Name: draft.l3.name.trim(),
		};
	}
	return null;
}

export function categoryPathLabel(draft: CategoryPathDraft): string {
	const label = (choice: CategoryDraftChoice, fallback: string) => {
		if (choice.kind === "existing") return choice.name;
		if (choice.kind === "new") return choice.name.trim() || fallback;
		return fallback;
	};
	return [
		draft.l1?.name ?? "選擇 L1",
		label(draft.l2, "選擇 L2"),
		label(draft.l3, "選擇 L3"),
	].join(" › ");
}
