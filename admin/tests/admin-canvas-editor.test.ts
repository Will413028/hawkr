import assert from "node:assert/strict";
import test, { after } from "node:test";
import type { Canvas } from "fabric";
import { JSDOM } from "jsdom";

// fabric 的 "." 入口永遠是瀏覽器版，需要全域 DOM；jsdom 搭配 canvas 提供 2D context。
const dom = new JSDOM("<!doctype html><body></body>", {
	pretendToBeVisual: true,
});
Object.assign(globalThis, {
	window: dom.window,
	document: dom.window.document,
	HTMLElement: dom.window.HTMLElement,
	HTMLInputElement: dom.window.HTMLInputElement,
	HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
	HTMLCanvasElement: dom.window.HTMLCanvasElement,
	Image: dom.window.Image,
});
Object.defineProperty(globalThis, "navigator", {
	value: dom.window.navigator,
	configurable: true,
});
const doc = dom.window.document;
after(() => dom.window.close());

// CanvasEditor 的 import 會立刻讀全域 document，所以要等上面的 DOM 就位後才載入。
async function makeEditor() {
	const { CanvasEditor } = await import("../src/utils/CanvasEditor");
	const el = doc.createElement("canvas");
	el.id = `c${Math.random().toString(36).slice(2)}`;
	doc.body.appendChild(el);
	return new CanvasEditor(el.id, 400, 200);
}

test("CanvasEditor：建構後有空圖層，新增形狀以左上角定位", async () => {
	const editor = await makeEditor();
	assert.equal(editor.getLayers().length, 0);
	editor.addRect();
	editor.addCircle();
	editor.addText("hi");
	assert.equal(editor.getLayers().length, 3);
	const canvas = (editor as unknown as { canvas: Canvas }).canvas;
	for (const obj of canvas.getObjects()) {
		assert.equal(obj.originX, "left");
		assert.equal(obj.originY, "top");
	}
	editor.dispose();
});

test("CanvasEditor：漸層填色與背景漸層寫入 Gradient，且 JSON 來回後保留", async () => {
	const editor = await makeEditor();
	const canvas = (editor as unknown as { canvas: Canvas }).canvas;
	editor.addRect();
	canvas.setActiveObject(canvas.getObjects()[0]);
	editor.setGradientFill("#ff0000", "#0000ff", "horizontal");
	const fill = canvas.getObjects()[0].fill as {
		type: string;
		colorStops: { color: string }[];
	};
	assert.equal(fill.type, "linear");
	assert.deepEqual(
		fill.colorStops.map((s) => s.color),
		["#ff0000", "#0000ff"],
	);
	editor.setBackgroundGradient("#ffffff", "#000000", "vertical");
	const bg = canvas.backgroundColor as unknown as { type: string };
	assert.equal(bg.type, "linear");

	const json = JSON.stringify(canvas.toJSON());
	await editor.loadFromJSON(json);
	const restored = canvas.getObjects()[0];
	assert.equal((restored.fill as { type: string }).type, "linear");
	assert.equal(restored.originX, "left");
	editor.dispose();
});
