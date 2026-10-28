import { describe, expect, it, vi } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import type { SellerProductDetail } from "@/lib/products/seller-repo";
import {
	applyAiPatch,
	applyImagesUpdate,
	draftToSubmitBody,
	emptyDraft,
	normaliseDraftFromApi,
	resolveStatusButtons,
} from "../productFormAdapter";
import productDetailFixture from "./fixtures/app-products-866542657374814537.json";

describe("draftToSubmitBody images", () => {
	it("maps resolved stagedToken inputs to body.images objects in order", () => {
		const draft = emptyDraft();
		const body = draftToSubmitBody(draft, undefined, {
			images: [{ stagedToken: "a" }, { stagedToken: "b" }],
		});
		expect(body.images).toEqual([{ stagedToken: "a" }, { stagedToken: "b" }]);
	});

	it("omits images when none provided", () => {
		const body = draftToSubmitBody(emptyDraft(), undefined, { images: [] });
		expect(body.images).toBeUndefined();
	});
});

describe("draftToSubmitBody status", () => {
	// 後端 UpdateProductRequest.status 是 string CodeMeta；送數字會 400。
	it("maps numeric on-shelf override (1) to CodeMeta '4004'", () => {
		expect(draftToSubmitBody(emptyDraft(), 1).status).toBe("4004");
	});
	it("maps numeric draft override (0) to '4001'", () => {
		expect(draftToSubmitBody(emptyDraft(), 0).status).toBe("4001");
	});
	it("defaults to draft status '4001' with no override", () => {
		expect(draftToSubmitBody(emptyDraft()).status).toBe("4001");
	});
	it("passes through an existing CodeMeta string status", () => {
		const draft = { ...emptyDraft(), status: "4003" as unknown as number };
		expect(draftToSubmitBody(draft).status).toBe("4003");
	});
});

describe("shipping methods per carrier", () => {
	it("loads one entry per carrier and submits it once with the ambient constant", () => {
		const draft = normaliseDraftFromApi({
			...emptyDraft(),
			skus: [
				{
					shippingMethods: [
						{ type: "3001", temperature: "3201", feePayer: "3101" },
						{ type: "3001", temperature: "3202", feePayer: "3101" },
						{ type: "3001", temperature: "3203", feePayer: "3101" },
					],
				},
			],
		} as never);
		const body = draftToSubmitBody(draft);
		const sent = (body.skus?.[0] as { shippingMethods: unknown[] })
			.shippingMethods;
		expect(sent).toHaveLength(1);
		expect(sent[0]).toMatchObject({ type: "3001" });
		expect(sent[0]).not.toHaveProperty("temperature");
	});
});

