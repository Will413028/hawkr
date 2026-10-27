import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { render } from "../tools/gen";

const GEN_PATH = fileURLToPath(
	new URL("../src/material-symbols.gen.ts", import.meta.url),
);

describe("material-symbols.gen.ts", () => {
	it("與重跑生成器的輸出逐字相同", () => {
		expect(readFileSync(GEN_PATH, "utf8")).toBe(render());
	});
});
