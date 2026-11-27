import { describe, expect, it } from "vitest";
import { correctionEntry } from "./correctionEntry";

const base = { canRequestCorrection: false, snapshotVersion: "3" };

describe("correctionEntry", () => {
	it("只認後端的 canRequestCorrection", () => {
		expect(
			correctionEntry({ ...base, canRequestCorrection: true }).visible,
		).toBe(true);
		expect(correctionEntry(base).visible).toBe(false);
	});

	it("後端說不能改時，不論有沒有發票文件都不顯示", () => {
		expect(
			correctionEntry({ ...base, canRequestCorrection: false }).visible,
		).toBe(false);
	});

	it("送出用的是內容軸的 snapshotVersion", () => {
		expect(
			correctionEntry({ canRequestCorrection: true, snapshotVersion: "7" })
				.expectedSnapshotVersion,
		).toBe("7");
	});

	it("invoice 還沒載入時不顯示，也不送空版本以外的東西", () => {
		const e = correctionEntry(null);
		expect(e.visible).toBe(false);
		expect(e.expectedSnapshotVersion).toBe("");
	});
});
