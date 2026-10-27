import { describe, expect, it, vi } from "vitest";
import { createAgUiEventDispatcher } from "./agui-dispatch";

function handlers() {
	return {
		onText: vi.fn(),
		onNamedEvent: vi.fn(),
		onDone: vi.fn(),
		onError: vi.fn(),
	};
}

describe("createAgUiEventDispatcher", () => {
	it("CUSTOM 事件轉成具名事件，value 以 JSON 字串交給呼叫端自行解析", () => {
		const h = handlers();
		createAgUiEventDispatcher(h)({
			type: "CUSTOM",
			name: "activity",
			value: { rowId: "9001", code: "SEARCH_CANDIDATES", text: "" },
		});
		expect(h.onNamedEvent).toHaveBeenCalledWith(
			"activity",
			JSON.stringify({ rowId: "9001", code: "SEARCH_CANDIDATES", text: "" }),
		);
	});

	it("RUN_FINISHED 帶出 result 與收斂後的 outcome", () => {
		const h = handlers();
		createAgUiEventDispatcher(h)({
			type: "RUN_FINISHED",
			threadId: "t1",
			runId: "r1",
			result: { ok: true },
		});
		expect(h.onDone).toHaveBeenCalledWith(JSON.stringify({ ok: true }), {
			type: "success",
		});
	});

	it("RUN_ERROR 的 reason 從未經 schema 裁剪的原始事件補讀（官方 schema 沒有這個欄位）", () => {
		const h = handlers();
		createAgUiEventDispatcher(h)({
			type: "RUN_ERROR",
			message: "游標已過保留視窗",
			code: "410",
			reason: "REPLAY_CURSOR_EXPIRED",
		});
		expect(h.onError).toHaveBeenCalledWith(
			"游標已過保留視窗",
			410,
			"REPLAY_CURSOR_EXPIRED",
		);
	});

	it("不合 AG-UI 契約的事件走 onError，不靜默丟棄", () => {
		const h = handlers();
		createAgUiEventDispatcher(h)({ type: "NOT_AN_AGUI_EVENT" });
		expect(h.onError).toHaveBeenCalledTimes(1);
		expect(h.onNamedEvent).not.toHaveBeenCalled();
	});

	it("有提供 onInvalidEvent 時，schema 拒絕改走它而不是 onError（只丟這個 frame）", () => {
		const h = { ...handlers(), onInvalidEvent: vi.fn() };
		const dispatch = createAgUiEventDispatcher(h);
		dispatch({ type: "NOT_AN_AGUI_EVENT" });
		expect(h.onInvalidEvent).toHaveBeenCalledTimes(1);
		expect(h.onInvalidEvent.mock.calls[0][0]).toContain("AG-UI 契約");
		expect(h.onError).not.toHaveBeenCalled();

		// 同一個 dispatcher 後續的合法 frame 照常分派——被擋下的是那個 frame，
		// 不是整條串流。
		dispatch({ type: "CUSTOM", name: "activity", value: { rowId: "1" } });
		expect(h.onNamedEvent).toHaveBeenCalledWith(
			"activity",
			JSON.stringify({ rowId: "1" }),
		);
	});

	it("TOOL_CALL_RESULT 用同一個 dispatcher 內記住的 toolCallName 具名", () => {
		const h = handlers();
		const dispatch = createAgUiEventDispatcher(h);
		dispatch({
			type: "TOOL_CALL_START",
			toolCallId: "tc1",
			toolCallName: "seller_match",
		});
		dispatch({
			type: "TOOL_CALL_RESULT",
			toolCallId: "tc1",
			messageId: "m1",
			content: "[]",
		});
		expect(h.onNamedEvent).toHaveBeenCalledWith("seller_match", "[]");
	});
});
