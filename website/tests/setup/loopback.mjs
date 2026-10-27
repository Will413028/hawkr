/** What counts as "this machine" for the test guard (tests/setup/hermetic.mjs). Pure: importing it patches nothing. */
export function isLoopbackHost(host) {
	const name = String(host ?? "")
		.toLowerCase()
		.replace(/^\[|\]$/g, "");
	return (
		name === "localhost" ||
		name === "::1" ||
		name === "0:0:0:0:0:0:0:1" ||
		/^127(?:\.\d{1,3}){3}$/.test(name)
	);
}

/** An API base the guard leaves alone: a loopback URL, or a relative same-origin path such as `/api/v1`. */
export function isLoopbackBase(value) {
	if (value.startsWith("/")) return true;
	try {
		return isLoopbackHost(new URL(value).hostname);
	} catch {
		return false;
	}
}
