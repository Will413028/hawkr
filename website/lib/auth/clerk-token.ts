type ClerkSession = {
	getToken: (options?: { skipCache?: boolean }) => Promise<string | null>;
};

type BrowserClerk = {
	loaded?: boolean;
	session?: ClerkSession | null;
};

const TOKEN_WAIT_MS = 10_000;

export class AuthUnavailableError extends Error {
	constructor() {
		super("登入服務暫時無法使用，請稍後再試");
		this.name = "AuthUnavailableError";
	}
}

export class ClerkSessionEndedError extends Error {
	constructor() {
		super("登入已失效，請重新登入");
		this.name = "ClerkSessionEndedError";
	}
}

function browserClerk(): BrowserClerk | undefined {
	return (window as Window & { Clerk?: BrowserClerk }).Clerk;
}

export async function clerkSessionToken(
	options: { skipCache?: boolean } = {},
	signal?: AbortSignal | null,
): Promise<string | null> {
	signal?.throwIfAborted();
	if (typeof window === "undefined") return null;
	const deadline = Date.now() + TOKEN_WAIT_MS;
	let clerk = browserClerk();
	while (!clerk || clerk.loaded === false) {
		if (Date.now() >= deadline) throw new AuthUnavailableError();
		await waitFor(new Promise((resolve) => setTimeout(resolve, 50)), signal);
		clerk = browserClerk();
	}
	const session = clerk.session;
	if (!session) throw new ClerkSessionEndedError();
	let timer: ReturnType<typeof setTimeout> | undefined;
	try {
		const token = await waitFor(
			Promise.race([
				session.getToken(options),
				new Promise<never>((_, reject) => {
					timer = setTimeout(
						() => reject(new AuthUnavailableError()),
						Math.max(0, deadline - Date.now()),
					);
				}),
			]),
			signal,
		);
		if (token) return token;
		if (!browserClerk()?.session) throw new ClerkSessionEndedError();
		throw new AuthUnavailableError();
	} catch (error) {
		signal?.throwIfAborted();
		if (
			error instanceof AuthUnavailableError ||
			error instanceof ClerkSessionEndedError
		) {
			throw error;
		}
		const status = (error as { status?: number } | null)?.status;
		if (status === 401 || status === 403 || !browserClerk()?.session) {
			throw new ClerkSessionEndedError();
		}
		throw new AuthUnavailableError();
	} finally {
		clearTimeout(timer);
	}
}

export async function withClerkAuthorization(
	headers: Headers,
	options: { skipCache?: boolean } = {},
	signal?: AbortSignal | null,
): Promise<void> {
	try {
		const token = await clerkSessionToken(options, signal);
		if (token) headers.set("Authorization", `Bearer ${token}`);
	} catch (error) {
		if (!(error instanceof ClerkSessionEndedError)) throw error;
		headers.delete("Authorization");
	}
}

function waitFor<T>(
	promise: Promise<T>,
	signal?: AbortSignal | null,
): Promise<T> {
	if (!signal) return promise;
	signal.throwIfAborted();
	return new Promise<T>((resolve, reject) => {
		const abort = () => reject(signal.reason);
		signal.addEventListener("abort", abort, { once: true });
		promise
			.then(resolve, reject)
			.finally(() => signal.removeEventListener("abort", abort));
	});
}
