#!/usr/bin/env node

/**
 * Reproducible Lighthouse navigation baseline for PERF-00.
 *
 * Scope is deliberately narrow: public, cold, initial navigations. This tool
 * does not build/start/deploy the app, mutate page behaviour, authenticate a
 * user, or pretend that a navigation audit covers the separate 30-second
 * interaction journey required by the performance plan.
 */

import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import {
	access,
	mkdir,
	mkdtemp,
	readdir,
	readFile,
	rm,
	writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { extname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";

export const REQUIRED_LIGHTHOUSE_VERSION = "13.4.1";
export const DEFAULT_RUNS = 5;
export const MAX_RUNS = 10;
export const RUN_TIMEOUT_MS = 180_000;
export const SERVED_VALIDATION_TIMEOUT_MS = 15_000;
export const MAX_SERVED_HTML_BYTES = 5_000_000;

export const LIGHTHOUSE_SETTINGS = Object.freeze({
	formFactor: "mobile",
	locale: "en-US",
	throttlingMethod: "simulate",
	throttling: Object.freeze({
		rttMs: 150,
		throughputKbps: 1638.4,
		requestLatencyMs: 562.5,
		downloadThroughputKbps: 1474.56,
		uploadThroughputKbps: 675,
		cpuSlowdownMultiplier: 4,
	}),
	screenEmulation: Object.freeze({
		mobile: true,
		width: 412,
		height: 823,
		deviceScaleFactor: 2.625,
		disabled: false,
	}),
	categories: Object.freeze([
		"performance",
		"accessibility",
		"best-practices",
		"seo",
	]),
	maxWaitForLoadMs: 90_000,
	saveAssets: true,
});

export const JOURNEY_SCOPE = Object.freeze({
	covered: "cold initial navigation audited by Lighthouse",
	notCovered:
		"the real 30-second interaction journey (SPA navigation, chat, Hawkr Agent, and add-to-cart) is a separate browser acceptance suite and is not inferred from this baseline",
});

export const MEASURED_DOCUMENT_VERIFICATION = Object.freeze({
	status: "requires-measured-document-verification",
	required:
		"manually review every measured run's filmstrip and trace, and confirm the representative data/content page is the intended page; the later auxiliary document fetch cannot prove what Lighthouse measured",
});

const METRIC_AUDITS = Object.freeze({
	firstContentfulPaint: "first-contentful-paint",
	largestContentfulPaint: "largest-contentful-paint",
	totalBlockingTime: "total-blocking-time",
	cumulativeLayoutShift: "cumulative-layout-shift",
	speedIndex: "speed-index",
});

const CATEGORY_IDS = LIGHTHOUSE_SETTINGS.categories;

export function usage() {
	return `Hawkr reproducible Lighthouse baseline

Usage:
  node scripts/performance-baseline.mjs \\
    --url https://example.com/ \\
    --ready-marker 'data-hk-initial-content-ready="home"' \\
    [--url https://example.com/market/products \\
     --ready-marker 'data-hk-tab-content-ready="market"'] \\
    --build-marker <data-dpl-id-or-local-build-id> \\
    --lighthouse-path /absolute/path/to/lighthouse-13.4.1/cli/index.js \\
    --chrome-path /absolute/path/to/Chrome \\
    [--runs 5] [--output-dir output/performance-baseline/<utc>] \\
    [--git-revision <sha>] [--dry-run]

Required runtime contract:
  - Lighthouse must report exactly ${REQUIRED_LIGHTHOUSE_VERSION}; any other version fails before a URL is opened.
  - Runs are sequential. Each run uses a new temporary Chrome profile.
  - Repeat --ready-marker once per --url, in the same order. After that URL's
    cold runs, an auxiliary fetch checks HTTP 200, the public ready attribute,
    and served data-dpl-id. It cannot prove which document Lighthouse measured.
  - --build-marker must exactly match served HTML data-dpl-id. A caller-supplied
    marker alone is never treated as served-runtime proof.
  - Trace and DevTools log artifacts are required for every report.
  - A successful collection still requires manual review of every run's
    filmstrip/trace and the intended representative data/content page.
  - Exit code 0 means collection succeeded, not performance acceptance passed.

Offline modes:
  --help       Print this text. Does not inspect tools, files, Git, or URLs.
  --dry-run    Print the resolved plan. Does not spawn Lighthouse/Chrome/Git,
               create output files, or send network requests.

Scope:
  This command measures cold initial navigation only. It does not replace the
  separate real 30-second SPA/chat/Hawkr Agent/add-to-cart journey acceptance.
`;
}

function timestampSlug(date) {
	return date
		.toISOString()
		.replace(/[-:]/g, "")
		.replace(/\.\d{3}Z$/, "Z");
}

function requiredString(value, name) {
	if (typeof value !== "string" || value.trim() === "") {
		throw new Error(`${name} is required`);
	}
	return value.trim();
}

function parseRunCount(raw) {
	const value = Number(raw);
	if (!Number.isInteger(value) || value < 1 || value > MAX_RUNS) {
		throw new Error(`--runs must be an integer from 1 to ${MAX_RUNS}`);
	}
	return value;
}

function normalizeTarget(raw) {
	let url;
	try {
		url = new URL(raw);
	} catch {
		throw new Error(`--url must be an absolute http(s) URL: ${raw}`);
	}
	if (!["http:", "https:"].includes(url.protocol)) {
		throw new Error(`--url must use http or https: ${raw}`);
	}
	if (url.username || url.password) {
		throw new Error("--url must not contain credentials");
	}
	if (url.hash) {
		throw new Error("--url must not contain a fragment");
	}
	return url.href;
}

function normalizeReadyMarker(raw) {
	const value = requiredString(raw, "--ready-marker");
	if (value.length > 300 || /[\r\n]/.test(value)) {
		throw new Error(
			"--ready-marker must be one line of at most 300 characters",
		);
	}
	if (
		!/^data-hk-(?:initial-content-ready|tab-content-ready|page-content-ready)="[^"]+"$/.test(
			value,
		)
	) {
		throw new Error(
			'--ready-marker must be an exact public readiness attribute, for example data-hk-initial-content-ready="home"',
		);
	}
	return value;
}

