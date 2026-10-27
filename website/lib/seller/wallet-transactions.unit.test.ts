import { create } from "@bufbuild/protobuf";
import {
	GetMyStoreResponseSchema,
	KYCFlowSchema,
	PayUniState,
	SellerPayUniSchema,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { publicListingStatus } from "./capability-fixtures";
import { walletShowsTransactions } from "./wallet-transactions";

const store = (missing: SellerRequirement[] | undefined) =>
	create(GetMyStoreResponseSchema, {
		store: { id: "1" },
		capabilities: missing === undefined ? [] : [publicListingStatus(missing)],
	});

const flow = (state: PayUniState | undefined) =>
	create(KYCFlowSchema, {
		payuni:
			state === undefined ? undefined : create(SellerPayUniSchema, { state }),
	});

describe("walletShowsTransactions", () => {
	it("已認證而且 PayUni 狀態是 APPROVED", () => {
		expect(walletShowsTransactions(store([]), flow(PayUniState.APPROVED))).toBe(
			true,
		);
	});

	it("還缺 KYC_APPROVED、沒有 PUBLIC_LISTING、沒有 PayUni 狀態、PayUni 還不能收款（含 FIXABLE 與不認得的狀態）：都不顯示", () => {
		const paying = flow(PayUniState.APPROVED);
		expect(
			walletShowsTransactions(store([SellerRequirement.KYC_APPROVED]), paying),
		).toBe(false);
		expect(walletShowsTransactions(store(undefined), paying)).toBe(false);
		expect(walletShowsTransactions(store([]), null)).toBe(false);
		expect(walletShowsTransactions(store([]), flow(undefined))).toBe(false);
		expect(walletShowsTransactions(null, paying)).toBe(false);
		for (const state of [
			PayUniState.FIXABLE,
			PayUniState.TERMINATED,
			PayUniState.REVIEWING,
			PayUniState.UNSPECIFIED,
			99 as PayUniState,
		]) {
			expect(
				walletShowsTransactions(store([]), flow(state)),
				String(state),
			).toBe(false);
		}
	});
});
