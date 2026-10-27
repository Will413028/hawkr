import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
	assertLighthouseVersion,
	buildLighthouseArgs,
	createDryRunPlan,
	DEFAULT_RUNS,
	executeBaseline,
	expectedSavedAssetPaths,
	JOURNEY_SCOPE,
	LIGHTHOUSE_SETTINGS,
	median,
	normalizeLighthouseRun,
	parseOptions,
	REQUIRED_LIGHTHOUSE_VERSION,
	runCli,
	summarizeTarget,
	validateServedTarget,
} from "../scripts/performance-baseline.mjs";

const ROOT = "/repo";
const URL = "https://example.com/";
const READY_MARKER = 'data-hk-initial-content-ready="home"';
const MARKER = "457c5ec8-release-1";
const REVISION = "08cae72f1ce12784d628e5ff8b2c13404006ae0d";

function requiredArgs(extra: string[] = []): string[] {
	return [
		"--url",
		URL,
		"--ready-marker",
		READY_MARKER,
		"--build-marker",
		MARKER,
		"--lighthouse-path",
		"/tools/lighthouse/cli/index.js",
		"--chrome-path",
		"/tools/chrome",
		...extra,
	];
}

function options(extra: string[] = []) {
	return parseOptions(requiredArgs(extra), {
		cwd: ROOT,
		env: {},
		now: () => new Date("2026-09-05T01:02:03.000Z"),
	});
}

function fakeLhr(value: number) {
	return {
		lighthouseVersion: REQUIRED_LIGHTHOUSE_VERSION,
		requestedUrl: URL,
		finalDisplayedUrl: URL,
		fetchTime: "2026-09-05T01:02:03.000Z",
		userAgent: "fake Chrome",
		environment: { networkUserAgent: "fake mobile" },
		categories: {
			performance: { score: value / 100 },
			accessibility: { score: 0.93 },
			"best-practices": { score: 1 },
			seo: { score: 1 },
		},
		audits: {
			"errors-in-console": { score: 1, details: { items: [] } },
			"http-status-code": { score: 1 },
			"first-contentful-paint": {
				numericValue: value * 10,
				numericUnit: "millisecond",
				score: 0.5,
			},
			"largest-contentful-paint": {
				numericValue: value * 100,
				numericUnit: "millisecond",
				score: 0.5,
			},
			"total-blocking-time": {
				numericValue: value,
				numericUnit: "millisecond",
				score: 0.5,
			},
			"cumulative-layout-shift": {
				numericValue: value / 1000,
				numericUnit: "unitless",
				score: 1,
			},
			"speed-index": {
				numericValue: value * 200,
				numericUnit: "millisecond",
				score: 0.5,
			},
		},
		runWarnings: [],
	};
}

function normalizeFakeLhr(lhr: ReturnType<typeof fakeLhr>) {
	return normalizeLighthouseRun({
		lhr,
		requestedUrl: URL,
		runNumber: 1,
		startedAtUtc: "2026-09-05T01:02:03.000Z",
		finishedAtUtc: "2026-09-05T01:02:04.000Z",
		elapsedMs: 1000,
		cliExitCode: 0,
		reportFile: "run.report.json",
		logFile: "run.log",
		traceFile: "run.trace.json",
		devtoolsLogFile: "run.devtoolslog.json",
	});
}

test("parseOptions defaults to five sequential cold runs and explicit evidence inputs", () => {
	const parsed = options();
	assert.equal(parsed.runs, DEFAULT_RUNS);
	assert.deepEqual(parsed.urls, [URL]);
	assert.deepEqual(parsed.buildMarker, {
		value: MARKER,
		source: "cli",
		verification: "must-match-served-html-data-dpl-id",
	});
	assert.deepEqual(parsed.targets, [{ url: URL, readyMarker: READY_MARKER }]);
	assert.equal(
		parsed.outputDir,
		"/repo/output/performance-baseline/20260905T010203Z",
	);

	const plan = createDryRunPlan(parsed);
	assert.equal(plan.sequential, true);
	assert.equal(plan.coldChromeProfilePerRun, true);
	assert.equal(plan.requiredLighthouseVersion, "13.4.1");
	assert.equal(plan.networkRequestsSent, 0);
	assert.match(plan.journeyScope.notCovered, /30-second interaction journey/);
	assert.deepEqual(plan.targets, parsed.targets);
	assert.equal(
		plan.measuredDocumentVerification.status,
		"requires-measured-document-verification",
	);
	assert.match(
		plan.measuredDocumentVerification.required,
		/filmstrip and trace/,
	);
});

