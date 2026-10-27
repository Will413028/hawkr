import assert from "node:assert/strict";
import test from "node:test";
import { isKnownNonEnforcingConsoleError } from "./e2e/helpers/runtime-console";

const googlePayReportOnly =
	"Framing 'https://pay.google.com/' violates the following report-only Content Security Policy directive: \"frame-ancestors 'self'\". The violation has been logged, but no further action has been taken.";

test("ignores only the known non-enforcing Google Pay frame diagnostic", () => {
	assert.equal(isKnownNonEnforcingConsoleError(googlePayReportOnly), true);
	assert.equal(
		isKnownNonEnforcingConsoleError(
			googlePayReportOnly.replace("report-only ", ""),
		),
		false,
	);
	assert.equal(
		isKnownNonEnforcingConsoleError(
			googlePayReportOnly.replace("frame-ancestors", "frame-src"),
		),
		false,
	);
	assert.equal(
		isKnownNonEnforcingConsoleError(
			googlePayReportOnly.replace("pay.google.com", "example.com"),
		),
		false,
	);
	assert.equal(
		isKnownNonEnforcingConsoleError("ordinary console error"),
		false,
	);
});
