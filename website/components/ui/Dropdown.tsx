"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import {
	floatingLabelClasses,
	inputWrapperClasses,
	inputWrapperData,
} from "./inputWrapperClasses";

/**
 * Web port of the Seller App's `app_dropdown.dart` (lib/shared/widgets).
 *
 * Differences from the native `<Select>` adapter in `Input.tsx`:
 *   - Custom listbox rendered through a `createPortal` to `document.body`
 *     with `position: fixed` coordinates measured off the trigger, so the
 *     menu escapes parent `overflow` AND ancestors with `transform` /
 *     `filter` / `contain` (those create a containing block that defeats
 *     plain `position: fixed` — DatePicker hit the same trap). Rows match
 *     the App's pill treatment + olive highlight.
 *   - Optional in-menu search bar (mirrors the App's `showSearch` flag).
 *   - Keyboard navigation (↑ ↓ Home End Enter Esc) and roving focus.
 *   - Click-outside / Esc / scroll-on-window all dismiss.
 *
 * Trigger styling reuses the shared input-wrapper system (variant /
 * size / floating-label / invalid) so the dropdown sits in the same row
 * as `<Input>` without manual alignment.
 */

type DropdownVariant = "flat" | "bordered" | "faded";
type DropdownSize = "sm" | "md" | "lg";

export interface DropdownItem {
	value: string;
	label: string;
	/** Optional subtitle row, rendered in a softer tone below the label. */
	description?: string;
	disabled?: boolean;
	/** Keep utility actions such as “create new” available while searching. */
	alwaysVisible?: boolean;
	/** The entry that stands for “no value” (like a placeholder `<option>`): when it
	 * is the current value the trigger looks empty (placeholder text and style, label
	 * not floated) while the menu still marks it as selected. */
	empty?: boolean;
}

/**
 * `selected` is the entry equal to `value`; the menu marks it. `shown` is what the
 * trigger displays: the same entry, except that the `empty` entry shows nothing.
 */
export function resolveDropdownSelection(
	items: readonly DropdownItem[],
	value: string | undefined,
): { selected: DropdownItem | undefined; shown: DropdownItem | undefined } {
	const selected = items.find((it) => it.value === value);
	return { selected, shown: selected?.empty ? undefined : selected };
}

export interface DropdownProps {
	value: string | undefined;
	onChange: (value: string) => void;

	/** Either pre-built items or plain strings (used for both value + label). */
	items: ReadonlyArray<DropdownItem | string>;
	placeholder?: string;

	label?: React.ReactNode;
	description?: React.ReactNode;
	errorMessage?: React.ReactNode;
	isRequired?: boolean;
	isInvalid?: boolean;
	disabled?: boolean;

	variant?: DropdownVariant;
	size?: DropdownSize;
	floatingLabel?: boolean;

	/** Show a search input above the item list. Off by default. */
	searchable?: boolean;
	/** Empty-state copy when search produces no matches. */
	noResultsLabel?: string;

	/** When true, the menu width matches the trigger; otherwise grows to fit. */
	matchTriggerWidth?: boolean;

	className?: string;
	wrapperClassName?: string;
	id?: string;
	name?: string;
	"aria-label"?: string;
}

function useFieldId(provided?: string) {
	const generatedId = React.useId();
	return provided ?? `hkui-dropdown-${generatedId}`;
}

function normalise(items: DropdownProps["items"]): DropdownItem[] {
	return items.map((it) =>
		typeof it === "string" ? { value: it, label: it } : it,
	);
}

interface MenuPosition {
	left: number;
	top: number;
	width: number;
	maxHeight: number;
}

