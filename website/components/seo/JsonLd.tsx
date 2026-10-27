import type { Thing, WithContext } from "schema-dts";

/**
 * Server-render one or many schema.org graphs as a single
 * <script type="application/ld+json">. The `<` → < escape neutralizes a
 * "</script>" sequence in any string field (XSS hardening); JSON.stringify
 * already handles quotes/backslashes. Render inside a Server Component only.
 */
export default function JsonLd({
	data,
}: {
	data: WithContext<Thing> | WithContext<Thing>[];
}) {
	const json = JSON.stringify(data).replace(/</g, "\\u003c");
	return (
		<script
			type="application/ld+json"
			// biome-ignore lint/security/noDangerouslySetInnerHtml: serialized + <-escaped schema.org JSON
			dangerouslySetInnerHTML={{ __html: json }}
		/>
	);
}
