"use client";

import type EditorJS from "@editorjs/editorjs";
import * as React from "react";
import { createPortal } from "react-dom";
import { renderToStaticMarkup } from "react-dom/server";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import AppToaster from "@/components/shared/AppToaster";
import { DescriptionColorPicker } from "@/features/seller/components/DescriptionColorPicker";
import type { DescriptionImageData } from "@/features/seller/components/DescriptionImageTool";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { toast } from "@/lib/feedback/toast";
import type { MediaUploadRepo } from "@/lib/media/upload";
import {
	clampGrid,
	parseClipboardGrid,
	TABLE_MAX_COLS,
	TABLE_MAX_ROWS,
} from "@/lib/products/clipboardGrid";
import {
	collectSelectedEditorBlockIds,
	collectSelectedEditorBlockIndexes,
	DESCRIPTION_HISTORY_ACTION_LIMIT,
	DESCRIPTION_HISTORY_TYPING_WINDOW_MS,
	type DescriptionHistoryInputBatch,
	descriptionHistoryDirectionForInputType,
	descriptionHistoryInputBatch,
	editorBlocksToListItems,
	isAlignableEditorBlockType,
	isListConvertibleEditorBlock,
	limitDescriptionHistory,
	tableCellHtmlWithListMarker,
} from "@/lib/products/descriptionEditorActions";
import {
	cleanupDescriptionEditorData,
	parseDescriptionMarkdownPaste,
} from "@/lib/products/descriptionEditorCleanup";
import {
	fromEditorData,
	TABLE_CELL_ALIGN_ATTRIBUTE,
	toEditorData,
	toHexColor,
} from "@/lib/products/descriptionEditorJs";
import { descriptionEditorMobileToolbarTop } from "@/lib/products/descriptionEditorLayout";
import type { ProductDescription } from "@/lib/products/seller-repo";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { retainOverlayOpenAttribute } from "@/lib/state/overlay-open-attribute";
import styles from "@/styles/features/seller/DescriptionEditor.module.css";

interface Props {
	value: ProductDescription;
	onChange: (next: ProductDescription) => void;
}

/**
 * Notion-style block editor for product descriptions, built on Editor.js.
 * Sellers type inline, press "/" (or use the top 樣式工具列) to insert a
 * paragraph / heading / image, and drag the ↕ handle to reorder (the ⋮⋮ button
 * is click-to-adjust only).
 *
 * Editor.js owns its own (uncontrolled) DOM, so we bridge it to React's
 * controlled `value` / `onChange` via {@link fromEditorData} / {@link
 * toEditorData}, which keep the persisted shape identical to the mobile app's
 * `{ version, blocks }` format. The `selfUpdate` ref distinguishes edits the
 * user typed here (skip re-render) from external mutations such as an AI patch
 * (re-render the editor with the new value).
 *
 * UI is localised to 繁中 via Editor.js's `i18n` config; the top toolbar drives
 * the same APIs the inline toolbar / slash menu use (block convert + native
 * bold/italic/underline shortcuts).
 */

/** 繁體中文 localisation for the Editor.js chrome (toolbar, tunes, tools). */
const I18N = {
	messages: {
		ui: {
			blockTunes: {
				toggler: {
					"Click to tune": "點此調整",
					"or drag to move": "或拖曳搬移",
				},
			},
			inlineToolbar: { converter: { "Convert to": "內容轉換" } },
			toolbar: {
				toolbox: { Add: "新增", Filter: "搜尋區塊", "Nothing found": "找不到" },
			},
			popover: {
				Filter: "搜尋區塊",
				"Nothing found": "找不到",
				"Convert to": "內容轉換",
			},
		},
		toolNames: {
			Text: "內文",
			Heading: "標題",
			List: "清單",
			Table: "表格",
			Image: "圖片",
			Bold: "粗體",
			Italic: "斜體",
			Underline: "底線",
		},
		tools: {
			header: { "Heading 2": "大標", "Heading 3": "小標" },
			table: {
				Heading: "標題",
				"With headings": "有標題列",
				"Without headings": "無標題列",
				Stretch: "延伸寬度",
				Collapse: "收回寬度",
				"Add column to left": "左側插入欄",
				"Add column to right": "右側插入欄",
				"Delete column": "刪除這一欄",
				"Add row above": "上方插入列",
				"Add row below": "下方插入列",
				"Delete row": "刪除這一列",
			},
			image: {
				"Select an Image": "選擇圖片",
				Caption: "圖片說明",
				"With border": "加邊框",
				"Stretch image": "延伸寬度",
				"With background": "加背景",
			},
		},
		blockTunes: {
			delete: { Delete: "刪除", "Click to delete": "點此刪除" },
			moveUp: { "Move up": "上移" },
			moveDown: { "Move down": "下移" },
		},
	},
} as const;

/**
 * Inline tools kept registered so their save-sanitizers preserve underline /
 * colour / link applied from the top toolbar. The floating inline
 * (text-selection) toolbar is styled minimal in CSS, but this list governs
 * which inline markup survives `editor.save()`. Since the backend adopted the
 * inline-run model (02 §3.6), `<a href>` now round-trips as a run `href`, so
 * Editor.js's built-in `link` is included. Bold/italic/link are core tools;
 * underline/textColor are registered in `tools` below.
 */
const INLINE_TOOLS = ["bold", "italic", "underline", "textColor", "link"];

/**
 * Minimal inline tool whose only real job is to whitelist colour markup in the
 * paragraph sanitizer, so the `<span style="color:…">` produced by the top
 * toolbar's `execCommand("foreColor")` survives `editor.save()`. It also shows
 * a brand-green "A" in the inline toolbar as a convenience. The converter
 * extracts the colour back into `block.style.color` on save.
 */
class TextColorInlineTool {
	private openPicker: (range: Range) => void;
	constructor({ config }: { config: { onOpen: (range: Range) => void } }) {
		this.openPicker = config.onOpen;
	}
	static get isInline() {
		return true;
	}
	static get title() {
		return "顏色";
	}
	static get sanitize() {
		return { span: { style: true }, font: { color: true } };
	}
	render() {
		const btn = document.createElement("button");
		btn.type = "button";
		btn.classList.add("ce-inline-tool");
		btn.setAttribute("aria-label", "開啟文字色盤");
		btn.innerHTML =
			"<span style='font-weight:700;font-size:15px;text-decoration:underline'>A</span>";
		return btn;
	}
	surround(range: Range) {
		this.openPicker(range);
	}
	checkState() {
		return false;
	}
}

type Alignment = "left" | "center" | "right";

const ALIGN_OPTIONS: ReadonlyArray<{
	value: Alignment;
	label: string;
	icon: AppIconName;
}> = [
	{ value: "left", label: "置左", icon: "align-left" },
	{ value: "center", label: "置中", icon: "align-center" },
	{ value: "right", label: "置右", icon: "align-right" },
];

function alignIconMarkup(icon: AppIconName): string {
	return renderToStaticMarkup(<AppIcon name={icon} size={18} />);
}

/**
 * Block tune that carries paragraph / heading alignment. The saved
 * `{ alignment }` payload is bridged to `block.style.textAlign` by
 * {@link fromEditorData} ("left" is the default and is dropped there). The
 * top toolbar drives it via `editor.blocks.update(id, undefined, { align })`,
 * which re-renders the block so {@link AlignBlockTune.wrap} re-applies the
 * style; the same options also appear in the block's ⋮⋮ tune menu.
 */
class AlignBlockTune {
	static get isTune() {
		return true;
	}
	private alignment: Alignment;
	private wrapper: HTMLElement | null = null;
	private block: { dispatchChange?: () => void };

	constructor({
		data,
		block,
	}: {
		data?: { alignment?: unknown };
		block: { dispatchChange?: () => void };
	}) {
		const a = data?.alignment;
		this.alignment = a === "center" || a === "right" ? a : "left";
		this.block = block;
	}

	render() {
		return ALIGN_OPTIONS.map((opt) => ({
			icon: alignIconMarkup(opt.icon),
			title: opt.label,
			toggle: "align",
			isActive: this.alignment === opt.value,
			closeOnActivate: true,
			onActivate: () => {
				this.alignment = opt.value;
				this.applyStyle();
				this.block.dispatchChange?.();
			},
		}));
	}

	wrap(blockContent: HTMLElement) {
		this.wrapper = blockContent;
		this.applyStyle();
		return blockContent;
	}

	save() {
		return { alignment: this.alignment };
	}

	private applyStyle() {
		if (!this.wrapper) return;
		this.wrapper.style.textAlign =
			this.alignment === "left" ? "" : this.alignment;
	}
}

const IMAGE_PLUS_ICON_MARKUP = renderToStaticMarkup(
	<AppIcon name="image" size={24} />,
);
const EDITOR_JS_MOBILE_MEDIA = "(max-width: 649px)";

/** Two opposing vertical arrows keep the reorder handle visually and
 *  semantically distinct from the ⋮⋮ block-settings button. */
const DRAG_HANDLE_ICON_MARKUP = renderToStaticMarkup(
	<AppIcon name="reorder" size={20} />,
);

/**
 * Y-axis "magnetic" block reordering off a dedicated ↕ handle.
 *
 * Replaces the `editorjs-drag-drop` plugin, which resolved the drop target from
 * `e.target.closest(".ce-block")` — whatever sat *under the cursor* — so a drag
 * whose X drifted into the gutter found no block and silently did nothing. Here
 * the landing slot is computed purely from the pointer's Y against each block's
 * bounds, so the entire editor width is a valid drop zone and the drop snaps to
 * the nearest vertical gap. A full-width insertion line tracks where the block
 * will land; the source block dims while it is in flight.
 *
 * Drag lives on its own ↕ handle (injected next to ⋮⋮), so "drag to reorder" and
 * "click ⋮⋮ to adjust" never share one control. Reorders go through Editor.js's
 * own `blocks.move`, so the existing `onChange` bridge persists the new order.
 */
function attachBlockDragReorder(
	editor: EditorJS,
	holder: HTMLElement,
): () => void {
	// Our own ↕ handle. The +/⋮⋮ controls own the left gutter; this sits in the
	// RIGHT gutter of the block content — it, not ⋮⋮, is what the user grabs.
	const handle = document.createElement("button");
	handle.type = "button";
	handle.className = "desc-editor-drag-handle";
	handle.setAttribute("draggable", "true");
	handle.setAttribute("title", "拖曳排序");
	handle.setAttribute("aria-label", "區塊排序（方向鍵上下移動）");
	handle.innerHTML = DRAG_HANDLE_ICON_MARKUP;

	// Editor.js builds its toolbar lazily (first block hover via `moveAndOpen`)
	// and may rebuild it, so `.ce-toolbar__content` often doesn't exist yet at
	// `isReady`. Watch the holder and (re)mount the handle into the toolbar
	// content (centred to the block, position:relative) as soon as it appears;
	// CSS floats it into the right gutter. The `isConnected` guard makes it a
	// no-op once placed.
	const ensureHandle = () => {
		for (const scrollArea of holder.querySelectorAll<HTMLElement>(
			".ce-popover__items",
		)) {
			scrollArea.tabIndex = 0;
		}
		if (handle.isConnected) return;
		const content = holder.querySelector<HTMLElement>(".ce-toolbar__content");
		content?.appendChild(handle);
	};
	ensureHandle();
	const observer = new MutationObserver(ensureHandle);
	observer.observe(holder, { childList: true, subtree: true });

	let sourceIndex = -1;
	let targetIndex = -1;
	let draggedEl: HTMLElement | null = null;
	let line: HTMLDivElement | null = null;

	const redactor = () =>
		holder.querySelector<HTMLElement>(".codex-editor__redactor");
	const blockEls = () => {
		const r = redactor();
		return r
			? Array.from(r.querySelectorAll<HTMLElement>(":scope > .ce-block"))
			: [];
	};
	// The toolbar (and thus our handle) is anchored to the hovered block, but
	// pressing a non-⋮⋮ element doesn't update Editor.js's "current block", so the
	// source is resolved from the handle's own Y against the block bounds.
	const blockIndexAtY = (y: number, blocks: HTMLElement[]) => {
		let best = -1;
		let bestDist = Number.POSITIVE_INFINITY;
		blocks.forEach((b, i) => {
			const r = b.getBoundingClientRect();
			const dist = y < r.top ? r.top - y : y > r.bottom ? y - r.bottom : 0;
			if (dist < bestDist) {
				bestDist = dist;
				best = i;
			}
		});
		return best;
	};

	const onDragStart = (e: DragEvent) => {
		if (
			!(e.target as HTMLElement | null)?.closest?.(".desc-editor-drag-handle")
		)
			return;
		const blocks = blockEls();
		const hr = handle.getBoundingClientRect();
		sourceIndex = blockIndexAtY(hr.top + hr.height / 2, blocks);
		if (sourceIndex < 0) return;
		draggedEl = blocks[sourceIndex] ?? null;
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = "move";
			e.dataTransfer.setData("text/plain", ""); // Firefox needs a payload
		}
		// Let the browser snapshot the drag image first, then dim the live block.
		const el = draggedEl;
		requestAnimationFrame(() => el?.classList.add("desc-block--dragging"));
	};

	const onDragOver = (e: DragEvent) => {
		if (sourceIndex < 0) return;
		e.preventDefault(); // mark the whole editor box as a drop zone
		if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
		const blocks = blockEls();
		// Insert-before slot = first block whose vertical midpoint sits below the
		// cursor; falls through to "after the last block".
		let insertBefore = blocks.length;
		for (let i = 0; i < blocks.length; i++) {
			const rect = blocks[i].getBoundingClientRect();
			if (e.clientY < rect.top + rect.height / 2) {
				insertBefore = i;
				break;
			}
		}
		targetIndex = insertBefore;

		const r = redactor();
		if (!r) return;
		if (!line) {
			line = document.createElement("div");
			line.className = "desc-editor-drop-line";
			line.setAttribute("aria-hidden", "true");
		}
		if (line.parentNode !== r) r.appendChild(line);
		const rTop = r.getBoundingClientRect().top;
		const ref = blocks[insertBefore];
		const last = blocks[blocks.length - 1];
		const y = ref
			? ref.getBoundingClientRect().top - rTop
			: last
				? last.getBoundingClientRect().bottom - rTop
				: 0;
		line.style.top = `${y}px`;
		line.style.display = "block";
	};

	const reset = () => {
		draggedEl?.classList.remove("desc-block--dragging");
		if (line) line.style.display = "none";
		sourceIndex = -1;
		targetIndex = -1;
		draggedEl = null;
	};

	const onDrop = (e: DragEvent) => {
		if (sourceIndex < 0) {
			reset();
			return;
		}
		e.preventDefault();
		// `targetIndex` is an insert-*before* slot; once the source is lifted out,
		// every later index shifts down by one.
		let to = targetIndex > sourceIndex ? targetIndex - 1 : targetIndex;
		to = Math.max(0, Math.min(to, editor.blocks.getBlocksCount() - 1));
		if (to !== sourceIndex) editor.blocks.move(to, sourceIndex);
		reset();
	};

	const onHandleKey = (event: KeyboardEvent) => {
		if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
		event.preventDefault();
		event.stopPropagation();
		// The handle follows the hovered block, which may differ from the caret.
		const handleRect = handle.getBoundingClientRect();
		const current = blockIndexAtY(
			handleRect.top + handleRect.height / 2,
			blockEls(),
		);
		const next = current + (event.key === "ArrowUp" ? -1 : 1);
		if (current < 0 || next < 0 || next >= editor.blocks.getBlocksCount())
			return;
		editor.blocks.move(next, current);
		editor.caret.setToBlock(next, "end");
		editor.toolbar.open();
		handle.focus({ preventScroll: true });
	};
	const onHandleClick = () =>
		holder.querySelector<HTMLElement>(".ce-toolbar__settings-btn")?.click();
	handle.addEventListener("keydown", onHandleKey);
	handle.addEventListener("click", onHandleClick);
	holder.addEventListener("dragstart", onDragStart);
	holder.addEventListener("dragover", onDragOver);
	holder.addEventListener("drop", onDrop);
	holder.addEventListener("dragend", reset);

	return () => {
		observer.disconnect();
		holder.removeEventListener("dragstart", onDragStart);
		holder.removeEventListener("dragover", onDragOver);
		holder.removeEventListener("drop", onDrop);
		holder.removeEventListener("dragend", reset);
		handle.removeEventListener("keydown", onHandleKey);
		handle.removeEventListener("click", onHandleClick);
		handle.remove();
		line?.remove();
	};
}

