import { within } from "@testing-library/dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { userEvent } from "vitest/browser";
import { getEnv } from "@/lib/config/env";
import type { UploadedAsset } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import { mountEditor } from "@/tests/browser/description-editor";
import {
	descriptionImageDataUrl,
	descriptionImageSvg,
	startDescriptionImageFixtures,
} from "@/tests/browser/description-image-fixtures";
import { waitFor } from "@/tests/browser/dom";

// 媒體 origin 要在被測模組載入前就設定（getEnv 會記住第一次的結果）。fixture 攔的是 test-media.example.com，
// 與 .github/workflows/website-e2e.yml 的 NEXT_PUBLIC_MEDIA_BASE_URL 相同；沒設時 buildImageUrl 不組網址，圖片區塊會被判定資料無效。
vi.hoisted(() => {
	process.env.NEXT_PUBLIC_MEDIA_BASE_URL = "https://test-media.example.com";
});

const stopFixtures: Array<() => void> = [];
beforeEach(async () => {
	stopFixtures.push(await startDescriptionImageFixtures());
});
afterEach(() => {
	while (stopFixtures.length > 0) stopFixtures.pop()?.();
});

const newImageKey = "public/storybook/image-new.svg";

const uploaded: UploadedAsset = {
	// 上傳結果的 previewUrl 要落在設定的媒體 origin 才會還原成 object key（lib/media/cdn.ts），
	// 所以跟著 NEXT_PUBLIC_MEDIA_BASE_URL，不寫死主機。
	previewUrl: `${getEnv().mediaBaseUrl}/${newImageKey}`,
	stagedToken: "storybook-image-upload-token",
};

const textDocument: ProductDescription = {
	version: "1.0",
	blocks: [
		{
			id: "image-story-text",
			type: "text",
			runs: [{ text: "圖片搭配文字介紹商品特色" }],
		},
	],
};

const oldImage = {
	id: "image-story-existing",
	type: "image" as const,
	assetId: "storybook-old-asset",
	imageKey: "public/storybook/image-old.svg",
	imageUrl: descriptionImageDataUrl,
	alt: "既有商品圖片的替代文字",
};

const imageDocument: ProductDescription = {
	version: "1.0",
	blocks: [oldImage, ...textDocument.blocks],
};

async function setup(canvasElement: HTMLElement) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(() => expect(dialog).toHaveAttribute("data-ready", "true"), {
		timeout: 15_000,
	});
	return dialog;
}

function file() {
	return new File([descriptionImageSvg], "商品圖片.svg", {
		type: "image/svg+xml",
	});
}

function fileInput(dialog: HTMLElement) {
	return within(dialog).getByLabelText("選擇商品圖片") as HTMLInputElement;
}

async function insertImage(dialog: HTMLElement) {
	await userEvent.click(
		within(dialog).getByRole("button", { name: "插入或直接貼上圖片" }),
	);
	await waitFor(() => {
		// The hidden file input can exist while the new block is still being
		// revealed. Upload stories must wait for the same usable control as users.
		expect(fileInput(dialog)).toBeInTheDocument();
		const choose = within(dialog).getByRole("button", {
			name: /加入商品圖片/,
		});
		expect(choose).toBeVisible();
		expect(choose).toBeEnabled();
	});
}

function images(value: ProductDescription | undefined) {
	return value?.blocks.filter((block) => block.type === "image") ?? [];
}

function assertNoCaption(dialog: HTMLElement) {
	expect(
		dialog.querySelector('.hk-description-image [contenteditable="true"]'),
	).toBeNull();
	expect(dialog.querySelector(".image-tool__caption")).toBeNull();
}

async function assertDecodedImage(dialog: HTMLElement, alt?: string) {
	await waitFor(() => {
		const image = within(dialog).getByRole("img", alt ? { name: alt } : {});
		expect(image).toBeVisible();
		expect(image).toHaveStyle({ opacity: "1" });
		expect((image as HTMLImageElement).naturalWidth).toBeGreaterThan(0);
	});
	assertNoCaption(dialog);
}

