import { within } from "@testing-library/dom";
import { useState } from "react";
import { expect, vi } from "vitest";
import { userEvent } from "vitest/browser";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import type { MediaUploadRepo } from "@/lib/media/upload";
import type { ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";
import "./dom-matchers";
import { waitFor } from "./dom";
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
					width: options.width,
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

type ToolbarScope = HTMLElement | ReturnType<typeof within>;
type ToolbarUser = Pick<typeof userEvent, "click">;

async function clickHistoryButton(
	scope: ToolbarScope,
	pattern: RegExp,
	label: string | undefined,
	user: ToolbarUser,
) {
	const ui = scope instanceof HTMLElement ? within(scope) : scope;
	const button = await waitFor(() => {
		const found = ui.getByRole("button", { name: label ?? pattern });
		expect(found).toBeEnabledSync();
		return found;
	});
	await user.click(button);
}

/** 重試到「復原」按鈕出現且可按（可選：標籤的步數吻合）再點擊。 */
export function clickUndo(
	scope: ToolbarScope,
	user: ToolbarUser = userEvent,
	steps?: number,
) {
	return clickHistoryButton(
		scope,
		/^復原（/,
		steps === undefined ? undefined : `復原（⌘/Ctrl+Z，還可復原 ${steps} 步）`,
		user,
	);
}

/** 重試到「重做」按鈕出現且可按（可選：標籤的步數吻合）再點擊。 */
export function clickRedo(
	scope: ToolbarScope,
	user: ToolbarUser = userEvent,
	steps?: number,
) {
	return clickHistoryButton(
		scope,
		/^重做（/,
		steps === undefined
			? undefined
			: `重做（⌘/Ctrl+Shift+Z，還可重做 ${steps} 步）`,
		user,
	);
}
