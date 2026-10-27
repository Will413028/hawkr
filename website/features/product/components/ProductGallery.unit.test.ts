// @vitest-environment jsdom

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import type { ImageProps } from "next/image";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import ProductGallery from "./ProductGallery";

vi.mock("next/image", () => ({
	default: ({
		src,
		priority,
		fill: _fill,
		unoptimized: _unoptimized,
		...props
	}: ImageProps) =>
		createElement("img", {
			...props,
			src: typeof src === "string" ? src : "",
			"data-priority": String(priority === true),
		}),
}));

vi.mock("@/components/shared/ImageFallback", () => ({
	default: ({ label }: { label: string }) => createElement("span", null, label),
}));

const MEDIA_BASE = "https://test-media.example.com";
const images = Array.from({ length: 9 }, (_, index) =>
	index === 1
		? "public/product/gallery/animation.GIF"
		: `public/product/gallery/image-${index + 1}.jpg`,
);

let galleryWidth: number;
let observers: TestResizeObserver[];
let densityQueries: MediaQueryList[];

class TestResizeObserver {
	target: Element | undefined;
	disconnect = vi.fn();

	constructor(private callback: ResizeObserverCallback) {
		observers.push(this);
	}

	observe(target: Element) {
		this.target = target;
	}

	emit(width: number) {
		if (!this.target) throw new Error("No gallery is observed");
		this.callback(
			[
				{
					target: this.target,
					contentRect: new DOMRect(0, 0, width, width * 0.75),
				} as ResizeObserverEntry,
			],
			this as unknown as ResizeObserver,
		);
	}
}

function previews(): HTMLImageElement[] {
	return Array.from(
		document.querySelectorAll<HTMLImageElement>(
			"[data-product-gallery-track] img[data-gallery-preview]",
		),
	);
}

function details(): HTMLImageElement[] {
	return Array.from(
		document.querySelectorAll<HTMLImageElement>(
			"[data-product-gallery-track] img[data-gallery-detail]",
		),
	);
}

function detailAt(index: number): HTMLImageElement {
	const image = previews()[
		index
	].parentElement?.querySelector<HTMLImageElement>("img[data-gallery-detail]");
	if (!image) throw new Error(`No larger image for slide ${index + 1}`);
	return image;
}

function renderGallery(thumbnails = images) {
	return render(
		createElement(ProductGallery, {
			thumbnails,
			productName: "在地番茄",
		}),
	);
}

function resizeGallery(width: number) {
	galleryWidth = width;
	act(() => observers[0].emit(width));
}

function changeDensity(density: number) {
	vi.stubGlobal("devicePixelRatio", density);
	const current = densityQueries.at(-1);
	if (!current) throw new Error("Pixel density is not being observed");
	act(() => current.dispatchEvent(new Event("change")));
}

beforeEach(() => {
	__setEnvForTest({ ...getEnv(), flavor: "dev", mediaBaseUrl: MEDIA_BASE });
	galleryWidth = 390;
	observers = [];
	densityQueries = [];
	vi.stubGlobal("devicePixelRatio", 3);
	vi.stubGlobal("ResizeObserver", TestResizeObserver);
	vi.stubGlobal(
		"matchMedia",
		vi.fn((media: string) => {
			const query = Object.assign(new EventTarget(), {
				media,
				matches: media.startsWith("(resolution:"),
				onchange: null,
				addListener: vi.fn(),
				removeListener: vi.fn(),
			}) as MediaQueryList;
			vi.spyOn(query, "removeEventListener");
			if (media.startsWith("(resolution:")) densityQueries.push(query);
			return query;
		}),
	);
	vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
		() => new DOMRect(0, 0, galleryWidth, galleryWidth * 0.75),
	);
});

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
	__setEnvForTest(null);
});

