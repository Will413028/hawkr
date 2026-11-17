import { describe, expect, it } from "vitest";
import { feedScopeOf } from "@/lib/feed/scope";

describe("feedScopeOf", () => {
	it("a store and a hashtag together scope to the store only (the server rejects both)", () => {
		const scope = feedScopeOf(new URLSearchParams("sellerId=7&hashtag=Tea"));
		expect(scope).toMatchObject({ sellerId: "7", hashtag: "" });
	});

	it("the legacy `seller` param is the store too", () => {
		expect(
			feedScopeOf(new URLSearchParams("seller=7&hashtag=tea")),
		).toMatchObject({ sellerId: "7", hashtag: "" });
	});

	it("a hashtag alone is stripped of its # and kept", () => {
		expect(feedScopeOf(new URLSearchParams("hashtag=%23Tea"))).toMatchObject({
			sellerId: "",
			hashtag: "Tea",
		});
	});
});
