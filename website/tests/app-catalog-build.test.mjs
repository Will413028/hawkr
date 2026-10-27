import assert from "node:assert/strict";
import {
	chmodSync,
	mkdirSync,
	mkdtempSync,
	readFileSync,
	rmSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
	buildAppCatalog,
	flutterCommand,
} from "../scripts/build-app-catalogs.mjs";

function fixture(t) {
	const root = mkdtempSync(join(tmpdir(), "hk-app-sdk-test-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	mkdirSync(join(root, "lib"));
	writeFileSync(join(root, "lib/main_catalog.dart"), "void main() {}\n");
	writeFileSync(join(root, ".fvmrc"), JSON.stringify({ flutter: "3.41.2" }));
	return root;
}

function fakeFlutter(root) {
	const path = join(root, "flutter sdk");
	writeFileSync(
		path,
		`#!${process.execPath}\nconst fs = require('node:fs');\nconst args = process.argv.slice(2);\nfs.appendFileSync(process.env.SDK_TEST_LOG, JSON.stringify(args) + '\\n');\nif (args[0] === '--version') console.log(JSON.stringify({frameworkVersion: process.env.SDK_TEST_VERSION}));\n`,
	);
	chmodSync(path, 0o755);
	return path;
}

test("Flutter selection prefers explicit CI executable, then local SDK, FVM and PATH Flutter", (t) => {
	const cwd = fixture(t);
	const local = join(cwd, ".fvm/flutter_sdk/bin/flutter");
	mkdirSync(join(local, ".."), { recursive: true });
	writeFileSync(local, "local SDK");
	assert.deepEqual(
		flutterCommand(cwd, {
			UI_LIBRARY_FLUTTER_BIN: "/CI/flutter sdk",
			PATH: "",
		}),
		{ command: "/CI/flutter sdk", prefix: [] },
	);
	assert.deepEqual(flutterCommand(cwd, { PATH: "" }), {
		command: local,
		prefix: [],
	});
	rmSync(local);
	const fvm = join(cwd, "fvm");
	writeFileSync(fvm, "fvm");
	chmodSync(fvm, 0o755);
	assert.deepEqual(flutterCommand(cwd, { PATH: cwd }), {
		command: fvm,
		prefix: ["flutter"],
	});
	assert.deepEqual(flutterCommand(cwd, { PATH: "" }), {
		command: "flutter",
		prefix: [],
	});
	assert.throws(
		() => flutterCommand(cwd, { UI_LIBRARY_FLUTTER_BIN: "" }),
		/UI_LIBRARY_FLUTTER_BIN/,
	);
});

test("CI Flutter override checks the pinned SDK and builds the snapshot base path without FVM", (t) => {
	const cwd = fixture(t);
	const log = join(cwd, "calls.jsonl");
	const output = join(cwd, "output");
	buildAppCatalog({
		cwd,
		output,
		basePath: "/admin/documentation/library/0.123.2/",
		app: "buyer",
		env: {
			...process.env,
			UI_LIBRARY_FLUTTER_BIN: fakeFlutter(cwd),
			SDK_TEST_VERSION: "3.41.2",
			SDK_TEST_LOG: log,
		},
	});
	const calls = readFileSync(log, "utf8").trim().split("\n").map(JSON.parse);
	assert.deepEqual(calls[0], ["--version", "--machine"]);
	assert.deepEqual(calls[1], [
		"build",
		"web",
		"--target",
		"lib/main_catalog.dart",
		"--output",
		output,
		"--base-href",
		"/admin/documentation/library/0.123.2/flutter/buyer/",
		"--no-web-resources-cdn",
	]);
});

test("an incorrect CI Flutter version fails before invoking build", (t) => {
	const cwd = fixture(t);
	const log = join(cwd, "calls.jsonl");
	assert.throws(
		() =>
			buildAppCatalog({
				cwd,
				output: join(cwd, "output"),
				basePath: "/",
				app: "seller",
				env: {
					...process.env,
					UI_LIBRARY_FLUTTER_BIN: fakeFlutter(cwd),
					SDK_TEST_VERSION: "3.38.5",
					SDK_TEST_LOG: log,
				},
			}),
		/does not match .fvmrc 3\.41\.2/,
	);
	assert.equal(readFileSync(log, "utf8").trim().split("\n").length, 1);
});
