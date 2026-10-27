"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import CategorySelector from "@/features/seller/components/CategorySelector";
import { useCategoryResolution } from "@/lib/hooks/useCategoryResolution";
import { usePublicCategoryTree } from "@/lib/hooks/usePublicCategoryTree";
import type { PricebookSelection } from "@/lib/pricebook/repo";
import {
	type CategoryNode,
	findCategoryPath,
} from "@/lib/products/seller-repo";
import {
	type CategoryResolutionOutcome,
	resolveCategoryResolutionOutcome,
} from "./categoryResolutionOutcome";
import {
	type PartialCategorySelection,
	resolvePartialCategorySelection,
} from "./partialCategorySelection";

export interface CropCatalogSearchHandle {
	focus: () => void;
}

interface CropCatalogSearchProps {
	selection: PricebookSelection | null;
	selectedCategoryId?: number;
	selectedLabel?: string | null;
	onResolve: (
		selection: PricebookSelection,
		label: string,
		categoryId: number,
	) => void;
	onCategoryChange: (categoryId: number | undefined) => void;
	/** Fires whenever the deepest picked level is short of L3 (L1-only or
	 *  L1+L2), with `null` once L3 is picked or nothing is picked — lets a
	 *  caller show the simplified `GetCategoryPriceRollup` summary while the
	 *  full crop/group resolution (`onResolve`) isn't reachable yet. */
	onPartialCategoryChange?: (partial: PartialCategorySelection | null) => void;
}

interface SelectedCategoryContext {
	leafName: string;
	pathLabel: string;
}

function selectedCategoryContext(
	tree: CategoryNode[],
	categoryId: number | undefined,
): SelectedCategoryContext | null {
	const [l1Id, l2Id, l3Id] = findCategoryPath(tree, categoryId);
	if (!l1Id || !l2Id || !l3Id) return null;
	const l1 = tree.find((node) => String(node.id) === l1Id);
	const l2 = l1?.children?.find((node) => String(node.id) === l2Id);
	const l3 = l2?.children?.find((node) => String(node.id) === l3Id);
	if (!l1 || !l2 || !l3) return null;
	return {
		leafName: l3.name,
		pathLabel: [l1.name, l2.name, l3.name].join(" / "),
	};
}

function selectionKey(selection: PricebookSelection | null) {
	if (!selection) return undefined;
	switch (selection.kind) {
		case "group":
			return `group:${selection.groupCode}`;
		case "crop":
			return `crop:${selection.cropCode}`;
		case "cropName":
			return `crop-name:${selection.cropName}`;
	}
}

/** Inline three-level classification selector for the Pricebook workspace.
 *  Resolution to a Pricebook crop/group is entirely backend-authoritative
 *  (`/sellers/pricebook/category-resolution`, keyed by categoryId) — this
 *  component never guesses from a name search (issue #2786). */
export const CropCatalogSearch = React.forwardRef<
	CropCatalogSearchHandle,
	CropCatalogSearchProps
