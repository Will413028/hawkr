#!/usr/bin/env node

/**
 * Dependency-override gate.
 *
 * `overrides` in pnpm-workspace.yaml is us overriding a version decision that
 * belongs to somebody else's package. That is legitimate when a transitive
 * dependency has an advisory and the intermediate package has not bumped yet:
 * we cannot patch ajv, so we raise the floor on what ajv resolves. What makes
 * it debt is that nothing ever forces the entry back out again — the upstream
 * fix lands, the override stays, and it silently pins the tree for years.
 *
 * So each entry must earn its place on every CI run. Five rules (plus a
 * sixth for `allowBuilds`, below):
 *
 * 1. It must carry `REMOVE-WHEN: requesters-demand >=<version>` — the version
 *    at which the entry stops being necessary. No entry without a stated exit.
 *
 * 2. The value must be BOUNDED. `>=3.1.4` is not a pin, it is a subscription:
 *    pnpm resolves it to the newest thing on the registry, so `fast-uri`
 *    silently rode from 3.x into 4.1.1 — a major ajv never declared — and that
 *    borrowed major is what picked up GHSA-7p8r-x3mc-p8w7 and took the deploy
 *    gate down on 2026-08-03. An override must name a range it cannot escape.
 *
 * 3. The overridden package must still be in the tree at all. Drop the
 *    dependency that dragged it in and the entry becomes dead config.
 *
 * 4. The redundancy check, and the reason this file exists: read what every
 *    upstream requester actually declares. Once they all demand the
 *    REMOVE-WHEN version or newer, the override is doing nothing — the gate
 *    turns red and names the entry to delete. A comment saying "remove this
 *    later" is a wish; this is the thing that comes to collect.
 *
 * 5. Rules 1-4 assume the override remains inside every upstream requester's
 *    declared range. Sometimes one workspace-wide security floor cannot do
 *    that. `postcss` is the current example: Next pins 8.5.23 exactly, while the
 *    workspace needs a bounded `^8.5.23` security floor for other requesters.
 *    The floor is safe, but it is still broader than Next declared and must be
 *    reviewed explicitly rather than silently widening an exact dependency.
 *
 *    That is a legitimate exception, but it must be a STATED one. So whenever
 *    the forced range is not a subset of what a requester declares, the entry
 *    must carry `EXCEEDS-UPSTREAM: <requester>, <requester>` naming exactly the
 *    packages it overshoots. The gate compares that list against what it
 *    computes, both directions:
 *      - overshooting somebody who is not listed → red (a silent superset,
 *        which is the state this rule was written to end)
 *      - listing somebody we no longer overshoot → red (delete that name; if
 *        the list empties, delete the marker, and rule 4 will usually be
 *        telling you to delete the whole entry by then)
 *
 *    So the marker cannot be used to wave the check through. It is a claim
 *    about the tree, re-checked on every run, and it goes stale loudly.
 *
 * 6. `allowBuilds` has the same disease: a package that left the tree keeps its
 *    install-script approval forever. Every entry must still be in the tree
 *    (same `present` set as rule 3); otherwise delete it.
 *
 * 7. The npm `overrides` of backend/workers/image-gateway get rules 1-5 too
 *    (scoped under the requester, notes in `//overrides`; see the npm section
 *    below), checked against its package-lock.json.
 *
 * Rules 4 and 5 only ever fire when the tree has genuinely moved, which is
 * exactly when editing the entry is safe.
 */

import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

// 根目錄的 workspace（pnpm-workspace.yaml 與 node_modules/.pnpm 都在這裡），
// 由腳本位置推得，從任何 app 目錄呼叫結果相同；測試用 CHECK_OVERRIDES_ROOT 指向 fixture。
const root = process.env.CHECK_OVERRIDES_ROOT ?? join(import.meta.dirname, "..");
const failures: string[] = [];