function mount(
	overrides: Parameters<typeof mountEditor>[0] = {},
	options: Parameters<typeof mountEditor>[1] = {},
) {
	return mountEditor({ value: textDocument, ...overrides }, options);
}

describe("DescriptionEditorImage", () => {
	it("加入圖片 · 清楚的空白狀態", async () => {
		const { container: canvasElement, args } = mount({});

		const dialog = await setup(canvasElement);
		await insertImage(dialog);
		await waitFor(() =>
			expect(
				within(dialog).getByRole("button", { name: /加入商品圖片/ }),
			).toBeVisible(),
		);
		await expect(fileInput(dialog)).toHaveAttribute("accept", "image/*");
		assertNoCaption(dialog);
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});

	it("上傳圖片 · 實際進度與完成", async () => {
		const { container: canvasElement, args } = mount({});

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		let complete!: (asset: UploadedAsset) => void;
		let progress: ((ratio: number) => void) | undefined;
		const pending = new Promise<UploadedAsset>((resolve) => {
			complete = resolve;
		});
		args.uploadImageFile.mockImplementation((_file, onProgress) => {
			progress = onProgress;
			onProgress?.(0.37);
			return pending;
		});
		await insertImage(dialog);
		const selected = file();
		await userEvent.upload(fileInput(dialog), selected);
		await waitFor(() =>
			expect(ui.getByRole("progressbar", { name: "圖片上傳進度" })).toHaveValue(
				37,
			),
		);
		await expect(args.uploadImageFile).toHaveBeenCalledWith(
			selected,
			expect.any(Function),
			{ compress: false },
		);
		progress?.(0.72);
		await waitFor(() =>
			expect(ui.getByRole("progressbar", { name: "圖片上傳進度" })).toHaveValue(
				72,
			),
		);
		complete(uploaded);
		await waitFor(() => {
			expect(images(args.onChange.mock.lastCall?.[0])).toEqual([
				expect.objectContaining({
					imageKey: newImageKey,
					stagedToken: uploaded.stagedToken,
				}),
			]);
			expect(ui.queryByRole("progressbar")).not.toBeInTheDocument();
		});
		await assertDecodedImage(dialog);
	});

	it("上傳失敗 · 保留檔案並重試", async () => {
		const { container: canvasElement, args } = mount({});

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		args.uploadImageFile
			.mockRejectedValueOnce(new Error("本地測試上傳失敗"))
			.mockResolvedValueOnce(uploaded);
		await insertImage(dialog);
		const selected = file();
		await userEvent.upload(fileInput(dialog), selected);
		await expect(await ui.findByRole("alert")).toHaveTextContent(
			"圖片上傳失敗，請重試或選擇其他圖片。",
		);
		await expect(images(args.onChange.mock.lastCall?.[0])).toEqual([]);
		await userEvent.click(ui.getByRole("button", { name: "重試上傳" }));
		await waitFor(() =>
			expect(images(args.onChange.mock.lastCall?.[0])[0]?.stagedToken).toBe(
				uploaded.stagedToken,
			),
		);
		await expect(args.uploadImageFile).toHaveBeenCalledTimes(2);
		// vitest/browser 的 upload 在頁面內重建 File：不能比對測試端那個物件的身分，
		// 改驗檔名與型別，並驗重試帶的是第一次的同一個 File。
		const firstUpload = args.uploadImageFile.mock.calls[0]?.[0];
		expect(firstUpload?.name).toBe(selected.name);
		expect(firstUpload?.type).toBe(selected.type);
		expect(args.uploadImageFile.mock.calls[1]?.[0]).toBe(firstUpload);
		await assertDecodedImage(dialog);
	});

	it("無效檔案 · 不上傳並保留原圖", async () => {
		const { container: canvasElement, args } = mount({ value: imageDocument });

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		await assertDecodedImage(dialog, oldImage.alt);
		const assertRejected = async (message: string) => {
			await waitFor(() =>
				expect(ui.getByRole("alert")).toHaveTextContent(message),
			);
			await expect(args.uploadImageFile).not.toHaveBeenCalled();
			await expect(
				ui.queryByRole("button", { name: "重試上傳" }),
			).not.toBeInTheDocument();
			await expect(
				images(args.onChange.mock.lastCall?.[0] ?? args.value),
			).toEqual([
				expect.objectContaining({
					assetId: oldImage.assetId,
					imageKey: oldImage.imageKey,
					alt: oldImage.alt,
				}),
			]);
			await assertDecodedImage(dialog, oldImage.alt);
		};
		await userEvent.upload(
			fileInput(dialog),
			new File(["這不是圖片"], "商品資料.txt", { type: "text/plain" }),
		);
		await assertRejected("請選擇圖片檔案。");

		// Keep the oversized fixture valid SVG; whitespace after the root is
		// valid XML, so rejection specifically exercises the original file size.
		const oversized = new File(
			[
				descriptionImageSvg,
				" ".repeat(10 * 1024 * 1024 + 1 - descriptionImageSvg.length),
			],
			"超大商品圖片.svg",
			{ type: "image/svg+xml" },
		);
		await expect(oversized.size).toBe(10 * 1024 * 1024 + 1);
		await userEvent.upload(fileInput(dialog), oversized);
		await assertRejected("圖片不可超過 10 MB，請選擇較小的圖片。");
	});

	it("更換圖片 · 失敗保留與復原重做", async () => {
		const { container: canvasElement, args } = mount({ value: imageDocument });

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		args.uploadImageFile
			.mockRejectedValueOnce(new Error("本地測試更換失敗"))
			.mockResolvedValueOnce(uploaded);
		await assertDecodedImage(dialog, oldImage.alt);
		await userEvent.upload(fileInput(dialog), file());
		await expect(await ui.findByRole("alert")).toBeVisible();
		await expect(
			images(args.onChange.mock.lastCall?.[0] ?? args.value),
		).toEqual([
			expect.objectContaining({
				assetId: oldImage.assetId,
				imageKey: oldImage.imageKey,
				alt: oldImage.alt,
			}),
		]);
		await assertDecodedImage(dialog, oldImage.alt);
		await userEvent.click(ui.getByRole("button", { name: "重試上傳" }));
		await waitFor(() => {
			const image = images(args.onChange.mock.lastCall?.[0])[0];
			expect(image?.imageKey).toBe(newImageKey);
			expect(image?.stagedToken).toBe(uploaded.stagedToken);
			expect(image?.assetId).toBeUndefined();
		});
		await assertDecodedImage(dialog);
		await userEvent.click(ui.getByRole("button", { name: /^復原（/ }));
		await waitFor(() =>
			expect(images(args.onChange.mock.lastCall?.[0])).toEqual([
				expect.objectContaining({
					assetId: oldImage.assetId,
					imageKey: oldImage.imageKey,
					alt: oldImage.alt,
				}),
			]),
		);
		await assertDecodedImage(dialog, oldImage.alt);
		await userEvent.click(ui.getByRole("button", { name: /^重做（/ }));
		await waitFor(() => {
			const image = images(args.onChange.mock.lastCall?.[0])[0];
			expect(image?.imageKey).toBe(newImageKey);
			expect(image?.stagedToken).toBe(uploaded.stagedToken);
			expect(image?.assetId).toBeUndefined();
		});
		await assertDecodedImage(dialog);
	});

	it("移除圖片 · 一步復原", async () => {
		const { container: canvasElement, args } = mount({ value: imageDocument });

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		await userEvent.click(ui.getByRole("button", { name: "移除圖片" }));
		await waitFor(() => {
			expect(images(args.onChange.mock.lastCall?.[0])).toEqual([]);
			expect(ui.queryByRole("img")).not.toBeInTheDocument();
		});
		await userEvent.click(ui.getByRole("button", { name: /^復原（/ }));
		await waitFor(() =>
			expect(images(args.onChange.mock.lastCall?.[0])).toEqual([
				expect.objectContaining({
					assetId: oldImage.assetId,
					imageKey: oldImage.imageKey,
					alt: oldImage.alt,
				}),
			]),
		);
		await assertDecodedImage(dialog, oldImage.alt);
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});

	it("上傳途中移除 · 完成後不復活", async () => {
		const { container: canvasElement, args } = mount({});

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		let complete!: (asset: UploadedAsset) => void;
		const pending = new Promise<UploadedAsset>((resolve) => {
			complete = resolve;
		});
		args.uploadImageFile.mockImplementation((_file, onProgress) => {
			onProgress?.(0.42);
			return pending;
		});
		await insertImage(dialog);
		await userEvent.upload(fileInput(dialog), file());
		await waitFor(() =>
			expect(ui.getByRole("progressbar", { name: "圖片上傳進度" })).toHaveValue(
				42,
			),
		);
		await expect(ui.getByRole("button", { name: "移除圖片" })).toBeEnabled();
		await userEvent.click(ui.getByRole("button", { name: "移除圖片" }));
		await waitFor(() => {
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(ui.queryByLabelText("選擇商品圖片")).not.toBeInTheDocument();
			expect(ui.queryByRole("img")).not.toBeInTheDocument();
			expect(images(args.onChange.mock.lastCall?.[0] ?? args.value)).toEqual(
				[],
			);
		});

		// The network response arrives only after the block is gone. Let the
		// upload continuation run, then serialize through the actual preview UI.
		complete(uploaded);
		await pending;
		await new Promise<void>((resolve) =>
			requestAnimationFrame(() => resolve()),
		);
		await userEvent.click(ui.getByRole("button", { name: "閱讀預覽" }));
		await waitFor(() => {
			expect(dialog).toHaveAttribute("data-preview", "true");
			expect(dialog).toHaveAttribute("aria-busy", "false");
			expect(ui.queryByRole("img")).not.toBeInTheDocument();
			expect((args.onChange.mock.lastCall?.[0] ?? args.value).blocks).toEqual(
				textDocument.blocks,
			);
		});
		await expect(
			args.onChange.mock.calls.every(([value]) => images(value).length === 0),
		).toBe(true);
		await expect(args.uploadImageFile).toHaveBeenCalledTimes(1);
	});

	it("閱讀預覽 · 圖片無編輯操作", async () => {
		const { container: canvasElement, args } = mount({ value: imageDocument });

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		await userEvent.click(ui.getByRole("button", { name: "閱讀預覽" }));
		await waitFor(() => expect(dialog).toHaveAttribute("data-preview", "true"));
		await assertDecodedImage(dialog, oldImage.alt);
		await expect(ui.queryByRole("button", { name: "更換圖片" })).toBeNull();
		await expect(ui.queryByRole("button", { name: "移除圖片" })).toBeNull();
		await expect(ui.queryByLabelText("選擇商品圖片")).toBeNull();
		await expect(
			images(args.onChange.mock.lastCall?.[0] ?? args.value)[0]?.alt,
		).toBe(oldImage.alt);
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});

	it("窄版深色 · 圖片與操作", async () => {
		const { container: canvasElement, args } = mount(
			{ value: imageDocument },
			{ width: 375, theme: "dark" },
		);

		const dialog = await setup(canvasElement);
		const ui = within(dialog);
		await assertDecodedImage(dialog, oldImage.alt);
		await expect(dialog.getBoundingClientRect().width).toBeLessThanOrEqual(375);
		await expect(dialog.scrollWidth).toBeLessThanOrEqual(
			dialog.clientWidth + 1,
		);
		await expect(ui.getByRole("button", { name: "更換圖片" })).toBeVisible();
		await expect(ui.getByRole("button", { name: "移除圖片" })).toBeVisible();
		await expect(args.uploadImageFile).not.toHaveBeenCalled();
	});
});
