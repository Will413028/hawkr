import { create } from "@bufbuild/protobuf";
import {
	CapabilityStatusSchema,
	GetMyStoreResponseSchema,
	SellerCapability,
	SellerRequirement,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { activeStatuses, publicListingStatus } from "./capability-fixtures";
import {
	activeCapabilities,
	hasCapability,
	isStoreCertified,
	isStorePendingDeletion,
	isStoreSuspended,
	missingRequirements,
	needsOnboarding,
	shippingGateOf,
	storeIdOf,
	storeImageUrls,
} from "./store";

function my(
	init: Parameters<typeof create<typeof GetMyStoreResponseSchema>>[1],
) {
	return create(GetMyStoreResponseSchema, init);
}

describe("shippingGateOf", () => {
	const status = (active: boolean, missing: SellerRequirement[]) =>
		my({
			capabilities: [
				create(CapabilityStatusSchema, {
					capability: SellerCapability.SHIPPING_SETUP,
					active,
					missing,
				}),
			],
		});

	it("opens only when SHIPPING_SETUP is active", () => {
		expect(shippingGateOf(status(true, []))).toBe("open");
	});

	it("missing KYC_APPROVED gates on certification", () => {
		expect(
			shippingGateOf(status(false, [SellerRequirement.KYC_APPROVED])),
		).toBe("certification");
	});

	it("suspension or pending deletion outranks KYC in the missing list", () => {
		expect(
			shippingGateOf(
				status(false, [
					SellerRequirement.NOT_SUSPENDED,
					SellerRequirement.KYC_APPROVED,
				]),
			),
		).toBe("suspended");
		expect(
			shippingGateOf(status(false, [SellerRequirement.NOT_PENDING_DELETION])),
		).toBe("pending_deletion");
	});

	it("is unknown (never the certification gate) when the server did not report the capability", () => {
		expect(shippingGateOf(my({}))).toBe("unknown");
		expect(shippingGateOf(null)).toBe("unknown");
		expect(
			shippingGateOf(status(false, [SellerRequirement.STORE_ACTIVE])),
		).toBe("unknown");
		expect(shippingGateOf(status(false, []))).toBe("unknown");
	});

	it("pending deletion > suspended > certification when several are missing", () => {
		expect(
			shippingGateOf(
				status(false, [
					SellerRequirement.NOT_PENDING_DELETION,
					SellerRequirement.NOT_SUSPENDED,
					SellerRequirement.KYC_APPROVED,
				]),
			),
		).toBe("pending_deletion");
	});

	it("is independent of PUBLIC_LISTING (certified + not-yet-public still opens)", () => {
		const m = my({
			capabilities: [
				...activeStatuses([SellerCapability.SHIPPING_SETUP]),
				publicListingStatus([SellerRequirement.LOGO]),
			],
		});
		expect(shippingGateOf(m)).toBe("open");
		expect(isStoreCertified(m)).toBe(true);
	});
});

describe("isStoreCertified", () => {
	it("is certified when KYC_APPROVED is not among the missing requirements", () => {
		expect(
			isStoreCertified(
				my({
					capabilities: [publicListingStatus([SellerRequirement.LOGO])],
				}),
			),
		).toBe(true);
	});

	it("is not certified while KYC_APPROVED is missing", () => {
		expect(
			isStoreCertified(
				my({
					capabilities: [
						publicListingStatus([
							SellerRequirement.KYC_APPROVED,
							SellerRequirement.LOGO,
						]),
					],
				}),
			),
		).toBe(false);
	});

	it("does not treat a missing PUBLIC_LISTING status as certified", () => {
		expect(isStoreCertified(my({}))).toBe(false);
		expect(isStoreCertified(null)).toBe(false);
	});
});

describe("store lifecycle", () => {
	it("pending deletion only for STORE_STATUS_PENDING_DELETION", () => {
		expect(
			isStorePendingDeletion(
				my({ store: { status: StoreStatus.PENDING_DELETION } }),
			),
		).toBe(true);
		expect(
			isStorePendingDeletion(my({ store: { status: StoreStatus.ACTIVE } })),
		).toBe(false);
		expect(
			isStorePendingDeletion(
				my({ store: { status: StoreStatus.UNSPECIFIED } }),
			),
		).toBe(false);
		expect(isStorePendingDeletion(null)).toBe(false);
	});

	it("suspended only when the server says so (GetMyStoreResponse.suspended)", () => {
		expect(isStoreSuspended(my({ suspended: true }))).toBe(true);
		expect(isStoreSuspended(my({ suspended: false }))).toBe(false);
		expect(isStoreSuspended(my({ store: { storeName: "x" } }))).toBe(false);
		expect(isStoreSuspended(null)).toBe(false);
	});

	it("needs onboarding until the store marks it completed", () => {
		expect(needsOnboarding(null)).toBe(true);
		expect(needsOnboarding(my({}))).toBe(true);
		expect(
			needsOnboarding(my({ store: { id: "1", onboardingCompleted: false } })),
		).toBe(true);
		expect(
			needsOnboarding(my({ store: { id: "1", onboardingCompleted: true } })),
		).toBe(false);
	});
});

describe("capabilities", () => {
	it("reads the active capabilities", () => {
		const granted = my({
			capabilities: [
				...activeStatuses([SellerCapability.ORDER_FULFILL]),
				create(CapabilityStatusSchema, {
					capability: SellerCapability.MARKETING,
					active: false,
					missing: [SellerRequirement.NOT_SUSPENDED],
				}),
			],
		});
		expect(hasCapability(granted, SellerCapability.ORDER_FULFILL)).toBe(true);
		expect(hasCapability(granted, SellerCapability.MARKETING)).toBe(false);
		expect(hasCapability(granted, SellerCapability.CHAT_SEND)).toBe(false);
		expect(hasCapability(null, SellerCapability.ORDER_FULFILL)).toBe(false);
		expect(activeCapabilities(granted)).toEqual([
			SellerCapability.ORDER_FULFILL,
		]);
		expect(activeCapabilities(null)).toEqual([]);
	});

	it("reads the requirements PUBLIC_LISTING is missing", () => {
		expect(
			missingRequirements(
				my({
					capabilities: [publicListingStatus([SellerRequirement.LOGO])],
				}),
			),
		).toEqual([SellerRequirement.LOGO]);
		expect(
			missingRequirements(
				my({ capabilities: activeStatuses([SellerCapability.ORDER_FULFILL]) }),
			),
		).toBeUndefined();
	});
});

describe("unset fields are not values", () => {
	it("returns undefined for an empty id", () => {
		expect(storeIdOf(my({}))).toBeUndefined();
		expect(storeIdOf(my({ store: { id: "" } }))).toBeUndefined();
		expect(storeIdOf(my({ store: { id: "9" } }))).toBe("9");
	});

	it("returns undefined image urls when logo and cover are unset", () => {
		expect(storeImageUrls(my({ store: { id: "1" } }).store)).toEqual({
			logoUrl: undefined,
			coverUrl: undefined,
		});
	});

	it("expands the uri template with the preset", () => {
		const images = storeImageUrls(
			my({
				store: {
					logo: { uriTemplate: "https://img.test/i/{preset}/logo.jpg" },
				},
			}).store,
		);
		expect(images.logoUrl).toBe("https://img.test/i/avatar/logo.jpg");
	});
});
