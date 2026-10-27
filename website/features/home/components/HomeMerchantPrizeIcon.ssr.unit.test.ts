import { PassThrough } from "node:stream";
import * as React from "react";
import { renderToPipeableStream } from "react-dom/server";
import { describe, expect, it } from "vitest";
import HomeMerchantPrizeIcon from "./HomeMerchantPrizeIcon";

function renderAllReady(element: React.ReactNode): Promise<string> {
	return new Promise((resolve, reject) => {
		let html = "";
		const output = new PassThrough();
		output.on("data", (chunk: Buffer) => {
			html += chunk.toString("utf8");
		});
		output.on("end", () => resolve(html));
		output.on("error", reject);

		const stream = renderToPipeableStream(element, {
			onAllReady() {
				stream.pipe(output);
			},
			onShellError: reject,
			onError: reject,
		});
	});
}

describe("home merchant prize icon server rendering", () => {
	it("streams the real trophy SVG after the lazy import resolves", async () => {
		const html = await renderAllReady(
			React.createElement(HomeMerchantPrizeIcon),
		);

		expect(html).toContain('data-icon-name="trophy"');
		expect(html).toContain('width="11"');
		expect(html).toContain('height="11"');
		expect(html).toContain('aria-hidden="true"');
		expect(html).toContain("currentColor");
	});
});
