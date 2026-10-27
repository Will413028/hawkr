import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import { getEnv } from "@/lib/config/env";
import type { MediaUploadRepo, UploadedAsset } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";
import {
	descriptionImageDataUrl,
	descriptionImageSvg,
	startDescriptionImageFixtures,
} from "../../.storybook/description-image-fixtures";

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

const meta = {
	title: "Web/Seller Center/DescriptionEditor Image",
	component: DescriptionEditorCanvas,
	beforeEach: startDescriptionImageFixtures,
	parameters: {
		layout: "padded",
		docs: {
			description: {
				component:
					"商品描述圖片：加入、上傳進度、失敗重試、替換與復原。圖片沒有可編輯說明，既有 alt 仍保留。SVG 為本地幾何測試圖；只有這組故事啟用 MSW，攔截專用 CDN fixture 路徑並回傳本地圖片。上傳方法只回傳測試 token，不連接上傳服務或商品交易。",
			},
		},
	},
	args: {
		value: textDocument,
		onChange: fn<(value: ProductDescription) => void>(),
		uploadImageFile: fn<MediaUploadRepo["uploadImageFile"]>(
			async () => uploaded,
		),
	},
	render: function Controlled(args) {
		const [value, setValue] = useState(args.value);
		return (
			<div style={{ width: "100%", maxWidth: 1040, marginInline: "auto" }}>
				<DescriptionEditorCanvas
					{...args}
					value={value}
					onChange={(next) => {
						setValue(next);
						args.onChange(next);
					}}
				/>
			</div>
		);
	},
} satisfies Meta<typeof DescriptionEditorCanvas>;
export default meta;
type Story = StoryObj<typeof meta>;

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

export const EmptyPicker: Story = {
	name: "加入圖片 · 清楚的空白狀態",
	play: async ({ canvasElement, args }) => {
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
	},
};

export const UploadWithProgress: Story = {
	name: "上傳圖片 · 實際進度與完成",
	play: async ({ canvasElement, args }) => {
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
	},
};

export const RetryFailedUpload: Story = {
	name: "上傳失敗 · 保留檔案並重試",
	play: async ({ canvasElement, args }) => {
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
		await expect(args.uploadImageFile.mock.calls[1]?.[0]).toBe(selected);
		await assertDecodedImage(dialog);
	},
};

export const InvalidFile: Story = {
	name: "無效檔案 · 不上傳並保留原圖",
	args: { value: imageDocument },
	play: async ({ canvasElement, args }) => {
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
			{ applyAccept: false },
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
	},
};

export const ReplaceAndUndoRedo: Story = {
	name: "更換圖片 · 失敗保留與復原重做",
	args: { value: imageDocument },
	play: async ({ canvasElement, args }) => {
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
	},
};

export const RemoveAndUndo: Story = {
	name: "移除圖片 · 一步復原",
	args: { value: imageDocument },
	play: async ({ canvasElement, args }) => {
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
	},
};

export const RemoveDuringUpload: Story = {
	name: "上傳途中移除 · 完成後不復活",
	play: async ({ canvasElement, args }) => {
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
	},
};

export const ReadOnly: Story = {
	name: "閱讀預覽 · 圖片無編輯操作",
	args: { value: imageDocument },
	play: async ({ canvasElement, args }) => {
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
	},
};

export const NarrowDark: Story = {
	name: "窄版深色 · 圖片與操作",
	args: { value: imageDocument },
	globals: { theme: "dark" },
	decorators: [
		(Story) => (
			<div style={{ width: 375, maxWidth: "100%", marginInline: "auto" }}>
				<Story />
			</div>
		),
	],
	play: async ({ canvasElement, args }) => {
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
	},
};