function normalizeRevision(raw) {
	const value = raw?.trim();
	if (!value) return undefined;
	if (!/^[0-9a-f]{7,64}$/i.test(value)) {
		throw new Error(
			"--git-revision must be a 7-64 character hexadecimal revision",
		);
	}
	return value.toLowerCase();
}

export function parseOptions(
	argv,
	{ env = process.env, cwd = process.cwd(), now = () => new Date() } = {},
) {
	const { values } = parseArgs({
		args: argv,
		options: {
			help: { type: "boolean", short: "h", default: false },
			"dry-run": { type: "boolean", default: false },
			url: { type: "string", multiple: true },
			"ready-marker": { type: "string", multiple: true },
			runs: { type: "string", default: String(DEFAULT_RUNS) },
			"output-dir": { type: "string" },
			"build-marker": { type: "string" },
			"git-revision": { type: "string" },
			"lighthouse-path": { type: "string" },
			"chrome-path": { type: "string" },
		},
		allowPositionals: false,
		strict: true,
	});

	if (values.help) return { help: true };

	const rawUrls = values.url ?? [];
	if (rawUrls.length === 0) throw new Error("at least one --url is required");
	const urls = rawUrls.map(normalizeTarget);
	if (new Set(urls).size !== urls.length) {
		throw new Error("duplicate --url values are not allowed");
	}
	const rawReadyMarkers = values["ready-marker"] ?? [];
	if (rawReadyMarkers.length !== urls.length) {
		throw new Error(
			"repeat --ready-marker exactly once per --url, in the same order",
		);
	}
	const readyMarkers = rawReadyMarkers.map(normalizeReadyMarker);
	const targets = urls.map((url, index) => ({
		url,
		readyMarker: readyMarkers[index],
	}));

	const markerFromCli = values["build-marker"]?.trim();
	const markerFromEnv = env.DEPLOYMENT_ID?.trim();
	const buildMarker = requiredString(
		markerFromCli || markerFromEnv,
		"--build-marker (or DEPLOYMENT_ID)",
	);
	if (buildMarker.length > 200 || /[\r\n]/.test(buildMarker)) {
		throw new Error(
			"--build-marker must be one line of at most 200 characters",
		);
	}

	const lighthousePath = resolve(
		cwd,
		requiredString(
			values["lighthouse-path"] || env.LIGHTHOUSE_PATH,
			"--lighthouse-path (or LIGHTHOUSE_PATH)",
		),
	);
	const chromePath = resolve(
		cwd,
		requiredString(
			values["chrome-path"] || env.CHROME_PATH,
			"--chrome-path (or CHROME_PATH)",
		),
	);
	const outputDir = resolve(
		cwd,
		values["output-dir"] ||
			join("output", "performance-baseline", timestampSlug(now())),
	);

	return {
		help: false,
		dryRun: values["dry-run"],
		urls,
		targets,
		runs: parseRunCount(values.runs),
		outputDir,
		buildMarker: {
			value: buildMarker,
			source: markerFromCli ? "cli" : "DEPLOYMENT_ID",
			verification: "must-match-served-html-data-dpl-id",
		},
		gitRevision: normalizeRevision(values["git-revision"]),
		lighthousePath,
		chromePath,
		cwd: resolve(cwd),
	};
}

