import { useState } from "react";
import { describe, expect, it } from "vitest";
import { page } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import {
	AfterSalePhotoUploader,
	type StagedPhoto,
} from "./AfterSalePhotoUploader";

const PREVIEW =
	"data:image/svg+xml;utf8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='96' height='96'%3E%3Crect width='96' height='96' fill='%23e8efe3'/%3E%3C/svg%3E";

const PHOTOS: StagedPhoto[] = [
	{ token: "token-a", previewUrl: PREVIEW },
	{ token: "token-b", previewUrl: PREVIEW },
];

function Controlled(
	props: Partial<React.ComponentProps<typeof AfterSalePhotoUploader>>,
) {
	const [value, setValue] = useState(PHOTOS);
	return (
		<AfterSalePhotoUploader
			max={5}
			label="佐證圖片（選填）"
			{...props}
			value={value}
			onChange={setValue}
		/>
	);
}

const frame = { frameMaxWidth: 420 };

describe("AfterSalePhotoUploader", () => {
	it("shows the per-photo error", async () => {
		renderWithIntl(
			<Controlled
				itemErrors={{ "token-b": "這張照片已過期，請移除後重新新增" }}
			/>,
			frame,
		);
		await expect
			.element(
				page.getByText("這張照片已過期，請移除後重新新增", { exact: true }),
			)
			.toBeVisible();
	});

	it("shows the field error", async () => {
		renderWithIntl(<Controlled error="佐證圖片最多 5 張" />, frame);
		await expect
			.element(page.getByText("佐證圖片最多 5 張", { exact: true }))
			.toBeVisible();
	});
});
