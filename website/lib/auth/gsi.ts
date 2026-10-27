/**
 * Google Identity Services loader. Loads the GSI client script once and
 * resolves the idToken (the `credential` in the callback) for a given button
 * container. No popup chain — GSI renders a declarative button.
 */

export const GSI_SRC = "https://accounts.google.com/gsi/client";
export const GSI_LOAD_TIMEOUT_MS = 8_000;
export const GSI_RETRY_BACKOFF_MS = [750] as const;

const GSI_SCRIPT_MARKER = "data-hawkr-gsi-loader";

export type GsiFailureStage =
	| "script_load"
	| "script_timeout"
	| "api_missing"
	| "initialize"
	| "render";

interface GsiUnavailableErrorOptions extends ErrorOptions {
	attempts?: number;
	firstFailureStage?: GsiFailureStage;
}

export class GsiUnavailableError extends Error {
	readonly attempts: number;
	readonly firstFailureStage: GsiFailureStage;

	constructor(
		readonly stage: GsiFailureStage,
		options: GsiUnavailableErrorOptions = {},
	) {
		const {
			attempts = 1,
			firstFailureStage = stage,
			...errorOptions
		} = options;
		super(`Google Identity Services unavailable at ${stage}`, errorOptions);
		this.name = "GsiUnavailableError";
		this.attempts = attempts;
		this.firstFailureStage = firstFailureStage;
	}
}

export function getGsiFailureStage(
	error: unknown,
): GsiFailureStage | "unknown" {
	return error instanceof GsiUnavailableError ? error.stage : "unknown";
}

export function getGsiFailureAttempts(error: unknown): number {
	return error instanceof GsiUnavailableError ? error.attempts : 1;
}

export function getGsiFirstFailureStage(
	error: unknown,
): GsiFailureStage | "unknown" {
	return error instanceof GsiUnavailableError
		? error.firstFailureStage
		: "unknown";
}

export interface GsiLoadResult {
	attempts: number;
	recovered: boolean;
	recoveredFrom?: GsiFailureStage;
	durationMs: number;
}

export interface GsiLoadOptions {
	timeoutMs?: number;
	retryBackoffMs?: readonly number[];
}

interface GsiCredentialResponse {
	credential: string; // the Google ID token (JWT)
}

interface GoogleAccountsId {
	initialize(config: {
		client_id: string;
		callback: (resp: GsiCredentialResponse) => void;
		ux_mode?: "popup" | "redirect";
		auto_select?: boolean;
		nonce?: string;
	}): void;
	renderButton(parent: HTMLElement, options: Record<string, unknown>): void;
	prompt(): void;
}

declare global {
	interface Window {
		google?: { accounts: { id: GoogleAccountsId } };
	}
}

let loadPromise: Promise<GsiLoadResult> | null = null;
let loadState: "idle" | "loading" | "ready" = "idle";
let activeScript: HTMLScriptElement | null = null;
let activeTimeout: ReturnType<typeof setTimeout> | null = null;

