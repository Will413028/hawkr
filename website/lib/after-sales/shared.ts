import { apiClient } from "../api/client";
import { normalizeCase } from "./normalize";
import type { AfterSaleListResult } from "./types";

interface ListWire {
	records?: Record<string, unknown>[];
	list?: Record<string, unknown>[];
	total?: number | string;
}

/** 共用的 after-sale 列表抓取：soft-fail 404 到空列表（給端點還沒上線的環境
 * 一個可渲染的初始狀態），並解包 `records`/`list` 兩種欄位命名。買/賣家兩邊
 * 唯一的差異只有 endpoint 字串跟預設 pageSize，都由呼叫端決定。 */
export async function fetchAfterSaleList(
	endpoint: string,
	params: { page?: number; pageSize?: number; status?: string } = {},
): Promise<AfterSaleListResult> {
	const { page = 1, pageSize = 50, status } = params;
	try {
		const env = await apiClient.authed.get<ListWire>(endpoint, {
			query: { page, page_size: pageSize, status: status ?? undefined },
		});
		const list = env.data?.records ?? env.data?.list ?? [];
		return {
			records: list.map(normalizeCase),
			total: Number(env.data?.total ?? list.length),
		};
	} catch (e) {
		if (e instanceof Error && /\b404\b/.test(e.message)) {
			return { records: [], total: 0 };
		}
		throw e;
	}
}
