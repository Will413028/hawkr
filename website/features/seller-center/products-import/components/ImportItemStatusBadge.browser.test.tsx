import { describe, expect, it } from "vitest";
import { page } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { IMPORT_ITEM_STATUS } from "../types";
import { ImportItemStatusBadge } from "./ImportItemStatusBadge";

describe("ImportItemStatusBadge", () => {
	it("shows 需修正 instead of 可上架 when a ready item needs a fix", async () => {
		renderWithIntl(
			<ImportItemStatusBadge status={IMPORT_ITEM_STATUS.READY} needsFix />,
		);
		await expect
			.element(page.getByText("需修正", { exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByText("可上架", { exact: true }))
			.not.toBeInTheDocument();
	});
});
