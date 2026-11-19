import { beforeEach, describe, expect, it, vi } from "vitest";

const cached = vi.hoisted(() => ({ getCachedStore: vi.fn() }));
vi.mock("@/lib/api/cached", () => cached);
vi.mock("@/lib/forum/repo", () => ({}));
vi.mock("@/features/sellers/components/SellerMessageTab", () => ({
	default: () => null,
}));

import { generateMetadata as thread } from "./SellerForumThreadPage";
import { generateMetadata as tab } from "./SellerMessagesPage";

const pages = { tab, thread };
const cases = [
	{ locale: "zh-TW", title: "小店 的留言", missing: "找不到賣家" },
	{ locale: "en", title: "小店's messages", missing: "Seller not found" },
];

beforeEach(() => cached.getCachedStore.mockReset());

describe.each(Object.entries(pages))("%s page metadata", (_n, gen) => {
	it.each(cases)("$locale title and not-found", async (c) => {
		const params = Promise.resolve({
			locale: c.locale,
			id: "shop",
			threadId: "t",
		});
		cached.getCachedStore.mockResolvedValue({
			name: "小店",
			storeCode: "shop",
			storeId: "7",
		});
		expect((await gen({ params })).title).toBe(c.title);
		cached.getCachedStore.mockResolvedValue(undefined);
		expect((await gen({ params })).title).toBe(c.missing);
	});
});
