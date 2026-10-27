// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	ProductCertificationStatus,
} from "@/lib/codemeta/enums";
import { PRODUCT_CERT_CATALOG } from "@/lib/products/certifications";
import type { ProductCertificationEntry } from "@/lib/products/seller-repo";

vi.mock("@/components/shared/BrandLogoLoader", () => ({
	default: () => null,
}));

import { ProductCertificationRow } from "./ProductCertificationRow";

const item = PRODUCT_CERT_CATALOG[0];

afterEach(cleanup);

describe("ProductCertificationRow draft input", () => {
	it("lets the seller select, type an exact draft value, and deselect without submitting", () => {
		const onDraftChange = vi.fn();
		const onSubmit = vi.fn();
		const onToggle = vi.fn();
		function DraftRow() {
			const [entry, setEntry] = React.useState<
				ProductCertificationEntry | undefined
			>();
			return React.createElement(ProductCertificationRow, {
				item,
				mode: "draft",
				entry,
				idPrefix: "create",
				onToggle,
				onSubmit,
				onDraftChange: (certNo) => {
					onDraftChange(certNo);
					setEntry(certNo === null ? undefined : { code: item.code, certNo });
				},
			});
		}
		render(React.createElement(DraftRow));

		const checkbox = screen.getByRole<HTMLInputElement>("checkbox", {
			name: new RegExp(item.label),
		});
		expect(checkbox.disabled).toBe(false);
		fireEvent.click(checkbox);
		expect(checkbox.checked).toBe(true);
		expect(onDraftChange).toHaveBeenLastCalledWith("");

		const input = screen.getByRole<HTMLInputElement>("textbox", {
			name: "證書編號",
		});
		expect(input.readOnly).toBe(false);
		fireEvent.change(input, { target: { value: "  A-123 " } });
		expect(input.value).toBe("  A-123 ");
		expect(onDraftChange).toHaveBeenLastCalledWith("  A-123 ");
		expect(screen.queryByRole("button", { name: "申請" })).toBeNull();
		expect(
			screen.getByText(
				"商品建立成功後會自動送出認證申請；審核通過才會顯示標章。",
			),
		).not.toBeNull();
		for (const status of [
			ProductCertificationStatus.APPROVED,
			ProductCertificationStatus.SUBMITTED,
		]) {
			expect(
				screen.queryByText(PRODUCT_CERTIFICATION_STATUS_LABEL[status]),
			).toBeNull();
		}

		fireEvent.click(checkbox);
		expect(onDraftChange).toHaveBeenLastCalledWith(null);
		expect(checkbox.checked).toBe(false);
		expect(screen.queryByRole("textbox")).toBeNull();
		expect(onSubmit).not.toHaveBeenCalled();
		expect(onToggle).not.toHaveBeenCalled();
	});

	it("retains remove-only read-only behavior for draft callers without a change callback", () => {
		const onToggle = vi.fn();
		const props = {
			item,
			mode: "draft" as const,
			idPrefix: "import",
			onToggle,
		};
		const { rerender } = render(
			React.createElement(ProductCertificationRow, props),
		);
		expect(screen.getByRole<HTMLInputElement>("checkbox").disabled).toBe(true);
		fireEvent.click(screen.getByRole("checkbox"));
		expect(screen.queryByRole("textbox")).toBeNull();
		expect(onToggle).not.toHaveBeenCalled();

		rerender(
			React.createElement(ProductCertificationRow, {
				...props,
				entry: { code: item.code, certNo: "CSV-123" },
			}),
		);
		const checkbox = screen.getByRole<HTMLInputElement>("checkbox");
		expect(checkbox.disabled).toBe(false);
		expect(checkbox.checked).toBe(true);
		expect(screen.getByRole<HTMLInputElement>("textbox").readOnly).toBe(true);
		expect(screen.queryByRole("button", { name: "申請" })).toBeNull();
		fireEvent.click(checkbox);
		expect(onToggle).toHaveBeenCalledOnce();
	});

	it("blocks both draft selection and typing while busy and associates field errors", () => {
		const onDraftChange = vi.fn();
		const onToggle = vi.fn();
		render(
			React.createElement(ProductCertificationRow, {
				item,
				mode: "draft",
				entry: { code: item.code, certNo: "A-123" },
				idPrefix: "create",
				busy: true,
				error: "請填寫證書編號",
				onDraftChange,
				onToggle,
			}),
		);
		const checkbox = screen.getByRole<HTMLInputElement>("checkbox");
		const input = screen.getByRole<HTMLInputElement>("textbox");
		expect(checkbox.disabled).toBe(true);
		expect(input.disabled).toBe(true);
		fireEvent.click(checkbox);
		fireEvent.change(input, { target: { value: "blocked change" } });
		expect(onDraftChange).not.toHaveBeenCalled();
		expect(onToggle).not.toHaveBeenCalled();
		expect(input.getAttribute("aria-invalid")).toBe("true");
		const error = screen.getByText("請填寫證書編號");
		expect(input.getAttribute("aria-describedby")?.split(" ")).toContain(
			error.id,
		);
	});

	it("keeps review submission explicit after the seller selects and enters a number", () => {
		const onSubmit = vi.fn();
		const onDraftChange = vi.fn();
		render(
			React.createElement(ProductCertificationRow, {
				item,
				mode: "review",
				idPrefix: "edit",
				onToggle: vi.fn(),
				onSubmit,
				onDraftChange,
			}),
		);
		fireEvent.click(screen.getByRole("checkbox"));
		fireEvent.change(screen.getByRole("textbox"), {
			target: { value: "  A-123 " },
		});
		expect(onSubmit).not.toHaveBeenCalled();
		expect(onDraftChange).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole("button", { name: "申請" }));
		expect(onSubmit).toHaveBeenCalledExactlyOnceWith("A-123");
	});
});