test("invalid, duplicate, credentialed, and unbounded targets fail closed", () => {
	assert.throws(
		() =>
			parseOptions(["--url", URL, "--ready-marker", READY_MARKER], {
				cwd: ROOT,
				env: {},
			}),
		/--build-marker/,
	);
	assert.throws(() => options(["--url", URL]), /duplicate --url/);
	assert.throws(
		() => options(["--url", "https://user:pass@example.com/"]),
		/must not contain credentials/,
	);
	assert.throws(() => options(["--runs", "11"]), /integer from 1 to 10/);
	assert.throws(() => options(["--runs", "1.5"]), /integer from 1 to 10/);
	assert.throws(
		() =>
			parseOptions(
				requiredArgs(["--url", "https://example.com/market/products"]),
				{ cwd: ROOT, env: {} },
			),
		/repeat --ready-marker exactly once per --url/,
	);
	assert.throws(() => {
		const args = requiredArgs();
		args[3] = 'data-testid="generic-marker"';
		return parseOptions(args, { cwd: ROOT, env: {} });
	}, /exact public readiness attribute/);
});

test("the Lighthouse command pins the mobile model without blocking or deferring app work", () => {
	const args = buildLighthouseArgs({
		url: URL,
		reportPath: "/out/run.report.json",
		profilePath: "/tmp/cold-profile",
	});
	assert.equal(args[0], URL);
	assert.ok(args.includes("--form-factor=mobile"));
	assert.ok(args.includes("--screenEmulation.width=412"));
	assert.ok(args.includes("--screenEmulation.height=823"));
	assert.ok(args.includes("--screenEmulation.deviceScaleFactor=2.625"));
	assert.ok(args.includes("--throttling-method=simulate"));
	assert.ok(args.includes("--save-assets"));
	assert.ok(args.includes("--throttling.cpuSlowdownMultiplier=4"));
	assert.ok(
		args.includes(
			"--only-categories=performance,accessibility,best-practices,seo",
		),
	);
	assert.ok(
		args.some((arg) => arg.includes("--user-data-dir=/tmp/cold-profile")),
	);
	assert.equal(
		args.some((arg) => arg.includes("blocked-url-patterns")),
		false,
	);
	assert.equal(
		args.some((arg) => arg.includes("disable-storage-reset")),
		false,
	);
	assert.deepEqual(LIGHTHOUSE_SETTINGS.throttling, {
		rttMs: 150,
		throughputKbps: 1638.4,
		requestLatencyMs: 562.5,
		downloadThroughputKbps: 1474.56,
		uploadThroughputKbps: 675,
		cpuSlowdownMultiplier: 4,
	});
});

test("Lighthouse version is exact rather than a compatible range", () => {
	assert.equal(assertLighthouseVersion("13.4.1\n"), "13.4.1");
	assert.throws(
		() => assertLighthouseVersion("12.6.1\n"),
		/Lighthouse 13\.4\.1 is required; received 12\.6\.1/,
	);
	assert.throws(
		() => assertLighthouseVersion("13.4.2\n"),
		/Lighthouse 13\.4\.1 is required; received 13\.4\.2/,
	);
});

