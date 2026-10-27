import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(testDir);

const clientConfig = () =>
	readFileSync(join(projectRoot, "instrumentation-client.ts"), "utf8");

/**
 * The org's Replay allowance is a hard cap, not billing elasticity: the Team
 * plan reserves **50 replays per billing period** (measured 2026-07-29 via
 * `/customers/<org>/`: reserved=50, used=52).
 *
 * Session sampling at any rate exhausts that. It already happened: dev ran at
 * `1.0`, recorded 52 replays in a single day (07-28), and from 07-29 the whole
 * org — prod included — was rate limited (234 rejected). The limit's scope is
 * `organization` and both projects share one org, so a dev-side sampling rate
 * decides whether prod has any post-incident reconstruction at all.
 *
 * These tests exist so that never silently regresses. Raising the rate is a
 * legitimate decision *after* buying quota; it must not happen by someone
 * tuning a number without seeing the cap.
 */
test("session replay sampling stays off so the 50/period cap is reserved for errors", () => {
	const config = clientConfig();
	const sessionRate = config.match(
		/replaysSessionSampleRate:\s*([^,\n]+)/,
	)?.[1];

	assert.ok(sessionRate, "replaysSessionSampleRate must be set explicitly");
	assert.equal(
		sessionRate.trim(),
		"0",
		"Session replay must stay at 0 — the org reserves only 50 replays per " +
			"billing period, and background session recording consumes the entire " +
			"allowance (dev burned it in one day on 07-28), leaving prod blind. " +
			"Buy quota before raising this.",
	);
});

test("error-triggered replay stays at full sampling", () => {
	const config = clientConfig();
	const errorRate = config.match(/replaysOnErrorSampleRate:\s*([^,\n]+)/)?.[1];

	assert.equal(
		errorRate?.trim(),
		"1.0",
		"The whole point of reserving the quota is that a real incident always " +
			"produces a replay.",
	);
});

test("the quota rationale stays next to the value", () => {
	const config = clientConfig();
	const preamble = config.slice(0, config.indexOf("replaysSessionSampleRate"));

	assert.match(
		preamble,
		/reserved\s*=\s*\*\*50/,
		"The measured cap must stay documented beside the setting — a bare 0 " +
			"reads as an arbitrary choice and invites someone to 'fix' it.",
	);
	assert.match(
		preamble,
		/organization/,
		"The org-wide scope is the reason dev's rate can blind prod; without it " +
			"the setting looks like a dev-only concern.",
	);
});
