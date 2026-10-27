import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { buildOutputs } from "./generate-legal";
import { parseLegalMarkdown } from "./legal-markdown";

const ROOT = resolve(import.meta.dirname, "../..");

const HEADER = "# 標題\n\n版本：2031-01-02（說明）\n";

describe("parseLegalMarkdown", () => {
	it("標題與版本行抽出，版本取生效日", () => {
		const parsed = parseLegalMarkdown(`${HEADER}\n內文。\n`);
		expect(parsed.title).toBe("標題");
		expect(parsed.version).toBe("2031-01-02");
		expect(parsed.versionLine).toBe("版本：2031-01-02（說明）");
		expect(parsed.blocks).toEqual([{ type: "p", lines: ["內文。"] }]);
	});

	it("版本行與同段的其他行：只抽出版本行", () => {
		const parsed = parseLegalMarkdown(
			"# 標題\n\n版本：2031-01-02（說明）\n生效日期：同意之日\n",
		);
		expect(parsed.blocks).toEqual([
			{ type: "p", lines: ["生效日期：同意之日"] },
		]);
	});

	it("每種區塊：h2、h3、段落換行、編號含子項目、項目、核取、表格", () => {
		const parsed = parseLegalMarkdown(
			`${HEADER}
## 第一條

甲行  
乙行

### 小節

1. **粗體**項目
2. 有子項目：
   - 子一
   - 子二

- 項目甲
- 項目乙

- [ ] 核取甲
- [ ] 核取乙

| 欄一 | 欄二 |
| --- | --- |
| a | b |
| c | d |
`,
		);
		expect(parsed.blocks).toEqual([
			{ type: "h2", text: "第一條" },
			{ type: "p", lines: ["甲行", "乙行"] },
			{ type: "h3", text: "小節" },
			{
				type: "ol",
				items: ["**粗體**項目", { text: "有子項目：", sub: ["子一", "子二"] }],
			},
			{ type: "ul", items: ["項目甲", "項目乙"] },
			{ type: "checklist", items: ["核取甲", "核取乙"] },
			{
				type: "table",
				header: ["欄一", "欄二"],
				rows: [
					["a", "b"],
					["c", "d"],
				],
			},
		]);
	});

	it("分隔線與 HTML 註解不進區塊，也切開前後的段落", () => {
		const parsed = parseLegalMarkdown(
			`${HEADER}\n前段\n<!-- pagebreak -->\n後段\n\n---\n\n末段\n`,
		);
		expect(parsed.blocks).toEqual([
			{ type: "p", lines: ["前段"] },
			{ type: "p", lines: ["後段"] },
			{ type: "p", lines: ["末段"] },
		]);
	});

	it("編號不連續、沒有版本行、版本行格式不符、重複標題都 throw", () => {
		expect(() => parseLegalMarkdown(`${HEADER}\n1. 甲\n3. 乙\n`)).toThrow(
			/編號不連續/,
		);
		expect(() => parseLegalMarkdown("# 標題\n\n內文\n")).toThrow(/版本行/);
		expect(() =>
			parseLegalMarkdown("# 標題\n\n版本：〔生效日〕（草稿）\n"),
		).toThrow(/格式不符/);
		expect(() => parseLegalMarkdown(`${HEADER}\n# 另一個\n`)).toThrow(/重複/);
	});
});

describe("generate-legal 的輸出", () => {
	const outputs = buildOutputs(ROOT);

	it("已提交的生成物與重新產生的結果相同", () => {
		expect([...outputs.keys()].sort()).toEqual([
			"backend/internal/modules/seller/internal/model/agreement_gen.go",
			"website/features/legal/documents/sellerAgreements.gen.ts",
			"website/features/legal/documents/sellerKycNotice.gen.ts",
			"website/features/legal/documents/sellerTerms.gen.ts",
		]);
		for (const [rel, content] of outputs) {
			expect(readFileSync(resolve(ROOT, rel), "utf8"), rel).toBe(content);
		}
	});

	it("版本取自正本的版本行；沒有正本的 PAYUNI_TERMS 取 manifest 的版本", () => {
		const manifest = JSON.parse(
			readFileSync(
				resolve(ROOT, "website/docs/legal/seller-agreements.json"),
				"utf8",
			),
		) as { agreements: { kind: string; version?: string }[] };
		const payuni = manifest.agreements.find((a) => a.kind === "PAYUNI_TERMS");
		const go = outputs.get(
			"backend/internal/modules/seller/internal/model/agreement_gen.go",
		) as string;
		for (const doc of ["seller-terms.md", "seller-kyc-notice.md"]) {
			const { version } = parseLegalMarkdown(
				readFileSync(resolve(ROOT, "website/docs/legal", doc), "utf8"),
			);
			expect(go).toContain(`Version: "${version}"`);
		}
		expect(go).toContain(`Version: "${payuni?.version}"`);
	});

	it("manifest 的網址原樣進到兩邊的生成物", () => {
		const ts = outputs.get(
			"website/features/legal/documents/sellerAgreements.gen.ts",
		) as string;
		const go = outputs.get(
			"backend/internal/modules/seller/internal/model/agreement_gen.go",
		) as string;
		for (const url of ["/seller-terms", "/privacy#seller-kyc-notice"]) {
			expect(ts).toContain(`"${url}"`);
			expect(go).toContain(`"${url}"`);
		}
	});

	it("正文裡的佔位符保留到投影", () => {
		const parsed = parseLegalMarkdown(
			`${HEADER}\n〔營運者名稱〕的聯絡信箱是 〔聯絡信箱〕，網址 〔平台網址〕。\n`,
		);
		expect(parsed.blocks).toEqual([
			{
				type: "p",
				lines: ["〔營運者名稱〕的聯絡信箱是 〔聯絡信箱〕，網址 〔平台網址〕。"],
			},
		]);
	});
});
