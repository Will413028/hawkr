import { describe, expect, it } from "vitest";
import {
	extractTextTokens,
	firstTextDifferences,
	formatTextDifferences,
} from "./hydration-text-diff";

const ssr = `<html><head><style>.a{}</style></head><body>
<script>self.__next_f.push([1,"x"])</script>
<!--$?--><template id="B:0"></template><!--/$-->
<div hidden id="S:0"><p>staged</p></div>
<main><h1>Price</h1><p>Total &amp; tax</p><span>10:00</span></main></body></html>`;

describe("hydration text diff", () => {
	it("ignores scripts, styles, templates, comments and staging copies", () => {
		expect(extractTextTokens(ssr)).toEqual(["Price", "Total & tax", "10:00"]);
	});

	it("reports a changed text node", () => {
		const client = ssr.replace("10:00", "10:01");
		expect(firstTextDifferences(ssr, client)).toEqual([
			{ index: 2, ssr: "10:00", client: "10:01" },
		]);
	});

	it("reports nothing when only ignored content differs", () => {
		const client = ssr.replace("self.__next_f.push", "other.push");
		expect(firstTextDifferences(ssr, client)).toEqual([]);
		expect(formatTextDifferences([])).toContain("No visible text token");
	});

	it("reports missing tokens", () => {
		const client = ssr.replace("<span>10:00</span>", "");
		expect(firstTextDifferences(ssr, client)).toEqual([
			{ index: 2, ssr: "10:00", client: undefined },
		]);
	});
});
