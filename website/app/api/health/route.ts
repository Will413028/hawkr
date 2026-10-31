/**
 * Liveness probe: "this deployment is up and serving traffic."
 *
 * That sentence is the whole contract, and the endpoint is deliberately built
 * so it cannot answer anything wider. It reads process env and the clock — no
 * gateway call, no R2, no Durable Object, no cache read. A health check that
 * touches a dependency reports the dependency's health, so a slow upstream
 * turns the probe itself red and the monitor pages for an outage the site is
 * not having. Those dependencies have their own signals (the tail worker for
 * Worker-level failures, Sentry for exceptions, the freshness canary at
 * /sentinel for the cache path); this one answers only for the Worker.
 *
 * `buildId` is what makes the response worth more than a bare 200: it is the
 * same deploy namespace the version-promotion workflow and the canary use
 * (VERCEL_DEPLOYMENT_ID, set by the platform), so a monitor polling this
 * endpoint can tell "up" from "up, and already on the version I just shipped".
 * Unset locally, where this env var never exists — hence the explicit
 * "unknown" rather than an omitted field, so the shape never varies.
 *
 * `no-store` is load-bearing. A cached health response describes the past and
 * could let a dead deployment appear healthy after it stopped serving traffic.
 */
export const dynamic = "force-dynamic";
export const revalidate = 0;

export function GET() {
	return Response.json(
		{
			code: 0,
			message: "success",
			data: {
				buildId: process.env.VERCEL_DEPLOYMENT_ID ?? "unknown",
				servedAt: new Date().toISOString(),
			},
		},
		{ headers: { "cache-control": "no-store" } },
	);
}
