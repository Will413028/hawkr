import type { OutputBlockData } from "@editorjs/editorjs";

export const DESCRIPTION_HISTORY_ACTION_LIMIT = 20;
export const DESCRIPTION_HISTORY_TYPING_WINDOW_MS = 1400;

export type DescriptionHistoryInputKind = "insert" | "delete" | "other";

export interface DescriptionHistoryInputBatch {
	kind: DescriptionHistoryInputKind;
	blockKey: string;
	timestamp: number;
	breakAfter: boolean;
}

const TEXT_INPUT_TYPES = new Set([
	"insertText",
	"insertCompositionText",
	"insertFromComposition",
]);

/**
 * Word-like history grouping: consecutive letters in the same block merge,
 * while a pause, a word-ending character, delete-vs-insert switch, paste,
 * enter, formatting, or a different block starts a new undo action.
 */
export function descriptionHistoryInputBatch(
	inputType: string,
	data: string | null,
	blockKey: string,
	timestamp: number,
	previous: DescriptionHistoryInputBatch | null,
): { forceNewAction: boolean; next: DescriptionHistoryInputBatch } {
	const kind: DescriptionHistoryInputKind = inputType.startsWith("delete")
		? "delete"
		: TEXT_INPUT_TYPES.has(inputType)
			? "insert"
			: "other";
	const forceNewAction =
		!previous ||
		kind === "other" ||
		previous.kind !== kind ||
		previous.blockKey !== blockKey ||
		previous.breakAfter ||
		timestamp - previous.timestamp >= DESCRIPTION_HISTORY_TYPING_WINDOW_MS;
	const breakAfter =
		kind === "other" ||
		(kind === "insert" && /[\s，。！？、；：,.!?;:]$/.test(data ?? ""));
	return {
		forceNewAction,
		next: { kind, blockKey, timestamp, breakAfter },
	};
}

export interface EditorBlockSelectionState {
	isEditorSelected: boolean;
	intersectsNativeRange: boolean;
}

export interface EditorBlockSelectionCandidate
	extends EditorBlockSelectionState {
	id: string;
}

/**
 * Editor.js replaces a native cross-block Range with its own
 * `.ce-block--selected` state. Toolbar actions must accept either source so a
 * drag-selection does not fall back to the current block after Editor.js has
 * removed the browser Range.
 */
export function collectSelectedEditorBlockIndexes(
	blocks: ReadonlyArray<EditorBlockSelectionState>,
): number[] {
	return blocks
		.map((block, index) =>
			block.isEditorSelected || block.intersectsNativeRange ? index : -1,
		)
		.filter((index) => index >= 0);
}

/** Stable-id variant used by toolbar actions. Editor.js clears its native
 * Range before an outside toolbar receives `mousedown`, so indexes captured
 * later are both lossy and vulnerable to intervening block moves. */
export function collectSelectedEditorBlockIds(
	blocks: ReadonlyArray<EditorBlockSelectionCandidate>,
): string[] {
	return blocks
		.filter((block) => block.isEditorSelected || block.intersectsNativeRange)
		.map((block) => block.id)
		.filter((id) => id.length > 0);
}

export function isAlignableEditorBlockType(type: string): boolean {
	return type === "paragraph" || type === "header" || type === "list";
}

export function descriptionHistoryDirectionForInputType(
	inputType: string,
): -1 | 1 | null {
	if (inputType === "historyUndo") return -1;
	if (inputType === "historyRedo") return 1;
	return null;
}

/** Prefix a table cell with the requested list marker, replacing any marker
 * previously applied by the toolbar so bullet ↔ number switches cleanly. */
export function tableCellHtmlWithListMarker(
	html: string,
	ordered: boolean,
	index: number,
): string {
	const clean = html.replace(/^(?:\s|&nbsp;)*(?:•|\d+\.)\s*/i, "");
	return `${ordered ? `${index + 1}. ` : "• "}${clean}`;
}

/** An undo stack needs one baseline snapshot in addition to the retained
 * actions. Keeping 21 snapshots therefore exposes exactly 20 undo steps. */
export function limitDescriptionHistory<T>(snapshots: T[]): T[] {
	const maxSnapshots = DESCRIPTION_HISTORY_ACTION_LIMIT + 1;
	return snapshots.length > maxSnapshots
		? snapshots.slice(-maxSnapshots)
		: snapshots;
}

/**
 * Turn Editor.js text/list blocks into the flat item payload expected by
 * `@editorjs/list`. This is kept DOM-free so selection-driven toolbar actions
 * can be regression-tested without mounting the editor.
 */
export function editorBlocksToListItems(
	blocks: ReadonlyArray<Pick<OutputBlockData, "type" | "data">>,
): string[] {
	const items: string[] = [];

	for (const block of blocks) {
		if (block.type === "list") {
			const rawItems = Array.isArray(block.data?.items)
				? (block.data.items as unknown[])
				: [];
			for (const raw of rawItems) {
				const html =
					typeof raw === "string"
						? raw
						: raw && typeof raw === "object" && "content" in raw
							? String((raw as { content?: unknown }).content ?? "")
							: "";
				if (hasVisibleContent(html)) items.push(html.trim());
			}
			continue;
		}

		if (block.type !== "paragraph" && block.type !== "header") continue;
		const html = String(block.data?.text ?? "");
		for (const line of html.split(/(?:<br\s*\/?>|\r?\n)+/gi)) {
			if (hasVisibleContent(line)) items.push(line.trim());
		}
	}

	return items;
}

export function isListConvertibleEditorBlock(
	block: Pick<OutputBlockData, "type">,
): boolean {
	return (
		block.type === "paragraph" ||
		block.type === "header" ||
		block.type === "list"
	);
}

function hasVisibleContent(html: string): boolean {
	return (
		html
			.replace(/<[^>]*>/g, "")
			.replace(/&nbsp;|&#160;/gi, " ")
			.trim().length > 0
	);
}