/**
 * Editor.js deliberately places its toolbar below the active block whenever
 * `window.innerWidth < 650`. Our product chrome uses that toolbar as a left
 * block rail, so on phones the + / ⋮⋮ controls otherwise appear detached from
 * the selected block. Keep the upstream package untouched and adapt only its
 * mobile position to the first editable line (or the top of a media block).
 */
function attachMobileBlockToolbarAlignment(
	editor: EditorJS,
	holder: HTMLElement,
): () => void {
	const mobileViewport = window.matchMedia(EDITOR_JS_MOBILE_MEDIA);
	let toolbar: HTMLElement | null = null;
	let redactor: HTMLElement | null = null;
	let frame = 0;

	const sync = () => {
		frame = 0;
		if (
			!mobileViewport.matches ||
			!toolbar?.classList.contains("ce-toolbar--opened")
		)
			return;

		const blockIndex = editor.blocks.getCurrentBlockIndex();
		const block =
			blockIndex >= 0
				? editor.blocks.getBlockByIndex(blockIndex)?.holder
				: null;
		const offsetParent = toolbar.offsetParent as HTMLElement | null;
		if (!block || !offsetParent) return;

		// Prefer the first actual writing surface. Filled images have an editable
		// caption at the bottom, so their image frame intentionally wins instead.
		const anchor =
			block.querySelector<HTMLElement>(
				'.image-tool__image, .tc-cell[contenteditable="true"], .tc-cell [contenteditable="true"], .cdx-list [contenteditable="true"], .ce-paragraph[contenteditable="true"], .ce-header[contenteditable="true"], [contenteditable="true"]',
			) ??
			block.querySelector<HTMLElement>(".ce-block__content") ??
			block;
		const actions = toolbar.querySelector<HTMLElement>(".ce-toolbar__actions");
		const actionHeight = actions?.getBoundingClientRect().height || 32;
		const anchorRect = anchor.getBoundingClientRect();
		const parentRect = offsetParent.getBoundingClientRect();
		const anchorStyle = window.getComputedStyle(anchor);
		const paddingTop = Number.parseFloat(anchorStyle.paddingTop) || 0;
		const parsedLineHeight = Number.parseFloat(anchorStyle.lineHeight);
		const nextTop = descriptionEditorMobileToolbarTop({
			anchorTop: anchorRect.top,
			anchorHeight: anchorRect.height,
			parentTop: parentRect.top,
			parentClientTop: offsetParent.clientTop,
			paddingTop,
			isEditable: anchor.matches('[contenteditable="true"]'),
			lineHeight: parsedLineHeight,
			actionHeight,
		});
		const nextTopCss = `${nextTop}px`;

		if (toolbar.style.top !== nextTopCss) toolbar.style.top = nextTopCss;
		toolbar.dataset.mobileBlockIndex = String(blockIndex);
	};

	const scheduleSync = () => {
		if (frame) return;
		frame = window.requestAnimationFrame(sync);
	};

	const toolbarObserver = new MutationObserver(scheduleSync);
	const layoutObserver =
		typeof ResizeObserver === "undefined"
			? null
			: new ResizeObserver(scheduleSync);

	const connectEditorDom = () => {
		const nextToolbar = holder.querySelector<HTMLElement>(".ce-toolbar");
		if (nextToolbar !== toolbar) {
			toolbarObserver.disconnect();
			toolbar = nextToolbar;
			if (toolbar) {
				toolbarObserver.observe(toolbar, {
					attributes: true,
					attributeFilter: ["class", "style"],
				});
			}
		}

		const nextRedactor = holder.querySelector<HTMLElement>(
			".codex-editor__redactor",
		);
		if (nextRedactor !== redactor) {
			if (redactor) layoutObserver?.unobserve(redactor);
			redactor = nextRedactor;
			if (redactor) layoutObserver?.observe(redactor);
		}
		scheduleSync();
	};

	const structureObserver = new MutationObserver(connectEditorDom);
	structureObserver.observe(holder, { childList: true, subtree: true });
	layoutObserver?.observe(holder);
	connectEditorDom();

	const onViewportModeChange = () => {
		if (!mobileViewport.matches) {
			if (frame) window.cancelAnimationFrame(frame);
			frame = 0;
			if (toolbar) delete toolbar.dataset.mobileBlockIndex;
			// Reset Editor.js's inline mobile coordinate before desktop positioning
			// takes over on the next hover/focus.
			editor.toolbar.close();
			return;
		}
		scheduleSync();
	};
	const visualViewport = window.visualViewport;

	mobileViewport.addEventListener("change", onViewportModeChange);
	window.addEventListener("resize", scheduleSync, { passive: true });
	visualViewport?.addEventListener("resize", scheduleSync, { passive: true });
	holder.addEventListener("focusin", scheduleSync, true);
	holder.addEventListener("pointerdown", scheduleSync, true);

	return () => {
		if (frame) window.cancelAnimationFrame(frame);
		structureObserver.disconnect();
		toolbarObserver.disconnect();
		layoutObserver?.disconnect();
		mobileViewport.removeEventListener("change", onViewportModeChange);
		window.removeEventListener("resize", scheduleSync);
		visualViewport?.removeEventListener("resize", scheduleSync);
		holder.removeEventListener("focusin", scheduleSync, true);
		holder.removeEventListener("pointerdown", scheduleSync, true);
		if (toolbar) delete toolbar.dataset.mobileBlockIndex;
	};
}

/**
 * Undo/redo 專用：把快照以「區塊級 diff」套回編輯器，不走整篇
 * editor.render()。整篇 render 會拆光 DOM 重建 — 每次還原整個畫布閃一下
 * 而且慢；undo/redo 幾乎都只動一個區塊，這裡只 update / insert / delete /
 * move 有差異的區塊，其餘 DOM 原封不動。`prev` 是跳轉前的快照，用來判斷
 * 哪些同 id 區塊需要 update。結構對不上就 throw，由呼叫端 fallback 整篇。
 */
async function safeSaveEditor(editor: EditorJS) {
	const data = await editor.save();
	// Editor.js catches saver errors internally and may resolve undefined.
	// Never turn that failure into a valid empty description.
	if (!data || !Array.isArray(data.blocks))
		throw new Error("Editor could not save the current content");
	return data;
}

async function applySnapshotToEditor(
	editor: EditorJS,
	prev: ProductDescription,
	snapshot: ProductDescription,
): Promise<void> {
	const target = toEditorData(snapshot).blocks;
	const prevById = new Map(
		toEditorData(prev)
			.blocks.filter((b) => b.id)
			.map((b) => [b.id as string, b]),
	);

	const count = editor.blocks.getBlocksCount();
	const current: { id: string; name: string }[] = [];
	for (let i = 0; i < count; i++) {
		const b = editor.blocks.getBlockByIndex(i);
		if (!b) throw new Error("missing block");
		current.push({ id: b.id, name: b.name });
	}

	let firstChanged = -1;
	const markChanged = (i: number) => {
		if (firstChanged === -1 || i < firstChanged) firstChanged = i;
	};

	// Insert/move the target first, then trim leftovers. Deleting the last live
	// block first makes Editor.js insert an untracked placeholder of its own.
	for (let j = 0; j < target.length; j++) {
		const t = target[j];
		const at = t.id ? current.findIndex((c) => c.id === t.id) : -1;
		if (!t.id || at === -1) {
			const [inserted] = editor.blocks.insertMany([t], j);
			if (!inserted) throw new Error("Could not restore block");
			// Editor.js insertMany discards tunes; restore them explicitly.
			if (t.tunes) await editor.blocks.update(inserted.id, undefined, t.tunes);
			current.splice(j, 0, { id: inserted.id, name: t.type });
			markChanged(j);
			continue;
		}
		if (at !== j) {
			editor.blocks.move(j, at);
			const [moved] = current.splice(at, 1);
			current.splice(j, 0, moved);
			markChanged(j);
		}
		if (current[j].name !== t.type) {
			// Replace the tool first, then restore tunes omitted by insertMany.
			const [inserted] = editor.blocks.insertMany([t], j);
			if (!inserted) throw new Error("Could not restore block type");
			editor.blocks.delete(j + 1);
			if (t.tunes) await editor.blocks.update(inserted.id, undefined, t.tunes);
			current[j] = { id: inserted.id, name: t.type };
			markChanged(j);
			continue;
		}
		// 3) 內容 / 對齊 tune 跟 prev 快照有差才 update（re-render 單一區塊）
		const p = prevById.get(t.id);
		const changed =
			!p ||
			JSON.stringify(p.data) !== JSON.stringify(t.data) ||
			JSON.stringify(p.tunes) !== JSON.stringify(t.tunes);
		if (changed) {
			// target 沒帶 tunes 但 prev 有（置中/置右 → 置左）時要明確送回
			// 預設值 — update 的 tunes 參數缺省就完全不會動到既有 tune。
			const tunes =
				t.tunes ?? (p?.tunes ? { align: { alignment: "left" } } : undefined);
			await editor.blocks.update(t.id, t.data, tunes);
			markChanged(j);
		}
	}

	for (let index = current.length - 1; index >= target.length; index--) {
		editor.blocks.delete(index);
		markChanged(Math.max(0, target.length - 1));
	}

	// 游標放回第一個變動區塊的尾端，還原後接續編輯不用重新點位。
	if (firstChanged !== -1) {
		const total = editor.blocks.getBlocksCount();
		try {
			editor.caret.setToBlock(Math.min(firstChanged, total - 1), "end");
		} catch {
			/* image 等不可聚焦區塊 — 略過 */
		}
	}
}

/**
 * 試算表語意的儲存格貼上（#1246 對齊 Notion）：把格陣從錨點格開始逐格填入
 * `@editorjs/table` 的 DOM，列/欄不足時觸發 tool 內建的加列/加欄控制
 * （走 tool 自己的 click 管線，內部狀態不脫鉤；tool 的 save() 讀 DOM，資料一致）。
 * 回傳是否因表格上限（錨點偏移超出 10 欄 × 50 列）而截掉部分資料。
 */
function expandGridIntoTable(
	anchorCell: HTMLElement,
	grid: string[][],
): boolean {
	const wrap = anchorCell.closest(".tc-wrap") as HTMLElement | null;
	const table = anchorCell.closest(".tc-table") as HTMLElement | null;
	const rowEl = anchorCell.closest(".tc-row") as HTMLElement | null;
	if (!wrap || !table || !rowEl) return false;
	const rows = () =>
		Array.from(table.querySelectorAll(":scope > .tc-row")) as HTMLElement[];
	const cellsOf = (r: HTMLElement) =>
		Array.from(r.querySelectorAll(":scope > .tc-cell")) as HTMLElement[];
	const anchorRow = rows().indexOf(rowEl);
	const anchorCol = cellsOf(rowEl).indexOf(anchorCell);
	if (anchorRow < 0 || anchorCol < 0) return false;

	const gridCols = Math.max(...grid.map((r) => r.length));
	const wantRows = anchorRow + grid.length;
	const wantCols = anchorCol + gridCols;
	const neededRows = Math.min(wantRows, TABLE_MAX_ROWS);
	const neededCols = Math.min(wantCols, TABLE_MAX_COLS);
	const truncated = wantRows > neededRows || wantCols > neededCols;

	const addRowBtn = wrap.querySelector(
		":scope > .tc-add-row",
	) as HTMLElement | null;
	const addColBtn = wrap.querySelector(
		":scope > .tc-add-column",
	) as HTMLElement | null;
	let guard = 0;
	while (rows().length < neededRows && addRowBtn && guard++ < TABLE_MAX_ROWS) {
		addRowBtn.click();
	}
	guard = 0;
	while (
		cellsOf(rows()[0]).length < neededCols &&
		addColBtn &&
		guard++ < TABLE_MAX_COLS
	) {
		addColBtn.click();
	}
	// 擴增未達位（唯讀模式無按鈕 / tool 內建上限擋下）也算截切，讓 caller 提示
	const shortRows = rows().length < neededRows;
	const shortCols = cellsOf(rows()[0]).length < neededCols;

	const allRows = rows();
	grid.forEach((rowData, i) => {
		const target = allRows[anchorRow + i];
		if (!target) return;
		const cells = cellsOf(target);
		rowData.forEach((cellHtml, j) => {
			const cell = cells[anchorCol + j];
			if (!cell) return;
			cell.innerHTML = cellHtml;
			cell.dispatchEvent(new InputEvent("input", { bubbles: true }));
		});
	});
	return truncated || shortRows || shortCols;
}

function activeSelectionRange(holder: HTMLElement): Range | null {
	const selection = window.getSelection();
	if (!selection || selection.rangeCount === 0 || selection.isCollapsed)
		return null;
	const range = selection.getRangeAt(0);
	const start =
		range.startContainer.nodeType === Node.ELEMENT_NODE
			? (range.startContainer as Element)
			: range.startContainer.parentElement;
	const end =
		range.endContainer.nodeType === Node.ELEMENT_NODE
			? (range.endContainer as Element)
			: range.endContainer.parentElement;
	return start && end && holder.contains(start) && holder.contains(end)
		? range
		: null;
}

interface CrossBlockDeletion {
	ids: string[];
	prefix: string;
	suffix: string;
	prefixLength: number;
	startListItems: string[] | null;
	endListItems: string[] | null;
	tableCells?: { address: ToolbarTableCellAddress; html: string }[];
}

/** Independent contenteditables do not merge a native range reliably. Capture
 * its two unselected ends before any asynchronous save/re-render can move it. */
