import http from "node:http";
import { SESSION_COOKIE_NAME } from "../../../lib/auth/session-cookie";

/**
 * The monolith's GetSession for the e2e webServer. The site reads the session on the server (lib/auth/dal.ts),
 * so a browser-side `page.route` never sees that call; the server asks API_BASE_URL instead, and for the e2e
 * that is this process. The answer is derived from the request's own session cookie, which
 * `signInWithSessionCookie` fills with the identity: no per-test state lives here, so parallel workers and
 * retries cannot disturb each other. Every other path closes the socket, like the discarded loopback port it
 * replaces, so an unmocked SSR request still fails instead of succeeding quietly.
 */

export const GET_SESSION_PATH =
	"/hawkr.identity.v1alpha1.IdentityAppService/GetSession";

export const SESSION_VALUE_PREFIX = "e2e-session-";

export interface SessionIdentity {
	accountId: string;
	roles: ReadonlyArray<"buyer" | "seller">;
}

/** The cookie value is opaque to the site and the middleware; only this upstream decodes it. */
export function encodeSessionValue(identity: SessionIdentity): string {
	const payload = Buffer.from(
		JSON.stringify({ accountId: identity.accountId, roles: identity.roles }),
	).toString("base64url");
	return `${SESSION_VALUE_PREFIX}${payload}`;
}

export function decodeSessionValue(value: string): SessionIdentity | null {
	if (!value.startsWith(SESSION_VALUE_PREFIX)) return null;
	try {
		const parsed: unknown = JSON.parse(
			Buffer.from(
				value.slice(SESSION_VALUE_PREFIX.length),
				"base64url",
			).toString("utf8"),
		);
		if (typeof parsed !== "object" || parsed === null) return null;
		const { accountId, roles } = parsed as Record<string, unknown>;
		if (typeof accountId !== "string" || !accountId) return null;
		if (!Array.isArray(roles)) return null;
		return {
			accountId,
			roles: roles.filter(
				(role): role is "buyer" | "seller" =>
					role === "buyer" || role === "seller",
			),
		};
	} catch {
		return null;
	}
}

/** proto3 JSON of GetSessionResponse (identity.proto): the workspace is client state, so the response omits it. */
export function sessionJson(identity: SessionIdentity) {
	return {
		account: {
			id: identity.accountId,
			roles: identity.roles.includes("seller") ? ["ROLE_SELLER"] : [],
		},
	};
}

function sessionCookieValue(header: string | undefined): string | null {
	for (const part of (header ?? "").split(";")) {
		const [name, ...rest] = part.trim().split("=");
		if (name === SESSION_COOKIE_NAME) return rest.join("=");
	}
	return null;
}

export function createFakeSessionUpstream(): http.Server {
	return http.createServer((req, res) => {
		const url = new URL(req.url ?? "/", "http://upstream.invalid");
		if (url.pathname === "/healthz") {
			res.writeHead(200, { "content-type": "text/plain" }).end("ok");
			return;
		}
		if (!url.pathname.endsWith(GET_SESSION_PATH)) {
			req.socket.destroy();
			return;
		}
		const value = sessionCookieValue(req.headers.cookie);
		const identity = value === null ? null : decodeSessionValue(value);
		if (!identity) {
			res.writeHead(401, { "content-type": "application/json" }).end(
				JSON.stringify({
					code: "unauthenticated",
					message: "unauthenticated",
				}),
			);
			return;
		}
		res
			.writeHead(200, { "content-type": "application/json" })
			.end(JSON.stringify(sessionJson(identity)));
	});
}
