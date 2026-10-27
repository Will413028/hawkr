import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import postcss from "postcss";

const defaultRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const contractPath = "design-system/cross-platform-tokens.json";
const escapeRegex = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

function stripDartComments(source) {
	return source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
}

function enclosed(source, start, open, close) {
	let depth = 0;
	for (let index = start; index < source.length; index++) {
		if (source[index] === open) depth++;
		if (source[index] === close && --depth === 0) {
			return source.slice(start + 1, index);
		}
	}
	throw new Error(`Unclosed ${open} in Dart token declaration`);
}

function dartClass(source, className) {
	const match = new RegExp(`\\bclass\\s+${escapeRegex(className)}\\s*\\{`).exec(
		source,
	);
	if (!match) throw new Error(`Missing Dart class ${className}`);
	return enclosed(source, match.index + match[0].length - 1, "{", "}");
}

function readDart(source, key, property, seen = new Set()) {
	if (seen.has(key)) throw new Error(`Circular Dart alias ${key}`);
	seen.add(key);
	const [className, member] = key.split(".");
	if (!className || !member) throw new Error(`Expected Class.member: ${key}`);
	const body = dartClass(source, className);
	if (property) {
		const getter = new RegExp(
			`static\\s+TextStyle\\s+get\\s+${escapeRegex(member)}\\s*=>\\s*(?:const\\s+)?TextStyle\\s*\\(`,
		).exec(body);
		if (!getter) throw new Error(`Missing TextStyle getter ${key}`);
		const fields = enclosed(
			body,
			getter.index + getter[0].length - 1,
			"(",
			")",
		);
		const field = new RegExp(
			`\\b${escapeRegex(property)}\\s*:\\s*([^,\\n]+)`,
		).exec(fields);
		if (!field) throw new Error(`Missing property ${key}.${property}`);
		return field[1].trim();
	}
	const match = new RegExp(
		`static\\s+const\\s+[\\w?]+\\s+${escapeRegex(member)}\\s*=\\s*([^;]+);`,
	).exec(body);
	if (!match) throw new Error(`Missing Dart constant ${key}`);
	const value = match[1].trim();
	if (/^[A-Za-z_]\w*(?:\.\w+)?$/.test(value) && value !== "null") {
		return readDart(
			source,
			value.includes(".") ? value : `${className}.${value}`,
			undefined,
			seen,
		);
	}
	return value;
}

function cssRootTokens(source, path) {
	const values = new Map();
	postcss.parse(source, { from: path }).walkRules((rule) => {
		// Deliberately default root only. Never blend dark, media or route rules
		// into a nominal light-theme token assertion.
		if (
			rule.parent?.type !== "root" ||
			!rule.selector
				.split(",")
				.map((part) => part.trim())
				.includes(":root")
		)
			return;
		for (const node of rule.nodes) {
			if (node.type === "decl" && node.prop.startsWith("--"))
				values.set(node.prop, node.value);
		}
	});
	return values;
}

function resolveCss(value, pool, seen = new Set()) {
	return value.replace(/var\(\s*(--[\w-]+)\s*\)/g, (_, key) => {
		if (seen.has(key)) throw new Error(`Circular CSS alias ${key}`);
		if (!pool.has(key)) throw new Error(`Missing CSS alias ${key}`);
		return resolveCss(pool.get(key), pool, new Set([...seen, key]));
	});
}

function number(value) {
	const cleaned = value
		.replace(/\.sp\b/g, "")
		.replace(/^FontWeight\.w/, "")
		.trim();
	if (/^-?\d+(?:\.\d+)?$/.test(cleaned)) return Number(cleaned);
	const fraction = /^(-?\d+(?:\.\d+)?)\s*\/\s*(\d+(?:\.\d+)?)$/.exec(cleaned);
	if (fraction && Number(fraction[2]) !== 0)
		return Number(fraction[1]) / Number(fraction[2]);
	throw new Error(`Unsupported numeric token ${value}`);
}

