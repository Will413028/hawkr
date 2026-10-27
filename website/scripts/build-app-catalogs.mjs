#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import {
	accessSync,
	constants,
	existsSync,
	mkdirSync,
	readFileSync,
} from "node:fs";
import { delimiter, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
	flutterCatalogBasePath,
	normalizeStorybookBasePath,
} from "./storybook-paths.mjs";

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

export function flutterCommand(cwd, env = process.env) {
	if (env.UI_LIBRARY_FLUTTER_BIN !== undefined) {
		if (!env.UI_LIBRARY_FLUTTER_BIN.trim())
			throw new Error("UI_LIBRARY_FLUTTER_BIN must name a Flutter executable.");
		return { command: env.UI_LIBRARY_FLUTTER_BIN, prefix: [] };
	}
	const local = resolve(cwd, ".fvm/flutter_sdk/bin/flutter");
	if (existsSync(local)) return { command: local, prefix: [] };
	for (const directory of (env.PATH || "").split(delimiter)) {
		if (!directory) continue;
		const fvm = resolve(directory, "fvm");
		try {
			accessSync(fvm, constants.X_OK);
			return { command: fvm, prefix: ["flutter"] };
		} catch {
			// Continue through PATH before using the Flutter action's SDK.
		}
	}
	return { command: "flutter", prefix: [] };
}

export function buildAppCatalog({
	cwd,
	output,
	basePath,
	app,
	env = process.env,
}) {
	if (!existsSync(resolve(cwd, "lib/main_catalog.dart")))
		throw new Error(
			`Missing catalog: ${cwd}. Set HAWKR_APPS_ROOT to the Flutter workspace.`,
		);
	const pinnedVersion = JSON.parse(
		readFileSync(resolve(cwd, ".fvmrc"), "utf8"),
	).flutter;
	const { command, prefix } = flutterCommand(cwd, env);
	const version = spawnSync(command, [...prefix, "--version", "--machine"], {
		cwd,
		env,
		encoding: "utf8",
	});
	if (version.error) throw version.error;
	if (version.status !== 0)
		throw new Error(`${app} Flutter version check failed: ${version.stderr}`);
	const actualVersion = JSON.parse(version.stdout).frameworkVersion;
	if (typeof pinnedVersion !== "string" || actualVersion !== pinnedVersion)
		throw new Error(
			`${app} Flutter SDK ${actualVersion} does not match .fvmrc ${pinnedVersion}.`,
		);
	mkdirSync(output, { recursive: true });
	const result = spawnSync(
		command,
		[
			...prefix,
			"build",
			"web",
			"--target",
			"lib/main_catalog.dart",
			"--output",
			output,
			"--base-href",
			flutterCatalogBasePath(basePath, app),
			"--no-web-resources-cdn",
		],
		{ cwd, env, stdio: "inherit" },
	);
	if (result.error) throw result.error;
	if (result.status !== 0)
		throw new Error(`${app} Flutter catalog build failed (${result.status}).`);
}

export function main() {
	const basePath = normalizeStorybookBasePath(
		process.env.STORYBOOK_BASE_PATH || "/",
	);
	const appsRoot = resolve(
		process.env.HAWKR_APPS_ROOT || resolve(webRoot, ".."),
	);
	for (const app of ["buyer", "seller"])
		buildAppCatalog({
			cwd: resolve(appsRoot, `${app}-app`),
			output: resolve(webRoot, "output/flutter", app),
			basePath,
			app,
		});
}

if (
	process.argv[1] &&
	resolve(process.argv[1]) === fileURLToPath(import.meta.url)
)
	main();
