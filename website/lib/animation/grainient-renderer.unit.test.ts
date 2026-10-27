/** @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	createGrainientRenderer,
	type GrainientRenderer,
} from "./grainient-renderer";

// Mock only browser/GPU boundaries. All scheduling, sizing, timekeeping and
// disposal below execute the production renderer; shader pixels need real GPU QA.
function createFakeGl() {
	let nextShader = 0;
	const uniforms = new Map<string, WebGLUniformLocation>();
	return {
		VERTEX_SHADER: 35633,
		FRAGMENT_SHADER: 35632,
		COMPILE_STATUS: 35713,
		LINK_STATUS: 35714,
		ARRAY_BUFFER: 34962,
		STATIC_DRAW: 35044,
		FLOAT: 5126,
		TRIANGLES: 4,
		uniforms,
		createShader: vi.fn((_type: number): WebGLShader => ({ id: ++nextShader })),
		shaderSource: vi.fn<WebGL2RenderingContext["shaderSource"]>(),
		compileShader: vi.fn<WebGL2RenderingContext["compileShader"]>(),
		getShaderParameter: vi.fn(
			(_shader: WebGLShader, _parameter: number) => true,
		),
		deleteShader: vi.fn<WebGL2RenderingContext["deleteShader"]>(),
		createProgram: vi.fn((): WebGLProgram => ({ id: "program" })),
		attachShader: vi.fn<WebGL2RenderingContext["attachShader"]>(),
		detachShader: vi.fn<WebGL2RenderingContext["detachShader"]>(),
		bindAttribLocation: vi.fn<WebGL2RenderingContext["bindAttribLocation"]>(),
		linkProgram: vi.fn<WebGL2RenderingContext["linkProgram"]>(),
		getProgramParameter: vi.fn(
			(_program: WebGLProgram, _parameter: number) => true,
		),
		deleteProgram: vi.fn<WebGL2RenderingContext["deleteProgram"]>(),
		createVertexArray: vi.fn((): WebGLVertexArrayObject => ({ id: "vao" })),
		createBuffer: vi.fn((): WebGLBuffer => ({ id: "buffer" })),
		bindVertexArray: vi.fn<WebGL2RenderingContext["bindVertexArray"]>(),
		bindBuffer: vi.fn<WebGL2RenderingContext["bindBuffer"]>(),
		bufferData: vi.fn(
			(
				_target: number,
				_data: number | AllowSharedBufferSource | null,
				_usage: number,
				_sourceOffset?: number,
				_length?: number,
			) => {},
		),
		enableVertexAttribArray:
			vi.fn<WebGL2RenderingContext["enableVertexAttribArray"]>(),
		vertexAttribPointer: vi.fn<WebGL2RenderingContext["vertexAttribPointer"]>(),
		deleteVertexArray: vi.fn<WebGL2RenderingContext["deleteVertexArray"]>(),
		deleteBuffer: vi.fn<WebGL2RenderingContext["deleteBuffer"]>(),
		getUniformLocation: vi.fn((_program: WebGLProgram, name: string) => {
			const location: WebGLUniformLocation = { name };
			uniforms.set(name, location);
			return location;
		}),
		useProgram: vi.fn<WebGL2RenderingContext["useProgram"]>(),
		uniform1f: vi.fn<WebGL2RenderingContext["uniform1f"]>(),
		uniform2f: vi.fn<WebGL2RenderingContext["uniform2f"]>(),
		uniform3f: vi.fn<WebGL2RenderingContext["uniform3f"]>(),
		viewport: vi.fn<WebGL2RenderingContext["viewport"]>(),
		drawArrays: vi.fn<WebGL2RenderingContext["drawArrays"]>(),
		getExtension: vi.fn(() => null),
	} satisfies Partial<WebGL2RenderingContext> & {
		uniforms: Map<string, WebGLUniformLocation>;
	};
}

class TestResizeObserver {
	static instances: TestResizeObserver[] = [];
	readonly targets = new Set<Element>();
	readonly observe = vi.fn((target: Element) => this.targets.add(target));
	readonly unobserve = vi.fn((target: Element) => this.targets.delete(target));
	readonly disconnect = vi.fn(() => this.targets.clear());

	constructor(private readonly callback: ResizeObserverCallback) {
		TestResizeObserver.instances.push(this);
	}

	emit(width: number, height: number) {
		const entries = [...this.targets].map((target) => ({
			target,
			contentRect: new DOMRect(0, 0, width, height),
			borderBoxSize: [{ inlineSize: width, blockSize: height }],
			contentBoxSize: [{ inlineSize: width, blockSize: height }],
			devicePixelContentBoxSize: [{ inlineSize: width, blockSize: height }],
		}));
		this.callback(entries, this);
	}
}

const colors = ["#ffffff", "#809c7d", "#b1ae51", "#1a2b3c"] as const;
const frames = new Map<number, FrameRequestCallback>();
const requestFrame = vi.fn((callback: FrameRequestCallback) => {
	const id = ++nextFrame;
	frames.set(id, callback);
	return id;
});
const cancelFrame = vi.fn((id: number) => frames.delete(id));
let nextFrame = 0;
let gl = createFakeGl();
let canvas: HTMLCanvasElement;
let parent: HTMLDivElement;
let renderer: GrainientRenderer | null = null;
let onError = vi.fn();
const originalDpr = Object.getOwnPropertyDescriptor(window, "devicePixelRatio");

function tick(timestamp: number) {
	const pending = [...frames.values()];
	frames.clear();
	for (const callback of pending) callback(timestamp);
}

function start() {
	renderer = createGrainientRenderer(canvas, {
		colors,
		dark: false,
		onError,
	});
	expect(renderer).not.toBeNull();
	if (!renderer) throw new Error("fixture renderer did not initialize");
	return renderer;
}

function latestTime() {
	return gl.uniform1f.mock.calls
		.filter(([location]) => location === gl.uniforms.get("iTime"))
		.at(-1)?.[1];
}

function latestBrightness() {
	return gl.uniform1f.mock.calls
		.filter(([location]) => location === gl.uniforms.get("uBrightness"))
		.at(-1)?.[1];
}

function expectResourcesReleased() {
	for (const shader of gl.createShader.mock.results) {
		expect(
			gl.deleteShader.mock.calls.filter(
				([deleted]) => deleted === shader.value,
			),
		).toHaveLength(1);
	}
	for (const result of gl.createProgram.mock.results) {
		expect(
			gl.deleteProgram.mock.calls.filter(
				([deleted]) => deleted === result.value,
			),
		).toHaveLength(1);
	}
	for (const result of gl.createVertexArray.mock.results) {
		expect(
			gl.deleteVertexArray.mock.calls.filter(
				([deleted]) => deleted === result.value,
			),
		).toHaveLength(1);
	}
	for (const result of gl.createBuffer.mock.results) {
		expect(
			gl.deleteBuffer.mock.calls.filter(
				([deleted]) => deleted === result.value,
			),
		).toHaveLength(1);
	}
	expect(frames.size).toBe(0);
}

beforeEach(() => {
	vi.clearAllMocks();
	frames.clear();
	nextFrame = 0;
	TestResizeObserver.instances = [];
	gl = createFakeGl();
	onError = vi.fn();
	renderer = null;
	parent = document.createElement("div");
	canvas = document.createElement("canvas");
	parent.appendChild(canvas);
	document.body.appendChild(parent);
	vi.spyOn(parent, "getBoundingClientRect").mockReturnValue(
		new DOMRect(0, 0, 800, 600),
	);
	// Replace only this instance's browser boundary. The partial fake is checked
	// against WebGL2 above; production code still calls the real renderer API.
	Object.defineProperty(canvas, "getContext", {
		configurable: true,
		value: vi.fn((_kind: string) => gl),
	});
	vi.stubGlobal("ResizeObserver", TestResizeObserver);
	vi.stubGlobal("requestAnimationFrame", requestFrame);
	vi.stubGlobal("cancelAnimationFrame", cancelFrame);
	Object.defineProperty(window, "devicePixelRatio", {
		configurable: true,
		value: 3,
	});
});

afterEach(() => {
	renderer?.destroy();
	parent.remove();
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
	if (originalDpr)
		Object.defineProperty(window, "devicePixelRatio", originalDpr);
	else Reflect.deleteProperty(window, "devicePixelRatio");
});

describe("Grainient renderer runtime", () => {
	it("accepts the CSS optimizer's short white hex without silently falling back", () => {
		renderer = createGrainientRenderer(canvas, {
			colors: ["#fff", "#809c7d", "#b1ae51", "#1a2b3c"],
			dark: false,
			onError,
		});
		expect(renderer).not.toBeNull();
		expect(gl.uniform3f).toHaveBeenCalledWith(
			gl.uniforms.get("uColor1"),
			1,
			1,
			1,
		);
		expect(gl.drawArrays).toHaveBeenCalledTimes(1);
	});

	it("draws one initial frame but stays paused until explicitly resumed", () => {
		start();
		expect(canvas.getContext).toHaveBeenCalledWith(
			"webgl2",
			expect.objectContaining({
				antialias: false,
				depth: false,
				failIfMajorPerformanceCaveat: true,
				preserveDrawingBuffer: false,
				powerPreference: "low-power",
			}),
		);
		expect(gl.drawArrays).toHaveBeenCalledTimes(1);
		expect(requestFrame).not.toHaveBeenCalled();
		expect(latestTime()).toBe(0);
		for (const [index, expected] of [
			[1, [1, 1, 1]],
			[2, [128 / 255, 156 / 255, 125 / 255]],
			[3, [177 / 255, 174 / 255, 81 / 255]],
			[4, [26 / 255, 43 / 255, 60 / 255]],
		] as const) {
			expect(gl.uniform3f).toHaveBeenCalledWith(
				gl.uniforms.get(`uColor${index}`),
				...expected,
			);
		}
	});

	it("caps draws at 24fps on a 120Hz clock without creating duplicate RAF loops", () => {
		const active = start();
		gl.drawArrays.mockClear();
		active.resume();
		active.resume();
		expect(frames.size).toBe(1);
		for (let frame = 0; frame <= 120; frame++) {
			tick((frame * 1000) / 120);
			expect(frames.size).toBe(1);
		}
		expect(gl.drawArrays.mock.calls.length).toBeGreaterThan(18);
		expect(gl.drawArrays.mock.calls.length).toBeLessThanOrEqual(24);
		active.pause();
		expect(frames.size).toBe(0);
	});

	it("bounds DPR and total drawing pixels while following the viewport size", () => {
		start();
		expect([canvas.width, canvas.height]).toEqual([600, 450]);
		const observer = TestResizeObserver.instances[0];
		expect(observer.targets.has(parent)).toBe(true);
		observer.emit(8000, 4000);
		expect(canvas.width * canvas.height).toBeLessThanOrEqual(320_000);
		expect(canvas.width / canvas.height).toBeCloseTo(2, 2);
		expect(gl.viewport).toHaveBeenLastCalledWith(
			0,
			0,
			canvas.width,
			canvas.height,
		);
		observer.emit(390, 844);
		expect([canvas.width, canvas.height]).toEqual([293, 633]);
		observer.emit(0, 0);
		expect([canvas.width, canvas.height]).toEqual([1, 1]);
		expect(requestFrame).not.toHaveBeenCalled();
	});

	it("updates dark mode while paused without resetting animation time", () => {
		const active = start();
		active.resume();
		tick(0);
		tick(50);
		expect(latestTime()).toBeCloseTo(0.05, 6);
		active.pause();
		tick(50_000);
		expect(latestTime()).toBeCloseTo(0.05, 6);
		active.setDarkMode(true);
		expect(latestBrightness()).toBe(0.12);
		expect(latestTime()).toBeCloseTo(0.05, 6);
		active.setDarkMode(false);
		expect(latestBrightness()).toBe(1);
		expect(latestTime()).toBeCloseTo(0.05, 6);
		active.resume();
		tick(100_000);
		tick(100_050);
		expect(latestTime()).toBeCloseTo(0.1, 6);
	});

	it("deletes every allocated GPU handle and stops observers and RAF exactly once", () => {
		const active = start();
		active.resume();
		active.destroy();
		active.destroy();
		expectResourcesReleased();
		expect(gl.detachShader).toHaveBeenCalledTimes(2);
		expect(TestResizeObserver.instances[0].disconnect).toHaveBeenCalledTimes(1);
		gl.drawArrays.mockClear();
		active.resume();
		tick(100);
		TestResizeObserver.instances[0].emit(100, 100);
		expect(gl.drawArrays).not.toHaveBeenCalled();
		expect(onError).not.toHaveBeenCalled();
	});

	it.each([
		"compile",
		"link",
	])("cleans partial GPU allocations after a %s failure", (phase) => {
		if (phase === "compile")
			gl.getShaderParameter
				.mockReturnValueOnce(true)
				.mockReturnValueOnce(false);
		else gl.getProgramParameter.mockReturnValue(false);
		renderer = createGrainientRenderer(canvas, {
			colors,
			dark: false,
			onError,
		});
		expect(renderer).toBeNull();
		expectResourcesReleased();
		expect(gl.drawArrays).not.toHaveBeenCalled();
		expect(onError).not.toHaveBeenCalled();
	});

	it("reports context loss only once and cannot restart the failed renderer", () => {
		const active = start();
		active.resume();
		canvas.dispatchEvent(new Event("webglcontextlost", { cancelable: true }));
		canvas.dispatchEvent(new Event("webglcontextlost", { cancelable: true }));
		expect(onError).toHaveBeenCalledTimes(1);
		expectResourcesReleased();
		active.resume();
		expect(frames.size).toBe(0);
	});

	it("contains draw and error-callback failures while releasing the active context resources", () => {
		const active = start();
		gl.drawArrays.mockImplementation(() => {
			throw new Error("fixture GPU draw failed");
		});
		onError.mockImplementation(() => {
			throw new Error("fixture callback failed");
		});
		active.resume();
		tick(0);
		expect(() => tick(50)).not.toThrow();
		expect(onError).toHaveBeenCalledTimes(1);
		expectResourcesReleased();
	});
});