test("median and target summary use only report-valid runs without concealing invalidity", () => {
	assert.equal(median([55, 53, 90, 54, 51]), 54);
	assert.equal(median([2, 4]), 3);
	assert.equal(median([]), null);

	const records = [51, 55, 54, 56, 53].map((score, index) => ({
		reportStatus: "report-valid",
		categories: {
			performance: score,
			accessibility: 93,
			"best-practices": 100,
			seo: 100,
		},
		metrics: {
			firstContentfulPaint: { value: score * 10 },
			largestContentfulPaint: { value: score * 100 },
			totalBlockingTime: { value: score },
			cumulativeLayoutShift: { value: score / 1000 },
			speedIndex: { value: score * 200 },
		},
		runNumber: index + 1,
	}));
	records[4].reportStatus = "report-invalid";
	const summary = summarizeTarget({
		requestedUrl: URL,
		directory: "01-home",
		records,
	});
	assert.equal(summary.reportStatus, "report-invalid");
	assert.equal(summary.reportValidRuns, 4);
	assert.equal(summary.median.categories.performance, 54.5);
	assert.equal(summary.median.metrics.largestContentfulPaint, 5450);
});

test("console errors prevent an LHR from becoming report-valid", () => {
	const lhr = fakeLhr(55);
	lhr.audits["errors-in-console"] = {
		score: 0,
		details: { items: [{ description: "CORS blocked" }] },
	};
	const record = normalizeFakeLhr(lhr);
	assert.equal(record.reportStatus, "report-invalid");
	assert.match(
		record.problems.join("\n"),
		/browser console errors audit failed/,
	);
});

test("every requested category score and metric numeric value is required", () => {
	const missingSeo = fakeLhr(55);
	delete missingSeo.categories.seo;
	const seoRecord = normalizeFakeLhr(missingSeo);
	assert.equal(seoRecord.reportStatus, "report-invalid");
	assert.match(seoRecord.problems.join("\n"), /missing seo category score/);

	const missingLcp = fakeLhr(55);
	delete missingLcp.audits["largest-contentful-paint"].numericValue;
	const lcpRecord = normalizeFakeLhr(missingLcp);
	assert.equal(lcpRecord.reportStatus, "report-invalid");
	assert.match(
		lcpRecord.problems.join("\n"),
		/missing largest-contentful-paint numeric value/,
	);
});

test("the auxiliary document check requires HTTP HTML, route readiness, and matching build", async () => {
	const verified = await validateServedTarget(
		{ url: URL, readyMarker: READY_MARKER, expectedBuildMarker: MARKER },
		{
			fetchImpl: async () =>
				new Response(
					`<!doctype html><html data-dpl-id="${MARKER}"><main ${READY_MARKER}></main></html>`,
					{ status: 200, headers: { "content-type": "text/html" } },
				),
			clock: () => new Date("2026-09-05T01:02:03.000Z"),
		},
	);
	assert.equal(verified.status, "auxiliary-check-passed");
	assert.equal(verified.buildMarkerMatches, true);

	const wrongPage = await validateServedTarget(
		{ url: URL, readyMarker: READY_MARKER, expectedBuildMarker: MARKER },
		{
			fetchImpl: async () =>
				new Response(
					'<html data-dpl-id="some-other-build"><h1>Challenge</h1></html>',
					{ status: 200, headers: { "content-type": "text/html" } },
				),
		},
	);
	assert.equal(wrongPage.status, "auxiliary-check-failed");
	assert.match(wrongPage.problems.join("\n"), /readiness marker/);
	assert.match(wrongPage.problems.join("\n"), /did not match/);
});

test("--help and --dry-run never invoke the runtime executor", async () => {
	let executions = 0;
	const output: string[] = [];
	const executor = async () => {
		executions += 1;
		throw new Error("must not execute");
	};

	assert.equal(
		await runCli(["--help"], {
			executor,
			stdout: (text: string) => output.push(text),
		}),
		0,
	);
	assert.equal(
		await runCli([...requiredArgs(), "--dry-run"], {
			cwd: ROOT,
			env: {},
			now: () => new Date("2026-09-05T01:02:03.000Z"),
			executor,
			stdout: (text: string) => output.push(text),
		}),
		0,
	);
	assert.equal(executions, 0);
	assert.match(
		output.join("\n"),
		/does not replace the\n {2}separate real 30-second/,
	);
	assert.match(output.join("\n"), /"networkRequestsSent": 0/);
});