function now(): number {
	return typeof performance === "undefined" ? Date.now() : performance.now();
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

function hasGsiApi(): boolean {
	return Boolean(window.google?.accounts?.id);
}

function removeOwnedScripts(): void {
	if (typeof document === "undefined") return;
	for (const script of document.querySelectorAll<HTMLScriptElement>(
		`script[${GSI_SCRIPT_MARKER}="true"]`,
	)) {
		script.onload = null;
		script.onerror = null;
		script.remove();
	}
	activeScript = null;
}

function loadGsiAttempt(attempt: number, timeoutMs: number): Promise<void> {
	return new Promise((resolve, reject) => {
		const s = document.createElement("script");
		let settled = false;

		const finish = (
			result:
				| { ok: true; removeScript?: boolean }
				| { ok: false; error: GsiUnavailableError },
		) => {
			if (settled) return;
			settled = true;
			if (activeTimeout) {
				clearTimeout(activeTimeout);
				activeTimeout = null;
			}
			s.onload = null;
			s.onerror = null;
			activeScript = null;
			if (!result.ok || result.removeScript) s.remove();
			if (result.ok) resolve();
			else reject(result.error);
		};

		activeScript = s;
		s.setAttribute(GSI_SCRIPT_MARKER, "true");
		s.src = GSI_SRC;
		s.async = true;
		s.defer = true;
		s.onload = () => {
			if (hasGsiApi()) {
				finish({ ok: true });
				return;
			}
			finish({
				ok: false,
				error: new GsiUnavailableError("api_missing", { attempts: attempt }),
			});
		};
		s.onerror = () => {
			if (hasGsiApi()) {
				finish({ ok: true, removeScript: true });
				return;
			}
			finish({
				ok: false,
				error: new GsiUnavailableError("script_load", { attempts: attempt }),
			});
		};
		activeTimeout = setTimeout(() => {
			if (hasGsiApi()) {
				finish({ ok: true, removeScript: true });
				return;
			}
			finish({
				ok: false,
				error: new GsiUnavailableError("script_timeout", {
					attempts: attempt,
				}),
			});
		}, timeoutMs);
		try {
			document.head.appendChild(s);
		} catch (error) {
			finish({
				ok: false,
				error: new GsiUnavailableError("script_load", {
					attempts: attempt,
					cause: error,
				}),
			});
		}
	});
}

export function loadGsi(options: GsiLoadOptions = {}): Promise<GsiLoadResult> {
	if (typeof window === "undefined") {
		return Promise.resolve({ attempts: 0, recovered: false, durationMs: 0 });
	}
	if (hasGsiApi()) {
		loadState = "ready";
		return Promise.resolve({ attempts: 0, recovered: false, durationMs: 0 });
	}
	if (loadState === "loading" && loadPromise) return loadPromise;

	// A previously successful script lost its API (for example after an SDK
	// bootstrap that fired load before exposing `google.accounts.id`). Never
	// keep returning that fulfilled promise: discard the stale node and rebuild.
	if (loadState === "ready") {
		loadState = "idle";
		loadPromise = null;
		removeOwnedScripts();
	}

	const timeoutMs = options.timeoutMs ?? GSI_LOAD_TIMEOUT_MS;
	const retryBackoffMs = options.retryBackoffMs ?? GSI_RETRY_BACKOFF_MS;
	const startedAt = now();
	loadState = "loading";
	loadPromise = (async () => {
		let firstFailureStage: GsiFailureStage | undefined;
		for (let attempt = 1; attempt <= retryBackoffMs.length + 1; attempt += 1) {
			// A timed-out/removed script can still finish evaluating, or another
			// caller may have supplied GSI during the backoff. Reuse that API
			// instead of appending a duplicate script or reporting a false failure.
			if (firstFailureStage && hasGsiApi()) {
				loadState = "ready";
				return {
					attempts: attempt - 1,
					recovered: true,
					recoveredFrom: firstFailureStage,
					durationMs: Math.round(now() - startedAt),
				};
			}
			try {
				await loadGsiAttempt(attempt, timeoutMs);
				loadState = "ready";
				return {
					attempts: attempt,
					recovered: attempt > 1,
					recoveredFrom: firstFailureStage,
					durationMs: Math.round(now() - startedAt),
				};
			} catch (error) {
				if (error instanceof GsiUnavailableError) {
					firstFailureStage ??= error.stage;
				}
				if (hasGsiApi()) {
					loadState = "ready";
					return {
						attempts: attempt,
						recovered: true,
						recoveredFrom: firstFailureStage,
						durationMs: Math.round(now() - startedAt),
					};
				}
				const retryDelay = retryBackoffMs[attempt - 1];
				if (retryDelay === undefined) {
					loadState = "idle";
					loadPromise = null;
					if (error instanceof GsiUnavailableError) {
						throw new GsiUnavailableError(error.stage, {
							attempts: error.attempts,
							firstFailureStage,
						});
					}
					throw error;
				}
				await delay(retryDelay);
			}
		}
		throw new GsiUnavailableError("script_load", {
			attempts: retryBackoffMs.length + 1,
			firstFailureStage,
		});
	})();
	return loadPromise;
}

export interface GsiRenderOptions {
	signal?: AbortSignal;
	/**
	 * 寫進 ID token 的 `nonce`（OIDC Core §3.2.2.1）：呼叫端要把 token 綁在某個 session 上時，由伺服器
	 * 給、原樣交給 GSI，伺服器驗 token 時比對。沒有給就不帶。
	 */
	nonce?: string;
}

function throwIfAborted(signal?: AbortSignal): void {
	if (!signal?.aborted) return;
	const error = new Error("Google Identity Services render aborted");
	error.name = "AbortError";
	throw error;
}

/** Initialise GSI with a credential (idToken) callback and render its button into `el`. */
export async function renderGoogleButton(
	el: HTMLElement,
	clientId: string,
	onIdToken: (idToken: string) => void,
	options: GsiRenderOptions = {},
): Promise<GsiLoadResult> {
	const loadResult = await loadGsi();
	throwIfAborted(options.signal);
	const id = window.google?.accounts?.id;
	if (!id) {
		loadState = "idle";
		loadPromise = null;
		removeOwnedScripts();
		throw new GsiUnavailableError("api_missing", {
			attempts: loadResult.attempts,
			firstFailureStage: loadResult.recoveredFrom,
		});
	}
	try {
		id.initialize({
			client_id: clientId,
			callback: (resp) => {
				if (!options.signal?.aborted) onIdToken(resp.credential);
			},
			ux_mode: "popup",
			...(options.nonce ? { nonce: options.nonce } : {}),
		});
	} catch (error) {
		throw new GsiUnavailableError("initialize", {
			attempts: loadResult.attempts,
			firstFailureStage: loadResult.recoveredFrom,
			cause: error,
		});
	}
	// `width` is the only GSI knob that controls the click target's pixel
	// width. The visible button on top is fluid (100% of container), so we
	// pin the iframe width to the parent's measured width — falling back to
	// 400 (GSI's max) when measurement is unavailable. shape/theme are
	// invisible (the iframe sits at opacity:0 under our custom button), but
	// "large" keeps the click target ~40px so it covers the 52px CTA's
	// vertical centre comfortably.
	const measured = Math.min(400, Math.round(el.getBoundingClientRect().width));
	throwIfAborted(options.signal);
	try {
		id.renderButton(el, {
			theme: "outline",
			size: "large",
			width: measured || 400,
			text: "continue_with",
		});
	} catch (error) {
		throw new GsiUnavailableError("render", {
			attempts: loadResult.attempts,
			firstFailureStage: loadResult.recoveredFrom,
			cause: error,
		});
	}
	return loadResult;
}

/** Test-only reset for the module-level promise, timer, and owned script. */
export function __resetGsiForTest(): void {
	if (activeTimeout) {
		clearTimeout(activeTimeout);
		activeTimeout = null;
	}
	activeScript?.remove();
	removeOwnedScripts();
	loadPromise = null;
	loadState = "idle";
}
