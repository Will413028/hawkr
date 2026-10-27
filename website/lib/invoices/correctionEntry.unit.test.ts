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
		// 🔴 這條是本檔的核心：前端若改用「有沒有開過發票」當判準，這裡會轉紅。
		// canRequestCorrection 涵蓋開票狀態、法定作廢窗口、是否已有進行中的申請
		// 三件事，只看其中一件必然漂移。
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
