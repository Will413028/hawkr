export interface DescriptionEditorToolbarGeometry {
	anchorTop: number;
	anchorHeight: number;
	parentTop: number;
	parentClientTop: number;
	paddingTop: number;
	isEditable: boolean;
	lineHeight: number;
	actionHeight: number;
}

/**
 * Resolve the Editor.js toolbar's local Y coordinate for our mobile block rail.
 * Kept DOM-free so the paragraph, table, list, and media alignment math can be
 * covered without booting the editor runtime.
 */
export function descriptionEditorMobileToolbarTop({
	anchorTop,
	anchorHeight,
	parentTop,
	parentClientTop,
	paddingTop,
	isEditable,
	lineHeight,
	actionHeight,
}: DescriptionEditorToolbarGeometry): number {
	const resolvedLineHeight =
		Number.isFinite(lineHeight) && lineHeight > 0
			? lineHeight
			: Math.min(anchorHeight, actionHeight);
	const contentTop = anchorTop + (isEditable ? paddingTop : 0);

	return Math.max(
		0,
		Math.round(
			contentTop -
				(parentTop + parentClientTop) +
				(resolvedLineHeight - actionHeight) / 2,
		),
	);
}
