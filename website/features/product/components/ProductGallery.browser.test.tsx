import { afterEach, describe, expect, it } from "vitest";
import { page, userEvent } from "vitest/browser";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import { renderWithIntl } from "@/tests/browser/render";
import ProductGallery from "./ProductGallery";

const thumbnails = ["#b8441f", "#e4572e", "#1f2933", "#1e5fab"].map(
	(color, index) =>
		`data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="700" height="525"><rect width="700" height="525" fill="${color}"/><text x="350" y="285" text-anchor="middle" font-size="64" fill="white">${index + 1}</text></svg>`)}`,
);
const fixtureThumbnails = [1, 2, 3, 4].map(
	(index) => `public/product/gallery-fixture/${index}.svg`,
);

function thumbnailButton(index: number) {
	return page.getByRole("button", {
		name: `查看商品圖片 ${index}`,
		exact: true,
	});
}

async function clickEachThumbnail() {
	for (const index of [4, 2, 3, 1, 4]) {
		await userEvent.click(thumbnailButton(index));
		await expect
			.element(thumbnailButton(index))
			.toHaveAttribute("aria-pressed", "true");
	}
}

afterEach(() => {
	__setEnvForTest(null);
});

describe("ProductGallery", () => {
	it("marks the clicked thumbnail as pressed", async () => {
		renderWithIntl(
			<ProductGallery
				thumbnails={thumbnails}
				productName="商品圖片測試"
				tags={[]}
			/>,
			{ frameMaxWidth: 700 },
		);
		await clickEachThumbnail();
	});

	it("marks the clicked thumbnail as pressed with local gateway fixtures", async () => {
		__setEnvForTest({
			...getEnv(),
			flavor: "dev",
			mediaBaseUrl: window.location.origin,
		});
		renderWithIntl(
			<ProductGallery
				thumbnails={fixtureThumbnails}
				productName="商品圖片測試"
				tags={[]}
			/>,
			{ frameMaxWidth: 700 },
		);
		await clickEachThumbnail();
	});

	it("moves between cards with arrow, End and Home keys", async () => {
		renderWithIntl(
			<ProductGallery
				thumbnails={thumbnails}
				productName="商品圖片測試"
				tags={["當季"]}
			/>,
			{ frameMaxWidth: 700 },
		);
		const gallery = page.getByRole("region", { name: "商品圖片", exact: true });
		await expect.element(gallery).toBeInTheDocument();
		(gallery.element() as HTMLElement).focus();
		await userEvent.keyboard("{ArrowRight}");
		await expect
			.element(thumbnailButton(2))
			.toHaveAttribute("aria-pressed", "true");
		await userEvent.keyboard("{End}");
		await expect
			.element(thumbnailButton(4))
			.toHaveAttribute("aria-pressed", "true");
		await userEvent.keyboard("{Home}");
		await expect
			.element(thumbnailButton(1))
			.toHaveAttribute("aria-pressed", "true");
	});
});
