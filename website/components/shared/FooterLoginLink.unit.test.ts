// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ status: "unknown" as string }));
vi.mock("@/lib/auth/context", () => ({ useAuth: () => auth }));
vi.mock("@/components/shared/Link", () => ({
	default: (props: { children?: React.ReactNode }) =>
		React.createElement("a", null, props.children),
}));

import FooterLoginLink from "./FooterLoginLink";

afterEach(cleanup);

describe("FooterLoginLink", () => {
	it("shows nothing while the server's answer is unknown, not the guest link", () => {
		auth.status = "unknown";
		render(React.createElement(FooterLoginLink));
		expect(screen.queryByText("登入")).toBeNull();
	});

	it.each(["guest", "unavailable"])("%s shows the login link", (status) => {
		auth.status = status;
		render(React.createElement(FooterLoginLink));
		expect(screen.getByText("登入")).toBeTruthy();
	});

	it("signedIn shows nothing", () => {
		auth.status = "signedIn";
		render(React.createElement(FooterLoginLink));
		expect(screen.queryByText("登入")).toBeNull();
	});
});