/** @import import { Dropdown } from '@/components/ui/Dropdown'; */
export function Dropdown({
	value,
	onChange,
	items,
	placeholder = "請選擇",
	label,
	description,
	errorMessage,
	isRequired,
	isInvalid,
	disabled,
	variant = "bordered",
	size = "md",
	floatingLabel,
	searchable = false,
	noResultsLabel = "找不到符合的選項",
	matchTriggerWidth = true,
	className = "",
	wrapperClassName = "",
	id,
	name,
	"aria-label": ariaLabel,
}: DropdownProps) {
	const fieldId = useFieldId(id);
	const triggerRef = React.useRef<HTMLButtonElement>(null);
	// The menu aligns to the WRAPPER (border box), not the inner trigger button.
	// The button is inset by the wrapper's padding + border, so measuring it
	// would make the menu narrower than the field and leave a vertical gap.
	const wrapRef = React.useRef<HTMLDivElement>(null);
	const menuRef = React.useRef<HTMLDivElement>(null);
	const searchRef = React.useRef<HTMLInputElement>(null);
	const [open, setOpen] = React.useState(false);
	const [focused, setFocused] = React.useState(false);
	const [position, setPosition] = React.useState<MenuPosition | null>(null);
	const [query, setQuery] = React.useState("");
	const [activeIdx, setActiveIdx] = React.useState(-1);
	// 滑鼠停留只是視覺，不動鍵盤游標：舊版 hover 會改 activeIdx，於是
	// aria-activedescendant 跟著跳，讀屏收到「使用者根本沒按鍵」的假宣告。
	const [hoverIdx, setHoverIdx] = React.useState(-1);
	/** type-ahead：500ms 內連打的字母累積成前綴，跳到第一個符合的選項。 */
	const typeaheadRef = React.useRef({ buffer: "", at: 0 });

	const normalised = React.useMemo(() => normalise(items), [items]);
	const { selected: selectedItem, shown: shownItem } = React.useMemo(
		() => resolveDropdownSelection(normalised, value),
		[normalised, value],
	);
	const filtered = React.useMemo(() => {
		const q = query.trim().toLowerCase();
		if (!q) return normalised;
		return normalised.filter(
			(it) =>
				it.alwaysVisible ||
				it.label.toLowerCase().includes(q) ||
				it.value.toLowerCase().includes(q) ||
				(it.description?.toLowerCase().includes(q) ?? false),
		);
	}, [normalised, query]);

	const invalid = isInvalid || Boolean(errorMessage);
	const hasContent = Boolean(shownItem);

	/* ===== Open / close ===== */

	const recomputePosition = React.useCallback(() => {
		const anchor = wrapRef.current ?? triggerRef.current;
		if (!anchor) return;
		const rect = anchor.getBoundingClientRect();
		const viewportH = window.innerHeight;
		const gap = 4;
		const edge = 16;
		const roomBelow = viewportH - rect.bottom - gap - edge;
		const roomAbove = rect.top - gap - edge;
		const openAbove = roomBelow < 160 && roomAbove > roomBelow;
		const available = openAbove ? roomAbove : roomBelow;
		const maxHeight = Math.min(360, Math.max(160, available));
		setPosition({
			left: rect.left,
			top: openAbove
				? Math.max(edge, rect.top - gap - maxHeight)
				: rect.bottom + gap,
			width: rect.width,
			maxHeight,
		});
	}, []);

	const openMenu = React.useCallback(() => {
		if (disabled || open) return;
		recomputePosition();
		setOpen(true);
		setHoverIdx(-1);
		typeaheadRef.current = { buffer: "", at: 0 };
		setActiveIdx(
			selectedItem
				? Math.max(
						0,
						normalised.findIndex((it) => it.value === selectedItem.value),
					)
				: 0,
		);
	}, [disabled, open, recomputePosition, selectedItem, normalised]);

	const closeMenu = React.useCallback((returnFocus = true) => {
		setOpen(false);
		setQuery("");
		setActiveIdx(-1);
		setHoverIdx(-1);
		typeaheadRef.current = { buffer: "", at: 0 };
		if (returnFocus) triggerRef.current?.focus();
	}, []);

	const selectAt = React.useCallback(
		(idx: number) => {
			const item = filtered[idx];
			if (!item || item.disabled) return;
			onChange(item.value);
			closeMenu();
		},
		[filtered, onChange, closeMenu],
	);

	// Close on outside click / Esc / scroll / resize while open
	React.useEffect(() => {
		if (!open) return;
		const onMouseDown = (e: MouseEvent) => {
			const t = e.target as Node;
			if (triggerRef.current?.contains(t) || menuRef.current?.contains(t)) {
				return;
			}
			closeMenu(false);
		};
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") {
				e.preventDefault();
				closeMenu();
			}
		};
		const reflow = () => recomputePosition();
		document.addEventListener("mousedown", onMouseDown);
		document.addEventListener("keydown", onKey);
		window.addEventListener("scroll", reflow, true);
		window.addEventListener("resize", reflow);
		return () => {
			document.removeEventListener("mousedown", onMouseDown);
			document.removeEventListener("keydown", onKey);
			window.removeEventListener("scroll", reflow, true);
			window.removeEventListener("resize", reflow);
		};
	}, [open, closeMenu, recomputePosition]);

	// Move keyboard focus into the popup. Searchable menus use the query field;
	// compact menus keep aria-activedescendant on the focused listbox.
	//
	// The old version waited a hard-coded 30ms — on a busy main thread the
	// listbox could still be unmounted when the timer fired, leaving focus on
	// the trigger and the arrow keys dead. A layout effect runs after the menu
	// has committed, and the rAF is only a fallback for the (portal) case where
	// the node is not queryable yet.
	React.useLayoutEffect(() => {
		if (!open) return;
		const focusTarget = () => {
			const node = searchable
				? searchRef.current
				: menuRef.current?.querySelector<HTMLElement>('[role="listbox"]');
			node?.focus({ preventScroll: true });
			return Boolean(node);
		};
		if (focusTarget()) return;
		const raf = requestAnimationFrame(focusTarget);
		return () => cancelAnimationFrame(raf);
	}, [open, searchable]);

	// Keep the highlighted row in view
	React.useEffect(() => {
		if (!open || activeIdx < 0) return;
		const el = menuRef.current?.querySelector<HTMLLIElement>(
			`[data-idx="${activeIdx}"]`,
		);
		el?.scrollIntoView({ block: "nearest" });
	}, [open, activeIdx]);

	// Re-clamp activeIdx when filter shrinks the list
	React.useEffect(() => {
		if (activeIdx >= filtered.length) {
			setActiveIdx(filtered.length > 0 ? filtered.length - 1 : -1);
		}
	}, [filtered.length, activeIdx]);

	const handleMenuKey = (e: React.KeyboardEvent) => {
		if (filtered.length === 0) {
			if (e.key === "Escape") closeMenu();
			return;
		}
		switch (e.key) {
			case "ArrowDown":
				e.preventDefault();
				setHoverIdx(-1);
				setActiveIdx((p) => (p + 1) % filtered.length);
				break;
			case "ArrowUp":
				e.preventDefault();
				setHoverIdx(-1);
				setActiveIdx((p) => (p <= 0 ? filtered.length - 1 : p - 1));
				break;
			case "Home":
				e.preventDefault();
				setHoverIdx(-1);
				setActiveIdx(0);
				break;
			case "End":
				e.preventDefault();
				setHoverIdx(-1);
				setActiveIdx(filtered.length - 1);
				break;
			case "Enter":
			case " ":
				if (
					e.key === " " &&
					(e.target as HTMLElement | null)?.tagName === "INPUT"
				) {
					break;
				}
				e.preventDefault();
				selectAt(activeIdx);
				break;
			case "Escape":
				e.preventDefault();
				closeMenu();
				break;
			default: {
				// type-ahead：非搜尋型下拉在 50 個選項下只能靠方向鍵一格一格爬。
				// 搜尋型的輸入框自己會吃字，不攔。
				if (searchable) break;
				if (e.key.length !== 1 || e.altKey || e.ctrlKey || e.metaKey) break;
				const now = Date.now();
				const state = typeaheadRef.current;
				const char = e.key.toLowerCase();
				state.buffer = now - state.at > 500 ? char : state.buffer + char;
				state.at = now;
				const match = (prefix: string) =>
					filtered.findIndex(
						(it) => !it.disabled && it.label.toLowerCase().startsWith(prefix),
					);
				let idx = match(state.buffer);
				// 累積的前綴找不到就退回「只用最後一個字」，避免打錯一個字之後
				// 整段 type-ahead 死掉直到逾時。
				if (idx < 0 && state.buffer.length > 1) {
					state.buffer = char;
					idx = match(char);
				}
				if (idx < 0) break;
				e.preventDefault();
				setHoverIdx(-1);
				setActiveIdx(idx);
				break;
			}
		}
	};

	/* ===== Trigger keyboard ===== */

	const handleTriggerKey = (e: React.KeyboardEvent<HTMLButtonElement>) => {
		if (disabled) return;
		if (open) {
			handleMenuKey(e);
			return;
		}
		switch (e.key) {
			case "ArrowDown":
			case "ArrowUp":
			case "Enter":
			case " ":
				e.preventDefault();
				openMenu();
				break;
			default:
				break;
		}
	};

	/* ===== Render ===== */

	const wrapperClasses = [
		inputWrapperClasses({
			variant,
			size,
			focused: focused || open,
			invalid,
			disabled: Boolean(disabled),
			floating: Boolean(floatingLabel && label),
			wrapperClassName,
		}),
		"hkui-dropdown__trigger-wrap cursor-pointer",
	]
		.filter(Boolean)
		.join(" ");

	return (
		<div
			data-slot="field"
			className={["flex flex-col gap-1.5 w-full", className]
				.filter(Boolean)
				.join(" ")}
		>
			{label && !floatingLabel && (
				<label
					data-slot="label"
					htmlFor={fieldId}
					className={`text-sm font-medium text-foreground${isRequired ? " after:content-['_*'] after:text-[var(--hkui-danger-text)]" : ""}`}
				>
					{label}
				</label>
			)}
			<div
				ref={wrapRef}
				{...inputWrapperData({
					variant,
					focused: focused || open,
					floating: Boolean(floatingLabel && label),
				})}
				className={wrapperClasses}
			>
				<button
					ref={triggerRef}
					id={fieldId}
					name={name}
					type="button"
					className="hkui-dropdown__trigger"
					aria-haspopup="listbox"
					aria-expanded={open}
					aria-controls={open ? `${fieldId}-listbox` : undefined}
					aria-invalid={invalid || undefined}
					aria-label={ariaLabel}
					aria-describedby={
						description || errorMessage ? `${fieldId}-desc` : undefined
					}
					data-required={isRequired || undefined}
					disabled={disabled}
					onClick={() => (open ? closeMenu(false) : openMenu())}
					onKeyDown={handleTriggerKey}
					onFocus={() => setFocused(true)}
					onBlur={() => setFocused(false)}
				>
					<span
						// `data-slot` (not the BEM class) is the hook the floating-label
						// vertical layout targets: Tailwind reads `_` inside an arbitrary
						// variant as a descendant combinator, so `.hkui-dropdown__value`
						// cannot be addressed from a utility at all.
						data-slot="dropdown-value"
						className={`hkui-dropdown__value${shownItem ? "" : " is-placeholder"}`}
					>
						{shownItem
							? shownItem.label
							: floatingLabel && label && !(focused || open)
								? ""
								: placeholder}
					</span>
					<span className="hkui-dropdown__chevron" aria-hidden="true">
						<ChevronDown />
					</span>
				</button>
				{floatingLabel && label && (
					<label
						htmlFor={fieldId}
						data-slot="floating-label"
						className={floatingLabelClasses({
							active: focused || open || hasContent,
							focused: focused || open,
							size,
							variant,
						})}
					>
						{label}
						{isRequired && (
							<span className="text-[var(--hkui-danger-text)]"> *</span>
						)}
					</label>
				)}
			</div>
			{/* Portal to <body>: the doc comment always promised this, but the
			 * menu was rendered inline inside the field, so `position: fixed`
			 * was defeated by any ancestor with transform / filter / contain
			 * (DatePicker.tsx already fixed the same bug for its popover). */}
			{open && position && typeof document !== "undefined"
				? createPortal(
						<DropdownMenu
							ref={menuRef}
							items={filtered}
							selectedValue={selectedItem?.value}
							activeIdx={activeIdx}
							hoverIdx={hoverIdx}
							onHover={setHoverIdx}
							onSelect={selectAt}
							searchable={searchable}
							query={query}
							onQueryChange={setQuery}
							searchRef={searchRef}
							position={position}
							matchTriggerWidth={matchTriggerWidth}
							onKeyDown={handleMenuKey}
							noResultsLabel={noResultsLabel}
							labelledBy={fieldId}
						/>,
						document.body,
					)
				: null}
			{invalid && errorMessage ? (
				<span
					id={`${fieldId}-desc`}
					data-slot="error"
					className="text-xs text-[var(--hkui-danger-text)] flex items-center gap-1"
				>
					{errorMessage}
				</span>
			) : description ? (
				<span
					id={`${fieldId}-desc`}
					data-slot="hint"
					className="text-xs text-[var(--hkui-field-copy)]"
				>
					{description}
				</span>
			) : null}
		</div>
	);
}

