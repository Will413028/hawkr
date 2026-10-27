#!/usr/bin/env node
// Small streamable-HTTP MCP client for environments that cannot hot-load tools.
const endpoint = process.env.STORYBOOK_MCP_URL || "http://127.0.0.1:6006/mcp";
let session;
let id = 0;
async function rpc(method, params) {
	const response = await fetch(endpoint, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
			Accept: "application/json, text/event-stream",
			...(session ? { "Mcp-Session-Id": session } : {}),
		},
		body: JSON.stringify({ jsonrpc: "2.0", id: ++id, method, params }),
		signal: AbortSignal.timeout(300000),
	});
	if (!response.ok)
		throw new Error(`MCP HTTP ${response.status}: ${await response.text()}`);
	session = response.headers.get("mcp-session-id") || session;
	const raw = await response.text();
	const messages = response.headers
		.get("content-type")
		?.includes("text/event-stream")
		? raw
				.split(/\r?\n/)
				.filter((line) => line.startsWith("data: "))
				.map((line) => JSON.parse(line.slice(6)))
		: [JSON.parse(raw)];
	const message = messages.find((entry) => entry.id === id);
	if (!message || message.error)
		throw new Error(JSON.stringify(message?.error || raw));
	return message.result;
}

await rpc("initialize", {
	protocolVersion: "2025-03-26",
	capabilities: {},
	clientInfo: { name: "hawkr-catalog-check", version: "1.0.0" },
});
const tool = process.argv[2] || "docs-list";
const result =
	tool === "tools/list"
		? await rpc("tools/list", {})
		: await rpc("tools/call", {
				name: tool,
				arguments: JSON.parse(process.argv[3] || "{}"),
			});
console.log(JSON.stringify(result, null, 2));
if (result.isError) process.exitCode = 1;
