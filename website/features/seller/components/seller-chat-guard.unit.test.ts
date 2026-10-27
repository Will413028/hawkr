import { create } from "@bufbuild/protobuf";
import { GetMyStoreResponseSchema } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import {
	decideSellerChatAction,
	isOwnSellerTarget,
	resolveSellerChatTargetId,
} from "./seller-chat-guard";

function myStore(id: string, storeCode = "") {
	return create(GetMyStoreResponseSchema, { store: { id, storeCode } });
}

const sellerSelf = myStore("2685914682", "2601234567");

describe("seller chat self-target guard", () => {
	it("uses only the internal store id as the chat room target", () => {
		expect(resolveSellerChatTargetId({ storeId: " 2685914682 " })).toBe(
			"2685914682",
		);
		expect(resolveSellerChatTargetId({})).toBeNull();
	});

	it("matches both the internal store id and the canonical store code", () => {
		expect(isOwnSellerTarget("2685914682", sellerSelf)).toBe(true);
		expect(isOwnSellerTarget(" 2601234567 ", sellerSelf)).toBe(true);
		expect(isOwnSellerTarget("2675237784", sellerSelf)).toBe(false);
	});

	it("compares large identifiers as exact strings without losing precision", () => {
		const largeStoreId = "9223372036854775807";
		expect(
			isOwnSellerTarget(largeStoreId, myStore(largeStoreId, "2601234567")),
		).toBe(true);
		expect(
			isOwnSellerTarget("9223372036854775806", myStore(largeStoreId)),
		).toBe(false);
	});

	it("keeps ordinary buyer-only accounts on the normal chat path", () => {
		expect(
			decideSellerChatAction({
				activeRole: "buyer",
				hasSellerRole: false,
				sellerProfileMissing: false,
				sellerSelf: null,
				sellerSelfLoading: false,
				targetSellerId: "2685914682",
			}),
		).toBe("open");
	});

	it("blocks a dual-role account from chatting with either id for its own store", () => {
		for (const targetSellerId of [
			sellerSelf.store?.id ?? "",
			sellerSelf.store?.storeCode ?? "",
		]) {
			expect(
				decideSellerChatAction({
					activeRole: "buyer",
					hasSellerRole: true,
					sellerProfileMissing: false,
					sellerSelf,
					sellerSelfLoading: false,
					targetSellerId,
				}),
			).toBe("own_store");
		}
	});

	it("allows a dual-role account to chat with another store", () => {
		expect(
			decideSellerChatAction({
				activeRole: "buyer",
				hasSellerRole: true,
				sellerProfileMissing: false,
				sellerSelf,
				sellerSelfLoading: false,
				targetSellerId: "2675237784",
			}),
		).toBe("open");
	});

	it("fails closed while a dual-role seller identity is unresolved", () => {
		expect(
			decideSellerChatAction({
				activeRole: "buyer",
				hasSellerRole: true,
				sellerProfileMissing: false,
				sellerSelf: null,
				sellerSelfLoading: true,
				targetSellerId: "2685914682",
			}),
		).toBe("checking_identity");

		expect(
			decideSellerChatAction({
				activeRole: "buyer",
				hasSellerRole: true,
				sellerProfileMissing: false,
				sellerSelf: null,
				sellerSelfLoading: false,
				targetSellerId: "2685914682",
			}),
		).toBe("identity_unavailable");
	});

	it("does not treat a missing seller profile as ownership of any store", () => {
		expect(
			decideSellerChatAction({
				activeRole: "buyer",
				hasSellerRole: true,
				sellerProfileMissing: true,
				sellerSelf: null,
				sellerSelfLoading: false,
				targetSellerId: "2685914682",
			}),
		).toBe("open");
	});

	it("still requires buyer view before any ownership decision", () => {
		expect(
			decideSellerChatAction({
				activeRole: "seller",
				hasSellerRole: true,
				sellerProfileMissing: false,
				sellerSelf,
				sellerSelfLoading: false,
				targetSellerId: "2675237784",
			}),
		).toBe("switch_role");
	});
});
