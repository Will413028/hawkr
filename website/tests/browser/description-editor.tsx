import { useState } from "react";
import { vi } from "vitest";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import type { MediaUploadRepo } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";
import "./dom-matchers";
import { renderWithIntl } from "./render";

export interface EditorArgs {
	value: ProductDescription;
	onChange: ReturnType<typeof vi.fn<(next: ProductDescription) => void>>;
	uploadImageFile: ReturnType<typeof vi.fn<MediaUploadRepo["uploadImageFile"]>>;
}

/**
 * 掛上商品描述編輯器：value 只存在本地 state，onChange 與上傳都是 spy；
 * 預設的上傳會丟錯（正式上傳不在這些測試的範圍），要用的測試自己 mockImplementation。
 */
export function mountEditor(
	overrides: Partial<Pick<EditorArgs, "value" | "uploadImageFile">> = {},
	options: {
		theme?: "light" | "dark";
		width?: number;
		fullWidth?: boolean;
	} = {},
) {
	const args: EditorArgs = {
		value: overrides.value ?? { version: "1.0", blocks: [] },
		onChange: vi.fn<(next: ProductDescription) => void>(),
		uploadImageFile:
			overrides.uploadImageFile ??
			vi.fn<MediaUploadRepo["uploadImageFile"]>(async () => {
				throw new Error("此測試不建立媒體上傳交易");
			}),
	};
	function Controlled() {
		const [value, setValue] = useState(args.value);
		return (
			<div
				style={{
					width: options.width ?? (options.fullWidth ? "100%" : undefined),
					maxWidth: options.width ? "100%" : 1040,
					marginInline: "auto",
				}}
			>
				<DescriptionEditorCanvas
					value={value}
					uploadImageFile={args.uploadImageFile}
					onChange={(next) => {
						setValue(next);
						args.onChange(next);
					}}
				/>
			</div>
		);
	}
	const { container } = renderWithIntl(<Controlled />, {
		theme: options.theme,
	});
	return { container, args };
}
