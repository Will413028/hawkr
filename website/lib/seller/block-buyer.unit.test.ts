import { describe, expect, it, vi } from "vitest";
import {
	blockBuyerByComment,
	blockBuyerByForumEntry,
	listBlockedBuyers,
} from "./block-buyer";

describe("blockBuyerByComment", () => {
	it("names the buyer by the comment id and reports whether the state changed", async () => {
		const blockBuyer = vi.fn().mockResolvedValue({ changed: true });
		const result = await blockBuyerByComment(
			"c-1",
			() => ({ blockBuyer }) as never,
		);
		expect(blockBuyer).toHaveBeenCalledExactlyOnceWith({
			source: { case: "commentId", value: "c-1" },
		});
		expect(result).toEqual({ changed: true });
	});

	it("an already blocked buyer is a success with changed false", async () => {
		const blockBuyer = vi.fn().mockResolvedValue({ changed: false });
		await expect(
			blockBuyerByComment("c-2", () => ({ blockBuyer }) as never),
		).resolves.toEqual({ changed: false });
	});

	it("lets the server's rejection reach the caller", async () => {
		const blockBuyer = vi.fn().mockRejectedValue(new Error("not_found"));
		await expect(
			blockBuyerByComment("c-3", () => ({ blockBuyer }) as never),
		).rejects.toThrow("not_found");
	});
});

describe("blockBuyerByForumEntry", () => {
	it("names the buyer by the forum entry id and reports whether the state changed", async () => {
		const blockBuyer = vi.fn().mockResolvedValue({ changed: true });
		const result = await blockBuyerByForumEntry(
			"f-1",
			() => ({ blockBuyer }) as never,
		);
		expect(blockBuyer).toHaveBeenCalledExactlyOnceWith({
			source: { case: "forumEntryId", value: "f-1" },
		});
		expect(result).toEqual({ changed: true });
	});
});

describe("listBlockedBuyers", () => {
	it("drops an entry without a buyer id: it cannot be unblocked", async () => {
		const listBlockedBuyers_ = vi.fn().mockResolvedValue({
			blockedBuyers: [
				{ buyer: { id: "11", displayName: "小明" } },
				{ buyer: undefined },
				{ buyer: { id: "", displayName: "沒有 id" } },
			],
			nextPageToken: "",
		});
		const page = await listBlockedBuyers(
			"",
			() => ({ listBlockedBuyers: listBlockedBuyers_ }) as never,
		);
		expect(page.items.map((i) => i.buyerId)).toEqual(["11"]);
	});
});
