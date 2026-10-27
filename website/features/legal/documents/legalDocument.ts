/**
 * 法務文件投影的資料形狀。正本是 `docs/legal/` 的 Markdown，投影（`*.gen.ts`）由
 * `scripts/generate-legal.ts` 產生；形狀只涵蓋正本用到的 Markdown 語法。
 * 字串保留正本的行內 `**粗體**` 標記，渲染時才轉成 <strong>。
 */
export type LegalListItem =
	| string
	| { readonly text: string; readonly sub: readonly string[] };

export type LegalBlock =
	| { readonly type: "h2" | "h3"; readonly text: string }
	| { readonly type: "p"; readonly lines: readonly string[] }
	| { readonly type: "ol"; readonly items: readonly LegalListItem[] }
	| { readonly type: "ul" | "checklist"; readonly items: readonly string[] }
	| {
			readonly type: "table";
			readonly header: readonly string[];
			readonly rows: readonly (readonly string[])[];
	  };

export interface LegalDocument {
	readonly title: string;
	/** 版本，值是生效日（YYYY-MM-DD）；伺服器公布的版本必須與它相同。 */
	readonly version: string;
	/** 正本的版本行，從 blocks 抽出。 */
	readonly versionLine: string;
	readonly blocks: readonly LegalBlock[];
}
