import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

/**
 * 認證列自 19 號起收斂成共用元件（`components/shared/ProductCertificationRow.tsx`）；
 * 這支測試跟著改讀那一份，否則它會停在一個已經不存在的 `CertRow` 上恆綠。
 */
const certRowSourceText = readFileSync(
	new URL(
		"../../../components/shared/ProductCertificationRow.tsx",
		import.meta.url,
	),
	"utf8",
);
const productEditDrawerSource = readFileSync(
	new URL("./ProductEditDrawer.tsx", import.meta.url),
	"utf8",
);

function certRowSource(): string {
	const start = certRowSourceText.indexOf(
		"export function ProductCertificationRow(",
	);
	expect(start).toBeGreaterThanOrEqual(0);
	return certRowSourceText.slice(start);
}

describe("seller certification focus geometry", () => {
	it("keeps the sr-only checkbox positioned inside its visible toggle label", () => {
		const toggle = certRowSource().match(
			/<label className="([^"]+)">\s*<input([\s\S]*?)\/>/,
		);

		expect(toggle).not.toBeNull();
		expect(toggle?.[2]).toContain('type="checkbox"');
		expect(toggle?.[2]).toMatch(/className="[^"]*\bsr-only\b[^"]*"/);
		expect(toggle?.[1].split(/\s+/)).toContain("relative");
	});

	it("keeps the fixed Drawer shell out of the focus scroll chain", () => {
		const drawerPanelClass = productEditDrawerSource.match(
			/className=\{`([\s\S]*?transition-\[max-width\][\s\S]*?)`\}/,
		)?.[1];

		expect(drawerPanelClass?.split(/\s+/)).toContain("overflow-clip");
	});
});
