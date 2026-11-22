import { describe, expect, it, vi } from "vitest";
import { blockPeerByMessage } from "../block";

function clients() {
	const buyer = { blockAuthor: vi.fn().mockResolvedValue({ changed: true }) };
	const seller = { blockBuyer: vi.fn().mockResolvedValue({ changed: false }) };
	return {
		buyer,
		seller,
		deps: {
			buyer: () => buyer as never,
			seller: () => seller as never,
		},
	};
}

describe("blockPeerByMessage", () => {
	it("buyer side blocks the store through BlockAuthor with the message id", async () => {
		const c = clients();
		const res = await blockPeerByMessage("buyer", "m1", c.deps);
		expect(c.buyer.blockAuthor).toHaveBeenCalledWith({
			source: { case: "messageId", value: "m1" },
		});
		expect(c.seller.blockBuyer).not.toHaveBeenCalled();
		expect(res).toEqual({ changed: true });
	});

	it("store side blocks the buyer through BlockBuyer with the message id", async () => {
		const c = clients();
		const res = await blockPeerByMessage("store", "m2", c.deps);
		expect(c.seller.blockBuyer).toHaveBeenCalledWith({
			source: { case: "messageId", value: "m2" },
		});
		expect(c.buyer.blockAuthor).not.toHaveBeenCalled();
		expect(res).toEqual({ changed: false });
	});
});