function normalize(value, type) {
	const input = value.trim();
	if (type === "color") {
		const dart = /^Color\(0x([\da-f]{8})\)$/i.exec(input);
		if (dart) {
			if (dart[1].slice(0, 2).toLowerCase() !== "ff")
				throw new Error(`Expected opaque color, received ${input}`);
			return `#${dart[1].slice(2).toLowerCase()}`;
		}
		if (/^#[\da-f]{6}$/i.test(input)) return input.toLowerCase();
		if (/^#[\da-f]{3}$/i.test(input))
			return `#${[...input.slice(1)]
				.map((char) => char.repeat(2))
				.join("")
				.toLowerCase()}`;
		throw new Error(`Unsupported opaque color ${input}`);
	}
	if (type === "dimension") {
		const css = /^([\d.]+)(px|rem)$/.exec(input);
		return css ? Number(css[1]) * (css[2] === "rem" ? 16 : 1) : number(input);
	}
	if (type === "number") return number(input);
	if (type === "duration") {
		const match = /^(?:Duration\(milliseconds:\s*(\d+)\)|(\d+)ms)$/.exec(input);
		if (!match) throw new Error(`Unsupported duration ${input}`);
		return Number(match[1] ?? match[2]);
	}
	if (type === "cubicBezier") {
		const match = /^(?:Cubic|cubic-bezier)\(([^)]+)\)$/.exec(input);
		if (!match) throw new Error(`Unsupported easing ${input}`);
		const points = match[1].split(",").map(number);
		if (points.length !== 4)
			throw new Error(`Expected four easing points: ${input}`);
		return points;
	}
	if (type === "string") return input.replace(/^['"]|['"]$/g, "");
	throw new Error(`Unsupported token type ${type}`);
}

/** Validate declarations against the source contract; never modifies source. */
export function checkCrossPlatformTokens({
	webRoot = defaultRoot,
	appsRoot,
	contract,
} = {}) {
	const manifest =
		contract ??
		JSON.parse(readFileSync(resolve(webRoot, contractPath), "utf8"));
	if (manifest.schemaVersion !== 1)
		throw new Error("Unsupported cross-platform token schema");
	const failures = [];
	const sourceCache = new Map();
	const cssPool = new Map();
	const active = new Set(appsRoot ? ["web", "buyer", "seller"] : ["web"]);
	for (const [id, source] of Object.entries(manifest.sources)) {
		if (!active.has(source.platform)) continue;
		try {
			const path = resolve(
				source.platform === "web" ? webRoot : appsRoot,
				source.path,
			);
			const content = readFileSync(path, "utf8");
			if (source.format === "css") {
				const values = cssRootTokens(content, path);
				sourceCache.set(id, values);
				for (const [key, value] of values) cssPool.set(key, value);
			} else if (source.format === "dart")
				sourceCache.set(id, stripDartComments(content));
			else throw new Error(`Unsupported source format ${source.format}`);
		} catch (error) {
			failures.push(`${id}: ${error.message}`);
		}
	}
	let checked = 0;
	let exceptionChecks = 0;
	const ids = new Set();
	for (const [kind, entries] of [
		["canonical", manifest.tokens],
		["exception", manifest.exceptions],
	]) {
		for (const entry of entries) {
			if (ids.has(entry.id)) failures.push(`Duplicate contract ID ${entry.id}`);
			ids.add(entry.id);
			if (!entry.bindings?.length)
				failures.push(`${entry.id}: no source bindings`);
			for (const binding of entry.bindings ?? []) {
				const source = manifest.sources[binding.source];
				const label = `${entry.id} [${binding.source}:${binding.key}${binding.property ? `.${binding.property}` : ""}]`;
				if (!source) {
					failures.push(`${label}: unknown source`);
					continue;
				}
				if (!active.has(source.platform)) continue;
				checked++;
				if (kind === "exception") exceptionChecks++;
				try {
					if (!sourceCache.has(binding.source))
						throw new Error("Source unavailable");
					const content = sourceCache.get(binding.source);
					let raw;
					if (source.format === "css") {
						if (!content.has(binding.key))
							throw new Error("Missing root CSS token");
						raw = resolveCss(content.get(binding.key), cssPool);
					} else raw = readDart(content, binding.key, binding.property);
					const actual = normalize(raw, entry.type);
					const expected = kind === "exception" ? binding.value : entry.value;
					if (JSON.stringify(actual) !== JSON.stringify(expected)) {
						throw new Error(
							`expected ${JSON.stringify(expected)}, found ${JSON.stringify(actual)}${kind === "exception" ? " (review and update the tracked exception)" : ""}`,
						);
					}
				} catch (error) {
					failures.push(`${label}: ${error.message}`);
				}
			}
		}
	}
	if (!manifest.tokens.length || !checked)
		failures.push("Contract has no active canonical checks");
	return {
		passed: failures.length === 0,
		checked,
		exceptionChecks,
		canonicalTokens: manifest.tokens.length,
		platforms: [...active],
		failures,
	};
}

function main() {
	const args = process.argv.slice(2);
	let appsRoot;
	for (let index = 0; index < args.length; index++) {
		if (args[index] === "--help") {
			console.log(
				"Usage: node scripts/check-cross-platform-tokens.mjs [--apps-root ..]\nDefault checks Web only; --apps-root also requires both Flutter repositories.",
			);
			return;
		}
		if (
			args[index] !== "--apps-root" ||
			!args[index + 1] ||
			args[index + 1].startsWith("--")
		)
			throw new Error(`Unknown or incomplete argument: ${args[index]}`);
		appsRoot = resolve(args[++index]);
	}
	const result = checkCrossPlatformTokens({ appsRoot });
	if (!result.passed) {
		console.error(
			`Cross-platform token gate failed:\n${result.failures.join("\n")}`,
		);
		process.exitCode = 1;
		return;
	}
	console.log(
		`Cross-platform token gate passed: ${result.canonicalTokens} canonical tokens, ${result.checked} source checks (${result.exceptionChecks} tracked-exception checks), platforms: ${result.platforms.join(", ")}.`,
	);
	if (!appsRoot)
		console.log(
			"Flutter repositories were not checked. Add --apps-root to verify all three platforms.",
		);
	console.log(
		"Scope: source declarations, light theme, nominal 16px Web root; not rendered parity or complete token adoption.",
	);
}

if (
	process.argv[1] &&
	resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
	try {
		main();
	} catch (error) {
		console.error(error.message);
		process.exitCode = 1;
	}
}
