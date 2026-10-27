import { create } from "@bufbuild/protobuf";
import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
} from "@connectrpc/connect";
import {
	AgreementKind,
	SellerAppService,
	type UpdateMyStoreRequest,
	UpdateMyStoreRequestSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ client: null as unknown }));
vi.mock("@/lib/connect/seller", () => ({
	sellerAppClient: () => mocks.client,
}));

import {
	buildUpdateMyStoreRequest,
	clearFoodRegistration,
	createSellerStorefrontRepo,
	submitFoodRegistration,
	withdrawFoodRegistration,
} from "./repo";

function recordingRepo() {
	const requests: UpdateMyStoreRequest[] = [];
	const transport = createRouterTransport((router) =>
		router.service(SellerAppService, {
			updateMyStore(req) {
				requests.push(req);
				return {
					store: { id: "7", etag: "e2", storeName: req.store?.storeName },
				};
			},
			getMyStore() {
				return { store: { id: "7", etag: "e1" } };
			},
		}),
	);
	const repo = createSellerStorefrontRepo(() =>
		createClient(SellerAppService, transport),
	);
	return { repo, requests };
}

describe("buildUpdateMyStoreRequest", () => {
	it("carries the etag on the store of every write", () => {
		const request = buildUpdateMyStoreRequest({
			etag: "e1",
			storeName: "山田",
		});
		expect(request.store?.etag).toBe("e1");
		expect(request.updateMask?.paths).toEqual(["store_name"]);
	});

	it("refuses to build a write without an etag", () => {
		expect(() =>
			buildUpdateMyStoreRequest({ etag: "", storeName: "山田" }),
		).toThrow(/etag/);
	});

	it("lists only the fields that are present in the mask", () => {
		const request = buildUpdateMyStoreRequest({
			etag: "e1",
			description: "",
			websiteUrl: "https://example.com",
			prize: "",
		});
		expect([...(request.updateMask?.paths ?? [])].sort()).toEqual([
			"description",
			"prize",
			"website_url",
		]);
		expect(request.store?.storeName).toBe("");
	});

	it("replaces the whole address, and clears it with null", () => {
		const set = buildUpdateMyStoreRequest({
			etag: "e1",
			address: { townCode: "63000010", addressDetail: "信義路 1 號" },
		});
		expect(set.updateMask?.paths).toEqual(["address"]);
		expect(set.store?.address?.townCode).toBe("63000010");
		expect(set.store?.address?.addressDetail).toBe("信義路 1 號");

		const cleared = buildUpdateMyStoreRequest({ etag: "e1", address: null });
		expect(cleared.updateMask?.paths).toEqual(["address"]);
		expect(cleared.store?.address).toBeUndefined();
	});

	it("masks image tokens", () => {
		const request = buildUpdateMyStoreRequest({
			etag: "e1",
			logoStagedToken: "stg_logo",
		});
		expect([...(request.updateMask?.paths ?? [])]).toEqual([
			"logo_staged_token",
		]);
		expect(request.store?.logoStagedToken).toBe("stg_logo");
	});

	it("is a valid UpdateMyStoreRequest message", () => {
		const request = buildUpdateMyStoreRequest({ etag: "e1", prize: "金牌" });
		expect(create(UpdateMyStoreRequestSchema, request)).toBeTruthy();
	});
});

describe("storefront repo over SellerAppService", () => {
	it("sends UpdateMyStore with the etag it was given and returns the next etag", async () => {
		const { repo, requests } = recordingRepo();
		const written = await repo.updateMyStore({ etag: "e1", storeName: "山田" });
		expect(requests).toHaveLength(1);
		expect(requests[0]?.store?.etag).toBe("e1");
		expect(requests[0]?.updateMask?.paths).toEqual(["store_name"]);
		expect(written.etag).toBe("e2");
		expect(written.store?.storeName).toBe("山田");
	});

	it("sends image replacements as staged tokens with the etag", async () => {
		const { repo, requests } = recordingRepo();
		await repo.updateStoreLogo("e1", "stg_logo");
		await repo.updateStoreCover("e2", "stg_cover");
		expect(requests.map((r) => r.store?.etag)).toEqual(["e1", "e2"]);
		expect(requests.map((r) => r.updateMask?.paths)).toEqual([
			["logo_staged_token"],
			["cover_staged_token"],
		]);
		expect(requests[0]?.store?.logoStagedToken).toBe("stg_logo");
		expect(requests[1]?.store?.coverStagedToken).toBe("stg_cover");
	});

	it("does not call the server without an etag", async () => {
		const { repo, requests } = recordingRepo();
		await expect(
			repo.updateMyStore({ etag: "", storeName: "x" }),
		).rejects.toThrow();
		expect(requests).toHaveLength(0);
	});

	it("reads the store with GetMyStore", async () => {
		const { repo } = recordingRepo();
		const my = await repo.fetchMyStore();
		expect(my.store?.etag).toBe("e1");
	});

	it("passes a version conflict through unchanged for the caller to classify", async () => {
		const transport = createRouterTransport((router) =>
			router.service(SellerAppService, {
				updateMyStore() {
					throw new ConnectError("stale", Code.Aborted);
				},
			}),
		);
		const repo = createSellerStorefrontRepo(() =>
			createClient(SellerAppService, transport),
		);
		await expect(
			repo.updateMyStore({ etag: "old", storeName: "x" }),
		).rejects.toMatchObject({ code: Code.Aborted });
	});
});

describe("食品業者登錄的三個指令", () => {
	it("送出（字號去掉空白）、撤回、清除各打自己的 RPC，回傳回應裡的店家", async () => {
		const calls: string[] = [];
		const transport = createRouterTransport((router) =>
			router.service(SellerAppService, {
				submitFoodRegistration(req) {
					calls.push(
						`submit:${req.registrationNumber}:${req.acceptedAgreements.map((a) => `${a.kind}@${a.version}`).join(",")}`,
					);
					return { store: { id: "7", etag: "e2" } };
				},
				withdrawFoodRegistration() {
					calls.push("cancel");
					return { store: { id: "7", etag: "e3" } };
				},
				clearFoodRegistration() {
					calls.push("clear");
					return { store: { id: "7", etag: "e4" } };
				},
			}),
		);
		mocks.client = createClient(SellerAppService, transport);
		expect(
			(
				await submitFoodRegistration(" A-1 ", [
					{
						kind: AgreementKind.FOOD_REGISTRATION_NOTICE,
						version: "2026-10-16",
					},
				])
			).etag,
		).toBe("e2");
		expect((await withdrawFoodRegistration()).etag).toBe("e3");
		expect((await clearFoodRegistration()).etag).toBe("e4");
		expect(calls).toEqual([
			`submit:A-1:${AgreementKind.FOOD_REGISTRATION_NOTICE}@2026-10-16`,
			"cancel",
			"clear",
		]);
	});
});