function captureCrossBlockDeletion(
	holder: HTMLElement,
	range: Range,
): CrossBlockDeletion | "unsupported" | null {
	const elementAt = (node: Node) =>
		node.nodeType === Node.ELEMENT_NODE
			? (node as Element)
			: node.parentElement;
	const startElement = elementAt(range.startContainer);
	const endElement = elementAt(range.endContainer);
	const startBlock = startElement?.closest<HTMLElement>(".ce-block");
	const endBlock = endElement?.closest<HTMLElement>(".ce-block");
	if (!startBlock || !endBlock) return null;
	if (startBlock === endBlock) {
		const startCell = startElement?.closest<HTMLElement>(".tc-cell");
		const endCell = endElement?.closest<HTMLElement>(".tc-cell");
		if (!startCell || !endCell || startCell === endCell) return null;
		const cells = Array.from(
			startBlock.querySelectorAll<HTMLElement>(".tc-cell"),
		);
		const first = cells.indexOf(startCell);
		const last = cells.indexOf(endCell);
		if (first < 0 || last <= first || !startBlock.dataset.id)
			return "unsupported";
		let prefixLength = 0;
		const tableCells: NonNullable<CrossBlockDeletion["tableCells"]> = [];
		for (let index = first; index <= last; index++) {
			const address = tableCellAddress(cells[index]);
			if (!address) return "unsupported";
			let html = "";
			if (index === first || index === last) {
				const part = range.cloneRange();
				part.selectNodeContents(tableCellContentRoot(cells[index]));
				if (index === first) {
					part.setEnd(range.startContainer, range.startOffset);
					prefixLength = part.toString().length;
				} else part.setStart(range.endContainer, range.endOffset);
				const wrapper = document.createElement("div");
				wrapper.append(part.cloneContents());
				html = wrapper.innerHTML;
			}
			tableCells.push({ address, html });
		}
		return {
			ids: [startBlock.dataset.id],
			prefix: "",
			suffix: "",
			prefixLength,
			startListItems: null,
			endListItems: null,
			tableCells,
		};
	}
	const selector = ".ce-paragraph, .ce-header, .cdx-list__item";
	const startRoot = startElement?.closest<HTMLElement>(selector);
	const endRoot = endElement?.closest<HTMLElement>(selector);
	if (!startRoot || !endRoot) return "unsupported";
	const blocks = Array.from(
		holder.querySelectorAll<HTMLElement>(".codex-editor__redactor > .ce-block"),
	);
	const first = blocks.indexOf(startBlock);
	const last = blocks.indexOf(endBlock);
	if (first < 0 || last <= first) return "unsupported";
	const ids = blocks
		.slice(first, last + 1)
		.map((block) => block.dataset.id ?? "");
	if (ids.some((id) => !id)) return "unsupported";
	const prefixRange = range.cloneRange();
	prefixRange.selectNodeContents(startRoot);
	prefixRange.setEnd(range.startContainer, range.startOffset);
	const suffixRange = range.cloneRange();
	suffixRange.selectNodeContents(endRoot);
	suffixRange.setStart(range.endContainer, range.endOffset);
	const html = (part: Range) => {
		const wrapper = document.createElement("div");
		wrapper.append(part.cloneContents());
		return wrapper.innerHTML;
	};
	const listItems = (root: HTMLElement, before: boolean) => {
		if (!root.matches(".cdx-list__item")) return null;
		const items = Array.from(root.parentElement?.children ?? []);
		const index = items.indexOf(root);
		return (before ? items.slice(0, index) : items.slice(index + 1)).map(
			(item) => item.innerHTML,
		);
	};
	return {
		ids,
		prefix: html(prefixRange),
		suffix: html(suffixRange),
		prefixLength: prefixRange.toString().length,
		startListItems: listItems(startRoot, true),
		endListItems: listItems(endRoot, false),
	};
}

function placeCaretAtTextOffset(root: HTMLElement, offset: number): void {
	root
		.closest<HTMLElement>("[contenteditable=true]")
		?.focus({ preventScroll: true });
	const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
	let node = walker.nextNode();
	while (node && offset > (node.textContent?.length ?? 0)) {
		offset -= node.textContent?.length ?? 0;
		node = walker.nextNode();
	}
	const caret = document.createRange();
	if (node) caret.setStart(node, offset);
	else {
		caret.selectNodeContents(root);
		caret.collapse(false);
	}
	caret.collapse(true);
	const selection = window.getSelection();
	selection?.removeAllRanges();
	selection?.addRange(caret);
}

function rangeIntersects(range: Range, node: Node): boolean {
	try {
		return range.intersectsNode(node);
	} catch {
		return false;
	}
}

function selectedBlockIndexes(
	holder: HTMLElement,
	range: Range | null,
): number[] {
	const blocks = holder.querySelectorAll<HTMLElement>(
		".codex-editor__redactor > .ce-block",
	);
	return collectSelectedEditorBlockIndexes(
		Array.from(blocks).map((block) => ({
			isEditorSelected: block.classList.contains("ce-block--selected"),
			intersectsNativeRange: range ? rangeIntersects(range, block) : false,
		})),
	);
}

function selectedBlockIds(holder: HTMLElement, range: Range | null): string[] {
	const blocks = holder.querySelectorAll<HTMLElement>(
		".codex-editor__redactor > .ce-block",
	);
	return collectSelectedEditorBlockIds(
		Array.from(blocks).map((block) => ({
			id: block.dataset.id ?? "",
			isEditorSelected: block.classList.contains("ce-block--selected"),
			intersectsNativeRange: range ? rangeIntersects(range, block) : false,
		})),
	);
}

function selectedTableCells(
	holder: HTMLElement,
	range: Range | null,
): HTMLElement[] {
	const cells = Array.from(holder.querySelectorAll<HTMLElement>(".tc-cell"));
	const selected = cells.filter(
		(cell) =>
			cell.classList.contains("tc-cell--selected") ||
			cell.parentElement?.classList.contains("tc-row--selected") ||
			(range ? rangeIntersects(range, cell) : false),
	);
	if (selected.length > 0) return selected;

	const focused = document.activeElement as HTMLElement | null;
	const activeCell = focused?.closest?.(".tc-cell") as HTMLElement | null;
	return activeCell && holder.contains(activeCell) ? [activeCell] : [];
}

function tableCellAddress(cell: HTMLElement): ToolbarTableCellAddress | null {
	const block = cell.closest<HTMLElement>(".ce-block");
	const row = cell.closest<HTMLElement>(".tc-row");
	const table = row?.parentElement;
	const blockId = block?.dataset.id ?? "";
	if (!blockId || !row || !table) return null;
	const rows = Array.from(
		table.querySelectorAll<HTMLElement>(":scope > .tc-row"),
	);
	const cells = Array.from(
		row.querySelectorAll<HTMLElement>(":scope > .tc-cell"),
	);
	const rowIndex = rows.indexOf(row);
	const columnIndex = cells.indexOf(cell);
	return rowIndex >= 0 && columnIndex >= 0
		? { blockId, row: rowIndex, column: columnIndex }
		: null;
}

function resolveTableCells(
	holder: HTMLElement,
	addresses: ReadonlyArray<ToolbarTableCellAddress>,
): HTMLElement[] {
	const blocks = Array.from(
		holder.querySelectorAll<HTMLElement>(".codex-editor__redactor > .ce-block"),
	);
	return addresses
		.map(({ blockId, row, column }) => {
			const block = blocks.find(
				(candidate) => candidate.dataset.id === blockId,
			);
			const rowElement = block?.querySelectorAll<HTMLElement>(".tc-row")[row];
			return (
				rowElement?.querySelectorAll<HTMLElement>(".tc-cell")[column] ?? null
			);
		})
		.filter((cell): cell is HTMLElement => cell !== null);
}

function toolbarSelectionSnapshot(
	holder: HTMLElement,
): ToolbarSelectionSnapshot | null {
	const range = activeSelectionRange(holder);
	const blockIds = selectedBlockIds(holder, range);
	const tableCells = selectedTableCells(holder, range)
		.map(tableCellAddress)
		.filter((cell): cell is ToolbarTableCellAddress => cell !== null);
	return blockIds.length > 0 || tableCells.length > 0
		? { blockIds, tableCells, capturedAt: Date.now() }
		: null;
}

function tableCellContentRoot(cell: HTMLElement): HTMLElement {
	const first = cell.firstElementChild as HTMLElement | null;
	return first &&
		cell.children.length === 1 &&
		first.hasAttribute(TABLE_CELL_ALIGN_ATTRIBUTE)
		? first
		: cell;
}

function applyAlignmentToTableCells(
	cells: ReadonlyArray<HTMLElement>,
	alignment: Alignment,
): void {
	for (const cell of cells) {
		const contentRoot = tableCellContentRoot(cell);
		const html = contentRoot.innerHTML;
		if (alignment === "left") {
			if (contentRoot !== cell) cell.innerHTML = html;
		} else if (contentRoot === cell) {
			const wrapper = document.createElement("div");
			wrapper.setAttribute(TABLE_CELL_ALIGN_ATTRIBUTE, alignment);
			wrapper.style.textAlign = alignment;
			wrapper.innerHTML = html;
			cell.replaceChildren(wrapper);
		} else {
			contentRoot.setAttribute(TABLE_CELL_ALIGN_ATTRIBUTE, alignment);
			contentRoot.style.textAlign = alignment;
		}
		cell.dispatchEvent(new InputEvent("input", { bubbles: true }));
	}
}

/** Table cells cannot contain an Editor.js list block. For a multi-cell
 * selection, use visible list markers inside each cell instead. */
function applyListMarkersToSelectedCells(
	holder: HTMLElement,
	range: Range | null,
	ordered: boolean,
): boolean {
	const cells = selectedTableCells(holder, range);
	if (cells.length === 0) return false;

	cells.forEach((cell, index) => {
		const contentRoot = tableCellContentRoot(cell);
		contentRoot.innerHTML = tableCellHtmlWithListMarker(
			contentRoot.innerHTML,
			ordered,
			index,
		);
		cell.dispatchEvent(new InputEvent("input", { bubbles: true }));
	});
	return true;
}

function clipboardImageFiles(data: DataTransfer | null): File[] {
	if (!data) return [];
	const candidates = [
		...Array.from(data.items)
			.filter((item) => item.kind === "file" && item.type.startsWith("image/"))
			.map((item) => item.getAsFile()),
		...Array.from(data.files),
	].filter((file): file is File => !!file && file.type.startsWith("image/"));
	const seen = new Set<string>();
	return candidates.filter((file) => {
		const key = `${file.name}:${file.size}:${file.lastModified}:${file.type}`;
		if (seen.has(key)) return false;
		seen.add(key);
		return true;
	});
}

interface ToolbarSelectionSnapshot {
	blockIds: string[];
	tableCells: ToolbarTableCellAddress[];
	capturedAt: number;
}

interface ToolbarTableCellAddress {
	blockId: string;
	row: number;
	column: number;
}

export default function DescriptionEditor(props: Props) {
	const scope = useSellerDataScope();
	return (
		<DescriptionEditorCanvas
			{...props}
			uploadImageFile={scope.repos.media.uploadImageFile}
		/>
	);
}

