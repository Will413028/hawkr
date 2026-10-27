import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import ProductCertificationSection from "./components/ProductCertificationSection";

const ROOT = fileURLToPath(new URL("../..", import.meta.url));
const layoutSource = readFileSync(
	join(ROOT, "features/product/ProductDetailLayout.tsx"),
	"utf8",
);
const sectionSource = readFileSync(
	join(ROOT, "features/product/components/ProductCertificationSection.tsx"),
	"utf8",
);
const spotlightSource = readFileSync(
	join(ROOT, "features/product/components/useProductCertificationSpotlight.ts"),
	"utf8",
);
const spotlightCss = readFileSync(
	join(ROOT, "styles/components/product-certification.css"),
	"utf8",
);
const appLayoutSource = readFileSync(
	join(ROOT, "components/shared/RootDocument.tsx"),
	"utf8",
);
const reactBitsNotice = readFileSync(
	join(ROOT, "third_party/react-bits/NOTICE.md"),
	"utf8",
);

function render(certifications: Array<Record<string, unknown>>): string {
	return renderToStaticMarkup(
		React.createElement(ProductCertificationSection, { certifications }),
	);
}

describe("buyer product certification surface", () => {
	it("routes the PDP through the dedicated certification component", () => {
		expect(layoutSource).toContain(
			"<ProductCertificationSection certifications={certifications} />",
		);
		expect(layoutSource).not.toContain("certIconKey");
		expect(layoutSource).not.toContain('name.includes("有機")');
	});

	it("wires the spotlight as a scoped progressive enhancement", () => {
		expect(sectionSource).toContain("useProductCertificationSpotlight(");
		expect(sectionSource).toContain("product-certification-spotlight-list");
		expect(sectionSource).toContain('data-product-cert-spotlight=""');
		expect(appLayoutSource).toContain(
			'import "@/styles/components/product-certification.css";',
		);
		expect(reactBitsNotice).toContain(
			"Product certification reviewed revision",
		);
	});

	it("limits cursor tracking to motion-safe fine mouse pointers", () => {
		expect(spotlightSource).toContain("prefers-reduced-motion: reduce");
		expect(spotlightSource).toContain("(hover: hover) and (pointer: fine)");
		expect(spotlightSource).toContain("(forced-colors: active)");
		expect(spotlightSource).toContain('event.pointerType !== "mouse"');
		expect(spotlightSource).toContain("requestAnimationFrame");
		expect(spotlightSource).toContain("cancelAnimationFrame");
		expect(spotlightSource).toContain("visibilitychange");
	});

	it("keeps the effect decorative and accessible in static modes", () => {
		expect(spotlightCss).toContain("radial-gradient(");
		expect(spotlightCss).toContain("pointer-events: none");
		expect(spotlightCss).toContain(":focus-within");
		expect(spotlightCss).toContain("pointer: coarse");
		expect(spotlightCss).toContain("prefers-reduced-motion: reduce");
		expect(spotlightCss).toContain("forced-colors: active");
		expect(spotlightCss).toContain("@media print");
		expect(spotlightCss).not.toContain("transition: all");
		expect(spotlightCss).not.toContain("transform:");
	});

	it("renders the production TAP payload as a text mark with a labelled number", () => {
		const html = render([{ name: "3702", certNo: "0987654321" }]);

		expect(html).toContain("產銷履歷 TAP");
		expect(html).toContain(">產銷履歷<");
		expect(html).not.toContain("<img");
		expect(html).toContain("認證字號");
		expect(html).toContain("0987654321");
		expect(html).not.toContain(">3702<");
	});

	it("normalises imported codes and legacy snake_case numbers", () => {
		const html = render([{ name: " 3702 ", cert_no: " 0987654321 " }]);

		expect(html).toContain("產銷履歷 TAP");
		expect(html).toContain(">產銷履歷<");
		expect(html).toContain("0987654321");
	});

	it("shows the organic mark label and routes buyers to the authority lookup", () => {
		const html = render([{ name: "3701", certNo: "1-009-210002" }]);

		expect(html).toContain(">有機<");
		expect(html).toContain("https://epv.afa.gov.tw/Home/MarkUse");
		expect(html).toContain('target="_blank"');
	});

	it("does not manufacture a verification result the public contract lacks", () => {
		const html = render([{ name: "3702", certNo: "0987654321" }]);

		expect(html).toContain("賣家提供");
		expect(html).toContain(
			"標章與認證字號由賣家提供，實際狀態請以官方查詢結果為準",
		);
		expect(html).not.toContain("已驗證");
		expect(html).not.toContain("驗證機構");
		expect(html).not.toContain("有效期限");
	});

	it("keeps unknown legacy claims visible without inventing a logo or lookup", () => {
		const html = render([{ name: "9999", certNo: "LEGACY-01" }]);

		expect(html).toContain("9999");
		expect(html).toContain("LEGACY-01");
		expect(html).not.toContain("<img");
		expect(html).not.toContain("前往官方查詢");
	});

	it("renders no section when the product has no certifications", () => {
		expect(render([])).toBe("");
	});
});
