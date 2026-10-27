import type { PostListItem } from "@/lib/api/openapi-types";

export interface FeedHeadPage {
	records: PostListItem[];
	page: number;
	pageSize: number;
	hasMore: boolean;
}

function dedupe(records: PostListItem[]) {
	const seen = new Set<string>();
	return records.filter((record) => {
		const id = String(record.id);
		if (seen.has(id)) return false;
		seen.add(id);
		return true;
	});
}

function withFreshEngagement(
	existing: PostListItem,
	fresh: PostListItem,
): PostListItem {
	return {
		...existing,
		commentCount: fresh.commentCount,
		likeCount: fresh.likeCount,
		shareCount: fresh.shareCount,
		viewCount: fresh.viewCount,
	};
}

function presentationFingerprint(record: PostListItem): string {
	const {
		commentCount: _commentCount,
		isLiked: _isLiked,
		likeCount: _likeCount,
		shareCount: _shareCount,
		viewCount: _viewCount,
		...presentation
	} = record;
	return JSON.stringify(presentation);
}

export function feedHeadRequiresReplacement(
	existingRecords: PostListItem[],
	freshRecords: PostListItem[],
): boolean {
	if (existingRecords.length !== freshRecords.length) return true;
	return existingRecords.some((record, index) => {
		const fresh = freshRecords[index];
		return (
			String(record.id) !== String(fresh?.id) ||
			presentationFingerprint(record) !== presentationFingerprint(fresh)
		);
	});
}

/**
 * Refreshes visible copies of page-one records without re-fetching every
 * infinite page. New ids are returned as a pending buffer. They are prepended
 * only when the caller knows doing so will not move a reader's scroll anchor.
 */
export function mergeFeedHeadPages(
	pages: FeedHeadPage[],
	freshRecords: PostListItem[],
	prependNew: boolean,
): { pages: FeedHeadPage[]; newRecords: PostListItem[] } {
	const existingIds = new Set(
		pages.flatMap((page) => page.records.map((record) => String(record.id))),
	);
	const latestById = new Map(
		freshRecords.map((record) => [String(record.id), record] as const),
	);
	const updatedPages = pages.map((page) => ({
		...page,
		records: page.records.map((record) => {
			const fresh = latestById.get(String(record.id));
			if (!fresh) return record;
			// While a reader is away from the top, content/image edits are
			// buffered with the replacement pill. Only fixed-size counters
			// reconcile immediately, so masonry heights and scroll anchors
			// cannot jump under their finger.
			return prependNew ? fresh : withFreshEngagement(record, fresh);
		}),
	}));
	const newRecords = dedupe(
		freshRecords.filter((record) => !existingIds.has(String(record.id))),
	);
	if (!prependNew || newRecords.length === 0 || updatedPages.length === 0) {
		return { pages: updatedPages, newRecords };
	}

	// Do not force the newly prepended ids back into the old fixed page
	// cardinality. Dropping the tail would create a pagination hole because the
	// next cursor still starts after the last page the reader already loaded.
	// A slightly larger page one is safe; every previously rendered item and all
	// page/cursor metadata remain intact.
	return {
		pages: updatedPages.map((page, index) =>
			index === 0
				? { ...page, records: dedupe([...newRecords, ...page.records]) }
				: page,
		),
		newRecords,
	};
}