/** Same production editor with an explicit upload boundary for isolated previews. */
export function DescriptionEditorCanvas({
	value,
	onChange,
	uploadImageFile,
}: Props & {
	uploadImageFile: MediaUploadRepo["uploadImageFile"];
}) {
	const uploadImageFileRef = React.useRef(uploadImageFile);
	uploadImageFileRef.current = uploadImageFile;
	const holderRef = React.useRef<HTMLDivElement>(null);
	const editorRef = React.useRef<EditorJS | null>(null);
	const toasterId = `description-editor-${React.useId()}`;
	// 放大編輯 dialog（原生 <dialog>）。宣告在最上面，undo/redo 的
	// ⌘Z keydown effect（下面）跟展開狀態的 effect（檔案後段）都要用。
	const dialogRef = React.useRef<HTMLDialogElement>(null);
	const selfUpdate = React.useRef(false);
	const lastSig = React.useRef<string>(JSON.stringify(value));
	// editor.render() 是 async 的（清空再逐塊重建）——外部變動（undo/redo
	// 連點、AI patch）觸發的 render 必須串成一條 chain 逐一跑，兩發重疊
	// 會清掉一半又插一半，內容直接壞掉。
	const renderChainRef = React.useRef<Promise<unknown>>(Promise.resolve());
	// Toolbar commands can update several Editor.js blocks in succession. Suppress
	// their intermediate onChange saves and publish one canonical snapshot after
	// the whole command finishes, so undo treats the command as one action.
	const editorMutationBatchRef = React.useRef(false);
	const saveGenerationRef = React.useRef(0);
	const pendingHistoryRef = React.useRef(0);
	const pendingDeleteRef = React.useRef(0);
	const beforeDeleteCheckpointRef = React.useRef<Promise<void>>(
		Promise.resolve(),
	);
	const toolbarSelectionRef = React.useRef<ToolbarSelectionSnapshot | null>(
		null,
	);
	const [ready, setReady] = React.useState(false);
	const [loadError, setLoadError] = React.useState(false);
	const [busy, setBusy] = React.useState(false);
	const [preview, setPreview] = React.useState(false);
	const previewRef = React.useRef(false);
	const inlineRangeRef = React.useRef<Range | null>(null);
	const [format, setFormat] = React.useState<EditorFormatState>({
		block: "paragraph",
		bold: false,
		italic: false,
		underline: false,
		color: "#42493f",
		alignment: "left",
		list: null,
	});
	const rememberInlineSelection = React.useCallback(() => {
		const holder = holderRef.current;
		const selection = window.getSelection();
		// Browsers may leave a collapsed editor Range behind while focus moves
		// into a toolbar/picker input. Keep the last real editor selection then.
		if (!holder?.contains(document.activeElement) || !selection?.rangeCount)
			return;
		const range = selection.getRangeAt(0);
		if (
			!holder.contains(range.startContainer) ||
			!holder.contains(range.endContainer)
		)
			return;
		inlineRangeRef.current = range.cloneRange();
		const node = selection.anchorNode;
		const element =
			node?.nodeType === Node.ELEMENT_NODE
				? (node as HTMLElement)
				: node?.parentElement;
		const block = element?.closest<HTMLElement>(".ce-block");
		if (!block) return;
		const editable = element?.closest<HTMLElement>("[contenteditable]");
		const heading = block.querySelector(".ce-header");
		const list = block.querySelector("ol.cdx-list, ul.cdx-list");
		const alignment = getComputedStyle(editable ?? block).textAlign;
		const next: EditorFormatState = {
			block:
				heading?.tagName === "H2"
					? "header2"
					: heading?.tagName === "H3"
						? "header3"
						: "paragraph",
			bold: document.queryCommandState("bold"),
			italic: document.queryCommandState("italic"),
			underline: document.queryCommandState("underline"),
			color:
				toHexColor(getComputedStyle(element ?? editable ?? block).color) ??
				"#42493f",
			alignment:
				alignment === "center" || alignment === "right" ? alignment : "left",
			list: list?.tagName === "OL" ? "ordered" : list ? "unordered" : null,
		};
		setFormat((previous) =>
			JSON.stringify(previous) === JSON.stringify(next) ? previous : next,
		);
	}, []);
	const restoreInlineSelection = () => {
		const holder = holderRef.current;
		const range = inlineRangeRef.current;
		if (
			!holder ||
			!range ||
			!holder.contains(range.startContainer) ||
			!holder.contains(range.endContainer)
		)
			return false;
		const selection = window.getSelection();
		if (!selection) return false;
		const element =
			range.startContainer.nodeType === Node.ELEMENT_NODE
				? (range.startContainer as HTMLElement)
				: range.startContainer.parentElement;
		element
			?.closest<HTMLElement>("[contenteditable]")
			?.focus({ preventScroll: true });
		selection.removeAllRanges();
		selection.addRange(range);
		return true;
	};

	const onChangeRef = React.useRef(onChange);
	onChangeRef.current = onChange;
	const initialRef = React.useRef(value);
	const forceNextHistoryCheckpointRef = React.useRef(false);
	const lastHistoryInputRef = React.useRef<DescriptionHistoryInputBatch | null>(
		null,
	);
	const markHistoryActionBoundary = React.useCallback(() => {
		forceNextHistoryCheckpointRef.current = true;
		lastHistoryInputRef.current = null;
	}, []);

	/* ---- undo/redo：對 `value`（controlled ProductDescription）做快照。
	 *      跳轉時同步呼叫 onChange 更新表單 state，並用 applySnapshotToEditor
	 *      對編輯器做「區塊級」套用（即時、不閃爍）——跳轉前先把 lastSig 寫成
	 *      快照 sig，套用過程的 editor onChange echo 與父層 value echo 都會被
	 *      sig 擋掉，不會變成新編輯。合併規則接近 Word：同一區塊的連續字元
	 *      合併；空白/標點後的新字、輸入與刪除切換、換段、貼上、格式操作、
	 *      跨區塊或停頓都會開新一步。 ---- */
	// Keep the initial snapshot plus the latest 20 user actions, so the seller
	// can always step back through exactly 20 edits without unbounded memory use.
	const historyRef = React.useRef<{
		stack: ProductDescription[];
		index: number;
	}>({ stack: [initialRef.current], index: 0 });
	const lastCheckpointAtRef = React.useRef(0);
	const [historyState, setHistoryState] = React.useState({
		undoCount: 0,
		redoCount: 0,
	});
	// Stable (empty-deps) callbacks — they only close over refs and the
	// setState setter, both stable across renders, so it's safe to also
	// call these from the Cmd+Z/Cmd+Shift+Z listener below without
	// re-subscribing that effect on every render.
	const syncHistoryFlags = React.useCallback(() => {
		const { stack, index } = historyRef.current;
		setHistoryState({
			undoCount: index,
			redoCount: stack.length - index - 1,
		});
	}, []);
	const checkpointHistory = React.useCallback(
		(next: ProductDescription) => {
			// 同一個區塊的連續文字編輯才能併成一步：區塊數/順序一樣，且內容
			// 有差的區塊最多一個。跨區塊或結構變動一律開新 checkpoint。
			const isSameSingleBlockEdit = (
				a: ProductDescription,
				b: ProductDescription,
			) => {
				if (a.blocks.length !== b.blocks.length) return false;
				let diffs = 0;
				for (let i = 0; i < a.blocks.length; i++) {
					if (a.blocks[i].id !== b.blocks[i].id) return false;
					if (JSON.stringify(a.blocks[i]) !== JSON.stringify(b.blocks[i])) {
						diffs += 1;
						if (diffs > 1) return false;
					}
				}
				return true;
			};
			const now = Date.now();
			const { stack, index } = historyRef.current;
			// 內容跟目前所在的快照一樣就不佔步驟 —— Editor.js 初始 render 後
			// 會補發一次 onChange（存檔輸出是 canonical 形，跟 mount 時的
			// lastSig 不同、擋不掉），沒有這條會在堆疊裡多出一格「按了沒
			// 反應」的重複初始狀態。
			if (JSON.stringify(stack[index]) === JSON.stringify(next)) return;
			const forceNewAction = forceNextHistoryCheckpointRef.current;
			forceNextHistoryCheckpointRef.current = false;
			if (
				!forceNewAction &&
				index === stack.length - 1 &&
				now - lastCheckpointAtRef.current <
					DESCRIPTION_HISTORY_TYPING_WINDOW_MS &&
				isSameSingleBlockEdit(stack[index], next)
			) {
				stack[index] = next;
			} else {
				const truncated = stack.slice(0, index + 1);
				truncated.push(next);
				const limited = limitDescriptionHistory(truncated);
				historyRef.current = { stack: limited, index: limited.length - 1 };
			}
			lastCheckpointAtRef.current = now;
			syncHistoryFlags();
		},
		[syncHistoryFlags],
	);
	const commitEditorValue = React.useCallback(
		(next: ProductDescription) => {
			const sig = JSON.stringify(next);
			if (sig === lastSig.current) return;
			saveGenerationRef.current++;
			lastSig.current = sig;
			selfUpdate.current = true;
			onChangeRef.current(next);
			checkpointHistory(next);
		},
		[checkpointHistory],
	);
	const jumpHistory = React.useCallback(
		(direction: -1 | 1) => {
			const editor = editorRef.current;
			if (!editor || previewRef.current) return;
			if (
				editorMutationBatchRef.current &&
				pendingHistoryRef.current === 0 &&
				pendingDeleteRef.current === 0
			)
				return;
			pendingHistoryRef.current++;
			saveGenerationRef.current++;
			editorMutationBatchRef.current = true;
			setBusy(true);
			// Flush live text before choosing the history entry: Editor.js batches
			// onChange, so Delete followed immediately by Undo may not be saved yet.
			renderChainRef.current = renderChainRef.current
				.then(async () => {
					await editor.isReady;
					await beforeDeleteCheckpointRef.current;
					if (editorRef.current !== editor) return;
					checkpointHistory(fromEditorData(await safeSaveEditor(editor)));
					const { stack, index } = historyRef.current;
					const nextIndex = index + direction;
					if (nextIndex < 0 || nextIndex >= stack.length) return;
					const prev = stack[index];
					const snapshot = stack[nextIndex];
					historyRef.current.index = nextIndex;
					lastCheckpointAtRef.current = 0;
					markHistoryActionBoundary();
					lastSig.current = JSON.stringify(snapshot);
					try {
						await applySnapshotToEditor(editor, prev, snapshot);
					} catch {
						await editor.render(toEditorData(snapshot));
					}
					inlineRangeRef.current = null;
					toolbarSelectionRef.current = null;
					selfUpdate.current = true;
					onChangeRef.current(snapshot);
					syncHistoryFlags();
				})
				.catch(() => {
					if (editorRef.current === editor)
						toast.error("復原未完成，請再試一次", { toasterId });
				})
				.finally(() => {
					pendingHistoryRef.current--;
					if (!pendingHistoryRef.current && !pendingDeleteRef.current) {
						editorMutationBatchRef.current = false;
						setBusy(false);
					}
				});
		},
		[checkpointHistory, markHistoryActionBoundary, syncHistoryFlags, toasterId],
	);
	const undo = React.useCallback(() => jumpHistory(-1), [jumpHistory]);
	const redo = React.useCallback(() => jumpHistory(1), [jumpHistory]);

	const deleteEditorBlocks = React.useCallback(
		(ids: string[], rangeDeletion?: CrossBlockDeletion) => {
			const editor = editorRef.current;
			if (!editor || previewRef.current || !ids.length) return;
			if (editorMutationBatchRef.current && !pendingDeleteRef.current) return;
			pendingDeleteRef.current++;
			saveGenerationRef.current++;
			editorMutationBatchRef.current = true;
			setBusy(true);
			renderChainRef.current = renderChainRef.current
				.then(async () => {
					await beforeDeleteCheckpointRef.current;
					if (editorRef.current !== editor) return;
					const indexes = [...new Set(ids)]
						.map((id) => editor.blocks.getBlockIndex(id))
						.filter((index) => index >= 0)
						.sort((a, b) => b - a);
					if (!indexes.length) return;
					const before = await safeSaveEditor(editor);
					if (rangeDeletion?.tableCells) {
						const holder = holderRef.current;
						if (!holder) return;
						const cells = resolveTableCells(
							holder,
							rangeDeletion.tableCells.map((cell) => cell.address),
						);
						if (cells.length !== rangeDeletion.tableCells.length)
							throw new Error("Selected table cells no longer exist");
						checkpointHistory(fromEditorData(before));
						markHistoryActionBoundary();
						editor.toolbar.close();
						cells.forEach((cell, index) => {
							tableCellContentRoot(cell).innerHTML =
								rangeDeletion.tableCells?.[index].html ?? "";
							cell.dispatchEvent(new InputEvent("input", { bubbles: true }));
						});
						placeCaretAtTextOffset(
							tableCellContentRoot(cells[0]),
							rangeDeletion.prefixLength,
						);
						inlineRangeRef.current = null;
						toolbarSelectionRef.current = null;
						commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
						return;
					}
					const readEndpoint = async (id: string) => {
						const saved = before.blocks.find((block) => block.id === id);
						if (saved) return saved;
						// Saver validation omits empty paragraphs/list items, but a
						// native selection can legitimately start or end in one.
						const raw = await editor.blocks.getById(id)?.save();
						if (!raw?.data) return undefined;
						return {
							id: raw.id,
							type: raw.tool,
							data: raw.data,
							tunes: ("tunes" in raw ? raw.tunes : undefined) as
								| Record<string, unknown>
								| undefined,
						};
					};
					const first = rangeDeletion ? await readEndpoint(ids[0]) : undefined;
					const last = rangeDeletion
						? await readEndpoint(ids[ids.length - 1])
						: undefined;
					if (
						rangeDeletion &&
						(!first || !last || indexes.length !== ids.length)
					)
						throw new Error("The selected blocks are no longer available");
					checkpointHistory(fromEditorData(before));
					markHistoryActionBoundary();
					editor.toolbar.close();
					let removeIds = ids;
					if (rangeDeletion && first && last) {
						const merged = rangeDeletion.prefix + rangeDeletion.suffix;
						const data = rangeDeletion.startListItems
							? {
									...first.data,
									items: [...rangeDeletion.startListItems, merged],
								}
							: { ...first.data, text: merged };
						await editor.blocks.update(ids[0], data, first.tunes);
						const keepLast = !!rangeDeletion.endListItems?.length;
						if (keepLast)
							await editor.blocks.update(
								ids[ids.length - 1],
								{ ...last.data, items: rangeDeletion.endListItems },
								last.tunes,
							);
						removeIds = ids.slice(1, keepLast ? -1 : undefined);
					}
					for (const index of removeIds
						.map((id) => editor.blocks.getBlockIndex(id))
						.filter((index) => index >= 0)
						.sort((a, b) => b - a))
						editor.blocks.delete(index);
					const focusIndex = Math.min(
						indexes[indexes.length - 1],
						editor.blocks.getBlocksCount() - 1,
					);
					if (focusIndex >= 0) editor.caret.setToBlock(focusIndex, "start");
					if (rangeDeletion) {
						const block = editor.blocks.getById(ids[0]);
						const root = rangeDeletion.startListItems
							? block?.holder.querySelectorAll<HTMLElement>(".cdx-list__item")[
									rangeDeletion.startListItems.length
								]
							: block?.holder.querySelector<HTMLElement>(
									".ce-paragraph, .ce-header",
								);
						if (root) placeCaretAtTextOffset(root, rangeDeletion.prefixLength);
					}
					inlineRangeRef.current = null;
					toolbarSelectionRef.current = null;
					commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
				})
				.catch(() => {
					if (editorRef.current === editor)
						toast.error("刪除未完成，請再試一次", { toasterId });
				})
				.finally(() => {
					pendingDeleteRef.current--;
					if (!pendingDeleteRef.current && !pendingHistoryRef.current) {
						editorMutationBatchRef.current = false;
						setBusy(false);
					}
				});
		},
		[
			checkpointHistory,
			commitEditorValue,
			markHistoryActionBoundary,
			toasterId,
		],
	);

	const replaceImageBlock = React.useCallback(
		(id: string, data: DescriptionImageData, isCurrent: () => boolean) => {
			const editor = editorRef.current;
			if (!editor) return Promise.resolve();
			// Keep uploading independent from text editing. Only the final local
			// replacement joins the editor's history queue, using the current file.
			const operation = renderChainRef.current.then(async () => {
				await editor.isReady;
				await beforeDeleteCheckpointRef.current;
				if (editorRef.current !== editor || !isCurrent() || previewRef.current)
					return;
				const block = editor.blocks.getById(id);
				if (!block || block.name !== "image") return;
				editorMutationBatchRef.current = true;
				saveGenerationRef.current++;
				setBusy(true);
				try {
					const before = await safeSaveEditor(editor);
					if (!isCurrent() || editorRef.current !== editor) return;
					checkpointHistory(fromEditorData(before));
					markHistoryActionBoundary();
					await editor.blocks.update(
						id,
						data,
						before.blocks.find((item) => item.id === id)?.tunes,
					);
					commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
				} finally {
					if (!pendingHistoryRef.current && !pendingDeleteRef.current) {
						editorMutationBatchRef.current = false;
						setBusy(false);
					}
				}
			});
			// The tool shows a persistent, retryable upload error; keep a rejection
			// from poisoning subsequent Undo, Delete or external-value operations.
			renderChainRef.current = operation.catch(() => {});
			return operation;
		},
		[checkpointHistory, commitEditorValue, markHistoryActionBoundary],
	);

	// ⌘Z / ⌘⇧Z（Windows/Linux 也收 Ctrl+Z / Ctrl+Y）：攔在 capture 前，preventDefault
	// 蓋掉 contenteditable 原生的逐字 undo，改走上面這條跨區塊的快照式
	// undo/redo，不然兩套機制會互相打架。
	React.useEffect(() => {
		const dialog = dialogRef.current;
		if (!dialog) return;
		const onKeyDown = (e: KeyboardEvent) => {
			if (previewRef.current || e.isComposing) return;
			if ((e.target as HTMLElement | null)?.closest("input, textarea, select"))
				return;
			if (e.key === "Backspace" || e.key === "Delete") {
				const holder = holderRef.current;
				const target = e.target as HTMLElement | null;
				const selection = window.getSelection();
				const selectedIds = holder ? selectedBlockIds(holder, null) : [];
				const editable = target?.closest<HTMLElement>("[contenteditable=true]");
				if (
					holder &&
					(holder.contains(editable ?? null) || selectedIds.length)
				) {
					if (pendingHistoryRef.current || pendingDeleteRef.current) {
						e.preventDefault();
						e.stopImmediatePropagation();
						return;
					}
					const nativeRange = activeSelectionRange(holder);
					const rangeDeletion = nativeRange
						? captureCrossBlockDeletion(holder, nativeRange)
						: null;
					if (rangeDeletion) {
						e.preventDefault();
						e.stopImmediatePropagation();
						if (rangeDeletion === "unsupported")
							toast.info("跨表格或圖片說明的選取，請分段刪除", { toasterId });
						else deleteEditorBlocks(rangeDeletion.ids, rangeDeletion);
						return;
					}
					if (
						selectedIds.length &&
						(!selection?.rangeCount || selection.isCollapsed)
					) {
						e.preventDefault();
						e.stopImmediatePropagation();
						deleteEditorBlocks(selectedIds);
						return;
					}
					const editor = editorRef.current;
					if (editor && editable) {
						const blockId =
							editable.closest<HTMLElement>(".ce-block")?.dataset.id ?? "";
						const cell = editable.closest(".tc-cell");
						const cellIndex = cell
							? Array.prototype.indexOf.call(
									holder.querySelectorAll(".tc-cell"),
									cell,
								)
							: -1;
						const blockKey = `${blockId}:${cellIndex}`;
						const decision = descriptionHistoryInputBatch(
							e.key === "Backspace"
								? "deleteContentBackward"
								: "deleteContentForward",
							null,
							blockKey,
							Date.now(),
							lastHistoryInputRef.current,
						);
						lastHistoryInputRef.current = decision.next;
						if (decision.forceNewAction) {
							// Start save before the browser/tool applies deletion. The tools
							// read their content synchronously, then sanitize asynchronously.
							const before = safeSaveEditor(editor);
							beforeDeleteCheckpointRef.current =
								beforeDeleteCheckpointRef.current
									.then(async () => {
										const data = await before;
										if (editorRef.current !== editor) return;
										checkpointHistory(fromEditorData(data));
										forceNextHistoryCheckpointRef.current = true;
									})
									.catch(() => {
										/* the editor may have unmounted */
									});
						}
					}
				}
			}
			if (!(e.metaKey || e.ctrlKey)) return;
			const key = e.key.toLowerCase();
			if (key !== "z" && key !== "y") return;
			e.preventDefault();
			if (key === "y" || e.shiftKey) jumpHistory(1);
			else jumpHistory(-1);
		};
		dialog.addEventListener("keydown", onKeyDown, true);
		return () => dialog.removeEventListener("keydown", onKeyDown, true);
	}, [checkpointHistory, deleteEditorBlocks, jumpHistory, toasterId]);

	React.useEffect(() => {
		let destroyed = false;
		let detachToolbox = () => {};
		let detachDrag = () => {};
		let detachMobileToolbar = () => {};
		let detachSelectionTracking = () => {};

		void (async () => {
			const [
				{ default: EditorJSCtor },
				{ default: Header },
				{ default: DescriptionImageTool },
				{ default: Underline },
				{ default: Table },
				{ default: List },
			] = await Promise.all([
				import("@editorjs/editorjs"),
				import("@editorjs/header"),
				import("@/features/seller/components/DescriptionImageTool"),
				import("@editorjs/underline"),
				import("@editorjs/table"),
				import("@editorjs/list"),
			]);
			if (destroyed || !holderRef.current) return;

			class AlignedTableTool extends Table {
				static get sanitize() {
					return {
						content: {
							div: (element: Element) => {
								const alignment = element.getAttribute(
									TABLE_CELL_ALIGN_ATTRIBUTE,
								);
								return alignment === "center" || alignment === "right"
									? {
											[TABLE_CELL_ALIGN_ATTRIBUTE]: alignment,
											style: `text-align:${alignment}`,
										}
									: false;
							},
						},
					};
				}
			}

			// 快照堆疊的底部（初始 value）先過一次 canonical round-trip：後端
			// 來的 JSON key 順序 / legacy content 形狀跟 fromEditorData 的輸出
			// 不一定 byte-equal，若不先正規化，undo 回到底部時 render-echo 的
			// onChange 會把同內容誤判成新編輯，redo 分支就被 truncate 掉。
			//（放在這裡是因為 htmlToRuns 需要 document，只能在 client 做。）
			historyRef.current = {
				stack: [fromEditorData(toEditorData(initialRef.current))],
				index: 0,
			};

			// 最後一次存檔失敗時沒有更新的存檔會接手，輸入停下後就不會再有
			// 下一次存檔；由這裡補存，否則 onChange 會停在失敗前的舊快照。
			// 重試額度每次新的存檔觸發都重算，前一次觸發用掉的不影響這次。
			const MAX_SAVE_RETRIES = 3;
			let saveRetries = 0;
			const saveCurrentEditor = async (retry = false) => {
				if (!retry) saveRetries = 0;
				const currentEditor = editorRef.current;
				if (
					!currentEditor ||
					destroyed ||
					editorMutationBatchRef.current ||
					previewRef.current
				)
					return;
				const generation = ++saveGenerationRef.current;
				try {
					const data = await safeSaveEditor(currentEditor);
					await beforeDeleteCheckpointRef.current;
					if (
						destroyed ||
						editorMutationBatchRef.current ||
						generation !== saveGenerationRef.current
					)
						return;
					commitEditorValue(fromEditorData(data));
				} catch {
					// A pending save may settle after a block/editor was destroyed.
					// Keep the last good snapshot and retry when no newer save took over.
					if (
						destroyed ||
						generation !== saveGenerationRef.current ||
						saveRetries >= MAX_SAVE_RETRIES
					)
						return;
					saveRetries += 1;
					requestAnimationFrame(() => void saveCurrentEditor(true));
				}
			};

			const editor = new EditorJSCtor({
				holder: holderRef.current,
				data: toEditorData(initialRef.current),
				minHeight: 120,
				placeholder: "撰寫商品故事…",
				i18n: I18N,
				tools: {
					// Built-in paragraph, re-declared to attach the align tune and
					// pin the inline toolbar (drops the built-in link tool).
					paragraph: {
						inlineToolbar: [...INLINE_TOOLS],
						tunes: ["align"],
					},
					header: {
						// biome-ignore lint/suspicious/noExplicitAny: Editor.js tool types are loose
						class: Header as any,
						inlineToolbar: [...INLINE_TOOLS],
						shortcut: "CMD+SHIFT+H",
						config: { levels: [2, 3], defaultLevel: 2, placeholder: "標題" },
						tunes: ["align"],
					},
					table: {
						// biome-ignore lint/suspicious/noExplicitAny: Editor.js tool types are loose
						class: AlignedTableTool as any,
						// 儲存格支援行內樣式 / 連結（inline-run 模型）
						inlineToolbar: [...INLINE_TOOLS],
						config: { rows: 2, cols: 2, withHeadings: true },
					},
					list: {
						// biome-ignore lint/suspicious/noExplicitAny: Editor.js tool types are loose
						class: List as any,
						// 項目支援行內樣式 / 連結（inline-run 模型）；外部貼上的 <ol>/<ul>
						// 由 tool 的 pasteConfig（OL/UL/LI）自動轉為 list block
						inlineToolbar: [...INLINE_TOOLS],
						config: { defaultStyle: "unordered" },
						tunes: ["align"],
					},
					align: {
						// biome-ignore lint/suspicious/noExplicitAny: Editor.js tool types are loose
						class: AlignBlockTune as any,
					},
					underline: {
						// biome-ignore lint/suspicious/noExplicitAny: Editor.js tool types are loose
						class: Underline as any,
						shortcut: "CMD+U",
					},
					textColor: {
						// biome-ignore lint/suspicious/noExplicitAny: Editor.js tool types are loose
						class: TextColorInlineTool as any,
						config: {
							onOpen: (range: Range) => {
								inlineRangeRef.current = range.cloneRange();
								dialogRef.current
									?.querySelector<HTMLButtonElement>(
										"[data-description-color-trigger]",
									)
									?.click();
							},
						},
					},
					image: {
						class: DescriptionImageTool,
						toolbox: { title: "圖片", icon: IMAGE_PLUS_ICON_MARKUP },
						config: {
							iconMarkup: IMAGE_PLUS_ICON_MARKUP,
							upload: async (
								file: File,
								onProgress: (ratio: number) => void,
							) => {
								const asset = await uploadImageFileRef.current(
									file,
									onProgress,
									{
										compress: false,
									},
								);
								return {
									url: asset.previewUrl,
									stagedToken: asset.stagedToken,
								};
							},
							replace: replaceImageBlock,
							remove: (id: string) => deleteEditorBlocks([id]),
						},
					},
				},
				onChange: () => saveCurrentEditor(),
			});
			editorRef.current = editor;
			await editor.isReady;
			if (destroyed) return;
			// Y-axis magnetic reordering off the dedicated handle (replaces the
			// X-sensitive editorjs-drag-drop plugin).
			try {
				detachDrag = attachBlockDragReorder(editor, holderRef.current);
			} catch {
				/* reordering is a nicety; never block the editor on it */
			}
			try {
				detachMobileToolbar = attachMobileBlockToolbarAlignment(
					editor,
					holderRef.current,
				);
			} catch {
				/* mobile alignment is progressive enhancement */
			}
			setReady(true);

			// The "+" should drop a fresh 內文 block straight below the active
			// block — no insert menu — and the seller re-types it via 內容轉換.
			// Pointer events also cover touch, unlike the previous mouseover-only
			// tracking that could make mobile insert at the document end.
			const holder = holderRef.current;
			if (holder) {
				const rememberToolbarSelection = () => {
					rememberInlineSelection();
					const snapshot = toolbarSelectionSnapshot(holder);
					if (snapshot) toolbarSelectionRef.current = snapshot;
				};
				const onEditorPointerDown = () => {
					// A new editor gesture invalidates an older persistent selection. The
					// pointerup/selection observer below records the replacement, if any.
					toolbarSelectionRef.current = null;
					inlineRangeRef.current = null;
				};
				const selectionObserver = new MutationObserver(() => {
					rememberToolbarSelection();
				});
				const redactor = holder.querySelector(".codex-editor__redactor");
				if (redactor) {
					selectionObserver.observe(redactor, {
						subtree: true,
						attributes: true,
						attributeFilter: ["class"],
					});
				}
				holder.addEventListener("pointerdown", onEditorPointerDown, true);
				holder.addEventListener("pointerup", rememberToolbarSelection, true);
				holder.addEventListener("keyup", rememberToolbarSelection, true);
				document.addEventListener("selectionchange", rememberToolbarSelection);
				detachSelectionTracking = () => {
					selectionObserver.disconnect();
					holder.removeEventListener("pointerdown", onEditorPointerDown, true);
					holder.removeEventListener(
						"pointerup",
						rememberToolbarSelection,
						true,
					);
					holder.removeEventListener("keyup", rememberToolbarSelection, true);
					document.removeEventListener(
						"selectionchange",
						rememberToolbarSelection,
					);
				};

				let activeIndex = -1;
				const onBlockIntent = (e: Event) => {
					const blockEl = (e.target as HTMLElement | null)?.closest?.(
						".ce-block",
					);
					if (!blockEl) return;
					const redactor = holder.querySelector(".codex-editor__redactor");
					if (!redactor) return;
					activeIndex = Array.prototype.indexOf.call(
						redactor.querySelectorAll(":scope > .ce-block"),
						blockEl,
					);
				};
				const onPlus = (e: MouseEvent) => {
					if (previewRef.current) return;
					if (!(e.target as HTMLElement | null)?.closest?.(".ce-toolbar__plus"))
						return;
					e.preventDefault();
					e.stopImmediatePropagation();
					const total = editor.blocks.getBlocksCount();
					const currentIndex = editor.blocks.getCurrentBlockIndex();
					const sourceIndex =
						window.matchMedia(EDITOR_JS_MOBILE_MEDIA).matches &&
						currentIndex >= 0
							? currentIndex
							: activeIndex >= 0
								? activeIndex
								: currentIndex;
					const at =
						sourceIndex >= 0 ? Math.min(sourceIndex + 1, total) : total;
					editor.blocks.insert("paragraph", undefined, undefined, at, true);
					activeIndex = at;
				};
				// Capture the menu's stable target when it opens, before focusing its
				// search field or later editor gestures can change the current index.
				let deleteTargetId: string | undefined;
				let settingsPopover: Element | null = null;
				const rememberDeleteTarget = () => {
					const nextPopover = holder.querySelector(".ce-settings .ce-popover");
					if (nextPopover === settingsPopover) return;
					settingsPopover = nextPopover;
					deleteTargetId = nextPopover
						? editor.blocks.getBlockByIndex(
								editor.blocks.getCurrentBlockIndex(),
							)?.id
						: undefined;
				};
				const settingsObserver = new MutationObserver(rememberDeleteTarget);
				settingsObserver.observe(holder, { childList: true, subtree: true });

				// One click remains reversible as a single history action.
				const onDeleteTune = (e: MouseEvent) => {
					const item = (e.target as HTMLElement | null)?.closest?.(
						'.ce-popover-item[data-item-name="delete"]',
					);
					if (!item) return;
					e.preventDefault();
					e.stopImmediatePropagation();
					rememberDeleteTarget();
					const id = deleteTargetId;
					if (id) deleteEditorBlocks([id]);
				};
				// ---- Notion 式貼上（#1246）----
				// capture 階段先於 Editor.js 管線：
				//   1. 游標在表格儲存格＋剪貼簿是表格資料（HTML <table> 或 TSV 純文字）
				//      → 試算表語意：從錨點格逐格填入、列/欄不足自動擴增、超上限裁切。
				//   2. 游標在段落/標題＋純文字 TSV（無 HTML 表格）→ 插入表格 block。
				//   其餘（含 HTML 表格貼在段落 = tool 原生 pasteConfig）一律放行。
				const onBeforeInput = (e: InputEvent) => {
					if (previewRef.current || e.isComposing) return;
					if (e.inputType.startsWith("delete") && e.cancelable) {
						const range = activeSelectionRange(holder);
						const rangeDeletion = range
							? captureCrossBlockDeletion(holder, range)
							: null;
						if (
							rangeDeletion ||
							pendingDeleteRef.current ||
							pendingHistoryRef.current
						) {
							e.preventDefault();
							e.stopImmediatePropagation();
							if (!pendingDeleteRef.current && !pendingHistoryRef.current) {
								if (rangeDeletion === "unsupported")
									toast.info("跨表格或圖片說明的選取，請分段刪除", {
										toasterId,
									});
								else if (rangeDeletion)
									deleteEditorBlocks(rangeDeletion.ids, rangeDeletion);
							}
							return;
						}
					}
					const historyDirection = descriptionHistoryDirectionForInputType(
						e.inputType,
					);
					if (historyDirection) {
						if (e.cancelable) e.preventDefault();
						e.stopPropagation();
						e.stopImmediatePropagation();
						jumpHistory(historyDirection);
						return;
					}
					const target = e.target as HTMLElement | null;
					const block = target?.closest?.(".ce-block") as HTMLElement | null;
					const allBlocks = holder.querySelectorAll(
						".codex-editor__redactor > .ce-block",
					);
					const blockIndex = block
						? Array.prototype.indexOf.call(allBlocks, block)
						: -1;
					const cell = target?.closest?.(".tc-cell") as HTMLElement | null;
					const cellIndex = cell
						? Array.prototype.indexOf.call(
								holder.querySelectorAll(".tc-cell"),
								cell,
							)
						: -1;
					const blockKey = `${block?.dataset.id ?? blockIndex}:${cellIndex}`;
					const decision = descriptionHistoryInputBatch(
						e.inputType,
						e.data,
						blockKey,
						Date.now(),
						lastHistoryInputRef.current,
					);
					if (decision.forceNewAction) {
						forceNextHistoryCheckpointRef.current = true;
					}
					lastHistoryInputRef.current = decision.next;
				};
				const onPaste = (e: ClipboardEvent) => {
					if (previewRef.current) return;
					forceNextHistoryCheckpointRef.current = true;
					lastHistoryInputRef.current = null;
					const html = e.clipboardData?.getData("text/html") ?? "";
					const plain = e.clipboardData?.getData("text/plain") ?? "";
					const target = e.target as HTMLElement | null;
					const imageFiles = clipboardImageFiles(e.clipboardData);
					if (imageFiles.length > 0) {
						e.preventDefault();
						e.stopImmediatePropagation();
						const current = editor.blocks.getCurrentBlockIndex();
						let insertAt =
							current >= 0 ? current + 1 : editor.blocks.getBlocksCount();
						const toastId = toast.loading(
							imageFiles.length > 1
								? `正在貼上 ${imageFiles.length} 張圖片…`
								: "正在貼上圖片…",
							{ toasterId },
						);
						void (async () => {
							let uploaded = 0;
							for (const file of imageFiles) {
								try {
									const asset = await uploadImageFileRef.current(
										file,
										undefined,
										{
											compress: false,
										},
									);
									editor.blocks.insert(
										"image",
										{
											file: {
												url: asset.previewUrl,
												stagedToken: asset.stagedToken,
											},
											caption: "",
											withBorder: false,
											withBackground: false,
											stretched: false,
										},
										undefined,
										insertAt++,
										false,
									);
									uploaded += 1;
								} catch {
									// Continue with the remaining clipboard images.
								}
							}
							if (uploaded === imageFiles.length) {
								toast.success(
									uploaded > 1 ? `已貼上 ${uploaded} 張圖片` : "圖片已貼上",
									{ id: toastId, toasterId },
								);
							} else if (uploaded > 0) {
								toast.warning(
									`已貼上 ${uploaded} 張，${imageFiles.length - uploaded} 張上傳失敗`,
									{ id: toastId, toasterId },
								);
							} else {
								toast.error("圖片貼上失敗，請確認格式與大小後再試", {
									id: toastId,
									toasterId,
								});
							}
						})();
						return;
					}
					const paragraph = target?.closest<HTMLElement>(".ce-paragraph");
					const markdown =
						!html &&
						!holder.querySelector(".ce-block--selected") &&
						paragraph &&
						!paragraph.textContent?.trim() &&
						(!window.getSelection()?.rangeCount ||
							paragraph.contains(
								window.getSelection()?.getRangeAt(0).commonAncestorContainer ??
									null,
							))
							? parseDescriptionMarkdownPaste(plain)
							: null;
					if (markdown) {
						e.preventDefault();
						e.stopImmediatePropagation();
						const targetId =
							paragraph?.closest<HTMLElement>(".ce-block")?.dataset.id;
						const at = targetId ? editor.blocks.getBlockIndex(targetId) : -1;
						if (at < 0) return;
						void (async () => {
							editorMutationBatchRef.current = true;
							try {
								checkpointHistory(fromEditorData(await safeSaveEditor(editor)));
								markHistoryActionBoundary();
								// Insert first, then remove the empty target: Editor.js never auto-adds a spare block.
								markdown.forEach((block, index) => {
									editor.blocks.insert(
										block.type,
										block.data,
										undefined,
										at + index + 1,
										false,
									);
								});
								editor.blocks.delete(at);
								editor.caret.setToBlock(at + markdown.length - 1, "end");
								commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
								toast.success("已將貼上的文字轉為標題與清單", { toasterId });
							} finally {
								editorMutationBatchRef.current = false;
							}
						})();
						return;
					}
					const grid = parseClipboardGrid(html, plain);
					if (!grid || !target) return;

					const cellEl = target.closest?.(".tc-cell") as HTMLElement | null;
					if (cellEl) {
						e.preventDefault();
						e.stopImmediatePropagation();
						const { grid: g, clamped } = clampGrid(grid);
						const truncated = expandGridIntoTable(cellEl, g);
						if (clamped || truncated) {
							toast.warning(
								`表格資料超過上限，已裁切至 ${TABLE_MAX_COLS} 欄 × ${TABLE_MAX_ROWS} 列`,
								{ toasterId },
							);
						}
						return;
					}

					// HTML 表格貼在文字區也走自己的解析器——原生 pasteConfig 對真實
					// 網站標記（colspan 跨欄標題列、th/td 混排）解析失敗會退回預設
					// 空表（#1246 cookie886 案例），不再放行。
					if (!target.closest?.(".ce-paragraph, .ce-header")) return;
					e.preventDefault();
					e.stopImmediatePropagation();
					const { grid: g, clamped } = clampGrid(grid);
					const idx = editor.blocks.getCurrentBlockIndex();
					const at = idx >= 0 ? idx + 1 : editor.blocks.getBlocksCount();
					editor.blocks.insert(
						"table",
						{ withHeadings: false, content: g },
						undefined,
						at,
						true,
					);
					if (clamped) {
						toast.warning(
							`表格資料超過上限，已裁切至 ${TABLE_MAX_COLS} 欄 × ${TABLE_MAX_ROWS} 列`,
							{ toasterId },
						);
					}
				};
				// ---- 內容以表格/圖片結尾時，點下方空白自動補段落（Notion「點哪都能寫」）----
				const onEmptyAreaClick = (e: MouseEvent) => {
					if (previewRef.current) return;
					const t = e.target as HTMLElement | null;
					if (!t || t.closest(".ce-block") || t.closest(".ce-toolbar")) return;
					const redactor = holder.querySelector(".codex-editor__redactor");
					if (!redactor?.contains(t)) {
						// 點在 redactor 外（holder padding）也算空白區，但要在 holder 內
						if (!t.closest(".codex-editor")) return;
					}
					const blocks = holder.querySelectorAll(
						".codex-editor__redactor > .ce-block",
					);
					const last = blocks[blocks.length - 1] as HTMLElement | undefined;
					if (!last) return;
					if (e.clientY <= last.getBoundingClientRect().bottom) return;
					// 段落/標題結尾：Editor.js 原生點擊已能聚焦，不插手
					if (!last.querySelector("table, .tc-table, .image-tool")) return;
					editor.blocks.insert(
						"paragraph",
						undefined,
						undefined,
						editor.blocks.getBlocksCount(),
						true,
					);
				};
				let immediateSaveFrame = 0;
				const scheduleImmediateSave = () => {
					if (immediateSaveFrame) return;
					immediateSaveFrame = requestAnimationFrame(() => {
						immediateSaveFrame = 0;
						void saveCurrentEditor();
					});
				};
				const onEditingKeyUp = (event: KeyboardEvent) => {
					if (["Backspace", "Delete", "Enter"].includes(event.key))
						scheduleImmediateSave();
				};
				holder.addEventListener("input", scheduleImmediateSave, true);
				holder.addEventListener("keyup", onEditingKeyUp, true);
				holder.addEventListener("pointerover", onBlockIntent, true);
				holder.addEventListener("focusin", onBlockIntent, true);
				holder.addEventListener("click", onPlus, true);
				holder.addEventListener("click", onDeleteTune, true);
				holder.addEventListener("beforeinput", onBeforeInput, true);
				holder.addEventListener("paste", onPaste, true);
				holder.addEventListener("click", onEmptyAreaClick);
				detachToolbox = () => {
					cancelAnimationFrame(immediateSaveFrame);
					holder.removeEventListener("input", scheduleImmediateSave, true);
					holder.removeEventListener("keyup", onEditingKeyUp, true);
					settingsObserver.disconnect();
					holder.removeEventListener("pointerover", onBlockIntent, true);
					holder.removeEventListener("focusin", onBlockIntent, true);
					holder.removeEventListener("click", onPlus, true);
					holder.removeEventListener("click", onDeleteTune, true);
					holder.removeEventListener("beforeinput", onBeforeInput, true);
					holder.removeEventListener("paste", onPaste, true);
					holder.removeEventListener("click", onEmptyAreaClick);
				};
			}
		})().catch(() => {
			if (!destroyed) setLoadError(true);
		});

		return () => {
			destroyed = true;
			detachToolbox();
			detachSelectionTracking();
			detachMobileToolbar();
			detachDrag();
			editorRef.current?.destroy?.();
			editorRef.current = null;
		};
		// These callbacks are ref-backed and stable, so this remains a mount-once
		// editor lifecycle in practice.
	}, [
		commitEditorValue,
		deleteEditorBlocks,
		jumpHistory,
		toasterId,
		rememberInlineSelection,
		replaceImageBlock,
		markHistoryActionBoundary,
		checkpointHistory,
	]);

	// Re-render the editor only for *external* value changes (undo/redo 跳轉、
	// AI patch)。
	React.useEffect(() => {
		const sig = JSON.stringify(value);
		if (sig === lastSig.current) {
			// 自己 onChange 打出去、經父層繞回來的 echo（內容相同）。旗標必須
			// 在這裡消化掉——否則殘留到「下一次」真正的外部變動（第一下
			// undo/redo）會被誤判成 self-update 吞掉，畫面完全不動。
			selfUpdate.current = false;
			return;
		}
		if (selfUpdate.current) {
			selfUpdate.current = false;
			lastSig.current = sig;
			return;
		}
		const editor = editorRef.current;
		if (!editor?.render) return;
		lastSig.current = sig;
		renderChainRef.current = renderChainRef.current
			.then(() => editor.isReady)
			.then(() => editor.render(toEditorData(value)))
			.catch(() => {
				/* editor torn down mid-render — nothing to restore */
			});
	}, [value]);

	/* ---- top toolbar actions (drive the same Editor.js APIs) ---- */
	const captureToolbarSelection = React.useCallback(() => {
		rememberInlineSelection();
		markHistoryActionBoundary();
		const holder = holderRef.current;
		if (!holder) {
			return;
		}
		let snapshot = toolbarSelectionSnapshot(holder);
		// Keyboard activation can happen after focus has moved into the toolbar.
		// In that case the live DOM selection is already empty, but the last
		// non-empty editor snapshot is still the user's intended target.
		if (!snapshot && toolbarSelectionRef.current) return;
		if (!snapshot) {
			const editor = editorRef.current;
			const currentIndex = editor?.blocks.getCurrentBlockIndex() ?? -1;
			const current =
				currentIndex >= 0 ? editor?.blocks.getBlockByIndex(currentIndex) : null;
			if (current) {
				snapshot = {
					blockIds: [current.id],
					tableCells: [],
					capturedAt: Date.now(),
				};
			}
		}
		// Editor.js clears `.ce-block--selected` from a document-level capture
		// mousedown listener. A later empty React mousedown must never overwrite
		// the snapshot captured on pointerdown or while the selection was active.
		if (snapshot) toolbarSelectionRef.current = snapshot;
	}, [markHistoryActionBoundary, rememberInlineSelection]);

	const consumeToolbarSelection = (): ToolbarSelectionSnapshot | null => {
		const snapshot = toolbarSelectionRef.current;
		toolbarSelectionRef.current = null;
		return snapshot;
	};

	const convertCurrent = async (
		type: string,
		data?: Record<string, unknown>,
	) => {
		const editor = editorRef.current;
		if (!editor) return;
		const snapshot = consumeToolbarSelection();
		const current = editor.blocks.getCurrentBlockIndex();
		const ids = snapshot?.blockIds.length
			? snapshot.blockIds
			: current >= 0
				? [editor.blocks.getBlockByIndex(current)?.id]
				: [];
		const blocks = ids
			.filter((id): id is string => !!id)
			.map((id) => editor.blocks.getById(id))
			.filter((block) => !!block);
		if (!blocks.length) {
			toast.info("請先點選要調整的段落", { toasterId });
			return;
		}
		if (blocks.some((block) => !["paragraph", "header"].includes(block.name))) {
			toast.info("段落樣式適用於文字；請先選取內文或標題", { toasterId });
			return;
		}
		editorMutationBatchRef.current = true;
		try {
			const before = await safeSaveEditor(editor);
			checkpointHistory(fromEditorData(before));
			markHistoryActionBoundary();
			let lastBlockId = blocks[blocks.length - 1].id;
			for (const block of blocks) {
				const tunes = before.blocks.find(
					(saved) => saved.id === block.id,
				)?.tunes;
				const updated =
					block.name === type
						? data
							? await editor.blocks.update(block.id, data, tunes)
							: block
						: await editor.blocks.convert(block.id, type, data);
				if (block.name !== type && tunes)
					await editor.blocks.update(updated.id, undefined, tunes);
				lastBlockId = updated.id;
			}
			commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
			const index = editor.blocks.getBlockIndex(lastBlockId);
			if (index >= 0) editor.caret.setToBlock(index, "end");
			rememberInlineSelection();
		} catch {
			toast.error("無法套用段落樣式，請重新選取後再試", { toasterId });
		} finally {
			editorMutationBatchRef.current = false;
		}
	};

	const applyInline = async (command: "bold" | "italic" | "underline") => {
		const editor = editorRef.current;
		if (!editor || busy || editorMutationBatchRef.current) return;
		if (!restoreInlineSelection()) {
			toast.info("請先選取文字或放置游標", { toasterId });
			return;
		}
		editorMutationBatchRef.current = true;
		try {
			await renderChainRef.current;
			checkpointHistory(fromEditorData(await safeSaveEditor(editor)));
			markHistoryActionBoundary();
			if (!restoreInlineSelection()) return;
			document.execCommand(command, false);
			commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
			rememberInlineSelection();
		} catch {
			toast.error("無法套用文字格式，請重新選取後再試", { toasterId });
		} finally {
			editorMutationBatchRef.current = false;
		}
	};
	const applyColor = async (hex: string) => {
		const editor = editorRef.current;
		if (!editor || busy || editorMutationBatchRef.current) return;
		if (!restoreInlineSelection()) {
			toast.info("請先選取要變色的文字", { toasterId });
			return;
		}
		editorMutationBatchRef.current = true;
		try {
			await renderChainRef.current;
			checkpointHistory(fromEditorData(await safeSaveEditor(editor)));
			markHistoryActionBoundary();
			if (!restoreInlineSelection()) return;
			const cssMode = document.queryCommandState("styleWithCSS");
			try {
				document.execCommand("styleWithCSS", false, "true");
				document.execCommand("foreColor", false, hex);
			} finally {
				document.execCommand("styleWithCSS", false, String(cssMode));
			}
			commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
			rememberInlineSelection();
		} catch {
			toast.error("無法套用文字顏色，請重新選取後再試", { toasterId });
		} finally {
			editorMutationBatchRef.current = false;
		}
	};

	const tidyFormatting = async () => {
		const editor = editorRef.current;
		if (!editor || busy) return;
		setBusy(true);
		editorMutationBatchRef.current = true;
		try {
			await renderChainRef.current;
			const before = await safeSaveEditor(editor);
			checkpointHistory(fromEditorData(before));
			const result = cleanupDescriptionEditorData(before);
			if (!result.changed) {
				toast.info("格式已整齊，沒有需要整理的內容", { toasterId });
				return;
			}
			markHistoryActionBoundary();
			renderChainRef.current = renderChainRef.current.then(() =>
				editor.render(
					result.data.blocks.length
						? result.data
						: { blocks: [{ type: "paragraph", data: { text: "" } }] },
				),
			);
			await renderChainRef.current;
			commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
			inlineRangeRef.current = null;
			toolbarSelectionRef.current = null;
			toast.success(`已整理 ${result.changedBlocks} 個區塊，可按復原還原`, {
				toasterId,
			});
		} catch {
			toast.error("整理未完成，請重試", { toasterId });
		} finally {
			editorMutationBatchRef.current = false;
			setBusy(false);
		}
	};
	const togglePreview = async () => {
		const editor = editorRef.current;
		if (!editor || busy) return;
		if (
			holderRef.current?.querySelector(
				'.hk-description-image[data-state="uploading"]',
			)
		) {
			toast.info("圖片仍在上傳，完成後即可預覽", { toasterId });
			return;
		}
		setBusy(true);
		editorMutationBatchRef.current = true;
		try {
			if (!preview)
				commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
			renderChainRef.current = renderChainRef.current.then(() =>
				editor.readOnly.toggle(!preview),
			);
			await renderChainRef.current;
			previewRef.current = !preview;
			setPreview(!preview);
			inlineRangeRef.current = null;
		} catch {
			toast.error("無法切換閱讀預覽，請再試一次", { toasterId });
		} finally {
			editorMutationBatchRef.current = false;
			setBusy(false);
		}
	};

	const applyAlign = async (alignment: Alignment) => {
		const editor = editorRef.current;
		if (!editor) return;
		const holder = holderRef.current;
		const snapshot = consumeToolbarSelection();
		const range = !snapshot && holder ? activeSelectionRange(holder) : null;
		const tableCells = snapshot
			? holder
				? resolveTableCells(holder, snapshot.tableCells)
				: []
			: holder
				? selectedTableCells(holder, range)
				: [];
		const selectedIds =
			snapshot?.blockIds ?? (holder ? selectedBlockIds(holder, range) : []);
		const current = editor.blocks.getCurrentBlockIndex();
		const currentBlock =
			selectedIds.length === 0 && current >= 0
				? editor.blocks.getBlockByIndex(current)
				: null;
		const blockIds =
			selectedIds.length > 0
				? selectedIds
				: currentBlock
					? [currentBlock.id]
					: [];
		const blocks = blockIds
			.map((id) => editor.blocks.getById(id))
			.filter((block) => block !== null);
		const cellBlockIds = new Set(
			tableCells
				.map((cell) => cell.closest<HTMLElement>(".ce-block")?.dataset.id)
				.filter((id): id is string => !!id),
		);
		const unsupported = blocks.filter(
			(block) =>
				!isAlignableEditorBlockType(block.name) &&
				!(block.name === "table" && cellBlockIds.has(block.id)),
		);
		if (unsupported.length > 0) {
			const containsImage = unsupported.some((block) => block.name === "image");
			toast.warning(
				containsImage
					? "選取內容包含圖片；圖片不支援文字對齊，這次未套用任何變更"
					: "請選取文字、清單或表格儲存格；這次未套用任何變更",
				{ toasterId },
			);
			return;
		}
		const alignableBlocks = blocks.filter((block) =>
			isAlignableEditorBlockType(block.name),
		);
		if (alignableBlocks.length === 0 && tableCells.length === 0) {
			toast.info("請先選取文字、清單或表格儲存格", { toasterId });
			return;
		}

		markHistoryActionBoundary();
		editorMutationBatchRef.current = true;
		let updateFailed = false;
		try {
			applyAlignmentToTableCells(tableCells, alignment);
			for (const block of alignableBlocks) {
				try {
					await editor.blocks.update(block.id, undefined, {
						align: { alignment },
					});
				} catch {
					updateFailed = true;
				}
			}
			const data = await safeSaveEditor(editor);
			commitEditorValue(fromEditorData(data));
		} finally {
			editorMutationBatchRef.current = false;
		}
		if (updateFailed) {
			toast.error("部分內容無法套用對齊，請重新選取後再試", { toasterId });
		}
		if (alignableBlocks.length === 1 && tableCells.length === 0) {
			const index = editor.blocks.getBlockIndex(alignableBlocks[0].id);
			if (index >= 0) editor.caret.setToBlock(index, "end");
		}
	};

	const insertImage = () => {
		const editor = editorRef.current;
		if (!editor) return;
		const currentIndex = editor.blocks.getCurrentBlockIndex();
		const insertAt = currentIndex >= 0 ? currentIndex + 1 : undefined;
		editor.blocks.insert("image", undefined, undefined, insertAt, true);
	};

	const insertTable = (rows: number, cols: number) => {
		const editor = editorRef.current;
		if (!editor) return;
		const currentIndex = editor.blocks.getCurrentBlockIndex();
		const insertAt = currentIndex >= 0 ? currentIndex + 1 : undefined;
		// Table tool sizes itself from `content`'s dimensions (see
		// computeInitialSize in @editorjs/table) — rows/cols in `config` are
		// only a fallback for when no content is given.
		const content = Array.from({ length: rows }, () =>
			Array.from({ length: cols }, () => ""),
		);
		editor.blocks.insert("table", { content }, undefined, insertAt, true);
	};

	const applyList = async (ordered: boolean) => {
		const editor = editorRef.current;
		if (!editor) return;
		const holder = holderRef.current;
		const snapshot = consumeToolbarSelection();
		const range = !snapshot && holder ? activeSelectionRange(holder) : null;
		const tableCells =
			snapshot && holder
				? resolveTableCells(holder, snapshot.tableCells)
				: undefined;
		if (tableCells && tableCells.length > 0) {
			tableCells.forEach((cell, index) => {
				const contentRoot = tableCellContentRoot(cell);
				contentRoot.innerHTML = tableCellHtmlWithListMarker(
					contentRoot.innerHTML,
					ordered,
					index,
				);
				cell.dispatchEvent(new InputEvent("input", { bubbles: true }));
			});
			return;
		}
		if (
			!snapshot &&
			holder &&
			applyListMarkersToSelectedCells(holder, range, ordered)
		) {
			return;
		}

		const saved = await safeSaveEditor(editor);
		const selected = snapshot
			? snapshot.blockIds
					.map((id) => editor.blocks.getBlockIndex(id))
					.filter((index) => index >= 0)
			: holder
				? selectedBlockIndexes(holder, range)
				: [];
		const current = editor.blocks.getCurrentBlockIndex();
		const indexes =
			selected.length > 0 ? selected : current >= 0 ? [current] : [];
		if (indexes.length === 0) {
			toast.info("請先選取段落或表格儲存格", { toasterId });
			return;
		}
		// Saver omits empty/invalid blocks, so its positions are not DOM indexes.
		// Preflight the actual selected tools even when a block was not saved.
		const targetBlocks = indexes.map((index) =>
			editor.blocks.getBlockByIndex(index),
		);
		const savedById = new Map(saved.blocks.map((block) => [block.id, block]));
		const blocks = targetBlocks.flatMap((block) => {
			const savedBlock = block ? savedById.get(block.id) : undefined;
			return savedBlock ? [savedBlock] : [];
		});
		const canReplace = targetBlocks.every(
			(block) => block && isListConvertibleEditorBlock({ type: block.name }),
		);
		if (canReplace) {
			const rawItems = editorBlocksToListItems(blocks);
			const cleaned = cleanupDescriptionEditorData({
				blocks: [
					{
						type: "list",
						data: { style: ordered ? "ordered" : "unordered", items: rawItems },
					},
				],
			});
			const items = cleaned.data.blocks[0]?.data.items ?? [];
			const first = indexes[0];
			if (indexes.length === 1 && blocks[0]?.type === "list") {
				const block = editor.blocks.getBlockByIndex(first);
				if (block) {
					await editor.blocks.update(
						block.id,
						{
							style: ordered ? "ordered" : "unordered",
							items,
						},
						blocks[0].tunes,
					);
					editor.caret.setToBlock(first, "end");
					return;
				}
			}
			for (const index of [...indexes].sort((a, b) => b - a)) {
				editor.blocks.delete(index);
			}
			const inserted = editor.blocks.insert(
				"list",
				{ style: ordered ? "ordered" : "unordered", items },
				undefined,
				first,
				true,
			);
			// One paragraph keeps its alignment when converted into a list.
			if (blocks.length === 1 && blocks[0].tunes) {
				await editor.blocks.update(inserted.id, undefined, blocks[0].tunes);
				editor.caret.setToBlock(first, "end");
			}
			return;
		}
		if (selected.length > 0) {
			toast.warning("選取內容包含圖片或整張表格，請改選文字段落或儲存格", {
				toasterId,
			});
			return;
		}

		const insertAt = current >= 0 ? current + 1 : undefined;
		editor.blocks.insert(
			"list",
			{ style: ordered ? "ordered" : "unordered", items: [] },
			undefined,
			insertAt,
			true,
		);
	};

	const insertList = async (ordered: boolean) => {
		const editor = editorRef.current;
		if (!editor) return;
		editorMutationBatchRef.current = true;
		try {
			checkpointHistory(fromEditorData(await safeSaveEditor(editor)));
			markHistoryActionBoundary();
			await applyList(ordered);
			commitEditorValue(fromEditorData(await safeSaveEditor(editor)));
			rememberInlineSelection();
		} catch {
			toast.error("清單格式未完成，請重新選取後再試", { toasterId });
		} finally {
			editorMutationBatchRef.current = false;
		}
	};

	/* ---- 放大編輯：外殼是常駐的 <dialog open>（inline 顯示），放大時
	 *      close()→showModal() 原地抬進瀏覽器 top layer — 不重掛 holder，
	 *      Editor.js 實例與內容原封不動，且不受任何祖先 stacking context
	 *      （view-transition-name / transform）壓制。Esc（native cancel）
	 *      與 ::backdrop 點擊收合。 ---- */
	const [expanded, setExpanded] = React.useState(false);

	React.useEffect(() => {
		const dialog = dialogRef.current;
		if (!expanded || !dialog) return;
		// showModal() 只接受 closed 起手；close→showModal 同一 tick 完成，
		// 中間不會 paint，DOM 不動。
		dialog.close();
		dialog.showModal();
		const release = retainOverlayOpenAttribute();
		const releaseScrollLock = acquireBodyScrollLock();
		// ::backdrop 點擊收合：backdrop 的 click target 是 <dialog> 本身
		//（padding 0，內容佔滿面板，點到內容時 target 都是子節點）。
		const onBackdropClick = (e: MouseEvent) => {
			if (e.target === dialog) setExpanded(false);
		};
		dialog.addEventListener("click", onBackdropClick);
		return () => {
			dialog.removeEventListener("click", onBackdropClick);
			release();
			releaseScrollLock();
			// 退出 top layer、回到 inline 顯示（open 需與 JSX 的 open prop 對齊）。
			if (dialog.open) dialog.close();
			dialog.setAttribute("open", "");
		};
	}, [expanded]);

	return (
		<dialog
			ref={dialogRef}
			open
			className={`desc-editorjs ${styles.editorShell} ${
				expanded ? styles.editorShell_expanded : ""
			}`}
			data-ready={ready ? "true" : "false"}
			data-preview={preview ? "true" : "false"}
			aria-busy={busy || !ready}
			aria-label="商品描述編輯器"
			onCancel={(e) => {
				// 攔下 Esc 的自動關閉，統一走 expanded state → effect cleanup
				// 收合（避免 dialog 短暫 closed 造成一幀消失）。
				e.preventDefault();
				setExpanded(false);
			}}
		>
			<AppToaster id={toasterId} containerAriaLabel="商品描述編輯器通知" />
			<div className={styles.editorHeader}>
				<strong>商品描述</strong>
				<div className={styles.headerActions}>
					<button
						type="button"
						className={styles.toolbarButton}
						disabled={!ready || busy || preview}
						onClick={tidyFormatting}
						title="整理 Markdown 標題、重複清單符號與空白項目；可復原"
					>
						整理格式
					</button>
					<button
						type="button"
						className={styles.toolbarButton}
						disabled={!ready || busy}
						aria-pressed={preview}
						onClick={togglePreview}
					>
						{preview ? "繼續編輯" : "閱讀預覽"}
					</button>
					<button
						type="button"
						className={styles.toolbarButton}
						aria-label={expanded ? "縮小編輯區" : "放大編輯"}
						aria-expanded={expanded}
						onClick={() => setExpanded((v) => !v)}
					>
						<AppIcon name="scan" size={18} />
						{expanded ? "縮小" : "放大編輯"}
					</button>
				</div>
			</div>
			<div hidden={preview} className={styles.toolbarDock}>
				<DescToolbar
					ready={ready && !busy}
					format={format}
					dialogRef={dialogRef}
					undoCount={historyState.undoCount}
					redoCount={historyState.redoCount}
					onBeforeAction={captureToolbarSelection}
					onUndo={undo}
					onRedo={redo}
					onBlock={convertCurrent}
					onInline={applyInline}
					onColor={applyColor}
					onAlign={applyAlign}
					onImage={insertImage}
					onInsertTable={insertTable}
					onList={insertList}
				/>
			</div>
			{!ready && (
				<p className={styles.editorStatus} role="status">
					{loadError
						? "編輯器載入失敗，請重新整理頁面後再試。"
						: "正在載入編輯器…"}
				</p>
			)}
			<section
				className={styles.canvas}
				aria-label={preview ? "商品描述閱讀預覽" : "商品描述編輯內容"}
			>
				<div ref={holderRef} className={styles.holder} />
			</section>
			<div className={styles.editorFooter}>
				<span>
					{preview
						? "閱讀預覽 · 檢查內容與排版"
						: "可直接貼上文字、圖片或試算表；空白段落支援 Markdown 貼上。"}
				</span>
				<span>完成後請儲存商品</span>
			</div>
		</dialog>
	);
}

