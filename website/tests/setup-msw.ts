import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./mocks/server";
import { unhandledRequests } from "./setup/unhandled-requests";

beforeAll(() => {
	server.listen({
		onUnhandledRequest: (request) => unhandledRequests.record(request),
	});
});

afterEach(() => {
	server.resetHandlers();
	unhandledRequests.assertNone();
});

afterAll(() => {
	server.close();
});
