import { create } from "@bufbuild/protobuf";
import {
	GetMyStoreResponseSchema,
	SellerCapability,
	SellerRequirement,
	StoreSchema,
	StoreStatsSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { mapProduct } from "@/lib/market/listFilters";
import type { SellerProductListItem } from "@/lib/products/seller-repo";
import { activeStatuses, publicListingStatus } from "./capability-fixtures";
import {
	isOnSaleProduct,
	previewDetailProduct,
	previewProductRecord,
	previewStoreStats,
	storeCanBePublic,
	storefrontViewOfMyStore,
} from "./storefront-preview";

function myStore(
	capabilities: ReturnType<typeof publicListingStatus>[],
	stats?: {
		followerCount?: number;
		reviewCount?: number;
		ratingAverage?: number;
	},
) {
	return create(GetMyStoreResponseSchema, {
		store: create(StoreSchema, {
			id: "700",
			storeCode: "123456",
			storeName: "預覽店",
			description: "簡介",
		}),
		capabilities,
		...(stats ? { stats: create(StoreStatsSchema, stats) } : {}),
	});
}

describe("storeCanBePublic", () => {
	it("is true only when PUBLIC_LISTING is active", () => {
		expect(storeCanBePublic(myStore([publicListingStatus([])]))).toBe(true);
		expect(
			storeCanBePublic(
				myStore([publicListingStatus([SellerRequirement.ONBOARDING_COMPLETE])]),
			),
		).toBe(false);
	});

	it("is false when the capability is not reported or there is no store", () => {
		expect(
			storeCanBePublic(
				myStore(activeStatuses([SellerCapability.PRODUCT_MANAGE])),
			),
		).toBe(false);
		expect(storeCanBePublic(null)).toBe(false);
	});
});

describe("storefrontViewOfMyStore", () => {
	it("carries GetMyStore stats through and leaves unset stats unset", () => {
		const full = storefrontViewOfMyStore(
			myStore([], { followerCount: 7, reviewCount: 0 }),
		);
		expect(full).toMatchObject({
			storeId: "700",
			storeCode: "123456",
			name: "預覽店",
			followerCount: 7,
			reviewCount: 0,
		});
		expect(full).not.toHaveProperty("ratingAverage");
		expect(storefrontViewOfMyStore(null)).toBeNull();
	});
});

describe("preview products", () => {
	const store = storefrontViewOfMyStore(myStore([]));
	if (!store) throw new Error("fixture");
	const item = (
		status: SellerProductListItem["status"],
	): SellerProductListItem => ({
		id: "9",
		name: "青菜",
		status,
		image_url: "products/9.jpg",
		price_min: "80",
		price_max: "80",
		list_price_min: "100",
		list_price_max: "100",
		sale_price_min: "80",
		stock: 5,
	});

	it("treats only the on-sale status as listable", () => {
		expect(isOnSaleProduct(item("4004"))).toBe(true);
		expect(isOnSaleProduct(item("4003"))).toBe(false);
		expect(isOnSaleProduct(item("4001"))).toBe(false);
		expect(isOnSaleProduct(item(undefined))).toBe(false);
	});

	it("converts a seller list item into what the market card takes", () => {
		const card = mapProduct(previewProductRecord(item("4004"), store), 0);
		expect(card).toMatchObject({
			id: "9",
			name: "青菜",
			image: "products/9.jpg",
			sellerName: "預覽店",
			sellerStoreCode: "123456",
			isOnSale: true,
			stock: 5,
		});
	});

	it("builds the drawer product from the seller detail, deriving shipping from SKUs", () => {
		const product = previewDetailProduct(
			{
				...item("4004"),
				skus: [
					{ id: "s1", shippingMethods: [{ type: "3001", feePayer: "3101" }] },
					{ id: "s2", shippingMethods: [{ type: "3001", feePayer: "3101" }] },
				],
			} as unknown as Parameters<typeof previewDetailProduct>[0],
			store,
		);
		expect(product.shippingMethods).toHaveLength(1);
		expect(product.storeId).toBe("700");
		expect(product.seller).toMatchObject({ storeId: "700", name: "預覽店" });
	});

	it("derives the seller card stats only from stats the store actually has", () => {
		expect(previewStoreStats(store)).toEqual({
			sellerStats: null,
			followerStats: null,
		});
		expect(
			previewStoreStats({
				...store,
				ratingAverage: 4.5,
				reviewCount: 3,
				followerCount: 8,
			}),
		).toEqual({
			sellerStats: { avgRating: 4.5, totalCount: 3 },
			followerStats: { followers: 8 },
		});
	});
});