interface DropdownMenuProps {
	items: DropdownItem[];
	selectedValue: string | undefined;
	activeIdx: number;
	hoverIdx: number;
	onHover: (idx: number) => void;
	onSelect: (idx: number) => void;
	searchable: boolean;
	query: string;
	onQueryChange: (q: string) => void;
	searchRef: React.RefObject<HTMLInputElement | null>;
	position: MenuPosition;
	matchTriggerWidth: boolean;
	onKeyDown: (e: React.KeyboardEvent) => void;
	noResultsLabel: string;
	labelledBy: string;
}

const DropdownMenu = React.forwardRef<HTMLDivElement, DropdownMenuProps>(
	function DropdownMenu(
		{
			items,
			selectedValue,
			activeIdx,
			hoverIdx,
			onHover,
			onSelect,
			searchable,
			query,
			onQueryChange,
			searchRef,
			position,
			matchTriggerWidth,
			onKeyDown,
			noResultsLabel,
			labelledBy,
		},
		ref,
	) {
		const style: React.CSSProperties = {
			left: position.left,
			top: position.top,
			minWidth: matchTriggerWidth ? position.width : undefined,
			maxWidth: matchTriggerWidth ? position.width : 480,
			maxHeight: position.maxHeight,
		};

		return (
			// biome-ignore lint/a11y/noStaticElementInteractions: container forwards keys when search/listbox is focused; semantics live on inner listbox
			<div
				ref={ref}
				className="hkui-dropdown__menu"
				role="presentation"
				style={style}
				onKeyDown={onKeyDown}
			>
				{searchable && (
					<div className="hkui-dropdown__search">
						<SearchIcon />
						<input
							ref={searchRef}
							type="text"
							role="combobox"
							value={query}
							onChange={(e) => onQueryChange(e.target.value)}
							placeholder="搜尋"
							className="hkui-dropdown__search-input"
							aria-label="搜尋選項"
							aria-expanded="true"
							aria-autocomplete="list"
							aria-controls={`${labelledBy}-listbox`}
							aria-activedescendant={
								activeIdx >= 0 ? `${labelledBy}-opt-${activeIdx}` : undefined
							}
							// 02-reset-layout 的 bare-control baseline 會給沒有 data-slot
							// 的 input 補品牌邊框＋focus ring — 在藥丸搜尋框裡變成內外雙框。
							data-slot="control"
						/>
					</div>
				)}
				{items.length === 0 ? (
					<div className="hkui-dropdown__empty">{noResultsLabel}</div>
				) : (
					<DropdownListbox
						labelledBy={labelledBy}
						items={items}
						activeIdx={activeIdx}
						hoverIdx={hoverIdx}
						selectedValue={selectedValue}
						onHover={onHover}
						onSelect={onSelect}
					/>
				)}
			</div>
		);
	},
);

