import { Code, ConnectError } from "@connectrpc/connect";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ getStore: vi.fn() }));
vi.mock("@/lib/connect/seller-public", () => ({
	sellerPublicClient: () => client,
}));

import {
	fetchPublicStore,
	isStoreAbsentError,
	isStoreCodeShape,
} from "./public-store";

const STORE_CODE = "2662192214";

function response(over: Record<string, unknown> = {}) {
	return {
		store: {
			id: "7487768729113814280",
			storeCode: STORE_CODE,
			storeName: "花蓮小鋪",
			description: "介紹",
			townCode: "10015010",
			prize: "",
			websiteUrl: "",
			logo: { uriTemplate: "https://m.example/i/{preset}/logo.png" },
			coverImage: undefined,
			...over,
		},
		stats: { followerCount: 3, reviewCount: 0 },
	};
}

beforeEach(() => client.getStore.mockReset());

describe("fetchPublicStore", () => {
	it("asks GetStore with the URL segment as store_code and returns store.id for later reads", async () => {
		client.getStore.mockResolvedValue(response());
		const view = await fetchPublicStore(STORE_CODE);
		expect(client.getStore).toHaveBeenCalledWith(
			{ storeCode: STORE_CODE },
			expect.anything(),
		);
		expect(view?.storeId).toBe("7487768729113814280");
		expect(view?.storeCode).toBe(STORE_CODE);
		expect(view?.logo_url).toBe("https://m.example/i/avatar/logo.png");
	});

	it("leaves unset stats unset instead of zero", async () => {
		client.getStore.mockResolvedValue(response());
		const view = await fetchPublicStore(STORE_CODE);
		expect(view?.followerCount).toBe(3);
		expect(view?.reviewCount).toBe(0);
		expect(view).not.toHaveProperty("ratingAverage");
	});

	it("maps town_code to county and district names, and nothing finer", async () => {
		client.getStore.mockResolvedValue(response());
		const view = await fetchPublicStore(STORE_CODE);
		expect(view?.city).toBeTruthy();
		expect(view?.district).toBeTruthy();
		expect(view).not.toHaveProperty("address");
	});

	it("only not_found means no store; invalid_argument and other errors are rethrown", async () => {
		client.getStore.mockRejectedValueOnce(new ConnectError("x", Code.NotFound));
		await expect(fetchPublicStore(STORE_CODE)).resolves.toBeNull();
		client.getStore.mockRejectedValueOnce(
			new ConnectError("x", Code.InvalidArgument),
		);
		await expect(fetchPublicStore(STORE_CODE)).rejects.toThrow();
		client.getStore.mockRejectedValueOnce(
			new ConnectError("x", Code.Unavailable),
		);
		await expect(fetchPublicStore(STORE_CODE)).rejects.toThrow();
	});

	it("throws instead of returning null when a success carries no store id", async () => {
		client.getStore.mockResolvedValue(response({ id: "" }));
		await expect(fetchPublicStore(STORE_CODE)).rejects.toThrow();
	});

	it("never calls the backend for a segment outside stores_store_code_check", async () => {
		for (const bad of ["0", "10000000000", "abc", "-1", "01", ""]) {
			await expect(fetchPublicStore(bad)).resolves.toBeNull();
		}
		expect(client.getStore).not.toHaveBeenCalled();
	});
});

describe("isStoreCodeShape", () => {
	it("accepts 1..9999999999 without leading zero", () => {
		expect(isStoreCodeShape("1")).toBe(true);
		expect(isStoreCodeShape("9999999999")).toBe(true);
		expect(isStoreCodeShape("10000000000")).toBe(false);
	});
});

describe("isStoreAbsentError", () => {
	it("is true only for not_found", () => {
		expect(isStoreAbsentError(new ConnectError("x", Code.NotFound))).toBe(true);
		expect(
			isStoreAbsentError(new ConnectError("x", Code.InvalidArgument)),
		).toBe(false);
		expect(isStoreAbsentError(new ConnectError("x", Code.Unavailable))).toBe(
			false,
		);
	});
});