function portablePath(path, cwd) {
	const local = relative(cwd, path);
	return local && !local.startsWith("..") && !isAbsolute(local) ? local : path;
}

export function configurationFingerprint(settings = LIGHTHOUSE_SETTINGS) {
	return createHash("sha256").update(JSON.stringify(settings)).digest("hex");
}

export function createDryRunPlan(options) {
	return {
		kind: "hawkr-performance-baseline-dry-run",
		willExecute: false,
		networkRequestsSent: 0,
		filesWritten: 0,
		requiredLighthouseVersion: REQUIRED_LIGHTHOUSE_VERSION,
		urls: options.urls,
		targets: options.targets,
		runsPerUrl: options.runs,
		sequential: true,
		coldChromeProfilePerRun: true,
		buildMarker: options.buildMarker,
		gitRevision: options.gitRevision ?? "auto-from-local-git-at-runtime",
		lighthousePath: portablePath(options.lighthousePath, options.cwd),
		chromePath: portablePath(options.chromePath, options.cwd),
		outputDir: portablePath(options.outputDir, options.cwd),
		configurationFingerprint: configurationFingerprint(),
		configuration: LIGHTHOUSE_SETTINGS,
		journeyScope: JOURNEY_SCOPE,
		measuredDocumentVerification: MEASURED_DOCUMENT_VERIFICATION,
	};
}

function jsCli(path) {
	return [".js", ".mjs", ".cjs"].includes(extname(path));
}

export function lighthouseInvocation(lighthousePath, args = []) {
	return jsCli(lighthousePath)
		? { command: process.execPath, args: [lighthousePath, ...args] }
		: { command: lighthousePath, args };
}

function fixedLighthouseArgs() {
	const { screenEmulation, throttling } = LIGHTHOUSE_SETTINGS;
	return [
		"--quiet",
		"--output=json",
		"--save-assets",
		`--only-categories=${LIGHTHOUSE_SETTINGS.categories.join(",")}`,
		`--locale=${LIGHTHOUSE_SETTINGS.locale}`,
		`--form-factor=${LIGHTHOUSE_SETTINGS.formFactor}`,
		`--screenEmulation.mobile=${screenEmulation.mobile}`,
		`--screenEmulation.width=${screenEmulation.width}`,
		`--screenEmulation.height=${screenEmulation.height}`,
		`--screenEmulation.deviceScaleFactor=${screenEmulation.deviceScaleFactor}`,
		`--screenEmulation.disabled=${screenEmulation.disabled}`,
		`--throttling-method=${LIGHTHOUSE_SETTINGS.throttlingMethod}`,
		`--throttling.rttMs=${throttling.rttMs}`,
		`--throttling.throughputKbps=${throttling.throughputKbps}`,
		`--throttling.requestLatencyMs=${throttling.requestLatencyMs}`,
		`--throttling.downloadThroughputKbps=${throttling.downloadThroughputKbps}`,
		`--throttling.uploadThroughputKbps=${throttling.uploadThroughputKbps}`,
		`--throttling.cpuSlowdownMultiplier=${throttling.cpuSlowdownMultiplier}`,
		`--max-wait-for-load=${LIGHTHOUSE_SETTINGS.maxWaitForLoadMs}`,
	];
}

