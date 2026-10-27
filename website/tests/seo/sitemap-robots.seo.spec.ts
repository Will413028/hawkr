import { expect, test } from "@playwright/test";
import { INDEXABLE_STATIC_ROUTES, isIndexable } from "@/lib/seo/indexability";

test.describe("sitemap purity", () => {
	test("every sitemap <loc> path is indexable and returns 200", async ({
		request,
	}) => {
		const res = await request.get("/sitemap.xml");
		expect(res.ok()).toBeTruthy();
		const xml = await res.text();
		const locs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]);
		expect(locs.length).toBeGreaterThan(0);

		for (const loc of locs) {
			const path = new URL(loc).pathname;
			// Registry purity: the sitemap must never list a noindex path.
			expect(isIndexable(path), `${path} must be indexable`).toBeTruthy();
			// Live purity: every listed URL must really resolve 200 (no 404/3xx).
			const page = await request.get(path, { maxRedirects: 0 });
			expect(page.status(), `${path} should be 200`).toBe(200);
		}
	});

	test("sitemap contains exactly the indexable static routes", async ({
		request,
	}) => {
		const xml = await (await request.get("/sitemap.xml")).text();
		const paths = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map(
			(m) => new URL(m[1]).pathname,
		);
		const expected = INDEXABLE_STATIC_ROUTES.filter((route) =>
			isIndexable(route),
		);
		expect(new Set(paths)).toEqual(new Set(expected));
	});
});

test.describe("robots policy (gate open)", () => {
	test("robots.txt allows crawling, blocks only /api, points at the sitemap", async ({
		request,
	}) => {
		const body = await (await request.get("/robots.txt")).text();
		// Gate OPEN → must NOT contain the closed-gate full-site block.
		expect(body).not.toMatch(/^Disallow:\s*\/\s*$/m);
		expect(body).toMatch(/^Allow:\s*\//m);
		expect(body).toMatch(/^Disallow:\s*\/api\//m);
		expect(body.toLowerCase()).toContain("sitemap:");
	});

	test("no money URL is disallowed by robots", async ({ request }) => {
		const body = await (await request.get("/robots.txt")).text();
		const disallows = [...body.matchAll(/^Disallow:\s*(\S+)\s*$/gm)].map(
			(m) => m[1],
		);
		for (const route of INDEXABLE_STATIC_ROUTES) {
			for (const rule of disallows) {
				const blocked = rule === "/" || route.startsWith(rule);
				expect(blocked, `${route} blocked by Disallow ${rule}`).toBeFalsy();
			}
		}
	});
});
