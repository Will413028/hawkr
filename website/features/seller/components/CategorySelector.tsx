"use client";

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { Dialog, Dropdown, Input } from "@/components/ui";
import Button from "@/components/ui/Button";
import { ApiClientError } from "@/lib/api/client";
import { toast } from "@/lib/feedback/toast";
import { useCategoryTree } from "@/lib/hooks/useCategoryTree";
import {
	type CategoryNode,
	categoryDropdownItems,
	createCategory,
	findCategoryPath,
	upsertCategoryNode,
} from "@/lib/products/seller-repo";
import { queryKeys } from "@/lib/query/keys";
import {
	type CategoryPathDraft,
	type CategoryPathErrors,
	CREATE_NEW_CATEGORY_VALUE,
	categoryPathDraftFromIds,
	categoryPathLabel,
	categoryPathPlan,
	clearDraftLevel,
	emptyCategoryPathDraft,
	markDraftL2Created,
	selectDraftL1,
	selectDraftL2,
	selectDraftL3,
	startDraftL2,
	startDraftL3,
	updateDraftCategoryName,
	validateCategoryPathDraft,
} from "./categoryCreateFlow";

export interface CategorySelectorTreeSource {
	tree: CategoryNode[];
	loading: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

interface Props {
	value: number | undefined;
	onChange: (categoryId: number | undefined) => void;
	/** Seller product forms may create an immediately-active L2/L3 category. */
	allowCreate?: boolean;
	/** Read-only consumers may supply the public ACTIVE tree and avoid the
	 * product-management capability required by `/app/categories/tree`. */
	treeSource?: CategorySelectorTreeSource;
	/** Fires on every L1/L2/L3 dropdown change with the raw path, including
	 * partial (pre-leaf) selections that `onChange` doesn't report (it only
	 * fires once a leaf L3 is reached). Additive and optional — existing
	 * consumers that don't pass it see no behavior change. */
	onPathChange?: (path: [string, string, string]) => void;
}

/**
 * Three-level cascading category picker. Mirrors mobile's UX of L1 → L2 →
 * L3 selection but compresses to a single row of three searchable
 * <Dropdown>s (same control as the 單位 / 縣市 selectors) so deep variety
 * lists can be filtered by typing a keyword.
 *
 * The full tree comes back from `/app/categories/tree` in one request
 * (mobile does the same); `useCategoryTree` keeps it in the shared query
 * cache — no refetch when navigating between create/edit.
 */
export default function CategorySelector({
	value,
	onChange,
	allowCreate = false,
	treeSource,
	onPathChange,
}: Props) {
	const sellerTreeSource = useCategoryTree({ enabled: treeSource == null });
	const { tree, loading, error, refresh } = treeSource ?? sellerTreeSource;
	const queryClient = useQueryClient();
	const createFormId = React.useId();
	const createButtonRef = React.useRef<HTMLButtonElement & HTMLAnchorElement>(
		null,
	);

	// `localPath` is the source of truth for what the three dropdowns show.
	// It must NOT be derived from `value`, because `value` stays undefined
	// until the user reaches a leaf (L3) — driving the options off `value`
	// leaves L2/L3 permanently empty (and therefore disabled) the moment
	// L1 is picked. `value` only *seeds* this path (see the effect below)
	// so an existing product opens with its category pre-filled in edit mode.
	const [localPath, setLocalPath] = React.useState<[string, string, string]>([
		"",
		"",
		"",
	]);

	const l1Options = tree;
	const l2Options =
		l1Options.find((n) => String(n.id) === localPath[0])?.children ?? [];
	const l3Options =
		l2Options.find((n) => String(n.id) === localPath[1])?.children ?? [];
	const selectedL1 = l1Options.find((node) => String(node.id) === localPath[0]);

	const [createOpen, setCreateOpen] = React.useState(false);
	const [createDraft, setCreateDraft] = React.useState<CategoryPathDraft>(() =>
		emptyCategoryPathDraft(),
	);
	const [createFieldErrors, setCreateFieldErrors] =
		React.useState<CategoryPathErrors>({});
	const [createError, setCreateError] = React.useState<string | null>(null);
	const [createNotice, setCreateNotice] = React.useState<string | null>(null);
	const [creating, setCreating] = React.useState(false);
	const [announcement, setAnnouncement] = React.useState("");

	const toItems = categoryDropdownItems;
	const createL2Options = createDraft.l1
		? (tree.find((node) => String(node.id) === createDraft.l1?.id)?.children ??
			[])
		: [];
	const createL2Id =
		createDraft.l2.kind === "existing" ? createDraft.l2.id : null;
	const createL3Options =
		createL2Id !== null
			? (createL2Options.find((node) => String(node.id) === createL2Id)
					?.children ?? [])
			: [];

	const handleL1 = (id: string) => {
		// Reset L2 / L3 — emit undefined until the user drills down to a leaf.
		onChange(undefined);
		const path: [string, string, string] = [id, "", ""];
		setLocalPath(path);
		onPathChange?.(path);
	};
	const handleL2 = (id: string) => {
		onChange(undefined);
		const path: [string, string, string] = [localPath[0], id, ""];
		setLocalPath(path);
		onPathChange?.(path);
	};
	const handleL3 = (id: string) => {
		const numId = Number(id);
		onChange(Number.isFinite(numId) ? numId : undefined);
		const path: [string, string, string] = [localPath[0], localPath[1], id];
		setLocalPath(path);
		onPathChange?.(path);
	};

	const restoreCreateButtonFocus = React.useCallback(() => {
		window.requestAnimationFrame(() => createButtonRef.current?.focus());
	}, []);

	const openCreateDialog = () => {
		setCreateDraft(categoryPathDraftFromIds(tree, localPath));
		setCreateFieldErrors({});
		setCreateError(null);
		setCreateNotice(null);
		setCreateOpen(true);
	};

	const closeCreateDialog = () => {
		if (creating) return;
		setCreateOpen(false);
		setCreateFieldErrors({});
		setCreateError(null);
		setCreateNotice(null);
		restoreCreateButtonFocus();
	};

	React.useEffect(() => {
		if (!createOpen) return;
		const frame = window.requestAnimationFrame(() => {
			document.getElementById(`${createFormId}-l1`)?.focus();
		});
		return () => window.cancelAnimationFrame(frame);
	}, [createFormId, createOpen]);

	const cacheCreatedCategory = React.useCallback(
		(parentId: string | number, category: CategoryNode) => {
			if (!Number.isFinite(Number(category.id))) {
				throw new Error("分類服務回傳了無效的分類編號");
			}
			queryClient.setQueryData<CategoryNode[]>(
				queryKeys.categories.tree(),
				(current = []) => upsertCategoryNode(current, parentId, category),
			);
			void queryClient.invalidateQueries({
				queryKey: queryKeys.categories.tree(),
				refetchType: "none",
			});
		},
		[queryClient],
	);

	const commitDialogPath = React.useCallback(
		(path: [string, string, string], leafName: string, created: boolean) => {
			const id = Number(path[2]);
			if (!Number.isFinite(id)) {
				throw new Error("分類服務回傳了無效的分類編號");
			}
			setLocalPath(path);
			onChange(id);
			const message = created
				? `已新增並選取「${leafName}」`
				: `已選取分類「${leafName}」`;
			setAnnouncement(message);
			if (created) {
				toast.success(message, { description: "分類已加入商品分類選單。" });
			} else {
				toast.info(message);
			}
			setCreateOpen(false);
			setCreateError(null);
			setCreateNotice(null);
			restoreCreateButtonFocus();
		},
		[onChange, restoreCreateButtonFocus],
	);

	const handleCreateCategory = async (event: React.FormEvent) => {
		event.preventDefault();
		if (creating) return;

		const fieldErrors = validateCategoryPathDraft(createDraft);
		setCreateFieldErrors(fieldErrors);
		const plan = categoryPathPlan(createDraft);
		if (!plan) return;

		setCreating(true);
		setCreateError(null);
		setCreateNotice(null);
		let newlyCreatedL2: CategoryNode | null = null;
		try {
			if (plan.kind === "commit") {
				const leaf = createDraft.l3;
				commitDialogPath(
					[plan.l1Id, plan.l2Id, plan.l3Id],
					leaf.kind === "existing" ? leaf.name : "已選分類",
					false,
				);
				return;
			}

			if (plan.kind === "create-l3") {
				const existingL3 = createL3Options.find(
					(node) => node.name.trim() === plan.l3Name,
				);
				if (existingL3) {
					commitDialogPath(
						[plan.l1Id, plan.l2Id, String(existingL3.id)],
						existingL3.name,
						false,
					);
					return;
				}
				const createdL3 = await createCategory({
					parentId: plan.l2Id,
					name: plan.l3Name,
				});
				cacheCreatedCategory(plan.l2Id, createdL3);
				commitDialogPath(
					[plan.l1Id, plan.l2Id, String(createdL3.id)],
					createdL3.name,
					true,
				);
				return;
			}

			let resolvedL2 = createL2Options.find(
				(node) => node.name.trim() === plan.l2Name,
			);
			if (!resolvedL2) {
				const createdL2 = await createCategory({
					parentId: plan.l1Id,
					name: plan.l2Name,
				});
				resolvedL2 = createdL2;
				newlyCreatedL2 = createdL2;
				cacheCreatedCategory(plan.l1Id, createdL2);
				setCreateDraft((current) => markDraftL2Created(current, createdL2));
				setCreateNotice(`子分類「${createdL2.name}」已建立，正在建立細項。`);
			} else {
				const existingL2 = resolvedL2;
				setCreateDraft((current) => markDraftL2Created(current, existingL2));
			}

			if (!resolvedL2) throw new Error("無法取得已建立的子分類");
			const resolvedL2Node = resolvedL2;
			const l2Id = String(resolvedL2Node.id);
			const existingL3 = (resolvedL2Node.children ?? []).find(
				(node) => node.name.trim() === plan.l3Name,
			);
			if (existingL3) {
				commitDialogPath(
					[plan.l1Id, l2Id, String(existingL3.id)],
					existingL3.name,
					Boolean(newlyCreatedL2),
				);
				return;
			}

			const createdL3 = await createCategory({
				parentId: l2Id,
				name: plan.l3Name,
			});
			cacheCreatedCategory(l2Id, createdL3);
			commitDialogPath(
				[plan.l1Id, l2Id, String(createdL3.id)],
				createdL3.name,
				true,
			);
		} catch (cause) {
			const message = categoryCreateErrorMessage(cause);
			if (newlyCreatedL2) {
				setCreateNotice(`子分類「${newlyCreatedL2.name}」已建立。`);
				setCreateError(`細項尚未完成；請保留目前路徑並重試。${message}`);
			} else {
				setCreateError(message);
			}
		} finally {
			setCreating(false);
		}
	};

	// Seed the dropdowns from `value` when it resolves to a real category
	// (edit mode). Guarded on a non-empty L1 so it never clobbers an
	// in-progress selection: while the user is partway through, `value` is
	// undefined → path is empty → we leave `localPath` untouched.
	React.useEffect(() => {
		const [l1Id, l2Id, l3Id] = findCategoryPath(tree, value);
		if (
			!l1Id ||
			(localPath[0] === l1Id && localPath[1] === l2Id && localPath[2] === l3Id)
		) {
			return;
		}
		setLocalPath([l1Id, l2Id, l3Id]);
	}, [localPath, tree, value]);

	const clearCreateFeedback = (...fields: Array<keyof CategoryPathErrors>) => {
		setCreateFieldErrors((current) => {
			const next = { ...current };
			for (const field of fields) delete next[field];
			return next;
		});
		setCreateError(null);
		setCreateNotice(null);
	};

	const createPlan = categoryPathPlan(createDraft);
	const createL2Items = [
		...toItems(createL2Options),
		{
			value: CREATE_NEW_CATEGORY_VALUE,
			label: "＋ 自行輸入新的子分類",
			alwaysVisible: true,
			description: createDraft.l1
				? `建立在「${createDraft.l1.name}」下`
				: undefined,
		},
	];
	const createL3Items = [
		...toItems(createL3Options),
		{
			value: CREATE_NEW_CATEGORY_VALUE,
			label: "＋ 自行輸入新的細項",
			alwaysVisible: true,
			description:
				createDraft.l2.kind === "existing"
					? `建立在「${createDraft.l2.name}」下`
					: undefined,
		},
	];

	const handleCreateL1 = (id: string) => {
		const node = tree.find((item) => String(item.id) === id);
		if (!node) return;
		setCreateDraft((current) => selectDraftL1(current, node));
		clearCreateFeedback("l1", "l2", "l3");
	};

	const handleCreateL2 = (id: string) => {
		if (id === CREATE_NEW_CATEGORY_VALUE) {
			setCreateDraft((current) => startDraftL2(current));
		} else {
			const node = createL2Options.find((item) => String(item.id) === id);
			if (!node) return;
			setCreateDraft((current) => selectDraftL2(current, node));
		}
		clearCreateFeedback("l2", "l3");
	};

	const handleCreateL3 = (id: string) => {
		if (id === CREATE_NEW_CATEGORY_VALUE) {
			setCreateDraft((current) => startDraftL3(current));
		} else {
			const node = createL3Options.find((item) => String(item.id) === id);
			if (!node) return;
			setCreateDraft((current) => selectDraftL3(current, node));
		}
		clearCreateFeedback("l3");
	};

	if (loading) {
		return (
			<div className="cat-selector__loading" aria-busy="true" role="status">
				{["分類", "子分類", "細項"].map((label) => (
					<span key={label} aria-hidden="true" />
				))}
				<span className="visually-hidden">分類載入中</span>
			</div>
		);
	}
	if (error) {
		return (
			<div className="cat-selector__error" role="alert">
				<span>{error}</span>
				<Button
					variant="bordered"
					color="danger"
					size="sm"
					onClick={() => void refresh()}
				>
					重新載入
				</Button>
			</div>
		);
	}

	return (
		<div className="cat-selector-shell">
			<div className="cat-selector">
				<Dropdown
					label="分類"
					placeholder="選擇分類"
					variant="bordered"
					size="lg"
					floatingLabel
					searchable
					noResultsLabel="找不到符合的分類"
					value={localPath[0] || undefined}
					onChange={handleL1}
					items={toItems(l1Options)}
				/>

				<Dropdown
					label="子分類"
					placeholder="選擇子分類"
					variant="bordered"
					size="lg"
					floatingLabel
					searchable
					noResultsLabel="找不到符合的子分類"
					value={localPath[1] || undefined}
					onChange={handleL2}
					items={toItems(l2Options)}
					disabled={!localPath[0] || l2Options.length === 0}
				/>

				<Dropdown
					label="細項"
					placeholder="選擇細項"
					variant="bordered"
					size="lg"
					floatingLabel
					searchable
					noResultsLabel="找不到符合的細項"
					value={localPath[2] || undefined}
					onChange={handleL3}
					items={toItems(l3Options)}
					disabled={!localPath[1] || l3Options.length === 0}
				/>
			</div>

			{allowCreate ? (
				<>
					<div className="cat-selector__create-bar">
						<div className="cat-selector__create-copy">
							<strong>找不到合適的分類？</strong>
							<span>
								{selectedL1
									? "可從目前路徑繼續選擇或自行輸入。"
									: "可在視窗內選擇主分類，再新增子分類與細項。"}
							</span>
						</div>
						<Button
							ref={createButtonRef}
							variant="bordered"
							color="primary"
							size="sm"
							onClick={openCreateDialog}
						>
							新增分類
						</Button>
					</div>
					<p className="visually-hidden" role="status" aria-live="polite">
						{announcement}
					</p>
				</>
			) : null}

			<Dialog
				open={createOpen}
				onClose={closeCreateDialog}
				title="新增商品分類"
				description="直接調整分類路徑；L2、L3 找不到時可自行新增。"
				size="lg"
				isDismissable={!creating}
				footer={
					<>
						<Button
							variant="bordered"
							color="default"
							onClick={closeCreateDialog}
							disabled={creating}
						>
							取消
						</Button>
						<Button
							type="submit"
							form={createFormId}
							isLoading={creating}
							loadingLabel={
								createPlan?.kind === "commit"
									? "選取分類中…"
									: "新增並選取分類中…"
							}
							disabled={!createPlan || creating}
						>
							{createPlan?.kind === "commit" ? "選取此分類" : "新增並選取"}
						</Button>
					</>
				}
			>
				<form
					id={createFormId}
					className="grid gap-5"
					onSubmit={handleCreateCategory}
				>
					<fieldset className="m-0 grid min-w-0 gap-3 rounded-[var(--hkui-control-radius-lg)] border border-[var(--hkui-border)] bg-[var(--hkui-default-50)] px-4 py-4">
						<legend className="px-1 text-sm font-semibold text-[var(--hkui-foreground)]">
							目前路徑
						</legend>

						<div className="grid min-w-0 grid-cols-1 items-start gap-3 min-[721px]:grid-cols-3">
							<Dropdown
								id={`${createFormId}-l1`}
								label="L1 主分類"
								placeholder="選擇主分類"
								variant="bordered"
								size="lg"
								searchable
								isRequired
								disabled={creating}
								value={createDraft.l1?.id}
								items={toItems(tree)}
								description="只能選擇平台既有分類"
								errorMessage={createFieldErrors.l1}
								onChange={handleCreateL1}
							/>

							<div className="grid min-w-0 content-start gap-2">
								{createDraft.l2.kind === "new" ? (
									<>
										<Input
											id={`${createFormId}-l2-new`}
											label="L2 新子分類名稱"
											variant="bordered"
											size="lg"
											isRequired
											autoFocus
											disabled={creating}
											maxLength={100}
											value={createDraft.l2.name}
											description={`${createDraft.l2.name.trim().length} / 100 字`}
											errorMessage={createFieldErrors.l2}
											onChange={(event) => {
												setCreateDraft((current) =>
													updateDraftCategoryName(
														current,
														"l2",
														event.target.value,
													),
												);
												clearCreateFeedback("l2");
											}}
										/>
										<Button
											type="button"
											variant="light"
											size="sm"
											className="min-h-11 justify-self-start"
											disabled={creating}
											onClick={() => {
												setCreateDraft((current) =>
													clearDraftLevel(current, "l2"),
												);
												clearCreateFeedback("l2", "l3");
											}}
										>
											改選既有子分類
										</Button>
									</>
								) : (
									<Dropdown
										id={`${createFormId}-l2`}
										label="L2 子分類"
										placeholder="選擇或新增"
										variant="bordered"
										size="lg"
										searchable
										isRequired
										disabled={creating || !createDraft.l1}
										value={
											createDraft.l2.kind === "existing"
												? createDraft.l2.id
												: undefined
										}
										items={createL2Items}
										errorMessage={createFieldErrors.l2}
										onChange={handleCreateL2}
									/>
								)}
							</div>

							<div className="grid min-w-0 content-start gap-2">
								{createDraft.l3.kind === "new" ? (
									<>
										<Input
											id={`${createFormId}-l3-new`}
											label="L3 新細項名稱"
											variant="bordered"
											size="lg"
											isRequired
											autoFocus={createDraft.l2.kind !== "new"}
											disabled={creating}
											maxLength={100}
											value={createDraft.l3.name}
											description={`${createDraft.l3.name.trim().length} / 100 字`}
											errorMessage={createFieldErrors.l3}
											onChange={(event) => {
												setCreateDraft((current) =>
													updateDraftCategoryName(
														current,
														"l3",
														event.target.value,
													),
												);
												clearCreateFeedback("l3");
											}}
										/>
										{createDraft.l2.kind === "new" ? (
											<small className="text-xs text-[var(--hkui-foreground-muted)]">
												新子分類需同時建立一個細項。
											</small>
										) : (
											<Button
												type="button"
												variant="light"
												size="sm"
												className="min-h-11 justify-self-start"
												disabled={creating}
												onClick={() => {
													setCreateDraft((current) =>
														clearDraftLevel(current, "l3"),
													);
													clearCreateFeedback("l3");
												}}
											>
												改選既有細項
											</Button>
										)}
									</>
								) : (
									<Dropdown
										id={`${createFormId}-l3`}
										label="L3 細項"
										placeholder="選擇或新增"
										variant="bordered"
										size="lg"
										searchable
										isRequired
										disabled={creating || createDraft.l2.kind !== "existing"}
										value={
											createDraft.l3.kind === "existing"
												? createDraft.l3.id
												: undefined
										}
										items={createL3Items}
										errorMessage={createFieldErrors.l3}
										onChange={handleCreateL3}
									/>
								)}
							</div>
						</div>

						<p className="visually-hidden" role="status" aria-live="polite">
							目前路徑：{categoryPathLabel(createDraft)}
						</p>
					</fieldset>

					{createNotice ? (
						<p
							className="m-0 rounded-[var(--hkui-control-radius-md)] bg-[var(--hkui-primary-50)] px-3 py-2 text-sm text-[var(--hkui-primary-700)]"
							role="status"
						>
							{createNotice}
						</p>
					) : null}
					{createError ? (
						<p
							className="m-0 rounded-[var(--hkui-control-radius-md)] border border-[var(--hkui-danger)] bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]"
							role="alert"
						>
							{createError}
						</p>
					) : null}
				</form>
			</Dialog>
		</div>
	);
}

function categoryCreateErrorMessage(cause: unknown): string {
	if (cause instanceof ApiClientError) {
		if (cause.code === 3006) {
			return "這個名稱目前無法使用，請改用其他名稱。";
		}
		if (cause.code === 3011 || cause.code === 3013) {
			return "上層分類已變更，請重新選擇上層分類後再試一次。";
		}
	}
	return "新增失敗，請確認網路連線後再試一次。";
}