export function buildLighthouseArgs({ url, reportPath, profilePath }) {
	return [
		url,
		...fixedLighthouseArgs(),
		`--output-path=${reportPath}`,
		`--chrome-flags=--headless=new --no-sandbox --disable-dev-shm-usage --user-data-dir=${profilePath}`,
	];
}

export function parseVersionOutput(output, label) {
	const match = String(output).match(/\b(\d+\.\d+\.\d+(?:\.\d+)?)\b/);
	if (!match)
		throw new Error(`could not parse ${label} version from: ${output}`);
	return match[1];
}

export function assertLighthouseVersion(output) {
	const actual = parseVersionOutput(output, "Lighthouse");
	if (actual !== REQUIRED_LIGHTHOUSE_VERSION) {
		throw new Error(
			`Lighthouse ${REQUIRED_LIGHTHOUSE_VERSION} is required; received ${actual}`,
		);
	}
	return actual;
}

export function median(values) {
	if (!Array.isArray(values) || values.length === 0) return null;
	const sorted = values
		.filter((value) => Number.isFinite(value))
		.map(Number)
		.sort((a, b) => a - b);
	if (sorted.length === 0) return null;
	const middle = Math.floor(sorted.length / 2);
	return sorted.length % 2 === 1
		? sorted[middle]
		: (sorted[middle - 1] + sorted[middle]) / 2;
}

function auditResult(lhr, auditId) {
	const audit = lhr?.audits?.[auditId];
	if (!audit || !Number.isFinite(audit.numericValue)) return null;
	return {
		value: audit.numericValue,
		unit: audit.numericUnit ?? null,
		score: Number.isFinite(audit.score) ? audit.score : null,
	};
}

function categoryScores(lhr) {
	return Object.fromEntries(
		CATEGORY_IDS.map((id) => {
			const raw = lhr?.categories?.[id]?.score;
			return [id, Number.isFinite(raw) ? Math.round(raw * 1000) / 10 : null];
		}),
	);
}

function comparableUrl(raw) {
	const url = new URL(raw);
	const pathname =
		url.pathname.length > 1 ? url.pathname.replace(/\/+$/, "") : "/";
	return `${url.origin}${pathname}${url.search}`;
}

export function normalizeLighthouseRun({
	lhr,
	requestedUrl,
	runNumber,
	startedAtUtc,
	finishedAtUtc,
	elapsedMs,
	cliExitCode,
	reportFile,
	logFile,
	traceFile,
	devtoolsLogFile,
	assetProblems = [],
}) {
	const finalUrl = lhr?.finalDisplayedUrl ?? lhr?.finalUrl ?? null;
	const runtimeError = lhr?.runtimeError ?? null;
	const metrics = Object.fromEntries(
		Object.entries(METRIC_AUDITS).map(([key, auditId]) => [
			key,
			auditResult(lhr, auditId),
		]),
	);
	const categories = categoryScores(lhr);
	const consoleErrorsAudit = lhr?.audits?.["errors-in-console"];
	const httpStatusAudit = lhr?.audits?.["http-status-code"];
	const consoleErrorCount = Array.isArray(consoleErrorsAudit?.details?.items)
		? consoleErrorsAudit.details.items.length
		: null;
	const problems = [...assetProblems];
	if (cliExitCode !== 0) problems.push(`Lighthouse exited ${cliExitCode}`);
	if (lhr?.lighthouseVersion !== REQUIRED_LIGHTHOUSE_VERSION) {
		problems.push(
			`report used Lighthouse ${lhr?.lighthouseVersion ?? "unknown"}, expected ${REQUIRED_LIGHTHOUSE_VERSION}`,
		);
	}
	if (runtimeError)
		problems.push(`runtime error: ${runtimeError.code ?? "unknown"}`);
	for (const categoryId of CATEGORY_IDS) {
		if (categories[categoryId] === null) {
			problems.push(`missing ${categoryId} category score`);
		}
	}
	for (const [metricName, auditId] of Object.entries(METRIC_AUDITS)) {
		if (metrics[metricName] === null) {
			problems.push(`missing ${auditId} numeric value`);
		}
	}
	if (consoleErrorsAudit?.score !== 1) {
		problems.push(
			consoleErrorsAudit
				? `browser console errors audit failed${consoleErrorCount === null ? "" : ` (${consoleErrorCount} item(s))`}`
				: "missing errors-in-console audit",
		);
	}
	if (httpStatusAudit?.score !== 1) {
		problems.push(
			httpStatusAudit
				? "document HTTP status audit failed"
				: "missing http-status-code audit",
		);
	}
	if (!finalUrl) {
		problems.push("missing final URL");
	} else if (comparableUrl(finalUrl) !== comparableUrl(requestedUrl)) {
		problems.push(`unexpected final URL: ${finalUrl}`);
	}

	return {
		runNumber,
		reportStatus: problems.length === 0 ? "report-valid" : "report-invalid",
		problems,
		requestedUrl,
		finalUrl,
		fetchTimeUtc: lhr?.fetchTime ?? null,
		startedAtUtc,
		finishedAtUtc,
		elapsedMs,
		cliExitCode,
		reportFile,
		logFile,
		traceFile,
		devtoolsLogFile,
		lighthouseVersion: lhr?.lighthouseVersion ?? null,
		userAgent: lhr?.userAgent ?? null,
		environment: lhr?.environment ?? null,
		requiredAudits: {
			errorsInConsoleScore: consoleErrorsAudit?.score ?? null,
			consoleErrorCount,
			httpStatusCodeScore: httpStatusAudit?.score ?? null,
		},
		categories,
		metrics,
		runWarnings: Array.isArray(lhr?.runWarnings) ? lhr.runWarnings : [],
	};
}

