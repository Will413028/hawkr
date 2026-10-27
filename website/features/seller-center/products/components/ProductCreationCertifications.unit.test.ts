// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/BrandLogoLoader", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/Link", () => ({
	default: (props: React.ComponentProps<"a">) =>
		React.createElement("a", props),
}));

import ProductCreationCertifications from "./ProductCreationCertifications";

type RecoveryForm = React.ComponentProps<
	typeof ProductCreationCertifications
>["form"];

function recoveryForm(overrides: Partial<RecoveryForm> = {}): RecoveryForm {
	return {
		createdProduct: { id: "866542657374814537" },
		createdCertificationCount: 1,
		certificationDrafts: [{ code: "3702", certNo: "  TAP-123 " }],
		certErrors: { "3702": "認證申請服務暫時無法使用" },
		errors: ["商品已建立，部分認證申請未完成，請重試。"],
		submitting: false,
		submit: vi.fn().mockResolvedValue(false),
		...overrides,
	};
}

afterEach(cleanup);

describe("ProductCreationCertifications recovery", () => {
	it("focuses the success heading, preserves the original certificate, and retries only the application action", () => {
		const form = recoveryForm();
		render(React.createElement(ProductCreationCertifications, { form }));

		const heading = screen.getByRole("heading", { name: "商品已建立" });
		expect(document.activeElement).toBe(heading);
		expect(screen.getByRole("status").textContent).toBe(
			"已確認 1 項申請結果，尚有 1 項待確認。",
		);
		expect(screen.getByRole("alert").textContent).toContain(
			"商品已建立，部分認證申請未完成，請重試。",
		);
		expect(screen.getByText(/TAP-123/)).not.toBeNull();
		expect(screen.getByText("認證申請服務暫時無法使用")).not.toBeNull();
		expect(screen.queryByRole("textbox")).toBeNull();
		expect(screen.queryByRole("checkbox")).toBeNull();
		expect(screen.getAllByRole("button")).toHaveLength(1);
		fireEvent.click(screen.getByRole("button", { name: "重試未完成的申請" }));
		expect(form.submit).toHaveBeenCalledExactlyOnceWith("save");
		expect(
			screen.getByRole("link", { name: "前往商品編輯" }).getAttribute("href"),
		).toBe("/seller-center/products/866542657374814537/edit");
	});

	it("disables retry and navigation while applications are submitting without exposing editable drafts", () => {
		const form = recoveryForm({ submitting: true, errors: [], certErrors: {} });
		render(React.createElement(ProductCreationCertifications, { form }));

		const retry = screen.getByRole<HTMLButtonElement>("button", {
			name: "送出申請中…",
		});
		expect(retry.disabled).toBe(true);
		expect(screen.queryByRole("textbox")).toBeNull();
		expect(screen.queryByRole("checkbox")).toBeNull();
		expect(retry.closest("form")?.getAttribute("aria-busy")).toBe("true");
		expect(
			screen
				.getByRole("link", { name: "前往商品編輯" })
				.getAttribute("aria-disabled"),
		).toBe("true");
		fireEvent.click(retry);
		expect(form.submit).not.toHaveBeenCalled();
	});

	it("offers only product management when the successful create response omitted its id", () => {
		const form = recoveryForm({
			createdProduct: {},
			createdCertificationCount: 0,
			errors: ["商品已建立，但回應未提供商品編號，請回商品管理確認。"],
			certErrors: {},
		});
		const { container } = render(
			React.createElement(ProductCreationCertifications, { form }),
		);

		expect(screen.queryByRole("button")).toBeNull();
		expect(screen.getAllByRole("link")).toHaveLength(1);
		expect(
			screen.getByRole("link", { name: "回商品管理" }).getAttribute("href"),
		).toBe("/seller-center/products");
		expect(screen.queryByRole("textbox")).toBeNull();
		expect(screen.queryByRole("checkbox")).toBeNull();
		expect(screen.getByRole("alert").textContent).toContain(
			"回應未提供商品編號",
		);
		const recoveryFormElement = container.querySelector("form");
		if (!recoveryFormElement) throw new Error("Missing recovery form");
		fireEvent.submit(recoveryFormElement);
		expect(form.submit).not.toHaveBeenCalled();
	});
});
