/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { authSignOut } = vi.hoisted(() => ({ authSignOut: vi.fn() }));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ signOut: authSignOut }),
}));

import {
	createSellerNavbarResource,
	SellerHeaderLoader,
	type SellerMenuChrome,
	SellerMenuLoader,
	type SellerMenuProps,
	type SellerNavbarModule,
} from "./SellerNavbarLoader";

const FIXTURE_CHROME: SellerMenuChrome = {
	root: "fixture-menu-root",
	trigger: "fixture-menu-trigger",
	avatar: "fixture-menu-avatar",
	name: "fixture-menu-name",
	chevron: "fixture-menu-chevron",
};

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((resolvePromise, rejectPromise) => {
		resolve = resolvePromise;
		reject = rejectPromise;
	});
	return { promise, resolve, reject };
}

// FIXTURE components make the role handoff observable without importing the
// real seller UI (and its auth/data dependencies) into this loader test.
function FixtureSellerHeader() {
	return React.createElement(
		"nav",
		{ "data-fixture-seller-role": "header" },
		"FIXTURE seller header",
	);
}

function FixtureSellerMenu({ chrome }: SellerMenuProps) {
	return React.createElement(
		"div",
		{
			className: chrome.root,
			"data-fixture-seller-role": "menu",
		},
		"FIXTURE seller menu",
	);
}

const FIXTURE_MODULE: SellerNavbarModule = {
	SellerHeader: FixtureSellerHeader,
	SellerMenu: FixtureSellerMenu,
};

function MenuLoaderFixture({
	resource,
	onSignOut = vi.fn(async () => false),
}: {
	resource: ReturnType<typeof createSellerNavbarResource>;
	onSignOut?: () => Promise<boolean>;
}) {
	return React.createElement(SellerMenuLoader, {
		resource,
		chrome: FIXTURE_CHROME,
		onSignOut,
	});
}

function RoleCancellationFixture({
	resource,
}: {
	resource: ReturnType<typeof createSellerNavbarResource>;
}) {
	const [role, setRole] = React.useState<"seller" | "buyer">("seller");
	return React.createElement(
		React.Fragment,
		null,
		React.createElement(
			"button",
			{ type: "button", onClick: () => setRole("buyer") },
			"FIXTURE switch to buyer",
		),
		role === "seller"
			? React.createElement(SellerHeaderLoader, { resource })
			: React.createElement(
					"div",
					{ "data-fixture-active-role": "buyer" },
					"FIXTURE buyer chrome",
				),
	);
}

afterEach(cleanup);

beforeEach(() => {
	authSignOut.mockReset();
});

describe("seller navbar async resource", () => {
	it("deduplicates one in-flight import and caches both required role exports", async () => {
		const request = deferred<Partial<SellerNavbarModule>>();
		const importer = vi.fn(() => request.promise);
		const resource = createSellerNavbarResource(importer);

		const first = resource.load();
		const second = resource.load();
		expect(first).toBe(second);
		expect(importer).toHaveBeenCalledTimes(1);
		expect(resource.peek()).toBeNull();

		request.resolve(FIXTURE_MODULE);
		expect(await first).toBe(FIXTURE_MODULE);
		expect(resource.peek()).toBe(FIXTURE_MODULE);
		expect(await resource.load()).toBe(FIXTURE_MODULE);
		expect(importer).toHaveBeenCalledTimes(1);
	});

	it("evicts rejected, synchronous, and incomplete imports so retry remains real", async () => {
		let attempt = 0;
		const importer = vi.fn((): Promise<Partial<SellerNavbarModule>> => {
			attempt += 1;
			if (attempt === 1) throw new Error("FIXTURE synchronous chunk failure");
			if (attempt === 2)
				return Promise.resolve({ SellerHeader: FixtureSellerHeader });
			if (attempt === 3) {
				return Promise.reject(new Error("FIXTURE rejected chunk"));
			}
			return Promise.resolve(FIXTURE_MODULE);
		});
		const resource = createSellerNavbarResource(importer);

		await expect(resource.load()).rejects.toThrow("synchronous chunk failure");
		await expect(resource.load()).rejects.toThrow("missing required exports");
		await expect(resource.load()).rejects.toThrow("rejected chunk");
		expect(resource.peek()).toBeNull();
		expect(await resource.load()).toBe(FIXTURE_MODULE);
		expect(importer).toHaveBeenCalledTimes(4);
	});
});