type Override = {
	/** Package being overridden, e.g. `brace-expansion`. */
	name: string;
	/** Optional selector from a `pkg@range` key — scopes which line we target. */
	selector: string | null;
	/** Version/range we force it to. */
	value: string;
	/** Version from `REMOVE-WHEN: requesters-demand >=X`. */
	removeWhen: string | null;
	/**
	 * Requester names from `EXCEEDS-UPSTREAM: a, b`, or null when the entry
	 * carries no such marker. An empty array means the marker was written with
	 * nothing after it, which rule 5 rejects the same as a wrong list.
	 */
	exceedsUpstream: string[] | null;
	line: number;
};

/* ------------------------------------------------------------------ *
 * semver helpers (kept deliberately small — no runtime dep for a gate)
 * ------------------------------------------------------------------ */

type Version = [number, number, number];

/** Shared by the pnpm comments and the npm `//overrides` notes. */
function parseNotes(text: string): {
	removeWhen: string | null;
	exceedsUpstream: string[] | null;
} {
	const removeWhen = text.match(
		/REMOVE-WHEN:\s*requesters-demand\s*>=\s*([0-9][^\s.]*(?:\.[0-9][^\s.]*)*)/,
	);
	// Stop at the em dash so a trailing prose reason on the same line ("— the
	// advisory covers the whole declared range") is not read as a package name.
	const exceeds = text.match(/EXCEEDS-UPSTREAM:\s*([^—]*)/);
	return {
		removeWhen: removeWhen ? removeWhen[1] : null,
		exceedsUpstream: exceeds
			? exceeds[1]
					.split(",")
					.map((x) => x.trim())
					.filter(Boolean)
			: null,
	};
}

