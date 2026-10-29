import { create } from "@bufbuild/protobuf";
import {
	StoreCardSchema,
	StoreCardStatus,
} from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { describe, expect, it } from "vitest";
import { storeCardView } from "./storeCardView";

const TEXT = {
	paused: "暫停營業",
	closed: "已關閉",
	closedStore: "已關閉的店家",
};

const card = (over: Record<string, unknown>) =>
	create(StoreCardSchema, {
		id: "7",
		storeCode: "shop7",
		name: "小店",
		...over,
	});

describe("storeCardView (B11)", () => {
	it("links an open store by its store code", () => {
		const v = storeCardView(card({ status: StoreCardStatus.OPEN }), TEXT);
		expect(v).toMatchObject({ name: "小店", href: "/sellers/shop7" });
		expect(v?.label).toBeUndefined();
	});

	it("shows a label and no link for paused and closed stores", () => {
		const paused = storeCardView(
			card({ status: StoreCardStatus.PAUSED }),
			TEXT,
		);
		expect(paused).toMatchObject({
			label: "paused",
			labelText: "暫停營業",
			href: "",
			name: "小店",
		});
		const closed = storeCardView(
			card({ status: StoreCardStatus.CLOSED }),
			TEXT,
		);
		expect(closed).toMatchObject({
			label: "closed",
			labelText: "已關閉",
			href: "",
		});
	});

	it("names a store without a name as 已關閉的店家", () => {
		const v = storeCardView(
			card({ status: StoreCardStatus.CLOSED, name: undefined }),
			TEXT,
		);
		expect(v?.name).toBe("已關閉的店家");
	});

	it("does not link an open card without a store code", () => {
		const v = storeCardView(
			card({ status: StoreCardStatus.OPEN, storeCode: "" }),
			TEXT,
		);
		expect(v?.href).toBe("");
	});

	it("treats an unspecified status as neither linkable nor labelled", () => {
		const v = storeCardView(
			card({ status: StoreCardStatus.UNSPECIFIED }),
			TEXT,
		);
		expect(v?.href).toBe("");
		expect(v?.label).toBeUndefined();
	});

	it("returns undefined when there is no card", () => {
		expect(storeCardView(undefined, TEXT)).toBeUndefined();
	});
});
