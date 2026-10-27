import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { MaterialSymbol } from "../src/material-symbol";
import { MATERIAL_SYMBOL_PATHS } from "../src/material-symbols.gen";

describe("MaterialSymbol", () => {
	it("沒有 title 時是裝飾用，aria-hidden", () => {
		const html = renderToStaticMarkup(<MaterialSymbol name="search" />);
		expect(html).toContain('aria-hidden="true"');
		expect(html).not.toContain("role=");
		expect(html).toContain('viewBox="0 -960 960 960"');
		expect(html).toContain('fill="currentColor"');
		expect(html).toContain('stroke="none"');
	});

	it("沒傳 size 時跟著字級，width／height 是 1em", () => {
		const html = renderToStaticMarkup(<MaterialSymbol name="search" />);
		expect(html).toContain('width="1em"');
		expect(html).toContain('height="1em"');
	});

	it("傳 size={18} 時 width／height 是 18", () => {
		const html = renderToStaticMarkup(
			<MaterialSymbol name="search" size={18} />,
		);
		expect(html).toContain('width="18"');
		expect(html).toContain('height="18"');
	});

	it("有 title 時是 role=img 並帶 accessible name", () => {
		const html = renderToStaticMarkup(
			<MaterialSymbol name="search" title="搜尋" size={16} />,
		);
		expect(html).toContain('role="img"');
		expect(html).toContain('aria-label="搜尋"');
		expect(html).not.toContain("aria-hidden");
		expect(html).toContain('width="16"');
	});

	it("fill 選用 filled 的 path，其他 SVG props 透傳", () => {
		const outlined = renderToStaticMarkup(<MaterialSymbol name="star" />);
		const filled = renderToStaticMarkup(
			<MaterialSymbol name="star" fill className="x" data-k="v" />,
		);
		expect(outlined).toContain(MATERIAL_SYMBOL_PATHS.star.outlined);
		expect(filled).toContain(MATERIAL_SYMBOL_PATHS.star.filled);
		expect(filled).toContain('class="x"');
		expect(filled).toContain('data-k="v"');
	});
});
