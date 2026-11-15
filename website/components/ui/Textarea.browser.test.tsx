import { describe, expect, it } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Textarea } from "./Input";

describe("Textarea", () => {
	it("accepts typed text", async () => {
		renderWithIntl(
			<Textarea
				label="商品說明"
				placeholder="分享產地、風味與保存方式"
				minRows={3}
			/>,
			{ frameMaxWidth: 420 },
		);
		await userEvent.type(page.getByRole("textbox"), "來自花蓮的當季收成");
		await expect
			.element(page.getByRole("textbox"))
			.toHaveValue("來自花蓮的當季收成");
	});
});