test("CLI success means collection success and still reports pending measured-document review", async () => {
	const output: string[] = [];
	const exitCode = await runCli(requiredArgs(), {
		cwd: ROOT,
		env: {},
		executor: async () => ({
			collectionSucceeded: true,
			verificationStatus: "requires-measured-document-verification",
		}),
		stdout: (text: string) => output.push(text),
	});
	assert.equal(exitCode, 0);
	assert.match(output.join("\n"), /collection succeeded/);
	assert.match(
		output.join("\n"),
		/verification requires-measured-document-verification/,
	);
});

test("executeBaseline runs five audits sequentially and writes raw and normalized evidence", async () => {
	const tempRoot = await mkdtemp(join(tmpdir(), "performance-baseline-test-"));
	const outputDir = join(tempRoot, "evidence");
	const parsed = parseOptions(
		requiredArgs(["--output-dir", outputDir, "--git-revision", REVISION]),
		{ cwd: ROOT, env: {} },
	);
	let activeAudits = 0;
	let maxActiveAudits = 0;
	let auditNumber = 0;

	const commandRunner = async (
		command: string,
		args: string[],
	): Promise<{
		code: number;
		stdout: string;
		stderr: string;
		timedOut: boolean;
	}> => {
		if (args.includes("--version") && command === process.execPath) {
			return { code: 0, stdout: "13.4.1\n", stderr: "", timedOut: false };
		}
		if (args.includes("--version")) {
			return {
				code: 0,
				stdout: "Google Chrome 151.0.7922.71\n",
				stderr: "",
				timedOut: false,
			};
		}

		activeAudits += 1;
		maxActiveAudits = Math.max(maxActiveAudits, activeAudits);
		auditNumber += 1;
		const reportArg = args.find((arg) => arg.startsWith("--output-path="));
		assert.ok(reportArg);
		const reportPath = reportArg.slice("--output-path=".length);
		await Promise.resolve();
		await writeFile(
			reportPath,
			JSON.stringify(fakeLhr(50 + auditNumber)),
			"utf8",
		);
		const assets = expectedSavedAssetPaths(reportPath);
		await writeFile(assets.tracePath, "{}", "utf8");
		await writeFile(assets.devtoolsLogPath, "{}", "utf8");
		activeAudits -= 1;
		return { code: 0, stdout: "", stderr: "", timedOut: false };
	};

	try {
		const summary = await executeBaseline(parsed, {
			commandRunner,
			fetchImpl: async () => {
				assert.equal(auditNumber, 5);
				return new Response(
					`<!doctype html><html data-dpl-id="${MARKER}"><main ${READY_MARKER}></main></html>`,
					{ status: 200, headers: { "content-type": "text/html" } },
				);
			},
			clock: () => new Date("2026-09-05T01:02:03.000Z"),
		});
		assert.equal(auditNumber, 5);
		assert.equal(maxActiveAudits, 1);
		assert.equal(summary.collectionSucceeded, true);
		assert.equal(
			summary.verificationStatus,
			"requires-measured-document-verification",
		);
		assert.equal(summary.targets[0].reportStatus, "report-valid");
		assert.equal(summary.targets[0].collectionSucceeded, true);
		assert.equal(
			summary.targets[0].auxiliaryDocumentCheck.status,
			"auxiliary-check-passed",
		);
		assert.equal(summary.gitRevision, REVISION);
		assert.equal(summary.targets[0].median.categories.performance, 53);
		assert.equal(
			summary.targets[0].median.metrics.largestContentfulPaint,
			5300,
		);
		assert.equal(summary.journeyScope, JOURNEY_SCOPE);

		const targetDir = join(outputDir, summary.targets[0].directory);
		const raw = JSON.parse(
			await readFile(join(targetDir, "run-01.report.json"), "utf8"),
		);
		const normalized = JSON.parse(
			await readFile(join(targetDir, "run-01.result.json"), "utf8"),
		);
		assert.equal(raw.lighthouseVersion, "13.4.1");
		assert.equal(normalized.reportStatus, "report-valid");
		assert.equal(normalized.requestedUrl, URL);
		assert.match(normalized.traceFile, /run-01\.report-0\.trace\.json$/);
		assert.match(
			normalized.devtoolsLogFile,
			/run-01\.report-0\.devtoolslog\.json$/,
		);
	} finally {
		await rm(tempRoot, { recursive: true, force: true });
	}
});
