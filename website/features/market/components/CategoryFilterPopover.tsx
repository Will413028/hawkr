"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { type CategoryNode, findCategoryNode } from "@/lib/market/listFilters";
import { updateUrlParams } from "@/lib/navigation/url-params";

interface CategoryFilterPopoverProps {
	tree: CategoryNode[];
	/** Canonical category ids from SSR, sourced from `?categories=…` (csv). */
	selected: string[];
}

const CARET_DOWN = (
	<svg
		width="11"
		height="11"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2.4"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="m6 9 6 6 6-6" />
	</svg>
);

const CARET_RIGHT = (
	<svg
		width="9"
		height="9"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2.4"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="m9 6 6 6-6 6" />
	</svg>
);

const CHECK = (
	<svg
		width="11"
		height="11"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="3.2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="m5 12 5 5 9-11" />
	</svg>
);

function childrenOf(node: CategoryNode | undefined): CategoryNode[] {
	if (!node || !Array.isArray(node.children)) return [];
	return node.children.filter((c) => Boolean(c?.name));
}

function tokenOf(node: CategoryNode): string {
	return node.id != null ? String(node.id) : String(node.name ?? "");
}

export default function CategoryFilterPopover({
	tree,
	selected,
}: CategoryFilterPopoverProps) {
	// The route is ISR: writing `?categories` via history.pushState re-reads the
	// URL on the client (useUrlSearchParams) and re-filters in place — no server
	// round trip, no useSearchParams bailout. Legacy single-value `category` and
	// the stable-id csv `categories` are updated together.
	const selectedKey = selected.join("\u001f");

	const [open, setOpen] = React.useState(false);
	const [renderPanel, setRenderPanel] = React.useState(false);
	const [isApplying, setIsApplying] = React.useState(false);
	const [announcement, setAnnouncement] = React.useState("");
	const [activeL1, setActiveL1] = React.useState<string>(tree[0]?.name ?? "");
	const [checked, setChecked] = React.useState<Set<string>>(
		() => new Set(selected),
	);
	const [expandedL2, setExpandedL2] = React.useState<Set<string>>(new Set());
	/** Trigger-anchored coordinates for the portalled panel. Recomputed
	 * on open + on scroll/resize while open. Null on first render before
	 * the trigger has mounted (portal must wait for client-side mount). */
	const [coords, setCoords] = React.useState<{
		top?: number;
		bottom?: number;
		left: number;
		maxHeight: number;
		placement: "bottom" | "top";
	} | null>(null);
	const [mounted, setMounted] = React.useState(false);

	const panelRef = React.useRef<HTMLDivElement>(null);
	const triggerRef = React.useRef<HTMLButtonElement>(null);
	const closeTimerRef = React.useRef<number | null>(null);
	const applySequenceRef = React.useRef(0);
	const mobileScrollReleaseRef = React.useRef<(() => void) | null>(null);
	const releaseMobileScrollLock = React.useCallback(() => {
		mobileScrollReleaseRef.current?.();
		mobileScrollReleaseRef.current = null;
	}, []);
	const closePopover = React.useCallback(
		(restoreFocus = false) => {
			setOpen(false);
			releaseMobileScrollLock();
			if (closeTimerRef.current != null) {
				window.clearTimeout(closeTimerRef.current);
			}
			const closeDelay = window.matchMedia("(prefers-reduced-motion: reduce)")
				.matches
				? 0
				: 180;
			closeTimerRef.current = window.setTimeout(() => {
				setRenderPanel(false);
				setCoords(null);
				closeTimerRef.current = null;
				if (restoreFocus) triggerRef.current?.focus();
			}, closeDelay);
		},
		[releaseMobileScrollLock],
	);

	React.useEffect(() => {
		setMounted(true);
		return () => {
			if (closeTimerRef.current != null) {
				window.clearTimeout(closeTimerRef.current);
			}
		};
	}, []);

	const updateCoords = React.useCallback(() => {
		if (!triggerRef.current) return;
		if (window.matchMedia("(max-width: 767px)").matches) {
			setCoords(null);
			return;
		}
		const rect = triggerRef.current.getBoundingClientRect();
		const gutter = 16;
		const gap = 8;
		const panelWidth = Math.min(520, window.innerWidth - gutter * 2);
		const left = Math.min(
			Math.max(rect.left, gutter),
			Math.max(gutter, window.innerWidth - panelWidth - gutter),
		);
		const spaceBelow = window.innerHeight - rect.bottom - gutter - gap;
		const spaceAbove = rect.top - gutter - gap;
		const placement =
			spaceBelow >= 320 || spaceBelow >= spaceAbove ? "bottom" : "top";
		const available = placement === "bottom" ? spaceBelow : spaceAbove;
		const maxUsableHeight = Math.max(220, window.innerHeight - gutter * 2);
		const maxHeight = Math.min(560, maxUsableHeight, Math.max(220, available));

		setCoords(
			placement === "bottom"
				? {
						top: rect.bottom + gap,
						left,
						maxHeight,
						placement,
					}
				: {
						bottom: window.innerHeight - rect.top + gap,
						left,
						maxHeight,
						placement,
					},
		);
	}, []);

	const openPopover = React.useCallback(() => {
		if (closeTimerRef.current != null) {
			window.clearTimeout(closeTimerRef.current);
			closeTimerRef.current = null;
		}
		updateCoords();
		setRenderPanel(true);
		setOpen(true);
	}, [updateCoords]);

	// Recompute the panel anchor whenever the trigger could have moved
	// (scroll / resize). Mobile CSS pins to bottom so the coordinates
	// become a no-op there. We skip scrolls that originate inside the
	// panel itself — the L2 list and chips strip both scroll internally,
	// and re-running getBoundingClientRect on every keystroke of scroll
	// caused the panel to thrash on re-render.
	React.useEffect(() => {
		if (!open) return;
		updateCoords();
		const onScroll = (e: Event) => {
			const target = e.target as Node | null;
			if (target && panelRef.current?.contains(target)) return;
			updateCoords();
		};
		window.addEventListener("scroll", onScroll, true);
		window.addEventListener("resize", updateCoords);
		return () => {
			window.removeEventListener("scroll", onScroll, true);
			window.removeEventListener("resize", updateCoords);
		};
	}, [open, updateCoords]);

	React.useEffect(() => {
		if (!open || !mounted) return;
		const media = window.matchMedia("(max-width: 767px)");
		const syncBodyLock = () => {
			if (media.matches && !mobileScrollReleaseRef.current) {
				mobileScrollReleaseRef.current = acquireBodyScrollLock();
			} else if (!media.matches) {
				releaseMobileScrollLock();
			}
		};
		syncBodyLock();
		media.addEventListener("change", syncBodyLock);
		return () => {
			media.removeEventListener("change", syncBodyLock);
			releaseMobileScrollLock();
		};
	}, [mounted, open, releaseMobileScrollLock]);

	// Re-sync local state if the URL changes underneath us (e.g. user removes
	// a chip from the applied-filters strip while the popover is closed). While
	// open, the local selection intentionally stays ahead of streamed SSR props.
	React.useEffect(() => {
		if (open || isApplying) return;
		const nextSelected =
			selectedKey.length > 0 ? selectedKey.split("\u001f") : [];
		setChecked(new Set(nextSelected));
	}, [isApplying, open, selectedKey]);

	// Click-outside + Escape close the chooser. Checkbox changes already apply.
	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") closePopover(true);
		};
		const onClick = (e: MouseEvent) => {
			const target = e.target as Node | null;
			if (!target) return;
			if (panelRef.current?.contains(target)) return;
			if (triggerRef.current?.contains(target)) return;
			closePopover();
		};
		document.addEventListener("keydown", onKey);
		document.addEventListener("mousedown", onClick);
		return () => {
			document.removeEventListener("keydown", onKey);
			document.removeEventListener("mousedown", onClick);
		};
	}, [closePopover, open]);

	const activeL1Node = React.useMemo(
		() => tree.find((n) => n?.name === activeL1) ?? tree[0],
		[tree, activeL1],
	);
	const l2List = childrenOf(activeL1Node);

	const commitSelection = React.useCallback(
		async (next: Set<string>) => {
			const sequence = ++applySequenceRef.current;
			setIsApplying(true);
			setAnnouncement("正在更新商品");
			try {
				updateUrlParams({
					// Every new write removes the legacy name-only value.
					category: null,
					categories: next.size === 0 ? null : Array.from(next).join(","),
				});
				if (sequence === applySequenceRef.current) {
					setAnnouncement(
						next.size === 0 ? "已清除分類篩選" : "分類篩選已更新",
					);
				}
			} catch {
				if (sequence === applySequenceRef.current) {
					const lastConfirmed =
						selectedKey.length > 0 ? selectedKey.split("\u001f") : [];
					setChecked(new Set(lastConfirmed));
					setAnnouncement("分類更新失敗，請再試一次");
				}
			} finally {
				if (sequence === applySequenceRef.current) setIsApplying(false);
			}
		},
		[selectedKey],
	);
	const toggle = (token: string) => {
		const next = new Set(checked);
		if (next.has(token)) next.delete(token);
		else next.add(token);
		setChecked(next);
		void commitSelection(next);
	};
	const toggleL2Expand = (name: string) => {
		setExpandedL2((prev) => {
			const next = new Set(prev);
			if (next.has(name)) next.delete(name);
			else next.add(name);
			return next;
		});
	};
	const clear = () => {
		const next = new Set<string>();
		setChecked(next);
		void commitSelection(next);
	};

	// Map L1 name -> count of selected descendants (any depth), used for
	// the badge on the master column entries.
	const countsByL1 = React.useMemo(() => {
		const out = new Map<string, number>();
		for (const l1 of tree) {
			const l1Name = l1?.name;
			if (!l1Name) continue;
			let count = 0;
			if (checked.has(tokenOf(l1))) count += 1;
			for (const l2 of childrenOf(l1)) {
				if (checked.has(tokenOf(l2))) count += 1;
				for (const l3 of childrenOf(l2)) {
					if (checked.has(tokenOf(l3))) count += 1;
				}
			}
			out.set(l1Name, count);
		}
		return out;
	}, [tree, checked]);

	const totalSelected = checked.size;
	const panelStyle = coords
		? ({
				"--market-category-popover-left": `${coords.left}px`,
				"--market-category-popover-max-height": `${coords.maxHeight}px`,
				...(coords.top == null
					? {}
					: { "--market-category-popover-top": `${coords.top}px` }),
				...(coords.bottom == null
					? {}
					: { "--market-category-popover-bottom": `${coords.bottom}px` }),
			} as React.CSSProperties)
		: undefined;

	return (
		<div className="market-category-popover">
			<button
				ref={triggerRef}
				type="button"
				className="market-category-popover__trigger"
				aria-haspopup="dialog"
				aria-expanded={open}
				aria-busy={isApplying}
				aria-label={
					totalSelected > 0 ? `分類，已選 ${totalSelected} 項` : "分類"
				}
				onClick={() => {
					if (open) closePopover();
					else openPopover();
				}}
			>
				<span className="market-category-popover__trigger-label">
					分類
					{totalSelected > 0 ? (
						<span className="market-category-popover__trigger-count">
							{totalSelected}
						</span>
					) : null}
				</span>
				<span className="market-category-popover__trigger-caret">
					{CARET_DOWN}
				</span>
			</button>
			<span className="visually-hidden" aria-live="polite">
				{announcement}
			</span>

			{renderPanel && mounted
				? createPortal(
						<>
							<div
								className="market-category-popover__scrim"
								data-state={open ? "open" : "closed"}
								aria-hidden="true"
							/>
							<div
								ref={panelRef}
								className="market-category-popover__panel"
								role="dialog"
								aria-label="分類篩選"
								aria-hidden={!open}
								inert={!open || undefined}
								data-state={open ? "open" : "closed"}
								data-placement={coords?.placement ?? "bottom"}
								style={panelStyle}
							>
								<header className="market-category-popover__sheet-head">
									<strong>分類</strong>
									<button
										type="button"
										className="market-category-popover__sheet-close"
										onClick={() => closePopover(true)}
										aria-label="關閉分類篩選"
									>
										×
									</button>
								</header>
								<div className="market-category-popover__body">
									<nav
										className="market-category-popover__l1"
										aria-label="一級分類"
									>
										{tree
											.filter((l1) => Boolean(l1?.name))
											.map((l1) => {
												const name = String(l1.name);
												const count = countsByL1.get(name) ?? 0;
												return (
													<button
														type="button"
														key={name}
														className="market-category-popover__l1-item"
														data-active={name === activeL1}
														onClick={() => setActiveL1(name)}
													>
														<span>{name}</span>
														{count > 0 ? (
															<span className="market-category-popover__l1-count">
																{count}
															</span>
														) : null}
													</button>
												);
											})}
									</nav>

									{/* biome-ignore lint/a11y/useSemanticElements: <fieldset>
									    breaks `overflow-y: auto` inside the grid cell — its
									    legacy intrinsic sizing lets content push past the
									    grid track, leaving the panel showing the empty
									    background below the L2 list. */}
									<div
										className="market-category-popover__l2"
										role="group"
										aria-label={`${activeL1} 的子分類`}
									>
										{l2List.length === 0 ? (
											<p className="market-category-popover__empty">
												{activeL1} 沒有子分類
											</p>
										) : (
											<ul className="market-category-popover__l2-list">
												{l2List.map((l2) => {
													const l2Name = String(l2.name);
													const l2Token = tokenOf(l2);
													const l3List = childrenOf(l2);
													const isChecked = checked.has(l2Token);
													const isExpanded = expandedL2.has(l2Name);
													const l3SelectedCount = l3List.filter((l3) =>
														checked.has(tokenOf(l3)),
													).length;
													return (
														<li
															key={l2Name}
															className="market-category-popover__l2-item"
														>
															<div className="market-category-popover__l2-row">
																<label className="market-category-popover__checkbox">
																	<input
																		type="checkbox"
																		checked={isChecked}
																		onChange={() => {
																			const wasChecked = isChecked;
																			toggle(l2Token);
																			// Mirror L3 expansion to the L2 check state:
																			// checking expands the sub-options, unchecking
																			// collapses them — a single click drives both.
																			if (l3List.length > 0) {
																				setExpandedL2((prev) => {
																					const next = new Set(prev);
																					if (wasChecked) next.delete(l2Name);
																					else next.add(l2Name);
																					return next;
																				});
																			}
																		}}
																	/>
																	<span
																		className="market-category-popover__checkbox-box"
																		aria-hidden="true"
																	>
																		{isChecked ? CHECK : null}
																	</span>
																	<span className="market-category-popover__checkbox-label">
																		{l2Name}
																	</span>
																</label>
																{l3List.length > 0 ? (
																	<button
																		type="button"
																		className="market-category-popover__l2-expand"
																		data-expanded={isExpanded}
																		onClick={() => toggleL2Expand(l2Name)}
																		aria-expanded={isExpanded}
																		aria-label={`${
																			isExpanded ? "收合" : "展開"
																		} ${l2Name} 子項${
																			l3SelectedCount > 0
																				? `（已選 ${l3SelectedCount}）`
																				: ""
																		}`}
																	>
																		{l3SelectedCount > 0 ? (
																			<span className="market-category-popover__l3-badge">
																				{l3SelectedCount}
																			</span>
																		) : null}
																		{CARET_RIGHT}
																	</button>
																) : null}
															</div>
															{isExpanded && l3List.length > 0 ? (
																<ul className="market-category-popover__l3-list">
																	{l3List.map((l3) => {
																		const l3Name = String(l3.name);
																		const l3Token = tokenOf(l3);
																		const l3Checked = checked.has(l3Token);
																		return (
																			<li key={l3Name}>
																				<label className="market-category-popover__checkbox market-category-popover__checkbox--sub">
																					<input
																						type="checkbox"
																						checked={l3Checked}
																						onChange={() => toggle(l3Token)}
																					/>
																					<span
																						className="market-category-popover__checkbox-box"
																						aria-hidden="true"
																					>
																						{l3Checked ? CHECK : null}
																					</span>
																					<span className="market-category-popover__checkbox-label">
																						{l3Name}
																					</span>
																				</label>
																			</li>
																		);
																	})}
																</ul>
															) : null}
														</li>
													);
												})}
											</ul>
										)}
									</div>
								</div>

								<footer className="market-category-popover__foot">
									<button
										type="button"
										className="market-category-popover__clear"
										onClick={clear}
										disabled={totalSelected === 0}
									>
										清除
									</button>
									<span
										className="market-category-popover__count"
										aria-live="polite"
									>
										{isApplying ? "正在更新商品…" : `已選 ${totalSelected} 項`}
									</span>
									{totalSelected > 0 ? (
										<ul
											className="market-category-popover__chips"
											aria-label="已選分類"
										>
											{Array.from(checked).map((token) => {
												const name =
													findCategoryNode(tree, token)?.name ?? token;
												return (
													<li key={token}>
														<button
															type="button"
															className="market-category-popover__chip"
															onClick={() => toggle(token)}
															aria-label={`移除 ${name}`}
														>
															<span className="market-category-popover__chip-label">
																{name}
															</span>
															<span
																className="market-category-popover__chip-x"
																aria-hidden="true"
															>
																×
															</span>
														</button>
													</li>
												);
											})}
										</ul>
									) : (
										<span
											className="market-category-popover__chips-spacer"
											aria-hidden="true"
										/>
									)}
									<button
										type="button"
										className="market-category-popover__apply"
										onClick={() => closePopover(true)}
									>
										完成
									</button>
								</footer>
							</div>
						</>,
						document.body,
					)
				: null}
		</div>
	);
}
