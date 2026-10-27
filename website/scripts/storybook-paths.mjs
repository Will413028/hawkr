/** Shared by catalog builds and tooling; contains no browser or Node globals. */
export function normalizeStorybookBasePath(value = "/") {
	if (typeof value !== "string" || !value.startsWith("/")) {
		throw new Error("STORYBOOK_BASE_PATH must be an absolute URL path.");
	}
	if (
		value.includes("//") ||
		/[?#\\\s%]/.test(value) ||
		value.split("/").some((segment) => segment === "." || segment === "..")
	) {
		throw new Error(
			"STORYBOOK_BASE_PATH must not contain URL origins or traversal.",
		);
	}
	return `${value.replace(/\/+$/, "")}/`;
}

const staticPrefixes = [
	"/assets/",
	"/brand/",
	"/fonts/",
	"/placeholders/",
	"/generated/",
	"/templates/",
];

/** Rebase only static asset paths; API endpoints and application routes stay intact. */
export function rebaseCatalogAssetUrl(value, basePath = "/") {
	const base = normalizeStorybookBasePath(basePath);
	if (base === "/" || value.startsWith(base)) return value;
	return staticPrefixes.some((prefix) => value.startsWith(prefix))
		? `${base}${value.slice(1)}`
		: value;
}

/** The source font stylesheet references repo assets relative to styles/foundation. */
export function rebaseCatalogCssUrl(value, basePath = "/") {
	const base = normalizeStorybookBasePath(basePath);
	const font = /^(?:\.\.\/)+assets\/fonts\/(.+)$/.exec(value);
	if (font && !font[1].split("/").includes("..")) {
		return `${base}assets/fonts/${font[1]}`;
	}
	return rebaseCatalogAssetUrl(value, base);
}

export function flutterCatalogBasePath(basePath, app) {
	if (app !== "buyer" && app !== "seller")
		throw new Error("Unknown catalog App.");
	return `${normalizeStorybookBasePath(basePath)}flutter/${app}/`;
}
