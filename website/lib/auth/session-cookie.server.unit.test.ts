import { afterEach, describe, expect, it, vi } from "vitest";

const store = vi.hoisted(() => ({
	cookies: vi.fn(),
}));
vi.mock("next/headers", () => ({ cookies: store.cookies }));

import { SESSION_COOKIE_NAME } from "./session-cookie";
import { sessionCookieHeader } from "./session-cookie.server";

afterEach(() => store.cookies.mockReset());

function jar(values: Record<string, string>) {
	return {
		get: (name: string) =>
			name in values ? { name, value: values[name] } : undefined,
	};
}

describe("server-side session cookie (D6: handed to reads only)", () => {
	it("returns exactly the session cookie, not the visitor's other cookies", async () => {
		store.cookies.mockResolvedValue(
			jar({ [SESSION_COOKIE_NAME]: "opaque", analytics: "x" }),
		);
		expect(await sessionCookieHeader()).toBe(`${SESSION_COOKIE_NAME}=opaque`);
	});

	it("a visitor without the cookie gets null", async () => {
		store.cookies.mockResolvedValue(jar({ analytics: "x" }));
		expect(await sessionCookieHeader()).toBeNull();
	});

	it("outside a request scope there is no browser cookie: no header, no throw", async () => {
		store.cookies.mockRejectedValue(
			new Error("`cookies` was called outside a request scope"),
		);
		expect(await sessionCookieHeader()).toBeNull();
	});

	it("Next's own dynamic-rendering signal (an error with a digest) is not swallowed", async () => {
		const signal = Object.assign(new Error("Dynamic server usage"), {
			digest: "DYNAMIC_SERVER_USAGE",
		});
		store.cookies.mockRejectedValue(signal);
		await expect(sessionCookieHeader()).rejects.toBe(signal);
	});
});