describe("applyImagesUpdate", () => {
	it("replaces the gallery and recomputes main_image for an array value", () => {
		const next = applyImagesUpdate(emptyDraft(), [
			{ url: "https://cdn/a.jpg", stagedToken: "a" },
			{ url: "https://cdn/b.jpg", stagedToken: "b" },
		]);
		expect(next.images).toHaveLength(2);
		expect(next.main_image).toBe("https://cdn/a.jpg");
	});

	it("applies a functional updater against the current images", () => {
		const base = applyImagesUpdate(emptyDraft(), [
			{ url: "https://cdn/a.jpg", stagedToken: "a" },
		]);
		const next = applyImagesUpdate(base, (prev) => [
			...prev,
			{ url: "https://cdn/b.jpg", stagedToken: "b" },
		]);
		expect(next.images?.map((i) => i.stagedToken)).toEqual(["a", "b"]);
		expect(next.main_image).toBe("https://cdn/a.jpg");
	});

	// Regression: a multi-select fires N overlapping uploads that each resolve
	// and append. With the functional-updater contract every upload accumulates
	// onto fresh state; the old value-based onChange shared a stale base and
	// collapsed the gallery to a single (last-resolved) image.
	it("accumulates every image across overlapping uploads (no last-write-wins clobber)", () => {
		const uploads = [
			{ url: "https://cdn/1.jpg", stagedToken: "t1" },
			{ url: "https://cdn/2.jpg", stagedToken: "t2" },
			{ url: "https://cdn/3.jpg", stagedToken: "t3" },
		];
		// Each upload's completion handler is the functional append; threading
		// them through the resolver mimics React applying queued state updates.
		let draft = emptyDraft();
		for (const asset of uploads) {
			draft = applyImagesUpdate(draft, (prev) => [...prev, asset].slice(0, 9));
		}
		expect(draft.images?.map((i) => i.stagedToken)).toEqual(["t1", "t2", "t3"]);
		expect(draft.main_image).toBe("https://cdn/1.jpg");
	});

	it("respects the max cap when appending", () => {
		const full = Array.from({ length: 9 }, (_, i) => ({
			url: `https://cdn/${i}.jpg`,
			stagedToken: `t${i}`,
		}));
		const draft = applyImagesUpdate(emptyDraft(), full);
		const next = applyImagesUpdate(draft, (prev) =>
			[...prev, { url: "https://cdn/extra.jpg", stagedToken: "tx" }].slice(
				0,
				9,
			),
		);
		expect(next.images).toHaveLength(9);
		expect(next.images?.some((i) => i.stagedToken === "tx")).toBe(false);
	});
});

describe("normaliseDraftFromApi status", () => {
	// 後端回 CodeMeta 字串，radio 用 0/1/2 比對；收斂錯了開編輯頁 radio 預選會錯。
	const norm = (status: unknown) =>
		normaliseDraftFromApi({ ...emptyDraft(), status } as never).status;
	it("coerces 4004 (上架) → 1", () => expect(norm("4004")).toBe(1));
	it("coerces 4003 (下架) → 2", () => expect(norm("4003")).toBe(2));
	it("coerces 4001 (草稿) → 0", () => expect(norm("4001")).toBe(0));
	it("maps 審核中/unknown → 0 (draft)", () => expect(norm("4002")).toBe(0));
});

/**
 * 圖庫映射：fixture 是**實打回應**，不是手寫 wire shape。
 *
 * `fixtures/app-products-866542657374814537.json` ＝
 * `GET https://test-api.example.com/api/v1/app/products/866542657374814537`
 * 的原始回應（dev 環境，賣家 account 2685914682，2026-08-04T17:27:53Z 擷取）。
 *
 * 這件事必須用真回應鎖住：本檔上一版的 fixture 假設後端會回 `images: string[]`，
 * 後端從來沒有這個欄位 —— 於是測試一路綠、編輯頁圖庫一路空，賣家一動圖片就把
 * 既有圖整組洗掉（#430 A1）。手寫的 wire shape 會替 bug 背書。
 */
// `as unknown as`, not a direct cast: the live payload wires `stock` as a
// string while `SellerProductListItem.stock` is declared `number`, so the two
// types genuinely do not overlap. That mismatch is a pre-existing type lie
// tracked under #430 D4, not something this fixture should paper over silently.
const liveDetail = productDetailFixture.data as unknown as SellerProductDetail &
	Record<string, unknown>;