>(function CropCatalogSearch(
	{
		selection,
		selectedCategoryId,
		selectedLabel,
		onResolve,
		onCategoryChange,
		onPartialCategoryChange,
	},
	ref,
) {
	const rootRef = React.useRef<HTMLFieldSetElement>(null);
	const focusRequestedRef = React.useRef(false);
	const lastAppliedSignatureRef = React.useRef<string | null>(null);
	const activeUserCategoryRef = React.useRef<number | "draft" | null>(null);
	const [categoryId, setCategoryId] = React.useState<number | undefined>(
		selectedCategoryId,
	);
	const categoryTreeSource = usePublicCategoryTree();
	const categoryContext = React.useMemo(
		() => selectedCategoryContext(categoryTreeSource.tree, categoryId),
		[categoryId, categoryTreeSource.tree],
	);
	const resolution = useCategoryResolution(
		categoryId != null ? String(categoryId) : undefined,
		{ enabled: categoryContext != null },
	);

	React.useEffect(() => {
		if (selectedCategoryId == null) return;
		const activeUserCategory = activeUserCategoryRef.current;
		if (
			activeUserCategory !== null &&
			activeUserCategory !== selectedCategoryId
		) {
			return;
		}
		setCategoryId(selectedCategoryId);
		if (activeUserCategory === selectedCategoryId) {
			activeUserCategoryRef.current = null;
		}
	}, [selectedCategoryId]);

	const focusFirstControl = React.useCallback(() => {
		const control = rootRef.current?.querySelector<HTMLButtonElement>(
			'[aria-haspopup="listbox"]',
		);
		if (!control) return false;
		control.focus({ preventScroll: true });
		focusRequestedRef.current = false;
		return true;
	}, []);

	React.useEffect(() => {
		if (categoryTreeSource.loading || !focusRequestedRef.current) return;
		const frame = window.requestAnimationFrame(focusFirstControl);
		return () => window.cancelAnimationFrame(frame);
	}, [categoryTreeSource.loading, focusFirstControl]);

	React.useImperativeHandle(
		ref,
		() => ({
			focus: () => {
				focusRequestedRef.current = true;
				rootRef.current?.scrollIntoView({
					block: "center",
					behavior: "smooth",
				});
				window.requestAnimationFrame(focusFirstControl);
			},
		}),
		[focusFirstControl],
	);

	const handleCategoryPathChange = React.useCallback(
		(path: [string, string, string]) => {
			onPartialCategoryChange?.(
				resolvePartialCategorySelection(categoryTreeSource.tree, path),
			);
		},
		[categoryTreeSource.tree, onPartialCategoryChange],
	);

	const outcome = React.useMemo(
		() => resolveCategoryResolutionOutcome(resolution.data),
		[resolution.data],
	);
	const currentSelectionKey = selectionKey(selection);
	const applyOutcome = React.useCallback(
		(resolved: Extract<CategoryResolutionOutcome, { kind: "resolved" }>) => {
			if (categoryId == null) return;
			const activeUserCategory = activeUserCategoryRef.current;
			if (activeUserCategory !== null && activeUserCategory !== categoryId) {
				return;
			}
			lastAppliedSignatureRef.current = `${categoryId}:${resolved.signature}`;
			onResolve(resolved.selection, resolved.label, categoryId);
		},
		[categoryId, onResolve],
	);

	React.useEffect(() => {
		if (
			categoryId == null ||
			resolution.loading ||
			resolution.error ||
			outcome.kind !== "resolved"
		) {
			return;
		}
		const signature = `${categoryId}:${outcome.signature}`;
		if (
			currentSelectionKey === outcome.signature ||
			lastAppliedSignatureRef.current === signature
		) {
			return;
		}
		applyOutcome(outcome);
	}, [
		applyOutcome,
		resolution.error,
		resolution.loading,
		categoryId,
		currentSelectionKey,
		outcome,
	]);

	const selectionStatus = !categoryContext
		? selectedLabel
			? `目前行情為「${selectedLabel}」；重新選到細項後會直接更新。`
			: "請依序選擇分類、子分類與細項。"
		: resolution.loading
			? `正在套用「${categoryContext.leafName}」的行情資料…`
			: resolution.error
				? "這個細項的行情資料暫時無法載入。"
				: outcome.kind === "ambiguous"
					? "這個細項目前有多筆同名行情資料，系統已停止套用以避免顯示錯誤價格。"
					: outcome.kind === "invalid"
						? "這個細項的行情代碼不完整，暫時無法分析。"
						: outcome.kind === "unmapped"
							? "這個細項目前沒有可唯一對應的行情資料。"
							: currentSelectionKey === outcome.signature
								? `已套用「${outcome.label}」，下方價格與色階會同步更新。`
								: `正在套用「${outcome.label}」…`;

	return (
		<fieldset ref={rootRef} className="seller-price-lookup__inline-catalog">
			<legend className="sr-only">商品分類</legend>
			<CategorySelector
				value={categoryId}
				allowCreate={false}
				treeSource={categoryTreeSource}
				onChange={(nextCategoryId) => {
					lastAppliedSignatureRef.current = null;
					activeUserCategoryRef.current = nextCategoryId ?? "draft";
					setCategoryId(nextCategoryId);
					onCategoryChange(nextCategoryId);
				}}
				onPathChange={handleCategoryPathChange}
			/>

			{resolution.error ? (
				<div className="seller-price-lookup__catalog-error" role="alert">
					<span>無法確認這個細項的行情資料。</span>
					<Button
						type="button"
						variant="light"
						color="danger"
						size="sm"
						onClick={resolution.retry}
					>
						重新載入
					</Button>
				</div>
			) : null}

			<p
				className="seller-price-lookup__catalog-status"
				role="status"
				aria-live="polite"
			>
				{categoryContext
					? `${categoryContext.pathLabel}。${selectionStatus}`
					: selectionStatus}
			</p>
		</fieldset>
	);
});