function parseVersion(raw: string): Version | null {
	const m = raw.match(/^(\d+)\.(\d+)\.(\d+)/);
	return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

function compare(a: Version, b: Version): number {
	return a[0] - b[0] || a[1] - b[1] || a[2] - b[2];
}

/**
 * Lowest version a range can resolve to. Returns null when the range is not a
 * plain registry range (`workspace:`, `npm:`, git URLs) or uses alternation —
 * we would rather skip a requester than wrongly declare an override redundant.
 */
function rangeFloor(range: string): Version | null {
	const r = range.trim();
	if (!r || r.includes("||")) return null;
	if (/^(workspace|npm|file|link|git|github):/.test(r)) return null;
	if (/^[*x]$/.test(r)) return null;
	return parseVersion(r.replace(/^[\^~><=\s]+/, ""));
}

/**
 * Half-open interval `[lo, hi)` a range can resolve within; `hi === null` means
 * unbounded above. Returns null for anything this deliberately small parser
 * cannot read exactly (alternation, compound `>=x <y`, protocol specifiers) —
 * a skipped requester can only make rule 5 quieter, never wrong.
 */
type Interval = { lo: Version; hi: Version | null };

function rangeInterval(range: string): Interval | null {
	const r = range.trim();
	if (!r || r.includes("||") || /\s/.test(r)) return null;
	if (/^(workspace|npm|file|link|git|github):/.test(r)) return null;
	if (/^[*x]$/.test(r)) return null;

	const gte = r.startsWith(">=");
	const caret = r.startsWith("^");
	const tilde = r.startsWith("~");
	// `>x`, `<x`, `<=x` and anything else exotic: skip rather than guess.
	if (!gte && !caret && !tilde && !/^\d/.test(r)) return null;

	const lo = parseVersion(r.replace(/^[\^~>=\s]+/, ""));
	if (!lo) return null;

	if (gte) return { lo, hi: null };
	if (caret) {
		// The whole point of this rule: on 0.x, `^` does NOT open the minor.
		// `^0.34.5` is `<0.35.0`; `^0.0.3` is `<0.0.4`; `^1.2.3` is `<2.0.0`.
		if (lo[0] > 0) return { lo, hi: [lo[0] + 1, 0, 0] };
		if (lo[1] > 0) return { lo, hi: [0, lo[1] + 1, 0] };
		return { lo, hi: [0, 0, lo[2] + 1] };
	}
	if (tilde) return { lo, hi: [lo[0], lo[1] + 1, 0] };
	// Exact version.
	return { lo, hi: [lo[0], lo[1], lo[2] + 1] };
}

/** Is every version `inner` admits also admitted by `outer`? */
function containsInterval(outer: Interval, inner: Interval): boolean {
	if (compare(outer.lo, inner.lo) > 0) return false;
	if (outer.hi === null) return true;
	if (inner.hi === null) return false;
	return compare(inner.hi, outer.hi) <= 0;
}

/** A bounded range cannot drift into a major nobody declared (rule 2). */
function isBounded(value: string): boolean {
	const v = value.trim();
	if (!v || /^[*x]$/.test(v)) return false;
	// `>=x` / `>x` are open-ended; `^x` / `~x` / exact / `<x` are not.
	if (/^>=?[^=]/.test(v)) return false;
	return true;
}

/* ------------------------------------------------------------------ *
 * Parse the `overrides:` block of pnpm-workspace.yaml
 * ------------------------------------------------------------------ */

const workspacePath = join(root, "pnpm-workspace.yaml");
const workspaceLines = readFileSync(workspacePath, "utf8").split("\n");

const overrides: Override[] = [];
let inOverrides = false;
let pendingComments: string[] = [];

for (let i = 0; i < workspaceLines.length; i++) {
	const line = workspaceLines[i];
	if (/^overrides:\s*$/.test(line)) {
		inOverrides = true;
		pendingComments = [];
		continue;
	}
	if (!inOverrides) continue;
	// A non-indented, non-empty line ends the block.
	if (line.trim() !== "" && !/^\s/.test(line)) break;
	if (line.trim() === "") continue;

	const comment = line.match(/^\s+#\s?(.*)$/);
	if (comment) {
		pendingComments.push(comment[1]);
		continue;
	}

	const entry = line.match(/^\s+(.+?):\s*(.+?)\s*$/);
	if (!entry) continue;

	const rawKey = entry[1].trim().replace(/^['"]|['"]$/g, "");
	const value = entry[2].trim().replace(/^['"]|['"]$/g, "");

	// Split on the last `@` so scoped names (`@scope/pkg@^1.0.0`) survive.
	const at = rawKey.lastIndexOf("@");
	const scoped = at > 0;
	const name = scoped ? rawKey.slice(0, at) : rawKey;
	const selector = scoped ? rawKey.slice(at + 1) : null;

	const notes = parseNotes(pendingComments.join(" "));

	overrides.push({
		name,
		selector,
		value,
		removeWhen: notes.removeWhen,
		exceedsUpstream: notes.exceedsUpstream,
		line: i + 1,
	});
	pendingComments = [];
}

if (overrides.length === 0) {
	console.error(
		"check-dependency-overrides: no overrides parsed from pnpm-workspace.yaml.",
	);
	console.error(
		"If the block was intentionally emptied, delete this gate too; a gate that",
	);
	console.error("checks nothing is worse than no gate.");
	process.exit(1);
}

/* ------------------------------------------------------------------ *
 * Collect declared dependency ranges from the installed store
 * ------------------------------------------------------------------ */

const pnpmStore = join(root, "node_modules/.pnpm");

let storeEntries: string[];
try {
	storeEntries = readdirSync(pnpmStore);
} catch {
	console.error(
		"check-dependency-overrides: node_modules/.pnpm missing — run `pnpm install` first.",
	);
	process.exit(1);
}

/** pkg name -> list of { requester, declared range } */
const requesters = new Map<
	string,
	{ name: string; by: string; range: string }[]
>();
/** every package name present in the tree (rule 3) */
const present = new Set<string>();

function readManifest(dir: string): void {
	let pkg: {
		name?: string;
		version?: string;
		dependencies?: Record<string, string>;
		optionalDependencies?: Record<string, string>;
	};
	try {
		pkg = JSON.parse(readFileSync(join(dir, "package.json"), "utf8"));
	} catch {
		return;
	}
	if (!pkg.name) return;
	present.add(pkg.name);

	const deps = { ...pkg.dependencies, ...pkg.optionalDependencies };
	for (const [dep, range] of Object.entries(deps)) {
		if (typeof range !== "string") continue;
		const list = requesters.get(dep) ?? [];
		list.push({
			name: pkg.name,
			by: `${pkg.name}@${pkg.version ?? "?"}`,
			range,
		});
		requesters.set(dep, list);
	}
}

for (const entry of storeEntries) {
	const nm = join(pnpmStore, entry, "node_modules");
	let children: import("node:fs").Dirent[];
	try {
		children = readdirSync(nm, { withFileTypes: true });
	} catch {
		continue;
	}
	for (const child of children) {
		// Real directories are the store entry's own package; siblings are
		// symlinks to its dependencies and would double-count.
		if (!child.isDirectory()) continue;
		if (child.name.startsWith("@")) {
			let inner: import("node:fs").Dirent[];
			try {
				inner = readdirSync(join(nm, child.name), { withFileTypes: true });
			} catch {
				continue;
			}
			for (const sub of inner) {
				if (sub.isDirectory()) readManifest(join(nm, child.name, sub.name));
			}
			continue;
		}
		readManifest(join(nm, child.name));
	}
}

/* ------------------------------------------------------------------ *
 * Apply the rules
 * ------------------------------------------------------------------ */

type Requester = { name: string; by: string; range: string };

/**
 * Rules 1-5 for one override. `all` is every declaration of the overridden
 * package by the requesters the override applies to; `inTree` says whether the
 * override still has anything to act on (rule 3).
 */
function checkOverride(o: Override, where: string, all: Requester[], inTree: boolean): void {

	// Rule 1 — a stated exit condition.
	if (!o.removeWhen) {
		failures.push(
			`${where}\n    missing REMOVE-WHEN. Add a comment above the entry:\n` +
				"      # REMOVE-WHEN: requesters-demand >=<version>\n" +
				"    An override with no exit condition never leaves.",
		);
		return;
	}

	const removeWhen = parseVersion(o.removeWhen);
	if (!removeWhen) {
		failures.push(
			`${where}\n    REMOVE-WHEN version "${o.removeWhen}" is not a plain x.y.z version.`,
		);
		return;
	}

	// Rule 2 — bounded value.
	if (!isBounded(o.value)) {
		failures.push(
			`${where}\n    override value "${o.value}" is open-ended.\n` +
				"    pnpm resolves it to the newest published version, so it can drift into\n" +
				"    a major no upstream package declares — and inherit that major's\n" +
				"    advisories. Use a bounded range (^x.y.z) or an exact pin.",
		);
		return;
	}

	// Rule 3 — still in the tree.
	if (!inTree) {
		failures.push(
			`${where}\n    no longer present in the dependency tree — this override is dead\n` +
				"    config. Delete the entry.",
		);
		return;
	}

	// Rule 4 — redundancy.
	const selectorMajor = o.selector ? rangeFloor(o.selector)?.[0] : undefined;

	const relevant: {
		name: string;
		by: string;
		range: string;
		floor: Version;
	}[] = [];
	for (const r of all) {
		const floor = rangeFloor(r.range);
		if (!floor) continue;
		// A `pkg@range` key targets one line only; ignore requesters on others.
		if (selectorMajor !== undefined && floor[0] !== selectorMajor) continue;
		relevant.push({ ...r, floor });
	}

	if (relevant.length > 0) {
		const stragglers = relevant.filter((r) => compare(r.floor, removeWhen) < 0);
		if (stragglers.length === 0) {
			const list = relevant
				.slice(0, 6)
				.map((r) => `      ${r.by} declares ${o.name}@${r.range}`)
				.join("\n");
			failures.push(
				`${where}\n    REMOVE-WHEN satisfied: every requester now demands >=${o.removeWhen},\n` +
					"    so this override no longer changes the resolved version. Delete the\n" +
					"    entry (and this is the safe moment to do it).\n" +
					`${list}`,
			);
		}
	}

	// Rule 5 — an override that lands outside a requester's declared range must
	// say so, and must name exactly the requesters it overshoots.
	const forced = rangeInterval(o.value);
	const exceeded = new Map<string, string>(); // requester name -> declared range
	if (forced) {
		for (const r of relevant) {
			const declared = rangeInterval(r.range);
			if (!declared) continue;
			if (!containsInterval(declared, forced)) exceeded.set(r.name, r.range);
		}
	}

	const declaredExceeds = o.exceedsUpstream;
	const undeclared = [...exceeded.keys()].filter(
		(n) => !declaredExceeds?.includes(n),
	);
	const stale = (declaredExceeds ?? []).filter((n) => !exceeded.has(n));

	if (undeclared.length > 0) {
		const list = undeclared
			.map(
				(n) =>
					`      ${n} declares ${o.name}@${exceeded.get(n)}, we force ${o.value}`,
			)
			.join("\n");
		failures.push(
			`${where}\n    forces a version outside what these requesters declare:\n` +
				`${list}\n` +
				"    That can be the right call — when the advisory covers the whole declared\n" +
				"    range there is no safe version inside it — but it has to be stated, not\n" +
				"    silent. Add to the comment above the entry:\n" +
				`      # EXCEEDS-UPSTREAM: ${[...exceeded.keys()].join(", ")} — <why the declared range is unusable>\n` +
				"    and say in the prose which advisory forces it.",
		);
	}

	if (stale.length > 0) {
		failures.push(
			`${where}\n    EXCEEDS-UPSTREAM names ${stale.join(", ")}, but ${stale.length > 1 ? "those are" : "that is"} no longer\n` +
				"    overshot — the requester moved, or stopped requesting this package.\n" +
				(exceeded.size > 0
					? `    Narrow the marker to: ${[...exceeded.keys()].join(", ")}\n`
					: "    Nothing is overshot any more: delete the marker, and check whether\n    the entry itself is still needed.\n") +
				"    A marker that has stopped being true is worse than none — it reads as\n" +
				"    though somebody checked.",
		);
	}
}

for (const o of overrides) {
	checkOverride(
		o,
		`pnpm-workspace.yaml:${o.line}  ${o.name}`,
		requesters.get(o.name) ?? [],
		present.has(o.name),
	);
}

/* ------------------------------------------------------------------ *
 * Rules 1-5 for npm `overrides` (backend/workers/image-gateway)
 *
 * npm overrides are scoped under the requester they act on:
 *   "overrides": { "<requester>": { "<dep>": "<value>" } }
 * and each one carries its notes in `//overrides`, keyed "<requester> > <dep>",
 * written in the same vocabulary as the pnpm comments:
 *   "EXCEEDS-UPSTREAM: <requester> — <why>. REMOVE-WHEN: requesters-demand >=<version>"
 * The lockfile keeps what each package declared (not the override), so the
 * declared ranges come from package-lock.json.
 * ------------------------------------------------------------------ */

const npmDir = "backend/workers/image-gateway";
let npmCount = 0;
{
	const pkgPath = join(root, npmDir, "package.json");
	const lockPath = join(root, npmDir, "package-lock.json");
	let pkg: { overrides?: Record<string, unknown>; [k: string]: unknown } | null = null;
	try {
		pkg = JSON.parse(readFileSync(pkgPath, "utf8"));
	} catch {
		// no npm project here (fixtures, or the worker was removed): nothing to check
	}
	let lockOk = true;
	if (pkg?.overrides) {
		let lock: { packages?: Record<string, { version?: string; dependencies?: Record<string, string>; optionalDependencies?: Record<string, string> }> } = {};
		try {
			lock = JSON.parse(readFileSync(lockPath, "utf8"));
		} catch {
			failures.push(`${npmDir}/package-lock.json missing or unreadable — run \`npm install\` there.`);
			lockOk = false;
		}
		if (!lockOk) {
			// one error is enough; the remaining npm checks all depend on the lockfile
		} else {
		const notes = (pkg["//overrides"] ?? {}) as Record<string, string>;
		if (typeof pkg["//overrides"] !== "object" || pkg["//overrides"] === null) {
			failures.push(
				`${npmDir}/package.json  \`//overrides\` must be an object keyed "<requester> > <dep>" holding the\n` +
					"    REMOVE-WHEN / EXCEEDS-UPSTREAM notes (same vocabulary as pnpm-workspace.yaml).",
			);
		}
		const seenKeys = new Set<string>();
		for (const [requester, scope] of Object.entries(pkg.overrides)) {
			if (requester === "." || (requester.lastIndexOf("@") > 0)) {
				failures.push(
					`${npmDir}/package.json  overrides.${requester}\n    requester keys with a version selector (\`name@range\`) or the \`.\` self key are not supported by this gate;\n` +
						"    scope the override under the plain package name.",
				);
				continue;
			}
			if (typeof scope !== "object" || scope === null) {
				failures.push(
					`${npmDir}/package.json  overrides.${requester}\n    must be scoped under the requester ({ "<requester>": { "<dep>": "<value>" } });\n` +
						"    an unscoped override rewrites every package that depends on it.",
				);
				continue;
			}
			for (const [dep, value] of Object.entries(scope as Record<string, unknown>)) {
				const where = `${npmDir}/package.json  ${requester} > ${dep}`;
				if (typeof value !== "string") {
					failures.push(`${where}\n    nested overrides are not supported by this gate; flatten to one level.`);
					continue;
				}
				npmCount++;
				seenKeys.add(`${requester} > ${dep}`);
				const note = typeof notes[`${requester} > ${dep}`] === "string" ? notes[`${requester} > ${dep}`] : "";
				const parsed = parseNotes(note);
				const all: Requester[] = [];
				for (const [key, entry] of Object.entries(lock.packages ?? {})) {
					if (key.slice(key.lastIndexOf("node_modules/") + "node_modules/".length) !== requester) continue;
					const range = { ...entry.dependencies, ...entry.optionalDependencies }[dep];
					if (typeof range === "string") all.push({ name: requester, by: `${requester}@${entry.version ?? "?"}`, range });
				}
				checkOverride(
					{
						name: dep,
						selector: null,
						value,
						removeWhen: parsed.removeWhen,
						exceedsUpstream: parsed.exceedsUpstream,
						line: 0,
					},
					where,
					all,
					all.length > 0,
				);
			}
		}
		for (const key of Object.keys(notes)) {
			if (!seenKeys.has(key)) {
				failures.push(
					`${npmDir}/package.json  \`//overrides\` note "${key}" matches no override.\n    Delete it, or fix the key to "<requester> > <dep>".`,
				);
			}
		}
		}
	}
}

// Rule 6 — every `allowBuilds` entry is still in the tree.
const allowBuilds: { name: string; line: number }[] = [];
let inAllowBuilds = false;
for (let i = 0; i < workspaceLines.length; i++) {
	const line = workspaceLines[i];
	if (/^allowBuilds:\s*$/.test(line)) {
		inAllowBuilds = true;
		continue;
	}
	if (!inAllowBuilds) continue;
	if (line.trim() !== "" && !/^\s/.test(line)) break;
	if (line.trim() === "" || /^\s+#/.test(line)) continue;
	const entry = line.match(/^\s+(.+?):\s*\S+\s*$/);
	if (!entry) continue;
	allowBuilds.push({
		name: entry[1].trim().replace(/^['"]|['"]$/g, ""),
		line: i + 1,
	});
}
if (allowBuilds.length === 0) {
	failures.push(
		"pnpm-workspace.yaml: no allowBuilds entries parsed. If the block was\n" +
			"    intentionally emptied, delete this rule too.",
	);
}
for (const a of allowBuilds) {
	if (!present.has(a.name)) {
		failures.push(
			`pnpm-workspace.yaml:${a.line}  allowBuilds ${a.name}\n    not in the dependency tree — the install-script approval is dead config.\n` +
				"    Delete the entry (`pnpm approve-builds` lists what the current tree needs).",
		);
	}
}

if (failures.length > 0) {
	console.error("Dependency-override gate failed:\n");
	for (const f of failures) console.error(`  ${f}\n`);
	console.error(
		`${failures.length} problem(s). See the header of scripts/check-dependency-overrides.ts.`,
	);
	process.exit(1);
}

console.log(
	`check-dependency-overrides: ${overrides.length} pnpm and ${npmCount} npm override(s) and ${allowBuilds.length} allowBuilds entr(ies) still load-bearing.`,
);
