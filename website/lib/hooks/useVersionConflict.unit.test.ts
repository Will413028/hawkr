// @vitest-environment jsdom

import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
	useVersionConflict,
	VERSION_CONFLICT_RELOAD_FAILED,
	versionConflictNotice,
} from "./useVersionConflict";

describe("useVersionConflict", () => {
	it("重新載入、採用最新版本、通知預設提示", async () => {
		const adopt = vi.fn();
		const notify = vi.fn();
		const { result } = renderHook(() =>
			useVersionConflict({
				reload: async () => ({ etag: "e9" }),
				adopt,
				notify,
				notice: "已更新",
			}),
		);
		expect(await result.current()).toBe(true);
		expect(adopt).toHaveBeenCalledWith({ etag: "e9" });
		expect(notify).toHaveBeenCalledWith("已更新");
	});

	it("呼叫端可以覆寫提示，adopt 回傳的字串優先", async () => {
		const notify = vi.fn();
		const { result, rerender } = renderHook(
			({ adoptResult }: { adoptResult?: string }) =>
				useVersionConflict({
					reload: async () => null,
					adopt: () => adoptResult,
					notify,
					notice: "預設",
				}),
			{ initialProps: {} },
		);
		await result.current("覆寫");
		expect(notify).toHaveBeenLastCalledWith("覆寫");
		rerender({ adoptResult: "已取消" });
		await result.current("覆寫");
		expect(notify).toHaveBeenLastCalledWith("已取消");
	});

	it("載入失敗：不採用、通知失敗提示、回 false", async () => {
		const adopt = vi.fn();
		const notify = vi.fn();
		const { result } = renderHook(() =>
			useVersionConflict({
				reload: async () => {
					throw new Error("down");
				},
				adopt,
				notify,
				notice: "已更新",
			}),
		);
		expect(await result.current()).toBe(false);
		expect(adopt).not.toHaveBeenCalled();
		expect(notify).toHaveBeenCalledWith(VERSION_CONFLICT_RELOAD_FAILED);
	});

	it("文字提示：保留輸入、已載入最新版本", () => {
		expect(versionConflictNotice("聯絡資訊")).toBe(
			"聯絡資訊已在其他地方更新，已載入最新版本，你輸入的內容仍保留，請確認後再儲存。",
		);
		expect(versionConflictNotice("申請資料", "送出")).toContain("再送出。");
	});
});
