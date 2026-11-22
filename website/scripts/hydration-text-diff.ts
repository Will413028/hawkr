const IGNORED_BLOCKS =
	/<(script|style|template|noscript)\b[^>]*>[\s\S]*?<\/\1>|<!--[\s\S]*?-->/gi;
const HIDDEN_STAGING =
	/<div\b[^>]*\bhidden\b[^>]*\bid="S:[^"]*"[^>]*>[\s\S]*?<\/div>/gi;

export function extractTextTokens(html: string): string[] {
	const withoutBlocks = html
		.replace(IGNORED_BLOCKS, " ")
		.replace(HIDDEN_STAGING, " ");
	return withoutBlocks
		.split(/<[^>]*>/)
		.map((token) =>
			token
				.replace(/&amp;/g, "&")
				.replace(/&lt;/g, "<")
				.replace(/&gt;/g, ">")
				.replace(/&quot;/g, '"')
				.replace(/&#x27;|&#39;/g, "'")
				.replace(/\s+/g, " ")
				.trim(),
		)
		.filter((token) => token.length > 0);
}

export type TextTokenDifference = {
	index: number;
	ssr: string | undefined;
	client: string | undefined;
};

export function firstTextDifferences(
	ssrHtml: string,
	clientHtml: string,
	limit = 10,
): TextTokenDifference[] {
	const ssr = extractTextTokens(ssrHtml);
	const client = extractTextTokens(clientHtml);
	const differences: TextTokenDifference[] = [];
	const length = Math.max(ssr.length, client.length);
	for (let index = 0; index < length && differences.length < limit; index++) {
		if (ssr[index] !== client[index]) {
			differences.push({ index, ssr: ssr[index], client: client[index] });
		}
	}
	return differences;
}

export function formatTextDifferences(
	differences: readonly TextTokenDifference[],
): string {
	if (differences.length === 0) {
		return "No visible text token differs between the SSR and client HTML.";
	}
	return differences
		.map(
			(d) =>
				`#${d.index}\n  ssr:    ${JSON.stringify(d.ssr ?? null)}\n  client: ${JSON.stringify(d.client ?? null)}`,
		)
		.join("\n");
}
