import assert from "node:assert/strict";
import { test } from "node:test";
import { NextRequest } from "next/server";
import { GET, POST } from "../app/(commerce)/store-map/return/route";

interface CallbackPayload {
	type: string;
	storeId: string;
	storeName: string;
	storeAddress: string;
	error?: string;
}

async function withSiteURL<T>(
	value: string | undefined,
	run: () => Promise<T>,
): Promise<T> {
	const previous = process.env.NEXT_PUBLIC_SITE_URL;
	if (value === undefined) delete process.env.NEXT_PUBLIC_SITE_URL;
	else process.env.NEXT_PUBLIC_SITE_URL = value;

	try {
		return await run();
	} finally {
		if (previous === undefined) delete process.env.NEXT_PUBLIC_SITE_URL;
		else process.env.NEXT_PUBLIC_SITE_URL = previous;
	}
}

async function readCallbackResponse(
	response: Response,
	expectedOrigin: string,
): Promise<{ html: string; payload: CallbackPayload }> {
	assert.equal(response.status, 200);
	assert.match(
		response.headers.get("content-type") ?? "",
		/^text\/html;\s*charset=utf-8$/i,
	);
	assert.equal(response.headers.get("cache-control"), "no-store");

	const html = await response.text();
	assert.match(html, /target\.postMessage\(payload, origins\[i\]\);/);
	assert.ok(
		html.includes(`addOrigin("${expectedOrigin}");`),
		`callback omitted target origin ${expectedOrigin}`,
	);

	const payloadLiteral = html.match(
		/var payload = JSON\.parse\(("(?:\\.|[^"\\])*")\);/,
	)?.[1];
	assert.ok(payloadLiteral, "callback HTML omitted its JSON payload");
	const serializedPayload = JSON.parse(payloadLiteral) as string;
	const payload = JSON.parse(serializedPayload) as CallbackPayload;
	return { html, payload };
}

function assertScriptInputIsEscaped(html: string) {
	assert.equal(
		html.match(/<\/script>/gi)?.length,
		1,
		"user input must not create an additional script element",
	);
	assert.doesNotMatch(html, /<\/script><script>/i);
	assert.match(html, /<\\\/script>/i);
	assert.ok(
		html.includes("\\n"),
		"line feeds must stay escaped in script data",
	);
	assert.ok(
		html.includes("\\u2028"),
		"U+2028 must stay escaped in script data",
	);
}

test("GET returns an HTML postMessage callback with the configured site origin and escaped input", {
	concurrency: false,
}, async () => {
	await withSiteURL("https://example.com/deploy-path", async () => {
		const storeName =
			'北門"</script><script>globalThis.pwned=true</script>\n門市\u2028';
		const url = new URL("https://internal.invalid/store-map/return");
		url.searchParams.set("storeCode", "711-GET");
		url.searchParams.set("storeName", storeName);
		url.searchParams.set("storeAddress", "台北市中正區忠孝西路一段");
		url.searchParams.set("errorMessage", "callback warning");

		const { html, payload } = await readCallbackResponse(
			GET(
				new NextRequest(url, {
					headers: {
						"x-forwarded-host": "attacker.invalid",
						"x-forwarded-proto": "http",
					},
				}),
			),
			"https://example.com",
		);

		assert.deepEqual(payload, {
			type: "hk-store-map-result",
			storeId: "711-GET",
			storeName,
			storeAddress: "台北市中正區忠孝西路一段",
			error: "callback warning",
		});
		assertScriptInputIsEscaped(html);
	});
});

test("POST accepts PayUni form aliases and targets the forwarded public origin safely", {
	concurrency: false,
}, async () => {
	await withSiteURL(undefined, async () => {
		const storeAddress =
			'台南市中西區"</script><script>globalThis.pwned=true</script>\n路\u2028';
		const body = new URLSearchParams({
			CVSStoreID: "711-POST",
			CVSStoreName: "範例門市",
			CVSAddress: storeAddress,
		});
		const response = await POST(
			new NextRequest("https://internal.invalid/store-map/return", {
				method: "POST",
				headers: {
					"content-type": "application/x-www-form-urlencoded",
					"x-forwarded-host": "web-dev.example.com",
					"x-forwarded-proto": "https",
				},
				body: body.toString(),
			}),
		);
		const { html, payload } = await readCallbackResponse(
			response,
			"https://web-dev.example.com",
		);

		assert.deepEqual(payload, {
			type: "hk-store-map-result",
			storeId: "711-POST",
			storeName: "範例門市",
			storeAddress,
		});
		assertScriptInputIsEscaped(html);
	});
});