interface DropdownListboxProps {
	labelledBy: string;
	items: DropdownItem[];
	activeIdx: number;
	hoverIdx: number;
	selectedValue: string | undefined;
	onHover: (idx: number) => void;
	onSelect: (idx: number) => void;
}

function DropdownListbox({
	labelledBy,
	items,
	activeIdx,
	hoverIdx,
	selectedValue,
	onHover,
	onSelect,
}: DropdownListboxProps) {
	return (
		<div
			id={`${labelledBy}-listbox`}
			className="hkui-dropdown__list"
			role="listbox"
			tabIndex={-1}
			aria-labelledby={labelledBy}
			aria-activedescendant={
				activeIdx >= 0 ? `${labelledBy}-opt-${activeIdx}` : undefined
			}
		>
			{items.map((item, idx) => (
				<DropdownOption
					key={item.value}
					idx={idx}
					labelledBy={labelledBy}
					item={item}
					isSelected={item.value === selectedValue}
					isActive={idx === activeIdx}
					isHovered={idx === hoverIdx}
					onHover={onHover}
					onSelect={onSelect}
				/>
			))}
		</div>
	);
}

interface DropdownOptionProps {
	idx: number;
	labelledBy: string;
	item: DropdownItem;
	isSelected: boolean;
	isActive: boolean;
	isHovered: boolean;
	onHover: (idx: number) => void;
	onSelect: (idx: number) => void;
}