/* ============================================================
 *                 Top style toolbar
 * ============================================================ */

type EditorFormatState = {
	block: string;
	bold: boolean;
	italic: boolean;
	underline: boolean;
	color: string;
	alignment: Alignment;
	list: "ordered" | "unordered" | null;
};

function DescToolbar({
	ready,
	format,
	dialogRef,
	undoCount,
	redoCount,
	onBeforeAction,
	onUndo,
	onRedo,
	onBlock,
	onInline,
	onColor,
	onAlign,
	onImage,
	onInsertTable,
	onList,
}: {
	ready: boolean;
	format: EditorFormatState;
	dialogRef: React.RefObject<HTMLDialogElement | null>;
	undoCount: number;
	redoCount: number;
	onBeforeAction: () => void;
	onUndo: () => void;
	onRedo: () => void;
	onBlock: (type: string, data?: Record<string, unknown>) => void;
	onInline: (command: "bold" | "italic" | "underline") => void;
	onColor: (hex: string) => void;
	onAlign: (alignment: Alignment) => void;
	onImage: () => void;
	onInsertTable: (rows: number, cols: number) => void;
	onList: (ordered: boolean) => void;
}) {
	const [more, setMore] = React.useState(false);
	const moreId = React.useId();
	const keep = (e: React.MouseEvent) => {
		e.preventDefault();
		e.stopPropagation();
	};
	return (
		<div className={styles.toolbarRow}>
			<div
				role="toolbar"
				aria-label="文字樣式"
				className={styles.toolbar}
				onPointerDownCapture={(e) => {
					if ((e.target as HTMLElement).closest("button:not(:disabled)"))
						onBeforeAction();
				}}
				onKeyDownCapture={(e) => {
					if (e.key === "Enter" || e.key === " ") onBeforeAction();
					if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key))
						return;
					const buttons = Array.from(
						e.currentTarget.querySelectorAll<HTMLButtonElement>(
							"button:not(:disabled)",
						),
					).filter((b) => b.offsetParent !== null);
					const index = buttons.indexOf(e.target as HTMLButtonElement);
					if (index < 0) return;
					e.preventDefault();
					const next =
						e.key === "Home"
							? 0
							: e.key === "End"
								? buttons.length - 1
								: (index + (e.key === "ArrowRight" ? 1 : -1) + buttons.length) %
									buttons.length;
					buttons[next]?.focus();
				}}
			>
				<TbGroup label="復原 / 重做">
					<TbButton
						ready={ready && undoCount > 0}
						onMouseDown={keep}
						onClick={onUndo}
						title={
							undoCount > 0
								? `復原（⌘/Ctrl+Z，還可復原 ${Math.min(undoCount, DESCRIPTION_HISTORY_ACTION_LIMIT)} 步）`
								: "目前沒有可復原的動作"
						}
					>
						<AppIcon name="undo" size={18} />
					</TbButton>
					<TbButton
						ready={ready && redoCount > 0}
						onMouseDown={keep}
						onClick={onRedo}
						title={
							redoCount > 0
								? `重做（⌘/Ctrl+Shift+Z，還可重做 ${redoCount} 步）`
								: "目前沒有可重做的動作"
						}
					>
						<AppIcon name="redo" size={18} />
					</TbButton>
				</TbGroup>

				<TbDivider />

				<TbGroup label="段落樣式" variant="segmented">
					<TbButton
						ready={ready}
						variant="segment"
						onMouseDown={keep}
						onClick={() => onBlock("paragraph")}
						active={format.block === "paragraph" && !format.list}
						title="內文"
					>
						內文
					</TbButton>
					<TbButton
						ready={ready}
						variant="segment"
						onMouseDown={keep}
						onClick={() => onBlock("header", { level: 2 })}
						active={format.block === "header2"}
						title="大標（⌘⇧H）"
					>
						大標
					</TbButton>
					<TbButton
						ready={ready}
						variant="segment"
						onMouseDown={keep}
						onClick={() => onBlock("header", { level: 3 })}
						active={format.block === "header3"}
						title="小標"
					>
						小標
					</TbButton>
				</TbGroup>

				<TbDivider />

				<TbGroup label="文字格式">
					<TbButton
						ready={ready}
						onMouseDown={keep}
						onClick={() => onInline("bold")}
						active={format.bold}
						title="粗體（⌘B）"
					>
						<AppIcon name="bold" size={18} />
					</TbButton>
					<TbButton
						ready={ready}
						onMouseDown={keep}
						onClick={() => onInline("italic")}
						active={format.italic}
						title="斜體（⌘I）"
					>
						<AppIcon name="italic" size={18} />
					</TbButton>
					<TbButton
						ready={ready}
						onMouseDown={keep}
						onClick={() => onInline("underline")}
						active={format.underline}
						title="底線（⌘U）"
					>
						<AppIcon name="underline" size={18} />
					</TbButton>
				</TbGroup>

				<DescriptionColorPicker
					value={format.color}
					onApply={onColor}
					onOpen={onBeforeAction}
					disabled={!ready}
					portalContainer={dialogRef.current}
				/>

				<TbDivider />

				<TableInsertButton
					ready={ready}
					onMouseDown={keep}
					onInsertTable={onInsertTable}
					dialogRef={dialogRef}
				/>

				<TbButton
					ready={ready}
					onMouseDown={keep}
					onClick={() => onList(false)}
					active={format.list === "unordered"}
					title="套用項目符號（可多選段落或表格欄位）"
					variant="action"
				>
					<AppIcon name="list-bullet" size={20} />
				</TbButton>

				<TbButton
					ready={ready}
					onMouseDown={keep}
					onClick={() => onList(true)}
					active={format.list === "ordered"}
					title="套用編號清單（可多選段落或表格欄位）"
					variant="action"
				>
					<AppIcon name="list-number" size={20} />
				</TbButton>

				<TbButton
					ready={ready}
					onMouseDown={keep}
					onClick={onImage}
					title="插入或直接貼上圖片"
					variant="segment"
				>
					<ImagePlusIcon />
					圖片
				</TbButton>
				<button
					type="button"
					className={styles.toolbarButton}
					aria-expanded={more}
					aria-controls={moreId}
					onMouseDown={keep}
					onClick={() => setMore((v) => !v)}
				>
					更多格式 <span aria-hidden="true">{more ? "−" : "+"}</span>
				</button>
			</div>
			<div
				id={moreId}
				className={styles.advancedToolbar}
				hidden={!more}
				onPointerDownCapture={onBeforeAction}
				onKeyDownCapture={(e) => {
					if (e.key === "Enter" || e.key === " ") onBeforeAction();
				}}
			>
				<span className={styles.advancedLabel}>段落對齊</span>

				<TbGroup label="對齊">
					{ALIGN_OPTIONS.map((opt) => (
						<TbButton
							key={opt.value}
							ready={ready}
							onMouseDown={keep}
							onClick={() => onAlign(opt.value)}
							active={format.alignment === opt.value}
							title={opt.label}
						>
							<AppIcon name={opt.icon} size={18} />
						</TbButton>
					))}
				</TbGroup>
			</div>
		</div>
	);
}

