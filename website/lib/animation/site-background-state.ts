type BackgroundState = Readonly<{ available: boolean; paused: boolean }>;
const SERVER_STATE: BackgroundState = { available: false, paused: false };
const STORAGE_KEY = "hk:site-background-paused";
let state = SERVER_STATE;
let hydrated = false;
const listeners = new Set<() => void>();

export const getSiteBackgroundState = () => state;
export const getServerSiteBackgroundState = () => SERVER_STATE;
export function subscribeSiteBackground(listener: () => void) {
	listeners.add(listener);
	return () => {
		listeners.delete(listener);
	};
}
function update(next: BackgroundState) {
	if (next.available === state.available && next.paused === state.paused)
		return;
	state = next;
	for (const listener of listeners) listener();
}
export function setSiteBackgroundAvailable(available: boolean) {
	update({ ...state, available });
}
export function hydrateSiteBackgroundState() {
	if (hydrated) return;
	hydrated = true;
	try {
		const saved =
			sessionStorage.getItem(STORAGE_KEY) ??
			sessionStorage.getItem("hk:home-grainient-paused");
		update({ ...state, paused: saved === "1" });
	} catch {
		/* Optional persistence must never block shopping. */
	}
}
export function toggleSiteBackgroundPaused() {
	const paused = !state.paused;
	update({ ...state, paused });
	try {
		sessionStorage.setItem(STORAGE_KEY, paused ? "1" : "0");
	} catch {
		/* The control still works when storage is unavailable. */
	}
}