function DropdownOption({
	idx,
	labelledBy,
	item,
	isSelected,
	isActive,
	isHovered,
	onHover,
	onSelect,
}: DropdownOptionProps) {
	const cls = [
		"hkui-dropdown__item",
		isSelected ? "is-selected" : "",
		isActive ? "is-active" : "",
		isHovered ? "is-hover" : "",
		item.disabled ? "is-disabled" : "",
	]
		.filter(Boolean)
		.join(" ");
	return (
		// biome-ignore lint/a11y/useKeyWithClickEvents: keyboard handled centrally on the menu container
		<div
			id={`${labelledBy}-opt-${idx}`}
			data-idx={idx}
			role="option"
			aria-selected={isSelected}
			aria-disabled={item.disabled || undefined}
			tabIndex={-1}
			className={cls}
			onMouseEnter={() => !item.disabled && onHover(idx)}
			onMouseLeave={() => !item.disabled && onHover(-1)}
			onMouseDown={(e) => {
				// prevent the trigger from blurring before click fires
				e.preventDefault();
			}}
			onClick={() => !item.disabled && onSelect(idx)}
		>
			<span className="hkui-dropdown__item-label">{item.label}</span>
			{item.description && (
				<span className="hkui-dropdown__item-desc">{item.description}</span>
			)}
			{isSelected && (
				<span className="hkui-dropdown__item-check" aria-hidden="true">
					<CheckIcon />
				</span>
			)}
		</div>
	);
}

function ChevronDown() {
	return (
		<svg
			width="16"
			height="16"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<title>向下展開</title>
			<path d="m6 9 6 6 6-6" />
		</svg>
	);
}

function CheckIcon() {
	return (
		<svg
			width="14"
			height="14"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.4"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<title>已選</title>
			<path d="M20 6 9 17l-5-5" />
		</svg>
	);
}

function SearchIcon() {
	return (
		<svg
			width="16"
			height="16"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<title>搜尋</title>
			<circle cx="11" cy="11" r="7" />
			<path d="m20 20-3.5-3.5" />
		</svg>
	);
}

export default Dropdown;
