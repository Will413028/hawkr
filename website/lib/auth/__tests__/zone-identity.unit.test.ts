// @vitest-environment jsdom

import * as React from "react";
import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	auth: {
		status: "unavailable" as string,
		identity: null as null | { accountId: string; isSeller: boolean },
		activeRole: null as null | "buyer" | "seller",
	},
}));
vi.mock("../context", () => ({ useAuth: () => mocks.auth }));

import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { useAccountStatus } from "../use-account-status";
import { ZoneIdentityProvider } from "../zone-context";
import { useZoneIdentity } from "../zone-identity";

function Probe() {
	const identity = useZoneIdentity();
	const key = useQueryIdentity();
	const status = useAccountStatus();
	return React.createElement(
		"span",
		null,
		`${identity.accountId}:${identity.isSeller}|${key.accountId}:${key.activeRole}|${status}`,
	);
}

function inZone(
	workspace: "buyer" | "seller" | null,
	node: React.ReactNode,
	accountId = "7",
) {
	return React.createElement(ZoneIdentityProvider, {
		identity: { accountId, isSeller: true },
		workspace,
		children: node,
	});
}

describe("zone identity", () => {
	it("soft navigation into a zone with a stale root `unavailable` renders content on the first render", () => {
		mocks.auth = { status: "unavailable", identity: null, activeRole: null };
		const html = renderToString(inZone("seller", React.createElement(Probe)));
		expect(html).toContain("7:true|7:seller|signedIn");
	});

	it("the zone workspace wins over the lagging client active role in query keys", () => {
		mocks.auth = {
			status: "signedIn",
			identity: { accountId: "9", isSeller: true },
			activeRole: "buyer",
		};
		const html = renderToString(inZone("seller", React.createElement(Probe)));
		expect(html).toContain("|7:seller|");
	});

	it("a zone without a workspace uses the client active role", () => {
		mocks.auth = {
			status: "signedIn",
			identity: { accountId: "7", isSeller: true },
			activeRole: "buyer",
		};
		const html = renderToString(inZone(null, React.createElement(Probe)));
		expect(html).toContain("|7:buyer|");
	});

	it("outside a zone the root status and identity are used", () => {
		mocks.auth = { status: "unavailable", identity: null, activeRole: null };
		function Outside() {
			const key = useQueryIdentity();
			return React.createElement(
				"span",
				null,
				`${useAccountStatus()}|${key.accountId}`,
			);
		}
		expect(renderToString(React.createElement(Outside))).toContain(
			"unavailable|null",
		);
	});

	it("useZoneIdentity throws outside a zone", () => {
		const error = vi.spyOn(console, "error").mockImplementation(() => {});
		function Bare() {
			useZoneIdentity();
			return null;
		}
		expect(() => renderToString(React.createElement(Bare))).toThrow(
			"useZoneIdentity",
		);
		error.mockRestore();
	});
});