describe("ProductGallery measured media", () => {
	it("renders only reusable previews before the browser measures the box", () => {
		const html = renderToString(
			createElement(ProductGallery, {
				thumbnails: [images[0]],
				productName: "在地番茄",
			}),
		);
		expect(html).toContain("/i/card/");
		expect(html).not.toContain("data-gallery-detail");
		expect(html).not.toContain("/i/detail/");
		expect(html).not.toContain("/i/banner/");
	});

	it("reuses 720px previews for a 350px box at 2x without another image layer", () => {
		galleryWidth = 350;
		vi.stubGlobal("devicePixelRatio", 2);
		renderGallery();
		expect(previews()).toHaveLength(9);
		expect(details()).toHaveLength(0);
		expect(previews()[0].src).toBe(`${MEDIA_BASE}/i/card/${images[0]}`);
		expect(previews()[0].getAttribute("data-priority")).toBe("true");
		expect(previews()[0].getAttribute("fetchpriority")).toBe("high");
		expect(previews()[1].getAttribute("loading")).toBe("eager");
		expect(previews()[2].getAttribute("loading")).toBe("lazy");
	});

	it.each([
		{ width: 390, density: 3, preset: "detail" },
		{ width: 700, density: 2, preset: "banner" },
	])("uses $preset for a $width px box at $density x", ({
		width,
		density,
		preset,
	}) => {
		galleryWidth = width;
		vi.stubGlobal("devicePixelRatio", density);
		renderGallery();
		expect(details()).toHaveLength(8);
		for (const [index, preview] of previews().entries()) {
			if (index === 1) {
				expect(preview.src).toBe(`${MEDIA_BASE}/i/motion/${images[index]}`);
				expect(preview.parentElement?.querySelectorAll("img")).toHaveLength(1);
				continue;
			}
			const detail = detailAt(index);
			expect(detail.src).toBe(`${MEDIA_BASE}/i/${preset}/${images[index]}`);
			expect(detail.getAttribute("data-priority")).toBe("false");
			expect(detail.getAttribute("loading")).toBe(
				index === 0 ? "eager" : "lazy",
			);
			expect(detail.getAttribute("fetchpriority")).toBe(
				index === 0 ? "high" : "low",
			);
		}
		expect(
			previews().filter(
				(image) => image.getAttribute("data-priority") === "true",
			),
		).toHaveLength(1);
	});

	it("promotes selected images and their neighbors while retaining decoded images on return", () => {
		renderGallery();
		const track = document.querySelector("[data-product-gallery-track]");
		const selected = detailAt(3);
		fireEvent.click(screen.getByRole("button", { name: "查看商品圖片 4" }));
		expect(document.querySelector("[data-product-gallery-track]")).toBe(track);
		expect(detailAt(3)).toBe(selected);
		expect(selected.getAttribute("fetchpriority")).toBe("high");
		expect(previews()[3].getAttribute("fetchpriority")).toBe("high");
		expect(selected.parentElement?.getAttribute("aria-hidden")).toBe("false");
		expect(previews()[3].alt).toBe("在地番茄");
		for (const index of [2, 3, 4]) {
			expect(detailAt(index).getAttribute("loading")).toBe("eager");
			expect(previews()[index].getAttribute("loading")).toBe("eager");
		}
		fireEvent.load(selected);
		expect(selected.className).toContain("opacity-100");
		fireEvent.click(screen.getByRole("button", { name: "查看商品圖片 2" }));
		expect(selected.getAttribute("loading")).toBe("lazy");
		expect(selected.getAttribute("fetchpriority")).toBe("low");
		expect(previews()[1].getAttribute("loading")).toBe("eager");
		expect(previews()[1].getAttribute("fetchpriority")).toBe("high");
		fireEvent.click(screen.getByRole("button", { name: "查看商品圖片 4" }));
		expect(detailAt(3)).toBe(selected);
		expect(selected.className).toContain("opacity-100");
	});

	it("resizes through the available presets using fractional container width", () => {
		galleryWidth = 360;
		vi.stubGlobal("devicePixelRatio", 2);
		renderGallery([images[0]]);
		expect(details()).toHaveLength(0);
		resizeGallery(360.25);
		expect(detailAt(0).src).toContain("/i/detail/");
		const detail = detailAt(0);
		resizeGallery(390);
		expect(detailAt(0)).toBe(detail);
		resizeGallery(700);
		expect(detailAt(0).src).toContain("/i/banner/");
		resizeGallery(350);
		expect(details()).toHaveLength(0);
		expect(previews()).toHaveLength(1);
	});

	it("updates on density changes without a resize and releases its observers", () => {
		galleryWidth = 350;
		vi.stubGlobal("devicePixelRatio", 2);
		const { unmount } = renderGallery([images[0]]);
		const firstQuery = densityQueries[0];
		expect(firstQuery.media).toBe("(resolution: 2dppx)");
		expect(details()).toHaveLength(0);
		changeDensity(3);
		expect(detailAt(0).src).toContain("/i/detail/");
		expect(firstQuery.removeEventListener).toHaveBeenCalledWith(
			"change",
			expect.any(Function),
		);
		expect(densityQueries.at(-1)?.media).toBe("(resolution: 3dppx)");
		changeDensity(4);
		expect(detailAt(0).src).toContain("/i/banner/");
		const lastQuery = densityQueries.at(-1);
		unmount();
		expect(observers[0].disconnect).toHaveBeenCalledOnce();
		expect(lastQuery?.removeEventListener).toHaveBeenCalledWith(
			"change",
			expect.any(Function),
		);
	});

	it("remeasures on window resize when ResizeObserver is unavailable", () => {
		vi.stubGlobal("ResizeObserver", undefined);
		galleryWidth = 350;
		vi.stubGlobal("devicePixelRatio", 2);
		renderGallery([images[0]]);
		expect(details()).toHaveLength(0);
		galleryWidth = 700;
		fireEvent(window, new Event("resize"));
		expect(detailAt(0).src).toContain("/i/banner/");
	});

	it("resets loading and failure state when upgrading to a different image size", () => {
		galleryWidth = 390;
		vi.stubGlobal("devicePixelRatio", 2);
		renderGallery([images[0]]);
		const preview = previews()[0];
		const detail = detailAt(0);
		expect(detail.className).toContain("opacity-0");
		fireEvent.load(detail);
		expect(detail.className).toContain("opacity-100");
		resizeGallery(700);
		const banner = detailAt(0);
		expect(banner).not.toBe(detail);
		expect(banner.src).toContain("/i/banner/");
		expect(banner.className).toContain("opacity-0");
		expect(previews()[0]).toBe(preview);
		fireEvent.error(banner);
		expect(details()).toHaveLength(0);
		expect(preview.isConnected).toBe(true);
		resizeGallery(390);
		const retried = detailAt(0);
		expect(retried.src).toContain("/i/detail/");
		expect(retried.className).toContain("opacity-0");
		fireEvent.load(retried);
		expect(retried.className).toContain("opacity-100");
	});

	it("uses deferred thumbnail derivatives and preserves GIF animation", () => {
		renderGallery();
		const thumbs = screen.getAllByAltText(/^縮圖 /) as HTMLImageElement[];
		expect(thumbs).toHaveLength(9);
		for (const [index, thumb] of thumbs.entries()) {
			expect(thumb.getAttribute("loading")).toBe("lazy");
			expect(thumb.getAttribute("fetchpriority")).toBe("low");
			expect(thumb.getAttribute("data-priority")).toBe("false");
			expect(thumb.src).not.toContain("/i/original/");
			expect(thumb.src).toContain(images[index]);
		}
		expect(thumbs[1].src).toBe(`${MEDIA_BASE}/i/motion/${images[1]}`);
		// The square thumb derivative would crop the uploaded 4:3 image again.
		expect(thumbs[0].src).toBe(`${MEDIA_BASE}/i/card/${images[0]}`);
	});

	it("retains signed and foreign URLs without requesting duplicate image layers", () => {
		const opaque = [
			`${MEDIA_BASE}/i/original/public/product/gallery/signed.jpg?signature=opaque`,
			"https://images.example.com/product/original.jpg?version=1",
		];
		renderGallery(opaque);
		expect(details()).toHaveLength(0);
		expect(previews().map((image) => image.src)).toEqual(opaque);
		expect(
			(screen.getAllByAltText(/^縮圖 /) as HTMLImageElement[]).map(
				(image) => image.src,
			),
		).toEqual(opaque);
		resizeGallery(700);
		expect(details()).toHaveLength(0);
		expect(previews().map((image) => image.src)).toEqual(opaque);
	});

	it("keeps controls available before loading, deduplicates and caps the gallery", () => {
		render(
			createElement(ProductGallery, {
				thumbnails: ["", " ", images[0], ...images, "public/product/extra.jpg"],
				productName: "在地番茄",
				tags: ["在地", "當季"],
			}),
		);
		expect(previews()).toHaveLength(9);
		expect(
			screen.getAllByRole("button", { name: /^查看商品圖片 / }),
		).toHaveLength(9);
		expect(
			screen.getByRole("list", { name: "商品標籤" }).textContent,
		).toContain("#在地");
		fireEvent.click(screen.getByRole("button", { name: "查看商品圖片 9" }));
		expect(
			screen
				.getByRole("button", { name: "查看商品圖片 9" })
				.getAttribute("aria-pressed"),
		).toBe("true");
	});

	it("retains the empty-gallery fallback without requesting an empty image URL", () => {
		renderGallery(["", " "]);
		expect(document.querySelectorAll("img")).toHaveLength(0);
		expect(screen.getByText("尚無圖片")).toBeTruthy();
		expect(screen.queryByRole("button", { name: /^查看商品圖片 / })).toBeNull();
	});
});
