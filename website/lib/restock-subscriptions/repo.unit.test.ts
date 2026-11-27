import { create } from "@bufbuild/protobuf";
import { RestockSubscriptionSchema } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({
	subscribeRestock: vi.fn(async () => ({})),
	unsubscribeRestock: vi.fn(async () => ({})),
}));
vi.mock("../connect/buyer", () => ({ buyerAppClient: () => client }));

import {
	restockFromConnect,
	subscribeRestock,
	unsubscribeRestock,
} from "./repo";

describe("restock subscriptions repo", () => {
	beforeEach(() => vi.clearAllMocks());

	it("subscribe sends product_id and sku_id", async () => {
		await subscribeRestock("11", "22");
		expect(client.subscribeRestock).toHaveBeenCalledWith({
			productId: "11",
			skuId: "22",
		});
	});

	it("unsubscribe sends sku_id only", async () => {
		await unsubscribeRestock("22");
		expect(client.unsubscribeRestock).toHaveBeenCalledWith({ skuId: "22" });
	});

	it("list mapping keys the subscription by sku_id", () => {
		const item = restockFromConnect(
			create(RestockSubscriptionSchema, {
				skuId: "22",
				product: { id: "11" },
			}),
		);
		expect(item.skuId).toBe("22");
		expect(item.subscriptionId).toBe("22");
		expect(item.productId).toBe("11");
	});
});