function TbGroup({
	children,
	label,
	variant = "plain",
}: {
	children: React.ReactNode;
	label: string;
	variant?: "plain" | "segmented" | "swatches";
}) {
	return (
		<fieldset
			className={`${styles.toolbarGroup} ${styles[`toolbarGroup_${variant}`]}`}
		>
			<legend className={styles.toolbarGroupLabel}>{label}</legend>
			{children}
		</fieldset>
	);
}

function TbDivider() {
	return <span className={styles.toolbarDivider} aria-hidden="true" />;
}

function TbButton({
	ready,
	children,
	title,
	onClick,
	onMouseDown,
	variant = "icon",
	active,
}: {
	ready: boolean;
	children: React.ReactNode;
	title: string;
	onClick: () => void;
	onMouseDown: (e: React.MouseEvent) => void;
	variant?: "icon" | "segment" | "action";
	active?: boolean;
}) {
	return (
		<button
			type="button"
			title={title}
			aria-label={title}
			aria-pressed={active}
			disabled={!ready}
			onMouseDown={onMouseDown}
			onClick={onClick}
			className={`${styles.toolbarButton} ${styles[`toolbarButton_${variant}`]}`}
		>
			{children}
		</button>
	);
}

/* ---- 插入表格：Excel/Notion 式懸浮網格挑尺寸。左鍵按住表格鈕直接往下/
 *      往右拖，拉到網格邊緣會自動再長一行/列（上限 12×10），放開即插入
 *      該尺寸；純點一下則進 hover 模式（移動預覽、再點一格插入）。 ---- */