describe("seller navbar role loaders", () => {
	it("reserves the existing 72px narrow onboarding rail only for onboarding", () => {
		const request = deferred<Partial<SellerNavbarModule>>();
		const resource = createSellerNavbarResource(() => request.promise);
		const { rerender } = render(
			React.createElement(SellerHeaderLoader, { resource, onboarding: true }),
		);
		const rail = screen.getByRole("navigation");
		expect(rail.getAttribute("data-onboarding")).toBe("true");
		expect(rail.classList.contains("max-[640px]:h-[72px]")).toBe(true);
		expect(rail.style.minHeight).toBe("var(--floating-nav-height, 84px)");
		rerender(React.createElement(SellerHeaderLoader, { resource }));
		expect(rail.hasAttribute("data-onboarding")).toBe(false);
		expect(rail.classList.contains("max-[640px]:h-[72px]")).toBe(false);
	});

	it("shares one cold request while handing header and menu to their correct roles", async () => {
		const request = deferred<Partial<SellerNavbarModule>>();
		const importer = vi.fn(() => request.promise);
		const resource = createSellerNavbarResource(importer);

		render(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SellerHeaderLoader, { resource }),
				React.createElement(MenuLoaderFixture, { resource }),
			),
		);
		expect(importer).toHaveBeenCalledTimes(1);
		expect(
			document.querySelector('[data-seller-navbar-load-state="loading"]'),
		).toBeTruthy();
		expect(
			document.querySelector('[data-seller-menu-load-state="loading"]'),
		).toBeTruthy();

		await act(async () => {
			request.resolve(FIXTURE_MODULE);
			await request.promise;
		});
		expect(
			document.querySelector('[data-fixture-seller-role="header"]'),
		).toBeTruthy();
		expect(
			document.querySelector('[data-fixture-seller-role="menu"]'),
		).toBeTruthy();
	});

	it("cancels a seller-role handoff so late completion cannot reopen seller chrome", async () => {
		const request = deferred<Partial<SellerNavbarModule>>();
		const resource = createSellerNavbarResource(() => request.promise);
		render(React.createElement(RoleCancellationFixture, { resource }));

		fireEvent.click(
			screen.getByRole("button", { name: "FIXTURE switch to buyer" }),
		);
		expect(
			document.querySelector('[data-fixture-active-role="buyer"]'),
		).toBeTruthy();

		await act(async () => {
			request.resolve(FIXTURE_MODULE);
			await request.promise;
		});
		expect(
			document.querySelector('[data-fixture-seller-role="header"]'),
		).toBeNull();
		expect(
			document.querySelector('[data-fixture-active-role="buyer"]'),
		).toBeTruthy();
	});

	it("retries a rejected shared request without mixing header and menu roles", async () => {
		const recovered = deferred<Partial<SellerNavbarModule>>();
		let attempt = 0;
		const importer = vi.fn(() => {
			attempt += 1;
			return attempt === 1
				? Promise.reject(new Error("FIXTURE seller chunk unavailable"))
				: recovered.promise;
		});
		const resource = createSellerNavbarResource(importer);
		render(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SellerHeaderLoader, { resource }),
				React.createElement(MenuLoaderFixture, { resource }),
			),
		);

		await waitFor(() => {
			expect(
				document.querySelector('[data-seller-navbar-load-state="error"]'),
			).toBeTruthy();
			expect(
				document.querySelector('[data-seller-menu-load-state="error"]'),
			).toBeTruthy();
		});
		fireEvent.click(
			document.querySelector("[data-seller-navbar-retry]") as HTMLElement,
		);
		fireEvent.click(
			document.querySelector("[data-seller-menu-retry]") as HTMLElement,
		);
		expect(importer).toHaveBeenCalledTimes(2);

		await act(async () => {
			recovered.resolve(FIXTURE_MODULE);
			await recovered.promise;
		});
		expect(
			document.querySelector('[data-fixture-seller-role="header"]'),
		).toBeTruthy();
		expect(
			document.querySelector('[data-fixture-seller-role="menu"]'),
		).toBeTruthy();
	});

	it("keeps the pending escape rail stable when its chunk resolves during logout", async () => {
		const moduleRequest = deferred<Partial<SellerNavbarModule>>();
		const signOutRequest = deferred<boolean>();
		const onSignOut = vi.fn(() => signOutRequest.promise);
		const resource = createSellerNavbarResource(() => moduleRequest.promise);
		render(React.createElement(MenuLoaderFixture, { resource, onSignOut }));

		const logout = document.querySelector(
			"[data-seller-menu-logout]",
		) as HTMLButtonElement;
		fireEvent.click(logout);
		fireEvent.click(logout);
		expect(onSignOut).toHaveBeenCalledTimes(1);

		await act(async () => {
			moduleRequest.resolve(FIXTURE_MODULE);
			await moduleRequest.promise;
		});
		expect(
			document.querySelector('[data-seller-menu-load-state="loading"]'),
		).toBeTruthy();
		expect(
			document.querySelector('[data-fixture-seller-role="menu"]'),
		).toBeNull();

		await act(async () => {
			signOutRequest.resolve(false);
			await signOutRequest.promise;
		});
		await waitFor(() => {
			expect(
				document.querySelector('[data-fixture-seller-role="menu"]'),
			).toBeTruthy();
		});
	});

	it("keeps both escape rails retryable after their logout action fails", async () => {
		const neverLoads = deferred<Partial<SellerNavbarModule>>();
		const menuSignOut = vi
			.fn<() => Promise<boolean>>()
			.mockRejectedValue(new Error("FIXTURE menu logout failure"));
		authSignOut.mockRejectedValue(new Error("FIXTURE header logout failure"));
		const resource = createSellerNavbarResource(() => neverLoads.promise);
		render(
			React.createElement(
				React.Fragment,
				null,
				React.createElement(SellerHeaderLoader, { resource }),
				React.createElement(MenuLoaderFixture, {
					resource,
					onSignOut: menuSignOut,
				}),
			),
		);

		const headerLogout = document.querySelector(
			"[data-seller-navbar-logout]",
		) as HTMLButtonElement;
		const menuLogout = document.querySelector(
			"[data-seller-menu-logout]",
		) as HTMLButtonElement;
		fireEvent.click(headerLogout);
		fireEvent.click(menuLogout);
		await waitFor(() => {
			expect(headerLogout.disabled).toBe(false);
			expect(menuLogout.disabled).toBe(false);
		});
		expect(authSignOut).toHaveBeenCalledTimes(1);
		expect(menuSignOut).toHaveBeenCalledTimes(1);
		expect(
			document.querySelector('[data-seller-navbar-load-state="loading"]'),
		).toBeTruthy();
		expect(
			document.querySelector('[data-seller-menu-load-state="loading"]'),
		).toBeTruthy();

		fireEvent.click(headerLogout);
		fireEvent.click(menuLogout);
		await waitFor(() => {
			expect(authSignOut).toHaveBeenCalledTimes(2);
			expect(menuSignOut).toHaveBeenCalledTimes(2);
		});
	});
});
