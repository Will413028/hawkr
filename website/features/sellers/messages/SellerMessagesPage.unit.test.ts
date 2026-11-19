import { beforeEach, describe, expect, it, vi } from "vitest";

const cached = vi.hoisted(() => ({ getCachedStore: vi.fn() }));
const forum = vi.hoisted(() => ({ fetchSellerForum: vi.fn() }));
vi.mock("@/lib/api/cached", () => cached);
vi.mock("@/lib/forum/repo", () => forum);
vi.mock("@/features/sellers/components/SellerMessageTab", () => ({
	default: () => null,
}));

import MessagesPage from "./SellerMessagesPage";

type Element = { props: Record<string, unknown> };

function find(node: unknown): Element | undefined {
	if (!node || typeof node !== "object") return undefined;
	const el = node as Element;
	if (el.props && "routeId" in el.props) return el;
	return el.props ? find(el.props.children) : undefined;
}

beforeEach(() => {
	forum.fetchSellerForum.mockReset().mockResolvedValue({ comments: [] });
	cached.getCachedStore
		.mockReset()
		.mockResolvedValue({ storeId: "7", storeCode: "shop-code" });
});

describe("store messages tab keys the forum by store id, not the URL store code", () => {
	it("reads and hands the tab the store id while keeping the route segment for login return", async () => {
		const tree = await MessagesPage({
			params: Promise.resolve({ locale: "zh-TW", id: "shop-code" }),
		});
		expect(forum.fetchSellerForum).toHaveBeenCalledWith("7", expect.anything());
		const tab = find(tree);
		expect(tab?.props.sellerId).toBe("7");
		expect(tab?.props.routeId).toBe("shop-code");
	});
});
