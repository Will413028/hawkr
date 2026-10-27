import { toBrowserHref } from "./resolve-url";

/**
 * The final browser I/O, isolated so previews can exercise real navigation state.
 *
 * `target` is an app path (`/`, `/login?role=seller`) or an absolute URL. An app path gets the
 * visitor's language prefix here (`toBrowserHref`, which leaves an already-prefixed path and an
 * absolute URL alone), so every hard navigation stays in the language the page is in.
 * Browser-only: it reads the page's active locale.
 */
export function commitDocumentNavigation(
	target: string,
	replace = false,
): void {
	const url = toBrowserHref(target);
	if (replace) window.location.replace(url);
	else window.location.assign(url);
}