const TABLE_PICKER_START_ROWS = 6;
const TABLE_PICKER_START_COLS = 8;
const TABLE_PICKER_MAX_ROWS = 12;
const TABLE_PICKER_MAX_COLS = 10;

/** rows/cols = 目前顯示的網格大小（會隨拖曳長大）；row/col = 反白到哪格。 */
interface TablePickerSel {
	rows: number;
	cols: number;
	row: number;
	col: number;
}

const TABLE_PICKER_INITIAL_SEL: TablePickerSel = {
	rows: TABLE_PICKER_START_ROWS,
	cols: TABLE_PICKER_START_COLS,
	row: 0,
	col: 0,
};

function TableInsertButton({
	ready,
	onMouseDown,
	onInsertTable,
	dialogRef,
}: {
	ready: boolean;
	onMouseDown: (e: React.MouseEvent) => void;
	onInsertTable: (rows: number, cols: number) => void;
	dialogRef: React.RefObject<HTMLDialogElement | null>;
}) {
	const [open, setOpen] = React.useState(false);
	const [coords, setCoords] = React.useState<{
		top: number;
		left: number;
	} | null>(null);
	const [sel, setSel] = React.useState<TablePickerSel>(
		TABLE_PICKER_INITIAL_SEL,
	);
	// pointerup 的 document listener 需要當下的選取值，state 在 listener
	// closure 裡是 stale 的 — 用 ref 鏡像。
	const selRef = React.useRef(sel);
	const triggerRef = React.useRef<HTMLSpanElement>(null);
	const panelRef = React.useRef<HTMLDivElement>(null);
	const gridRef = React.useRef<HTMLDivElement>(null);
	// 按住左鍵從表格鈕拖進網格的 drag session；entered = 指標進過網格
	//（純點擊不會 entered，放開後留在 hover 模式）。
	const dragRef = React.useRef<{ pointerId: number; entered: boolean } | null>(
		null,
	);
	// pointerdown 已處理過開/關 → 隨後補發的 click 不要再 toggle 一次。
	const skipClickRef = React.useRef(false);

	const hoverCell = React.useCallback((row: number, col: number) => {
		setSel((prev) => {
			const r = Math.max(0, Math.min(row, TABLE_PICKER_MAX_ROWS - 1));
			const c = Math.max(0, Math.min(col, TABLE_PICKER_MAX_COLS - 1));
			const next: TablePickerSel = {
				// 反白到最後一行/列時自動再長一行/列，永遠留一格可以繼續拉
				rows: Math.min(Math.max(prev.rows, r + 2), TABLE_PICKER_MAX_ROWS),
				cols: Math.min(Math.max(prev.cols, c + 2), TABLE_PICKER_MAX_COLS),
				row: r,
				col: c,
			};
			if (
				next.rows === prev.rows &&
				next.cols === prev.cols &&
				next.row === prev.row &&
				next.col === prev.col
			) {
				return prev;
			}
			selRef.current = next;
			return next;
		});
	}, []);

	const close = React.useCallback(() => {
		dragRef.current = null;
		setOpen(false);
		setCoords(null);
	}, []);

	const openPicker = React.useCallback(() => {
		const rect = triggerRef.current?.getBoundingClientRect();
		if (!rect) return;
		selRef.current = TABLE_PICKER_INITIAL_SEL;
		setSel(TABLE_PICKER_INITIAL_SEL);
		setCoords({ top: rect.bottom + 6, left: rect.left });
		setOpen(true);
	}, []);

	const commit = React.useCallback(() => {
		const s = selRef.current;
		onInsertTable(s.row + 1, s.col + 1);
		close();
	}, [close, onInsertTable]);

	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") close();
		};
		const onDocMouseDown = (e: MouseEvent) => {
			const target = e.target as Node | null;
			if (triggerRef.current?.contains(target)) return;
			if (panelRef.current?.contains(target)) return;
			close();
		};
		document.addEventListener("keydown", onKey);
		document.addEventListener("mousedown", onDocMouseDown);
		window.addEventListener("scroll", close, true);
		window.addEventListener("resize", close);
		return () => {
			document.removeEventListener("keydown", onKey);
			document.removeEventListener("mousedown", onDocMouseDown);
			window.removeEventListener("scroll", close, true);
			window.removeEventListener("resize", close);
		};
	}, [close, open]);

	// 拖曳選尺寸：pointer 座標對網格 rect 換算成 row/col（不是靠 cell 的
	// mouseenter），所以指標拖出網格右/下邊界照樣算成最遠那格 → 觸發長大。
	React.useEffect(() => {
		if (!open) return;
		const onPointerMove = (e: PointerEvent) => {
			const session = dragRef.current;
			if (!session || e.pointerId !== session.pointerId) return;
			const grid = gridRef.current;
			if (!grid) return;
			const rect = grid.getBoundingClientRect();
			const cell = grid.firstElementChild?.getBoundingClientRect();
			const gridStyle = window.getComputedStyle(grid);
			const pitchX =
				(cell?.width || 20) + (Number.parseFloat(gridStyle.columnGap) || 3);
			const pitchY =
				(cell?.height || 20) + (Number.parseFloat(gridStyle.rowGap) || 3);
			const x = e.clientX - rect.left;
			const y = e.clientY - rect.top;
			// 還在網格左/上方（多半仍停在觸發鈕上）就先不選
			if (x < 0 || y < 0) return;
			session.entered = true;
			hoverCell(Math.floor(y / pitchY), Math.floor(x / pitchX));
		};
		const endSession = () => {
			dragRef.current = null;
			// 若放開的位置回到觸發鈕上，隨後的 click 先讓 skip 旗標吞掉
			window.setTimeout(() => {
				skipClickRef.current = false;
			}, 0);
		};
		const onPointerUp = (e: PointerEvent) => {
			const session = dragRef.current;
			if (!session || e.pointerId !== session.pointerId) return;
			const entered = session.entered;
			endSession();
			// 拖進網格後放開 = 插入；純點擊（沒進過網格）則留在 hover 模式
			if (entered) commit();
		};
		const onPointerCancel = (e: PointerEvent) => {
			if (dragRef.current?.pointerId !== e.pointerId) return;
			endSession();
		};
		document.addEventListener("pointermove", onPointerMove);
		document.addEventListener("pointerup", onPointerUp);
		document.addEventListener("pointercancel", onPointerCancel);
		return () => {
			document.removeEventListener("pointermove", onPointerMove);
			document.removeEventListener("pointerup", onPointerUp);
			document.removeEventListener("pointercancel", onPointerCancel);
		};
	}, [open, commit, hoverCell]);

	return (
		<>
			<span
				ref={triggerRef}
				className={styles.tableTriggerAnchor}
				data-desc-table-trigger
				onPointerDown={(e) => {
					if (!ready || e.button !== 0) return;
					skipClickRef.current = true;
					if (open) {
						close();
						return;
					}
					openPicker();
					dragRef.current = { pointerId: e.pointerId, entered: false };
				}}
			>
				<TbButton
					ready={ready}
					onMouseDown={onMouseDown}
					onClick={() => {
						// 滑鼠路徑已在 pointerdown 處理；這裡只收鍵盤觸發的 click。
						if (skipClickRef.current) {
							skipClickRef.current = false;
							return;
						}
						if (open) close();
						else openPicker();
					}}
					title="插入表格"
					variant="segment"
				>
					<AppIcon name="table" size={20} />
					表格
				</TbButton>
			</span>

			{open && coords && dialogRef.current
				? createPortal(
						<TableSizePicker
							panelRef={panelRef}
							gridRef={gridRef}
							top={coords.top}
							left={coords.left}
							sel={sel}
							onHoverCell={hoverCell}
							onPickCell={commit}
						/>,
						dialogRef.current,
					)
				: null}
		</>
	);
}

