import type {
	InfiniteData,
	QueryClient,
	QueryKey,
} from "@tanstack/react-query";

export interface ListPage<T> {
	records: T[];
	/** 空字串是沒有下一頁。 */
	nextPageToken: string;
	pageSize: number;
}

export type ListData<T> = InfiniteData<ListPage<T>, string>;

export const FIRST_PAGE_PARAM: string = "";

export function nextPageParam(last: ListPage<unknown>): string | undefined {
	return last.nextPageToken === "" ? undefined : last.nextPageToken;
}

export function seedPages<T>(page: ListPage<T>): ListData<T> {
	return { pages: [page], pageParams: [FIRST_PAGE_PARAM] };
}

export function flattenPages<T>(
	data: InfiniteData<ListPage<T>, unknown> | undefined,
): T[] {
	return data ? data.pages.flatMap((page) => page.records) : [];
}

/** 最後一頁已載入：伺服器依建立時間舊到新排，新項目才會落在已載入的資料裡。 */
export function lastPageLoaded<T>(data: ListData<T> | undefined): boolean {
	if (!data || data.pages.length === 0) return false;
	return data.pages[data.pages.length - 1].nextPageToken === "";
}

export function appendToLastPage<T>(data: ListData<T>, item: T): ListData<T> {
	const last = data.pages.length - 1;
	return {
		...data,
		pages: data.pages.map((page, index) =>
			index === last ? { ...page, records: [...page.records, item] } : page,
		),
	};
}

export function mapItems<T>(
	data: ListData<T>,
	fn: (item: T) => T,
): ListData<T> {
	return {
		...data,
		pages: data.pages.map((page) => ({
			...page,
			records: page.records.map(fn),
		})),
	};
}

export function mapItem<T extends { id: string }>(
	data: ListData<T>,
	id: string,
	fn: (item: T) => T,
): ListData<T> {
	return mapItems(data, (item) => (item.id === id ? fn(item) : item));
}

export function removeItem<T extends { id: string }>(
	data: ListData<T>,
	id: string,
): ListData<T> {
	return {
		...data,
		pages: data.pages.map((page) => ({
			...page,
			records: page.records.filter((item) => item.id !== id),
		})),
	};
}

export interface ItemLocation {
	page: number;
	index: number;
}

export function locateItem<T extends { id: string }>(
	data: ListData<T> | undefined,
	id: string,
): ItemLocation | undefined {
	if (!data) return undefined;
	for (let page = 0; page < data.pages.length; page++) {
		const index = data.pages[page].records.findIndex((item) => item.id === id);
		if (index >= 0) return { page, index };
	}
	return undefined;
}

export function findItem<T extends { id: string }>(
	data: ListData<T> | undefined,
	id: string,
): T | undefined {
	const at = locateItem(data, id);
	return at && data ? data.pages[at.page].records[at.index] : undefined;
}

/** 把一個被移除的項目放回原本的頁與位置（位置超出時放到該頁尾）；已經在裡面就原地換回。 */
export function restoreItem<T extends { id: string }>(
	data: ListData<T>,
	item: T,
	at: ItemLocation,
): ListData<T> {
	if (locateItem(data, item.id)) return mapItem(data, item.id, () => item);
	if (data.pages.length === 0) return data;
	const pageIndex = Math.min(at.page, data.pages.length - 1);
	return {
		...data,
		pages: data.pages.map((page, i) => {
			if (i !== pageIndex) return page;
			const records = [...page.records];
			records.splice(Math.min(at.index, records.length), 0, item);
			return { ...page, records };
		}),
	};
}

interface Counted {
	id: string;
	state: string;
	replyCount?: number;
}

export function addReplyCount<T extends Counted>(
	data: ListData<T>,
	id: string,
	delta: number,
): ListData<T> {
	return mapItem(data, id, (item) => ({
		...item,
		replyCount: Math.max(0, (item.replyCount ?? 0) + delta),
	}));
}

/**
 * 刪掉一則回覆後減少所屬留言的回覆數；已刪除或隱藏的佔位項目只為了掛回覆而存在，
 * 回覆數歸零就跟著消失（ListComments、ListForumEntries 的契約）。
 */
export function dropReplyCount<T extends Counted>(
	data: ListData<T>,
	id: string,
): ListData<T> {
	const next = addReplyCount(data, id, -1);
	const parent = findItem(next, id);
	if (parent && parent.state !== "visible" && (parent.replyCount ?? 0) === 0) {
		return removeItem(next, id);
	}
	return next;
}

function fetchingMore(queryClient: QueryClient, key: QueryKey): boolean {
	const state = queryClient.getQueryState(key);
	if (state?.fetchStatus !== "fetching") return false;
	const meta = state.fetchMeta as { fetchMore?: unknown } | null;
	return Boolean(meta?.fetchMore);
}

/**
 * 樂觀更新前取消進行中的讀取，免得舊結果蓋掉樂觀資料。還沒有資料（首次載入）或正在載入更多時不取消：
 * 這兩種情況都不會插入樂觀項目（最後一頁不在已載入的資料裡），取消只會丟掉首頁或使用者要的那一頁。
 */
export async function cancelListRefetch(
	queryClient: QueryClient,
	key: QueryKey,
): Promise<void> {
	if (!queryClient.getQueryState(key)?.data) return;
	if (fetchingMore(queryClient, key)) return;
	await queryClient.cancelQueries({ queryKey: key });
}

/** 載入更多進行中時不打斷它（打斷會丟掉使用者要的那一頁）；沒載入的部分之後的讀取會補上。 */
export async function invalidateList(
	queryClient: QueryClient,
	key: QueryKey,
): Promise<void> {
	await queryClient.invalidateQueries(
		{ queryKey: key },
		{ cancelRefetch: !fetchingMore(queryClient, key) },
	);
}