function medianCategory(records, id) {
	return median(
		records.map((record) => record.categories?.[id]).filter(Number.isFinite),
	);
}

function medianMetric(records, key) {
	return median(
		records
			.map((record) => record.metrics?.[key]?.value)
			.filter(Number.isFinite),
	);
}

export function summarizeTarget({ requestedUrl, directory, records }) {
	const valid = records.filter(
		(record) => record.reportStatus === "report-valid",
	);
	return {
		requestedUrl,
		directory,
		reportStatus:
			valid.length === records.length ? "report-valid" : "report-invalid",
		reportValidRuns: valid.length,
		totalRuns: records.length,
		median: {
			categories: Object.fromEntries(
				CATEGORY_IDS.map((id) => [id, medianCategory(valid, id)]),
			),
			metrics: Object.fromEntries(
				Object.keys(METRIC_AUDITS).map((key) => [
					key,
					medianMetric(valid, key),
				]),
			),
		},
		runs: records,
	};
}

function targetDirectoryName(url, index) {
	const parsed = new URL(url);
	const route = parsed.pathname === "/" ? "home" : parsed.pathname.slice(1);
	const slug = `${parsed.hostname}-${route}`
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "-")
		.replace(/^-+|-+$/g, "")
		.slice(0, 80);
	return `${String(index + 1).padStart(2, "0")}-${slug || "target"}`;
}

export function expectedSavedAssetPaths(reportPath) {
	const extension = extname(reportPath);
	const base = extension ? reportPath.slice(0, -extension.length) : reportPath;
	return {
		tracePath: `${base}-0.trace.json`,
		devtoolsLogPath: `${base}-0.devtoolslog.json`,
	};
}

async function inspectSavedAssets(reportPath, outputDir) {
	const { tracePath, devtoolsLogPath } = expectedSavedAssetPaths(reportPath);
	const problems = [];
	for (const [label, path] of [
		["trace", tracePath],
		["DevTools log", devtoolsLogPath],
	]) {
		try {
			await access(path);
		} catch {
			problems.push(`missing saved ${label} artifact`);
		}
	}
	return {
		traceFile: relative(outputDir, tracePath),
		devtoolsLogFile: relative(outputDir, devtoolsLogPath),
		problems,
	};
}

