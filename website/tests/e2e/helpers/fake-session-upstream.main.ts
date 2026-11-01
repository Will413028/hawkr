import { createFakeSessionUpstream } from "./fake-session-upstream";

const port = Number(process.env.FAKE_SESSION_UPSTREAM_PORT);
if (!Number.isInteger(port) || port <= 0) {
	throw new Error("FAKE_SESSION_UPSTREAM_PORT must be a port number");
}

const server = createFakeSessionUpstream();
server.listen(port, "127.0.0.1");
for (const signal of ["SIGINT", "SIGTERM"] as const) {
	process.on(signal, () => server.close(() => process.exit(0)));
}