function TableSizePicker({
	panelRef,
	gridRef,
	top,
	left,
	sel,
	onHoverCell,
	onPickCell,
}: {
	panelRef: React.RefObject<HTMLDivElement | null>;
	gridRef: React.RefObject<HTMLDivElement | null>;
	top: number;
	left: number;
	sel: TablePickerSel;
	onHoverCell: (row: number, col: number) => void;
	onPickCell: () => void;
}) {
	return (
		<div
			ref={panelRef}
			className={styles.tablePicker}
			role="dialog"
			aria-label="選擇表格大小"
			style={
				{
					"--desc-editor-table-picker-top": `${top}px`,
					"--desc-editor-table-picker-left": `${left}px`,
				} as React.CSSProperties
			}
		>
			<div className={styles.tablePickerLabel}>
				{sel.row + 1} × {sel.col + 1}
			</div>
			<div
				ref={gridRef}
				className={styles.tablePickerGrid}
				style={
					{
						"--desc-editor-table-picker-cols": sel.cols,
					} as React.CSSProperties
				}
			>
				{Array.from({ length: sel.rows }, (_, row) =>
					Array.from({ length: sel.cols }, (_, col) => (
						<button
							// biome-ignore lint/suspicious/noArrayIndexKey: fixed-size grid
							key={`${row}-${col}`}
							type="button"
							className={styles.tablePickerCell}
							data-active={row <= sel.row && col <= sel.col}
							onMouseEnter={() => onHoverCell(row, col)}
							onFocus={() => onHoverCell(row, col)}
							onClick={onPickCell}
							aria-label={`插入 ${row + 1} × ${col + 1} 表格`}
						/>
					)),
				)}
			</div>
		</div>
	);
}

function ImagePlusIcon() {
	return <AppIcon name="image" size={24} />;
}