function servedDeploymentId(html) {
	return html.match(/\bdata-dpl-id=(["'])(.*?)\1/i)?.[2] ?? null;
}

export async function validateServedTarget(
	{ url, readyMarker, expectedBuildMarker },
	{
		fetchImpl = globalThis.fetch,
		clock = () => new Date(),
		timeoutMs = SERVED_VALIDATION_TIMEOUT_MS,
	} = {},
) {
	const checkedAtUtc = clock().toISOString();
	const base = {
		checkedAtUtc,
		requestedUrl: url,
		readyMarker,
		expectedBuildMarker,
	};

	try {
		const response = await fetchImpl(url, {
			method: "GET",
			redirect: "manual",
			cache: "no-store",
			headers: {
				accept: "text/html",
				"cache-control": "no-cache",
				"user-agent": "hawkr-performance-baseline/1",
			},
			signal: AbortSignal.timeout(timeoutMs),
		});
		const problems = [];
		const contentType = response.headers.get("content-type");
		const declaredLength = Number(response.headers.get("content-length"));
		if (response.status !== 200)
			problems.push(`served HTTP ${response.status}`);
		if (!contentType?.toLowerCase().includes("text/html")) {
			problems.push(`served content-type was ${contentType ?? "missing"}`);
		}
		if (
			Number.isFinite(declaredLength) &&
			declaredLength > MAX_SERVED_HTML_BYTES
		) {
			problems.push(
				`served HTML exceeded ${MAX_SERVED_HTML_BYTES} byte validation limit`,
			);
		}

		const html = await response.text();
		if (Buffer.byteLength(html, "utf8") > MAX_SERVED_HTML_BYTES) {
			problems.push(
				`served HTML exceeded ${MAX_SERVED_HTML_BYTES} byte validation limit`,
			);
		}
		const readyMarkerFound = html.includes(readyMarker);
		if (!readyMarkerFound)
			problems.push("public readiness marker was not served");
		const observedBuildMarker = servedDeploymentId(html);
		if (!observedBuildMarker) {
			problems.push("served HTML had no data-dpl-id");
		} else if (observedBuildMarker !== expectedBuildMarker) {
			problems.push(`served data-dpl-id did not match declared build marker`);
		}

		return {
			...base,
			status:
				problems.length === 0
					? "auxiliary-check-passed"
					: "auxiliary-check-failed",
			problems,
			httpStatus: response.status,
			servedUrl: response.url || url,
			contentType,
			readyMarkerFound,
			observedBuildMarker,
			buildMarkerMatches: observedBuildMarker === expectedBuildMarker,
		};
	} catch (error) {
		return {
			...base,
			status: "auxiliary-check-failed",
			problems: [
				`served document validation failed: ${
					error instanceof Error ? error.message : String(error)
				}`,
			],
			httpStatus: null,
			servedUrl: null,
			contentType: null,
			readyMarkerFound: false,
			observedBuildMarker: null,
			buildMarkerMatches: false,
		};
	}
}

async function writeJson(path, value) {
	await writeFile(path, `${JSON.stringify(value, null, 2)}\n`, "utf8");
}

export async function runCommand(
	command,
	args,
	{ cwd = process.cwd(), env = process.env, timeoutMs = RUN_TIMEOUT_MS } = {},
) {
	return new Promise((resolvePromise, rejectPromise) => {
		const child = spawn(command, args, {
			cwd,
			env,
			shell: false,
			stdio: ["ignore", "pipe", "pipe"],
		});
		let stdout = "";
		let stderr = "";
		let timedOut = false;
		const timer = setTimeout(() => {
			timedOut = true;
			child.kill("SIGTERM");
		}, timeoutMs);
		child.stdout.on("data", (chunk) => {
			stdout += String(chunk);
		});
		child.stderr.on("data", (chunk) => {
			stderr += String(chunk);
		});
		child.on("error", (error) => {
			clearTimeout(timer);
			rejectPromise(error);
		});
		child.on("close", (code, signal) => {
			clearTimeout(timer);
			resolvePromise({
				code: code ?? 1,
				signal,
				stdout,
				stderr,
				timedOut,
			});
		});
	});
}

async function assertEmptyOrCreateOutputDir(outputDir) {
	await mkdir(outputDir, { recursive: true });
	const entries = await readdir(outputDir);
	if (entries.length !== 0) {
		throw new Error(`--output-dir must be empty: ${outputDir}`);
	}
}

async function gitRevision(options, commandRunner) {
	if (options.gitRevision) return options.gitRevision;
	const result = await commandRunner("git", ["rev-parse", "HEAD"], {
		cwd: options.cwd,
		timeoutMs: 30_000,
	});
	if (result.code !== 0) {
		throw new Error(`git rev-parse HEAD failed: ${result.stderr.trim()}`);
	}
	return normalizeRevision(result.stdout);
}

async function toolchain(options, commandRunner) {
	const versionInvocation = lighthouseInvocation(options.lighthousePath, [
		"--version",
	]);
	const lighthouse = await commandRunner(
		versionInvocation.command,
		versionInvocation.args,
		{ cwd: options.cwd, timeoutMs: 30_000 },
	);
	if (lighthouse.code !== 0) {
		throw new Error(
			`Lighthouse version check failed: ${lighthouse.stderr.trim()}`,
		);
	}
	const lighthouseVersion = assertLighthouseVersion(
		`${lighthouse.stdout}\n${lighthouse.stderr}`,
	);

	const chrome = await commandRunner(options.chromePath, ["--version"], {
		cwd: options.cwd,
		timeoutMs: 30_000,
	});
	if (chrome.code !== 0) {
		throw new Error(`Chrome version check failed: ${chrome.stderr.trim()}`);
	}
	const chromeVersion = parseVersionOutput(
		`${chrome.stdout}\n${chrome.stderr}`,
		"Chrome",
	);
	return {
		lighthouseVersion,
		chromeVersion,
		chromeVersionOutput: chrome.stdout.trim(),
	};
}

export async function executeBaseline(
	options,
	{
		commandRunner = runCommand,
		fetchImpl = globalThis.fetch,
		clock = () => new Date(),
		makeProfile = () => mkdtemp(join(tmpdir(), "hawkr-lighthouse-")),
	} = {},
) {
	const startedAtUtc = clock().toISOString();
	const tools = await toolchain(options, commandRunner);
	const revision = await gitRevision(options, commandRunner);
	if (!revision) throw new Error("could not resolve git revision");

	await assertEmptyOrCreateOutputDir(options.outputDir);
	const manifest = {
		schemaVersion: 2,
		kind: "hawkr-performance-baseline",
		collectionSucceeded: null,
		verificationStatus: "collection-running",
		startedAtUtc,
		buildMarker: options.buildMarker,
		gitRevision: revision,
		tools: {
			lighthouse: {
				requiredVersion: REQUIRED_LIGHTHOUSE_VERSION,
				actualVersion: tools.lighthouseVersion,
				path: portablePath(options.lighthousePath, options.cwd),
				invokedWithNode: jsCli(options.lighthousePath),
			},
			chrome: {
				version: tools.chromeVersion,
				versionOutput: tools.chromeVersionOutput,
				path: portablePath(options.chromePath, options.cwd),
			},
		},
		runsPerUrl: options.runs,
		sequential: true,
		coldChromeProfilePerRun: true,
		configurationFingerprint: configurationFingerprint(),
		configuration: LIGHTHOUSE_SETTINGS,
		journeyScope: JOURNEY_SCOPE,
		measuredDocumentVerification: MEASURED_DOCUMENT_VERIFICATION,
		targets: options.targets,
	};
	await writeJson(join(options.outputDir, "manifest.json"), manifest);

	const targets = [];
	for (const [targetIndex, target] of options.targets.entries()) {
		const { url, readyMarker } = target;
		const directory = targetDirectoryName(url, targetIndex);
		const targetDir = join(options.outputDir, directory);
		await mkdir(targetDir);
		const records = [];

		for (let runIndex = 0; runIndex < options.runs; runIndex += 1) {
			const runNumber = runIndex + 1;
			const prefix = `run-${String(runNumber).padStart(2, "0")}`;
			const reportPath = join(targetDir, `${prefix}.report.json`);
			const resultPath = join(targetDir, `${prefix}.result.json`);
			const logPath = join(targetDir, `${prefix}.lighthouse.log`);
			const profilePath = await makeProfile();
			const runStarted = clock();
			let processResult = {
				code: 1,
				stdout: "",
				stderr: "",
				timedOut: false,
			};
			let lhr = null;
			let savedAssets = null;
			let infrastructureError = null;

			try {
				const args = buildLighthouseArgs({ url, reportPath, profilePath });
				const invocation = lighthouseInvocation(options.lighthousePath, args);
				processResult = await commandRunner(
					invocation.command,
					invocation.args,
					{
						cwd: options.cwd,
						env: { ...process.env, CHROME_PATH: options.chromePath },
						timeoutMs: RUN_TIMEOUT_MS,
					},
				);
				await writeFile(
					logPath,
					`${processResult.stdout}${processResult.stderr}`,
					"utf8",
				);
				lhr = JSON.parse(await readFile(reportPath, "utf8"));
				savedAssets = await inspectSavedAssets(reportPath, options.outputDir);
			} catch (error) {
				infrastructureError =
					error instanceof Error ? error.message : String(error);
				await writeFile(
					logPath,
					`${processResult.stdout}${processResult.stderr}\n${infrastructureError}\n`,
					"utf8",
				);
			} finally {
				// makeProfile() owns this exact OS-temp child. Never delete a caller path.
				await rm(profilePath, { recursive: true, force: true });
			}

			const runFinished = clock();
			let record;
			if (lhr) {
				record = normalizeLighthouseRun({
					lhr,
					requestedUrl: url,
					runNumber,
					startedAtUtc: runStarted.toISOString(),
					finishedAtUtc: runFinished.toISOString(),
					elapsedMs: Math.max(0, runFinished.getTime() - runStarted.getTime()),
					cliExitCode: processResult.code,
					reportFile: relative(options.outputDir, reportPath),
					logFile: relative(options.outputDir, logPath),
					traceFile: savedAssets.traceFile,
					devtoolsLogFile: savedAssets.devtoolsLogFile,
					assetProblems: savedAssets.problems,
				});
			} else {
				record = {
					runNumber,
					reportStatus: "report-invalid",
					problems: [
						infrastructureError || "Lighthouse report was not produced",
					],
					requestedUrl: url,
					startedAtUtc: runStarted.toISOString(),
					finishedAtUtc: runFinished.toISOString(),
					elapsedMs: Math.max(0, runFinished.getTime() - runStarted.getTime()),
					cliExitCode: processResult.code,
					timedOut: processResult.timedOut,
					reportFile: null,
					logFile: relative(options.outputDir, logPath),
				};
			}
			await writeJson(resultPath, record);
			records.push(record);
		}
		const auxiliaryDocumentCheck = await validateServedTarget(
			{
				url,
				readyMarker,
				expectedBuildMarker: options.buildMarker.value,
			},
			{ fetchImpl, clock },
		);
		const targetSummary = summarizeTarget({
			requestedUrl: url,
			directory,
			records,
		});
		const collectionSucceeded =
			targetSummary.reportStatus === "report-valid" &&
			auxiliaryDocumentCheck.status === "auxiliary-check-passed";
		targets.push({
			...targetSummary,
			auxiliaryDocumentCheck,
			collectionSucceeded,
			verificationStatus: collectionSucceeded
				? "requires-measured-document-verification"
				: "ineligible",
		});
	}

	const finishedAtUtc = clock().toISOString();
	const collectionSucceeded = targets.every(
		(target) => target.collectionSucceeded,
	);
	const summary = {
		...manifest,
		collectionSucceeded,
		verificationStatus: collectionSucceeded
			? "requires-measured-document-verification"
			: "ineligible",
		finishedAtUtc,
		targets,
	};
	await writeJson(join(options.outputDir, "summary.json"), summary);
	return summary;
}

export async function runCli(
	argv,
	{
		env = process.env,
		cwd = process.cwd(),
		now = () => new Date(),
		stdout = (text) => process.stdout.write(text),
		stderr = (text) => process.stderr.write(text),
		executor = executeBaseline,
	} = {},
) {
	try {
		if (argv.includes("--help") || argv.includes("-h")) {
			stdout(usage());
			return 0;
		}
		const options = parseOptions(argv, { env, cwd, now });
		if (options.dryRun) {
			stdout(`${JSON.stringify(createDryRunPlan(options), null, 2)}\n`);
			return 0;
		}
		const summary = await executor(options);
		stdout(
			`performance baseline collection ${summary.collectionSucceeded ? "succeeded" : "failed"}; verification ${summary.verificationStatus}: ${portablePath(options.outputDir, options.cwd)}/summary.json\n`,
		);
		return summary.collectionSucceeded ? 0 : 1;
	} catch (error) {
		stderr(`${error instanceof Error ? error.message : String(error)}\n`);
		return 1;
	}
}

const isMain =
	process.argv[1] &&
	resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) process.exitCode = await runCli(process.argv.slice(2));
