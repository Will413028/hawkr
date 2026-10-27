// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import type { AppLocale } from "@/lib/intl/locales";
import { RoutedLocaleProvider } from "@/lib/intl/routed-locale";
import { AgreementCheckbox } from "./AgreementCheckbox";

function linkHref(locale: AppLocale, href: string) {
	render(
		React.createElement(
			RoutedLocaleProvider,
			{ value: locale },
			React.createElement(AgreementCheckbox, {
				checked: false,
				onChange: () => {},
				before: "b",
				linkText: "terms",
				href,
			}),
		),
	);
	return screen.getByRole("link", { name: "terms" }).getAttribute("href");
}

describe("AgreementCheckbox localizes its document link", () => {
	afterEach(cleanup);

	it("zh-TW keeps the path byte for byte", () => {
		expect(linkHref("zh-TW", "/privacy#seller-kyc-notice")).toBe(
			"/privacy#seller-kyc-notice",
		);
	});

	it("an English page opens the English document, hash kept", () => {
		expect(linkHref("en", "/privacy#seller-kyc-notice")).toBe(
			"/en/privacy#seller-kyc-notice",
		);
	});
});
