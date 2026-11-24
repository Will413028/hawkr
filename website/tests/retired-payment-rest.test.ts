import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const ROOTS = ["app", "components", "features", "lib"];

function sourceFiles(dir: string): string[] {
	return readdirSync(join(ROOT, dir), { withFileTypes: true }).flatMap((e) => {
		const path = join(dir, e.name);
		if (e.isDirectory())
			return path.includes("codemeta") ? [] : sourceFiles(path);
		return /\.(ts|tsx)$/.test(path) && !/\.(unit\.)?test\.tsx?$/.test(path)
			? [path]
			: [];
	});
}

const stripComments = (source: string) =>
	source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(?<!:)\/\/[^\n]*/g, "");

/**
 * The gateway no longer serves the PayUni payment and ATM refund routes, and the
 * order and after-sale responses no longer carry the ATM fields; payments go
 * through the Connect `PaymentService`. A reintroduced call would 404 silently.
 */
const RETIRED = [
	/\/refund_page\b/,
	/\/atm-refund\b/,
	/\/app\/payments\b/,
	/\/admin\/payment_/,
	/\/admin\/grants\b/,
	/\/payment\/redirect\b/,
	/\bneedRefundPage\b/,
	/\batm(BankCode|VirtualAccount|ExpireDate)\b/,
	/\bestimatedRefundDate\b/,
	/\bbankDate\b/,
	/\bPaymentMethod\.ATM\b/,
];

test("no source calls a retired payment REST route or reads a removed ATM field", () => {
	const offenders = ROOTS.flatMap(sourceFiles).flatMap((file) => {
		const source = stripComments(readFileSync(join(ROOT, file), "utf8"));
		return RETIRED.filter((pattern) => pattern.test(source)).map(
			(pattern) => `${file} → ${pattern}`,
		);
	});
	assert.deepEqual(offenders, []);
});
