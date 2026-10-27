import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { StoreSchema } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { defaultErrorTextFor } from "@/lib/i18n/ui-text";
import type { StoreUpdate } from "./repo";
import { createStoreContactSaver } from "./store-contact-saver";

const defaults = defaultErrorTextFor("zh-TW");

const store = (etag: string, extra: Record<string, string> = {}) =>
	create(StoreSchema, { etag, ...extra });

const draft = (contactName: string, contactMobile = "", supportEmail = "") => ({
	contact: { contactName, contactMobile, supportEmail },
	address: { countyCode: "", townCode: "", detail: "" },
});

function deferred<T>() {
	let resolve!: (v: T) => void;
	const promise = new Promise<T>((r) => {
		resolve = r;
	});
	return { promise, resolve };
}

describe("createStoreContactSaver", () => {
	it("只寫改過的聯絡欄位，帶目前的 etag，成功後換 etag 與 baseline", async () => {
		const sent: StoreUpdate[] = [];
		const saver = createStoreContactSaver({
			defaults,
			save: async (u) => {
				sent.push(u);
				return {
					etag: "e2",
					store: store("e2", {
						contactName: "王小明",
						contactMobile: "0912345678",
					}),
				};
			},
		});
		saver.adopt(store("e1"));
		expect(saver.pending(draft(""))).toBeNull();
		const r = await saver.flush(draft(" 王小明 ", "0912345678"));
		expect(r.kind).toBe("saved");
		expect(sent).toEqual([
			{ etag: "e1", contactName: "王小明", contactMobile: "0912345678" },
		]);
		expect(saver.pending(draft("王小明", "0912345678"))).toBeNull();
		expect((await saver.flush(draft("王小明", "0912345678"))).kind).toBe(
			"idle",
		);
	});

	it("一次只有一個寫入在途：第二個排在第一個之後，用第一個回來的 etag", async () => {
		const first = deferred<{ etag: string; store: ReturnType<typeof store> }>();
		const sent: StoreUpdate[] = [];
		const saver = createStoreContactSaver({
			defaults,
			save: (u) => {
				sent.push(u);
				if (sent.length === 1) return first.promise;
				return Promise.resolve({
					etag: "e3",
					store: store("e3", {
						contactName: "甲乙",
						contactMobile: "0912345678",
					}),
				});
			},
		});
		saver.adopt(store("e1"));
		const a = saver.flush(draft("甲"));
		const b = saver.flush(draft("甲乙", "0912345678"));
		await Promise.resolve();
		expect(sent).toHaveLength(1);
		first.resolve({ etag: "e2", store: store("e2", { contactName: "甲" }) });
		await a;
		await b;
		expect(sent).toEqual([
			{ etag: "e1", contactName: "甲" },
			{ etag: "e2", contactName: "甲乙", contactMobile: "0912345678" },
		]);
	});

	it("版本衝突回 conflict，不改 baseline；adopt 最新版本之後以新 etag 再寫，輸入保留", async () => {
		const sent: StoreUpdate[] = [];
		const conflict = new ConnectError("衝突", Code.Aborted, undefined, [
			{
				desc: ErrorInfoSchema,
				value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
			},
		]);
		const saver = createStoreContactSaver({
			defaults,
			save: async (u) => {
				sent.push(u);
				if (sent.length === 1) throw conflict;
				return { etag: "e9", store: store("e9", { contactName: "王" }) };
			},
		});
		saver.adopt(store("e1"));
		expect((await saver.flush(draft("王"))).kind).toBe("conflict");
		saver.adopt(store("e5"));
		expect((await saver.flush(draft("王"))).kind).toBe("saved");
		expect(sent.map((u) => u.etag)).toEqual(["e1", "e5"]);
	});

	it("伺服器拒絕時回 failed 帶欄位錯誤，baseline 不變", async () => {
		const bad = new ConnectError("格式不正確", Code.InvalidArgument);
		const saver = createStoreContactSaver({
			defaults,
			save: async () => {
				throw bad;
			},
		});
		saver.adopt(store("e1", { contactName: "王" }));
		const r = await saver.flush(draft(""));
		expect(r.kind).toBe("failed");
		expect(saver.pending(draft(""))).not.toBeNull();
	});
});
