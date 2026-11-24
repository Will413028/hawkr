import { describe, expect, it } from "vitest";
import { apiProxyRewrites, filesProxyTarget } from "@/lib/api/local-proxy";

const API_BASE_URL = "https://api.example.org/edge/api/v1";

describe("files proxy", () => {
	it("targets the monolith root /api/files, not /api/v1/files", () => {
		expect(
			filesProxyTarget(
				"http://localhost:3000/api/files/secure_uploads/x/chunks/0",
				API_BASE_URL,
			),
		).toBe("https://api.example.org/api/files/secure_uploads/x/chunks/0");
	});

	it("keeps the query of a preview request", () => {
		expect(
			filesProxyTarget(
				"http://localhost:3000/api/files/secure_uploads/x?v=1",
				API_BASE_URL,
			),
		).toBe("https://api.example.org/api/files/secure_uploads/x?v=1");
	});

	it("is rewritten to the same monolith path in deployed builds", () => {
		const rewrite = apiProxyRewrites({
			NODE_ENV: "production",
			API_BASE_URL,
		}).find((r) => r.source === "/api/files/:path*");
		expect(rewrite?.destination).toBe(
			"https://api.example.org/api/files/:path*",
		);
	});
});