describe("normaliseDraftFromApi images (live wire fixture)", () => {
	const withMediaGateway = <T>(fn: () => T): T => {
		const base = getEnv();
		__setEnvForTest({
			...base,
			mediaBaseUrl: "https://test-media.example.com",
		});
		try {
			return fn();
		} finally {
			__setEnvForTest(null);
		}
	};

	it("the captured response has no `images` key — reading one yields an empty gallery", () => {
		expect("images" in liveDetail).toBe(false);
		expect(Array.isArray(liveDetail.gallery)).toBe(true);
		expect((liveDetail.gallery as unknown[]).length).toBeGreaterThan(0);
	});

	it("hydrates every gallery entry with the assetId that lets it round-trip on save", () => {
		const wire = liveDetail.gallery as Array<{
			assetId: string;
			imageKey: string;
			imageUrl: string;
		}>;
		const draft = withMediaGateway(() =>
			normaliseDraftFromApi({ ...emptyDraft(), ...liveDetail } as never),
		);

		expect(draft.images).toHaveLength(wire.length);
		// assetId is what `resolveStagedImages` sends back so the backend keeps the
		// image; without it a save replaces the gallery with only the new uploads.
		expect(draft.images?.map((i) => i.assetId)).toEqual(
			wire.map((g) => g.assetId),
		);
		// Our locally built display URL must agree with the one the gateway built
		// from the same imageKey (49 §6.5 single entry point).
		expect(draft.images?.map((i) => i.url)).toEqual(
			wire.map((g) => g.imageUrl),
		);
		expect(draft.main_image).toBe(wire[0].imageUrl);
	});

	it("falls back to the response's own imageKey for 主圖 when the gallery is empty", () => {
		const draft = withMediaGateway(() =>
			normaliseDraftFromApi({
				...emptyDraft(),
				...liveDetail,
				gallery: [],
			} as never),
		);
		expect(draft.images).toEqual([]);
		expect(draft.main_image).toBe(liveDetail.imageUrl);
	});

	it("keeps an entry whose URL cannot be built as long as it still has an assetId", () => {
		// Losing the thumbnail is cosmetic; losing the assetId deletes the image on
		// the next save. The entry must survive.
		const draft = normaliseDraftFromApi({
			...emptyDraft(),
			gallery: [{ assetId: "872530103878698664", imageKey: "" }],
		} as never);
		expect(draft.images).toEqual([{ url: "", assetId: "872530103878698664" }]);
	});

	it("warns instead of silently wiping when a non-empty gallery maps to zero images", () => {
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
		const draft = normaliseDraftFromApi({
			...emptyDraft(),
			gallery: [{ unexpected: true }, 123],
		} as never);
		expect(draft.images).toEqual([]);
		expect(warn).toHaveBeenCalledOnce();
		warn.mockRestore();
	});

	it("builds persisted object keys on the configured media origin", () => {
		const base = getEnv();
		__setEnvForTest({
			...base,
			flavor: "prod",
			mediaBaseUrl: "https://media.example.com",
		});
		try {
			const draft = normaliseDraftFromApi({
				...emptyDraft(),
				...liveDetail,
			} as never);
			const wire = liveDetail.gallery as Array<{ imageKey: string }>;
			expect(draft.images?.map((i) => i.url)).toEqual(
				wire.map((g) => `https://media.example.com/i/original/${g.imageKey}`),
			);
			expect(draft.main_image).toBe(
				`https://media.example.com/i/original/${wire[0].imageKey}`,
			);
		} finally {
			__setEnvForTest(null);
		}
	});
});

describe("resolveStatusButtons", () => {
	it("create：單顆建立並上架，無 secondary", () => {
		expect(resolveStatusButtons("create", 0)).toEqual({
			primary: { label: "建立並上架", action: "publish" },
			secondary: null,
		});
	});
	it("edit 草稿(0)：儲存草稿 + 儲存並上架", () => {
		expect(resolveStatusButtons("edit", 0)).toEqual({
			primary: { label: "儲存草稿", action: "save" },
			secondary: { label: "儲存並上架", action: "publish" },
		});
	});
	it("edit 下架(2)：儲存變更 + 儲存並上架", () => {
		expect(resolveStatusButtons("edit", 2)).toEqual({
			primary: { label: "儲存變更", action: "save" },
			secondary: { label: "儲存並上架", action: "publish" },
		});
	});
	it("edit 上架(1)：不經此函式（由頁面閘記攔截），fallback 回草稿配置", () => {
		expect(resolveStatusButtons("edit", 1)).toEqual({
			primary: { label: "儲存草稿", action: "save" },
			secondary: { label: "儲存並上架", action: "publish" },
		});
	});
});
